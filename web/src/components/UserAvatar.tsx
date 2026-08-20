import { User } from "lucide-react"

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { cn } from "@/lib/utils"

const AVATAR_COLORS = [
  "#D32F2F",
  "#E64A19",
  "#F0932B",
  "#6A1B9A",
  "#283593",
  "#1565C0",
  "#0277BD",
  "#00838F",
  "#00695C",
  "#2E7D32",
  "#AD1457",
  "#C2185B",
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
  const backgroundColor = AVATAR_COLORS[hashString(seed) % AVATAR_COLORS.length]
  const initial = Array.from(name || username || "")[0]?.toUpperCase()

  return (
    <Avatar className={className}>
      {src && <AvatarImage src={src} alt={alt ?? name ?? username} />}
      <AvatarFallback
        style={{ backgroundColor }}
        className={cn(
          "font-bold text-white select-none",
          fallbackClassName
        )}
      >
        {initial || <User className="h-2/5 w-2/5" aria-hidden />}
      </AvatarFallback>
    </Avatar>
  )
}