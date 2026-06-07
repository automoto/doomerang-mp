module github.com/automoto/doomerang-mp/server

go 1.25.0

// Use local parent module for shared/ packages during development
replace github.com/automoto/doomerang-mp => ../

// Local ggscale-go SDK checkout. Until the SDK is tagged and pushed
// publicly, depend on the local clone alongside this repo.
replace github.com/automoto/ggscale-go => ../../../../ggscale-go

require (
	agones.dev/agones v1.57.0
	github.com/automoto/doomerang-mp v0.0.0
	github.com/automoto/ggscale-go v0.0.0-00010101000000-000000000000
	github.com/leap-fish/necs v0.0.5-0.20250625124528-82c5928cb7a1
	github.com/solarlune/resolv v0.6.0
	github.com/stretchr/testify v1.11.1
	github.com/yohamta/donburi v1.15.7
	go.uber.org/goleak v1.3.0
)

require (
	github.com/beefsack/go-astar v0.0.0-20200827232313-4ecf9e304482 // indirect
	github.com/coder/websocket v1.8.12 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/disintegration/imaging v1.6.2 // indirect
	github.com/ebitengine/gomobile v0.0.0-20250923094054-ea854a63cce1 // indirect
	github.com/ebitengine/hideconsole v1.0.0 // indirect
	github.com/ebitengine/oto/v3 v3.4.0 // indirect
	github.com/ebitengine/purego v0.9.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.3 // indirect
	github.com/hajimehoshi/ebiten/v2 v2.9.7 // indirect
	github.com/hashicorp/go-msgpack/v2 v2.1.2 // indirect
	github.com/jezek/xgb v1.1.1 // indirect
	github.com/jfreymuth/oggvorbis v1.0.5 // indirect
	github.com/jfreymuth/vorbis v1.0.2 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/kvartborg/vector v0.1.2 // indirect
	github.com/lafriks/go-tiled v0.13.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rogpeppe/go-internal v1.15.0 // indirect
	github.com/tanema/gween v0.0.0-20221212145351-621cc8a459d1 // indirect
	golang.org/x/image v0.31.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/grpc v1.79.3 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
