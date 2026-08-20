# Demo-Rich Bookmarks + 4× Seed Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Seed 7 usable bookmark categories per user (only categories with ≥1 bookmark are created), leave ~10% bookmarks uncategorized, make `Uncategorized` a selectable filter alongside real categories, remove the forced `General` default, and grow the dataset 4×; support wipe & re-seed for live.

**Architecture:** `seedgen` stays deterministic (SeedValue 20260819). New pool of 7 named categories in `seedgen.go`; `genBookmarkCategories` + `genEngagement` in `content.go` assign bookmarks to category names with ~10% nil and prune unused names before Apply; `apply.go` gains `applyBookmarkCategories` that creates only used categories and resolves `Bookmark(..., *int)` vs nil. Backend `GetBookmarkedPostsFeed` gains `includeUncategorized bool` producing `AND (pb.category_id IS NULL OR pb.category_id IN (...))` union logic; frontend `BookmarksPage` adds an `Uncategorized` badge that composes with real category filters. A migration drops the `create_default_bookmark_category` trigger. Tests loosen exact counts to 4× targets.

**Tech Stack:** Go 1.24 (`github.com/ba-reynolds/gaggle`, `server/go.mod`), Postgres, `brianvoe/gofakeit/v7`, React/Vite (`web/src`), Docker Compose.

**Spec:** This conversation: 7 categories only-if-used (2026-08-20 thread), 10% uncategorized, Uncategorized filter selectable with categories, no forced General, 4× scale (150 users / 1600 top-level + 600 replies etc.), wipe & re-seed.

## Global Constraints

- Go module is `github.com/ba-reynolds/gaggle`, `go 1.24.3` (`server/go.mod`). Backend work in `server/`.
- Go on NixOS: `nix shell nixpkgs#go_1_25 --command bash -c '...'` with `CGO_ENABLED=0`; format via `gofumpt`.
- Tests run via `docker compose --profile tools run --rm tools go test ./...` against throwaway DB (`testDBName()` per-binary `social_test_<hex>`).
- `post_bookmarks.category_id` stays nullable (`ON DELETE SET NULL`, migration `000007_create-post-engagement.up.sql:40`).
- `bookmark_categories` unique is `UNIQUE(user_id, category_name)` (`000007:32`), `category_name VARCHAR(50)`.
- `Generate(faker, now)` deterministic; ordering in Apply is media-first, then users, then categories, then posts, then engagement.
- `BookmarkRequest.CategoryID *int` optional remains nullable; `post_engagement_store.go:89` Bookmark validates only if non-nil.
- `General` trigger at `000007:194` / `203` is to be dropped, not kept.

---

## File Structure

- Modify `server/internal/seedgen/seedgen.go` — scale constants, `GenBookmarkCategory`/pool, `GenEngagement.CategoryName *string`, `Dataset.BookmarkCategoryNames` + debug `Dataset.BookmarkCategoryDBG` optional
- Modify `server/internal/seedgen/content.go` — `genBookmarkCategories()`, `genEngagement` category assignment + 10% nil + prune + alice guarantee
- Modify `server/internal/seedgen/apply.go` — `applyBookmarkCategories()` after `applyUsers`, wire DB ids into `applyEngagement`
- Create `server/cmd/migrate/migrations/0000NN_remove_default_bookmark_category_trigger.up.sql` (+ .down) — drop trigger/function
- Modify `server/internal/store/post_store.go:1545` — `GetBookmarkedPostsFeed` add `includeUncategorized bool` with union SQL
- Modify `server/internal/store/post_engagement_store.go:480` — add `GetUncategorizedBookmarkCount` helper (or extend List response)
- Modify `server/internal/service/post_service.go:982` and `server/internal/handlers/post_handler.go:900` — plumb `includeUncategorized` param (`include_uncategorized=true`)
- Modify `web/src/api/posts.ts:115` and `web/src/hooks/usePost.ts:493` — `getBookmarkedPosts` / `useGetBookmarkedPosts` carry `includeUncategorized`
- Modify `web/src/pages/BookmarksPage.tsx:17,42,74` — `Uncategorized` badge state + composition with `selectedCategories`, display `uncategorizedCount`
- Modify `server/internal/seedgen/generate_test.go` and `server/internal/seedgen/apply_test.go` — update scale assertions for 4×, add category-only-if-used + uncategorized filter tests
- Modify `server/cmd/seed/main.go:41` — add `-force` flag to allow wipe & re-seed (TRUNCATE or drop) despite alice guard

