import { useMemo, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import FeedPost from '@/components/FeedPost';
import UserHoverCard from '@/components/UserHoverCard';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useSuggestedUsers, useTrends, useSearchPosts } from '@/hooks/useSearch';
import { SEARCH_DEBOUNCE_MS, useDebounce } from '@/hooks/useDebounce';
import { useFollowUser, useUnfollowUser } from '@/hooks/useUser';
import { useAuth } from '@/contexts/AuthContext';
import { getMediaUrl } from '@/util/media';
import { Loader2, Search as SearchIcon, TrendingUp, Users } from 'lucide-react';

export default function ExplorePage() {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const activeTab = searchParams.get('tab') === 'trending' ? 'trending' : 'suggested';
  const { token } = useAuth();
  const isAuthenticated = typeof token === 'string';
  const [query, setQuery] = useState('');
  const debouncedQuery = useDebounce(query, SEARCH_DEBOUNCE_MS);
  const trends = useTrends(isAuthenticated);
  const suggested = useSuggestedUsers(20, isAuthenticated);
  const results = useSearchPosts(debouncedQuery);
  const { mutate: follow } = useFollowUser();
  const { mutate: unfollow } = useUnfollowUser();
  const [following, setFollowing] = useState<Record<string, boolean>>({});

  const isFollowing = (username: string) => following[username] ?? false;

  const toggleFollow = (username: string) => (e?: React.MouseEvent) => {
    e?.stopPropagation();
    if (isFollowing(username)) {
      unfollow(username);
      setFollowing((prev) => ({ ...prev, [username]: false }));
    } else {
      follow(username);
      setFollowing((prev) => ({ ...prev, [username]: true }));
    }
  };

  const suggestedItems = useMemo(() => suggested.data?.items ?? [], [suggested.data]);
  const postItems = useMemo(() => results.data?.pages.flatMap((page) => page.data.items) ?? [], [results.data]);

  const submitSearch = (e: React.FormEvent) => {
    e.preventDefault();
    navigate(`/search?q=${encodeURIComponent(query.trim())}`);
  };

  return (
    <div className="mx-auto w-full max-w-xl">
      <header className="sticky top-0 z-10 border-b border-border p-4 backdrop-blur">
        <h1 className="text-2xl font-bold text-primary">Explore</h1>
        <form onSubmit={submitSearch} className="mt-3">
          <div className="relative">
            <SearchIcon className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search people, posts, or hashtags..."
              className="rounded-full pl-10"
            />
          </div>
        </form>
      </header>

      <Tabs value={activeTab} onValueChange={(value) => setSearchParams(value === 'trending' ? { tab: 'trending' } : {})} className="w-full">
        <TabsList className="m-4 grid w-[calc(100%-2rem)] grid-cols-2">
          <TabsTrigger value="suggested"><Users className="mr-2 h-4 w-4" />Who to follow</TabsTrigger>
          <TabsTrigger value="trending"><TrendingUp className="mr-2 h-4 w-4" />Trending</TabsTrigger>
        </TabsList>

        <TabsContent value="suggested" className="space-y-4 px-4 pb-8">
          {suggested.isLoading && <Loader2 className="mx-auto mt-8 h-8 w-8 animate-spin text-primary" />}
          {!suggested.isLoading && suggestedItems.length === 0 && (
            <p className="p-8 text-center text-muted-foreground">No suggestions right now.</p>
          )}
          {suggestedItems.map((profile) => (
            <div key={profile.username} className="flex items-center justify-between rounded-xl border border-border p-3">
              <UserHoverCard
                name={profile.display_name}
                username={profile.username}
                userDescription={profile.bio}
                followers={profile.followers_count}
                following={profile.following_count}
                isFollowing={isFollowing(profile.username)}
                onFollowToggle={toggleFollow(profile.username)}
              >
                <div className="flex items-center">
                  <Avatar className="h-10 w-10 mr-2">
                    <AvatarImage src={getMediaUrl(profile.profile_picture_uuid)} alt={profile.display_name} />
                    <AvatarFallback>{profile.display_name?.[0] ?? profile.username[0]}</AvatarFallback>
                  </Avatar>
                  <div>
                    <p className="font-semibold text-sm text-primary">{profile.display_name || profile.username}</p>
                    <p className="text-xs text-muted-foreground">@{profile.username}</p>
                  </div>
                </div>
              </UserHoverCard>
              <Button
                size="sm"
                className="rounded-full"
                variant={isFollowing(profile.username) ? "outline" : "default"}
                onClick={toggleFollow(profile.username)}
              >
                {isFollowing(profile.username) ? "Following" : "Follow"}
              </Button>
            </div>
          ))}
        </TabsContent>

        <TabsContent value="trending" className="space-y-2 px-4 pb-8">
          {trends.data?.length ? (
            trends.data.map((trend) => (
              <button
                key={trend.name}
                className="block w-full rounded-xl border border-border p-3 text-left hover:bg-muted"
                onClick={() => navigate(`/hashtags/${trend.name}`)}
              >
                <p className="text-xs text-muted-foreground">Trending now</p>
                <p className="font-semibold text-primary">#{trend.name}</p>
                <p className="text-xs text-muted-foreground">{trend.count} posts</p>
              </button>
            ))
          ) : (
            <p className="p-8 text-center text-muted-foreground">No trends yet.</p>
          )}
        </TabsContent>
      </Tabs>

      {postItems.length > 0 && (
        <div className="border-t border-border px-4 py-4">
          <h2 className="mb-3 text-lg font-semibold text-primary">Posts</h2>
          <div className="space-y-4">
            {postItems.map((post) => <FeedPost key={post.id} post={post} />)}
          </div>
        </div>
      )}
    </div>
  );
}