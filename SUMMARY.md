# instant-tabs — Instant sidebar tab navigation

## What was changed
- **Fix full-screen flash on tab click** (`web/src/App.tsx`): Removed top-level `<Suspense fallback={<AppSplash />}>` that wrapped all `<Routes>` and replaced it with per-route `<Suspense fallback={<RouteFallback />}>` (small centered spinner). Clicking a sidebar tab no longer replaces the entire app (sidebar + right rail) with a `min-h-screen` splash while the lazy chunk downloads; the layout stays mounted and only the center column shows a `min-h-[50vh]` fallback. `RouteFallback` is a tiny inline component.
- **Hover/focus + idle chunk prefetch** (`web/src/layout/SocialMediaLayout.tsx`): `NavItem` now accepts optional `preload` prop and triggers it on `onMouseEnter`/`onFocus`. Each sidebar link preloads its page chunk (`() => void import('@/pages/...')`) so the chunk is already cached before the click. `Messages`/`Notifications`/`Explore` also warm data (`dm-conversations`, `notificationsInfiniteOptions`, `trends`) via `queryClient` on hover. An idle warm (`requestIdleCallback` or 2s timeout fallback) prefetches `Explore`/`Bookmarks`/`Search` after auth. Extended the existing auth-warm `useEffect` to also `prefetchQuery(['trends'])`.
- Imports updated (`web/src/layout/SocialMediaLayout.tsx`): `getTrends`.

## Why
Cold first click to any tab not yet visited this session hit two sequential waterfalls: chunk download (global Suspense → full AppSplash flash, 200-600ms on 3G) then `useQuery` network (page skeleton, `staleTime: 60s` so second visit instant). Only 2 routes were warmed; `NavItem` was a bare `NavLink` with no preload. Users perceived the full-page spinner as "app frozen" and the follow-on skeleton as a second lag.

## Files touched
- `web/src/App.tsx` — per-route Suspense + `RouteFallback`
- `web/src/layout/SocialMediaLayout.tsx` — NavItem prefetch, idle warm, trends warm

## Reviewer checks
- Verify incognito with "Disable cache" + Network throttling: hover `Explore` should fetch `ExplorePage-*.js` before click; click should keep sidebar visible with only center spinner (or no spinner if preloaded). Test `Bookmarks`, `Lists`, `Search`, `Settings`, `Admin`, `Profile`, `Messages`, `Notifications` similarly.
- Verify `tsc --noEmit`, `npm run build`, and `npm run lint` pass (warnings pre-existing only).
- Verify no regression on `MobileNavigation` (separate bottom nav, not touched — intentional).
- Check `prefetch` does not eagerly import all pages on mount (only hover/idle for a few); confirm no extra `Cache-Control` needed (chunks already immutable).

## How to verify locally
`make proj-dev-web-only` inside `agent-branch/instant-tabs` (ports hashed/persisted in `~/.local/state/gaggle-proj/gaggle-instant-tabs.env`).
