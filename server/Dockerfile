FROM golang:1.24-bookworm AS builder

WORKDIR /app

# Copy the root module first (shared/ packages + go.mod for replace directive)
COPY go.mod go.sum ./
COPY shared/ shared/

# Copy the server module
COPY server/go.mod server/go.sum server/
COPY server/core/ server/core/
COPY server/cmd/ server/cmd/

# Download server deps then build a static binary
WORKDIR /app/server
RUN go mod download
RUN CGO_ENABLED=0 go build -o /doomerang-server ./cmd/server

# Minimal runtime image — no OS, just the binary
FROM gcr.io/distroless/static-debian12

COPY --from=builder /doomerang-server /doomerang-server

EXPOSE 7373

ENTRYPOINT ["/doomerang-server"]
