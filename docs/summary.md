# Handoff: doomerang-mp ↔ ggscale single-stack docker-compose integration

This is a mid-flight handoff. Most of the work is done, **one blocker remains** in the doomerang-server Docker build chain. The chosen workaround (Xvfb) compiles but the binary doesn't actually run inside the container yet — see [§Current blocker](#current-blocker).

The approved plan is at `/Users/mydev/.claude/plans/we-want-to-keep-rosy-hejlsberg.md`. The working branch is `feature/ggscale-auth` on doomerang-mp, plus uncommitted work on `main` in the ggscale parent repo and the ggscale-go SDK.

---

## Goal

`docker compose up -d` from inside `work/doomerang-mp/` should bring up the full demo stack — postgres + mailhog + ggscale-server + a single doomerang-server — networked together. doomerang's old `master/` discovery service is removed; ggscale takes over server discovery via a new `/v1/fleet/*` API. The native game client (run on the host via `make run`) talks to ggscale for auth + leaderboards + server browsing, and to doomerang-server directly for gameplay over websockets.

---

## Repos in scope

| Repo | Path | Branch | State |
|---|---|---|---|
| ggscale (parent) | `/Users/mydev/code/ggscale` | `main` (uncommitted) | A: API + tests done |
| ggscale-go (SDK) | `/Users/mydev/code/ggscale-go` | not a git repo yet | B: Fleet service done |
| doomerang-mp (game) | `/Users/mydev/code/ggscale/work/doomerang-mp` | `feature/ggscale-auth` (uncommitted) | C, D done; E in progress |

---

## What's done

### Group A — ggscale `/v1/fleet/*` HTTP API (complete, tests pass, lint clean)

- `internal/fleet/registry.go` — in-memory registry, `Register/Heartbeat/Deregister/List/Sweep/Size`, 30s TTL eviction.
- `internal/fleet/registry_test.go` — table tests covering the happy path, tenant isolation, TTL eviction.
- `internal/httpapi/fleet.go` — four handlers:
  - `POST /v1/fleet/servers` — register; project pin required; API-key auth.
  - `PUT /v1/fleet/servers/{id}/heartbeat` — refresh TTL; API-key auth.
  - `DELETE /v1/fleet/servers/{id}` — deregister; API-key auth.
  - `GET /v1/fleet/servers` — list active in caller's project; session-token auth.
- `internal/httpapi/fleet_test.go` — handler-level unit tests bypassing middleware via context injection.
- `internal/httpapi/mount.go` — `mountFleetWriteRoutes`, `mountFleetReadRoutes`.
- `internal/httpapi/router.go` — wires `Fleet *fleet.Registry` into `Deps`, mounts write routes inside the API-key block, read inside the session-token block.
- `cmd/ggscale-server/main.go` — instantiates `fleet.NewRegistry(30 * time.Second)`.

Pre-existing dashboard test fix: `internal/httpapi/dashboard_test.go` line 56 was asserting "Set up ggscale Dashboard" but the page renders "Set up ggscale" — adjusted the assertion.

`make lint` clean. `go test -short ./...` passes.

### Group B — ggscale-go SDK FleetService (complete, tests pass, lint clean)

- `fleet.go` — `FleetService` exposing `Register/Heartbeat/Deregister/List`. The first three call `transport.Call` directly (API-key only, no session). `List` uses the existing `c.callProtected` (session required).
- `fleet_test.go` — fakeTransport-driven tests for each method.
- `client.go` — added `Fleet *FleetService` field, instantiated in `NewClient`.

### Group C — doomerang-server registers on boot (complete, host build OK)

- `server/go.mod` — added `require github.com/automoto/ggscale-go ...` and `replace ... => ../../../../ggscale-go`.
- `server/cmd/server/main.go` — replaced master-server registration with `startGgscaleRegistration(...)`. Reads `GGSCALE_URL` and `GGSCALE_API_KEY`; if either is unset, runs unregistered (preserves `make run-server` ergonomics). On startup: builds the SDK client, calls `Fleet.Register`, spawns a 10s heartbeat ticker, deregisters in the SIGTERM handler. The `--master`/`--region` flags collapsed into env-driven config (only `--region` removed; SDK uses env for API URL).
- `server/core/registration.go` — **deleted**. (The HTTP client to doomerang-master.)

