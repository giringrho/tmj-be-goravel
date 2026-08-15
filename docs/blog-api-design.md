# Blog API — Design

## Tujuan

RESTful API untuk blog system dengan JWT auth, role-based authorization, CRUD posts & comments, input validation, dan database transactions.

## Keputusan

| Aspek | Pilihan |
|---|---|
| Auth | JWT (Goravel auth facade, config/jwt.go sudah ada) |
| Role | 3 role: `user`, `author`, `admin` |
| Comment ops | Full CRUD (create, read, update, delete) |
| Delete strategy | Hard delete + transaction (cascade) |
| ORM | Goravel ORM (GORM-based) via `facades.Orm()` |
| Validation | Goravel validation facade |

## Database Schema

```sql
-- users
CREATE TABLE users (
  id         BIGINT PRIMARY KEY AUTO_INCREMENT,
  name       VARCHAR(100) NOT NULL,
  email      VARCHAR(255) NOT NULL UNIQUE,
  password   VARCHAR(255) NOT NULL,
  role       VARCHAR(20)  NOT NULL DEFAULT 'user',  -- user|author|admin
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- posts
CREATE TABLE posts (
  id         BIGINT PRIMARY KEY AUTO_INCREMENT,
  user_id    BIGINT NOT NULL,
  title      VARCHAR(255) NOT NULL,
  content    TEXT NOT NULL,
  published  BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_user_id (user_id),
  INDEX idx_published (published),
  CONSTRAINT fk_posts_user FOREIGN KEY (user_id) REFERENCES users(id)
);

-- comments
CREATE TABLE comments (
  id         BIGINT PRIMARY KEY AUTO_INCREMENT,
  post_id    BIGINT NOT NULL,
  user_id    BIGINT NOT NULL,
  content    TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_post_id (post_id),
  INDEX idx_user_id (user_id),
  CONSTRAINT fk_comments_post FOREIGN KEY (post_id) REFERENCES posts(id),
  CONSTRAINT fk_comments_user FOREIGN KEY (user_id) REFERENCES users(id)
);
```

## Models

```go
// app/models/user.go
type User struct {
    ID        uint
    Name      string
    Email     string
    Password  string
    Role      string  // user|author|admin
    CreatedAt time.Time
    UpdatedAt time.Time
}

// app/models/post.go
type Post struct {
    ID        uint
    UserID    uint
    User      User    // belongs to
    Title     string
    Content   string
    Published bool
    Comments  []Comment  // has many
    CreatedAt time.Time
    UpdatedAt time.Time
}

// app/models/comment.go
type Comment struct {
    ID        uint
    PostID    uint
    Post      Post    // belongs to
    UserID    uint
    User      User    // belongs to
    Content   string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

## Authorization Matrix

| Action | user | author | admin |
|---|---|---|---|
| View published posts | ✓ | ✓ | ✓ |
| View draft posts | ✗ | own only | all |
| Create post | ✗ | ✓ | ✓ |
| Update post | ✗ | own | all |
| Delete post (+ cascade comments) | ✗ | own | all |
| Comment on published post | ✓ | ✓ | ✓ |
| View comments | ✓ | ✓ | ✓ |
| Update comment | own | own | all |
| Delete comment | own | own + on own post | all |
| List users | ✗ | ✗ | ✓ |
| Change user role | ✗ | ✗ | ✓ |

## Endpoints

### Auth (`/api/auth`)

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/auth/register` | public | Register new user (default role: user) |
| POST | `/api/auth/login` | public | Login, return JWT token |
| POST | `/api/auth/logout` | auth | Logout (client discards token) |
| GET | `/api/auth/me` | auth | Get current authenticated user |

### Posts (`/api/posts`)

| Method | Path | Auth | Role | Description |
|---|---|---|---|---|
| GET | `/api/posts` | public* | - | List posts (paginated). *Drafts only for author/admin own |
| GET | `/api/posts/:id` | public* | - | Get single post. *Draft requires auth + ownership/admin |
| POST | `/api/posts` | auth | author, admin | Create post |
| PUT | `/api/posts/:id` | auth | owner or admin | Update post |
| DELETE | `/api/posts/:id` | auth | owner or admin | Delete post + cascade comments (transaction) |

### Comments (`/api/posts/:id/comments` + `/api/comments/:id`)

| Method | Path | Auth | Role | Description |
|---|---|---|---|---|
| GET | `/api/posts/:id/comments` | public* | - | List comments for a post. *Post must be published or owned |
| POST | `/api/posts/:id/comments` | auth | any | Create comment on published post |
| GET | `/api/comments/:id` | auth | any | Get single comment |
| PUT | `/api/comments/:id` | auth | owner or admin | Update comment |
| DELETE | `/api/comments/:id` | auth | owner, post-owner, or admin | Delete comment |

### Users (`/api/users` — admin only)

