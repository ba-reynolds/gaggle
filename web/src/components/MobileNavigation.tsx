import { Bookmark, Home, PenSquare, Settings, User } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useUser } from "@/contexts/UserContext";
import { useNavigate } from "react-router-dom";

interface MobileNavigationProps {
  onComposeClick?: () => void;
}

export default function MobileNavigation({ onComposeClick }: MobileNavigationProps) {
  const navigate = useNavigate();
  const { user } = useUser();

  return (
    <div className="fixed bottom-0 left-0 right-0 bg-background border-t border-border md:hidden z-10">
      <div className="flex justify-around items-center p-2">
        <Button
          variant="ghost"
          size="icon"
          className="flex flex-col items-center justify-center text-muted-foreground"
          onClick={() => navigate("/")}
        >
          <Home className="h-6 w-6" />
          <span className="text-xs mt-1">Home</span>
        </Button>

        <Button
          variant="ghost"
          size="icon"
          className="flex flex-col items-center justify-center text-muted-foreground"
          onClick={() => navigate("/bookmarks")}
        >
          <Bookmark className="h-6 w-6" />
          <span className="text-xs mt-1">Saved</span>
        </Button>

        <Button
          className="rounded-full h-12 w-12 flex items-center justify-center shadow-md"
          onClick={onComposeClick}
        >
          <PenSquare className="h-6 w-6" />
        </Button>

        <Button
          variant="ghost"
          size="icon"
          className="flex flex-col items-center justify-center text-muted-foreground"
          onClick={() => navigate(`/profile/${user.username}`)}
        >
          <User className="h-6 w-6" />
          <span className="text-xs mt-1">Profile</span>
        </Button>

        <Button
          variant="ghost"
          size="icon"
          className="flex flex-col items-center justify-center text-muted-foreground"
          onClick={() => navigate("/settings")}
        >
          <Settings className="h-6 w-6" />
          <span className="text-xs mt-1">Settings</span>
        </Button>
      </div>
    </div>
  );
}