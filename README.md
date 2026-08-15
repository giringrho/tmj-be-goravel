# tesmajoo-be-goravel1 — Goravel Backend

Backend untuk tes Backend Engineer Position. Dibangun dengan **Goravel** (Go framework ala Laravel). Mencakup **Soal 1** (Concurrent CSV Processor) dan **Soal 2** (Blog REST API).

## Tech Stack

| Aspek | Pilihan |
|---|---|
| Framework | Goravel v1.18 (Gin engine) |
| Language | Go 1.23+ |
| Database | MySQL 8 (via GORM) |
| Auth | JWT (Goravel auth facade) |
| Docs | Swagger/OpenAPI (swaggo/gin-swagger) |
| Frontend | React + Vite (repo terpisah) |

## Prerequisites

- Go 1.23+
- MySQL 8.x
- (Optional) Docker / Docker Compose

## Setup & Run

### Local

```bash
# 1. Install dependencies
go mod download

# 2. Configure environment
cp .env.example .env
# Edit .env: set DB_CONNECTION, DB_HOST, DB_PORT, DB_DATABASE, DB_USERNAME, DB_PASSWORD
# IMPORTANT: set JWT_SECRET to a random 32+ char string

# 3. Run migrations + seed
go run . artisan migrate
go run . artisan db:seed

# 4. Start server (http://127.0.0.1:3000)
go run .
```

### Docker

```bash
docker-compose up --build
# Server runs on http://localhost:3000
```

## Seed Credentials

| Email | Password | Role |
|---|---|---|
| admin@example.com | password | admin |
| author@example.com | password | author |
| user@example.com | password | user |

## Soal 1 — Concurrent CSV Processor

Concurrent file processor yang membaca multiple CSV file Merchant, memproses dengan worker pool pattern, dan menghasilkan time-series per hari.

### Architecture

```
app/services/csvprocessor/
├── csvprocessor.go   # Orchestrator: spawns reader + builder goroutines
├── reader.go         # Reader goroutine: reads CSV file → jobs channel
├── builder.go        # Builder worker: parses rows from jobs channel
├── tracker.go        # Progress tracking with atomic counters
├── pool.go           # sync.Pool for *Row memory reuse
└── aggregate.go      # Time-series aggregation per day
```

### Endpoints

| Method | Path | Description |
|---|---|---|
| POST | `/merchants/process` | Upload multiple CSV files, process concurrently, return JSON result |
| POST | `/merchants/process-dir` | Process all CSV files from a directory path |
| POST | `/merchants/process/stream` | Upload CSV files, stream progress via SSE |

### Key Features

- **Worker pool**: N reader goroutines (one per file) + M configurable builder workers
- **Channels**: `jobsCh`, `dataCh`, `readReportCh`, `errCh` for coordination
- **Error handling**: Fail-fast with context cancellation on first error
- **Progress tracking**: Atomic counters, throttled emit every 100 rows, SSE streaming
- **Memory management**: `sync.Pool` for `*Row` structs to reduce GC pressure
- **Tests**: `csvprocessor_test.go` — 12 test functions covering happy path, errors, cancellation, edge cases

### Design Doc

See `docs/csvprocessor-design.md` for full architecture explanation.

---

## Soal 2 — Blog REST API

RESTful API untuk blog system dengan JWT auth, role-based authorization, CRUD posts & comments, input validation, dan database transactions.

### Database Schema

```sql
users    (id, name, email, password, role, created_at, updated_at)
posts    (id, user_id, title, content, published, created_at, updated_at)
comments (id, post_id, user_id, content, created_at, updated_at)
```

Migrations: `database/migrations/`

### Authorization Matrix

| Action | user | author | admin |
|---|---|---|---|
| View published posts | ✓ | ✓ | ✓ |
| View draft posts | ✗ | own only | all |
| Create post | ✗ | ✓ | ✓ |
| Update/Delete post | ✗ | own | all |
| Comment on published post | ✓ | ✓ | ✓ |
| Update comment | own | own | all |
| Delete comment | own | own + on own post | all |
| List users / Change role | ✗ | ✗ | ✓ |

