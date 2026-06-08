# Doomerang Game-Server Architecture

How `doomerang-server` fits between the player, ggscale, and Agones. Read
this before touching anything in `server/cmd/server/` or
`server/core/server.go`.

---

## TL;DR

`doomerang-server` is the dedicated, authoritative game-server process.
It accepts player connections over WebSocket, runs the 60 Hz simulation
(physics, combat, scoring), and broadcasts state to all connected
clients. **Nothing else runs the game.** Agones orchestrates the pods;
ggscale runs the matchmaker and player directory; the game itself
happens inside `doomerang-server`.

Same binary, two modes:

| Mode | Where | Detected by | Lifecycle driver |
|---|---|---|---|
| Local / docker | dev laptop, docker-compose | `AGONES_SDK_GRPC_PORT` unset | SIGTERM only |
| Agones-managed | k3s + Agones in a real cluster | sidecar sets `AGONES_SDK_GRPC_PORT` | Agones state watcher + SIGTERM, both routed through `Server.Drain()` |

---

## Roles

Three independent control-plane services plus the game-server binary,
each doing one thing:

| Component | What it does | What it does NOT do |
|---|---|---|
| **Agones controller** (`agones-system` namespace) | Schedules GameServer pods, marks them Ready/Allocated/Shutdown, exposes the GameServerAllocator API. | Run any game logic. Touch player packets. Track scores. |
| **Agones SDK sidecar** (injected into each pod) | Carries the gRPC lifecycle protocol between this pod's `doomerang-server` and the controller (`Ready`, `Health`, `WatchGameServer`, `Shutdown`). | Forward gameplay traffic. The sidecar is on `localhost:$AGONES_SDK_GRPC_PORT` and only speaks the Agones SDK protocol. |
| **ggscale** | Authenticates players, runs the matchmaker, calls the Agones allocator on each ticket, returns the allocated pod's `address:port` to the client, owns leaderboards. | Stand between the client and gameplay. After the matchmaker handoff, ggscale is out of the realtime path. |
| **doomerang-server** | Accepts WS player connections on `:7373`, runs the authoritative game loop, broadcasts state, fires leaderboard scores at match end. | Decide who can play (ggscale gates that via session tokens). Manage its own pod lifecycle (Agones owns that). |

The split is deliberate: each service has one concern, and gameplay
latency is not gated by either control plane.

---

## Data path

```
                       ┌─────────────────────────────┐
                       │   Agones controller         │
                       │   (cluster-wide)            │
                       └──────┬──────────┬───────────┘
                              │ allocate │ Ready/Health/Shutdown
                              │          │  (gRPC, sidecar)
                              ▼          │
┌─────────┐  ticket (HTTPS)  ┌──────────┐│        ┌──────────────────┐
│  client │ ──────────────►  │ ggscale  │└──────► │  GameServer pod  │
│ (Go +   │                  │ matchmkr │         │  ┌────────────┐  │
│  ebiten)│ ◄──────────────  │          │ ◄────── │  │ doomerang- │  │
└────┬────┘  match_address   └──────────┘  addr   │  │  server    │  │
     │                                            │  └─────┬──────┘  │
     │                                            │        │         │
     │                                            │  ┌─────▼──────┐  │
     │       gameplay (WebSocket, 60 Hz)          │  │ Agones SDK │  │
     └────────────────────────────────────────────┼─►│ sidecar    │  │
                                                  │  └────────────┘  │
                                                  └──────────────────┘
```

`ggscale` and Agones touch the **control plane**: find a slot, return
its address. Once the client has `match_address`, every subsequent
packet is client ↔ `doomerang-server` directly. The matchmaker and the
Agones controller never see a gameplay packet.

---

## Why a dedicated server, instead of …

