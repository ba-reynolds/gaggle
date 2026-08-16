import { HoverCard, HoverCardContent, HoverCardTrigger } from "@/components/ui/hover-card";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { useFetchProfile } from "@/hooks/useUser";
import { getMediaUrl } from "@/util/media";
import { useUser } from "@/contexts/UserContext";
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
              <Avatar className="h-16 w-16">
                <AvatarImage src={getMediaUrl(profile?.profile_picture_uuid)} alt={displayName} />
                <AvatarFallback>{displayName.charAt(0)}</AvatarFallback>
              </Avatar>
            </Link>
            {!isCurrentUser && (
              <Button
                variant={isFollowing ? "outline" : "default"}
                size="sm"
                onClick={(e) => onFollowToggle?.(e)}
              >
                {isFollowing ? "Following" : "Follow"}
              </Button>
            )}
          </div>

          <div>
            <Link to={profileUrl} className="cursor-pointer">
              <h4 className="font-bold">{displayName}</h4>
              <p className="text-sm text-muted-foreground">@{username}</p>
            </Link>

            <p className="text-sm mt-2 line-clamp-4">{bio}</p>

            <div className="flex space-x-4 mt-2 text-sm">
              <span><span className="font-semibold">{followingCount.toLocaleString()}</span> Following</span>
              <span><span className="font-semibold">{followersCount.toLocaleString()}</span> Followers</span>
            </div>
          </div>
        </div>
      </HoverCardContent>
    </HoverCard>
  );
};

export default UserHoverCard;