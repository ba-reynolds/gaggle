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
        <div className="mt-2">
          {parentChain.map((parent) => (
            <FeedPost key={parent.id} post={parent} />
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