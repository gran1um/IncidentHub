import { format } from "date-fns";
import { FileText, Plus } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { TabsContent } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";

type CasePagesTabProps = {
  t: (key: string, params?: Record<string, string>) => string;
  pageCount: number;
  pageTitle: string;
  pageBody: string;
  setPageTitle: (value: string) => void;
  setPageBody: (value: string) => void;
  onCreatePage: () => void;
  createPagePending: boolean;
  pages: any[];
  usersByID: Map<string, any>;
  panelClass: string;
  subpanelClass: string;
  inputClass: string;
};

export function CasePagesTab({
  t,
  pageCount,
  pageTitle,
  pageBody,
  setPageTitle,
  setPageBody,
  onCreatePage,
  createPagePending,
  pages,
  usersByID,
  panelClass,
  subpanelClass,
  inputClass,
}: CasePagesTabProps) {
  return (
    <TabsContent value="pages" className="mt-6 space-y-6" data-testid="tab-content-pages">
      <Card className={panelClass}>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm font-semibold uppercase tracking-[0.12em] text-[#9ca3af]">
            <FileText size={16} className="text-primary" /> {t("caseDetail.page.title", { count: String(pageCount) })}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-[#9ca3af]">{t("caseDetail.page.subtitle")}</p>

          <Card className={`${subpanelClass} space-y-3 p-4`}>
            <div className="grid gap-1.5">
              <Label htmlFor="input-case-page-title">{t("caseDetail.page.formTitle")}</Label>
              <Input
                id="input-case-page-title"
                value={pageTitle}
                onChange={(event) => setPageTitle(event.target.value)}
                placeholder={t("caseDetail.page.formTitlePlaceholder")}
                className={inputClass}
                data-testid="input-case-page-title"
              />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="input-case-page-body">{t("caseDetail.page.formBody")}</Label>
              <Textarea
                id="input-case-page-body"
                value={pageBody}
                onChange={(event) => setPageBody(event.target.value)}
                placeholder={t("caseDetail.page.formBodyPlaceholder")}
                className="min-h-[140px] rounded-lg border-[#2a2c3c] bg-[#0f1118] text-[#f3f4f6] placeholder:text-[#6b7280] focus-visible:ring-[#3b82f6]/40"
                data-testid="input-case-page-body"
              />
            </div>
            <div className="flex justify-end">
              <Button
                onClick={onCreatePage}
                className="rounded-xl gap-2 bg-[#11141d] font-semibold text-[#f3f4f6] hover:bg-[#1d2433]"
                disabled={!pageTitle.trim() || !pageBody.trim() || createPagePending}
                data-testid="button-create-case-page"
              >
                <Plus size={14} />
                {createPagePending ? t("common.loading") : t("caseDetail.page.create")}
              </Button>
            </div>
          </Card>

          {pages.length === 0 ? (
            <div className="py-8 text-center text-[#9ca3af]" data-testid="case-pages-empty">
              <FileText size={32} className="mx-auto mb-3 opacity-50" />
              <p className="text-sm">{t("caseDetail.page.empty")}</p>
            </div>
          ) : (
            <div className="space-y-3">
              {pages.map((page: any) => {
                const createdByName = usersByID.get(String(page.createdBy || ""))?.name || page.createdBy || "—";
                const createdAtLabel =
                  page.createdAt && !Number.isNaN(new Date(page.createdAt).getTime())
                    ? format(new Date(page.createdAt), "dd.MM.yyyy HH:mm")
                    : "—";
                return (
                  <Card key={page.id} className={`${subpanelClass} space-y-2 p-4`} data-testid={`case-page-card-${page.id}`}>
                    <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
                      <h3 className="text-sm font-semibold">{page.title || "—"}</h3>
                      <span className="text-xs text-[#9ca3af]">{createdAtLabel}</span>
                    </div>
                    <div className="text-xs text-[#9ca3af]">{t("caseDetail.page.createdBy", { name: String(createdByName) })}</div>
                    <div className="break-words whitespace-pre-wrap text-sm leading-relaxed">{page.body || "—"}</div>
                  </Card>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>
    </TabsContent>
  );
}
