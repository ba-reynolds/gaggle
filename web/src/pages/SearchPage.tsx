import { useEffect, useState } from 'react';
import FeedPost from '@/components/FeedPost';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { type PostSearchFilters } from '@/api/search';
import { useSearchPosts, useSearchUsers } from '@/hooks/useSearch';
import { getMediaUrl } from '@/util/media';
import { Loader2, Search as SearchIcon, SlidersHorizontal } from 'lucide-react';
import { useSearchParams, Link } from 'react-router-dom';

const FILTER_KEYS = ['from', 'hashtag', 'media', 'min_likes', 'replies', 'since', 'until'] as const;

interface FilterDraft {
  from: string;
  hashtag: string;
  media: boolean;
  minLikes: string;
  includeReplies: boolean;
  since: string;
  until: string;
}

function emptyDraft(): FilterDraft {
  return { from: '', hashtag: '', media: false, minLikes: '', includeReplies: false, since: '', until: '' };
}

function draftFromParams(params: URLSearchParams): FilterDraft {
  return {
    from: params.get('from') ?? '',
    hashtag: params.get('hashtag') ?? '',
    media: params.get('media') === 'true',
    minLikes: params.get('min_likes') ?? '',
    includeReplies: params.get('replies') === 'true',
    since: params.get('since') ?? '',
    until: params.get('until') ?? '',
  };
}

function filtersFromParams(params: URLSearchParams): PostSearchFilters {
  const draft = draftFromParams(params);
  return {
    from: draft.from || undefined,
    hashtag: draft.hashtag || undefined,
    media: draft.media || undefined,
    minLikes: draft.minLikes ? Number(draft.minLikes) : undefined,
    includeReplies: draft.includeReplies || undefined,
    since: draft.since || undefined,
    until: draft.until || undefined,
  };
}

function activeFilterCount(params: URLSearchParams): number {
  return FILTER_KEYS.filter((key) => {
    const value = params.get(key);
    return value !== null && value !== '' && value !== 'false';
  }).length;
}

