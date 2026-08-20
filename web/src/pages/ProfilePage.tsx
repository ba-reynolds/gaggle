import FeedPost from "@/components/FeedPost";
import { UserAvatar } from "@/components/UserAvatar";
import { Button } from "@/components/ui/button";
import { CustomDialogContent } from "@/components/ui/custom-dialog";
import { Dialog, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { useUser } from "@/contexts/UserContext";
import { useMediaUpload } from "@/hooks/useMediaUpload";
import { useGetUserMedia, useGetUserPosts, useGetUserReplies } from "@/hooks/usePost";
import { useFetchProfile, useFollowUser, useMuteUser, usePinnedPost, useUnblockUser, useUnfollowUser, useUnmuteUser, useUpdateProfile, useBlockUser } from "@/hooks/useUser";
import { useUserLists } from "@/hooks/useLists";
import UserBadges from "@/components/UserBadges";
import { getMediaUrl } from "@/util/media";
import { format, parseISO } from "date-fns";
import { Calendar, Camera, Link as LinkIcon, Loader2, Lock, MapPin, MessageSquare, MoreHorizontal, ShieldBan, ShieldCheck, UserMinus, UserPlus, Volume2, VolumeX } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useInView } from "react-intersection-observer";
import { useParams, Link, useNavigate } from "react-router-dom";
import type { InfiniteData } from "@tanstack/react-query";
import type { Envelope, PaginatedFeedResponse } from "@/types/api";
import { toast } from "sonner";

// The API serializes an unset birth_date (DB NULL) as the zero Date
// "0001-01-01". Treat that sentinel as "no birthday set" everywhere instead
// of showing a fake "Born January 1, 0001" or pre-filling the editor with it.
const UNSET_BIRTH_DATE = "0001-01-01";

const isUnsetBirthDate = (dateString: string | null | undefined): boolean =>
  !dateString || dateString === UNSET_BIRTH_DATE;

