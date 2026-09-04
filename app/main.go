package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync/atomic"
)

const version = "v0.3.0"

var counter atomic.Uint64

type healthResp struct {
	Status string `json:"status"`
}

type idResp struct {
	ID  uint64 `json:"id"`
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
	next := counter.Add(1)
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

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/id", idHandler)
	mux.HandleFunc("/version", versionHandler)
	mux.HandleFunc("/ping", pingHandler)

	addr := ":8080"
	log.Printf("notiflex-api listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
