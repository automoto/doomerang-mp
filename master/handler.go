package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type registerRequest struct {
	Name       string `json:"name"`
	Address    string `json:"address"`
	Players    int    `json:"players"`
	MaxPlayers int    `json:"maxPlayers"`
	Version    string `json:"version"`
	Region     string `json:"region"`
}

type registerResponse struct {
	ID string `json:"id"`
}

type heartbeatRequest struct {
	ID      string `json:"id"`
	Players int    `json:"players"`
}

func ListServers(reg *Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		servers := reg.List()
		if err := json.NewEncoder(w).Encode(servers); err != nil {
			log.Printf("[master] list encode error: %v", err)
		}
	}
}

const maxRequestBody = 1 << 16 // 64 KB

func RegisterServer(reg *Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		var req registerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}
		if req.Name == "" || req.Address == "" {
			http.Error(w, `{"error":"name and address required"}`, http.StatusBadRequest)
			return
		}

		id := reg.Register(ServerInfo{
			Name:       req.Name,
			Address:    req.Address,
			Players:    req.Players,
			MaxPlayers: req.MaxPlayers,
			Version:    req.Version,
			Region:     req.Region,
		})

		log.Printf("[master] registered server %q at %s (id=%s)", req.Name, req.Address, id)

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(registerResponse{ID: id})
	}
}

func Heartbeat(reg *Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		var req heartbeatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}

		if !reg.Heartbeat(req.ID, req.Players) {
			http.Error(w, `{"error":"unknown server"}`, http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}

func Health() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}
