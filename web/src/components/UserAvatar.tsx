import { User } from "lucide-react"

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { cn } from "@/lib/utils"

const AVATAR_COLORS = [
  "#3b82f6",
  "#0ea5e9",
  "#14b8a6",
  "#10b981",
  "#84cc16",
  "#eab308",
  "#f59e0b",
  "#f97316",
  "#ef4444",
  "#ec4899",
  "#a855f7",
  "#6366f1",
]

function hashString(str: string): number {
  let hash = 0
  for (let i = 0; i < str.length; i++) {
    hash = ((hash << 5) - hash + str.charCodeAt(i)) | 0
  }
  return Math.abs(hash)
}

export interface UserAvatarProps {
  src?: string
  name?: string
  username?: string
  alt?: string
  className?: string
  fallbackClassName?: string
}

export function UserAvatar({
  src,
  name,
  username,
  alt,
  className,
  fallbackClassName,
}: UserAvatarProps) {
  const seed = username || name || ""
  const color = AVATAR_COLORS[hashString(seed) % AVATAR_COLORS.length]
  const initial = Array.from(name || username || "")[0]?.toUpperCase()

  return (
    <Avatar className={className}>
      {src && <AvatarImage src={src} alt={alt ?? name ?? username} />}
      <AvatarFallback
        style={{
          backgroundColor: `color-mix(in oklab, ${color} 18%, transparent)`,
          color: `color-mix(in oklab, ${color} 72%, var(--foreground))`,
        }}
        className={cn(
          "font-semibold select-none",
          fallbackClassName
        )}
      >
        {initial || <User className="h-2/5 w-2/5" aria-hidden />}
      </AvatarFallback>
    </Avatar>
  )
}