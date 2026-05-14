import { Link } from "wouter";
import { FileText, MessageSquare } from "lucide-react";

import { withTenantPath } from "@/lib/tenant-url";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Textarea } from "@/components/ui/textarea";

type Props = any;

export function CaseOverviewCollaborationSection(props: Props) {
  const {
    t,
    panelClass,
    forumThread,
    resolvedForumId,
    currentTenantSlug,
    handleCreateForum,
    inlineEditingField,
    descriptionViewMode,
    setDescriptionViewMode,
    renderInlineEditButton,
    inlineEditorRef,
    inlineEditingValue,
    setInlineEditingValue,
    renderInlineEditorActions,
    caseData,
    renderedDescriptionHTML,
  } = props;

  return (
    <div className="space-y-6">
      <Card className={panelClass} data-testid="card-forum">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm font-semibold uppercase tracking-[0.12em] text-[#9ca3af]">
            <MessageSquare size={16} className="text-primary" /> {t("caseDetail.forum.title")}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {forumThread ? (
            <Link href={withTenantPath(currentTenantSlug, `/forum/${forumThread.id}`)}>
              <Card className="cursor-pointer rounded-xl border border-[#2a2c3c] bg-[#0f1118] p-4 transition-colors hover:bg-[#1a1f2d]">
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-[rgba(59,130,246,0.16)] text-[#93c5fd]">
                    <MessageSquare size={18} />
                  </div>
                  <div className="flex-1">
                    <div className="text-sm font-bold">{forumThread.title}</div>
                    <div className="mt-0.5 text-xs text-[#9ca3af]">
                      {t("caseDetail.forum.posts", { count: String(forumThread.posts?.length || 0) })} · {t("cases.field.status")}: {forumThread.status}
                    </div>
                  </div>
                </div>
              </Card>
            </Link>
          ) : resolvedForumId ? (
            <Link href={withTenantPath(currentTenantSlug, `/forum/${resolvedForumId}`)}>
              <Button variant="outline" className="h-9 rounded-xl gap-2 border-[#2a2c3c] bg-[#0f1118] text-[#d1d5db] hover:bg-[#1a1f2d]" data-testid="link-forum-thread">
                <MessageSquare size={16} /> {t("caseDetail.forum.view")}
              </Button>
            </Link>
          ) : (
            <div className="flex flex-col items-center gap-3 py-4">
              <p className="text-sm text-[#9ca3af]">{t("caseDetail.forum.empty")}</p>
              <Button onClick={handleCreateForum} className="h-9 rounded-xl gap-2 font-semibold" data-testid="button-start-discussion">
                <MessageSquare size={16} /> {t("caseDetail.forum.start")}
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      <Card className={`group ${panelClass}`} data-testid="card-description">
        <CardHeader>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <CardTitle className="flex items-center gap-2 text-sm font-semibold uppercase tracking-[0.12em] text-[#9ca3af]">
              <FileText size={16} className="text-primary" /> {t("cases.field.description")}
            </CardTitle>
            {inlineEditingField !== "description" ? (
              <div className="flex items-center gap-2">
                <div className="inline-flex rounded-lg border border-[#2a2c3c] bg-[#0f1118] p-0.5">
                  <Button
                    type="button"
                    variant={descriptionViewMode === "rendered" ? "secondary" : "ghost"}
                    size="sm"
                    className="h-7 rounded-md px-2 text-xs text-[#d1d5db] hover:bg-[#1a1f2d]"
                    onClick={() => setDescriptionViewMode("rendered")}
                    data-testid="button-description-view-rendered"
                  >
                    {t("caseDetail.description.viewRendered")}
                  </Button>
                  <Button
                    type="button"
                    variant={descriptionViewMode === "raw" ? "secondary" : "ghost"}
                    size="sm"
                    className="h-7 rounded-md px-2 text-xs text-[#d1d5db] hover:bg-[#1a1f2d]"
                    onClick={() => setDescriptionViewMode("raw")}
                    data-testid="button-description-view-raw"
                  >
                    {t("caseDetail.description.viewRaw")}
                  </Button>
                </div>
                {renderInlineEditButton("description", "button-inline-edit-description", "md")}
              </div>
            ) : null}
          </div>
        </CardHeader>
        <CardContent>
          {inlineEditingField === "description" ? (
            <div ref={inlineEditorRef} data-testid="inline-editor-description">
              <Textarea
                autoFocus
                value={inlineEditingValue}
                onChange={(event) => setInlineEditingValue(event.target.value)}
                className="min-h-[220px] resize-y rounded-xl"
                data-testid="input-inline-description"
              />
              {renderInlineEditorActions("description")}
            </div>
          ) : (
            <div className="rounded-xl border border-transparent p-2 text-left" data-testid="text-description">
              {descriptionViewMode === "rendered" ? (
                caseData.description ? (
                  <div
                    className="whitespace-pre-wrap break-words text-sm leading-relaxed text-[#d1d5db] [&_a]:text-[#93c5fd] [&_a]:underline [&_code]:rounded [&_code]:bg-[#171b28] [&_code]:px-1 [&_pre]:overflow-x-auto [&_pre]:rounded [&_pre]:bg-[#171b28] [&_pre]:p-2"
                    data-testid="text-description-rendered"
                    dangerouslySetInnerHTML={{ __html: renderedDescriptionHTML }}
                  />
                ) : (
                  <p className="whitespace-pre-wrap break-words text-sm leading-relaxed text-[#d1d5db]">
                    {t("caseDetail.description.empty")}
                  </p>
                )
              ) : (
                <pre
                  className="overflow-x-auto whitespace-pre-wrap break-words rounded-lg border border-[#2a2c3c] bg-[#171b28] p-3 text-xs leading-relaxed text-[#d1d5db]"
                  data-testid="text-description-raw"
                >
                  {caseData.description || t("caseDetail.description.empty")}
                </pre>
              )}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
