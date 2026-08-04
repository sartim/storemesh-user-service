# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26.5

FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/storemesh-user-service \
    ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder \
    /out/storemesh-user-service \
    /app/storemesh-user-service

EXPOSE 50051
EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/app/storemesh-user-service"]
