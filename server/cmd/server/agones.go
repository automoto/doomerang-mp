package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	pkgsdk "agones.dev/agones/pkg/sdk"
	sdk "agones.dev/agones/sdks/go"
)

// agonesHealthInterval is well under the Fleet's health.periodSeconds (5 s)
// so a single dropped tick can't push the SDK sidecar over the
// failureThreshold and flip the GameServer to Unhealthy.
const agonesHealthInterval = 2 * time.Second

// healthEscalateAfter is the consecutive-Health()-failure count at
// which the lifecycle escalates from per-tick debug logs to a single
// ERROR log so operators see the sidecar is sustained-broken before
// Agones force-kills the pod. Chosen so ~6 s of failed pings (3 ticks
// at 2 s) trips the warning, leaving margin under the Fleet's default
// 5 s × 3 = 15 s grace.
const healthEscalateAfter = 3

// agonesSDK is the narrow surface of *sdk.SDK that agonesLifecycle uses;
// defining it as an interface lets tests swap in a fake without standing
// up the real gRPC sidecar.
type agonesSDK interface {
	Ready() error
	Health() error
	Shutdown() error
	WatchGameServer(sdk.GameServerCallback) error
}

// agonesLifecycle runs the Agones game-server-side protocol: register the
// state watcher, mark Ready, run a periodic Health heartbeat, and fire
// drain() exactly once when the GameServer state transitions to Shutdown.
type agonesLifecycle struct {
	sdk      agonesSDK
	interval time.Duration
	drain    func()

	startOnce sync.Once
	startErr  error
	started   atomic.Bool

	drainOnce sync.Once

	stopOnce   sync.Once
	stopCh     chan struct{}
	healthDone chan struct{}
}

// newAgonesLifecycle returns a lifecycle wired to the real Agones SDK
// when AGONES_SDK_GRPC_PORT is set (the sidecar always sets it). When
// the env var is missing — server running outside Agones, e.g. local
// docker — it returns (nil, nil) so the caller can skip the Agones path.
func newAgonesLifecycle(drain func()) (*agonesLifecycle, error) {
	if os.Getenv("AGONES_SDK_GRPC_PORT") == "" {
		return nil, nil
	}
	s, err := sdk.NewSDK()
	if err != nil {
		return nil, fmt.Errorf("agones sdk init: %w", err)
	}
	return newAgonesLifecycleWith(s, agonesHealthInterval, drain), nil
}

func newAgonesLifecycleWith(s agonesSDK, interval time.Duration, drain func()) *agonesLifecycle {
	return &agonesLifecycle{
		sdk:        s,
		interval:   interval,
		drain:      drain,
		stopCh:     make(chan struct{}),
		healthDone: make(chan struct{}),
	}
}

// Start registers the GameServer watcher, then marks the GameServer
// Ready, then launches the Health heartbeat. The watcher is attached
// BEFORE Ready so a Shutdown transition during the handshake can't slip
// through the gap. Start is idempotent — second and subsequent calls
// return the same error as the first.
func (a *agonesLifecycle) Start(_ context.Context) error {
	a.startOnce.Do(func() {
		if err := a.sdk.WatchGameServer(a.handleStateChange); err != nil {
			a.startErr = fmt.Errorf("agones watch: %w", err)
			return
		}
		if err := a.sdk.Ready(); err != nil {
			a.startErr = fmt.Errorf("agones ready: %w", err)
			return
		}
		a.started.Store(true)
		go a.healthLoop()
	})
	return a.startErr
}

func (a *agonesLifecycle) handleStateChange(gs *pkgsdk.GameServer) {
	if gs == nil || gs.GetStatus() == nil {
		return
	}
	if gs.GetStatus().GetState() != "Shutdown" {
		return
	}
	a.drainOnce.Do(func() {
		// Run drain in a goroutine so we don't block the Agones SDK's
		// watch callback for the full drainTimeout — that would queue
		// every subsequent state update behind a 30 s wait.
		go a.drain()
	})
}

func (a *agonesLifecycle) healthLoop() {
	defer close(a.healthDone)
	t := time.NewTicker(a.interval)
	defer t.Stop()
	consecutiveFailures := 0
	escalated := false
	for {
		select {
		case <-a.stopCh:
			return
		case <-t.C:
			err := a.sdk.Health()
			if err == nil {
				if escalated {
					log.Printf("[agones] health: recovered after %d consecutive failures", consecutiveFailures)
				}
				consecutiveFailures = 0
				escalated = false
				continue
			}
			log.Printf("[agones] health: %v", err)
			consecutiveFailures++
			if consecutiveFailures >= healthEscalateAfter && !escalated {
				log.Printf("[agones] ERROR: %d consecutive Health() failures — Agones will mark this GameServer Unhealthy soon",
					consecutiveFailures)
				escalated = true
			}
		}
	}
}

// Stop tells the Agones controller this game server is exiting
// (sdk.Shutdown), then signals the health loop to exit and waits for it
// to drain. Safe to call multiple times and safe to call without a
// preceding Start — the started flag prevents the wait from deadlocking
// when no health goroutine was ever launched.
func (a *agonesLifecycle) Stop() {
	if err := a.sdk.Shutdown(); err != nil {
		log.Printf("[agones] shutdown: %v", err)
	}
	a.stopOnce.Do(func() { close(a.stopCh) })
	if a.started.Load() {
		<-a.healthDone
	}
}
