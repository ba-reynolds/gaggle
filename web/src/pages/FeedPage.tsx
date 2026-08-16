import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useGetFeedPosts } from "@/hooks/usePost";
import { Loader2, Sparkles } from "lucide-react";
import FeedPost from "@/components/FeedPost";
import { useInView } from "react-intersection-observer";
import { useEffect, useRef } from "react";

// TODO: make it so that when you scroll, all sidebars scroll with the page
// error mode?

const FeedPage: React.FC = () => {
  const { ref, inView } = useInView();
  const { data, isLoading, isError, fetchNextPage, hasNextPage, isFetchingNextPage } = useGetFeedPosts();
  const isFetchingRef = useRef(false);

  useEffect(() => {
    // Without `setTimeout`:
    // 1. `fetchNextPage()` resolves
    // 2. `.finally()` microtask --> `ref = false`
    // 3. React re-renders (`useEffect` runs again immediately)
    // 4. Duplicate call happens

    // With setTimeout(..., 0):
    // 1. `fetchNextPage()` resolves  
    // 2. `.finally()` microtask: `setTimeout` queued
    // 3. React re-renders with `ref` still true
    // 4. `useEffect` sees `ref` is true, skips fetch
    // 5. Next event loop: setTimeout macrotask runs, `ref = false`
    if (inView && hasNextPage && !isFetchingRef.current) {
      isFetchingRef.current = true;
      fetchNextPage().finally(() => {
        setTimeout(() => {
          isFetchingRef.current = false;
        }, 0);
      });
    }
  }, [inView, hasNextPage, fetchNextPage]);

  // Render loading state
  if (isLoading) {
    return (
      <div className="w-full max-w-xl mx-auto flex items-center justify-center p-8">
        <Loader2 className="h-8 w-8 animate-spin text-primary" />
      </div>
    );
  }

  // Render error state
  if (isError) {
    return (
      <div className="w-full max-w-xl mx-auto p-8">
        <div className="text-center text-destructive">
          <p>Failed to load posts. Please try again later.</p>
        </div>
      </div>
    );
  }

  // Flatten all pages of posts
  const allPosts = data?.pages.flatMap(page => page.data.items) ?? [];

  return (
    <div className="w-full max-w-xl mx-auto">
      <Tabs defaultValue="following" className="w-full">

        {/* Feed Tabs, outtermost div is sticky to the top of the page and has a backdrop blur */}
        <div className="sticky top-0 py-2 px-1 z-10 backdrop-blur rounded-b-xl shadow-lg">
          <TabsList className="w-full h-full grid grid-cols-2">
            <TabsTrigger value="following" className="data-[state=active]:font-semibold">
              Following
            </TabsTrigger>
            <TabsTrigger value="discover" className="data-[state=active]:font-semibold">
              Discover
            </TabsTrigger>
          </TabsList>
        </div>

        {/* Following feed Content */}
        <TabsContent value="following" className="mt-2 space-y-4">
          {allPosts.map(post => (
            <FeedPost key={post.id} post={post} />
          ))}
          
          {/* Loading indicator for infinite scroll */}
          <div ref={ref} className="w-full flex justify-center py-4">
            {isFetchingNextPage && (
              <Loader2 className="h-8 w-8 animate-spin text-primary" />
            )}
          </div>
        </TabsContent>

        {/* Discover feed Content */}
        <TabsContent value="discover" className="mt-2 space-y-4">
          <div className="flex items-center justify-center p-8 text-muted-foreground">
            <div className="text-center">
              <Sparkles className="h-8 w-8 mx-auto mb-2" />
              <p>Discover trending content here</p>
            </div>
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}

export default FeedPage;
