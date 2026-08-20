import { HoverCard, HoverCardContent, HoverCardTrigger } from "@/components/ui/hover-card";
import { UserAvatar } from "@/components/UserAvatar";
import { Button } from "@/components/ui/button";
import { useFetchProfile } from "@/hooks/useUser";
import UserBadges from "@/components/UserBadges";
import { getMediaUrl } from "@/util/media";
import { useUser } from "@/contexts/UserContext";
import { useI18n } from "@/contexts/I18nContext";
import { Link } from "react-router-dom";

interface UserHoverCardProps {
  name: string;
  username: string;
  userDescription?: string;
  following?: number;
  followers?: number;
  isFollowing?: boolean;
  onFollowToggle?: (e?: React.MouseEvent) => void;
  fetchProfile?: boolean;
  children: React.ReactNode;
}

const UserHoverCard: React.FC<UserHoverCardProps> = ({
  name,
  username,
  userDescription,
  following = 0,
  followers = 0,
  isFollowing = false,
  onFollowToggle,
  fetchProfile = true,
  children,
}) => {
  const profileUrl = `/profile/${username}`;
  const { data: profileData } = useFetchProfile(username, fetchProfile);
  const { user } = useUser();
  const { t } = useI18n();
  const isCurrentUser = user.username === username;

  const profile = profileData?.data;
  const displayName = profile?.display_name || name;
  const bio = profile?.bio || userDescription;
  const followingCount = profile?.following_count ?? following;
  const followersCount = profile?.followers_count ?? followers;

  return (
    <HoverCard>
      <HoverCardTrigger asChild>
        <a href={profileUrl} className="cursor-pointer">
          {children}
        </a>
      </HoverCardTrigger>
      <HoverCardContent className="w-80 border border-muted">
        <div className="flex flex-col space-y-4">
          <div className="flex justify-between items-start">
            <Link to={profileUrl} className="cursor-pointer">
              <UserAvatar className="h-16 w-16" src={getMediaUrl(profile?.profile_picture_uuid)} name={displayName} username={username} />
            </Link>
            {!isCurrentUser && (
              <Button
                variant={isFollowing ? "outline" : "default"}
                size="sm"
                onClick={(e) => onFollowToggle?.(e)}
              >
                {isFollowing ? t("whoToFollow.following") : t("whoToFollow.follow")}
              </Button>
            )}
          </div>

          <div>
            <Link to={profileUrl} className="cursor-pointer">
              <h4 className="font-bold">{displayName}</h4>
              <p className="text-sm text-muted-foreground">@{username}</p>
            </Link>
            <UserBadges badges={profile?.badges} className="mt-1.5" />

            <p className="text-sm mt-2 line-clamp-4">{bio}</p>

            <div className="flex space-x-4 mt-2 text-sm">
              <span><span className="font-semibold">{followingCount.toLocaleString()}</span> {t("whoToFollow.followingNoun")}</span>
              <span><span className="font-semibold">{followersCount.toLocaleString()}</span> {t("whoToFollow.followersNoun")}</span>
            </div>
          </div>
        </div>
      </HoverCardContent>
    </HoverCard>
  );
};

export default UserHoverCard;