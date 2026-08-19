# Gaggle

A Twitter-like social media app: **Go** (chi + PostgreSQL + Redis) backend and a
**React + TypeScript + shadcn/ui** (Vite) frontend, served by **nginx**. Single
repository, two apps, fully containerized with Docker Compose.

## Features

- JWT auth (access + refresh tokens, refresh-token rotation via HttpOnly cookie)
- Posts with text + up to 4 images, replies, quotes, reposts
- Likes, reposts, quotes, bookmarks (with per-user bookmark categories)
- Home feed (following), user feeds, liked / bookmarked feeds
- Feed of users who **liked / reposted** a post, and **quotes** of a post
- Profiles with banner/avatar upload, follower/following counts, follow/block
- User settings (notifications, privacy, appearance, language)
- Cursor-based (keyset) pagination on every feed
- Redis: home-feed caching + auth rate limiting (degrades gracefully if absent)
- In-app notifications for likes, reposts, quotes, replies, follows, and mentions
- Live notification/feed events over cookie-authenticated Server-Sent Events (SSE)
- Full-text post and user search, clickable hashtags, hashtag feeds, and trends
- Post power features: edit with history, pin to profile (one per author),
  cascade delete (post + all replies), and opinion polls (top-level only,
  2-4 options, one vote per user)
- Profile badges: auto-earned milestone badges (account age, posts, followers,
  likes received) plus admin-assigned badges managed from an admin UI
- Admin area at `/admin` (only for admins; `alice` is the seeded admin)
- Explore page at `/explore`: search, trending hashtags, and suggested users
  ("Who to follow" in the sidebar is backed by the same suggested endpoint)
- User-managed lists at `/lists`: public, owner-curated collections of users
  with their own aggregated feed
- Direct messages at `/messages`: 1:1 private conversations delivered over the
  SSE stream with unread badges; blocked users cannot message
- Consistent `{data, error}` JSON envelope on every endpoint
- Swagger UI at `/swagger` (regenerate with `make swag`)

## Repository layout

```
server/    Go API server (handlers -> services -> stores -> PostgreSQL) + Dockerfile
web/       React + Vite + shadcn/ui client + nginx reverse proxy (Dockerfile)
compose.yaml  One-shot local stack: Postgres + Redis + API + Web
```

## Quick start (Docker)

**Prerequisite:** Docker Engine running (`docker --version`). On NixOS that's
`virtualisation.docker.enable = true` + `nixos-rebuild switch` + reboot.

```bash
make dev       # build + start db, redis, api, and web
make seed      # create demo users (idempotent)
```

Open **http://localhost:5173** and sign in as `alice@example.com` / `password123`.

- The `web` container (nginx on host `:5173`) serves the frontend and
  reverse-proxies `/api/*` and `/swagger/*` to the `api` container — the whole
  app runs on a single origin (no CORS).
- The `api` container **auto-applies migrations** on first start and seeds
  nothing by itself; run `make seed` once for demo data.

### Common commands

| Command | What it does |
|---------|--------------|
| `make dev` | build + run the full stack in the background |
| `make dev-logs` | stream all container logs |
| `make dev-stop` | stop the stack (data volumes persist) |
| `make seed` | create demo users (idempotent) |
| `make test` | backend integration tests (throwaway `social_test` DB) |
| `make swag` | regenerate Swagger docs into `server/docs` |
| `make lint-frontend`, `make test-frontend` | frontend checks |
| `make reset-db` | drop + recreate the app database |

No Go/Node toolchain is needed on the host — everything runs through the
`tools` / `web-tools` compose services (a `tools` profile not started by
`make dev`).

Notifications are available at `/notifications`; the SSE stream is exposed at
`/api/v1/stream` and is consumed automatically by the frontend. The stream
uses the same-origin HttpOnly refresh cookie, so no token is placed in URLs.

## Configuration

Copy `.env.example` → `.env` to override compose variables (DB user/password,
ports, JWT secret). Sensible defaults work out of the box. The API container's
full config lives in `compose.yaml` under the `api` service `environment`.

## API contract

Every response is an envelope:

```json
{ "data": <payload>, "error": null }
{ "data": null, "error": { "code": "NOT_FOUND", "message": "..." } }
```

Paginated feeds return `{ "items": [...], "next_cursor": ..., "has_more": bool }`.
Post objects expose counts and the requesting user's state under `engagement`
(`is_liked`, `is_reposted`, `is_bookmarked`, `like_count`, ...).
See `server/docs/swagger.yaml` for the full spec.

## Notes

- Media files are served publicly at `/api/v1/media/{uuid}` (UUIDs are
  unguessable); `<img>` tags can't send auth headers.
- The backend degrades gracefully when Redis is down (no caching / rate limiting).
- Migrations: applied automatically by the api container on start; manage
  manually with `make migrate-up` / `make new-migration <name>`.