| Method | Path | Auth | Role | Description |
|---|---|---|---|---|
| GET | `/api/users` | auth | admin | List all users |
| PUT | `/api/users/:id/role` | auth | admin | Change user role |

## Middleware

```
app/http/middleware/
├── auth_middleware.go     # Validate JWT, load user into context
└── role_middleware.go     # Check user role (parameterized: role:author, role:admin)
```

### auth middleware flow
```
request → extract Authorization header (Bearer <token>)
  → no header? 401 Unauthorized
  → parse token via facades.Auth().Parse(token)
  → invalid/expired? 401 Unauthorized
  → get user ID from token → facades.Auth().User(ctx, &user)
  → set user in context (ctx.WithUser(user) or custom key)
  → next handler
```

### role middleware flow
```
request → auth middleware must run first
  → get user from context
  → check user.Role against required roles
  → not authorized? 403 Forbidden
  → next handler
```

## Validation Rules

### Register
| Field | Rules |
|---|---|
| name | required, max 100 |
| email | required, email format, unique in users table |
| password | required, min 8, confirmed (password_confirmation) |

### Login
| Field | Rules |
|---|---|
| email | required, email format |
| password | required |

### Post create
| Field | Rules |
|---|---|
| title | required, max 255 |
| content | required |
| published | boolean, default false |

### Post update
| Field | Rules |
|---|---|
| title | optional, max 255 |
| content | optional |
| published | optional, boolean |

### Comment create/update
| Field | Rules |
|---|---|
| content | required |

### Role change
| Field | Rules |
|---|---|
| role | required, in: user, author, admin |

## Error Response Format

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Validation failed",
    "details": [
      {"field": "email", "message": "email is required"},
      {"field": "password", "message": "password must be at least 8 characters"}
    ]
  }
}
```

### HTTP Status Codes

| Code | Error Code | Description |
|---|---|---|
| 400 | VALIDATION_ERROR | Input validation failed |
| 401 | UNAUTHENTICATED | No token or invalid token |
| 403 | FORBIDDEN | Authenticated but not authorized |
| 404 | NOT_FOUND | Resource not found |
| 409 | CONFLICT | Duplicate resource (e.g. email) |
| 500 | INTERNAL_ERROR | Server error |

## Flow Diagrams

### Register flow
```
POST /api/auth/register
│
├── validate input (name, email, password, password_confirmation)
│     └── invalid? → 400 VALIDATION_ERROR with details
│
├── check email uniqueness
│     └── exists? → 409 CONFLICT "email already registered"
│
├── hash password (facades.Hash().Make(password))
│
├── create user in DB (role = "user" by default)
│
├── generate JWT (facades.Auth().LoginUsingID(ctx, user.ID))
│
└── return 201 { token, user: {id, name, email, role} }
```

### Login flow
```
POST /api/auth/login
│
├── validate input (email, password)
│     └── invalid? → 400 VALIDATION_ERROR
│
├── find user by email
│     └── not found? → 401 UNAUTHENTICATED "invalid credentials"
│
├── verify password (facades.Hash().Check(password, user.Password))
│     └── mismatch? → 401 UNAUTHENTICATED "invalid credentials"
│
├── generate JWT (facades.Auth().LoginUsingID(ctx, user.ID))
│
└── return 200 { token, user: {id, name, email, role} }
```

### Create post flow
```
POST /api/posts
│
├── auth middleware → load user
│     └── no/invalid token? → 401
│
├── role middleware → check role is author or admin
│     └── user role? → 403 FORBIDDEN
│
├── validate input (title, content, published)
│     └── invalid? → 400 VALIDATION_ERROR
│
├── create post (user_id = current user, published default false)
│
└── return 201 { post }
```

### Update post flow
```
PUT /api/posts/:id
│
├── auth middleware → load user
│
├── find post by ID
│     └── not found? → 404 NOT_FOUND
│
├── ownership/role check:
│     ├── user.Role == "admin" → allow
│     ├── post.UserID == user.ID → allow
│     └── else → 403 FORBIDDEN
│
├── validate input (title?, content?, published?)
│     └── invalid? → 400 VALIDATION_ERROR
│
├── update post in DB
│
└── return 200 { post }
```

### Delete post flow (with transaction)
```
DELETE /api/posts/:id
│
├── auth middleware → load user
│
├── find post by ID
│     └── not found? → 404 NOT_FOUND
│
├── ownership/role check (same as update)
│     └── not authorized? → 403 FORBIDDEN
│
├── TRANSACTION BEGIN
│     ├── delete all comments where post_id = :id
│     └── delete post where id = :id
│     └── error? → ROLLBACK → 500 INTERNAL_ERROR
├── COMMIT
│
└── return 204 No Content
```

### Create comment flow
```
POST /api/posts/:id/comments
│
├── auth middleware → load user
│
├── find post by ID
│     └── not found? → 404 NOT_FOUND
│
├── check post is published OR user owns post OR user is admin
│     └── not published & not authorized? → 403 FORBIDDEN
│
├── validate input (content)
│     └── invalid? → 400 VALIDATION_ERROR
│
├── create comment (post_id, user_id = current user)
│
└── return 201 { comment }
```

### Delete comment flow
```
DELETE /api/comments/:id
│
├── auth middleware → load user
│
├── find comment by ID (with associated post)
│     └── not found? → 404 NOT_FOUND
│
├── authorization check (any of):
│     ├── comment.UserID == user.ID → allow (owner)
│     ├── comment.Post.UserID == user.ID → allow (post owner)
│     ├── user.Role == "admin" → allow
│     └── else → 403 FORBIDDEN
│
├── delete comment from DB
│
└── return 204 No Content
```


## Project Structure

```
app/
├── http/
│   ├── controllers/
│   │   ├── auth_controller.go        # register, login, logout, me
│   │   ├── post_controller.go        # CRUD posts
│   │   ├── comment_controller.go     # CRUD comments
│   │   ├── user_controller.go        # existing (keep)
│   │   └── admin_user_controller.go  # admin: list users, change role
│   └── middleware/
│       ├── auth_middleware.go
│       └── role_middleware.go
├── models/
│   ├── user.go
│   ├── post.go
│   └── comment.go
├── services/
│   ├── auth_service.go               # register, login, token gen
│   ├── post_service.go               # CRUD + ownership check + transaction
│   ├── comment_service.go            # CRUD + ownership check
│   └── csvprocessor/                 # existing concurrent CSV processor (standalone)
├── requests/
│   ├── auth_request.go               # validation rules for auth
│   ├── post_request.go               # validation rules for posts
│   └── comment_request.go            # validation rules for comments
└── responses/
    └── error_response.go             # standardized error response helper

