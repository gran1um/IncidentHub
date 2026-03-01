import { AppLayout } from "@/components/layout";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import { useAppState, useCases, useCreateForumThread, useForumThreads } from "@/lib/api";
import { formatDistanceToNow } from "date-fns";
import { Clock, Filter, FolderKanban, MessageSquare, Plus, RotateCcw, Search } from "lucide-react";
import { Link, useLocation } from "wouter";
import { useMemo, useState } from "react";
import { Skeleton } from "@/components/ui/skeleton";
import { useT } from "@/lib/i18n";
import { EllipsisText } from "@/components/ui/ellipsis-text";
import { withTenantPath } from "@/lib/tenant-url";
import { useMinimumLoading } from "@/lib/use-minimum-loading";
import { toast } from "sonner";

const PAGE_SHELL_CLASS = "mx-auto w-full max-w-[1144px] space-y-6 pb-6";
const PANEL_CLASS =
  "rounded-2xl border border-[rgba(255,255,255,0.06)] bg-[linear-gradient(180deg,rgba(19,20,28,0.97),rgba(17,20,32,0.97))] shadow-[0_14px_34px_rgba(0,0,0,0.28)]";
const SUBPANEL_CLASS =
  "rounded-xl border border-[#2a2c3c] bg-[#111624] p-5 transition-colors hover:border-[#4b5168] hover:bg-[#171b2a]";
const MUTED_TEXT_CLASS = "text-xs text-[#8b91a3]";
const SELECT_TRIGGER_CLASS = "h-11 rounded-xl border border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] focus:ring-1 focus:ring-[#3b4a79]";
const SELECT_CONTENT_CLASS = "rounded-xl border border-[#2a2c3c] bg-[#13141c] text-white";
const CASE_BADGE_CLASS = "border-[#2a2c3c] bg-[#0b0c10] text-[#9ca3af] font-mono text-xs";
const THREAD_STATUS_CLASS = "uppercase tracking-wider font-semibold text-[10px]";

function isThreadMatchesSearch(thread: any, linkedCase: any, search: string): boolean {
  const normalizedSearch = String(search || "").trim().toLowerCase();
  if (!normalizedSearch) {
    return true;
  }
  const chunks = [
    String(thread?.title || ""),
    String(thread?.status || ""),
    String(linkedCase?.id || ""),
    String(linkedCase?.title || ""),
    String(linkedCase?.description || ""),
    ...(Array.isArray(linkedCase?.tags) ? linkedCase.tags.map((item: any) => String(item || "")) : []),
  ]
    .join(" ")
    .toLowerCase();
  return chunks.includes(normalizedSearch);
}

