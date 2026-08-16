# GopherSocial

A Twitter-like social media app: **Go** (chi + PostgreSQL + Redis) backend and a
**React + TypeScript + shadcn/ui** (Vite) frontend. Single repository, two apps.

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
- Consistent `{data, error}` JSON envelope on every endpoint
- Swagger UI at `/swagger` (regenerate with `make swag`)

## Repository layout

```
social-back/   Go API server (handlers -> services -> stores -> PostgreSQL)
social-front/  React + Vite + shadcn/ui client
flake.nix      Nix dev environment (Go, Node, Postgres, Redis)
```

## Quick start (Docker)

```bash
cd social-back
cp .env.example .env
docker compose up -d          # PostgreSQL (6969) + Redis (6379)
make migrate-up               # apply migrations
make seed-db                  # seed demo users (alice@example.com / password123)
make run                      # API on :2021

cd ../social-front
npm install
npm run dev                   # Vite dev server
```

Open http://localhost:5173 and sign in as `alice@example.com` / `password123`.

## Quick start (Nix, no Docker)

```bash
nix develop                    # reproducible toolchain
cd social-back
make dev-services              # local Postgres + Redis in /tmp
make migrate-up && make seed-db
make run

cd ../social-front
npm install && npm run dev
```

## Testing

```bash
# Backend integration tests (needs a local Postgres; creates a throwaway `social_test` DB)
cd social-back && make test

# Frontend
cd social-front && npm run build && npm run lint
```

## API contract

Every response is an envelope:

```json
{ "data": <payload>, "error": null }
{ "data": null, "error": { "code": "NOT_FOUND", "message": "..." } }
```

Paginated feeds return `{ "items": [...], "next_cursor": ..., "has_more": bool }`.
Post objects expose counts and the requesting user's state under `engagement`
(`is_liked`, `is_reposted`, `is_bookmarked`, `like_count`, ...).
See `social-back/docs/swagger.yaml` for the full spec.

## Notes

- Media files are served publicly at `/api/v1/media/{uuid}` (UUIDs are
  unguessable); `<img>` tags can't send auth headers.
- The backend degrades gracefully when Redis is down (no caching / rate limiting).
