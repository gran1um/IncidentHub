import { format } from "date-fns";

import { UserAvatar } from "@/components/user-avatar";
import { useUser } from "@/lib/api";

function isSyntheticCommentAuthor(comment: any): boolean {
  const authorKind = String(comment?.authorKind || comment?.actor?.authorKind || "").trim().toLowerCase();
  const authorId = String(comment?.authorId || comment?.actor?.authorId || "").trim().toLowerCase();
  const avatarKey = String(comment?.authorAvatarKey || comment?.actor?.authorAvatarKey || "").trim().toLowerCase();
  return authorKind === "ai_agent" || avatarKey === "ai-agent" || authorId.startsWith("ai-agent:");
}

export function CommentItem({ comment }: { comment: any }) {
  const syntheticAuthor = isSyntheticCommentAuthor(comment);
  const { data: author } = useUser(syntheticAuthor ? "" : (comment.authorId || ""));
  const authorName = comment.authorName || comment.actor?.authorName || author?.name || comment.authorId || "Unknown";

  return (
    <div className="flex items-start gap-3" data-testid={`comment-${comment.id}`}>
      <UserAvatar
        name={authorName}
        avatar={syntheticAuthor ? undefined : author?.avatar}
        fallback={comment.authorId}
        authorKind={comment.authorKind || comment.actor?.authorKind}
        authorAvatarKey={comment.authorAvatarKey || comment.actor?.authorAvatarKey}
        className="h-9 w-9 rounded-xl border border-[#2a2c3c]"
        fallbackClassName="text-sm font-bold"
      />
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="text-sm font-bold" data-testid={`comment-author-${comment.id}`}>{authorName}</span>
          <span className="text-xs text-[#9ca3af]" data-testid={`comment-time-${comment.id}`}>
            {format(new Date(comment.createdAt), "dd.MM.yyyy HH:mm")}
          </span>
        </div>
        <p className="mt-1 whitespace-pre-wrap text-sm text-[#d1d5db]" data-testid={`comment-content-${comment.id}`}>
          {comment.content}
        </p>
      </div>
    </div>
  );
}
