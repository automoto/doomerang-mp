package main

import (
	"bytes"
	"context"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pkgsdk "agones.dev/agones/pkg/sdk"
	sdk "agones.dev/agones/sdks/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// logBuffer is a goroutine-safe wrapper around bytes.Buffer used to
// capture log output while the healthLoop writes from another goroutine.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// captureLog redirects the default logger to dst and returns a restore
// closure. Tests must call the closure (typically via defer) so the
// global logger is reset before the next test.
func captureLog(dst *logBuffer) func() {
	prevOut := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(dst)
	log.SetFlags(0)
	return func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	}
}

// waitFor polls cond every 5 ms until it returns true or the timeout
// elapses. Used by health-loop tests instead of fixed sleeps so the
// suite stays fast on a healthy machine and tolerant on a loaded CI.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for: %s", what)
}

// fakeAgonesSDK satisfies the agonesSDK interface that agonesLifecycle
// depends on. It records call counts and replays Watch updates fed in by
// the test via emit().
type fakeAgonesSDK struct {
	mu sync.Mutex

	readyErr      error
	readyCalls    int
	healthErr     error
	healthCalls   int
	shutdownErr   error
	shutdownCalls int
	watchErr      error
	watchCb       sdk.GameServerCallback
}

func (f *fakeAgonesSDK) Ready() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.readyCalls++
	return f.readyErr
}

func (f *fakeAgonesSDK) Health() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.healthCalls++
	return f.healthErr
}

func (f *fakeAgonesSDK) Shutdown() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shutdownCalls++
	return f.shutdownErr
}

func (f *fakeAgonesSDK) WatchGameServer(cb sdk.GameServerCallback) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.watchCb = cb
	return f.watchErr
}

func (f *fakeAgonesSDK) emit(state string) {
	f.mu.Lock()
	cb := f.watchCb
	f.mu.Unlock()
	if cb == nil {
		return
	}
	cb(&pkgsdk.GameServer{Status: &pkgsdk.GameServer_Status{State: state}})
}

func (f *fakeAgonesSDK) readyCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.readyCalls
}

func (f *fakeAgonesSDK) healthCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.healthCalls
}

func (f *fakeAgonesSDK) shutdownCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.shutdownCalls
}

func TestNewAgonesLifecycle_returns_nil_when_AGONES_SDK_GRPC_PORT_unset(t *testing.T) {
	t.Setenv("AGONES_SDK_GRPC_PORT", "")
	lc, err := newAgonesLifecycle(func() {})
	require.NoError(t, err)
	assert.Nil(t, lc, "lifecycle must be nil so caller can skip Agones path")
}

func TestAgonesLifecycle_registers_watch_before_Ready(t *testing.T) {
	defer goleak.VerifyNone(t)

	f := &fakeAgonesSDK{}
	// blockingReadySDK gates Ready on a channel so we can observe what
	// the lifecycle called before Ready completed.
	gate := make(chan struct{})
	lc := newAgonesLifecycleWith(&blockingReadySDK{fakeAgonesSDK: f, gate: gate}, 10*time.Millisecond, func() {})

	startDone := make(chan error, 1)
	go func() { startDone <- lc.Start(context.Background()) }()

	// The watch callback must be installed before Ready returns,
	// otherwise a Shutdown transition during the handshake could slip
	// through the gap.
	assert.Eventually(t, func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.watchCb != nil
	}, 200*time.Millisecond, 5*time.Millisecond, "WatchGameServer must run before Ready")

	close(gate)
	require.NoError(t, <-startDone)
	defer lc.Stop()

	assert.Equal(t, 1, f.readyCount())
}

// blockingReadySDK wraps fakeAgonesSDK and gates Ready on a channel so
// the test can observe Start's ordering before Ready completes.
type blockingReadySDK struct {
	*fakeAgonesSDK
	gate <-chan struct{}
}

