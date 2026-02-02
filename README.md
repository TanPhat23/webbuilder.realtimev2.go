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

- **Language:** Go (1.25+)
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

- Go 1.21 or higher
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

## 📡 WebSocket API

The engine exposes a WebSocket endpoint for real-time interaction:

`ws://localhost:3000/ws/:projectId`

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