export default function SearchPage() {
  const [params, setParams] = useSearchParams();
  const query = params.get('q')?.trim() ?? '';
  const [draft, setDraft] = useState<FilterDraft>(() => draftFromParams(params));
  const [filtersOpen, setFiltersOpen] = useState(false);

  const activeFilters = filtersFromParams(params);
  const posts = useSearchPosts(query, activeFilters);
  const users = useSearchUsers(query);
  const postItems = posts.data?.pages.flatMap((page) => page.data.items) ?? [];
  const filterCount = activeFilterCount(params);

  useEffect(() => {
    setDraft(draftFromParams(params));
  }, [params]);

  const applyFilters = (e: React.FormEvent) => {
    e.preventDefault();
    const next = new URLSearchParams(params);
    for (const key of FILTER_KEYS) next.delete(key);
    if (draft.from.trim()) next.set('from', draft.from.trim());
    if (draft.hashtag.trim()) next.set('hashtag', draft.hashtag.trim());
    if (draft.media) next.set('media', 'true');
    if (draft.minLikes.trim()) next.set('min_likes', draft.minLikes.trim());
    if (draft.includeReplies) next.set('replies', 'true');
    if (draft.since) next.set('since', draft.since);
    if (draft.until) next.set('until', draft.until);
    setParams(next);
  };

  const clearFilters = () => {
    const next = new URLSearchParams(params);
    for (const key of FILTER_KEYS) next.delete(key);
    setParams(next);
    setDraft(emptyDraft());
  };

  const updateDraft = <K extends keyof FilterDraft>(key: K, value: FilterDraft[K]) => {
    setDraft((prev) => ({ ...prev, [key]: value }));
  };

  return (
    <div className="mx-auto w-full max-w-xl">
      <header className="sticky top-0 z-10 border-b border-border p-4 backdrop-blur">
        <p className="text-xs uppercase tracking-wider text-muted-foreground">Search results</p>
        <h1 className="mt-1 text-2xl font-bold text-primary">{query ? `“${query}”` : 'Search'}</h1>
      </header>
      {!query ? (
        <div className="flex flex-col items-center gap-3 p-12 text-center text-muted-foreground">
          <SearchIcon className="h-10 w-10" />
          <p>Search for people, posts, or ideas.</p>
        </div>
      ) : (
        <Tabs defaultValue="posts" className="w-full">
          <TabsList className="m-4 grid w-[calc(100%-2rem)] grid-cols-2">
            <TabsTrigger value="posts">Posts</TabsTrigger>
            <TabsTrigger value="users">People</TabsTrigger>
          </TabsList>
          <TabsContent value="posts" className="space-y-4 px-4 pb-8">
            <Collapsible open={filtersOpen} onOpenChange={setFiltersOpen}>
              <CollapsibleTrigger asChild>
                <Button
                  variant={filterCount > 0 ? 'default' : 'outline'}
                  size="sm"
                  className="rounded-full"
                  type="button"
                >
                  <SlidersHorizontal className="h-4 w-4" />
                  Filters{filterCount > 0 ? ` (${filterCount})` : ''}
                </Button>
              </CollapsibleTrigger>
              <CollapsibleContent>
                <form onSubmit={applyFilters} className="mt-3 space-y-4 rounded-xl border border-border p-4">
                  <div className="grid grid-cols-2 gap-3">
                    <div className="space-y-1">
                      <Label htmlFor="filter-from">From user</Label>
                      <Input
                        id="filter-from"
                        value={draft.from}
                        onChange={(e) => updateDraft('from', e.target.value)}
                        placeholder="username"
                      />
                    </div>
                    <div className="space-y-1">
                      <Label htmlFor="filter-hashtag">Hashtag</Label>
                      <Input
                        id="filter-hashtag"
                        value={draft.hashtag}
                        onChange={(e) => updateDraft('hashtag', e.target.value)}
                        placeholder="e.g. golang"
                      />
                    </div>
                    <div className="space-y-1">
                      <Label htmlFor="filter-min-likes">Min likes</Label>
                      <Input
                        id="filter-min-likes"
                        type="number"
                        min={0}
                        value={draft.minLikes}
                        onChange={(e) => updateDraft('minLikes', e.target.value)}
                        placeholder="0"
                      />
                    </div>
                    <div className="space-y-1">
                      <Label htmlFor="filter-since">From date</Label>
                      <Input
                        id="filter-since"
                        type="date"
                        value={draft.since}
                        onChange={(e) => updateDraft('since', e.target.value)}
                      />
                    </div>
                    <div className="space-y-1">
                      <Label htmlFor="filter-until">To date</Label>
                      <Input
                        id="filter-until"
                        type="date"
                        value={draft.until}
                        onChange={(e) => updateDraft('until', e.target.value)}
                      />
                    </div>
                  </div>
                  <div className="flex items-center justify-between">
                    <Label htmlFor="filter-media">Only with media</Label>
                    <Switch id="filter-media" checked={draft.media} onCheckedChange={(v) => updateDraft('media', v)} />
                  </div>
                  <div className="flex items-center justify-between">
                    <Label htmlFor="filter-replies">Include replies</Label>
                    <Switch
                      id="filter-replies"
                      checked={draft.includeReplies}
                      onCheckedChange={(v) => updateDraft('includeReplies', v)}
                    />
                  </div>
                  <div className="flex gap-2 pt-1">
                    <Button type="submit" className="flex-1 rounded-full">
                      Apply filters
                    </Button>
                    <Button type="button" variant="ghost" className="rounded-full" onClick={clearFilters}>
                      Clear
                    </Button>
                  </div>
                </form>
              </CollapsibleContent>
            </Collapsible>

            {posts.isLoading && <Loader2 className="mx-auto mt-8 h-8 w-8 animate-spin text-primary" />}
            {!posts.isLoading && postItems.length === 0 && <p className="p-8 text-center text-muted-foreground">No matching posts.</p>}
            {postItems.map((post) => <FeedPost key={post.id} post={post} />)}
            {posts.hasNextPage && <Button variant="outline" className="w-full" onClick={() => void posts.fetchNextPage()}>Load more</Button>}
          </TabsContent>
          <TabsContent value="users" className="space-y-2 px-4 pb-8">
            {users.isLoading && <Loader2 className="mx-auto mt-8 h-8 w-8 animate-spin text-primary" />}
            {users.data?.data.items.map((user) => (
              <Link key={user.username} to={`/profile/${user.username}`} className="flex items-center gap-3 rounded-xl p-3 hover:bg-muted">
                <Avatar className="h-12 w-12">
                  <AvatarImage src={getMediaUrl(user.profile_picture_uuid)} />
                  <AvatarFallback>{user.display_name?.[0] ?? user.username[0]}</AvatarFallback>
                </Avatar>
                <div>
                  <p className="font-semibold text-primary">{user.display_name || user.username}</p>
                  <p className="text-sm text-muted-foreground">@{user.username} · {user.followers_count} followers</p>
                </div>
              </Link>
            ))}
            {!users.isLoading && users.data?.data.items.length === 0 && <p className="p-8 text-center text-muted-foreground">No matching people.</p>}
          </TabsContent>
        </Tabs>
      )}
    </div>
  );
}