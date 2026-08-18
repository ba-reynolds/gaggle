import { useParams, useNavigate } from "react-router-dom";
import { useGetPost } from "@/hooks/usePost";
import FeedPost from "@/components/FeedPost";
import { Button } from "@/components/ui/button";
import { Loader2, ArrowLeft } from "lucide-react";
import ComposeContent from "@/components/ComposeContent";

type FeedPostThreadPosition = 'first' | 'middle' | 'last' | undefined;

const PostPage = () => {
  const { id } = useParams<{ id: string }>();
  const postId = parseInt(id || "0", 10);
  const navigate = useNavigate();

  const { data: postData, isLoading: isLoadingPost, isError } = useGetPost(postId);

  if (isLoadingPost) {
    return (
      <div className="w-full flex items-center justify-center py-8">
        <Loader2 className="h-8 w-8 animate-spin text-primary" />
      </div>
    );
  }

  if (isError || !postData?.data?.post) {
    return (
      <div className="text-center py-8 text-muted-foreground">
        Post not found
      </div>
    );
  }

  const post = postData.data.post;
  const ancestors = postData.data.ancestors?.items ?? [];
  const replies = postData.data.descendants?.items ?? [];
  // Ancestors arrive nearest-parent-first; display them furthest-first so the
  // current post anchors the bottom of the conversation.
  const parentChain = [...ancestors].reverse();
  // The chain uses the same FeedPost cards as everywhere else in the app; a
  // connector rail lives in a gutter to the LEFT of the whole chain, with a
  // C-shaped elbow pointing from each parent's picture down to the child.
  const threadPosts = [...parentChain, post];
  const threadPosition = (index: number): FeedPostThreadPosition => {
    if (threadPosts.length === 1) return undefined;
    if (index === 0) return 'first';
    if (index === threadPosts.length - 1) return 'last';
    return 'middle';
  };

  return (
    <div className="w-full max-w-xl mx-auto">
      {/* Sticky page header */}
      <header className="sticky top-0 z-10 flex items-center gap-3 border-b border-border p-4 backdrop-blur">
        <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => navigate(-1)} aria-label="Go back">
          <ArrowLeft className="h-5 w-5" />
        </Button>
        <div>
          <h1 className="text-lg font-bold leading-tight text-primary">Post</h1>
        </div>
      </header>

      {/* Conversation: ancestors (furthest-first) + the current post. The cards are
          the same FeedPost cards used everywhere; the C-shaped connector lives
          in a gutter on the left of the whole chain. */}
      <div className="mt-2">
        {threadPosts.map((threadPost, index) => {
          const position = threadPosition(index);
          return (
            <div key={threadPost.id} className={position ? "flex gap-x-1.5" : ""}>
              {position && (
                <div aria-hidden className="relative w-6 shrink-0">
                  {/* Straight vertical rail. Starts at the top post's avatar,
                      runs through, and stops at the last post's elbow. */}
                  <div
                    className={`absolute left-[11px] w-0.5 bg-border ${
                      position === 'first' ? 'top-6 bottom-0 rounded-t-full' :
                      position === 'last' ? 'top-0 h-[45px]' :
                      'top-0 bottom-0'
                    }`}
                  />
                  {/* Straight horizontal tick, at every post's avatar level,
                      pointing right toward the profile picture. */}
                  <div className="absolute top-[43px] left-[11px] w-[17px] h-0.5 rounded-full bg-border" />
                  {/* Small rounded fillet where the rail meets the tick. */}
                  <div className="absolute top-[42px] left-[10px] w-[5px] h-[5px] rounded-full bg-border" />
                </div>
              )}
              <div className="flex-1 min-w-0">
                <FeedPost post={threadPost} />
              </div>
            </div>
          );
        })}
      </div>

      {/* Reply composer */}
      <div className="mt-4 px-4">
        <ComposeContent
          placeholder="Post your reply"
          submitLabel="Reply"
          textareaHeight="h-24"
          parentId={post.id}
        />
      </div>

      {/* Replies */}
      <div className="mt-4">
        {replies.length > 0 ? (
          <div className="space-y-2">
            {replies.map((reply) => (
              <FeedPost key={reply.id} post={reply} />
            ))}
          </div>
        ) : (
          <div className="text-center py-8 text-muted-foreground">
            No replies yet
          </div>
        )}
      </div>
    </div>
  );
};

export default PostPage;