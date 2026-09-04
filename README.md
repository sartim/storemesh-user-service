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

### Disposable demo accounts

The service seeds demo customer and administrator accounts only when all four
`DEMO_*` email/password environment variables are supplied. Existing users are
not overwritten on restart. Keep these variables limited to local or CI
environments; production secrets should omit them.

## Run locally without Docker or Kubernetes

Requires Go 1.26.6 or newer. The service can run with its local compatibility
configuration while developing the HTTP/gRPC boundary:

```sh
GRPC_PORT=50054 HTTP_PORT=8090 \
JWT_SECRET='local-development-secret-at-least-32-characters' \
DEMO_CUSTOMER_EMAIL='demo@storemesh.local' \
DEMO_CUSTOMER_PASSWORD='StoreMesh-demo-2026!' \
DEMO_ADMIN_EMAIL='admin@storemesh.local' \
DEMO_ADMIN_PASSWORD='StoreMesh-admin-2026!' \
go run ./cmd/server
```

Without `DATABASE_URL` and `REDIS_URL`, use the service's local development
mode where supported. Set those variables only when exercising persistent
readiness and session behavior against separately managed dependencies. The
ports above allow the BFF to use `localhost:50054` while Product, Inventory,
and Order use their own local gRPC ports. The user service HTTP port is `8090`
to avoid colliding with the BFF's `8080` port.