---

### Task 1: 4× Scale Constants + Category Model

**Files:**
- Modify: `server/internal/seedgen/seedgen.go:23-44`
- Modify: `server/internal/seedgen/seedgen.go:90-167` (types + Dataset)

**Interfaces:**
- Consumes: existing `SeedValue`, `TotalUsers = AnchorUsers+FakerUsers`, `DaysOfHistory` etc.
- Produces: new exported scale constants, `bookmarkCategoryPool []struct{Name,Color string}`, `type GenEngagement { PostIdx, UserIdx int; CategoryName *string }`, `Dataset.BookmarkCategoryNames [][]string`, `Dataset.BookmarkCategoryAssigned map[int]map[string]int` or `UserCategoryIDs [][]int` (implementer picks one, documented in report)

- [ ] **Step 1: Write failing scale test**

```go
// in server/internal/seedgen/generate_test.go add or adapt:
func TestGenerateScale_4x(t *testing.T) {
    ds := gDataset()
    if len(ds.Users) != 150 {
        t.Fatalf("users = %d, want 150", len(ds.Users))
    }
    var top int
    for _, p := range ds.Posts { if p.ParentIdx == -1 { top++ } }
    if top != 1600 {
        t.Fatalf("top-level = %d, want 1600", top)
    }
    if len(ds.Posts) != 2200 {
        t.Fatalf("posts = %d, want 2200", len(ds.Posts))
    }
}
```

Run: `nix shell nixpkgs#go_1_25 --command bash -c 'CGO_ENABLED=0 go test ./internal/seedgen -run TestGenerateScale_4x -v'`
Expected: FAIL (counts still 38/400/550)

- [ ] **Step 2: Bump constants in `seedgen.go`**

```go
const (
    SeedValue = 20260819
    AnchorUsers = 8
    FakerUsers  = 142 // was 30 → TotalUsers 150
    TotalUsers  = AnchorUsers + FakerUsers
    DaysOfHistory = 28
    TopLevelPosts = 1600 // was 400
    ReplyPosts    = 600  // was 150
    DMConversations = 40 // was 10
    Lists         = 24   // was 6
    AssignedBadges = 3
    MediaPosts    = 60   // was 15

    FollowMin = 8
    FollowMax = 15
    LikeMin, LikeMax         = 0, 40 // was 0,25
    RepostMin, RepostMax     = 0, 10 // was 0,8
    BookmarkMin, BookmarkMax = 0, 16 // was 0,12
    PollVoteMin, PollVoteMax = 0, 12
)
```

- [ ] **Step 3: Add category pool and Dataset fields**

```go
// at bottom of seedgen.go type section, near GenEngagement
var bookmarkCategoryPool = []struct{Name, Color string}{
    {"Tech", "#0ea5e9"},
    {"Inspiration", "#a855f7"},
    {"Reading List", "#f59e0b"},
    {"Research", "#06b6d4"},
    {"Work", "#6366f1"},
    {"Ideas", "#ec4899"},
    {"Watch Later", "#22c55e"},
}

// GenEngagement now carries optional category assignment by name.
type GenEngagement struct {
    PostIdx      int
    UserIdx      int
    CategoryName *string // nil = uncategorized (~10%)
}

// In Dataset struct after Bookmarks:
BookmarkCategoryNames [][]string `json:"-"` // per-user pool pick (pruned to used only before Apply)
// Alternative: UserBookmarkCategoryIDs populated in Apply — document choice.
```

