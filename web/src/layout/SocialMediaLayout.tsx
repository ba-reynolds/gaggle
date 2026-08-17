import ComposeContent from "@/components/ComposeContent";
import MobileNavigation from "@/components/MobileNavigation";
import { ThemeToggle } from "@/components/ThemeToggle";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { CustomDialogContent } from "@/components/ui/custom-dialog";
import { DialogHeader } from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import UserHoverCard from "@/components/UserHoverCard";
import { useAuth } from "@/contexts/AuthContext";
import { useNotifications } from "@/contexts/NotificationsContext";
import { useTrends, useSuggestedUsers } from "@/hooks/useSearch";
import { useUser } from "@/contexts/UserContext";
import { useDmUnreadCount } from "@/hooks/useDms";
import { useLogoutMutation } from "@/hooks/useAuth";
import { getMediaUrl } from "@/util/media";
import { Dialog, DialogTitle } from "@radix-ui/react-dialog";
import {
  Bookmark,
  Home,
  LogOut,
  MoreHorizontal,
  Settings,
  Bell,
  User,
  Shield,
  Compass,
  List as ListIcon,
  MessageSquare
} from "lucide-react";
import { useState } from "react";
import { NavLink, Navigate, useNavigate } from 'react-router-dom';

interface SocialMediaLayoutProps {
  children: React.ReactNode;
}

