# doomerang-server: headless game server. Built with -tags nogui to
# exclude all ebiten-dependent files (rendering, audio, gamepad input —
# none of which the server uses). CGO disabled, so the binary is fully
# static and runs on distroless without any system libraries.
#
# Build context is the parent of `ggscale/` and `ggscale-go/` (set by
# docker-compose.yml). The directory layout inside the image mirrors the
# host so the relative `replace` directives in server/go.mod resolve
# unchanged.
FROM golang:1.24-alpine AS builder
WORKDIR /code
COPY ggscale-go/ /code/ggscale-go/
COPY ggscale/work/doomerang-mp/ /code/ggscale/work/doomerang-mp/

WORKDIR /code/ggscale/work/doomerang-mp/server
RUN go mod download
RUN CGO_ENABLED=0 go build -tags nogui -o /doomerang-server ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=builder /doomerang-server /doomerang-server
COPY ggscale/work/doomerang-mp/assets/levels/ /app/assets/levels/

EXPOSE 7373
ENTRYPOINT ["/doomerang-server", "--assets=/app/assets"]
