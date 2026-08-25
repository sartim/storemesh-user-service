# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26.5
ARG TARGETOS=linux
ARG TARGETARCH=amd64

FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/storemesh-user-service \
    ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

ARG BUILD_DATE="unknown"
ARG VERSION="dev"
ARG VCS_REF="unknown"
ARG SOURCE_URL="https://github.com/sartim/storemesh-user-service"

LABEL org.opencontainers.image.title="StoreMesh User Service" \
      org.opencontainers.image.description="Identity and user service for the StoreMesh platform." \
      org.opencontainers.image.source="${SOURCE_URL}" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.licenses="Apache-2.0"

WORKDIR /app

COPY --from=builder \
    /out/storemesh-user-service \
    /app/storemesh-user-service

EXPOSE 50051
EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/app/storemesh-user-service"]