export default function SocialMediaLayout({
  children,
}: SocialMediaLayoutProps) {
  const { user } = useUser();
  const { mutate: logout } = useLogoutMutation();
  const { token } = useAuth();
  const { unreadCount } = useNotifications();
  const navigate = useNavigate();
  const [isComposing, setIsComposing] = useState(false);
  const [followingUsers, setFollowingUsers] = useState<{ [key: string]: boolean }>({});
  const [search, setSearch] = useState('');
  const trends = useTrends();
  const suggested = useSuggestedUsers(10);
  const dmUnread = useDmUnreadCount();

  const handleNewPost = () => {
    setIsComposing(false);
  };

  const handleFollowToggle = (userId: string) => (e?: React.MouseEvent) => {
    e?.stopPropagation();
    setFollowingUsers(prev => ({
      ...prev,
      [userId]: !prev[userId]
    }));
  };


  const NavItem = ({ icon: Icon, label, to, badge }: { icon: React.ElementType, label: string, to: string, badge?: number }) => (
    <NavLink
      to={to}
      className={({ isActive }) => `
        flex justify-start items-center space-x-4 w-full p-2 rounded-md
        ${isActive ? "bg-foreground/10 hover:bg-foreground/15" : "hover:bg-foreground/5"}
        text-gray-800 dark:text-gray-100
      `}
    >
      <span className="relative"><Icon className="h-5 w-5" />{badge ? <span className="absolute -right-2 -top-2 rounded-full bg-destructive px-1.5 text-[10px] leading-4 text-destructive-foreground">{badge > 99 ? '99+' : badge}</span> : null}</span>
      <span className="hidden md:inline">{label}</span>
    </NavLink>
  );



  if (token === undefined) {
    return <div>Loading...</div>;
  }

  if (token === null) {
    return <Navigate to="/login" replace />;
  }

  return (
    // container of the whole app, `min-height: 100vh`
    <div className="min-h-screen bg-sidebar relative">
      {/* `mx-auto` centers the container horizontally by applying automatic
      margins to the left and right. `px-4` applies horizontal padding of 1rem
      on both sides, ensuring the content doesn't touch the edges of the
      container, especially on small screens. */}
      <div className="container mx-auto px-4">

        {/* `grid-cols-12` divides the grid into 12 equal columns, and `gap-4`
        sets a consistent gap of 1rem between grid items. Common startegy for
        responsive layouts, where children of this grid container will use classes
        like `col-span-X` or `col-start-X` to control their placement across
        the 12-column structure. `col-span-*` classes specify how many columns
        an element should span within a parent container that defines a grid
        What makes this powerful is that Tailwind allows responsive column spans
        using breakpoint prefixes like `md:` and `lg:` */}
        <div className="grid grid-cols-12 gap-4">

          {/* Left Sidebar */}
          <div className="col-span-2 md:col-span-3 lg:col-span-2">
            <div className="sticky top-4 space-y-6">
              {/* App Logo */}
              <div className="flex items-center mb-6">
                <div className="w-10 h-10 bg-primary rounded-full flex items-center justify-center">
                  <span className="text-primary-foreground font-bold text-xl">G</span>
                </div>
                <span className="text-xl font-bold ml-2 hidden md:inline text-primary">GopherSocial</span>
              </div>

              {/* Navigation */}
              <nav className="space-y-1">
                <NavItem icon={Home} label="Home" to="/" />
                <NavItem icon={Compass} label="Explore" to="/explore" />
                <NavItem icon={Bookmark} label="Bookmarks" to="/bookmarks" />
                <NavItem icon={ListIcon} label="Lists" to="/lists" />
                <NavItem icon={MessageSquare} label="Messages" to="/messages" badge={dmUnread.data?.data?.unread_count ?? 0} />
                <NavItem icon={Bell} label="Notifications" to="/notifications" badge={unreadCount} />
                <NavItem icon={User} label="Profile" to={`/profile/${user.username}`} />
                <NavItem icon={Settings} label="Settings" to="/settings" />
                {user.isAdmin && <NavItem icon={Shield} label="Admin" to="/admin" />}


                <Button
                  className="w-full rounded-full mt-4 md:py-6 flex items-center justify-center"
                  onClick={() => setIsComposing(true)}
                >
                  <span className="hidden md:inline">Post</span>
                </Button>
              </nav>

              {/* Dropdown menu for the user @ left sidebar */}
              <div className="mt-auto">
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="ghost" className="w-full flex items-center justify-between text-primary">
                      <div className="flex items-center">
                        <Avatar className="h-8 w-8 mr-2">
                          <AvatarImage src={getMediaUrl(user.profilePictureUUID)} />
                          <AvatarFallback>{user.displayName[0]}</AvatarFallback>
                        </Avatar>
                        <div className="hidden md:block text-left">
                          <p className="text-sm font-medium text-primary">{user.displayName}</p>
                          <p className="text-xs text-muted-foreground">@{user.username}</p>
                        </div>
                      </div>
                      <MoreHorizontal className="h-4 w-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="w-56 border border-muted">
                    <DropdownMenuLabel>My Account</DropdownMenuLabel>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem onClick={() => navigate(`/profile/${user.username}`)}>
                      <User className="h-4 w-4 mr-2" />
                      Profile
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => navigate("/settings")}>
                      <Settings className="h-4 w-4 mr-2" />
                      Settings
                    </DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem onClick={() => logout()}>
                      <LogOut className="h-4 w-4 mr-2" />
                      Log out
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            </div>
          </div>

          {/* Main Content */}
          <div className="col-span-10 md:col-span-9 lg:col-span-7 border-x border-border bg-background/25 min-h-screen px-6">
            {children}
          </div>

          {/* Right Sidebar */}
          <div className="md:block md:col-span-0 lg:col-span-3">
            <div className="py-2 sticky top-4 space-y-6">
              {/* Search */}
              <Input
                placeholder="Search"
                className="bg-transparent p-6 rounded-full focus-visible:ring-0 focus-visible:ring-offset-0 text-primary"
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' && search.trim()) navigate(`/search?q=${encodeURIComponent(search.trim())}`);
                }}
              />

              <div className="bg-muted rounded-xl p-4 mb-4">
                <h3 className="font-bold text-xl mb-4 text-primary">Appearance</h3>
                <ThemeToggle />
              </div>

              {/* Trending */}
              <div className="bg-muted rounded-xl p-4">
                <h3 className="font-bold text-xl mb-4 text-primary">Trending</h3>
                <div className="space-y-2">
                  {trends.data?.slice(0, 5).map((trend) => (
                    <button key={trend.name} className="block w-full rounded p-2 text-left hover:bg-accent" onClick={() => navigate(`/hashtags/${trend.name}`)}>
                      <p className="text-xs text-muted-foreground">Trending now</p>
                      <p className="font-semibold text-primary">#{trend.name}</p>
                      <p className="text-xs text-muted-foreground">{trend.count} posts</p>
                    </button>
                  ))}
                  {!trends.isLoading && !trends.data?.length && <p className="text-sm text-muted-foreground">No trends yet.</p>}
                </div>
                <Button variant="ghost" className="w-full mt-2 text-primary justify-start p-2">
                  Show more
                </Button>
              </div>

              {/* Who to follow */}
              <div className="bg-muted rounded-xl p-4">
                <h3 className="font-bold text-xl mb-4 text-primary">Who to follow</h3>
                <div className="space-y-4">
                  {suggested.data?.items.slice(0, 3).map((profile) => {
                    const username = profile.username;
                    const isFollowing = followingUsers[username] || false;
                    return (
                      <div key={username} className="flex items-center justify-between">
                        <UserHoverCard
                          name={profile.display_name}
                          username={username}
                          userDescription={profile.bio}
                          followers={profile.followers_count}
                          following={profile.following_count}
                          isFollowing={isFollowing}
                          onFollowToggle={handleFollowToggle(username)}
                        >
                          <div className="flex items-center">
                            <Avatar className="h-10 w-10 mr-2">
                              <AvatarImage src={getMediaUrl(profile.profile_picture_uuid)} />
                              <AvatarFallback>{profile.display_name.charAt(0)}</AvatarFallback>
                            </Avatar>
                            <div>
                              <p className="font-semibold text-sm text-primary">{profile.display_name}</p>
                              <p className="text-xs text-muted-foreground">@{username}</p>
                            </div>
                          </div>
                        </UserHoverCard>
                        <Button
                          size="sm"
                          className="rounded-full"
                          variant={isFollowing ? "outline" : "default"}
                          onClick={handleFollowToggle(username)}
                        >
                          {isFollowing ? "Following" : "Follow"}
                        </Button>
                      </div>
                    )
                  })}
                  {!suggested.isLoading && !suggested.data?.items.length && (
                    <p className="text-sm text-muted-foreground">No suggestions right now.</p>
                  )}
                </div>
                <Button variant="ghost" className="w-full mt-2 text-primary justify-start p-2" onClick={() => navigate('/explore')}>
                  Show more
                </Button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <Dialog open={isComposing} onOpenChange={setIsComposing}>
        <CustomDialogContent
          className="sm:max-w-xl max-h-[90vh] overflow-y-auto bg-card"
        >
          <DialogHeader className="mb-2">
            <DialogTitle className="text-primary">New Post</DialogTitle>
          </DialogHeader>

          <ComposeContent
            onSubmit={handleNewPost}
            placeholder="What's happening?"
            submitLabel="Post"
            textareaHeight="h-32"
          />
        </CustomDialogContent>
      </Dialog>
      <MobileNavigation onComposeClick={() => setIsComposing(true)} />
    </div>
  );
}
