module github.com/automoto/doomerang-mp/server

go 1.24.0

toolchain go1.24.5

// Use local parent module for shared/ packages during development
replace github.com/automoto/doomerang-mp => ../

require (
	github.com/automoto/doomerang-mp v0.0.0
	github.com/leap-fish/necs v0.0.5-0.20250625124528-82c5928cb7a1
	github.com/solarlune/resolv v0.6.0
	github.com/yohamta/donburi v1.15.7
)

require (
	github.com/beefsack/go-astar v0.0.0-20200827232313-4ecf9e304482 // indirect
	github.com/coder/websocket v1.8.12 // indirect
	github.com/disintegration/imaging v1.6.2 // indirect
	github.com/ebitengine/gomobile v0.0.0-20250923094054-ea854a63cce1 // indirect
	github.com/ebitengine/hideconsole v1.0.0 // indirect
	github.com/ebitengine/oto/v3 v3.4.0 // indirect
	github.com/ebitengine/purego v0.9.0 // indirect
	github.com/hajimehoshi/ebiten/v2 v2.9.7 // indirect
	github.com/hashicorp/go-msgpack/v2 v2.1.2 // indirect
	github.com/jezek/xgb v1.1.1 // indirect
	github.com/jfreymuth/oggvorbis v1.0.5 // indirect
	github.com/jfreymuth/vorbis v1.0.2 // indirect
	github.com/kvartborg/vector v0.1.2 // indirect
	github.com/lafriks/go-tiled v0.13.0 // indirect
	github.com/tanema/gween v0.0.0-20221212145351-621cc8a459d1 // indirect
	golang.org/x/image v0.31.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
)