export default function ForumPage() {
  const t = useT();
  const [, setLocation] = useLocation();
  const { currentTenantId, currentTenantSlug, currentUserId } = useAppState();
  const { data: forumThreads = [], isLoading: threadsLoading } = useForumThreads(currentTenantId);
  const { data: cases = [], isLoading: casesLoading } = useCases(currentTenantId);
  const createForumThread = useCreateForumThread();

  const [searchQuery, setSearchQuery] = useState("");
  const [severityFilter, setSeverityFilter] = useState<string>("all");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [newThreadCaseId, setNewThreadCaseId] = useState("");
  const [newThreadTitle, setNewThreadTitle] = useState("");
  const [newThreadStatus, setNewThreadStatus] = useState("In Progress");
  const [newThreadMessage, setNewThreadMessage] = useState("");
  const showForumPageSkeleton = useMinimumLoading(threadsLoading || casesLoading);

  const casesByID = useMemo(() => {
    const byID = new Map<string, any>();
    cases.forEach((item: any) => {
      if (!item?.id) return;
      byID.set(String(item.id), item);
    });
    return byID;
  }, [cases]);

  const activeFiltersCount = useMemo(() => {
    let count = 0;
    if (searchQuery.trim()) count += 1;
    if (severityFilter !== "all") count += 1;
    if (statusFilter !== "all") count += 1;
    return count;
  }, [searchQuery, severityFilter, statusFilter]);

  const metrics = useMemo(() => {
    let inProgress = 0;
    let questions = 0;
    let closed = 0;
    forumThreads.forEach((thread: any) => {
      const status = String(thread?.status || "").trim().toLowerCase();
      if (status === "in progress") {
        inProgress += 1;
      } else if (status === "questions") {
        questions += 1;
      } else if (status === "closed") {
        closed += 1;
      }
    });
    return {
      total: forumThreads.length,
      inProgress,
      questions,
      closed,
    };
  }, [forumThreads]);

  const filteredThreads = useMemo(
    () =>
      forumThreads.filter((thread: any) => {
        const linkedCase = casesByID.get(String(thread.caseId || ""));

        if (severityFilter !== "all" && (!linkedCase || linkedCase.sev !== severityFilter)) {
          return false;
        }

        if (statusFilter !== "all" && thread.status !== statusFilter) {
          return false;
        }

        return isThreadMatchesSearch(thread, linkedCase, searchQuery);
      }),
    [forumThreads, casesByID, severityFilter, statusFilter, searchQuery],
  );

  const resetFilters = () => {
    setSearchQuery("");
    setSeverityFilter("all");
    setStatusFilter("all");
  };

  const openCreateDialog = (open: boolean) => {
    setCreateDialogOpen(open);
    if (!open) {
      return;
    }
    if (!newThreadCaseId && cases.length > 0) {
      setNewThreadCaseId(String(cases[0].id));
    }
  };

  const handleCreateThread = () => {
    const caseId = String(newThreadCaseId || "").trim();
    const title = newThreadTitle.trim();
    const status = String(newThreadStatus || "In Progress").trim() || "In Progress";
    const initialMessage = newThreadMessage.trim();

    if (!caseId) {
      toast.error("Select a case to continue.");
      return;
    }
    if (title.length < 3) {
      toast.error("Thread title must be at least 3 characters.");
      return;
    }

    createForumThread.mutate(
      {
        caseId,
        title,
        status,
        initialMessage,
        authorId: currentUserId,
        tenantId: currentTenantId,
      },
      {
        onSuccess: (thread: any) => {
          setCreateDialogOpen(false);
          setNewThreadCaseId("");
          setNewThreadTitle("");
          setNewThreadStatus("In Progress");
          setNewThreadMessage("");
          toast.success("Forum thread created");
          if (thread?.id) {
            setLocation(withTenantPath(currentTenantSlug, `/forum/${thread.id}`));
          }
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to create forum thread");
        },
      },
    );
  };

  if (showForumPageSkeleton) {
    return (
      <AppLayout>
        <div className={PAGE_SHELL_CLASS}>
          <Skeleton className="mb-2 h-9 w-48" />
          <Skeleton className="h-5 w-80" />
          <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={`forum-kpi-${i}`} className="h-24 w-full rounded-xl" />
            ))}
          </div>
          <Skeleton className="h-40 w-full rounded-xl" />
          <div className="space-y-4">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={`forum-thread-${i}`} className="h-36 w-full rounded-xl" />
            ))}
          </div>
        </div>
      </AppLayout>
    );
  }

  return (
    <AppLayout>
      <div className={PAGE_SHELL_CLASS}>
        <div className="flex flex-col justify-between gap-4 md:flex-row md:items-end">
          <div>
            <h1 className="text-[32px] font-semibold leading-8 tracking-[-0.5px] text-white">{t("forum.titleSecurity")}</h1>
            <p className={`mt-1 ${MUTED_TEXT_CLASS}`}>{t("forum.subtitleSecurity")}</p>
          </div>

          <Dialog open={createDialogOpen} onOpenChange={openCreateDialog}>
            <DialogTrigger asChild>
              <Button
                className="h-11 gap-2 rounded-xl border border-[#4adf37] bg-[#4adf37] px-5 font-semibold text-[#0b0c10] shadow-[0_6px_18px_rgba(74,223,55,0.35)] hover:bg-[#61f44f]"
                data-testid="button-forum-new-thread"
              >
                <Plus size={16} />
                New Thread
              </Button>
            </DialogTrigger>
            <DialogContent className={`${PANEL_CLASS} max-w-2xl overflow-hidden rounded-2xl border-[#2a2c3c] p-0 text-white`}>
              <DialogHeader className="border-b border-[#2a2c3c] px-6 py-5 text-left">
                <DialogTitle className="text-xl font-semibold">Create Forum Thread</DialogTitle>
                <DialogDescription className={MUTED_TEXT_CLASS}>
                  Start a structured discussion linked to a case.
                </DialogDescription>
              </DialogHeader>

              <div className="space-y-4 px-6 py-5">
                <div className="space-y-2">
                  <label className="text-xs font-semibold uppercase tracking-[0.08em] text-[#8b91a3]">Case</label>
                  <Select value={newThreadCaseId || "none"} onValueChange={(value) => setNewThreadCaseId(value === "none" ? "" : value)}>
                    <SelectTrigger className={SELECT_TRIGGER_CLASS} data-testid="select-forum-thread-case">
                      <SelectValue placeholder="Select case" />
                    </SelectTrigger>
                    <SelectContent className={SELECT_CONTENT_CLASS}>
                      <SelectItem value="none">Select case</SelectItem>
                      {cases.map((item: any) => (
                        <SelectItem key={`forum-case-${item.id}`} value={String(item.id)}>
                          {item.id} · {item.title}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                  <div className="space-y-2">
                    <label className="text-xs font-semibold uppercase tracking-[0.08em] text-[#8b91a3]">Thread Title</label>
                    <Input
                      value={newThreadTitle}
                      onChange={(event) => setNewThreadTitle(event.target.value)}
                      placeholder="Short discussion title"
                      className="h-11 rounded-xl border border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] placeholder:text-[#6b7280] focus-visible:ring-[#3b4a79]"
                      data-testid="input-forum-thread-title"
                    />
                  </div>

                  <div className="space-y-2">
                    <label className="text-xs font-semibold uppercase tracking-[0.08em] text-[#8b91a3]">Status</label>
                    <Select value={newThreadStatus} onValueChange={setNewThreadStatus}>
                      <SelectTrigger className={SELECT_TRIGGER_CLASS}>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent className={SELECT_CONTENT_CLASS}>
                        <SelectItem value="In Progress">In Progress</SelectItem>
                        <SelectItem value="Questions">Questions</SelectItem>
                        <SelectItem value="Closed">Closed</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                </div>

                <div className="space-y-2">
                  <label className="text-xs font-semibold uppercase tracking-[0.08em] text-[#8b91a3]">Initial Message (optional)</label>
                  <Textarea
                    value={newThreadMessage}
                    onChange={(event) => setNewThreadMessage(event.target.value)}
                    rows={4}
                    placeholder="Provide context for participants"
                    className="rounded-xl border border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] placeholder:text-[#6b7280] focus-visible:ring-[#3b4a79]"
                    data-testid="input-forum-thread-message"
                  />
                </div>
              </div>

              <DialogFooter className="border-t border-[#2a2c3c] px-6 py-4 sm:justify-end">
                <Button variant="outline" className={`h-11 rounded-xl ${SELECT_TRIGGER_CLASS}`} onClick={() => setCreateDialogOpen(false)}>
                  Cancel
                </Button>
                <Button
                  className="h-11 rounded-xl border border-[#4adf37] bg-[#4adf37] px-5 font-semibold text-[#0b0c10] hover:bg-[#61f44f]"
                  onClick={handleCreateThread}
                  disabled={createForumThread.isPending}
                  data-testid="button-create-forum-thread"
                >
                  {createForumThread.isPending ? "Creating..." : "Create Thread"}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>

        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          <Card className={`${PANEL_CLASS} rounded-xl px-4 py-3`}>
            <div className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Active Threads</div>
            <div className="mt-1 text-[22px] font-semibold leading-6 text-[#f3f4f6]">{metrics.total}</div>
          </Card>
          <Card className={`${PANEL_CLASS} rounded-xl px-4 py-3`}>
            <div className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">In Progress</div>
            <div className="mt-1 text-[22px] font-semibold leading-6 text-[#3b82f6]">{metrics.inProgress}</div>
          </Card>
          <Card className={`${PANEL_CLASS} rounded-xl px-4 py-3`}>
            <div className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Questions</div>
            <div className="mt-1 text-[22px] font-semibold leading-6 text-[#a855f7]">{metrics.questions}</div>
          </Card>
          <Card className={`${PANEL_CLASS} rounded-xl px-4 py-3`}>
            <div className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Closed</div>
            <div className="mt-1 text-[22px] font-semibold leading-6 text-[#66ff4c]">{metrics.closed}</div>
          </Card>
        </div>

        <Card className={`${PANEL_CLASS} p-5`}>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
            <div className="space-y-2 md:col-span-1">
              <label className="text-sm font-medium text-[#9ca3af]">Search</label>
              <div className="relative group">
                <Search
                  className="absolute left-3 top-1/2 -translate-y-1/2 text-[#6b7280] transition-colors group-focus-within:text-[#9ca3af]"
                  size={16}
                />
                <Input
                  value={searchQuery}
                  onChange={(event) => setSearchQuery(event.target.value)}
                  placeholder="Search thread title or case"
                  className="h-11 rounded-xl border border-[#2a2c3c] bg-[#0b0c10] pl-10 text-[#d1d5db] placeholder:text-[#6b7280] focus-visible:ring-[#3b4a79]"
                />
              </div>
            </div>

            <div className="space-y-2 md:col-span-1">
              <label className="text-sm font-medium text-[#9ca3af]">Severity</label>
              <Select value={severityFilter} onValueChange={setSeverityFilter}>
                <SelectTrigger className={SELECT_TRIGGER_CLASS}>
                  <SelectValue placeholder={t("forum.filter.severityPlaceholder")} />
                </SelectTrigger>
                <SelectContent className={SELECT_CONTENT_CLASS}>
                  <SelectItem value="all">{t("forum.allSeverities")}</SelectItem>
                  <SelectItem value="Critical">{t("severity.critical")}</SelectItem>
                  <SelectItem value="High">{t("severity.high")}</SelectItem>
                  <SelectItem value="Medium">{t("severity.medium")}</SelectItem>
                  <SelectItem value="Low">{t("severity.low")}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2 md:col-span-1">
              <label className="text-sm font-medium text-[#9ca3af]">Status</label>
              <Select value={statusFilter} onValueChange={setStatusFilter}>
                <SelectTrigger className={SELECT_TRIGGER_CLASS}>
                  <SelectValue placeholder={t("forum.filter.statusPlaceholder")} />
                </SelectTrigger>
                <SelectContent className={SELECT_CONTENT_CLASS}>
                  <SelectItem value="all">{t("forum.allStatuses")}</SelectItem>
                  <SelectItem value="In Progress">{t("forum.inProgress")}</SelectItem>
                  <SelectItem value="Questions">{t("forum.questions")}</SelectItem>
                  <SelectItem value="Closed">{t("forum.closed")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="mt-5 flex flex-wrap items-center justify-between gap-3 border-t border-[#2a2c3c] pt-4">
            <div className="flex items-center gap-2 text-sm text-[#8b91a3]">
              <Filter size={14} className="text-[#8b91a3]" />
              <span>
                Active Filters: <span className="font-semibold text-white">{activeFiltersCount}</span>
              </span>
            </div>
            <Button
              type="button"
              variant="ghost"
              className="h-auto gap-2 px-0 text-sm font-medium text-[#66ff4c] hover:bg-transparent hover:text-[#4ed938]"
              onClick={resetFilters}
            >
              <RotateCcw size={14} />
              Reset Filters
            </Button>
          </div>
        </Card>

        <div className="grid gap-4">
          {filteredThreads.map((thread: any) => {
            const linkedCase = casesByID.get(String(thread.caseId || ""));
            const threadPostsCount = Number(thread?.postsCount ?? thread?.posts_count ?? 0) || 0;
            const lastPostTimestamp = String(thread?.lastPostAt || thread?.last_post_at || thread?.lastActivityAt || thread?.last_activity_at || "").trim();

            return (
              <Link key={thread.id} href={withTenantPath(currentTenantSlug, `/forum/${thread.id}`)}>
                <Card className={`${SUBPANEL_CLASS} group cursor-pointer`}>
                  <div className="flex flex-col gap-6 md:flex-row">
                    <div className="flex min-w-[220px] flex-col gap-2 border-r border-dashed border-[#2a2c3c] pr-6 md:w-1/4">
                      <div className="flex items-center justify-between gap-2">
                        <Badge variant="outline" className={`${CASE_BADGE_CLASS} max-w-[150px]`}>
                          <EllipsisText text={linkedCase?.id || "—"} className="max-w-[130px]" />
                        </Badge>
                        <Badge
                          className={`h-5 text-[10px] ${linkedCase?.sev === "Critical" ? "border-red-500/30 bg-red-500/20 text-red-200" : linkedCase?.sev === "High" ? "border-orange-500/30 bg-orange-500/20 text-orange-200" : "border-blue-500/30 bg-blue-500/20 text-blue-200"}`}
                        >
                          {linkedCase?.sev || "—"}
                        </Badge>
                      </div>
                      {linkedCase?.id ? (
                        <span
                          onClick={(event) => {
                            event.preventDefault();
                            event.stopPropagation();
                            setLocation(withTenantPath(currentTenantSlug, `/cases/${linkedCase.id}`));
                          }}
                          className="flex min-w-0 cursor-pointer items-center gap-1 text-xs font-semibold text-[#66ff4c] hover:text-[#4ed938]"
                        >
                          <FolderKanban size={12} />
                          <EllipsisText text={t("forum.viewCase")} className="max-w-[130px]" />
                        </span>
                      ) : null}
                      <EllipsisText text={linkedCase?.title || "—"} className={`max-w-[240px] ${MUTED_TEXT_CLASS}`} />
                    </div>

                    <div className="flex-1">
                      <div className="mb-2 flex items-center justify-between">
                        <div className="flex min-w-0 items-center gap-2">
                          <div className="rounded-lg border border-[#2a2c3c] bg-[#0f131d] p-2 text-[#60a5fa]">
                            <MessageSquare size={18} />
                          </div>
                          <EllipsisText
                            text={thread.title}
                            className="max-w-[460px] text-lg font-semibold text-white transition-colors group-hover:text-[#66ff4c]"
                          />
                        </div>
                        <Badge variant={thread.status === "Closed" ? "secondary" : "default"} className={THREAD_STATUS_CLASS}>
                          {thread.status}
                        </Badge>
                      </div>

                      <div className="mt-4 flex items-center gap-4 pl-11 text-sm text-[#9ca3af]">
                        <span className="flex items-center gap-1">
                          <MessageSquare size={14} /> {t("forum.posts").replace("{count}", String(threadPostsCount))}
                        </span>
                        {lastPostTimestamp && (
                          <span className="flex items-center gap-1">
                            <Clock size={14} />
                            {t("forum.lastActivity").replace(
                              "{time}",
                              formatDistanceToNow(new Date(lastPostTimestamp)),
                            )}
                          </span>
                        )}
                        <span className="ml-auto text-xs font-medium text-[#66ff4c]">{t("forum.openThread")}</span>
                      </div>
                    </div>
                  </div>
                </Card>
              </Link>
            );
          })}

          {filteredThreads.length === 0 && (
            <div className={`${PANEL_CLASS} rounded-3xl border-2 border-dashed border-[#2a2c3c] py-20 text-center`}>
              <div className="font-semibold italic text-[#9ca3af]">{t("forum.emptyFiltered")}</div>
            </div>
          )}
        </div>
      </div>
    </AppLayout>
  );
}
