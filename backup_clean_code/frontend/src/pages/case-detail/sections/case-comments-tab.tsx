import { MessageSquare } from "lucide-react";

import { CommentItem } from "../components/comment-item";
import { UserAvatar } from "@/components/user-avatar";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { TabsContent } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";

type CaseCommentsTabProps = {
  t: (key: string, params?: Record<string, string>) => string;
  commentCount: number;
  comments: any[];
  currentUser: any;
  newComment: string;
  setNewComment: (value: string) => void;
  onAddComment: () => void;
  panelClass: string;
};

export function CaseCommentsTab({
  t,
  commentCount,
  comments,
  currentUser,
  newComment,
  setNewComment,
  onAddComment,
  panelClass,
}: CaseCommentsTabProps) {
  return (
    <TabsContent value="comments" className="mt-6 space-y-6" data-testid="tab-content-comments">
      <Card className={panelClass}>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm font-semibold uppercase tracking-[0.12em] text-[#9ca3af]">
            <MessageSquare size={16} className="text-primary" /> {t("caseDetail.comment.title", { count: String(commentCount) })}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {!comments || comments.length === 0 ? (
            <div className="py-8 text-center text-[#9ca3af]" data-testid="comments-empty">
              <MessageSquare size={32} className="mx-auto mb-3 opacity-50" />
              <p className="text-sm">{t("caseDetail.comment.empty")}</p>
            </div>
          ) : (
            <div className="space-y-4">
              {comments.map((comment: any) => (
                <CommentItem key={comment.id} comment={comment} />
              ))}
            </div>
          )}
          <Separator />
          <div className="flex items-start gap-3">
            <UserAvatar
              name={currentUser?.name}
              avatar={currentUser?.avatar}
              className="h-9 w-9 rounded-xl border border-[#2a2c3c]"
              fallbackClassName="text-sm font-bold"
            />
            <div className="flex-1">
              <Textarea
                placeholder={t("caseDetail.comment.placeholder")}
                value={newComment}
                onChange={(event) => setNewComment(event.target.value)}
                className="rounded-xl border-[#2a2c3c] bg-[#0f1118] text-[#f3f4f6] placeholder:text-[#6b7280] focus-visible:ring-[#3b82f6]/40"
                rows={3}
                data-testid="input-comment"
              />
              <div className="mt-2 flex justify-end">
                <Button
                  onClick={onAddComment}
                  className="rounded-xl gap-2 bg-[#11141d] font-semibold text-[#f3f4f6] hover:bg-[#1d2433]"
                  disabled={!newComment.trim()}
                  data-testid="button-add-comment"
                >
                  <MessageSquare size={14} /> {t("caseDetail.comment.post")}
                </Button>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </TabsContent>
  );
}
