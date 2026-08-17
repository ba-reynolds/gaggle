import FeedPost from '@/components/FeedPost';
import { Button } from '@/components/ui/button';
import { useHashtagPosts } from '@/hooks/useSearch';
import { Loader2, Hash } from 'lucide-react';
import { useParams } from 'react-router-dom';

export default function HashtagPage() {
  const tag = useParams().tag ?? '';
  const query = useHashtagPosts(tag);
  const posts = query.data?.pages.flatMap((page) => page.data.items) ?? [];
  return (
    <div className="mx-auto w-full max-w-xl">
      <header className="border-b border-border p-5">
        <Hash className="h-6 w-6 text-primary" />
        <h1 className="mt-2 text-2xl font-bold text-primary">#{tag}</h1>
        <p className="text-sm text-muted-foreground">Posts using this hashtag</p>
      </header>
      <div className="space-y-4 p-4">
        {query.isLoading && <Loader2 className="mx-auto mt-8 h-8 w-8 animate-spin text-primary" />}
        {posts.map((post) => <FeedPost key={post.id} post={post} />)}
        {!query.isLoading && posts.length === 0 && <p className="p-8 text-center text-muted-foreground">No posts found.</p>}
        {query.hasNextPage && <Button variant="outline" className="w-full" onClick={() => void query.fetchNextPage()}>Load more</Button>}
      </div>
    </div>
  );
}
