import AppSplash from "@/components/AppSplash";
import ComposeContent from "@/components/ComposeContent";
import MobileNavigation from "@/components/MobileNavigation";
import { UserAvatar } from "@/components/UserAvatar";
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
import { useI18n } from "@/contexts/I18nContext";
import { useNotifications } from "@/contexts/NotificationsContext";
import { useTrends, useSuggestedUsers } from "@/hooks/useSearch";
import { useUser } from "@/contexts/UserContext";
import { useDmUnreadCount } from "@/hooks/useDms";
import { notificationsInfiniteOptions } from "@/hooks/useNotifications";
import { getConversations } from "@/api/dms";
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
  MessageSquare,
  AtSign,
  PenSquare
} from "lucide-react";
import { useEffect, useState } from "react";
import { NavLink, Navigate, useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';

interface SocialMediaLayoutProps {
  children: React.ReactNode;
}

export default function SocialMediaLayout({
  children,
}: SocialMediaLayoutProps) {
  const { user } = useUser();
  const { mutate: logout } = useLogoutMutation();
  const { t } = useI18n();
  const { token } = useAuth();
  const { unreadCount } = useNotifications();
  const navigate = useNavigate();
  const [isComposing, setIsComposing] = useState(false);
  const [followingUsers, setFollowingUsers] = useState<{ [key: string]: boolean }>({});
  const [search, setSearch] = useState('');
  const isAuthenticated = typeof token === 'string';
  const trends = useTrends(isAuthenticated);
  const suggested = useSuggestedUsers(10, isAuthenticated);
  const dmUnread = useDmUnreadCount(isAuthenticated);
  const queryClient = useQueryClient();

  // Warm the cache for the destinations users navigate to most, so the first
  // visit (Notifications / Messages) renders instantly instead of showing a
  // skeleton while the server round-trips.
  useEffect(() => {
    if (typeof token !== 'string') return;
    void queryClient.prefetchInfiniteQuery(notificationsInfiniteOptions);
    void queryClient.prefetchQuery({ queryKey: ['dm-conversations'], queryFn: getConversations });
  }, [token, queryClient]);

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
        flex items-center justify-center lg:justify-start gap-x-4 w-full p-2 rounded-md
        ${isActive ? "bg-foreground/10 hover:bg-foreground/15" : "hover:bg-foreground/5"}
        text-gray-800 dark:text-gray-100
      `}
    >
      <span className="relative"><Icon className="h-5 w-5" />{badge ? <span className="absolute -right-2 -top-2 rounded-full bg-destructive px-1.5 text-[10px] leading-4 text-destructive-foreground">{badge > 99 ? '99+' : badge}</span> : null}</span>
      <span className="hidden lg:inline">{label}</span>
    </NavLink>
  );



  if (token === undefined) {
    return <AppSplash />;
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
          <div className="hidden md:block md:col-span-2 lg:col-span-2 min-w-0">
            <div className="sticky top-4 space-y-6 min-w-0">
              {/* App Logo */}
              <div className="flex items-center justify-center lg:justify-start gap-2 mb-6 min-w-0">
                <span className="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-xl bg-primary">
                  <img src="/gaggle-goose.png" alt="Gaggle" width={160} height={160} fetchPriority="high" decoding="async" className="h-full w-full object-cover object-bottom" />
                </span>
                <span className="text-xl font-bold flex-1 min-w-0 truncate hidden lg:inline text-primary">Gaggle</span>
              </div>

              {/* Navigation */}
              <nav className="space-y-1">
                <NavItem icon={Home} label={t("nav.home")} to="/" />
                <NavItem icon={Compass} label={t("nav.explore")} to="/explore" />
                <NavItem icon={Bookmark} label={t("nav.bookmarks")} to="/bookmarks" />
                <NavItem icon={ListIcon} label={t("nav.lists")} to="/lists" />
                <NavItem icon={MessageSquare} label={t("nav.messages")} to="/messages" badge={dmUnread.data?.data?.unread_count ?? 0} />
                <NavItem icon={Bell} label={t("nav.notifications")} to="/notifications" badge={unreadCount} />
                <NavItem icon={AtSign} label={t("nav.mentions")} to="/mentions" />
                <NavItem icon={User} label={t("nav.profile")} to={`/profile/${user.username}`} />
                <NavItem icon={Settings} label={t("nav.settings")} to="/settings" />
                {user.isAdmin && <NavItem icon={Shield} label={t("nav.admin")} to="/admin" />}


                <Button
                  className="w-full rounded-full mt-4 flex items-center justify-center md:w-12 md:h-12 md:px-0 md:mx-auto lg:w-full lg:mx-0"
                  onClick={() => setIsComposing(true)}
                >
                  <PenSquare className="h-5 w-5 hidden md:block lg:hidden" />
                  <span className="hidden lg:inline">{t("app.post")}</span>
                </Button>
              </nav>

              {/* Dropdown menu for the user @ left sidebar */}
              <div className="mt-auto">
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="ghost" className="w-full flex items-center justify-center lg:justify-between text-primary">
                      <div className="flex items-center">
                        <UserAvatar className="h-8 w-8 mr-2" src={getMediaUrl(user.profilePictureUUID)} name={user.displayName} username={user.username} />
                        <div className="hidden lg:block text-left">
                          <p className="text-sm font-medium text-primary">{user.displayName}</p>
                          <p className="text-xs text-muted-foreground">@{user.username}</p>
                        </div>
                      </div>
                      <MoreHorizontal className="h-4 w-4 hidden lg:block" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="w-56 border border-muted">
                    <DropdownMenuLabel>{t("nav.myAccount")}</DropdownMenuLabel>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem onClick={() => navigate(`/profile/${user.username}`)}>
                      <User className="h-4 w-4 mr-2" />
                      {t("nav.profile")}
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => navigate("/settings")}>
                      <Settings className="h-4 w-4 mr-2" />
                      {t("nav.settings")}
                    </DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem onClick={() => logout()}>
                      <LogOut className="h-4 w-4 mr-2" />
                      {t("nav.logOut")}
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            </div>
          </div>

          {/* Main Content */}
          <div className="col-span-12 md:col-span-10 lg:col-span-7 border-x border-border bg-background/25 min-h-screen self-start flex flex-col px-6 pb-16 md:pb-0">
            <div className="flex-1 min-h-0">
              {children}
            </div>
          </div>

          {/* Right Sidebar */}
          <div className="hidden lg:block lg:col-span-3">
            <div className="py-2 sticky top-4 space-y-6">
              {/* Search */}
              <Input
                placeholder={t("app.search")}
                className="bg-transparent p-6 rounded-full focus-visible:ring-0 focus-visible:ring-offset-0 text-primary"
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' && search.trim()) navigate(`/search?q=${encodeURIComponent(search.trim())}`);
                }}
              />

              {/* Trending */}
              <div className="bg-muted rounded-xl p-4">
                <h3 className="font-bold text-xl mb-4 text-primary">{t("trends.title")}</h3>
                <div className="space-y-2">
                  {trends.data?.slice(0, 5).map((trend) => (
                    <button key={trend.name} className="block w-full rounded p-2 text-left hover:bg-accent" onClick={() => navigate(`/hashtags/${trend.name}`)}>
                      <p className="text-xs text-muted-foreground">{t("trends.now")}</p>
                      <p className="font-semibold text-primary">#{trend.name}</p>
                      <p className="text-xs text-muted-foreground">{t("trends.posts", { count: trend.count })}</p>
                    </button>
                  ))}
                  {!trends.isLoading && !trends.data?.length && <p className="text-sm text-muted-foreground">{t("trends.empty")}</p>}
                </div>
                <Button variant="ghost" className="w-full mt-2 text-primary justify-start p-2" onClick={() => navigate(`/explore?tab=trending`)}>
                  {t("trends.showMore")}
                </Button>
              </div>

              {/* Who to follow */}
              <div className="bg-muted rounded-xl p-4">
                <h3 className="font-bold text-xl mb-4 text-primary">{t("whoToFollow.title")}</h3>
                <div className="space-y-4">
                  {suggested.data?.items.slice(0, 3).map((profile) => {
                    const username = profile.username;
                    const isFollowing = followingUsers[username] || false;
                    return (
                      <div key={username} className="flex items-center justify-between gap-3">
                        <UserHoverCard
                          name={profile.display_name}
                          username={username}
                          userDescription={profile.bio}
                          followers={profile.followers_count}
                          following={profile.following_count}
                          isFollowing={isFollowing}
                          onFollowToggle={handleFollowToggle(username)}
                        >
                          <div className="flex min-w-0 items-center">
                            <UserAvatar className="mr-2 h-10 w-10 shrink-0" src={getMediaUrl(profile.profile_picture_uuid)} name={profile.display_name} username={profile.username} />
                            <div className="min-w-0">
                              <p className="truncate text-sm font-semibold text-primary">{profile.display_name}</p>
                              <p className="truncate text-xs text-muted-foreground">@{username}</p>
                            </div>
                          </div>
                        </UserHoverCard>
                        <Button
                          size="sm"
                          className="rounded-full"
                          variant={isFollowing ? "outline" : "default"}
                          onClick={handleFollowToggle(username)}
                        >
                          {isFollowing ? t("whoToFollow.following") : t("whoToFollow.follow")}
                        </Button>
                      </div>
                    )
                  })}
                  {!suggested.isLoading && !suggested.data?.items.length && (
                    <p className="text-sm text-muted-foreground">{t("whoToFollow.empty")}</p>
                  )}
                </div>
                <Button variant="ghost" className="w-full mt-2 text-primary justify-start p-2" onClick={() => navigate('/explore')}>
                  {t("trends.showMore")}
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
            <DialogTitle className="text-primary">{t("nav.newPost")}</DialogTitle>
          </DialogHeader>

          <ComposeContent
            onSubmit={handleNewPost}
            placeholder={t("composer.placeholder")}
            submitLabel={t("app.post")}
            textareaHeight="h-32"
          />
        </CustomDialogContent>
      </Dialog>
      <MobileNavigation onComposeClick={() => setIsComposing(true)} />
    </div>
  );
}