- [ ] **Step 4: Verify scale**

Run: `nix shell nixpkgs#go_1_25 --command bash -c 'CGO_ENABLED=0 go test ./internal/seedgen -run TestGenerateScale_4x -v'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add server/internal/seedgen/seedgen.go server/internal/seedgen/generate_test.go
git commit -m "seed: 4x scale constants and bookmark category model"
```

---

### Task 2: Generate Categories Only-If-Used + 10% Uncategorized

**Files:**
- Modify: `server/internal/seedgen/content.go:126-166` (genUsers private indices)
- Modify: `server/internal/seedgen/content.go:350-408` (genEngagement)
- Modify: `server/internal/seedgen/seedgen.go:179-197` (Generate ordering)

**Interfaces:**
- Consumes: `bookmarkCategoryPool`, `Dataset.BookmarkCategoryNames`, new `GenEngagement.CategoryName`
- Produces: `Dataset.BookmarkCategoryNames` populated then pruned to only categories with ≥1 bookmark; `Dataset.Bookmarks` with ~10% nil and alice ≥2 per remaining category

- [ ] **Step 1: Update genUsers private indices for 142 fakers**

```go
// in genUsers, replace:
// if i == 7 || i == 19 {
// with proportional:
if i == 7 || i == 35 || i == 90 {
    user.IsPrivate = true
}
```

- [ ] **Step 2: Add genBookmarkCategories and wire into Generate**

```go
func (g *generator) genBookmarkCategories() {
    g.ds.BookmarkCategoryNames = make([][]string, len(g.ds.Users))
    for u := range g.ds.Users {
        // deterministically shuffle pool via g.f and take 5-7
        pool := append([]struct{Name,Color string}(nil), bookmarkCategoryPool...)
        g.f.ShuffleAnySlice(pool)
        n := g.f.Number(5, len(pool)) // 5-7
        names := make([]string, n)
        for i := 0; i < n; i++ { names[i] = pool[i].Name }
        g.ds.BookmarkCategoryNames[u] = names
    }
}
// In Generate, after genUsers and before genPosts:
g.genBookmarkCategories()
```

- [ ] **Step 3: Extend genEngagement to assign categories with 10% nil and prune unused**

```go
func (g *generator) genEngagement() {
    seen := make(map[[3]int]bool)
    used := make(map[int]map[string]bool) // userIdx -> name -> used
    for i := range g.ds.Users { used[i] = make(map[string]bool) }

    addEngagement := func(kind string, postIdx, userIdx int, cat *string) {
        key := [3]int{kindIdx(kind), postIdx, userIdx}
        if seen[key] { return }
        seen[key] = true
        switch kind {
        case "like":
            g.ds.Likes = append(g.ds.Likes, GenEngagement{PostIdx: postIdx, UserIdx: userIdx})
        case "repost":
            g.ds.Reposts = append(g.ds.Reposts, GenEngagement{PostIdx: postIdx, UserIdx: userIdx})
        case "bookmark":
            g.ds.Bookmarks = append(g.ds.Bookmarks, GenEngagement{PostIdx: postIdx, UserIdx: userIdx, CategoryName: cat})
            if cat != nil { used[userIdx][*cat] = true }
        }
    }

    for i := range g.ds.Posts {
        p := &g.ds.Posts[i]
        for l := 0; l < g.f.Number(LikeMin, LikeMax); l++ {
            u := g.f.Number(0, TotalUsers-1)
            if u == p.AuthorIdx { continue }
            addEngagement("like", i, u, nil)
        }
        if p.ParentIdx == -1 {
            for r := 0; r < g.f.Number(RepostMin, RepostMax); r++ {
                u := g.f.Number(0, TotalUsers-1)
                if u == p.AuthorIdx { continue }
                addEngagement("repost", i, u, nil)
            }
        }
        for b := 0; b < g.f.Number(BookmarkMin, BookmarkMax); b++ {
            u := g.f.Number(0, TotalUsers-1)
            if u == p.AuthorIdx { continue }
            if g.f.Number(1, 100) <= 10 {
                addEngagement("bookmark", i, u, nil)
                continue
            }
            names := g.ds.BookmarkCategoryNames[u]
            if len(names) == 0 { addEngagement("bookmark", i, u, nil); continue }
            name := names[g.f.Number(0, len(names)-1)]
            n := name
            addEngagement("bookmark", i, u, &n)
        }
    }
    // prune unused category names per user
    for u, m := range used {
        var kept []string
        for _, n := range g.ds.BookmarkCategoryNames[u] {
            if m[n] { kept = append(kept, n) }
        }
        // keep at least 1 if user has bookmarks but pruning emptied (rare)
        if len(kept) == 0 && len(used[u]) == 0 {
            // no bookmarks for this user — leave BookmarkCategoryNames empty (no categories created)
        }
        g.ds.BookmarkCategoryNames[u] = kept
    }
    // alice guarantee: ensure alice (0) has ≥2 bookmarks per kept category
    if len(g.ds.BookmarkCategoryNames[0]) > 0 {
        countByCat := map[string]int{}
        for _, bm := range g.ds.Bookmarks { if bm.UserIdx == 0 && bm.CategoryName != nil { countByCat[*bm.CategoryName]++ } }
        for _, name := range g.ds.BookmarkCategoryNames[0] {
            for countByCat[name] < 2 {
                // find a random post not authored by alice and not already bookmarked by alice
                // simplified: pick first eligible post idx
                // ... (implementer fills exact loop)
            }
        }
    }
}
```

