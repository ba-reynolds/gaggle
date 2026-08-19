import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardFooter } from "@/components/ui/card";
import { CustomDialogContent } from "@/components/ui/custom-dialog";
import { Dialog, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Textarea } from "@/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import {
  useBookmarkPost,
  useCreateBookmarkCategory,
  useGetBookmarkCategories,
  useLikePost,
  useQuotePost,
  useRepostPost,
  useUnbookmarkPost,
  useUnlikePost,
  useUnrepostPost,
  useDeletePost,
  usePinPost,
  usePostEdits,
  useUpdatePost,
} from "@/hooks/usePost";
import { useBlockUser, useFollowUser, useUnblockUser, useUnfollowUser } from "@/hooks/useUser";
import { useUser } from "@/contexts/UserContext";
import type { Post } from "@/types/api";
import { formatPostDate, getFullDateFormat } from "@/util/date";
import { formatViews } from "@/util/number";
import {
  AtSign,
  Bookmark,
  Check,
  CornerUpLeft,
  Eye,
  Heart,
  Loader2,
  MessageCircle,
  MoreHorizontal,
  Pencil,
  Pin,
  Plus,
  Repeat2,
  Share,
  Trash2,
  UserPlus,
  UserX,
  Users,
} from "lucide-react";
import React, { useEffect, useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { toast } from "sonner";
import ComposeContent from "./ComposeContent";
import ConfirmDialog from "./ConfirmDialog";
import ContentLinks from "./ContentLinks";
import MediaGallery, { type GalleryItem } from "./MediaGallery";
import PollCard from "./PollCard";
import UserHoverCard from "./UserHoverCard";
import { getMediaUrl } from "@/util/media";
import { useDebounce } from "@/hooks/useDebounce";

interface PostProps {
  post: Post;
}

const FeedPost: React.FC<PostProps> = ({ post }) => {
  const { id, author, content, media, created_at, engagement } = post;
  const { user } = useUser();
  const isOwnPost = user.username === author.username;

  const [replyDialogOpen, setReplyDialogOpen] = useState(false);
  const [quoteDialogOpen, setQuoteDialogOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [editDialogOpen, setEditDialogOpen] = useState(false);
  const [editText, setEditText] = useState(content);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [bookmarkDropdownOpen, setBookmarkDropdownOpen] = useState(false);
  const [categorySearchQuery, setCategorySearchQuery] = useState("");
  const debouncedCategorySearchQuery = useDebounce(categorySearchQuery, 150);
  const [newCategoryName, setNewCategoryName] = useState("");
  const [following, setFollowing] = useState(false);
  const [quoteText, setQuoteText] = useState("");
  const [isShared, setIsShared] = useState(false);

  const shareTimeoutRef = useRef<number | null>(null);
  const navigate = useNavigate();

  // Hooks
  const { mutate: toggleLike } = useLikePost();
  const { mutate: toggleUnlike } = useUnlikePost();
  const { mutate: toggleRepost } = useRepostPost();
  const { mutate: toggleUnrepost } = useUnrepostPost();
  const { mutate: toggleBookmark } = useBookmarkPost();
  const { mutate: toggleUnbookmark } = useUnbookmarkPost();
  const { mutate: createCategory } = useCreateBookmarkCategory();
  const { data: categoriesEnvelope } = useGetBookmarkCategories();
  const quoteMutation = useQuotePost(id);
  const { mutate: follow } = useFollowUser();
  const { mutate: unfollow } = useUnfollowUser();
  const { mutate: block } = useBlockUser();
  const { mutate: unblock } = useUnblockUser();
  const updateMutation = useUpdatePost();
  const deleteMutation = useDeletePost();
  const pinMutation = usePinPost();
  const edits = usePostEdits(id, historyOpen);

  const categories = (categoriesEnvelope?.data ?? []).filter((category) =>
    category.name.toLowerCase().includes(debouncedCategorySearchQuery.toLowerCase())
  );

  // Computed values
  const timePosted = formatPostDate(created_at);
  const timePostedTooltip = getFullDateFormat(created_at);

  const toGalleryItems = (): GalleryItem[] =>
    media.map((item) => ({
      id: item.uuid,
      url: getMediaUrl(item.uuid) ?? "",
      altText: item.alt_text,
    }));

  const galleryItems = toGalleryItems();

  // Event handlers
  const handleLike = () => {
    const mutation = engagement.is_liked ? toggleUnlike : toggleLike;
    mutation(id, {
      onError: () => {
        toast.error(`Failed to ${engagement.is_liked ? 'unlike' : 'like'} post. Please try again.`);
      },
    });
  };

  const handleRepost = () => {
    const mutation = engagement.is_reposted ? toggleUnrepost : toggleRepost;
    mutation(id, {
      onError: () => {
        toast.error(`Failed to ${engagement.is_reposted ? 'undo repost' : 'repost'}. Please try again.`);
      },
    });
  };

  const handleBookmark = (e: React.MouseEvent) => {
    e.stopPropagation();

    if (!engagement.is_bookmarked) {
      toggleBookmark({ postId: id, categoryId: undefined }, {
        onError: () => {
          toast.error("Failed to bookmark post. Please try again.");
        },
      });
    }
    setBookmarkDropdownOpen(true);
  };

  const handleUnbookmark = () => {
    setBookmarkDropdownOpen(false);
    toggleUnbookmark(id, {
      onError: () => {
        toast.error("Failed to remove bookmark. Please try again.");
      },
    });
  };

  const handleBookmarkWithCategory = (categoryId: number) => {
    toggleBookmark({ postId: id, categoryId }, {
      onError: () => {
        toast.error("Failed to bookmark post in category. Please try again.");
      },
    });
    setCategorySearchQuery("");
    setBookmarkDropdownOpen(false);
  };

  const handleCreateCategory = () => {
    if (!newCategoryName.trim()) return;

    createCategory(
      { name: newCategoryName.trim() },
      {
        onSuccess: (response) => {
          handleBookmarkWithCategory(response.data.category.id);
          setNewCategoryName("");
        },
        onError: () => {
          toast.error("Failed to create category. Please try again.");
        },
      }
    );
  };

  const handleQuoteSubmit = () => {
    if (!quoteText.trim()) return;

    quoteMutation.mutate(
      { content: quoteText.trim(), media: [], parent_id: null, visibility: 'public' },
      {
        onSuccess: () => {
          toast.success("Quote posted successfully!");
          setQuoteDialogOpen(false);
          setQuoteText("");
        },
        onError: () => {
          toast.error("Failed to quote post. Please try again.");
        },
      }
    );
  };

  const handleShare = () => {
    const postUrl = `${window.location.origin}/post/${id}`;

    navigator.clipboard.writeText(postUrl).then(() => {
      setIsShared(true);
      if (shareTimeoutRef.current) {
        window.clearTimeout(shareTimeoutRef.current);
      }
      shareTimeoutRef.current = window.setTimeout(() => {
        setIsShared(false);
      }, 2000);
    });
  };

  const handleFollowToggle = (e?: React.MouseEvent) => {
    e?.stopPropagation();
    e?.preventDefault();
    if (following) {
      unfollow(author.username, {
        onError: () => toast.error("Failed to unfollow user."),
      });
    } else {
      follow(author.username, {
        onError: () => toast.error("Failed to follow user."),
      });
    }
    setFollowing(!following);
  };

  const handleBlock = (e: React.MouseEvent) => {
    e.stopPropagation();
    block(author.username, {
      onSuccess: () => toast.success(`Blocked @${author.username}`),
      onError: () => toast.error("Failed to block user."),
    });
  };

  const handleUnblock = (e: React.MouseEvent) => {
    e.stopPropagation();
    unblock(author.username, {
      onSuccess: () => toast.success(`Unblocked @${author.username}`),
      onError: () => toast.error("Failed to unblock user."),
    });
  };

  const handlePostClick = () => {
    navigate(`/post/${id}`);
  };

  useEffect(() => {
    return () => {
      if (shareTimeoutRef.current) {
        window.clearTimeout(shareTimeoutRef.current);
      }
    };
  }, []);

  const formatCount = (count: number) => {
    return count >= 1000 ? `${(count / 1000).toFixed(1)}K` : count;
  };

  return (
    <>
      <Card
        className="w-full max-w-xl border border-border rounded-lg overflow-hidden mb-2 cursor-pointer transition-colors hover:bg-accent py-2 gap-2"
        onClick={handlePostClick}
        tabIndex={0}
        role="link"
        aria-label={`Post by ${author.display_name}: ${content}`}
      >
        <CardContent className="p-4 pb-0">
          <div className="flex items-start space-x-3">
            <UserHoverCard
              name={author.display_name}
              username={author.username}
              userDescription=""
              isFollowing={following}
              onFollowToggle={handleFollowToggle}
            >
              <Avatar className="h-10 w-10 cursor-pointer">
                <AvatarImage src={getMediaUrl(author.profile_picture_uuid)} alt={author.display_name} />
                <AvatarFallback>{author.display_name.charAt(0)}</AvatarFallback>
              </Avatar>
            </UserHoverCard>

            <div className="flex-1 min-w-0">
              <div className="flex items-center">
                <UserHoverCard
                  name={author.display_name}
                  username={author.username}
                  userDescription=""
                  isFollowing={following}
                  onFollowToggle={handleFollowToggle}
                >
                  <div className="flex items-center">
                    <span className="font-semibold text-sm text-primary">{author.display_name}</span>
                    <span className="text-muted-foreground text-xs ml-2">@{author.username}</span>
                  </div>
                </UserHoverCard>

                <TooltipProvider>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <span className="text-muted-foreground text-xs ml-1 cursor-default">· {timePosted}</span>
                    </TooltipTrigger>
                    <TooltipContent side="bottom" className="pointer-events-none">
                      <p>{timePostedTooltip}</p>
                    </TooltipContent>
                  </Tooltip>
                </TooltipProvider>

                {post.visibility === "followers" && (
                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <span className="inline-flex items-center gap-1 ml-1 text-xs text-muted-foreground cursor-default">
                          <Users className="h-3 w-3" />
                        </span>
                      </TooltipTrigger>
                      <TooltipContent side="bottom" className="pointer-events-none">
                        <p>Only your followers can see this post</p>
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                )}
                {post.visibility === "mentions" && (
                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <span className="inline-flex items-center gap-1 ml-1 text-xs text-muted-foreground cursor-default">
                          <AtSign className="h-3 w-3" />
                        </span>
                      </TooltipTrigger>
                      <TooltipContent side="bottom" className="pointer-events-none">
                        <p>Only the people you mentioned can see this post</p>
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                )}

                <DropdownMenu modal={false}>
                  <DropdownMenuTrigger asChild className="ml-auto">
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8 border border-transparent hover:border-border hover:bg-accent"
                      onClick={(e) => e.stopPropagation()}
                    >
                      <MoreHorizontal className="h-4 w-4 text-primary" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="border border-muted">
                    {isOwnPost && <>
                      <DropdownMenuItem className="cursor-pointer" onClick={(e) => { e.stopPropagation(); setEditText(content); setEditDialogOpen(true); }}>
                        <Pencil className="h-4 w-4 mr-2" />
                        <span>Edit post</span>
                      </DropdownMenuItem>
                      <DropdownMenuItem className="cursor-pointer" onClick={(e) => { e.stopPropagation(); pinMutation.mutate({ postId: id, pinned: post.is_pinned, username: author.username }, { onError: () => toast.error(post.is_pinned ? "Failed to unpin post." : "Failed to pin post.") }); }}>
                        <Pin className="h-4 w-4 mr-2" />
                        <span>{post.is_pinned ? "Unpin from profile" : "Pin to profile"}</span>
                      </DropdownMenuItem>
                      <DropdownMenuItem className="cursor-pointer text-destructive focus:text-destructive" onClick={(e) => { e.stopPropagation(); setDeleteDialogOpen(true); }}>
                        <Trash2 className="h-4 w-4 mr-2" />
                        <span>Delete post</span>
                      </DropdownMenuItem>
                    </>}
                    {!isOwnPost && (
                      <DropdownMenuItem className="cursor-pointer" onClick={handleFollowToggle}>
                        {following ? (
                          <UserX className="h-4 w-4 mr-2" />
                        ) : (
                          <UserPlus className="h-4 w-4 mr-2" />
                        )}
                        <span>{following ? `Unfollow @${author.username}` : `Follow @${author.username}`}</span>
                      </DropdownMenuItem>
                    )}
                    {!isOwnPost && (
                      <>
                        {following ? (
                          <DropdownMenuItem className="cursor-pointer text-destructive focus:text-destructive" onClick={handleUnblock}>
                            <UserX className="h-4 w-4 mr-2" />
                            <span>Unblock @{author.username}</span>
                          </DropdownMenuItem>
                        ) : (
                          <DropdownMenuItem className="cursor-pointer text-destructive focus:text-destructive" onClick={handleBlock}>
                            <UserX className="h-4 w-4 mr-2" />
                            <span>Block @{author.username}</span>
                          </DropdownMenuItem>
                        )}
                      </>
                    )}
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>

              {post.parent_id != null && (
                <div className="mt-1 flex items-center gap-1 text-xs text-muted-foreground">
                  <CornerUpLeft className="h-3.5 w-3.5 shrink-0" />
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

              <p className="mt-2 whitespace-pre-wrap text-sm text-primary"><ContentLinks content={content} /></p>
              {post.poll && <PollCard poll={post.poll} postId={id} />}
{isOwnPost && post.edited_at && <button className="mt-2 text-xs text-muted-foreground hover:underline" onClick={(event) => { event.stopPropagation(); setHistoryOpen((open) => !open); }}>
                {historyOpen ? "Hide edit history" : "View edit history"}
              </button>}
              {historyOpen && edits.data?.data.items.map((edit) => <div key={edit.id} className="mt-1 rounded border border-border p-2 text-xs text-muted-foreground">{edit.content_before}</div>)}

              {post.quoted_post_id != null && (
                <Link
                  to={`/post/${post.quoted_post_id}`}
                  onClick={(e) => e.stopPropagation()}
                  className="block"
                >
                  <div className="mt-2 border border-border rounded-lg p-3 hover:bg-accent transition-colors">
                    <p className="text-xs text-muted-foreground">Quoted post</p>
                    <p className="text-sm text-primary mt-1">View quoted post #{post.quoted_post_id}</p>
                  </div>
                </Link>
              )}

              {galleryItems.length > 0 && (
                <div className="mt-2 -ml-1 mr-1">
                  <MediaGallery items={galleryItems} preventImageViewer={false} editable={false} />
                </div>
              )}
            </div>
          </div>
        </CardContent>

        <CardFooter className="pl-14 pr-4 pt-2 pb-0 flex justify-between">
          <div className="flex gap-2">
            {/* Reply Button */}
            <Button
              variant="ghost"
              size="sm"
              className="h-8 gap-1 border border-transparent hover:border-blue-400 hover:bg-blue-50 text-muted-foreground hover:text-blue-500 transition-colors duration-300"
              onClick={(e) => {
                e.stopPropagation();
                setReplyDialogOpen(true);
              }}
            >
              <MessageCircle className="h-4 w-4" />
              <span className="hidden md:inline text-xs">{engagement.reply_count}</span>
            </Button>

            {/* Repost Button (dropdown: repost or quote) */}
            <DropdownMenu modal={false}>
              <DropdownMenuTrigger asChild>
                <Button
                  variant="ghost"
                  size="sm"
                  className={`h-8 gap-1 border border-transparent transition-colors duration-300 ${
                    engagement.is_reposted
                      ? 'text-green-500 hover:text-green-500 hover:bg-green-50 hover:border-green-400'
                      : 'text-muted-foreground hover:text-green-500 hover:bg-green-50 hover:border-green-400'
                  }`}
                  onClick={(e) => e.stopPropagation()}
                >
                  <Repeat2 className="h-4 w-4" />
                  <span className="hidden md:inline text-xs">{formatCount(engagement.repost_count)}</span>
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start" className="border border-muted">
                <DropdownMenuItem className="cursor-pointer" onClick={(e) => { e.stopPropagation(); handleRepost(); }}>
                  <Repeat2 className="h-4 w-4 mr-2" />
                  <span>{engagement.is_reposted ? "Undo repost" : "Repost"}</span>
                </DropdownMenuItem>
                <DropdownMenuItem className="cursor-pointer" onClick={(e) => { e.stopPropagation(); setQuoteDialogOpen(true); }}>
                  <MessageCircle className="h-4 w-4 mr-2" />
                  <span>Quote post</span>
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>

            {/* Like Button */}
            <Button
              variant="ghost"
              size="sm"
              className={`h-8 gap-1 border border-transparent transition-colors duration-300 ${
                engagement.is_liked
                  ? 'text-red-500 hover:text-red-500 hover:bg-red-50 hover:border-red-400'
                  : 'text-muted-foreground hover:text-red-500 hover:bg-red-50 hover:border-red-400'
              }`}
              onClick={(e) => {
                e.stopPropagation();
                handleLike();
              }}
            >
              <Heart className="h-4 w-4" fill={engagement.is_liked ? "currentColor" : "none"} />
              <span className="hidden md:inline text-xs">{formatCount(engagement.like_count)}</span>
            </Button>
          </div>

          <div className="flex items-center gap-2">
            {/* Views Count */}
            <div className="hidden md:flex items-center text-muted-foreground h-8 gap-1 px-2">
              <Eye className="h-4 w-4" />
              <span className="text-xs">{formatViews(engagement.view_count)}</span>
            </div>

            {/* Bookmark Button */}
            <Popover open={bookmarkDropdownOpen} onOpenChange={setBookmarkDropdownOpen} modal={false}>
              <PopoverTrigger asChild>
                <Button
                  variant="ghost"
                  size="sm"
                  className={`h-8 gap-1 border border-transparent transition-colors duration-300 ${
                    engagement.is_bookmarked
                      ? 'text-blue-500 hover:text-blue-500 hover:bg-blue-50 hover:border-blue-400'
                      : 'text-muted-foreground hover:text-blue-500 hover:bg-blue-50 hover:border-blue-400'
                  }`}
                  onClick={handleBookmark}
                >
                  <Bookmark className="h-4 w-4" fill={engagement.is_bookmarked ? "currentColor" : "none"} />
                  <span className="hidden md:inline text-xs">{formatCount(engagement.bookmark_count)}</span>
                </Button>
              </PopoverTrigger>
              <PopoverContent
                className="flex flex-col space-y-1 w-56 p-1 z-50"
                align="end"
                side="bottom"
                sideOffset={5}
                onClick={(e) => e.stopPropagation()}
              >
                {engagement.is_bookmarked && (
                  <>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={handleUnbookmark}
                    >
                      <Bookmark className="h-4 w-4 mr-2" />
                      <span>Remove bookmark</span>
                    </Button>
                    <div className="h-px bg-border" />
                  </>
                )}

                <div className="px-2 py-2">
                  <Input
                    placeholder="Search categories..."
                    value={categorySearchQuery}
                    onChange={(e) => setCategorySearchQuery(e.target.value)}
                    className="h-8"
                    autoFocus
                  />
                </div>

                {categories.slice(0, 5).map((category) => (
                  <Button
                    key={category.id}
                    variant="ghost"
                    size="sm"
                    className="justify-start"
                    onClick={() => handleBookmarkWithCategory(category.id)}
                  >
                    <Bookmark className="h-4 w-4" />
                    <span>{category.name}</span>
                    <span className="ml-auto text-muted-foreground text-xs">
                      {category.post_count}
                    </span>
                  </Button>
                ))}

                <div className="h-px bg-border" />

                <div className="px-2 py-2 flex gap-2">
                  <Input
                    placeholder="New category..."
                    value={newCategoryName}
                    onChange={(e) => setNewCategoryName(e.target.value)}
                    className="h-8"
                  />
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={handleCreateCategory}
                    disabled={!newCategoryName.trim()}
                  >
                    <Plus className="h-4 w-4" />
                  </Button>
                </div>
              </PopoverContent>
            </Popover>

            {/* Share Button */}
            <Button
              variant="ghost"
              size="sm"
              className="h-8 border border-transparent hover:border-border hover:bg-accent transition-colors duration-300 text-muted-foreground"
              onClick={(e) => {
                e.stopPropagation();
                handleShare();
              }}
            >
              {isShared ? <Check className="h-4 w-4" /> : <Share className="h-4 w-4" />}
            </Button>
          </div>
        </CardFooter>
      </Card>

      {/* Reply Dialog */}
      <Dialog open={replyDialogOpen} onOpenChange={setReplyDialogOpen}>
        <CustomDialogContent className="sm:max-w-xl max-h-[90vh] overflow-y-auto bg-card">
          <DialogHeader className="mb-2">
            <DialogTitle className="text-primary">Reply</DialogTitle>
          </DialogHeader>

          <div className="relative flex items-start space-x-3">
            <Avatar className="h-10 w-10">
              <AvatarImage src={getMediaUrl(author.profile_picture_uuid)} alt={author.display_name} />
              <AvatarFallback>{author.display_name.charAt(0)}</AvatarFallback>
            </Avatar>

            <div className="flex-1">
              <div className="flex items-center">
                <span className="font-semibold text-sm text-primary">{author.display_name}</span>
                <span className="text-muted-foreground text-xs ml-2">@{author.username} · {timePosted}</span>
              </div>

              <p className="mt-1 text-sm text-primary">{content}</p>

              {galleryItems.length > 0 && (
                <div className="mt-2 max-h-40 overflow-hidden">
                  <MediaGallery items={galleryItems} preventImageViewer={true} editable={false} />
                </div>
              )}

              <p className="mt-4 mb-8 text-sm text-muted-foreground">
                Replying to <span className="text-blue-500">@{author.username}</span>
              </p>
            </div>
          </div>

          <ComposeContent
            placeholder="Post your reply"
            submitLabel="Reply"
            textareaHeight="h-24"
            parentId={id}
            onSubmit={() => setReplyDialogOpen(false)}
          />
        </CustomDialogContent>
      </Dialog>

      {/* Quote Dialog */}
      <Dialog open={quoteDialogOpen} onOpenChange={setQuoteDialogOpen}>
        <CustomDialogContent className="sm:max-w-xl max-h-[90vh] overflow-y-auto bg-card">
          <DialogHeader className="mb-2">
            <DialogTitle className="text-primary">Quote post</DialogTitle>
          </DialogHeader>

          <div className="border border-border rounded-lg p-3">
            <div className="flex items-center">
              <span className="font-semibold text-sm text-primary">{author.display_name}</span>
              <span className="text-muted-foreground text-xs ml-2">@{author.username}</span>
            </div>
            <p className="mt-1 text-sm text-primary">{content}</p>
          </div>

          <Textarea
            placeholder="Add a comment..."
            className="min-h-24 resize-none text-primary"
            value={quoteText}
            onChange={(e) => setQuoteText(e.target.value)}
          />

          <DialogFooter>
            <Button
              onClick={handleQuoteSubmit}
              disabled={!quoteText.trim() || quoteMutation.isPending}
            >
              {quoteMutation.isPending && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
              Quote
            </Button>
          </DialogFooter>
        </CustomDialogContent>
      </Dialog>

      <Dialog open={editDialogOpen} onOpenChange={setEditDialogOpen}>
        <CustomDialogContent className="sm:max-w-xl bg-card">
          <DialogHeader><DialogTitle className="text-primary">Edit post</DialogTitle></DialogHeader>
          <Textarea value={editText} onChange={(event) => setEditText(event.target.value)} maxLength={280} className="min-h-32" />
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditDialogOpen(false)}>Cancel</Button>
            <Button disabled={!editText.trim() || updateMutation.isPending} onClick={() => updateMutation.mutate({ postId: id, content: editText, username: author.username }, { onSuccess: () => setEditDialogOpen(false), onError: () => toast.error("Failed to update post.") })}>Save changes</Button>
          </DialogFooter>
        </CustomDialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteDialogOpen}
        title="Delete this post?"
        description="This will permanently delete the post and all its replies."
        confirmLabel="Delete"
        onConfirm={() => deleteMutation.mutate({ postId: id, username: author.username }, { onError: () => toast.error("Failed to delete post.") })}
        onOpenChange={setDeleteDialogOpen}
      />
    </>
  );
};

export default FeedPost;
export type { PostProps };
