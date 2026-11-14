# WebBuilder V2 Real-time Engine

A high-performance, horizontally scalable real-time synchronization backend for a collaborative web building platform. Built with Go, Fiber, and Redis.

## 🚀 Features

- **Real-time Collaboration:** Low-latency synchronization of page elements using WebSockets.
- **Horizontal Scalability:** Distributed Hub architecture using Redis Pub/Sub, allowing multiple server instances to stay in sync.
- **Granular Updates:** Efficient patching of element properties and styles to minimize network overhead.
- **Presence Tracking:** Real-time cursor movement and user selection broadcasting.
- **Clean Architecture:** Strictly decoupled layers (Delivery, UseCase, Repository, Domain) for high maintainability.
- **Automatic Caching:** Redis-backed document snapshots for fast initial page loads.
- **Command Pattern:** Extensible message handling system for easy addition of new collaboration features.

## 🛠 Tech Stack

- **Language:** Go (1.25.5+)
- **Web Framework:** [Fiber v2](https://gofiber.io/)
- **WebSocket:** [Fiber WebSocket Contrib](https://github.com/gofiber/contrib/tree/main/websocket)
- **Database:** PostgreSQL with [GORM](https://gorm.io/)
- **Cache & Pub/Sub:** [Redis v9](https://github.com/redis/go-redis)
- **Validation:** [Go Playground Validator](https://github.com/go-playground/validator)

## 📁 Project Structure

```text
.
├── cmd/
│   └── api/                # Application entry point
├── docs/                   # Architectural and API documentation
├── internal/
│   ├── config/             # Configuration management
│   ├── delivery/           # Delivery layer (WebSocket handlers, routes)
│   ├── domain/             # Enterprise business rules (Entities, Interfaces, Errors)
│   ├── repository/         # Data access implementation (Postgres, Redis)
│   ├── usecase/            # Application business logic
│   └── utils/              # Shared utilities (Logger, Validation)
└── pkg/
    ├── database/           # DB connection helpers
    └── redis/              # Redis client initialization
```

## 🚦 Getting Started

### Prerequisites

- Go 1.25.5 or higher
- PostgreSQL instance
- Redis instance

### Installation

1. Clone the repository:

   ```bash
   git clone <repository-url>
   cd webbuilderv2.realtime.go
   ```

2. Install dependencies:

   ```bash
   go mod download
   ```

3. Configure environment variables (or use default config):
   The application loads configuration via `internal/config`. Ensure your PostgreSQL and Redis instances are accessible.

4. Run the application:
   ```bash
   go run cmd/api/main.go
   ```

### Configuration

The application loads configuration from environment variables (defaults shown):

- `SERVER_PORT` — default `8080`
- `DATABASE_URL` — default `""` (if set, it takes precedence over individual DB\_\* variables)
- `DB_HOST` — default `localhost`
- `DB_PORT` — default `5432`
- `DB_USER` — default `postgres`
- `DB_PASSWORD` — default `postgres`
- `DB_NAME` — default `webbuilder`
- `DB_SSLMODE` — default `disable`
- `REDIS_URL` — default `""` (if set, it takes precedence over individual REDIS\_\* variables)
- `REDIS_HOST` — default `localhost`
- `REDIS_PORT` — default `6379`
- `REDIS_PASSWORD` — default `""`
- `REDIS_DB` — default `0`
- `DEV_USER_ID` — default `dev-user-id`. When `DEV_MODE=true`, this value will be used as the placeholder user ID.
- `DEV_MODE` — default `false`. When `DEV_MODE=true`, the HTTP middleware runs in Dev Mode and injects a placeholder user for requests (see `DEV_USER_ID`). Note: token validation (JWT/session) is not implemented and is marked as TODO in the code.

### Development

Useful Makefile targets:

- `make deps` — download and tidy dependencies
- `make build` — build the binary
- `make run` — build and run the binary
- `make test` — run tests
- `make seed CMD=user` — seed the database (usage: `make seed CMD=user`)
- `make docker-build` / `make docker-run` — docker build/run targets

Notes:

- Database auto-migration is commented out in `cmd/api/main.go`. Enable it there if you want the app to auto-migrate on startup.

## 📡 WebSocket API

The engine exposes a WebSocket endpoint for real-time interaction:

`ws://localhost:8080/ws/:projectId`

Key message types include:

- `join`: Join a specific page context.
- `presence`: Share cursor positions.
- `element:create / update / move / delete`: Collaborative canvas operations.

For detailed message schemas and error codes, see the [WebSocket API Docs](./docs/websocket_api.md).

## 🏗 Architecture

The system is designed for high availability and performance:

- **The Hub:** Manages local connections in Go channels.
- **Redis Backplane:** Synchronizes messages between different server nodes.
- **Write-Through Caching:** Updates are persisted to the DB and cached in Redis simultaneously to ensure consistency and speed.

See the [Architecture Documentation](./docs/architecture.md) for a deep dive into the system design.

## 📝 License

This project is licensed under the MIT License.

## Realtime schema compatibility

The API owns the shared database tables. Before serving sockets, realtime startup
checks for `Element.OrderKey` and adds it if missing, using the targeted migration:

```sql
ALTER TABLE "Element" ADD COLUMN IF NOT EXISTS "OrderKey" text NOT NULL DEFAULT '';
```

Existing elements and numeric ordering are preserved. Legacy rows get fractional
keys when their page is loaded or mutated; persistence saves those keys along with
`Order`. No other table definitions are migrated. Deploy this version of the
realtime service to upgrade older databases before clients attempt to join pages.
The initial upgrade needs permission to alter `Element`; installations using a
restricted runtime role can run the SQL with their migration role first.

Regression tests can use disposable PostgreSQL and Redis instances:

```bash
WEBBUILDER_TEST_DATABASE_URL='postgres://postgres:password@localhost:5432/postgres?sslmode=disable' \
WEBBUILDER_TEST_REDIS_ADDR='localhost:6379' \
go test ./... -count=1
```

PostgreSQL regressions create and drop unique test schemas. They verify legacy
schema startup, repeat startup without data loss, order-key persistence, and a
real WebSocket page join followed by acknowledged element creation and editing.
The PostgreSQL and Redis integration tests skip when their respective variables
are unset.
