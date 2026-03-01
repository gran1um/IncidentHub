import { Bot } from "lucide-react";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { cn } from "@/lib/utils";

type UserAvatarProps = {
  name?: string | null;
  avatar?: string | null;
  fallback?: string | null;
  className?: string;
  imageClassName?: string;
  fallbackClassName?: string;
  authorKind?: string | null;
  authorAvatarKey?: string | null;
};

function initialsFromValue(value?: string | null): string {
  const source = String(value || "").trim();
  if (!source) return "U";
  const words = source.split(/\s+/).filter(Boolean);
  if (words.length === 1) {
    return words[0].slice(0, 2).toUpperCase();
  }
  const first = words[0]?.[0] || "";
  const second = words[1]?.[0] || "";
  return `${first}${second}`.toUpperCase() || "U";
}

function isSyntheticAI(authorKind?: string | null, authorAvatarKey?: string | null, fallback?: string | null): boolean {
  const kind = String(authorKind || "").trim().toLowerCase();
  const avatarKey = String(authorAvatarKey || "").trim().toLowerCase();
  const fallbackValue = String(fallback || "").trim().toLowerCase();
  return kind === "ai_agent" || avatarKey === "ai-agent" || fallbackValue.startsWith("ai-agent:");
}

export function UserAvatar({
  name,
  avatar,
  fallback,
  className,
  imageClassName,
  fallbackClassName,
  authorKind,
  authorAvatarKey,
}: UserAvatarProps) {
  const initials = initialsFromValue(name || fallback);
  const syntheticAI = isSyntheticAI(authorKind, authorAvatarKey, fallback);
  return (
    <Avatar className={cn("shrink-0", className)}>
      {!syntheticAI ? <AvatarImage src={avatar || undefined} className={cn("object-cover", imageClassName)} /> : null}
      <AvatarFallback
        className={cn(
          syntheticAI
            ? "bg-[rgba(34,197,94,0.16)] text-[#86efac] border border-[rgba(34,197,94,0.28)]"
            : "bg-primary/10 text-primary font-semibold",
          fallbackClassName,
        )}
      >
        {syntheticAI ? <Bot size={16} strokeWidth={2.1} /> : initials}
      </AvatarFallback>
    </Avatar>
  );
}
