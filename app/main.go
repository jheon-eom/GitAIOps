package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync/atomic"
)

var counter atomic.Uint64

type healthResp struct {
	Status string `json:"status"`
}

type idResp struct {
	ID  uint64 `json:"id"`
	Pod string `json:"pod"`
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

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/id", idHandler)

	addr := ":8080"
	log.Printf("notiflex-api listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
