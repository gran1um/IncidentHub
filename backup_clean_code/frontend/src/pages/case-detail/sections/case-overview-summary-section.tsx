import { Link } from "wouter";
import { Eye, Link2, ListChecks, MessageSquare } from "lucide-react";

import { withTenantPath } from "@/lib/tenant-url";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

type Props = {
  t: (...args: any[]) => string;
  panelClass: string;
  taskCompletion: number;
  observableCount: number;
  commentCount: number;
  relatedLinkBy: string;
  setRelatedLinkBy: (value: any) => void;
  parseRelatedCasesLinkBy: (value: string | null | undefined) => any;
  relatedCasesLinkByOptions: readonly string[];
  relatedActiveRecentCases: any[];
  relatedAllTimeCases: any[];
  currentTenantSlug: string;
  renderRelatedMatchedSummary: (related: any, scope: string) => any;
};

export function CaseOverviewSummarySection({
  t,
  panelClass,
  taskCompletion,
  observableCount,
  commentCount,
  relatedLinkBy,
  setRelatedLinkBy,
  parseRelatedCasesLinkBy,
  relatedCasesLinkByOptions,
  relatedActiveRecentCases,
  relatedAllTimeCases,
  currentTenantSlug,
  renderRelatedMatchedSummary,
}: Props) {
  return (
    <>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <Card className={panelClass} data-testid="stat-tasks">
          <CardContent className="pt-6">
            <div className="flex items-center gap-3">
              <ListChecks size={20} className="text-primary" />
              <div>
                <div className="text-2xl font-bold">{taskCompletion}%</div>
                <div className="text-xs text-[#9ca3af]">{t("caseDetail.overview.taskCompletion")}</div>
              </div>
            </div>
            <Progress value={taskCompletion} className="mt-3 h-2" />
          </CardContent>
        </Card>
        <Card className={panelClass} data-testid="stat-observables">
          <CardContent className="pt-6">
            <div className="flex items-center gap-3">
              <Eye size={20} className="text-purple-500" />
              <div>
                <div className="text-2xl font-bold">{observableCount}</div>
                <div className="text-xs text-[#9ca3af]">{t("caseDetail.tab.observables")}</div>
              </div>
            </div>
          </CardContent>
        </Card>
        <Card className={panelClass} data-testid="stat-comments">
          <CardContent className="pt-6">
            <div className="flex items-center gap-3">
              <MessageSquare size={20} className="text-cyan-500" />
              <div>
                <div className="text-2xl font-bold">{commentCount}</div>
                <div className="text-xs text-[#9ca3af]">{t("caseDetail.tab.comments")}</div>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      <Card className={panelClass} data-testid="card-related-cases">
        <CardHeader className="pb-3">
          <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <CardTitle className="flex items-center gap-2 text-sm font-semibold uppercase tracking-[0.12em] text-[#9ca3af]">
              <Link2 size={16} className="text-primary" /> {t("caseDetail.related.title")}
            </CardTitle>
            <div className="flex flex-nowrap items-center gap-2 overflow-hidden">
              <span className="shrink-0 whitespace-nowrap text-xs text-[#9ca3af]">{t("caseDetail.related.linkBy")}</span>
              <Select value={relatedLinkBy} onValueChange={(value) => setRelatedLinkBy(parseRelatedCasesLinkBy(value))}>
                <SelectTrigger className="h-8 min-w-[170px] rounded-lg" data-testid="select-related-link-by">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {relatedCasesLinkByOptions.map((option) => (
                    <SelectItem key={option} value={option}>
                      {t(`caseDetail.related.linkBy.${option}`)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-5">
          <div className="space-y-2" data-testid="related-active-recent">
            <div className="text-xs font-semibold uppercase tracking-wide text-[#9ca3af]">{t("caseDetail.related.activeRecent")}</div>
            {relatedActiveRecentCases.length === 0 ? (
              <div className="rounded-xl border border-dashed px-3 py-2 text-sm text-[#9ca3af]">
                {t("caseDetail.related.empty", { field: t(`caseDetail.related.linkBy.${relatedLinkBy}`) })}
              </div>
            ) : (
              <div className="space-y-2">
                {relatedActiveRecentCases.map((related: any) => (
                  <Link
                    key={`related-active-${related.id}`}
                    href={withTenantPath(currentTenantSlug, `/cases/${related.id}`)}
                    className="block rounded-xl border border-[#2a2c3c] px-3 py-2 transition-colors hover:border-[#2a2c3c] hover:bg-[#1a1f2d]"
                    data-testid={`related-case-active-${related.id}`}
                  >
                    <div className="flex items-center justify-between gap-3">
                      <div className="min-w-0">
                        <div className="truncate text-sm font-semibold">{related.caseNumber || related.id}</div>
                        <div className="truncate text-xs text-[#9ca3af]">{related.title || related.id}</div>
                      </div>
                      <Badge variant="outline" className="shrink-0 text-[11px]">
                        {t("caseDetail.related.matches", { count: related.matchCount || 0 })}
                      </Badge>
                    </div>
                    {renderRelatedMatchedSummary(related, "active")}
                  </Link>
                ))}
              </div>
            )}
          </div>

          <div className="space-y-2" data-testid="related-all-time">
            <div className="text-xs font-semibold uppercase tracking-wide text-[#9ca3af]">{t("caseDetail.related.allTime")}</div>
            {relatedAllTimeCases.length === 0 ? (
              <div className="rounded-xl border border-dashed px-3 py-2 text-sm text-[#9ca3af]">
                {t("caseDetail.related.empty", { field: t(`caseDetail.related.linkBy.${relatedLinkBy}`) })}
              </div>
            ) : (
              <div className="space-y-2">
                {relatedAllTimeCases.map((related: any) => (
                  <Link
                    key={`related-all-${related.id}`}
                    href={withTenantPath(currentTenantSlug, `/cases/${related.id}`)}
                    className="block rounded-xl border border-[#2a2c3c] px-3 py-2 transition-colors hover:border-[#2a2c3c] hover:bg-[#1a1f2d]"
                    data-testid={`related-case-all-${related.id}`}
                  >
                    <div className="flex items-center justify-between gap-3">
                      <div className="min-w-0">
                        <div className="truncate text-sm font-semibold">{related.caseNumber || related.id}</div>
                        <div className="truncate text-xs text-[#9ca3af]">{related.title || related.id}</div>
                      </div>
                      <Badge variant="outline" className="shrink-0 text-[11px]">
                        {t("caseDetail.related.matches", { count: related.matchCount || 0 })}
                      </Badge>
                    </div>
                    {renderRelatedMatchedSummary(related, "all")}
                  </Link>
                ))}
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </>
  );
}
