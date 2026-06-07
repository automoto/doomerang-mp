package core

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// newDrainTestServer builds the minimum Server needed to exercise Drain:
// an atomic match-in-progress flag, a stop-able loop, and the drain
// bookkeeping fields. NewServer is avoided because it requires real
// levels and starts background goroutines.
func newDrainTestServer(timeout time.Duration) *Server {
	return &Server{
		loop:         &GameLoop{stopChan: make(chan struct{})},
		drainDone:    make(chan struct{}),
		drainTimeout: timeout,
	}
}

func TestServerDrain(t *testing.T) {
	tests := []struct {
		name             string
		matchInProgress  bool
		duringDrain      func(s *Server)
		drainTimeout     time.Duration
		minDrainDuration time.Duration
		maxDrainDuration time.Duration
	}{
		{
			name:             "idle server drains immediately",
			matchInProgress:  false,
			drainTimeout:     5 * time.Second,
			maxDrainDuration: 300 * time.Millisecond,
		},
		{
			name:            "waits for active match to complete",
			matchInProgress: true,
			duringDrain: func(s *Server) {
				time.Sleep(80 * time.Millisecond)
				s.matchInProgress.Store(false)
			},
			drainTimeout:     5 * time.Second,
			minDrainDuration: 60 * time.Millisecond,
			maxDrainDuration: 500 * time.Millisecond,
		},
		{
			name:             "stops anyway when drain timeout exceeded",
			matchInProgress:  true, // never cleared
			drainTimeout:     80 * time.Millisecond,
			minDrainDuration: 70 * time.Millisecond,
			maxDrainDuration: 400 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newDrainTestServer(tt.drainTimeout)
			s.matchInProgress.Store(tt.matchInProgress)

			done := make(chan struct{})
			start := time.Now()
			go func() {
				s.Drain()
				close(done)
			}()
			if tt.duringDrain != nil {
				go tt.duringDrain(s)
			}

			select {
			case <-done:
			case <-time.After(tt.maxDrainDuration + time.Second):
				t.Fatalf("Drain did not return within %v", tt.maxDrainDuration+time.Second)
			}
			elapsed := time.Since(start)

			assert.True(t, s.draining.Load(), "draining flag must be set after Drain")
			if tt.minDrainDuration > 0 {
				assert.GreaterOrEqualf(t, elapsed, tt.minDrainDuration,
					"Drain returned too early (%v < %v)", elapsed, tt.minDrainDuration)
			}
			assert.LessOrEqualf(t, elapsed, tt.maxDrainDuration,
				"Drain took too long (%v > %v)", elapsed, tt.maxDrainDuration)
			assertChannelClosed(t, s.loop.stopChan, "loop.stopChan")
		})
	}
}

func TestServerDrain_concurrent_calls_are_idempotent(t *testing.T) {
	s := newDrainTestServer(5 * time.Second)

	const callers = 4
	var wg sync.WaitGroup
	wg.Add(callers)
	for i := 0; i < callers; i++ {
		go func() {
			defer wg.Done()
			s.Drain() // must not panic on close-of-closed-channel
		}()
	}

	allDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(allDone)
	}()
	select {
	case <-allDone:
	case <-time.After(time.Second):
		t.Fatal("concurrent Drain calls did not all return within 1s")
	}

	assert.True(t, s.draining.Load())
	assertChannelClosed(t, s.loop.stopChan, "loop.stopChan")
}

func assertChannelClosed(t *testing.T, ch <-chan struct{}, name string) {
	t.Helper()
	select {
	case _, ok := <-ch:
		assert.Falsef(t, ok, "%s should be closed (received open-side value)", name)
	default:
		t.Fatalf("%s is not closed", name)
	}
}
