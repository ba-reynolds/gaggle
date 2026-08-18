import FeedPost from '@/components/FeedPost';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useSearchPosts, useSearchUsers } from '@/hooks/useSearch';
import { getMediaUrl } from '@/util/media';
import { Loader2, Search as SearchIcon } from 'lucide-react';
import { useSearchParams, Link } from 'react-router-dom';

export default function SearchPage() {
  const [params] = useSearchParams();
  const query = params.get('q')?.trim() ?? '';
  const posts = useSearchPosts(query);
  const users = useSearchUsers(query);
  const postItems = posts.data?.pages.flatMap((page) => page.data.items) ?? [];

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
