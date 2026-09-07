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
)

const version = "v0.7.0"

var (
	valkeyClient  valkey.Client
	kafkaProducer sarama.SyncProducer
)

const notificationsTopic = "notifications"

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

	next, err := valkeyClient.Do(ctx, valkeyClient.B().Incr().Key("notiflex:id").Build()).AsInt64()
	if err != nil {
		log.Printf("valkey INCR failed: %v", err)
		http.Error(w, "id backend unavailable", http.StatusServiceUnavailable)
		return
	}
	pod, _ := os.Hostname()

	if kafkaProducer != nil {
		msg := &sarama.ProducerMessage{
			Topic: notificationsTopic,
			Key:   sarama.StringEncoder(strconv.FormatInt(next, 10)),
			Value: sarama.StringEncoder(`{"id":` + strconv.FormatInt(next, 10) + `,"pod":"` + pod + `"}`),
		}
		if partition, offset, err := kafkaProducer.SendMessage(msg); err != nil {
			log.Printf("kafka produce failed: %v", err)
		} else {
			log.Printf("kafka produced id=%d partition=%d offset=%d", next, partition, offset)
		}
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
		log.Printf("kafka consumed topic=%s partition=%d offset=%d key=%s value=%s",
			msg.Topic, msg.Partition, msg.Offset, string(msg.Key), string(msg.Value))
		sess.MarkMessage(msg, "")
	}
	return nil
}

func main() {
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
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/id", idHandler)
	mux.HandleFunc("/version", versionHandler)
	mux.HandleFunc("/ping", pingHandler)

	addr := ":8080"
	log.Printf("notiflex-api %s listening on %s", version, addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
