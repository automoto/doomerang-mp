FROM golang:1.24-bookworm AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY *.go ./
RUN CGO_ENABLED=0 go build -o /doomerang-master .

FROM gcr.io/distroless/static-debian12

COPY --from=builder /doomerang-master /doomerang-master

EXPOSE 8080

ENTRYPOINT ["/doomerang-master"]
