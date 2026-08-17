import type { LucideIcon } from "lucide-react";
import * as LucideIcons from "lucide-react";
import type { UserBadge } from "@/types/api";
import { Badge } from "@/components/ui/badge";
import { TooltipProvider, Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";

interface UserBadgesProps {
  badges?: UserBadge[];
  className?: string;
}

const UserBadges: React.FC<UserBadgesProps> = ({ badges, className }) => {
  if (!badges || badges.length === 0) return null;

  return (
    <TooltipProvider delayDuration={100}>
      <div className={`flex flex-wrap items-center gap-1.5 ${className ?? ""}`}>
        {badges.map((badge) => {
          const Icon: LucideIcon = (LucideIcons as unknown as Record<string, LucideIcon>)[badge.icon] ?? LucideIcons.Award;
          return (
            <Tooltip key={badge.id}>
              <TooltipTrigger asChild>
                <Badge variant="outline" className="cursor-help">
                  <Icon className="text-primary" />
                  {badge.label}
                </Badge>
              </TooltipTrigger>
              <TooltipContent side="bottom" className="pointer-events-none">
                <p>{badge.description}</p>
              </TooltipContent>
            </Tooltip>
          );
        })}
      </div>
    </TooltipProvider>
  );
};

export default UserBadges;