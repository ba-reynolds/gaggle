import { useParams } from "react-router-dom";
import { useGetPost } from "@/hooks/usePost";
import FeedPost from "@/components/FeedPost";
import { Loader2 } from "lucide-react";
import ComposeContent from "@/components/ComposeContent";

const PostPage = () => {
  const { id } = useParams<{ id: string }>();
  const postId = parseInt(id || "0", 10);

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
  // Ancestors arrive nearest-parent-first; display them furthest-first.
  const parentChain = [...ancestors].reverse();

  return (
    <div className="w-full max-w-xl mx-auto">
      {/* Main post */}
      <FeedPost post={post} />

      {/* Parent chain */}
      {parentChain.length > 0 && (
        <div className="relative">
          {/* Connecting line */}
          <div className="absolute left-[1.25rem] top-0 bottom-0 w-[2px] bg-border" />

          {parentChain.map((parent) => (
            <div key={parent.id} className="relative">
              {/* Horizontal connecting line */}
              <div className="absolute left-[1.25rem] top-1/2 w-4 h-[2px] bg-border" />
              <FeedPost post={parent} />
            </div>
          ))}
        </div>
      )}

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
      <div className="mt-4 relative">
        {/* Connecting line for replies */}
        <div className="absolute left-[1.25rem] top-0 bottom-0 w-[2px] bg-border" />

        {replies.length > 0 ? (
          <>
            {replies.map((reply) => (
              <div key={reply.id} className="relative">
                {/* Horizontal connecting line */}
                <div className="absolute left-[1.25rem] top-1/2 w-4 h-[2px] bg-border" />
                <FeedPost post={reply} />
              </div>
            ))}
          </>
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