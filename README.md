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

## Buf commands

    buf build
    buf generate
