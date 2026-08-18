import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import HashtagText from "@/components/HashtagText";
import MediaGallery, { type GalleryItem } from "@/components/MediaGallery";
import PollCard from "@/components/PollCard";
import type { Post } from "@/types/api";
import { formatPostDate } from "@/util/date";
import { getMediaUrl } from "@/util/media";
import { CornerUpLeft } from "lucide-react";
import { Link, useNavigate } from "react-router-dom";

interface ThreadPanelProps {
  /** The parent chain, furthest-first. Rendered as a comment-style list with a
   *  rail beside the content (never through the avatars). */
  posts: Post[];
}

const ThreadPanel: React.FC<ThreadPanelProps> = ({ posts }) => {
  const navigate = useNavigate();

  return (
    <div className="mt-2">
      {posts.map((post) => {
        const galleryItems: GalleryItem[] = post.media.map((item) => ({
          id: item.uuid,
          url: getMediaUrl(item.uuid) ?? "",
          altText: item.alt_text,
        }));

        return (
          <div
            key={post.id}
            role="link"
            tabIndex={0}
            aria-label={`Post by ${post.author.display_name}: ${post.content}`}
            className="group flex items-start gap-3 rounded-lg cursor-pointer transition-colors hover:bg-accent/40"
            onClick={() => navigate(`/post/${post.id}`)}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                navigate(`/post/${post.id}`);
              }
            }}
          >
            <Avatar className="h-10 w-10 mt-1">
              <AvatarImage src={getMediaUrl(post.author.profile_picture_uuid)} alt={post.author.display_name} />
              <AvatarFallback>{post.author.display_name.charAt(0)}</AvatarFallback>
            </Avatar>

            {/* The rail is a border on the content column, so it runs the whole
                row height (including the pb spacing) — continuous and symmetric
                across rows, and clear of the avatar column. */}
            <div className="flex-1 min-w-0 border-l border-foreground/15 pl-4 pb-5">
              <div className="flex flex-wrap items-center gap-x-1.5 gap-y-0.5">
                <span className="font-semibold text-sm text-primary">{post.author.display_name}</span>
                <span className="text-muted-foreground text-xs">@{post.author.username}</span>
                <span className="text-muted-foreground text-xs">· {formatPostDate(post.created_at)}</span>
              </div>

              {post.parent_id != null && (
                <div className="mt-1 flex items-center gap-1 text-xs text-muted-foreground">
                  <CornerUpLeft className="h-3 w-3 shrink-0" />
                  {post.parent && !post.parent.deleted && post.parent.author ? (
                    <>
                      <span>Replying to</span>
                      <Link
                        to={`/post/${post.parent.id}`}
                        onClick={(e) => e.stopPropagation()}
                        className="text-blue-500 hover:underline"
                      >
                        @{post.parent.author.username}
                      </Link>
                    </>
                  ) : (
                    <span>Replying to a deleted post</span>
                  )}
                </div>
              )}

              <p className="mt-1.5 whitespace-pre-wrap text-sm text-primary">
                <HashtagText content={post.content} />
              </p>
              {post.poll && <PollCard poll={post.poll} postId={post.id} />}
              {galleryItems.length > 0 && (
                <div className="mt-2">
                  <MediaGallery items={galleryItems} preventImageViewer={false} editable={false} />
                </div>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
};

export default ThreadPanel;