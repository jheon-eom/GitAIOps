package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"github.com/valkey-io/valkey-go"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	version            = "v0.8.0"
	serviceName        = "notiflex-api"
	notificationsTopic = "notifications"
)

var (
	valkeyClient  valkey.Client
	kafkaProducer sarama.SyncProducer
	tracer        = otel.Tracer(serviceName)
)

type healthResp struct {
	Status string `json:"status"`
}

type idResp struct {
	ID  int64  `json:"id"`
	Pod string `json:"pod"`
}

type versionResp struct {
	Version string `json:"version"`
	Pod     string `json:"pod"`
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(healthResp{Status: "ok"})
}

func idHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	ctx, valkeySpan := tracer.Start(ctx, "valkey.incr",
		trace.WithAttributes(attribute.String("db.system", "valkey"), attribute.String("db.operation", "INCR")))
	next, err := valkeyClient.Do(ctx, valkeyClient.B().Incr().Key("notiflex:id").Build()).AsInt64()
	if err != nil {
		valkeySpan.RecordError(err)
		valkeySpan.End()
		log.Printf("valkey INCR failed: %v", err)
		http.Error(w, "id backend unavailable", http.StatusServiceUnavailable)
		return
	}
	valkeySpan.SetAttributes(attribute.Int64("notiflex.id", next))
	valkeySpan.End()

	pod, _ := os.Hostname()

	if kafkaProducer != nil {
		produceCtx, produceSpan := tracer.Start(ctx, "kafka.produce",
			trace.WithAttributes(
				attribute.String("messaging.system", "kafka"),
				attribute.String("messaging.destination.name", notificationsTopic),
				attribute.Int64("notiflex.id", next),
			))
		headers := injectTraceHeaders(produceCtx)
		msg := &sarama.ProducerMessage{
			Topic:   notificationsTopic,
			Key:     sarama.StringEncoder(strconv.FormatInt(next, 10)),
			Value:   sarama.StringEncoder(`{"id":` + strconv.FormatInt(next, 10) + `,"pod":"` + pod + `"}`),
			Headers: headers,
		}
		partition, offset, err := kafkaProducer.SendMessage(msg)
		if err != nil {
			produceSpan.RecordError(err)
			log.Printf("kafka produce failed: %v", err)
		} else {
			produceSpan.SetAttributes(
				attribute.Int("messaging.kafka.partition", int(partition)),
				attribute.Int64("messaging.kafka.offset", offset),
			)
			log.Printf("kafka produced id=%d partition=%d offset=%d", next, partition, offset)
		}
		produceSpan.End()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(idResp{ID: next, Pod: pod})
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
	pod, _ := os.Hostname()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(versionResp{Version: version, Pod: pod})
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"pong":"ok"}`))
}

func loadValkeyPassword() string {
	if pwFile := os.Getenv("VALKEY_PASSWORD_FILE"); pwFile != "" {
		if data, err := os.ReadFile(pwFile); err == nil {
			log.Printf("valkey password loaded from file: %s", pwFile)
			return strings.TrimSpace(string(data))
		} else {
			log.Printf("VALKEY_PASSWORD_FILE 읽기 실패 (%s): %v — env로 폴백", pwFile, err)
		}
	}
	return os.Getenv("VALKEY_PASSWORD")
}

func connectValkey() valkey.Client {
	addr := os.Getenv("VALKEY_ADDR")
	if addr == "" {
		addr = "valkey-primary.notiflex.svc.cluster.local:6379"
	}
	opt := valkey.ClientOption{
		InitAddress: []string{addr},
		Password:    loadValkeyPassword(),
	}

	var client valkey.Client
	var err error
	for i := 0; i < 10; i++ {
		client, err = valkey.NewClient(opt)
		if err == nil {
			log.Printf("valkey connected: %s", addr)
			return client
		}
		log.Printf("Valkey 연결 재시도 %d/10: %v", i+1, err)
		time.Sleep(3 * time.Second)
	}
	log.Fatalf("valkey 연결 실패 (10회 재시도): %v", err)
	return nil
}

func kafkaConfig() *sarama.Config {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V4_3_0_0
	cfg.Producer.Return.Successes = true
	cfg.Producer.RequiredAcks = sarama.WaitForLocal
	cfg.Producer.Retry.Max = 5
	cfg.Consumer.Offsets.Initial = sarama.OffsetNewest
	cfg.ClientID = "notiflex-api"
	return cfg
}

func connectKafkaProducer(brokers []string) sarama.SyncProducer {
	cfg := kafkaConfig()
	var producer sarama.SyncProducer
	var err error
	for i := 0; i < 10; i++ {
		producer, err = sarama.NewSyncProducer(brokers, cfg)
		if err == nil {
			log.Printf("kafka producer connected: %v", brokers)
			return producer
		}
		log.Printf("Kafka producer 연결 재시도 %d/10: %v", i+1, err)
		time.Sleep(3 * time.Second)
	}
	log.Printf("kafka producer 연결 실패 (10회 재시도): %v — producer 없이 계속 실행", err)
	return nil
}

func startKafkaConsumer(ctx context.Context, brokers []string, groupID string) {
	cfg := kafkaConfig()
	go func() {
		var group sarama.ConsumerGroup
		var err error
		for i := 0; i < 10; i++ {
			group, err = sarama.NewConsumerGroup(brokers, groupID, cfg)
			if err == nil {
				break
			}
			log.Printf("Kafka consumer 연결 재시도 %d/10: %v", i+1, err)
			time.Sleep(3 * time.Second)
		}
		if err != nil {
			log.Printf("kafka consumer 연결 실패 (10회 재시도): %v", err)
			return
		}
		defer group.Close()

		handler := &notificationsHandler{}
		for {
			if err := group.Consume(ctx, []string{notificationsTopic}, handler); err != nil {
				log.Printf("kafka consume error: %v", err)
				time.Sleep(2 * time.Second)
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()
}

type notificationsHandler struct{}

func (h *notificationsHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (h *notificationsHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }
func (h *notificationsHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		ctx := extractTraceHeaders(context.Background(), msg.Headers)
		_, span := tracer.Start(ctx, "kafka.consume",
			trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(
				attribute.String("messaging.system", "kafka"),
				attribute.String("messaging.source.name", msg.Topic),
				attribute.Int("messaging.kafka.partition", int(msg.Partition)),
				attribute.Int64("messaging.kafka.offset", msg.Offset),
			))
		log.Printf("kafka consumed topic=%s partition=%d offset=%d key=%s value=%s",
			msg.Topic, msg.Partition, msg.Offset, string(msg.Key), string(msg.Value))
		span.End()
		sess.MarkMessage(msg, "")
	}
	return nil
}

type kafkaHeaderCarrier []sarama.RecordHeader

func (c *kafkaHeaderCarrier) Get(key string) string {
	for _, h := range *c {
		if string(h.Key) == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c *kafkaHeaderCarrier) Set(key, value string) {
	for i, h := range *c {
		if string(h.Key) == key {
			(*c)[i].Value = []byte(value)
			return
		}
	}
	*c = append(*c, sarama.RecordHeader{Key: []byte(key), Value: []byte(value)})
}

func (c *kafkaHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(*c))
	for _, h := range *c {
		keys = append(keys, string(h.Key))
	}
	return keys
}

func injectTraceHeaders(ctx context.Context) []sarama.RecordHeader {
	carrier := kafkaHeaderCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, &carrier)
	return []sarama.RecordHeader(carrier)
}

func extractTraceHeaders(ctx context.Context, headers []*sarama.RecordHeader) context.Context {
	carrier := make(kafkaHeaderCarrier, 0, len(headers))
	for _, h := range headers {
		if h == nil {
			continue
		}
		carrier = append(carrier, sarama.RecordHeader{Key: h.Key, Value: h.Value})
	}
	return otel.GetTextMapPropagator().Extract(ctx, &carrier)
}

func initTracer(ctx context.Context) (func(context.Context) error, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		log.Printf("OTEL_EXPORTER_OTLP_ENDPOINT 미설정 — 트레이싱 비활성 모드")
		return func(context.Context) error { return nil }, nil
	}
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	exp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, err
	}
	res, err := sdkresource.New(ctx,
		sdkresource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(version),
		),
	)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	tracer = tp.Tracer(serviceName)
	log.Printf("OTel tracer initialized: exporter=%s", endpoint)
	return tp.Shutdown, nil
}

func main() {
	ctx := context.Background()

	shutdownTracer, err := initTracer(ctx)
	if err != nil {
		log.Printf("OTel 초기화 실패 (트레이싱 없이 계속 실행): %v", err)
		shutdownTracer = func(context.Context) error { return nil }
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracer(shutdownCtx)
	}()

	valkeyClient = connectValkey()
	defer valkeyClient.Close()

	if broker := os.Getenv("KAFKA_BROKER"); broker != "" {
		brokers := strings.Split(broker, ",")
		kafkaProducer = connectKafkaProducer(brokers)
		if kafkaProducer != nil {
			defer kafkaProducer.Close()
		}
		startKafkaConsumer(context.Background(), brokers, "notiflex-api")
	} else {
		log.Printf("KAFKA_BROKER 미설정 — Kafka 비활성 모드로 실행")
	}

	mux := http.NewServeMux()
	mux.Handle("/health", otelhttp.NewHandler(http.HandlerFunc(healthHandler), "GET /health"))
	mux.Handle("/id", otelhttp.NewHandler(http.HandlerFunc(idHandler), "GET /id"))
	mux.Handle("/version", otelhttp.NewHandler(http.HandlerFunc(versionHandler), "GET /version"))
	mux.Handle("/ping", otelhttp.NewHandler(http.HandlerFunc(pingHandler), "GET /ping"))

	addr := ":8080"
	log.Printf("notiflex-api %s listening on %s", version, addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
