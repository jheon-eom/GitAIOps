package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/valkey-io/valkey-go"
)

const version = "v0.4.0"

var valkeyClient valkey.Client

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

func connectValkey() valkey.Client {
	addr := os.Getenv("VALKEY_ADDR")
	if addr == "" {
		addr = "valkey-primary.notiflex.svc.cluster.local:6379"
	}
	opt := valkey.ClientOption{
		InitAddress: []string{addr},
		Password:    os.Getenv("VALKEY_PASSWORD"),
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

func main() {
	valkeyClient = connectValkey()
	defer valkeyClient.Close()

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
