package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Registration handles registering and heartbeating with the master server.
type Registration struct {
	masterURL  string
	serverID   string
	name       string
	address    string
	version    string
	region     string
	maxPlayers int
	server     *Server
	client     *http.Client
	stopCh     chan struct{}
}

type regRequest struct {
	Name       string `json:"name"`
	Address    string `json:"address"`
	Players    int    `json:"players"`
	MaxPlayers int    `json:"maxPlayers"`
	Version    string `json:"version"`
	Region     string `json:"region"`
}

type regResponse struct {
	ID string `json:"id"`
}

type heartbeatRequest struct {
	ID      string `json:"id"`
	Players int    `json:"players"`
}

func NewRegistration(masterURL, name, address, version, region string, maxPlayers int, server *Server) *Registration {
	return &Registration{
		masterURL:  masterURL,
		name:       name,
		address:    address,
		version:    version,
		region:     region,
		maxPlayers: maxPlayers,
		server:     server,
		client:     &http.Client{Timeout: 5 * time.Second},
		stopCh:     make(chan struct{}),
	}
}

func (r *Registration) Start() {
	if err := r.register(); err != nil {
		log.Printf("[registration] initial registration failed: %v", err)
	}
	go r.heartbeatLoop()
}

func (r *Registration) Stop() {
	close(r.stopCh)
}

func (r *Registration) register() error {
	body, err := json.Marshal(regRequest{
		Name:       r.name,
		Address:    r.address,
		Players:    r.server.PlayerCount(),
		MaxPlayers: r.maxPlayers,
		Version:    r.version,
		Region:     r.region,
	})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	resp, err := r.client.Post(r.masterURL+"/servers/register", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result regResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	r.serverID = result.ID
	log.Printf("[registration] registered with master (id=%s)", r.serverID)
	return nil
}

func (r *Registration) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			if err := r.sendHeartbeat(); err != nil {
				log.Printf("[registration] heartbeat failed: %v", err)
			}
		}
	}
}

func (r *Registration) sendHeartbeat() error {
	body, err := json.Marshal(heartbeatRequest{
		ID:      r.serverID,
		Players: r.server.PlayerCount(),
	})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	resp, err := r.client.Post(r.masterURL+"/servers/heartbeat", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		log.Println("[registration] master lost our registration, re-registering")
		return r.register()
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}