| Pattern | Why we don't | Where it works |
|---|---|---|
| **Peer-to-peer, no dedicated server** | NAT-traversal is hard on consumer connections; one client is always authoritative which exposes you to cheats; latency varies wildly per pair. | Slow co-op indies where one player explicitly hosts and the others trust them. |
| **Listen-server** (one player's machine is authoritative) | Same cheat/host-advantage problems as P2P plus the host pays asymmetric CPU/bandwidth. | Minecraft-style games where the player community accepts those tradeoffs. |
| **Serverless / FaaS** (function-per-input) | A 60 Hz tick of a small match would burn ~3.6 k invocations/minute per match, with cold-start jitter. Function billing dwarfs a VM. | Turn-based games (chess, card games) where a tick is "a player moved". |
| **"Just talk to Agones directly"** | Agones is a Kubernetes operator. It schedules pods and runs an allocator API; it doesn't have a game loop, doesn't understand your protocol, and its sidecar speaks gRPC for lifecycle only. | Doesn't exist as a real pattern. |

Dedicated server is the default for action games for the same reasons
it's been the default since Quake.

---

## Transport

doomerang-server uses **WebSocket** (TCP). The transport choice is set
in `server/core/server.go`:

```go
s.transport = transports.NewWsServerTransport(port, "", nil)
```

WebSocket is TCP, which means doomerang inherits TCP head-of-line
blocking: if a segment is lost, every subsequent segment waits for the
retransmit. At 60 Hz that can be 3–12 ticks of stutter for everyone
behind the lost packet. For a 2–4 player game with short matches on
typical home connections this is rarely noticeable; for a 32-player
shooter on bad mobile it would be.

We pick WebSocket anyway because **browser playability is a hard
requirement**. Browsers can't open raw UDP sockets — the only
alternatives that work in a browser tab are WebSocket (TCP) and
WebRTC DataChannels (UDP-like, but heavy: needs TURN/ICE/SDP
machinery). For v1, WebSocket is the right trade.

This is doomerang's choice. **It is not a platform constraint.** The
ggscale BaaS supports both transports as a first-class concept:
each game ships its own Fleet manifest declaring its protocol, and
the matchmaker surfaces it back to the client via `protocol_hint`.
A future native-only PC/Xbox game on this platform would use
`protocol: UDP` and a raw-UDP server binary with no changes to
ggscale.

If at some future point we want UDP-quality feel for native doomerang
players (Steam release, ranked mode), the upgrade path is **dual
transport** — one pod listening on both ports (`protocol: TCPUDP`),
WebSocket for browser players, UDP for native. The `netcomponents`
abstraction is already in place; adding a second listener is mostly
plumbing. We won't pull that lever until measurement says we should.

See `docs/fleet-onboarding.md` in the ggscale repo for the platform
view of transport choice across games.

---

## Same binary, two modes

`doomerang-server` doesn't care whether it's running under Agones or in
a local docker-compose. The detection is one env var:

```go
// server/cmd/server/agones.go
if os.Getenv("AGONES_SDK_GRPC_PORT") == "" {
    return nil, nil // not under Agones; caller skips the SDK path
}
```

The Agones sidecar always sets `AGONES_SDK_GRPC_PORT` when it injects
itself into a pod. Absent that, the binary still runs end-to-end: it
binds `:7373`, accepts players, runs the loop, and exits on SIGTERM.

Independently, the ggscale fleet registration (`Register`/`Heartbeat`/
`Deregister`) runs in both modes when `GGSCALE_URL` and
`GGSCALE_SECRET_KEY` are set. The two integrations are orthogonal — you
can be under ggscale and not under Agones (local docker demo), under
both (production), or neither (a quick `make run-server` for dev).

| Env / flag | Effect |
|---|---|
| `AGONES_SDK_GRPC_PORT` (set by sidecar) | Enables the Agones SDK lifecycle: `Ready`, 2 s `Health` heartbeat, `WatchGameServer` for the `Shutdown` state. |
| `GGSCALE_URL` + `GGSCALE_SECRET_KEY[_FILE]` | Enables ggscale fleet registration + heartbeat + leaderboard submission. |
| `GGSCALE_LEADERBOARD_ID` | Enables match-end score submission to that leaderboard. |
| `--bots N` | Spawns N bots on startup; useful for solo dev runs. |

---

## A player joining a match, step by step

What happens between the player clicking "Find match" and a UDP/WS
packet round-tripping to `doomerang-server`:

1. **Client → ggscale**: `POST /v1/matchmaker/tickets` with the
   fleet name (`doomerang`), region, and the player's session token.
2. **ggscale matchmaker**: validates the session, picks a fleet,
   enqueues a ticket, then asks the agones backend to allocate.
3. **agones backend** (`internal/fleet/agones/backend.go`): builds a
   `GameServerAllocation` CR with `agones.dev/fleet=doomerang` and
   (if requested) `ggscale.region=<region>` selectors, submits it,
   reads back the synchronously-allocated GameServer's
   `Status.Address:Status.Ports[0]`.
4. **ggscale ticket response**: `match_address: <nodeIP>:<port>`.
5. **Client connects directly**: dials that address over WebSocket;
   sends a `JoinRequest` carrying the session token.
6. **doomerang-server**: rejects if the server is `draining` (Phase C),
   otherwise spawns the player entity, joins the active match,
   responds with `JoinAccepted`.
7. **Game loop** (60 Hz): each tick processes queued inputs, runs
   physics + combat, broadcasts deltas to all connected players.

ggscale isn't involved after step 5; the Agones controller isn't
involved after step 3.

---

## Shutdown and drain

Two events can initiate shutdown. Both converge on the same
`shutdown()` function in `main.go`:

| Trigger | Source | Wire |
|---|---|---|
| `SIGTERM` / `SIGINT` | kubelet (pod termination), operator (`kubectl delete`), local Ctrl-C | `signal.Notify` → `sigChan` |
| Agones `Shutdown` state | Agones controller (Fleet downscale, `gameserver` delete, scale-down) | `WatchGameServer` callback → `sigChan <- SIGTERM` |

The drain sequence:

1. Stop the ggscale heartbeat ticker so the fleet entry's TTL starts counting down.
2. Deregister from ggscale immediately so new matchmaker tickets don't pick this server.
3. `server.Drain()` — sets the `draining` atomic flag (`onJoinRequest`
   immediately starts rejecting new players with "server draining"),
   waits for any in-progress match to complete or for `drainTimeout`
   (default 30 s), then stops the game loop.
4. `agones.Stop()` — calls `sdk.Shutdown()` (tells the Agones
   controller this exit was intentional, not a health timeout), then
   joins the health-heartbeat goroutine.
5. `os.Exit(0)`.

`Server.Drain()` and the Agones drain trigger are both idempotent
(`sync.Once`), so the SIGTERM-then-Agones-watcher or
Agones-watcher-then-SIGTERM orderings both work.

---

## Code map

| Concern | File | Notes |
|---|---|---|
| Process entry + wiring | `server/cmd/server/main.go` | Single `shutdown()` helper, signal handler armed before any blocking init. |
| Agones SDK lifecycle | `server/cmd/server/agones.go` | Narrow `agonesSDK` interface for test-fake-ability. Watcher registered before `Ready` to close the handshake race. Drain runs on its own goroutine so the SDK callback isn't blocked. |
| Drain semantics | `server/core/server.go` (`Drain`, `waitForMatchEnd`, `draining`, `matchInProgress`) | Atomic flag + `sync.Once`; bounded wait for active match. |
| Match state | `server/core/match.go` | Flips `matchInProgress` at `startMatch`/`endMatch`; fires the leaderboard hook at match end. |
| Game loop | `server/core/loop.go` | 60 Hz ticker; processes queued commands, updates match, physics, combat; runs `srvsync.DoSync`. |
| Bot AI | `server/core/botsystem.go` | Server-side AI ticks, optional `--bots N` startup spawn. |
| Network sync | uses `github.com/leap-fish/necs` (esync, srvsync) | The framework that mirrors entity state to all clients. |

---

## What we didn't bake into the server

| Out of scope | Lives in | Why |
|---|---|---|
| Authentication | ggscale | Game-server gets a session token in `JoinRequest` and trusts ggscale's signature. The server never sees a password. |
| Matchmaking | ggscale | Ticket queue, region pick, fleet pick. The server only sees pre-matched players. |
| Leaderboard storage | ggscale | The server *submits* scores via the SDK at match end; ggscale owns the ranking. |
| Pod lifecycle (start/stop/scale) | Agones | The server only signals state; the controller decides when a pod lives or dies. |
| Anti-cheat | future | Currently the server is authoritative for physics/scoring (which catches naïve cheats); deeper anti-cheat is a separate workstream. |

---

## Further reading

- `docs/fleet-onboarding.md` in the ggscale repo — the platform view
  of how any new game integrates: responsibilities split, manifest
  shape, transport choice (TCP/UDP/TCPUDP), scale notes, and a
  common-mistakes checklist. doomerang-server is one example of the
  pattern described there.
- Agones architecture: <https://agones.dev/site/docs/overview/>
- Agones GameServer SDK guide: <https://agones.dev/site/docs/guides/client-sdks/>
- ggscale's agones backend: `internal/fleet/agones/backend.go` in the ggscale repo.
- The Phase A → E rollout that brought this online: `docs/temp/agones-gameserver-sdk.md` in the ggscale repo.
