import { useGetBookmarkedPosts, useGetBookmarkCategories } from "@/hooks/usePost";
import { Loader2, Search } from "lucide-react";
import FeedPost from "@/components/FeedPost";
import { Input } from "@/components/ui/input";
import { useState, useEffect, useRef } from "react";
import { useDebounce } from "@/hooks/useDebounce";
import { useInView } from "react-intersection-observer";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

const BookmarksPage: React.FC = () => {
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedCategories, setSelectedCategories] = useState<number[]>([]);
  const debouncedSearchQuery = useDebounce(searchQuery, 300);
  const { ref, inView } = useInView();

  const { data: categoriesData } = useGetBookmarkCategories();
  const categories = categoriesData?.data ?? [];

  const {
    data: bookmarkedPosts,
    isLoading,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage
  } = useGetBookmarkedPosts(
    selectedCategories.length > 0 ? selectedCategories : undefined
  );
  const isFetchingRef = useRef(false);

  useEffect(() => {
    if (inView && hasNextPage && !isFetchingRef.current) {
      isFetchingRef.current = true;
      fetchNextPage().finally(() => {
        setTimeout(() => {
          isFetchingRef.current = false;
        }, 0);
      });
    }
  }, [inView, hasNextPage, fetchNextPage]);

  const toggleCategory = (categoryId: number) => {
    setSelectedCategories(prev =>
      prev.includes(categoryId)
        ? prev.filter(id => id !== categoryId)
        : [...prev, categoryId]
    );
  };

  // Flatten all pages of posts and apply client-side search filtering
  const allPosts = bookmarkedPosts?.pages.flatMap(page => page.data.items) ?? [];
  const posts = debouncedSearchQuery
    ? allPosts.filter(post => post.content.toLowerCase().includes(debouncedSearchQuery.toLowerCase()))
    : allPosts;

  return (
    <div className="w-full max-w-xl mx-auto">
      {/* Search Bar (always rendered) */}
      <div className="sticky top-0 py-2 px-1 z-10 backdrop-blur rounded-b-xl shadow-lg">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            type="text"
            placeholder="Search bookmarks..."
            value={searchQuery}
            onChange={e => setSearchQuery(e.target.value)}
            className="pl-9"
          />
        </div>
      </div>

      {/* Category Filters */}
      <div className="px-1 py-3 border-b">
        <div className="flex flex-wrap gap-1.5">
          {categories.map(category => (
            <Badge
              key={category.id}
              variant={selectedCategories.includes(category.id) ? "default" : "secondary"}
              className={cn(
                "cursor-pointer transition-colors flex gap-1 text-sm font-medium",
                selectedCategories.includes(category.id)
                  ? "bg-primary text-primary-foreground hover:bg-primary/90"
                  : "bg-secondary text-secondary-foreground hover:bg-secondary/80"
              )}
              onClick={() => toggleCategory(category.id)}
            >
              <span>{category.name}</span>
              <span className={cn("text-xs", selectedCategories.includes(category.id) ? "text-primary-foreground/80" : "text-muted-foreground")}>{category.post_count}</span>
            </Badge>
          ))}
        </div>
      </div>

      {/* Bookmarks List (inline loader) */}
      <div className="mt-4 space-y-4">
        {isLoading ? (
          <div className="w-full flex items-center justify-center py-8">
            <Loader2 className="h-8 w-8 animate-spin text-primary" />
          </div>
        ) : posts.length === 0 ? (
          <div className="text-center py-8 text-muted-foreground">
            {debouncedSearchQuery || selectedCategories.length > 0
              ? "No matching bookmarks found"
              : "No bookmarks yet"}
          </div>
        ) : (
          <>
            {posts.map(post => (
              <FeedPost key={post.id} post={post} />
            ))}

            {/* Loading indicator for infinite scroll */}
            <div ref={ref} className="w-full flex justify-center py-4">
              {isFetchingNextPage && (
                <Loader2 className="h-8 w-8 animate-spin text-primary" />
              )}
            </div>
          </>
        )}
      </div>
    </div>
  );
}

export default BookmarksPage;