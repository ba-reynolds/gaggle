import { Bell, Bookmark, Compass, Home, MessageSquare, PenSquare, Settings, User } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useUser } from "@/contexts/UserContext";
import { useI18n } from "@/contexts/I18nContext";
import { useNavigate } from "react-router-dom";
import { useNotifications } from "@/contexts/NotificationsContext";
import { useDmUnreadCount } from "@/hooks/useDms";

interface MobileNavigationProps {
  onComposeClick?: () => void;
}

export default function MobileNavigation({ onComposeClick }: MobileNavigationProps) {
  const navigate = useNavigate();
  const { user } = useUser();
  const { t } = useI18n();
  const { unreadCount } = useNotifications();
  const dmUnread = useDmUnreadCount(true);

  return (
    <div className="fixed bottom-0 left-0 right-0 bg-background border-t border-border md:hidden z-10">
      <div className="flex items-center p-2">
        <div className="flex flex-1 justify-around items-center">
          <Button
            variant="ghost"
            size="icon"
            className="flex flex-col items-center justify-center text-muted-foreground"
            onClick={() => navigate("/")}
          >
            <Home className="h-6 w-6" />
            <span className="text-xs mt-1">{t("nav.home")}</span>
          </Button>

          <Button
            variant="ghost"
            size="icon"
            className="flex flex-col items-center justify-center text-muted-foreground"
            onClick={() => navigate("/explore")}
          >
            <Compass className="h-6 w-6" />
            <span className="text-xs mt-1">{t("nav.explore")}</span>
          </Button>

          <Button
            variant="ghost"
            size="icon"
            className="relative flex flex-col items-center justify-center text-muted-foreground"
            onClick={() => navigate("/notifications")}
          >
            <Bell className="h-6 w-6" />
            {unreadCount > 0 && <span className="absolute right-1 top-0 h-2 w-2 rounded-full bg-destructive" />}
            <span className="text-xs mt-1">{t("nav.alerts")}</span>
          </Button>
        </div>

        <Button
          className="rounded-full h-12 w-12 shrink-0 flex items-center justify-center shadow-md"
          onClick={onComposeClick}
        >
          <PenSquare className="h-6 w-6" />
        </Button>

        <div className="flex flex-1 justify-around items-center">
          <Button
            variant="ghost"
            size="icon"
            className="relative flex flex-col items-center justify-center text-muted-foreground"
            onClick={() => navigate("/messages")}
          >
            <MessageSquare className="h-6 w-6" />
            {(dmUnread.data?.data?.unread_count ?? 0) > 0 && <span className="absolute right-0 top-0 h-2 w-2 rounded-full bg-destructive" />}
            <span className="text-xs mt-1">{t("nav.dms")}</span>
          </Button>

          <Button
            variant="ghost"
            size="icon"
            className="flex flex-col items-center justify-center text-muted-foreground"
            onClick={() => navigate("/bookmarks")}
          >
            <Bookmark className="h-6 w-6" />
            <span className="text-xs mt-1">{t("nav.saved")}</span>
          </Button>

          <Button
            variant="ghost"
            size="icon"
            className="flex flex-col items-center justify-center text-muted-foreground"
            onClick={() => navigate(`/profile/${user.username}`)}
          >
            <User className="h-6 w-6" />
            <span className="text-xs mt-1">{t("nav.profile")}</span>
          </Button>

          <Button
            variant="ghost"
            size="icon"
            className="flex flex-col items-center justify-center text-muted-foreground"
            onClick={() => navigate("/settings")}
          >
            <Settings className="h-6 w-6" />
            <span className="text-xs mt-1">{t("nav.settings")}</span>
          </Button>
        </div>
      </div>
    </div>
  );
}