func (b *blockingReadySDK) Ready() error {
	<-b.gate
	return b.fakeAgonesSDK.Ready()
}

func TestAgonesLifecycle_health_failures_escalate_to_error_log_once(t *testing.T) {
	defer goleak.VerifyNone(t)

	var buf logBuffer
	restore := captureLog(&buf)
	defer restore()

	f := &fakeAgonesSDK{healthErr: readyFailure("sidecar gRPC EOF")}
	lc := newAgonesLifecycleWith(f, 5*time.Millisecond, func() {})
	require.NoError(t, lc.Start(context.Background()))
	defer lc.Stop()

	// Wait for at least healthEscalateAfter + a couple more ticks so we
	// can also assert the escalation log fires only ONCE.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if f.healthCount() >= healthEscalateAfter+3 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	logs := buf.String()
	assert.Contains(t, logs, "ERROR:", "must escalate after sustained Health failures")
	count := strings.Count(logs, "consecutive Health() failures")
	assert.Equal(t, 1, count, "escalation log must fire exactly once, not on every tick; got %d", count)
}

func TestAgonesLifecycle_health_recovery_resets_escalation(t *testing.T) {
	defer goleak.VerifyNone(t)

	var buf logBuffer
	restore := captureLog(&buf)
	defer restore()

	// Start failing; after escalation, recover; then fail again — second
	// escalation should fire (state reset on recovery).
	f := &fakeAgonesSDK{healthErr: readyFailure("transient")}
	lc := newAgonesLifecycleWith(f, 5*time.Millisecond, func() {})
	require.NoError(t, lc.Start(context.Background()))
	defer lc.Stop()

	waitFor(t, 300*time.Millisecond, func() bool {
		return strings.Count(buf.String(), "consecutive Health() failures") >= 1
	}, "first escalation")

	// Recover.
	f.mu.Lock()
	f.healthErr = nil
	f.mu.Unlock()
	waitFor(t, 300*time.Millisecond, func() bool {
		return strings.Contains(buf.String(), "recovered after")
	}, "recovery log")

	// Fail again.
	f.mu.Lock()
	f.healthErr = readyFailure("again")
	f.mu.Unlock()
	waitFor(t, 300*time.Millisecond, func() bool {
		return strings.Count(buf.String(), "consecutive Health() failures") >= 2
	}, "second escalation after recovery")
}

func TestAgonesLifecycle_calls_Health_on_heartbeat_tick(t *testing.T) {
	defer goleak.VerifyNone(t)

	f := &fakeAgonesSDK{}
	lc := newAgonesLifecycleWith(f, 10*time.Millisecond, func() {})
	require.NoError(t, lc.Start(context.Background()))
	defer lc.Stop()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if f.healthCount() >= 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	assert.GreaterOrEqualf(t, f.healthCount(), 3,
		"expected at least 3 Health calls in ~500ms at 10ms tick, got %d", f.healthCount())
}

func TestAgonesLifecycle_drains_exactly_once_when_state_becomes_Shutdown(t *testing.T) {
	defer goleak.VerifyNone(t)

	f := &fakeAgonesSDK{}
	var drainCalls atomic.Int32
	drainFired := make(chan struct{}, 1)
	lc := newAgonesLifecycleWith(f, 10*time.Millisecond, func() {
		drainCalls.Add(1)
		select {
		case drainFired <- struct{}{}:
		default:
		}
	})
	require.NoError(t, lc.Start(context.Background()))
	defer lc.Stop()

	f.emit("Ready")
	f.emit("Allocated")
	// Give any spurious drain goroutine a chance to run before asserting zero.
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, int32(0), drainCalls.Load(), "non-Shutdown states must not trigger drain")

	f.emit("Shutdown")
	f.emit("Shutdown") // duplicate updates are common; must be idempotent

	select {
	case <-drainFired:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("drain callback did not fire within 200ms of Shutdown emission")
	}
	// Wait briefly for any second goroutine to (incorrectly) increment.
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, int32(1), drainCalls.Load(), "drain must fire exactly once across multiple Shutdown emissions")
}

