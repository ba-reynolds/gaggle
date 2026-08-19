import FeedPost from '@/components/FeedPost';
import { Button } from '@/components/ui/button';
import { useMentionsFeed } from '@/hooks/useSearch';
import { Loader2, AtSign } from 'lucide-react';

export default function MentionsPage() {
  const query = useMentionsFeed();
  const posts = query.data?.pages.flatMap((page) => page.data.items) ?? [];
  return (
    <div className="mx-auto w-full max-w-xl">
      <header className="border-b border-border p-5">
        <AtSign className="h-6 w-6 text-primary" />
        <h1 className="mt-2 text-2xl font-bold text-primary">Mentions</h1>
        <p className="text-sm text-muted-foreground">Posts that tagged you with @username</p>
      </header>
      <div className="space-y-4 p-4">
        {query.isLoading && <Loader2 className="mx-auto mt-8 h-8 w-8 animate-spin text-primary" />}
        {posts.map((post) => <FeedPost key={post.id} post={post} />)}
        {!query.isLoading && posts.length === 0 && <p className="p-8 text-center text-muted-foreground">No mentions yet.</p>}
        {query.hasNextPage && <Button variant="outline" className="w-full" onClick={() => void query.fetchNextPage()}>Load more</Button>}
      </div>
    </div>
  );
}