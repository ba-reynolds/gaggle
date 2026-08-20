# Gaggle Seed Strategy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give Gaggle a deterministic, realistic demo dataset that lands automatically on fresh deploys (local `make dev` and the AWS box) so the site looks lived-in without a manual seeding step, plus a live-activity seam (`cmd/simulate`) that keeps the community growing over time.

**Spec:** `docs/superpowers/specs/2026-08-19-seed-strategy-design.md`

## Global Constraints

- Go module is `github.com/ba-reynolds/gaggle`, `go 1.24.3` (`server/go.mod`). Working dir for Go is `server/`.
- Go on NixOS host: `nix shell nixpkgs#go_1_25 --command bash -c '...'`, always `CGO_ENABLED=0`. Format via `nix shell nixpkgs#gofumpt`.
- Tests: `docker compose --profile tools run --rm tools go test ./...` against the throwaway test DB; local PG at `localhost:6969` (`white`/`teeth`).
- `testDBName()` in `server/internal/testutil/testutil.go` is now a **function** — per-binary `social_test_<hex>` derived from `sha256.Sum256(os.Args[0])[:4]`, so parallel `go test ./...` packages no longer race a fixed `social_test` DB.
- Apply ordering is critical: media rows+files FIRST (user_profiles FKs into `media`), then users+profiles, posts, polls+votes, engagement, relationships, DMs, lists, badges, post-media links last.
- Store gotchas (verified against real store code):
  - `Users.Create` inserts only `(username,email,password)`, requires non-nil tx; `is_admin`/`is_private` NOT writable via store → raw `UPDATE users SET is_admin=TRUE` and `st.Users.SetPrivate(...)` after commit.
  - `pollStore.Vote` requires non-nil tx; `Polls.Create` doesn't return option IDs → look up `poll_options` by position in-tx. Poll question is `VARCHAR(140)` → clamp from content.
  - `Lists.Create` returns only error (sets ID on passed struct).
  - Engagement stores are `ON CONFLICT DO NOTHING`; `genEngagement` emits unique `(kind,post,user)` pairs so dataset counts == DB row counts.
  - Relationships unique key is `UNIQUE(follower_id, following_id, relationship_type)`; Apply dedupes + swallows AlreadyExists.
  - Only `kind='assigned'` badges are grantable → 3 assigned; event-badge lookup keys (`button`,`bench`,`crown`,`static`,`stairs`) seeded at migration.
  - `Media.SaveFile` needs `multipart.File` → Apply writes placeholder PNGs directly; post links via `Media.LinkMediaToPost`.
  - `st.Users.Search(ctx,"",0)` truncates to `Items[:limit]` → a limit of 0 returns an empty list. Pass a real cap.
  - `DMs.ListConversations` never populates `ParticipantA`/`ParticipantB` on items → send DMs as viewer or `OtherParticipant.UserID`.
- Dataset: `SeedValue=20260819`; 8 anchors + 30 fakers = 38 users; 400 top-level + 150 replies backdated 28 days; hashtags ~40%, mentions ~15%; polls ~10%; first 15 posts have media; avatars all users, banners first 4; follows 8–15/user + 2 blocks + 2 mutes; 3 private users; ~10 DM convos; 6 lists; 3 assigned badges.
- Seed is idempotent (anchor guard `alice@example.com`); Tick is intentionally NOT idempotent.

## Task Status

### Task 1: Honor `post.CreatedAt` in the post store — `8d1cee0`
- [x] `server/internal/store/post_store.go` Create / CreateQuotedPost use COALESCE to honor backdated timestamps.

### Task 2: seedgen `Generate` — `54b364f`
- [x] `server/internal/seedgen/seedgen.go`, `content.go`, `generate_test.go`; gofakeit v7.15.0 added.

### Tasks 3–5: seedgen `Apply` — `6e81708`
- [x] `server/internal/seedgen/apply.go` + `apply_test.go`; testutil per-binary DB names. Verified: media-FK ordering (23503), poll-question clamp, Lists.Create 1-return, genEngagement unique pairs, gofumpt.
- [x] Full `go test ./...` green.

### Task 6: Rewrite `cmd/seed` to Generate+Apply — part of `37ff931`
- [x] Anchor-guarded, prints counts + demo accounts; builds.

### Task 7: Auto-seed on boot — part of `37ff931`
- [x] `SEED_ON_START` (default true) in `server/scripts/docker-entrypoint.sh`, `compose.yaml`, `compose.prod.yaml`. Verified end-to-end: wiped DB → `docker compose up -d api` → migrated + seeded + API up.

### Task 8: `cmd/simulate` live-activity seam — part of `37ff931`
- [x] `server/internal/seedgen/tick.go` (Tick: posts w/ hashtags, likes, replies, DMs).
- [x] `server/cmd/simulate/main.go`, `/app/simulate` in `server/Dockerfile`, `make simulate` in `Makefile`.
- [x] Runtime-verified: first run panicked on empty user pool (`Search` limit-0 quirk); fixed with real cap. Second run exposed DM sender bug (`ParticipantA/B` never populated); fixed via viewer/`OtherParticipant`. Both verified inserting rows (posts 550→579, messages 47→49).

## Remaining / Follow-ups

- [ ] User must push `main` (harness blocks `git push`): `git push origin main`.
- [ ] User must SSH to the AWS box and wipe `postgres_data` volume once so the new seed (28-day history) lands on the fresh deploy.
- [ ] (Optional) Wire `cmd/simulate` into a scheduled cron on the EC2 box for continuous growth.
