package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"sync"
	"time"
)

// ServerInfo describes a game server visible to clients.
type ServerInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Address    string `json:"address"`
	Players    int    `json:"players"`
	MaxPlayers int    `json:"maxPlayers"`
	Version    string `json:"version"`
	Region     string `json:"region"`
}

type serverRecord struct {
	ServerInfo
	LastSeen time.Time
}

// Registry is an in-memory store of active game servers with TTL-based expiry.
type Registry struct {
	mu      sync.RWMutex
	servers map[string]*serverRecord
	ttl     time.Duration
	stopCh  chan struct{}
}

func NewRegistry(ttl time.Duration) *Registry {
	r := &Registry{
		servers: make(map[string]*serverRecord),
		ttl:     ttl,
		stopCh:  make(chan struct{}),
	}
	go r.cleanupLoop()
	return r
}

func (r *Registry) Stop() {
	close(r.stopCh)
}

func (r *Registry) Register(info ServerInfo) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	id := fmt.Sprintf("%x", b)

	info.ID = id

	r.mu.Lock()
	r.servers[id] = &serverRecord{
		ServerInfo: info,
		LastSeen:   time.Now(),
	}
	r.mu.Unlock()

	return id
}

func (r *Registry) Heartbeat(id string, players int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	rec, ok := r.servers[id]
	if !ok {
		return false
	}
	rec.LastSeen = time.Now()
	rec.Players = players
	return true
}

func (r *Registry) List() []ServerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ServerInfo, 0, len(r.servers))
	for _, rec := range r.servers {
		result = append(result, rec.ServerInfo)
	}
	return result
}

func (r *Registry) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.mu.Lock()
			now := time.Now()
			for id, rec := range r.servers {
				if now.Sub(rec.LastSeen) >= r.ttl {
					log.Printf("[master] expired server %q (id=%s, last seen %s ago)",
						rec.Name, id, now.Sub(rec.LastSeen).Round(time.Second))
					delete(r.servers, id)
				}
			}
			r.mu.Unlock()
		}
	}
}