### Endpoints

#### Auth (`/api/auth`)

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/auth/register` | public | Register new user (default role: user) |
| POST | `/api/auth/login` | public | Login, return JWT token |
| POST | `/api/auth/logout` | auth | Logout (client discards token) |
| GET | `/api/auth/me` | auth | Get current authenticated user |

#### Posts (`/api/posts`)

| Method | Path | Auth | Role | Description |
|---|---|---|---|---|
| GET | `/api/posts` | public* | - | List posts (paginated). *Drafts visible to owner/admin |
| GET | `/api/posts/:id` | public* | - | Get single post. *Draft requires auth + ownership/admin |
| POST | `/api/posts` | auth | author, admin | Create post |
| PUT | `/api/posts/:id` | auth | owner or admin | Update post |
| DELETE | `/api/posts/:id` | auth | owner or admin | Delete post + cascade comments (transaction) |

#### Comments (`/api/posts/:id/comments` + `/api/comments/:id`)

| Method | Path | Auth | Role | Description |
|---|---|---|---|---|
| GET | `/api/posts/:id/comments` | public | - | List comments for a post |
| POST | `/api/posts/:id/comments` | auth | any | Create comment on published post |
| GET | `/api/comments/:id` | auth | any | Get single comment |
| PUT | `/api/comments/:id` | auth | owner or admin | Update comment |
| DELETE | `/api/comments/:id` | auth | owner, post-owner, or admin | Delete comment |

#### Users (`/api/users` — admin only)

| Method | Path | Auth | Role | Description |
|---|---|---|---|---|
| GET | `/api/users` | auth | admin | List all users |
| PUT | `/api/users/:id/role` | auth | admin | Change user role |

### Key Features

- **JWT auth**: Goravel auth facade with `facades.Auth(ctx).Parse(token)`
- **Optional auth middleware**: Public read endpoints parse token if present (shows drafts to logged-in users)
- **Role middleware**: Parameterized — `middleware.Role("author", "admin")`
- **Transactions**: `facades.Orm().Transaction()` for cascade delete (comments → post)
- **Validation**: Input validation with standardized error responses
- **Swagger**: Annotations on all controllers, served at `/swagger`

### Design Doc

See `docs/blog-api-design.md` for full architecture, flow diagrams, and authorization matrix.

---

## API Documentation (Swagger)

Swagger UI available at: `http://localhost:3000/swagger`

Generated via `swag init` — annotations in controllers.

---

## Project Structure

```
app/
├── console/commands/     # Artisan commands (db:seed)
├── http/
│   ├── controllers/      # auth, post, comment, admin_user, merchant controllers
│   ├── middleware/       # auth, optional-auth, role, cors
│   └── responses/        # Standardized error response helpers
├── models/               # User, Post, Comment
├── services/
│   ├── auth_service.go
│   ├── post_service.go
│   ├── comment_service.go
│   └── csvprocessor/     # Soal 1: concurrent CSV processor
└── facades/              # Goravel facade wrappers
bootstrap/                # App bootstrap (providers, commands, seeders)
config/                   # Configuration (database, jwt, etc.)
database/
├── migrations/           # Schema migrations
└── seeders/              # Demo data seeder
docs/                     # Design documents
routes/                   # api.go, web.go, swagger.go
```

## Testing

```bash
# Run all tests
go test ./...

# Run CSV processor tests
go test ./app/services/csvprocessor/...
```

## Known Limitations & Future Improvements

- No foreign key constraints in migrations (cascade handled at application level via transactions)
- Email format validation not enforced (basic presence check only)
- No refresh token mechanism (JWT stateless)
- No rate limiting on auth endpoints
- CSV processor: no resume/checkpoint for very large files
