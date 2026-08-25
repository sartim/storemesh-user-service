# storemesh-user-service

## Transport ownership

The service runs two transport adapters over the same domain service:

- gRPC is the internal service-to-service API on port `50051`.
- Gin owns the directly served HTTP API on port `8080`.

The HTTP annotations in `proto/user/v1/user.proto` document equivalent REST
operations and generate the OpenAPI contract. The process does not run a
grpc-gateway reverse proxy, so HTTP handlers remain explicit transport
adapters rather than a second source of business logic.

## Authentication operations

| Operation | HTTP | gRPC |
|---|---|---|
| Login | `POST /api/v1/auth/login` | `Authenticate` |
| Refresh tokens | `POST /api/v1/auth/refresh` | `RefreshToken` |
| Logout current session | `POST /api/v1/auth/logout` | `Logout` |
| Logout all sessions | `POST /api/v1/auth/logout-all` | `LogoutAll` |

## User listing

Administrators can list users with optional pagination and status filters:

```text
GET /api/v1/users?page=1&per_page=20&status=active
```

## Health and observability

| Endpoint | Purpose |
|---|---|
| `GET /healthz` | Process liveness; does not query dependencies |
| `GET /readyz` | Readiness for PostgreSQL and Redis |
| `GET /metrics` | Prometheus application, Go runtime, and process metrics |

Readiness returns `200 OK` only when both PostgreSQL and Redis respond within
the bounded check timeout. It returns `503 Service Unavailable` when either
dependency is unavailable.

HTTP requests are traced with OpenTelemetry and use W3C Trace Context and
Baggage propagation. Traces are exported to the configured `OTLP_ENDPOINT`.

HTTP metrics use normalized Gin route templates to prevent user IDs and other
path parameters from creating unbounded Prometheus label cardinality.

## Buf commands

    buf build
    buf generate