const ProfilePage: React.FC = () => {
  const { username } = useParams<{ username: string }>();
  const safeUsername = username ?? "";
  const navigate = useNavigate();
  const { user, setUser } = useUser();
  const { ref, inView } = useInView();
  const profileUpdateMutation = useUpdateProfile();
  const pinnedPost = usePinnedPost(safeUsername);
  const mediaUpload = useMediaUpload();
  const followMutation = useFollowUser();
  const unfollowMutation = useUnfollowUser();
  const muteMutation = useMuteUser();
  const unmuteMutation = useUnmuteUser();
  const blockMutation = useBlockUser();
  const unblockMutation = useUnblockUser();
  const [profileEditOpen, setProfileEditOpen] = useState(false);

  const [displayName, setDisplayName] = useState("");
  const [bio, setBio] = useState("");
  const [location, setLocation] = useState("");
  const [website, setWebsite] = useState("");
  const [birthDate, setBirthDate] = useState("");
  const [profilePictureFile, setProfilePictureFile] = useState<File | null>(null);
  const [bannerPictureFile, setBannerPictureFile] = useState<File | null>(null);
  const [profilePictureSrc, setProfilePictureSrc] = useState<string | undefined>();
  const [bannerSrc, setBannerSrc] = useState<string | undefined>();

  const profilePictureInputRef = useRef<HTMLInputElement>(null);
  const bannerPictureInputRef = useRef<HTMLInputElement>(null);

  const { data: fetchedProfileData, isLoading: profileLoading } = useFetchProfile(safeUsername);
  const profile = fetchedProfileData?.data;

  // Get user posts with pagination
  const {
    data: userPostsData,
    isLoading: isLoadingPosts,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage
  } = useGetUserPosts(safeUsername);

  const repliesQuery = useGetUserReplies(safeUsername);
  const mediaQuery = useGetUserMedia(safeUsername);

  useEffect(() => {
    if (inView && hasNextPage && !isFetchingNextPage) {
      fetchNextPage();
    }
  }, [inView, hasNextPage, isFetchingNextPage, fetchNextPage]);

  // Flatten all pages of posts
  const userPosts = userPostsData?.pages.flatMap(page => page.data.items) ?? [];

  // Populate the edit form whenever the fetched profile changes
  useEffect(() => {
    if (!profile) return;
    setDisplayName(profile.display_name);
    setBio(profile.bio);
    setLocation(profile.location);
    setWebsite(profile.website);
    setBirthDate(isUnsetBirthDate(profile.birth_date) ? "" : profile.birth_date);
    setProfilePictureSrc(getMediaUrl(profile.profile_picture_uuid));
    setBannerSrc(getMediaUrl(profile.banner_uuid));
    setProfilePictureFile(null);
    setBannerPictureFile(null);
  }, [profile]);

  // Should never happen, but it guards against a missing route param
  if (!username) {
    return <div>Username not found</div>;
  }

  const isCurrentUser = user.username === username;

  const handleFollowToggle = () => {
    if (profile?.is_following) {
      unfollowMutation.mutate(safeUsername, {
        onError: () => toast.error(`Failed to unfollow @${safeUsername}.`),
      });
    } else {
      followMutation.mutate(safeUsername, {
        onError: () => toast.error(`Failed to follow @${safeUsername}.`),
      });
    }
  };

  const handleMuteToggle = () => {
    if (profile?.is_muted) {
      unmuteMutation.mutate(safeUsername, {
        onError: () => toast.error(`Failed to unmute @${safeUsername}.`),
      });
    } else {
      muteMutation.mutate(safeUsername, {
        onError: () => toast.error(`Failed to mute @${safeUsername}.`),
      });
    }
  };

  const handleBlockToggle = () => {
    if (profile?.is_blocked) {
      unblockMutation.mutate(safeUsername, {
        onError: () => toast.error(`Failed to unblock @${safeUsername}.`),
      });
    } else {
      blockMutation.mutate(safeUsername, {
        onSuccess: () => toast.success(`Blocked @${safeUsername}.`),
        onError: () => toast.error(`Failed to block @${safeUsername}.`),
      });
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    try {
      let profilePictureUuid: string | undefined = profile?.profile_picture_uuid;
      let bannerUuid: string | undefined = profile?.banner_uuid;

      // If either a profile picture or banner was selected, upload them in a single request
      const filesToUpload = [];
      if (profilePictureFile) filesToUpload.push(profilePictureFile);
      if (bannerPictureFile) filesToUpload.push(bannerPictureFile);

      if (filesToUpload.length > 0) {
        const uploadResponse = await mediaUpload.mutateAsync(filesToUpload);

        // Server returns UUIDs in the same order as files were sent
        if (uploadResponse.data.uuids.length > 0) {
          let uuidIndex = 0;

          if (profilePictureFile) {
            profilePictureUuid = uploadResponse.data.uuids[uuidIndex++];
          }

          if (bannerPictureFile && uuidIndex < uploadResponse.data.uuids.length) {
            bannerUuid = uploadResponse.data.uuids[uuidIndex];
          }
        }
      }

      // Prepare the profile update payload
      const profileUpdatePayload = {
        display_name: displayName,
        bio: bio,
        birth_date: birthDate,
        website: website,
        location: location,
        profile_picture_uuid: profilePictureUuid,
        banner_uuid: bannerUuid,
      };

      // Update profile via API (single PATCH /users/me)
      const result = await profileUpdateMutation.mutateAsync(profileUpdatePayload);
      const updatedProfile = result.data;

      // Reflect the new media in local preview state
      setProfilePictureSrc(getMediaUrl(updatedProfile.profile_picture_uuid));
      setBannerSrc(getMediaUrl(updatedProfile.banner_uuid));

      // Update user context if this is the current user's profile
      if (isCurrentUser) {
        setUser({
          username: user.username,
          displayName: updatedProfile.display_name,
          profilePictureUUID: updatedProfile.profile_picture_uuid ?? '',
        });
      }

      toast.success("Profile updated successfully");
      setProfileEditOpen(false);
    } catch {
      console.error("Error updating profile");
      toast.error("Failed to update profile. Please try again.");
    }
  };

  const handleProfilePictureChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files[0]) {
      const file = e.target.files[0];
      setProfilePictureFile(file);
      setProfilePictureSrc(URL.createObjectURL(file));
    }
  };

  const handleBannerPictureChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files[0]) {
      const file = e.target.files[0];
      setBannerPictureFile(file);
      setBannerSrc(URL.createObjectURL(file));
    }
  };

  const formatDate = (dateString: string | null | undefined) => {
    if (!dateString || dateString === UNSET_BIRTH_DATE) return "";
    try {
      return format(parseISO(dateString), 'MMMM d, yyyy');
    } catch {
      return "";
    }
  };

  if (profileLoading) {
    return (
      <div className="w-full max-w-4xl mx-auto flex items-center justify-center py-20">
        <Loader2 className="h-8 w-8 animate-spin text-primary" />
      </div>
    );
  }

  return (
    <div className="w-full max-w-4xl mx-auto">
      {/* Banner */}
      <div className={`relative w-full h-48 md:h-64 overflow-hidden rounded-b-xl ${bannerSrc ? "" : "bg-muted"}`}>
        {bannerSrc && (
          <img
            src={bannerSrc}
            alt="Profile banner"
            className="w-full h-full object-cover"
          />
        )}
      </div>

      {/* Profile header */}
      <div className="relative px-4 pb-4">
        {/* Avatar - positioned to overlap the banner */}
        <div className="absolute -top-28 left-4 border-4 border-background rounded-full">
          <UserAvatar className="h-48 w-48" fallbackClassName="text-6xl" src={profilePictureSrc} name={displayName || username} username={username} />
        </div>

        {/* Profile action buttons */}
        <div className="flex justify-end items-center gap-2 mt-2">
          {!isCurrentUser && (
            <>
              <Button
                variant="outline"
                className="text-foreground"
                onClick={() => navigate(`/messages/new?user=${encodeURIComponent(safeUsername)}`)}
              >
                <MessageSquare className="h-4 w-4 mr-2" />
                Message
              </Button>
              <Button
                variant={profile?.is_following ? "outline" : "default"}
                className={profile?.is_following ? "text-foreground" : ""}
                onClick={handleFollowToggle}
                disabled={followMutation.isPending || unfollowMutation.isPending}
              >
                {profile?.is_following ? (
                  <>
                    <UserMinus className="h-4 w-4 mr-2" />
                    Unfollow
                  </>
                ) : (
                  <>
                    <UserPlus className="h-4 w-4 mr-2" />
                    Follow
                  </>
                )}
              </Button>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" size="icon" className="text-foreground">
                    <MoreHorizontal className="h-5 w-5" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-56 border border-muted">
                  <DropdownMenuItem onClick={handleMuteToggle} disabled={muteMutation.isPending || unmuteMutation.isPending}>
                    {profile?.is_muted ? <Volume2 className="h-4 w-4 mr-2" /> : <VolumeX className="h-4 w-4 mr-2" />}
                    {profile?.is_muted ? `Unmute @${safeUsername}` : `Mute @${safeUsername}`}
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem
                    onClick={handleBlockToggle}
                    disabled={blockMutation.isPending || unblockMutation.isPending}
                    className="text-destructive focus:text-destructive"
                  >
                    {profile?.is_blocked ? <ShieldCheck className="h-4 w-4 mr-2" /> : <ShieldBan className="h-4 w-4 mr-2" />}
                    {profile?.is_blocked ? `Unblock @${safeUsername}` : `Block @${safeUsername}`}
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </>
          )}
          {isCurrentUser && (
            <Button
              variant="outline"
              className="text-foreground"
              onClick={() => setProfileEditOpen(true)}
            >
              Edit profile
            </Button>
          )}
        </div>

        {/* Profile info */}
        <div className="mt-16">
          <h1 className="text-2xl font-bold text-primary">{profile?.display_name || displayName}</h1>
          <p className="text-muted-foreground">@{profile?.username || username}</p>
          <UserBadges badges={profile?.badges} className="mt-2" />

          <div className="mt-4 text-sm">
            <p className="whitespace-pre-wrap text-primary">{profile?.bio || bio}</p>

            <div className="flex flex-wrap gap-x-4 gap-y-2 mt-3 text-muted-foreground">
              {(profile?.location || location) && (
                <div className="flex items-center">
                  <MapPin className="mr-1 h-4 w-4" />
                  <span>{profile?.location || location}</span>
                </div>
              )}

              {(profile?.website || website) && (
                <a
                  href={profile?.website || website}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center hover:underline text-primary"
                >
                  <LinkIcon className="mr-1 h-4 w-4" />
                  {(profile?.website || website).replace(/^https?:\/\//, '')}
                </a>
              )}

              {formatDate(profile?.birth_date || birthDate) && (
                <div className="flex items-center">
                  <Calendar className="mr-1 h-4 w-4" />
                  <span>Born {formatDate(profile?.birth_date || birthDate)}</span>
                </div>
              )}
            </div>

            <div className="flex gap-4 mt-3">
              <Link to={`/profile/${encodeURIComponent(safeUsername)}/following`} className="hover:underline">
                <span className="font-semibold text-foreground">{profile?.following_count ?? 0}</span>{" "}
                <span className="text-muted-foreground">Following</span>
              </Link>
              <Link to={`/profile/${encodeURIComponent(safeUsername)}/followers`} className="hover:underline">
                <span className="font-semibold text-foreground">{profile?.followers_count ?? 0}</span>{" "}
                <span className="text-muted-foreground">Followers</span>
              </Link>
            </div>
          </div>
        </div>
      </div>

      {/* Private account notice */}
      {profile?.is_private && !isCurrentUser && !profile?.is_following && (
        <div className="mx-4 mt-4 flex items-center gap-3 rounded-xl border border-border bg-muted/50 px-4 py-3">
          <Lock className="h-5 w-5 text-muted-foreground shrink-0" />
          <p className="text-sm text-muted-foreground">
            These posts are protected. Follow <span className="font-medium text-foreground">@{profile.username}</span> to see their posts.
          </p>
        </div>
      )}

      {/* Tabs for posts, replies, media, etc. */}
      <Tabs defaultValue="posts" className="mt-4">
        <TabsList className="w-full grid grid-cols-4">
          <TabsTrigger value="posts">Posts</TabsTrigger>
          <TabsTrigger value="replies">Replies</TabsTrigger>
          <TabsTrigger value="media">Media</TabsTrigger>
          <TabsTrigger value="lists">Lists</TabsTrigger>
        </TabsList>

        <TabsContent value="posts" className="flex flex-col items-center mt-2 space-y-4">
          {pinnedPost.data?.data && (
            <div className="w-full max-w-xl rounded-lg border border-border bg-primary/5 p-3">
              <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Pinned post</p>
              <FeedPost post={pinnedPost.data.data} />
            </div>
          )}
          {isLoadingPosts ? (
            <div className="w-full flex items-center justify-center py-8">
              <Loader2 className="h-8 w-8 animate-spin text-primary" />
            </div>
          ) : userPosts.length > 0 ? (
            <>
              {userPosts.map(post => (
                <FeedPost key={post.id} post={post} />
              ))}

              {/* Loading indicator for infinite scroll */}
              <div ref={ref} className="w-full flex justify-center py-4">
                {isFetchingNextPage && (
                  <Loader2 className="h-8 w-8 animate-spin text-primary" />
                )}
              </div>
            </>
          ) : (
            <div className="text-center py-8 text-muted-foreground">
              No posts yet
            </div>
          )}
        </TabsContent>

        <TabsContent value="replies" className="flex flex-col items-center mt-2 space-y-4">
          <ProfileFeedTab query={repliesQuery} emptyText="No replies yet" />
        </TabsContent>

        <TabsContent value="media" className="flex flex-col items-center mt-2 space-y-4">
          <ProfileFeedTab query={mediaQuery} emptyText="No media yet" />
        </TabsContent>

        <TabsContent value="lists" className="mt-4 space-y-2">
          <ProfileLists username={safeUsername} />
        </TabsContent>
      </Tabs>

      {/* Edit Profile Dialog */}
      <Dialog open={profileEditOpen} onOpenChange={setProfileEditOpen}>
        <CustomDialogContent className="sm:max-w-md max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle className="text-primary">Edit Profile</DialogTitle>
          </DialogHeader>

          <form onSubmit={handleSubmit} className="space-y-4">
            {/* Banner with Profile Picture overlay */}
            <div className="relative">
              {/* Banner */}
              <div className={`relative w-full h-32 overflow-hidden rounded-lg ${bannerSrc ? "" : "bg-muted"}`}>
                {bannerSrc && (
                  <img
                    src={bannerSrc}
                    alt="Banner"
                    className="w-full h-full object-cover"
                  />
                )}

                {/* Banner upload button overlay */}
                <div
                  className="absolute inset-0 flex items-center justify-center bg-black/30 opacity-0 hover:opacity-100 transition-opacity cursor-pointer"
                  onClick={() => bannerPictureInputRef.current?.click()}
                >
                  <div className="flex flex-col items-center text-white">
                    <Camera className="h-6 w-6 mb-1" />
                    <span className="text-sm">Change Banner</span>
                  </div>
                </div>

                <input
                  ref={bannerPictureInputRef}
                  type="file"
                  accept="image/*"
                  className="hidden"
                  onChange={handleBannerPictureChange}
                />
              </div>

              {/* Profile Picture - positioned to overlap the banner */}
              <div className="absolute -bottom-10 left-4 border-4 border-background rounded-full">
                <div className="relative">
                  <UserAvatar className="h-20 w-20" fallbackClassName="text-2xl" src={profilePictureSrc} name={displayName || username} username={username} />

                  {/* Profile picture upload button overlay */}
                  <div
                    className="absolute inset-0 flex items-center justify-center bg-black/30 opacity-0 hover:opacity-100 transition-opacity cursor-pointer rounded-full"
                    onClick={() => profilePictureInputRef.current?.click()}
                  >
                    <Camera className="h-6 w-6 text-white" />
                  </div>

                  <input
                    ref={profilePictureInputRef}
                    type="file"
                    accept="image/*"
                    className="hidden"
                    onChange={handleProfilePictureChange}
                  />
                </div>
              </div>
            </div>

            {/* Add spacing to account for the overlapping profile picture */}
            <div className="h-10"></div>

            {/* Name */}
            <div className="space-y-2">
              <Label htmlFor="name" className="text-foreground">Name</Label>
              <Input
                id="displayName"
                className="text-foreground"
                name="displayName"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                required
                minLength={1}
                maxLength={50}
              />
              <p className="text-xs text-muted-foreground">1-50 characters</p>
            </div>

            {/* Username */}
            <div className="space-y-2">
              <Label htmlFor="username" className="text-foreground">Username</Label>
              <Input
                id="username"
                className="text-foreground"
                value={profile?.username || username}
                required
                disabled
              />
              <p className="text-xs text-muted-foreground">Username cannot be changed</p>
            </div>

            {/* Bio */}
            <div className="space-y-2">
              <Label htmlFor="bio" className="text-foreground">Bio</Label>
              <Textarea
                id="bio"
                className="text-foreground"
                name="bio"
                value={bio}
                onChange={(e) => setBio(e.target.value)}
                rows={3}
                maxLength={160}
              />
              <p className="text-xs text-muted-foreground">{bio.length}/160 characters</p>
            </div>

            {/* Location */}
            <div className="space-y-2">
              <Label htmlFor="location" className="text-foreground">Location</Label>
              <Input
                id="location"
                className="text-foreground"
                name="location"
                value={location}
                onChange={(e) => setLocation(e.target.value)}
                maxLength={30}
              />
              <p className="text-xs text-muted-foreground">Maximum 30 characters</p>
            </div>

            {/* Website */}
            <div className="space-y-2">
              <Label htmlFor="website" className="text-foreground">Website</Label>
              <Input
                id="website"
                className="text-foreground"
                name="website"
                value={website}
                onChange={(e) => setWebsite(e.target.value)}
                placeholder="https://"
                maxLength={50}
              />
              <p className="text-xs text-muted-foreground">Maximum 50 characters</p>
            </div>

            {/* Date of Birth */}
            <div className="space-y-2">
              <Label htmlFor="dateOfBirth" className="text-foreground">Date of Birth</Label>
              <Input
                id="dateOfBirth"
                className="text-foreground"
                type="date"
                value={birthDate}
                onChange={(e) => setBirthDate(e.target.value)}
              />
            </div>

            {/* Submit Buttons */}
            <div className="flex justify-end space-x-2 pt-4">
              <Button
                type="button"
                variant="outline"
                className="text-primary"
                onClick={() => setProfileEditOpen(false)}
                disabled={profileUpdateMutation.isPending}
              >
                Cancel
              </Button>
              <Button
                type="submit"
                disabled={profileUpdateMutation.isPending}
              >
                {profileUpdateMutation.isPending ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    Saving...
                  </>
                ) : "Save Changes"}
              </Button>
            </div>
          </form>
        </CustomDialogContent>
      </Dialog>
    </div>
  );
};

