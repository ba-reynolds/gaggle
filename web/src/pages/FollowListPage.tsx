import { UserAvatar } from "@/components/UserAvatar";
import { Button } from "@/components/ui/button";
import { Loader2 } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { useFollowUser, useUnfollowUser } from "@/hooks/useUser";
import { fetchUserFollowers, fetchUserFollowing } from "@/api/user";
import { getMediaUrl } from "@/util/media";
import { toast } from "sonner";
import type { UserProfileResponse } from "@/types/api";

interface FollowListPageProps {
  listType: "followers" | "following";
}

const FollowListPage: React.FC<FollowListPageProps> = ({ listType }) => {
  const { username } = useParams<{ username: string }>();
  const navigate = useNavigate();
  const safeUsername = username ?? "";
  const followMutation = useFollowUser();
  const unfollowMutation = useUnfollowUser();

  const [items, setItems] = useState<UserProfileResponse[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [following, setFollowing] = useState<Record<string, boolean>>({});

  const loadPage = useCallback(
    async (cursor?: string) => {
      if (!safeUsername) return;
      setLoading(true);
      try {
        const fetcher = listType === "followers" ? fetchUserFollowers : fetchUserFollowing;
        const res = await fetcher(safeUsername, cursor ?? undefined, 20);
        setItems(prev => (cursor ? [...prev, ...res.data.items] : res.data.items));
        setNextCursor(res.data.next_cursor ?? null);
        setHasMore(res.data.has_more);
        setFollowing(prev => {
          const next = { ...prev };
          res.data.items.forEach(it => {
            if (it.is_following !== undefined) next[it.username] = it.is_following;
          });
          return next;
        });
      } catch {
        toast.error(`Failed to load ${listType}.`);
      } finally {
        setLoading(false);
      }
    },
    [safeUsername, listType],
  );

  useEffect(() => {
    setItems([]);
    setNextCursor(null);
    setHasMore(false);
    loadPage(undefined);
  }, [loadPage]);

  const handleFollowToggle = (target: UserProfileResponse) => {
    if (following[target.username]) {
      unfollowMutation.mutate(target.username, {
        onSuccess: () => {
          setFollowing(prev => ({ ...prev, [target.username]: false }));
        },
        onError: () => toast.error(`Failed to unfollow @${target.username}.`),
      });
    } else {
      followMutation.mutate(target.username, {
        onSuccess: () => {
          setFollowing(prev => ({ ...prev, [target.username]: true }));
        },
        onError: () => toast.error(`Failed to follow @${target.username}.`),
      });
    }
  };

  return (
    <div className="w-full max-w-2xl mx-auto">
      <div className="sticky top-0 z-10 flex items-center gap-4 px-4 py-3 border-b border-border bg-background/95 backdrop-blur">
        <Button variant="ghost" size="icon" className="text-foreground" onClick={() => navigate(-1)}>
          ←
        </Button>
        <div>
          <h1 className="text-lg font-bold text-primary capitalize">{listType}</h1>
          <p className="text-sm text-muted-foreground">@{safeUsername}</p>
        </div>
      </div>

      {loading && items.length === 0 ? (
        <div className="w-full flex items-center justify-center py-16">
          <Loader2 className="h-8 w-8 animate-spin text-primary" />
        </div>
      ) : items.length === 0 ? (
        <div className="text-center py-16 text-muted-foreground">
          No {listType} yet.
        </div>
      ) : (
        <div className="divide-y divide-border">
          {items.map((profile) => {
            const isFollowing = following[profile.username] ?? profile.is_following ?? false;
            return (
              <div key={profile.username} className="flex items-start gap-3 p-4">
                <Link to={`/profile/${profile.username}`} className="shrink-0">
                  <UserAvatar className="h-12 w-12" src={getMediaUrl(profile.profile_picture_uuid)} name={profile.display_name} username={profile.username} />
                </Link>
                <div className="flex-1 min-w-0">
                  <Link to={`/profile/${profile.username}`} className="block hover:underline">
                    <span className="font-semibold text-primary truncate">{profile.display_name}</span>
                    <span className="text-muted-foreground ml-1 truncate">@{profile.username}</span>
                  </Link>
                  <p className="text-sm text-muted-foreground line-clamp-2 mt-0.5">{profile.bio}</p>
                </div>
                <Button
                  variant={isFollowing ? "outline" : "default"}
                  size="sm"
                  className={isFollowing ? "text-foreground" : ""}
                  disabled={followMutation.isPending || unfollowMutation.isPending}
                  onClick={() => handleFollowToggle(profile)}
                >
                  {isFollowing ? "Following" : "Follow"}
                </Button>
              </div>
            );
          })}
        </div>
      )}

      {hasMore && (
        <div className="flex justify-center py-6">
          <Button variant="outline" className="text-foreground" onClick={() => loadPage(nextCursor ?? undefined)} disabled={loading}>
            {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : "Load more"}
          </Button>
        </div>
      )}
    </div>
  );
};

export default FollowListPage;