Note: the host build (`go build ./...` from doomerang-mp root) is clean. The Docker build is the [blocker below](#current-blocker).

### Group D — client server browser uses ggscale (complete, host build + lint OK)

- `scenes/serverbrowser.go` — `queryMasterServer` replaced by `queryGgscaleFleet`. Calls `network.SharedGgscale().Fleet.List(ctx)` and maps `[]ggscale.FleetServer` → `[]ui.ServerEntry`. Removed `httpClient` field. Added `recordFetchResult` helper to dedupe error/success branches.
- `config/config.go` — removed `MasterServerURL` field (line 399) and its initializer (line 986). The `gofmt` realignment cleaned a few lines around it.

### Group E — compose rewrite (in progress, see blocker)

- `master/` directory **deleted**.
- `docker-compose.yml` rewritten as a single integrated stack: postgres + migrate + mailhog + ggscale-server (image `${GGSCALE_IMAGE:-buildwrangler/ggscale:latest}`) + doomerang-server. Network-shared. Migrations and init-sql mounted from sibling ggscale checkout (`../../db/migrations`, `../../compose/init-sql`).
- `server/Dockerfile` rewritten. Build context is now `../../..` (parent of both `ggscale/` and `ggscale-go/`); the directory layout inside the image mirrors the host so the relative `replace` directives in doomerang-mp's go.mod files resolve unchanged.
  - Builder image: `golang:1.24-bookworm` + apt installs `libgl1-mesa-dev libxcursor-dev libxi-dev libxinerama-dev libxrandr-dev libxxf86vm-dev libasound2-dev pkg-config`. Build is `CGO_ENABLED=1 go build`.
  - Runtime image: `debian:bookworm-slim` + apt installs `libasound2 libgl1 libxcursor1 libxi6 libxinerama1 libxrandr2 libxxf86vm1 libx11-6 xvfb xauth`. Entrypoint wraps the binary in `xvfb-run --auto-servernum --server-args="-screen 0 1x1x8"`.

`docker compose build doomerang-server` succeeds. `docker pull buildwrangler/ggscale:latest` works. Postgres, migrate, mailhog, ggscale-server all come up healthy.

---

## Current blocker

**doomerang-server starts under xvfb-run but the binary itself never produces output, then the container sits idle with only `xvfb-run` and `Xvfb` running** (confirmed by inspecting `/proc/*/cmdline` inside the container — there's no `/doomerang-server` process). `xvfb-run` swallowed whatever stderr the binary printed before dying. No registration request reaches ggscale-server, so `GET /v1/fleet/servers` returns an empty list.

### Why this is happening

The doomerang-server transitively imports `github.com/automoto/doomerang-mp/components`, which has files that import ebiten directly:

- `components/animation.go` — uses `*ebiten.Image`
- `components/audio.go` — uses `ebiten/v2/audio`
- `components/enemy.go`, `components/lobby.go`, `components/sprite.go` — ebiten direct
- `components/input.go` — uses `*ebiten.GamepadID` for `BoundGamepadID`

Because these are part of the same `components` package, importing the package compiles all of them. Plus `config/input.go` also imports ebiten (key bindings).

ebiten on Linux requires:
- CGO (we enabled this in the builder)
- libasound, libGL, libX11, libXcursor, etc. at runtime (we installed these)
- A display at process startup — `ebiten/v2/internal/ui.init.0()` calls GLFW's `Init()` unconditionally, even if the program never opens a window. Without `$DISPLAY` it panics with `glfw: X11: The DISPLAY environment variable is missing`.

`xvfb-run` was meant to satisfy that init by providing a fake X server. The wrapper does start Xvfb, but the doomerang-server binary it then exec's appears to be exiting silently.

### Verification of "binary not running"

```sh
$ docker exec doomerang-doomerang-server-1 sh -c 'for p in /proc/*/cmdline; do tr "\0" " " < "$p" 2>/dev/null; echo; done | head -5'
/bin/sh /usr/bin/xvfb-run --auto-servernum --server-args=-screen 0 1x1x8 /doomerang-server …
Xvfb :99 -screen 0 1x1x8 -nolisten tcp -auth /tmp/xvfb-run.yjKFQ3/Xauthority
# (no /doomerang-server process)
```

The wrapper hands off to the binary with `exec`, so when the binary panics or exits the wrapper exits too — but the container PID 1 keeps Xvfb alive, masking the failure.

---

## Recommended next steps

Two paths, in order of preference:

### Path 1 (recommended): split `components` so the server doesn't pull in ebiten

This is the right architectural fix and removes the need for xvfb, libasound, libgl, CGO, etc. entirely. The server doesn't render, so it shouldn't depend on a graphics engine.

The server actually only needs `components.Player`, `components.Bot`, `components.PlayerInput` from the components package (verified via `grep -rE "components\.[A-Z]" server/core/`). Everything else (Animation, Audio, Enemy, Lobby, Sprite, Input, InputData, ActionState, InputMethod constants) is client-only.

**Concrete plan:**

1. **Split `components/input.go`** into:
   - `components/playerinput.go` *(new, server-safe)* — `PlayerInputData`, `PlayerInput` component, `InputMethod` type + constants. **Change `BoundGamepadID *ebiten.GamepadID` to `BoundGamepadID *int`** (the underlying type — `ebiten.GamepadID = gamepad.ID = int`, verified via `go doc`). No ebiten import.
   - `components/input.go` *(modified, client-only)* — `Input`, `InputData`, `ActionState`, `KeyboardZone*` constants. Add `//go:build !nogui` at the top.

2. **Add `//go:build !nogui`** at the top of:
   - `components/animation.go`
   - `components/audio.go`
   - `components/enemy.go`
   - `components/lobby.go`
   - `components/sprite.go`

3. **Split `config/input.go`** into:
   - `config/actions.go` *(new, server-safe)* — `ActionID` alias, action constants, `ControlSchemeID` + `ControlSchemeNames`.
   - `config/input.go` *(modified, client-only)* — `InputBinding`, `InputConfig`, `Input` var, `ControlSchemeBindings`, `KeyboardZoneBindings`, `init()`. Add `//go:build !nogui`.

4. **Update callers** that use `BoundGamepadID *ebiten.GamepadID`:
   - `systems/input.go:200` — was `pollGamepadForPlayer(input, *input.BoundGamepadID)`. Change to `pollGamepadForPlayer(input, ebiten.GamepadID(*input.BoundGamepadID))`.
   - `systems/factory/player.go:38` — was `BoundGamepadID: inputCfg.GamepadID`. Need to know the type of `inputCfg.GamepadID`; if it's `*ebiten.GamepadID`, the assignment to `*int` won't compile directly. Either change `inputCfg.GamepadID` upstream or do a cast/conversion.

5. **Build doomerang-server with `-tags=nogui`**:
   - In `server/Dockerfile`, change `RUN CGO_ENABLED=1 go build ...` to `RUN CGO_ENABLED=0 go build -tags=nogui ...`.
   - Drop the entire builder `apt-get install ... libgl1-mesa-dev ... libasound2-dev ...` line.
   - Drop the runtime image's apt install (libasound2, libgl1, etc.).
   - Drop `xvfb`, `xauth`, and the `xvfb-run` ENTRYPOINT wrapper. Switch back to `gcr.io/distroless/static-debian12` for runtime.
   - Result: a much smaller, faster, healthier image.

6. **Rebuild + verify** (see [§Verification](#verification) below).

Estimated diff: ~80 LOC across 6 files. None of it is risky — all moving definitions around with clear semantic boundaries.

### Path 2 (fallback): debug why `xvfb-run /doomerang-server` exits silently

If Path 1 isn't viable for some reason, debug the current setup:

```sh
docker run --rm doomerang-doomerang-server xvfb-run --auto-servernum \
  --server-args="-screen 0 1x1x8" /doomerang-server --assets=/app/assets 2>&1
```

This was started in the background but interrupted before output was captured. Run interactively or capture output to see the actual failure (likely an ebiten/GLFW init problem the slim X libs don't fully satisfy, or a missing locale, or asset path issue). Path 2 keeps the architectural problem and only patches the symptoms; recommend not going this way.

---

## Verification

Once the server image runs, this is the smoke procedure (already documented in `/Users/mydev/code/ggscale/docs/integrations/doomerang.md`, will need a §"Single-stack compose" update as part of Group F):

```sh
# 1. Bring up the stack from doomerang-mp
cd /Users/mydev/code/ggscale/work/doomerang-mp
docker compose up -d --wait
docker compose ps  # all containers up; doomerang-server's logs show "Loaded N levels" and "Starting Doomerang server"

# 2. Bootstrap ggscale dashboard (one-time per fresh data volume)
cat /Users/mydev/code/ggscale/data/bootstrap.token   # bootstrap token
open http://localhost:3001/v1/dashboard/setup        # bootstrap admin, create tenant + project + project-pinned API key
KEY=ggs_...                                          # capture the API key

# 3. Re-up doomerang-server with the API key so it can register
GGSCALE_API_KEY=$KEY docker compose up -d doomerang-server
docker compose logs doomerang-server | grep "registered"  # expect: [ggscale] registered as id=…, advertising localhost:7373

# 4. Sign up + verify a demo player (curl from docs/SMOKE_TESTS.md §3)
# 5. Create a leaderboard via psql:
docker compose exec postgres psql -U ggscale -d ggscale \
  -c "INSERT INTO leaderboards (tenant_id, project_id, name, sort_order) VALUES (1,1,'doomerang-kos','desc') RETURNING id;"
LB=<id>

# 6. List the registered server via ggscale
JWT=$(curl -s -X POST http://localhost:8080/v1/auth/login \
  -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
  -d '{"email":"demo@example.com","password":"hunter2hunter2"}' | jq -r .access_token)
curl -s "http://localhost:8080/v1/fleet/servers" \
  -H "Authorization: Bearer $KEY" -H "X-Session-Token: $JWT" | jq .
# expect: {"servers":[{"id":"…","name":"Test Server","address":"localhost:7373",…}]}

# 7. Run the native client and confirm the in-game server browser shows it
GGSCALE_BASE_URL=http://localhost:8080 \
GGSCALE_API_KEY=$KEY \
GGSCALE_EMAIL=demo@example.com \
GGSCALE_PASSWORD=hunter2hunter2 \
GGSCALE_LEADERBOARD_ID=$LB \
  make run

# 8. Verify heartbeat: docker compose stop doomerang-server, wait 35s, re-curl /fleet/servers — expect empty
```

For the unit-test side, both ggscale parent and ggscale-go SDK pass:

```sh
cd /Users/mydev/code/ggscale && go test -short ./... && make lint
cd /Users/mydev/code/ggscale-go && go test ./... && make lint
cd /Users/mydev/code/ggscale/work/doomerang-mp && go build ./... && make lint
```

(All three are clean as of this handoff.)

---

## Group F (still TODO)

Once the docker build is healthy:

- Update `/Users/mydev/code/ggscale/docs/integrations/doomerang.md` "End-to-end smoke procedure" section to reflect the new single-compose flow (replaces the existing "Bring up ggscale-server / Run the doomerang server" steps).
- Add a brief `## Local stack (with ggscale)` section to `/Users/mydev/code/ggscale/work/doomerang-mp/README.md` pointing at the same procedure.
- Optionally remove or update `/Users/mydev/code/ggscale/ops/docker-compose.gameserver.yml` and `…gameserver.prod.yml` — they were paper-only Phase-2 sketches and now overlap conceptually with the real compose in doomerang-mp. Ask the user before deleting; they're tied to m1.md §7.

---

## Uncommitted state

**ggscale parent** (`/Users/mydev/code/ggscale`, branch `main`):

- New: `internal/fleet/`, `internal/httpapi/fleet.go`, `internal/httpapi/fleet_test.go`
- Modified: `internal/httpapi/mount.go`, `internal/httpapi/router.go`, `internal/httpapi/dashboard_test.go`, `cmd/ggscale-server/main.go`
- Carried over from previous session (uncommitted): `docs/integrations/doomerang.md`, `docs/temp/m1.md` (§9 ticks), `code-review.md`, `scripts/README.md`, `scripts/sync-docs.sh`

**ggscale-go SDK** (`/Users/mydev/code/ggscale-go`, no git):

- New: `fleet.go`, `fleet_test.go`
- Modified: `client.go`

**doomerang-mp** (`/Users/mydev/code/ggscale/work/doomerang-mp`, branch `feature/ggscale-auth`):

- Deleted: `master/` (whole directory), `server/core/registration.go`
- Modified: `docker-compose.yml`, `server/Dockerfile`, `server/cmd/server/main.go`, `server/go.mod`, `server/go.sum`, `scenes/serverbrowser.go`, `config/config.go`
- New: `summary.md` (this file)

Don't commit any of this until the blocker is resolved. The user has a clear preference for omitting `Co-Authored-By: Claude` from commit messages — see `/Users/mydev/.claude/projects/-Users-mydev-code-ggscale/memory/feedback_no_claude_coauthor.md`.

---

## Memory references worth knowing

These live in `/Users/mydev/.claude/projects/-Users-mydev-code-ggscale/memory/`:

- `feedback_just_fix_it.md` — when `make lint`/`test`/`build` fails (incl. pre-existing failures), fix it; don't ask whether to investigate.
- `feedback_no_claude_coauthor.md` — omit the `Co-Authored-By: Claude` trailer from every commit.
- `feedback_aggressive_refactor.md` — pre-launch, no users yet; collapse phased migrations and delete legacy paths in the same PR. (This is what authorised dropping `master/` outright instead of dual-pathing.)
- `reference_ggscale_docker_image.md` — `buildwrangler/ggscale:latest` on Docker Hub; push via `make docker-push` from the ggscale repo, optional `TAG=`.
- `project_sdk_go_module.md` — sdk-go is a separate Go module so it can move to its own repo later.

---

## Original plan reference

`/Users/mydev/.claude/plans/we-want-to-keep-rosy-hejlsberg.md` has the full design rationale, including why we picked in-memory storage over postgres for the fleet registry and why the same project-pinned API key authenticates both client (read) and server (write) calls.