- [ ] **Step 4: Expand genLists defs for 24 lists**

```go
// In genLists, replace static 6 defs with 24: keep 6 named plus generate remainder via faker:
defs := []struct{owner int; name, desc string}{
    {0, "Engineering reads", "Articles worth your time"},
    {1, "Design inspiration", ""},
    {3, "Travel bucket list", "Places on my radar"},
    {5, "Music to work to", "Focus playlist"},
    {8, "Startup crew", "Founders and builders"},
    {12, "Book club 2026", "This year's picks"},
}
for i := len(defs); i < Lists; i++ {
    defs = append(defs, struct{owner int; name, desc string}{owner: g.f.Number(0, TotalUsers-1), name: g.f.Sentence(2), desc: g.f.Sentence(5)})
}
```

- [ ] **Step 5: Commit**

```bash
git add server/internal/seedgen/content.go server/internal/seedgen/seedgen.go
git commit -m "seed: generate only-if-used bookmark categories with 10% uncategorized"
```

---

### Task 3: Apply Categories (Create Only Used) + Wire Bookmarks + Drop General Trigger

**Files:**
- Modify: `server/internal/seedgen/apply.go:39-71,283-301`
- Create: `server/cmd/migrate/migrations/0000NN_remove_default_bookmark_category_trigger.up.sql`
- Create: `server/cmd/migrate/migrations/0000NN_remove_default_bookmark_category_trigger.down.sql`

**Interfaces:**
- Consumes: `Dataset.BookmarkCategoryNames` (pruned), `Dataset.Bookmarks` with `CategoryName`
- Produces: DB `bookmark_categories` rows for used categories only; `Dataset.userCatDBIDs map[int]map[string]int` used by `applyEngagement` to call `Bookmark(..., *int)` vs nil; migration drops `create_default_bookmark_category_trigger` and `create_default_bookmark_category()` from `000007`

- [ ] **Step 1: Create migration**

```sql
-- 0000NN_remove_default_bookmark_category_trigger.up.sql
DROP TRIGGER IF EXISTS create_default_bookmark_category_trigger ON user_profiles;
DROP FUNCTION IF EXISTS create_default_bookmark_category();
-- down: recreate trigger (copy from 000007:194-207)
```

