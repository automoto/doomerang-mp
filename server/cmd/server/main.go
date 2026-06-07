package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/automoto/doomerang-mp/server/core"
	"github.com/automoto/doomerang-mp/shared/protocol"
	"github.com/automoto/ggscale-go"
)

const (
	heartbeatInterval = 10 * time.Second
	registerTimeout   = 10 * time.Second
)

func main() {
	port := flag.Uint("port", 7373, "Server port")
	tickRate := flag.Int("tickrate", 60, "Server tick rate (updates per second)")
	name := flag.String("name", "Doomerang Server", "Server display name")
	version := flag.String("version", "", "Required client version (empty = accept any)")
	assetsDir := flag.String("assets", "assets", "Path to assets directory")
	region := flag.String("region", "", "Server region for display")
	maxPlayers := flag.Int("maxplayers", 4, "Maximum players")
	address := flag.String("address", "localhost:7373", "Public address to advertise")
	numBots := flag.Int("bots", 0, "Number of bots to spawn on startup")
	flag.Parse()

	// Arm the signal handler before any blocking init (ggscale Register,
	// Agones Ready/Watch) so a SIGTERM landing during startup gets caught
	// by the same cleanup path as one delivered mid-run. Buffered=1 so a
	// signal arriving before the goroutine starts isn't lost.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	if err := protocol.RegisterComponents(); err != nil {
		log.Fatalf("Failed to register components: %v", err)
	}

	levels, levelNames, err := core.LoadAllServerLevels(*assetsDir)
	if err != nil {
		log.Fatalf("Failed to load levels: %v", err)
	}
	log.Printf("Loaded %d levels: %v", len(levelNames), levelNames)

	server := core.NewServer(*tickRate, *name, *version, levels, levelNames)

	for i := 0; i < *numBots; i++ {
		server.SpawnBot(fmt.Sprintf("Bot %d", i+1), 1)
	}

	stopHeartbeat, deregister := startGgscaleRegistration(server, *name, *address, *version, *region, *maxPlayers)

	// Agones lifecycle. The drain callback forwards into sigChan so the
	// Agones-Shutdown path and the SIGTERM path run the SAME cleanup
	// goroutine — there is only one shutdown sequence, no duplicate
	// cleanup, no path that forgets to deregister.
	agones, err := newAgonesLifecycle(func() {
		log.Println("[agones] Shutdown state received; signalling shutdown")
		select {
		case sigChan <- syscall.SIGTERM:
		default: // shutdown already underway
		}
	})

	var shutdownOnce sync.Once
	shutdown := func() {
		shutdownOnce.Do(func() {
			log.Println("Shutting down server...")
			if stopHeartbeat != nil {
				stopHeartbeat()
			}
			if deregister != nil {
				deregister()
			}
			// Drain sets the draining flag immediately (rejects new
			// joins), waits for any in-flight match to end (bounded by
			// drainTimeout), then stops the game loop.
			server.Drain()
			if agones != nil {
				agones.Stop()
			}
		})
	}

	go func() {
		<-sigChan
		shutdown()
		os.Exit(0)
	}()

	if err != nil {
		log.Printf("[agones] init: %v", err)
		shutdown()
		os.Exit(1)
	}
	if agones != nil {
		if err := agones.Start(context.Background()); err != nil {
			log.Printf("[agones] start: %v", err)
			shutdown()
			os.Exit(1)
		}
		log.Println("[agones] lifecycle started (watch + Ready sent, health active)")
	}

	log.Printf("Starting Doomerang server %q on port %d (tick rate: %d/s, version: %s)",
		*name, *port, *tickRate, *version)
	if err := server.Start(*port); err != nil {
		log.Printf("server start: %v", err)
		shutdown()
		os.Exit(1)
	}
}

