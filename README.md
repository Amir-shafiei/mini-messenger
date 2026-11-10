# Mini Messenger

A one-to-one realtime messenger built with Go, PostgreSQL, plain JavaScript, and WebSocket. The application includes a responsive Persian interface with local Vazir fonts and no frontend runtime dependencies.

## Quick start

Docker Desktop is the only prerequisite. From the project root, run:

```sh
docker compose up --build
```

Open [http://localhost:8080](http://localhost:8080). The application creates three demo accounts automatically:

| Username | Password |
| --- | --- |
| `alice` | `LocalDemo-2026!` |
| `bob` | `LocalDemo-2026!` |
| `charlie` | `LocalDemo-2026!` |

Use `docker compose down` to stop the application while preserving its database volume. Use `docker compose down -v` only when you want to delete the Docker demo data.

If port 8080 is occupied, set a different port before startup:

```powershell
$env:APP_PORT = "8082"
docker compose up --build
```

Then open `http://localhost:8082`.

The Compose setup is intended for local evaluation. It publishes the application only on `127.0.0.1`, does not publish PostgreSQL, and uses a demo-only database password. `DOCKER_LOCAL_DEMO` allows the explicit container listener and internal `db` hostname while the public origin remains loopback-only.

## Features

- Database-backed login sessions with 15-minute access JWTs and rotating refresh tokens.
- Real logout that revokes the session and closes all WebSocket connections for that session.
- Private text messages, paginated history, realtime delivery, and reconnect recovery.
- Online presence, typing hints, read receipts, unread counters, and conversation previews.
- Idempotent sends: retrying the same request key returns the original stored message.
- Responsive RTL interface, search, unread filter, drafts, emoji insertion, and message copy.
- Structured JSON logs with server-side request correlation; credentials and message text are excluded.

## Architecture

Dependencies point inward:

```text
main -> bootstrap -> adapters -> services -> model
```

| Directory | Responsibility |
| --- | --- |
| `backend/internal/model` | Framework-independent entities and validation rules |
| `backend/internal/service` | Application operations and repository contracts |
| `backend/internal/adapter/http` | HTTP input, cookies, validation, and JSON output |
| `backend/internal/adapter/postgres` | PostgreSQL repository implementations |
| `backend/internal/adapter/realtime` | WebSocket connections, presence, and events |
| `backend/internal/adapter/security` | JWTs, passwords, IDs, and refresh secrets |
| `backend/internal/bootstrap` | Configuration, dependency wiring, and server lifecycle |
| `backend/migrations` | Embedded, versioned database migrations |
| `frontend` | Plain HTML, CSS, and ES modules |

HTTP handlers never call PostgreSQL directly. Services own small interfaces for the data they consume, and adapters implement those interfaces.

## Message flow

The authenticated sender ID comes from the session, never from request JSON. A send operation validates the recipient and message, stores it in PostgreSQL, and only then publishes the same message ID to the sender and recipient through WebSocket. A failed database write publishes no event. A failed realtime publication does not roll back an already stored message; history recovers it.

The database stores every successfully accepted message and its idempotency key. Presence and typing are transient and remain in memory. Read cursors are persisted.

## Security decisions

Cookies are HttpOnly and SameSite=Strict. Production HTTPS uses Secure `__Host-` cookies. JWT validation pins HS256 and checks issuer, audience, expiry, subject, session ID, and the live database session. Refresh secrets are random, stored only as SHA-256 digests, rotated on use, and protected against replay.

Mutating browser requests and WebSocket upgrades require the exact configured origin. Login is rate-limited, password hashes use bcrypt, request bodies are bounded, JSON rejects unknown fields, and PostgreSQL errors do not cross the adapter boundary. Plain HTTP is restricted to loopback development. See [security.md](docs/security.md) for details.

## Configuration

| Variable | Purpose |
| --- | --- |
| `DATABASE_URL` | Required PostgreSQL URL |
| `JWT_SECRET_BASE64` | Base64 value encoding at least 32 random bytes |
| `HTTP_ADDR` | Listener; defaults to `127.0.0.1:8080` |
| `APP_ORIGIN` | Exact browser origin; defaults to `http://localhost:8080` |
| `COOKIE_SECURE` | Must be `true` for HTTPS |
| `TLS_CERT_FILE`, `TLS_KEY_FILE` | Certificate and key for direct HTTPS |
| `FRONTEND_DIR` | Static frontend directory |
| `SEED_DEMO` | Creates missing demo users when enabled |
| `DEMO_PASSWORD` | 12-72 byte password used by demo seeding |
| `DOCKER_LOCAL_DEMO` | Explicit local-container transport exception used by Compose |

The Go process reads environment variables; it does not load `.env` automatically. Use [.env.example](.env.example) as a reference and never commit real credentials.

## Running without Docker

Windows users with Go 1.24+ and PostgreSQL can configure the environment in the gitignored `.local.json` file and run:

```powershell
.\run-local.ps1
```

## Tests

From `backend`:

```sh
go test ./...
go vet ./...
```

Set `TEST_DATABASE_URL` to enable integration tests against PostgreSQL. Those tests create and remove an isolated random schema; without the variable they skip explicitly.

The test suite covers domain validation, service behavior, authentication and refresh rotation, idempotent sends, authorization boundaries, HTTP contracts, realtime connections, and repository integration.

## Current limitations

Realtime publication is best-effort after the message transaction commits. There is no durable Outbox or broker, so a process failure between storage and publication is recovered through history rather than guaranteed event redelivery. The in-memory WebSocket hub supports one backend instance. A multi-instance deployment would add a durable Outbox and shared event transport.

Public signup, group chat, attachments, and public deployment automation are outside this version's scope.

## Reference

- [API and realtime contract](docs/api.md)
- [Database design](docs/database.md)
- [Frontend design](docs/frontend-design.md)
- [Session and transport security](docs/security.md)