Implementer must run `make new-migration` or pick next free version (check `git ls-tree -r --name-only HEAD server/cmd/migrate/migrations/ | sort`). Today HEAD max is 000023? Use 000024 if free; verify `sdk ls`.

- [ ] **Step 2: Add applyBookmarkCategories in apply.go**

```go
func applyBookmarkCategories(ctx context.Context, st *store.Store, log *slog.Logger, ds *Dataset) (map[int]map[string]int, error) {
    colorByName := map[string]string{}
    for _, c := range bookmarkCategoryPool { colorByName[c.Name] = c.Color }
    out := make(map[int]map[string]int, len(ds.Users))
    for uIdx, names := range ds.BookmarkCategoryNames {
        if len(names) == 0 { continue }
        uid := ds.UserIDs[uIdx]
        m := make(map[string]int, len(names))
        for _, name := range names {
            color := colorByName[name]
            if color == "" { color = "#1DA1F2" }
            cat, err := st.PostEngagements.CreateBookmarkCategory(ctx, nil, uid, name, color)
            if err != nil {
                // AlreadyExists from old General row on re-seed without migration — treat as lookup
                // need helper: if AlreadyExists, SELECT category_id WHERE user_id=$1 AND category_name=$2
                // implementer adds that query
            }
            m[name] = cat.CategoryID
        }
        out[uIdx] = m
    }
    return out, nil
}
```

Call it in `Apply` after `applyUsers` and before `applyEngagement`, thread `userCatIDs` into `applyEngagement`.

- [ ] **Step 3: Update applyEngagement signature**

```go
func applyEngagement(ctx context.Context, st *store.Store, log *slog.Logger, ds *Dataset, userCatIDs map[int]map[string]int) error {
    // ...
    for _, e := range ds.Bookmarks {
        var catID *int
        if e.CategoryName != nil {
            if m, ok := userCatIDs[e.UserIdx]; ok {
                if id, ok := m[*e.CategoryName]; ok {
                    v := id
                    catID = &v
                }
            }
        }
        if err := st.PostEngagements.Bookmark(ctx, nil, ds.PostIDs[e.PostIdx], ds.UserIDs[e.UserIdx], catID); err != nil {
            return fmt.Errorf("bookmark post %d by user %d: %w", e.PostIdx, e.UserIdx, err)
        }
    }
    return nil
}
```

- [ ] **Step 4: Commit**

```bash
git add server/cmd/migrate/migrations/0000NN* server/internal/seedgen/apply.go
git commit -m "seed: apply only-if-used bookmark categories and wire bookmarks"
```

---

### Task 4: Backend Uncategorized Filter

**Files:**
- Modify: `server/internal/store/post_store.go:1545-1585`
- Modify: `server/internal/store/post_engagement_store.go:480` (add `GetUncategorizedBookmarkCount`)
- Modify: `server/internal/service/post_service.go:982`
- Modify: `server/internal/handlers/post_handler.go:900-923`
- Modify: `server/docs/swagger.yaml` if present (optional)

**Interfaces:**
- Consumes: `categoryIDs []int`, `includeUncategorized bool`
- Produces: `GetBookmarkedPostsFeed(ctx, userID, categoryIDs, includeUncategorized, limit, cursor) (*models.PostFeed, error)` returning union correctly; `ListBookmarkCategories` HTTP response extended with `uncategorizedCount` or separate field; handler parses `include_uncategorized=true`

- [ ] **Step 1: Update post_store.go**