export default ProfilePage;

function ProfileLists({ username }: { username: string }) {
  const { data } = useUserLists(username);
  const lists = data?.data ?? [];
  if (lists.length === 0) {
    return <div className="text-center py-8 text-muted-foreground">No lists yet.</div>;
  }
  return (
    <>
      {lists.map((list) => (
        <Link key={list.id} to={`/lists/${list.id}`} className="block rounded-xl border border-border p-4 hover:bg-muted">
          <p className="font-semibold text-primary">{list.name}</p>
          <p className="text-sm text-muted-foreground truncate">{list.description || 'No description'}</p>
          <p className="text-xs text-muted-foreground mt-1">{list.member_count} members</p>
        </Link>
      ))}
    </>
  );
}

interface InfiniteFeedQuery {
  data?: InfiniteData<Envelope<PaginatedFeedResponse>>;
  isLoading: boolean;
  fetchNextPage: () => void;
  hasNextPage?: boolean;
  isFetchingNextPage: boolean;
}

// Renders an infinitely-scrolling FeedPost list for a paginated feed query.
function ProfileFeedTab({ query, emptyText }: { query: InfiniteFeedQuery; emptyText: string }) {
  const { ref, inView } = useInView();
  const { data, isLoading, fetchNextPage, hasNextPage, isFetchingNextPage } = query;

  useEffect(() => {
    if (inView && hasNextPage && !isFetchingNextPage) {
      fetchNextPage();
    }
  }, [inView, hasNextPage, isFetchingNextPage, fetchNextPage]);

  const posts = data?.pages.flatMap(page => page.data.items) ?? [];

  if (isLoading) {
    return (
      <div className="w-full flex items-center justify-center py-8">
        <Loader2 className="h-8 w-8 animate-spin text-primary" />
      </div>
    );
  }

  if (posts.length === 0) {
    return <div className="w-full text-center py-8 text-muted-foreground">{emptyText}</div>;
  }

  return (
    <>
      {posts.map(post => (
        <FeedPost key={post.id} post={post} />
      ))}
      <div ref={ref} className="w-full flex justify-center py-4">
        {isFetchingNextPage && <Loader2 className="h-8 w-8 animate-spin text-primary" />}
      </div>
    </>
  );
}
