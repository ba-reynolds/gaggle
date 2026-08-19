# fuzzy-search-results

## What was changed

Post search (`GET /search?type=posts`) now uses case-insensitive **substring**
matching instead of Postgres full-text (lexeme) matching.

Previously the store claused on
`to_tsvector('simple', p.content) @@ plainto_tsquery('simple', $1)`, which only
matches whole lexemes/words. Searching for `e` therefore never matched a post
containing the word `hey`. It also wouldn't match partial words like `every` →
`everyo`.

Now it matches the existing user-search approach in `user_store.go`:
`p.content ILIKE '%' || $1 || '%' ESCAPE '\'` with the same
`strings.NewReplacer(\→\\, %→\%, _→\_)` escaping so user-supplied wildcards are
treated literally.

## Why

The task: "search results should be generic not strict". A single letter (or
partial word) should surface any post containing it. Full-text `@@` matching
is word-boundary based and can never do substring matching, so it was replaced
with `ILIKE`.

## Files touched

- `server/internal/store/post_store.go` — `Search()`: swapped the full-text
  clause for the escaped `ILIKE` substring clause.
- `server/internal/handlers/integration_test.go` — added
  `TestSearchSubstringMatch` (single-letter `e` → `hey`, partial word
  `hey every` → `hey everyone`).
- `.opencode/project-notes.md` — recorded the behavior change.

## Verification

- `go test ./internal/handlers/ -run TestSearchSubstringMatch` fails on the
  old clause (0 results), passes after the fix.
- `go test ./...` all pass (including `TestSearchFilters`, `TestSearchHashtagsAndTrends`).
- `gofmt` and `go vet` clean.

## Reviewer double-checks

- The GIN index `posts_content_search_idx` (migration `000011_create-search`,
  `to_tsvector('simple', content)`) is now **unused** by the search path. It was
  left in place deliberately — it only costs write overhead, is harmless to
  reads, and removing it would require a new (parallel-branch collision-prone)
  migration. Worth deciding separately if the index should be dropped.
- Behavior is intentionally looser than before: `q=kayak` still matches all the
  same test posts (substring ⊇ full word), but now also matches partial/single
  chars and is case-insensitive (`ILIKE`). This is the requested "generic"
  behavior.