database/
└── migrations/
    ├── 20260814000001_create_users_table.go
    ├── 20260814000002_create_posts_table.go
    └── 20260814000003_create_comments_table.go

routes/
└── api.go                            # new API route group

bootstrap/
├── app.go                            # add WithRouting for api.go
└── migrations.go                     # register 3 new migrations
```

## Request/Response Examples

### Register
```http
POST /api/auth/register
Content-Type: application/json

{
  "name": "John Doe",
  "email": "john@example.com",
  "password": "secret123",
  "password_confirmation": "secret123"
}
```
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": { "id": 1, "name": "John Doe", "email": "john@example.com", "role": "user" }
}
```

### Create post
```http
POST /api/posts
Authorization: Bearer <token>
Content-Type: application/json

{ "title": "My First Post", "content": "Hello world", "published": true }
```
```json
{
  "id": 1, "user_id": 2, "title": "My First Post", "content": "Hello world",
  "published": true, "created_at": "...", "updated_at": "..."
}
```

### Validation error
```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Validation failed",
    "details": [
      {"field": "title", "message": "title is required"},
      {"field": "content", "message": "content is required"}
    ]
  }
}
```


## Security Considerations

1. **Password hashing**: `facades.Hash().Make()` (bcrypt by default)
2. **JWT tokens**: signed with `JWT_SECRET`, expiry configured in `config/jwt.go`
3. **SQL injection**: ORM uses parameterized queries — never raw string concat
4. **Input validation**: all endpoints validate via Goravel validation facade
5. **Ownership checks**: every mutation checks resource ownership before proceeding
6. **Role-based access**: middleware enforces role requirements at route level
7. **No password in responses**: user serialization excludes password field
8. **CORS**: configured via `config/cors.go` (already exists)

## Test Plan

- `TestAuth_Register` — valid register, duplicate email, validation error
- `TestAuth_Login` — valid login, wrong password, non-existent email
- `TestAuth_Me` — valid token, expired token, no token
- `TestPost_CRUD` — create, read, update, delete (happy path)
- `TestPost_Authorization` — user cannot create post, non-owner cannot update, admin can update any
- `TestPost_DeleteCascade` — delete post removes all comments (transaction)
- `TestPost_DraftVisibility` — draft not visible to public, visible to owner/admin
- `TestComment_CRUD` — create, read, update, delete
- `TestComment_Authorization` — non-owner cannot update, post owner can delete, admin can delete any
- `TestValidation` — all endpoints return proper validation errors
- `TestMiddleware_Auth` — protected routes without token return 401
- `TestMiddleware_Role` — user role cannot access author/admin endpoints
- `TestAdmin_ChangeRole` — admin can change role, non-admin cannot

## Implementation Order

1. Migrations (users, posts, comments)
2. Models (User, Post, Comment)
3. Error response helper
4. Auth middleware + role middleware
5. Auth service + controller + routes
6. Post service + controller + routes
7. Comment service + controller + routes
8. Admin user controller + routes
9. Validation request structs
10. Tests