```go
func (store *postStore) GetBookmarkedPostsFeed(ctx context.Context, userID int, categoryIDs []int, includeUncategorized bool, limit int, cursor string) (*models.PostFeed, error) {
    // baseQuery: SELECT p.post_id, pb.created_at FROM post_bookmarks pb JOIN posts p ON pb.post_id = p.post_id WHERE pb.user_id = $1 AND p.soft_deleted = FALSE
    // then:
    hasCats := len(categoryIDs) > 0
    if includeUncategorized && hasCats {
        placeholders := make([]string, len(categoryIDs))
        for i, id := range categoryIDs { placeholders[i] = fmt.Sprintf("$%d", argIdx); args = append(args, id); argIdx++ }
        baseQuery += fmt.Sprintf(" AND (pb.category_id IS NULL OR pb.category_id IN (%s))", strings.Join(placeholders, ","))
    } else if includeUncategorized {
        baseQuery += " AND pb.category_id IS NULL"
    } else if hasCats {
        placeholders := ...
        baseQuery += fmt.Sprintf(" AND pb.category_id IN (%s)", strings.Join(placeholders, ","))
    }
    // rest: cursor, ORDER BY, LIMIT
}
```

- [ ] **Step 2: Add count helper in post_engagement_store.go**

```go
func (store *postEngagementStore) GetUncategorizedBookmarkCount(ctx context.Context, userID int) (int, error) {
    var n int
    err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM post_bookmarks WHERE user_id = $1 AND category_id IS NULL`, userID).Scan(&n)
    return n, err
}
```

Update `ListBookmarkCategories` service to fetch it and include in API response (either extend `BookmarkCategory` shim or return envelope `{categories, uncategorized_count}` — pick one and keep frontend consistent).

- [ ] **Step 3: Update service + handler**

```go
// post_service.go
func (s *PostService) GetBookmarkedPostsFeed(ctx context.Context, userID int, viewerID int, categoryIDs []int, includeUncategorized bool, limit int, cursor string) (*models.PostFeed, error) {

// post_handler.go
func (h *PostHandler) BookmarkedPostsFeed(w http.ResponseWriter, r *http.Request) {
    // existing category_ids parse
    includeUncategorized := r.URL.Query().Get("include_uncategorized") == "true"
    feed, err := h.service.Posts.GetBookmarkedPostsFeed(r.Context(), user.ID, user.ID, categoryIDs, includeUncategorized, limit, cursor)
}
```

- [ ] **Step 4: Commit**

```bash
git add server/internal/store/post_store.go server/internal/store/post_engagement_store.go server/internal/service/post_service.go server/internal/handlers/post_handler.go
git commit -m "bookmarks: backend Uncategorized filter (composable with category_ids)"
```

---

### Task 5: Frontend Uncategorized as Regular Category

**Files:**
- Modify: `web/src/api/posts.ts:115`
- Modify: `web/src/hooks/usePost.ts:493`
- Modify: `web/src/pages/BookmarksPage.tsx:17,42,74`
- Modify: `server/docs/swagger.yaml` example if needed (no-op)

**Interfaces:**
- Consumes: `includeUncategorized bool` from handler, `uncategorizedCount` from categories endpoint
- Produces: `BookmarksPage` renders `Uncategorized` `<Badge>` alongside real categories; selecting `Uncategorized` + `Funny` requests union; filter state reflects in `useGetBookmarkedPosts` queryKey

- [ ] **Step 1: Update api/posts.ts**

```typescript
export const getBookmarkedPosts = async (
  categoryIds?: number[],
  includeUncategorized?: boolean,
  cursor?: string,
  limit?: number,
): Promise<Envelope<PaginatedFeedResponse>> => {
  const params = new URLSearchParams();
  if (categoryIds?.length) params.set("category_ids", categoryIds.join(","));
  if (includeUncategorized) params.set("include_uncategorized", "true");
  if (cursor) params.set("cursor", cursor);
  if (limit) params.set("limit", String(limit));
  const { data } = await api.get<Envelope<PaginatedFeedResponse>>(`/posts/bookmarks?${params.toString()}`);
  return data;
};

export const getBookmarkCategories = async (): Promise<{ categories: BookmarkCategory[]; uncategorizedCount: number } | BookmarkCategory[]> => {
  // if backend returns envelope with uncategorized_count, unwrap; else fallback
};
```

If backend keeps categories array shape, add separate `getUncategorizedBookmarkCount` or extend categories fetch to return `{categories, uncategorized_count}`.

- [ ] **Step 2: Update hooks/usePost.ts**

```typescript
export function useGetBookmarkedPosts(categoryIds?: number[], includeUncategorized?: boolean, limit: number = 10) {
  return useInfiniteQuery({
    queryKey: ['bookmarked', categoryIds?.length ? categoryIds.join(',') : 'all', includeUncategorized ? 'uncat' : ''],
    queryFn: ({ pageParam }) => getBookmarkedPosts(categoryIds, includeUncategorized, pageParam, limit),
    // ...
  });
}
export function useGetBookmarkCategories() {
  // unwrap uncategorizedCount if present
}
```

- [ ] **Step 3: Update BookmarksPage.tsx**

```tsx
const [selectedCategories, setSelectedCategories] = useState<number[]>([]);
const [includeUncategorized, setIncludeUncategorized] = useState(false);
const { data: categoriesPayload } = useGetBookmarkCategories();
const categories = Array.isArray(categoriesPayload?.data) ? categoriesPayload.data : (categoriesPayload?.data as any)?.categories ?? [];
const uncategorizedCount = (categoriesPayload?.data as any)?.uncategorized_count ?? 0;

const { data: bookmarkedPosts } = useGetBookmarkedPosts(
  selectedCategories.length ? selectedCategories : undefined,
  includeUncategorized,
);

// category chips:
<Badge
  variant={includeUncategorized ? "default" : "secondary"}
  onClick={() => setIncludeUncategorized(v => !v)}
>
  <span>Uncategorized</span><span>{uncategorizedCount}</span>
</Badge>
{categories.map(c => (
  <Badge variant={selectedCategories.includes(c.id) ? "default":"secondary"} onClick={()=>toggleCategory(c.id)}>
    <span>{c.name}</span><span>{c.post_count}</span>
  </Badge>
))}
```

This makes `Uncategorized` + `Funny` a union (`include_uncategorized=true & category_ids=<funnyId>` → `IS NULL OR IN (...)`).

- [ ] **Step 4: Commit**

```bash
git add web/src/api/posts.ts web/src/hooks/usePost.ts web/src/pages/BookmarksPage.tsx
git commit -m "bookmarks: frontend Uncategorized filter composable with categories"
```

---

### Task 6: Wipe & Re-seed + Tests

**Files:**
- Modify: `server/cmd/seed/main.go:41` (add `-force` / `SEED_FORCE` env)
- Modify: `server/internal/seedgen/generate_test.go:30-101`
- Modify: `server/internal/seedgen/apply_test.go:33-329`
- Modify: `Makefile` or `scripts/reset-db.sh` (document wipe recipe)

**Interfaces:**
- Consumes: all prior tasks
- Produces: `go test ./internal/seedgen` green at 150/1600/600 scale; `make seed` with guard bypass works for wipe & re-seed; docs note `SEED_FORCE=1`

- [ ] **Step 1: Allow force re-seed**

```go
// server/cmd/seed/main.go
force := false
for _, a := range os.Args { if a == "--force" || a == "-force" { force = true } }
if os.Getenv("SEED_FORCE") == "1" { force = true }
if !force {
    if _, err := st.Users.GetByEmail(ctx, "alice@example.com"); err == nil {
        fmt.Println("✅ Database already seeded (alice@example.com exists); skipping. Use --force or SEED_FORCE=1 to re-seed.")
        return
    }
} else {
    // truncate in FK order or DROP + migrate path: simplest is to truncate all seed tables
    // or call store helper; implementer writes explicit TRUNCATE ... CASCADE for seed-owned tables
    // and removes media dir files, then fall through to Generate+Apply
}
```

- [ ] **Step 2: Update generate_test.go assertions**

```go
func TestGenerateScale(t *testing.T) {
    ds := gDataset()
    if len(ds.Users) != 150 { t.Errorf("users = %d, want 150", len(ds.Users)) }
    // etc: topLevel 1600, posts 2200, lists 24, media 60
}
func TestGenerateUserUniquenessAndConstraints(t *testing.T) { // keep ≤16 check
}
func TestGeneratePostConstraints(t *testing.T) { // topLevel 1600
}
func TestGenerateDMAndLists(t *testing.T) { if len(ds.DMConversations) < 40 { ... } if len(ds.Lists) < 24 { ... }}
func TestGenerateBookmarkCategoriesUsedOnly(t *testing.T) {
    ds := gDataset()
    // every name in BookmarkCategoryNames must appear in Bookmarks for that user
    // uncategorized ratio ~10%
}
```

- [ ] **Step 3: Update apply_test.go**

```go
func TestApplyUsersAndProfiles(t *testing.T) { if len(ds.UserIDs) != 150 { ... } if profiles != 150 { ... } }
func TestApplyPostsBackdatedAndSynced(t *testing.T) { if topLevel != 1600 { ... } if replies != 600 { ... }}
func TestApplyBookmarkCategoriesUsedOnly(t *testing.T) {
    st, ds, _ := seedEngine(t)
    var zeroCount int
    st.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM bookmark_categories c WHERE NOT EXISTS (SELECT 1 FROM post_bookmarks b WHERE b.user_id=c.user_id AND b.category_id=c.category_id)`).Scan(&zeroCount)
    if zeroCount != 0 { t.Errorf("found %d empty bookmark categories; only-if-used violated", zeroCount) }
    var uncategorized, total int
    st.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM post_bookmarks WHERE category_id IS NULL`).Scan(&uncategorized)
    st.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM post_bookmarks`).Scan(&total)
    ratio := float64(uncategorized)/float64(total)
    if ratio < 0.05 || ratio > 0.15 { t.Errorf("uncategorized ratio %.2f outside 5-15%%", ratio) }
}
func TestApplyBookmarkedFeedUncategorized(t *testing.T) {
    st, ds, _ := seedEngine(t)
    // pick alice's id, fetch feed with includeUncategorized=true + categoryIDs union and verify mix
}
```

- [ ] **Step 4: Verify 4× seed end-to-end**

```bash
nix shell nixpkgs#go_1_25 --command bash -c 'CGO_ENABLED=0 go test ./internal/seedgen -v'
docker compose --profile tools run --rm tools go test ./...  # or per-package if slow
# manual: DB_PORT=6971 REDIS_PORT=6381 make proj-dev && SEED_FORCE=1 make seed
psql -h localhost -p 6971 -U white -d social -c "SELECT COUNT(*) FROM bookmark_categories; SELECT COUNT(*) FILTER (WHERE category_id IS NULL) FROM post_bookmarks;"
curl -s -b /tmp/jar http://localhost:<api-port>/api/v1/bookmarks/category | jq
curl -s -b /tmp/jar "http://localhost:<api-port>/api/v1/posts/bookmarks?include_uncategorized=true&category_ids=1" | jq .data.items | head
```

- [ ] **Step 5: Commit**

```bash
git add server/cmd/seed/main.go server/internal/seedgen/generate_test.go server/internal/seedgen/apply_test.go
git commit -m "seed: force re-seed and 4x test updates"
```

---

## Verification Checklist (before merge)

- [ ] `go test ./internal/seedgen -v` — deterministic, 150 users / 1600+600 posts / no empty categories / ~10% uncategorized
- [ ] `GET /bookmarks/category` as alice shows 5-7 categories with non-zero `post_count` and `uncategorized_count` >0
- [ ] `GET /posts/bookmarks?include_uncategorized=true` returns only uncategorized; with `&category_ids=<id>` returns union
- [ ] `BookmarksPage` badge `Uncategorized (N)` toggles alongside `Tech` etc.; selecting both shows union feed
- [ ] Fresh DB via `SEED_FORCE=1` / wipe & re-seed lands 2200 posts and 5-7 used categories per active bookmarker; old `General` empty rows gone (trigger dropped)