// startGgscaleRegistration registers this game-server with ggscale,
// runs a heartbeat ticker, and (when GGSCALE_LEADERBOARD_ID is set)
// installs a match-end hook on srv that submits each player's score
// via Leaderboards.SubmitFor using the secret-tier API key.
//
// When GGSCALE_URL or GGSCALE_SECRET_KEY is unset, returns nil functions
// and the server runs unregistered (useful for `make run-server`
// without a live ggscale stack). The secret-tier key is required for
// fleet writes and leaderboard submit on the new server policy.
func startGgscaleRegistration(srv *core.Server, name, address, version, region string, maxPlayers int) (stop, deregister func()) {
	baseURL := os.Getenv("GGSCALE_URL")
	apiKey, err := loadSecret("GGSCALE_SECRET_KEY")
	if err != nil {
		log.Fatalf("[ggscale] %v", err)
	}
	if baseURL == "" || apiKey == "" {
		log.Println("[ggscale] GGSCALE_URL or GGSCALE_SECRET_KEY unset; running without fleet registration")
		return nil, nil
	}

	gg, err := ggscale.NewClient(ggscale.Options{BaseURL: baseURL, APIKey: apiKey})
	if err != nil {
		log.Fatalf("[ggscale] client init: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), registerTimeout)
	defer cancel()
	id, err := gg.Fleet.Register(ctx, ggscale.FleetRegisterRequest{
		Name:       name,
		Address:    address,
		Version:    version,
		Region:     region,
		MaxPlayers: maxPlayers,
	})
	if err != nil {
		log.Fatalf("[ggscale] register: %v", err)
	}
	log.Printf("[ggscale] registered as id=%s, advertising %s", id, address)

	if lbStr := os.Getenv("GGSCALE_LEADERBOARD_ID"); lbStr != "" {
		lbID, err := strconv.ParseInt(lbStr, 10, 64)
		if err != nil {
			log.Fatalf("[ggscale] GGSCALE_LEADERBOARD_ID must be an integer: %v", err)
		}
		srv.SetMatchEndHook(buildSubmitScoresHook(gg, lbID))
		log.Printf("[ggscale] match-end submission to leaderboard=%d enabled", lbID)
	}

	stopCh := make(chan struct{})
	var once sync.Once
	go heartbeatLoop(gg, id, stopCh)

	return func() {
			once.Do(func() { close(stopCh) })
		}, func() {
			ctx, cancel := context.WithTimeout(context.Background(), registerTimeout)
			defer cancel()
			if err := gg.Fleet.Deregister(ctx, id); err != nil {
				log.Printf("[ggscale] deregister: %v", err)
			}
		}
}

// buildSubmitScoresHook returns a MatchEndHook that submits each
// player's final KO count to ggscale via Leaderboards.SubmitFor. Each
// submission carries the player's session token (captured at join
// time) and the server's own secret-tier API key.
func buildSubmitScoresHook(gg *ggscale.Client, leaderboardID int64) core.MatchEndHook {
	return func(scores map[uint32]int, tokens map[uint32]string) {
		for netID, score := range scores {
			tok, ok := tokens[netID]
			if !ok || tok == "" {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), registerTimeout)
			err := gg.Leaderboards.SubmitFor(ctx, tok, leaderboardID, int64(score))
			cancel()
			if err != nil {
				log.Printf("[ggscale] submit netID=%d: %v", netID, err)
				continue
			}
			log.Printf("[ggscale] submitted netID=%d score=%d", netID, score)
		}
	}
}

// loadSecret reads a secret from <name>_FILE if set, else from <name>.
// _FILE is the standard pattern for docker/k8s/Vault file-mounted secrets;
// it wins over the plain env var. Returns an error if _FILE is set but
// unreadable, so a misconfigured mount fails loud instead of silently
// falling through to an empty value.
func loadSecret(name string) (string, error) {
	if path := os.Getenv(name + "_FILE"); path != "" {
		data, err := os.ReadFile(path) //nolint:gosec // operator-supplied secret path is the documented contract
		if err != nil {
			return "", fmt.Errorf("read %s_FILE %q: %w", name, path, err)
		}
		return strings.TrimRight(string(data), " \t\r\n"), nil
	}
	return os.Getenv(name), nil
}

func heartbeatLoop(gg *ggscale.Client, id string, stop <-chan struct{}) {
	t := time.NewTicker(heartbeatInterval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(context.Background(), registerTimeout)
			err := gg.Fleet.Heartbeat(ctx, id)
			cancel()
			if err != nil {
				log.Printf("[ggscale] heartbeat: %v", err)
			}
		}
	}
}