func TestAgonesLifecycle_drain_does_not_block_watch_callback(t *testing.T) {
	defer goleak.VerifyNone(t)

	f := &fakeAgonesSDK{}
	drainHold := make(chan struct{})
	drainEntered := make(chan struct{}, 1)
	lc := newAgonesLifecycleWith(f, 10*time.Millisecond, func() {
		drainEntered <- struct{}{}
		<-drainHold // simulate Drain blocking on an active match
	})
	require.NoError(t, lc.Start(context.Background()))

	// emit must return promptly even though drain is blocking — i.e. drain
	// ran in its own goroutine, not on the watch callback goroutine.
	emitDone := make(chan struct{})
	go func() {
		f.emit("Shutdown")
		close(emitDone)
	}()
	select {
	case <-emitDone:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("emit blocked — handleStateChange must run drain in its own goroutine")
	}

	<-drainEntered
	close(drainHold)
	lc.Stop()
}

func TestAgonesLifecycle_Stop_returns_cleanly_without_goroutine_leak(t *testing.T) {
	defer goleak.VerifyNone(t)

	f := &fakeAgonesSDK{}
	lc := newAgonesLifecycleWith(f, 5*time.Millisecond, func() {})
	require.NoError(t, lc.Start(context.Background()))

	done := make(chan struct{})
	go func() {
		lc.Stop()
		lc.Stop() // second Stop must be safe
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Stop did not return within 200ms")
	}
}

func TestAgonesLifecycle_Stop_calls_sdk_Shutdown(t *testing.T) {
	defer goleak.VerifyNone(t)

	f := &fakeAgonesSDK{}
	lc := newAgonesLifecycleWith(f, 5*time.Millisecond, func() {})
	require.NoError(t, lc.Start(context.Background()))
	lc.Stop()

	assert.Equal(t, 1, f.shutdownCount(), "Stop must notify Agones via sdk.Shutdown")
}

func TestAgonesLifecycle_Stop_is_safe_without_Start(t *testing.T) {
	defer goleak.VerifyNone(t)

	f := &fakeAgonesSDK{}
	lc := newAgonesLifecycleWith(f, 5*time.Millisecond, func() {})

	done := make(chan struct{})
	go func() {
		lc.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Stop hung when Start was never called — started flag is not protecting <-healthDone")
	}
	assert.Equal(t, 1, f.shutdownCount(), "Stop must still call sdk.Shutdown even when Start was never called")
}

func TestAgonesLifecycle_Start_is_idempotent(t *testing.T) {
	defer goleak.VerifyNone(t)

	f := &fakeAgonesSDK{}
	lc := newAgonesLifecycleWith(f, 10*time.Millisecond, func() {})

	require.NoError(t, lc.Start(context.Background()))
	require.NoError(t, lc.Start(context.Background())) // second Start must not spawn a second health goroutine
	defer lc.Stop()

	assert.Equal(t, 1, f.readyCount(), "Ready must be called exactly once across multiple Start calls")
}

func TestAgonesLifecycle_Start_returns_Ready_error_and_remains_safe_to_Stop(t *testing.T) {
	defer goleak.VerifyNone(t)

	f := &fakeAgonesSDK{readyErr: assertReadyError}
	lc := newAgonesLifecycleWith(f, 5*time.Millisecond, func() {})

	err := lc.Start(context.Background())
	require.Error(t, err)

	// Stop on a half-initialised lifecycle must not deadlock waiting on a
	// health loop that never started.
	done := make(chan struct{})
	go func() {
		lc.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Stop deadlocked after Start failed at Ready")
	}
}

// assertReadyError is a sentinel error injected into the fake's Ready
// path to force the Start-error branch.
var assertReadyError = readyFailure("ready failed")

type readyFailure string

func (e readyFailure) Error() string { return string(e) }
