import { useEffect, useMemo, useRef, useState, type ChangeEvent } from "react";
import {
  useAppState, useCase, useUser, useUsers, useTenants, useUpdateCase, useCopyCase, useEscalateCase, useCaseShares, useShareCaseAcrossTenants,
  useCaseRelatedCases,
  useCreateForumThread, useForumThread,
  useCaseTimeline, useCaseTasks, useCreateCaseTask, useUpdateCaseTask, useDeleteCaseTask,
  useCaseObservables, useCreateCaseObservable, useUpdateCaseObservable, useDeleteCaseObservable,
  useCaseAttachments, useUploadCaseAttachment, useCaseAttachmentDownloadURL,
  useCaseComments, useCreateCaseComment,
  useCreateCaseTimelineEvent, useCasePlaybookRuns,
  useWorkflowCatalog, useWorkflowRun,
  useCaseAIAnalyses, useAnalyzeCaseAI, useCaseStatuses, useCaseCategories, useCreateCaseCategory,
  useOutboundConnectors, useConnectorMethods, useTenantConnectorMethods, useExecuteConnectorHub, useConnectorHubExecutions, useAIAgentEntityTrace,
  useRestartAIAgentWorkload, useCloseAIAgentWorkload,
} from "@/lib/api";
import { AppLayout } from "@/components/layout";
import { useParams, Link, useLocation } from "wouter";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Progress } from "@/components/ui/progress";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { Separator } from "@/components/ui/separator";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { CaseCommunicationsTab } from "@/components/case-communications-tab";
import { ConnectorExecutionDrawer } from "@/components/connector-execution-drawer";
import { EllipsisText } from "@/components/ui/ellipsis-text";
import {
  ArrowLeft, Clock, MessageSquare, Shield, AlertTriangle,
  Edit, Copy, ChevronDown, Plus, Trash2, CheckCircle2, Circle, Timer, XCircle,
  Eye, Activity, FileText, Bell,
  ListChecks, BarChart3, Sparkles, Layers,
  Paperclip, Download, FileDown, Link2, CalendarClock, GitBranch, ArrowUpRight,
} from "lucide-react";
import { format } from "date-fns";
import { ResponsiveContainer, BarChart, Bar, CartesianGrid, XAxis, YAxis, Tooltip as ChartTooltip, Legend } from "recharts";
import { toast } from "sonner";
import { useI18n, useT } from "@/lib/i18n";
import { applySearchPatch, splitLocationPathAndSearch } from "@/lib/url-state";
import { UserAvatar } from "@/components/user-avatar";
import { withTenantPath } from "@/lib/tenant-url";
import { useMinimumLoading } from "@/lib/use-minimum-loading";
import { useNow } from "@/hooks/use-now";
import { isSOARCaseFieldKey, mergeMissingSOARCaseFields, normalizeSOARCaseFieldKey } from "@/lib/soar-case-fields";
import { buildObservableConnectorOptionsByType } from "@/lib/connectors";
import { ObservableConnectorMenu } from "@/features/connectors";
import { CaseDetailLoadingSkeleton } from "./components/case-detail-loading-skeleton";
import { CaseCommentsTab } from "./sections/case-comments-tab";
import { CaseOverviewCollaborationSection } from "./sections/case-overview-collaboration-section";
import { CaseOverviewSummarySection } from "./sections/case-overview-summary-section";
import { CaseConnectorsTab } from "./sections/case-connectors-tab";
import { CaseMitreTab } from "./sections/case-mitre-tab";
import { CaseAIWorkloadsTab } from "./sections/case-ai-workloads-tab";
import { CaseObservablesTab } from "./sections/case-observables-tab";
import {
  VERDICTS,
  TLP_VALUES,
  PAP_VALUES,
  TASK_STATUSES,
  TASK_STATUS_KEYS,
  TASK_CHAIN_MODES,
  TASK_CHAIN_MODE_KEYS,
  CASE_SLA_MINUTES_BY_SEVERITY,
  QUICK_REMINDER_PRESETS,
  DEFAULT_CASE_CATEGORIES,
  CLOSURE_APPROVAL_EVENT_TYPE,
  RELATED_CASES_LINK_BY_OPTIONS,
  VERDICT_KEYS,
  TIMELINE_EVENT_KEYS,
  INLINE_AUTOSAVE_FIELDS,
  getAIVerdictBadgeClass,
  getAIStatusBadgeClass,
  formatAnalysisDate,
  formatTimelineDate,
  formatDateTimeWithSeconds,
  sanitizeDescriptionHTML,
  humanizeTimelineEventType,
  summarizeWorkflowRunPayload,
  isImageAttachment,
  observableTypeKey,
  defaultPlaybookContext,
  toDateTimeLocal,
  fromDateTimeLocal,
  normalizeTrafficLight,
  normalizeObservableType,
  classifyObservableSource,
  parseObservableTagsInput,
  normalizeObservableTags,
  parseRelatedCasesLinkBy,
  parseClosureApprovalConfig,
  createCaseEditDraft,
  toCustomFieldDraftRows,
  buildCaseExportCSV,
  buildCaseExportMarkdown,
  tokensFromText,
  uniqueStrings,
  renderMarkdownToHTML,
  createCustomFieldDraftID,
  toTimestamp,
  formatDurationCompact,
  type RelatedCasesLinkBy,
  type CaseEditDraft,
  type CustomFieldDraftRow,
  type VisualizationSourceFilter,
  type VisualizationTypeFilter,
  type VisualizationAttachmentFilter,
  type InlineCaseFieldKey,
} from "./helpers";

const sevColors: Record<string, string> = {
  Critical: "border border-[rgba(239,68,68,0.28)] bg-[rgba(239,68,68,0.18)] text-[#fca5a5]",
  High: "border border-[rgba(245,158,11,0.28)] bg-[rgba(245,158,11,0.16)] text-[#fbbf24]",
  Medium: "border border-[rgba(59,130,246,0.28)] bg-[rgba(59,130,246,0.16)] text-[#93c5fd]",
  Low: "border border-[rgba(34,197,94,0.28)] bg-[rgba(34,197,94,0.16)] text-[#86efac]",
};

const statusColors: Record<string, string> = {
  Open: "border border-[rgba(59,130,246,0.28)] bg-[rgba(59,130,246,0.16)] text-[#93c5fd]",
  "In Progress": "border border-[rgba(245,158,11,0.3)] bg-[rgba(245,158,11,0.16)] text-[#fbbf24]",
  Resolved: "border border-[rgba(34,197,94,0.28)] bg-[rgba(34,197,94,0.16)] text-[#86efac]",
  Closed: "border border-[rgba(156,163,175,0.26)] bg-[rgba(107,114,128,0.16)] text-[#d1d5db]",
};

const tlpColors: Record<string, string> = {
  "TLP:RED": "border border-[rgba(239,68,68,0.3)] bg-[rgba(239,68,68,0.18)] text-[#fca5a5]",
  "TLP:AMBER": "border border-[rgba(245,158,11,0.3)] bg-[rgba(245,158,11,0.16)] text-[#fbbf24]",
  "TLP:GREEN": "border border-[rgba(34,197,94,0.3)] bg-[rgba(34,197,94,0.16)] text-[#86efac]",
  "TLP:CLEAR": "border border-[rgba(156,163,175,0.3)] bg-[rgba(107,114,128,0.16)] text-[#d1d5db]",
};


const taskStatusIcons: Record<string, React.ReactNode> = {
  Pending: <Circle size={14} className="text-gray-400" />,
  "In Progress": <Timer size={14} className="text-orange-500" />,
  Done: <CheckCircle2 size={14} className="text-green-500" />,
  Cancelled: <XCircle size={14} className="text-red-400" />,
};

const timelineIcons: Record<string, React.ReactNode> = {
  alert_import: <Bell size={14} className="text-blue-500" />,
  observable_added: <Eye size={14} className="text-purple-500" />,
  task_created: <ListChecks size={14} className="text-green-500" />,
  status_changed: <Activity size={14} className="text-orange-500" />,
  comment_added: <MessageSquare size={14} className="text-cyan-500" />,
  note: <FileText size={14} className="text-sky-500" />,
  reminder: <CalendarClock size={14} className="text-fuchsia-500" />,
  closure_approval: <CheckCircle2 size={14} className="text-emerald-600" />,
  case_escalated: <AlertTriangle size={14} className="text-amber-500" />,
  case_escalation_received: <Shield size={14} className="text-emerald-500" />,
  workflow_run: <Sparkles size={14} className="text-violet-500" />,
  "Alert imported": <Bell size={14} className="text-blue-500" />,
  "Observable added": <Eye size={14} className="text-purple-500" />,
  "Task created": <ListChecks size={14} className="text-green-500" />,
  "Status changed": <Activity size={14} className="text-orange-500" />,
  "Comment added": <MessageSquare size={14} className="text-cyan-500" />,
  Reminder: <CalendarClock size={14} className="text-fuchsia-500" />,
  "Closure approved": <CheckCircle2 size={14} className="text-emerald-600" />,
  default: <Clock size={14} className="text-gray-400" />,
};

const timelineColors: Record<string, string> = {
  alert_import: "bg-[rgba(59,130,246,0.15)] border-[rgba(59,130,246,0.35)]",
  observable_added: "bg-[rgba(168,85,247,0.15)] border-[rgba(168,85,247,0.35)]",
  task_created: "bg-[rgba(34,197,94,0.16)] border-[rgba(34,197,94,0.34)]",
  status_changed: "bg-[rgba(245,158,11,0.16)] border-[rgba(245,158,11,0.34)]",
  comment_added: "bg-[rgba(6,182,212,0.15)] border-[rgba(6,182,212,0.34)]",
  note: "bg-[rgba(56,189,248,0.15)] border-[rgba(56,189,248,0.32)]",
  reminder: "bg-[rgba(217,70,239,0.14)] border-[rgba(217,70,239,0.32)]",
  closure_approval: "bg-[rgba(16,185,129,0.16)] border-[rgba(16,185,129,0.34)]",
  case_escalated: "bg-[rgba(245,158,11,0.16)] border-[rgba(245,158,11,0.34)]",
  case_escalation_received: "bg-[rgba(16,185,129,0.16)] border-[rgba(16,185,129,0.34)]",
  workflow_run: "bg-[rgba(139,92,246,0.16)] border-[rgba(139,92,246,0.34)]",
  "Alert imported": "bg-[rgba(59,130,246,0.15)] border-[rgba(59,130,246,0.35)]",
  "Observable added": "bg-[rgba(168,85,247,0.15)] border-[rgba(168,85,247,0.35)]",
  "Task created": "bg-[rgba(34,197,94,0.16)] border-[rgba(34,197,94,0.34)]",
  "Status changed": "bg-[rgba(245,158,11,0.16)] border-[rgba(245,158,11,0.34)]",
  "Comment added": "bg-[rgba(6,182,212,0.15)] border-[rgba(6,182,212,0.34)]",
  Reminder: "bg-[rgba(217,70,239,0.14)] border-[rgba(217,70,239,0.32)]",
  "Closure approved": "bg-[rgba(16,185,129,0.16)] border-[rgba(16,185,129,0.34)]",
  default: "bg-[rgba(107,114,128,0.16)] border-[rgba(107,114,128,0.32)]",
};

const CASE_TABS = [
  "overview",
  "timeline",
  "tasks",
  "observables",
  "visuals",
  "attachments",
  "comments",
  "ai",
  "mitre",
  "connectors",
  "communications",
] as const;

const CASE_TAB_TRIGGER_CLASS =
  "relative inline-flex h-10 shrink-0 items-center justify-center whitespace-nowrap rounded-none border-b border-r border-[#1d1e29] px-2.5 md:px-3 text-[11px] md:text-xs font-semibold text-[#9ca3af] transition-colors hover:bg-[#171b2a] hover:text-[#e5e7eb] last:border-r-0 data-[state=active]:bg-[#13141c] data-[state=active]:text-[#ecfdf5] data-[state=active]:after:absolute data-[state=active]:after:bottom-0 data-[state=active]:after:left-2 data-[state=active]:after:right-2 data-[state=active]:after:h-[2px] data-[state=active]:after:rounded-full data-[state=active]:after:bg-[#66ff4c]";

const CASE_PAGE_SHELL_CLASS = "mx-auto w-full max-w-[1240px] space-y-6";
const CASE_PANEL_CLASS = "rounded-2xl border border-[#2a2c3c] bg-[#13141c] shadow-[0_4px_20px_rgba(0,0,0,0.32)]";
const CASE_PANEL_PADDED_CLASS = `${CASE_PANEL_CLASS} p-6`;
const CASE_SUBPANEL_CLASS = "rounded-xl border border-[#2a2c3c] bg-[#10131d]";
const CASE_INLINE_EDIT_BUTTON_CLASS = "h-8 w-8 rounded-lg border border-[#2a2c3c] bg-[#0f1118] text-[#9ca3af] hover:bg-[#1a1f2d] hover:text-[#f3f4f6]";
const CASE_INPUT_CLASS = "h-9 rounded-lg border-[#2a2c3c] bg-[#0f1118] text-[#f3f4f6] placeholder:text-[#6b7280] focus-visible:ring-[#3b82f6]/40";
const CASE_SELECT_TRIGGER_CLASS = "h-9 rounded-lg border-[#2a2c3c] bg-[#0f1118] text-[#f3f4f6] data-[placeholder]:text-[#6b7280]";
const CASE_SELECT_CONTENT_CLASS = "rounded-lg border-[#2a2c3c] bg-[#13141c] text-[#f3f4f6]";

const CASE_TAB_SET = new Set<string>(CASE_TABS);
const CASE_AI_TRACE_REFRESH_MS = 2000;

export default function CaseDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [location, setLocation] = useLocation();
  const [activeTab, setActiveTab] = useState("overview");
  const [aiFocusWorkloadId, setAiFocusWorkloadId] = useState("");
  const [aiFocusStageId, setAiFocusStageId] = useState("");
  const [aiTraceAutoRefresh, setAiTraceAutoRefresh] = useState(true);
  const nowTs = useNow(60_000);
  const { currentUserId, currentTenantId, currentTenantSlug, session } = useAppState();
  const { data: caseData, isLoading: caseLoading } = useCase(id || "", { refetchInterval: 5000 });
  const { data: tenants = [] } = useTenants();
  const [forumId, setForumId] = useState("");
  const { data: currentUser } = useUser(currentUserId);
  const { data: ownerUser } = useUser(caseData?.owner || "");
  const { data: assigneeUser } = useUser(caseData?.assignee || "");
  const resolvedForumId = forumId || caseData?.forumId || "";
  const { data: forumThread } = useForumThread(resolvedForumId);
  const { data: users } = useUsers(currentTenantId);
  const { data: outboundConnectors = [] } = useOutboundConnectors(currentTenantId);
  const { data: caseCategories = [] } = useCaseCategories(currentTenantId);
  const createCaseCategory = useCreateCaseCategory();
  const usersByID = useMemo(() => {
    const byID = new Map<string, any>();
    (users || []).forEach((user: any) => {
      if (!user?.id) return;
      byID.set(String(user.id), user);
    });
    return byID;
  }, [users]);
  const { data: caseStatuses = [] } = useCaseStatuses(currentTenantId);
  const updateCase = useUpdateCase();
  const copyCase = useCopyCase();
  const escalateCase = useEscalateCase();
  const shareCaseAcrossTenants = useShareCaseAcrossTenants();
  const { data: caseShares = [] } = useCaseShares(id || "");
  const [relatedLinkBy, setRelatedLinkBy] = useState<RelatedCasesLinkBy>("observables");
  const { data: relatedCasesData } = useCaseRelatedCases(id || "", relatedLinkBy);
  const createForumThread = useCreateForumThread();

  const { data: timeline } = useCaseTimeline(id || "");
  const { data: tasks } = useCaseTasks(id || "");
  const createTask = useCreateCaseTask();
  const updateTask = useUpdateCaseTask();
  const deleteTask = useDeleteCaseTask();
  const { data: observables } = useCaseObservables(id || "");
  const createObservable = useCreateCaseObservable();
  const updateObservable = useUpdateCaseObservable();
  const deleteObservable = useDeleteCaseObservable();
  const { data: attachments = [] } = useCaseAttachments(id || "");
  const uploadAttachment = useUploadCaseAttachment();
  const downloadAttachment = useCaseAttachmentDownloadURL();
  const { data: comments = [] } = useCaseComments(id || "");
  const createComment = useCreateCaseComment();
  const createTimelineEvent = useCreateCaseTimelineEvent();
  const { data: casePlaybookRunsData } = useCasePlaybookRuns(id || "", currentTenantId, 30);
  const { data: workflowCatalog = [] } = useWorkflowCatalog(currentTenantId);
  const runWorkflow = useWorkflowRun();

  const { data: caseAIAnalyses = [] } = useCaseAIAnalyses(id || "", 30);
  const analyzeCaseAI = useAnalyzeCaseAI();
  const {
    data: caseAITrace,
    isLoading: caseAITraceLoading,
    isFetching: caseAITraceFetching,
    refetch: refetchCaseAITrace,
  } = useAIAgentEntityTrace("case", id || "", currentTenantId, 8, {
    refetchInterval: activeTab === "ai" && aiTraceAutoRefresh ? CASE_AI_TRACE_REFRESH_MS : false,
  });
  const restartAIAgentWorkload = useRestartAIAgentWorkload();
  const closeAIAgentWorkload = useCloseAIAgentWorkload();
  const executeConnectorHub = useExecuteConnectorHub();
  const t = useT();
  const { language } = useI18n();
  const [pendingAITraceAction, setPendingAITraceAction] = useState<{ workloadId: string; action: "restart" | "close" } | null>(null);
  const currentTenantMembership = useMemo(
    () => (Array.isArray(session?.memberships) ? session.memberships : []).find((membership: any) => membership?.tenant_id === currentTenantId && membership?.is_active),
    [currentTenantId, session?.memberships],
  );
  const canManageAIWorkloads = Boolean(session?.identity?.is_platform_admin || currentTenantMembership?.role === "tenant_admin");

  useEffect(() => {
    if (activeTab !== "ai") {
      setAiTraceAutoRefresh(true);
      return;
    }
    if (!caseAITrace) {
      setAiTraceAutoRefresh(true);
      return;
    }
    const events = Array.isArray((caseAITrace as any)?.events) ? (caseAITrace as any).events : [];
    if (events.length === 0) {
      setAiTraceAutoRefresh(true);
      return;
    }
    const runningStatuses = new Set(["queued", "running", "processing", "in_progress", "in-progress", "active"]);
    const shouldAutoRefresh = events.some((event: any) => {
      const eventStatus = String(event?.status || "").trim().toLowerCase();
      if (runningStatuses.has(eventStatus)) {
        return true;
      }
      const workloads = Array.isArray(event?.workloads) ? event.workloads : [];
      return workloads.some((workload: any) => runningStatuses.has(String(workload?.status || "").trim().toLowerCase()));
    });
    setAiTraceAutoRefresh((prev) => (prev === shouldAutoRefresh ? prev : shouldAutoRefresh));
  }, [activeTab, caseAITrace]);

  const caseStatusOptions = useMemo(() => {
    if (caseStatuses.length > 0) return caseStatuses;
    return [
      { code: "open", label: t("caseDetail.status.open"), isClosed: false },
      { code: "resolved", label: t("caseDetail.status.resolved"), isClosed: true },
      { code: "closed", label: t("caseDetail.status.closed"), isClosed: true },
    ];
  }, [caseStatuses, t]);

  const caseCategoryOptions = useMemo(() => {
    const merged = new Set<string>(DEFAULT_CASE_CATEGORIES);
    (caseCategories || []).forEach((item: any) => {
      const name = String(item?.name || "").trim();
      if (name) merged.add(name);
    });
    if (caseData?.category) {
      merged.add(String(caseData.category));
    }
    return Array.from(merged.values());
  }, [caseCategories, caseData?.category]);

  const handleAITraceWorkloadAction = (workloadId: string, action: "restart" | "close") => {
    const normalizedWorkloadId = String(workloadId || "").trim();
    if (!normalizedWorkloadId) return;
    setPendingAITraceAction({ workloadId: normalizedWorkloadId, action });
    const mutation = action === "restart" ? restartAIAgentWorkload : closeAIAgentWorkload;
    mutation.mutate(
      { workloadId: normalizedWorkloadId },
      {
        onSuccess: () => {
          setPendingAITraceAction(null);
          toast.success(action === "restart" ? "AI workload restarted" : "AI workload closed");
        },
        onError: (error: any) => {
          setPendingAITraceAction(null);
          toast.error(error?.message || (action === "restart" ? "Failed to restart AI workload" : "Failed to close AI workload"));
        },
      },
    );
  };

  const handleAssignCurrentAnalystFromAITrace = () => {
    if (!id || !currentUserId || !caseData) return;
    updateCase.mutate(
      {
        id,
        data: {
          assignee: currentUserId,
          expectedUpdatedAt: String(caseData.updatedAt || "").trim(),
        },
      },
      {
        onSuccess: () => {
          toast.success("Case assigned to current analyst");
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to assign case to current analyst");
        },
      },
    );
  };

  const handleEscalateFromAITrace = (context: { workload: any; runResult: any }) => {
    const summaryParts = [
      String(context.runResult?.summary || "").trim(),
      String(context.runResult?.verdict || "").trim() ? `Verdict: ${String(context.runResult?.verdict || "").trim()}` : "",
      String(context.workload?.last_error || context.runResult?.error || "").trim(),
    ].filter(Boolean);
    setEscalationDraft((prev) => ({
      ...prev,
      summary: summaryParts.join("\n") || prev.summary,
    }));
    setEscalateDialogOpen(true);
  };

  const escalationTargetTenants = useMemo(
    () => (Array.isArray(tenants) ? tenants : []).filter((tenant: any) => tenant?.id && tenant.id !== currentTenantId && tenant.active !== false),
    [tenants, currentTenantId],
  );

  const currentCaseStatus = useMemo(() => {
    const currentCode = (caseData?.statusCode || "").toString().trim().toLowerCase();
    return (
      caseStatusOptions.find((item: any) => item.code === currentCode) || {
        code: currentCode || "open",
        label: caseData?.status || t("caseDetail.status.open"),
        isClosed: false,
      }
    );
  }, [caseData?.status, caseData?.statusCode, caseStatusOptions, t]);

  const availablePlaybooks = useMemo(
    () =>
      (Array.isArray(workflowCatalog) ? workflowCatalog : [])
        .filter((item: any) => Boolean(item?.enabled))
        .map((item: any) => ({
          id: String(item?.id || "").trim(),
          name: String(item?.name || "Workflow").trim(),
          description: String(item?.description || "").trim(),
          updatedAt: String(item?.updatedAt || item?.createdAt || "").trim(),
        }))
        .filter((item: any) => item.id),
    [workflowCatalog],
  );

  const playbookAutofillInput = useMemo(
    () => defaultPlaybookContext(caseData, Array.isArray(observables) ? observables : []),
    [caseData, observables],
  );

  const suggestedPlaybookMeta = useMemo(() => {
    const severity = String(caseData?.sev || "").toLowerCase();
    const incidentTokens = tokensFromText(caseData?.incidentType || "");
    const stageTokens = tokensFromText(caseData?.stage || "");
    const observableTypeTokens = uniqueStrings(
      (Array.isArray(observables) ? observables : []).map((item: any) => observableTypeKey(item?.type)),
    );

    return availablePlaybooks
      .map((playbook) => {
        const searchable = `${playbook.name} ${playbook.description}`.toLowerCase();
        let score = 0;
        const reasons: string[] = [];

        incidentTokens.forEach((token) => {
          if (searchable.includes(token)) {
            score += 4;
            reasons.push(`${t("caseDetail.timeline.suggest.reasonIncident")}: ${token}`);
          }
        });
        stageTokens.forEach((token) => {
          if (searchable.includes(token)) {
            score += 2;
            reasons.push(`${t("caseDetail.timeline.suggest.reasonStage")}: ${token}`);
          }
        });
        observableTypeTokens.forEach((token) => {
          if (token && searchable.includes(token)) {
            score += 3;
            reasons.push(`${t("caseDetail.timeline.suggest.reasonObservable")}: ${token}`);
          }
        });

        const highPressure = severity === "critical" || severity === "high";
        if (highPressure) {
          ["contain", "isolat", "block", "response", "triage"].forEach((token) => {
            if (searchable.includes(token)) {
              score += 2;
              reasons.push(t("caseDetail.timeline.suggest.reasonSeverity"));
            }
          });
        }

        return {
          ...playbook,
          score,
          reasons: uniqueStrings(reasons).slice(0, 2),
        };
      })
      .sort((left, right) => {
        if (left.score !== right.score) {
          return right.score - left.score;
        }
        return left.name.localeCompare(right.name);
      });
  }, [availablePlaybooks, caseData?.incidentType, caseData?.sev, caseData?.stage, observables, t]);

  const suggestedPlaybooks = useMemo(
    () => suggestedPlaybookMeta.filter((item) => item.score > 0).slice(0, 4),
    [suggestedPlaybookMeta],
  );

  const [newComment, setNewComment] = useState("");
  const [timelineNoteTitle, setTimelineNoteTitle] = useState("");
  const [timelineNoteBody, setTimelineNoteBody] = useState("");
  const [selectedPlaybookID, setSelectedPlaybookID] = useState("");
  const [playbookInputDraft, setPlaybookInputDraft] = useState("{}");
  const [playbookInputDirty, setPlaybookInputDirty] = useState(false);
  const [taskTitle, setTaskTitle] = useState("");
  const [taskDesc, setTaskDesc] = useState("");
  const [taskAssignee, setTaskAssignee] = useState("");
  const [taskDueAt, setTaskDueAt] = useState("");
  const [taskMandatory, setTaskMandatory] = useState(false);
  const [obsType, setObsType] = useState("IP");
  const [obsValue, setObsValue] = useState("");
  const [obsVerdict, setObsVerdict] = useState("Unknown");
  const [obsTags, setObsTags] = useState("");
  const [observableTagDrafts, setObservableTagDrafts] = useState<Record<string, string>>({});
  const [taskFilter, setTaskFilter] = useState("All");
  const [mitreDialogOpen, setMitreDialogOpen] = useState(false);
  const [editDialogOpen, setEditDialogOpen] = useState(false);
  const [escalateDialogOpen, setEscalateDialogOpen] = useState(false);
  const [duplicateCaseConfirmOpen, setDuplicateCaseConfirmOpen] = useState(false);
  const [escalationDraft, setEscalationDraft] = useState({
    targetTenantID: "",
    handoffType: "employee_client",
    summary: "",
    includeObservables: true,
  });
  const [shareDialogOpen, setShareDialogOpen] = useState(false);
  const [shareDraft, setShareDraft] = useState({
    targetTenantID: "",
  });
  const [caseEditDraft, setCaseEditDraft] = useState<CaseEditDraft | null>(null);
  const [caseEditBaseUpdatedAt, setCaseEditBaseUpdatedAt] = useState("");
  const [inlineEditingField, setInlineEditingField] = useState<InlineCaseFieldKey | null>(null);
  const [inlineEditingValue, setInlineEditingValue] = useState("");
  const [inlineEditBaseUpdatedAt, setInlineEditBaseUpdatedAt] = useState("");
  const [inlineSavingField, setInlineSavingField] = useState<InlineCaseFieldKey | null>(null);
  const [descriptionViewMode, setDescriptionViewMode] = useState<"rendered" | "raw">("rendered");
  const [reminderMessage, setReminderMessage] = useState("");
  const [visualSourceFilter, setVisualSourceFilter] = useState<VisualizationSourceFilter>("all");
  const [visualTypeFilter, setVisualTypeFilter] = useState<VisualizationTypeFilter>("all");
  const [visualAttachmentFilter, setVisualAttachmentFilter] = useState<VisualizationAttachmentFilter>("all");
  const [visualSearch, setVisualSearch] = useState("");
  const [customFieldRows, setCustomFieldRows] = useState<CustomFieldDraftRow[]>([]);
  const [customFieldsDirty, setCustomFieldsDirty] = useState(false);
  const [newCaseCategoryName, setNewCaseCategoryName] = useState("");
  const [connectorHubConnectorID, setConnectorHubConnectorID] = useState("");
  const [connectorHubDraft, setConnectorHubDraft] = useState({
    methodId: "",
    action: "",
    message: "",
    inputJson: "{\n  \"query\": \"\"\n}",
    metadataJson: "{\n  \"source\": \"case_detail\"\n}",
    dryRun: false,
  });
  const [selectedConnectorExecutionID, setSelectedConnectorExecutionID] = useState("");
  const [connectorExecutionDrawerOpen, setConnectorExecutionDrawerOpen] = useState(false);
  const attachmentInputRef = useRef<HTMLInputElement | null>(null);
  const inlineEditorRef = useRef<HTMLDivElement | null>(null);
  const queryHydratedRef = useRef(false);
  const { data: connectorHubMethods = [] } = useConnectorMethods(connectorHubConnectorID);
  const { data: connectorHubRuns = [] } = useConnectorHubExecutions(undefined, {
    caseId: id || "",
    limit: 20,
  });

  const observableRows = useMemo(() => (Array.isArray(observables) ? observables : []), [observables]);

  const { data: tenantConnectorMethods = [] } = useTenantConnectorMethods(currentTenantId);

  const observableConnectorOptionsByType = useMemo(
    () => buildObservableConnectorOptionsByType(outboundConnectors || [], tenantConnectorMethods || []),
    [outboundConnectors, tenantConnectorMethods],
  );

  const handleRunConnectorForObservable = (
    observable: any,
    option: { connectorId: string; methodId: string; action: string },
    context?: {
      source?: string;
      extraInput?: Record<string, any>;
      extraMetadata?: Record<string, any>;
    },
  ) => {
    if (!id) return;
    const value = String(observable?.value || "").trim();
    if (!value) {
      toast.error("Observable value is empty");
      return;
    }
    const typeKey = observableTypeKey(observable?.type || "");
    const observableId = String(observable?.id || "").trim();

    const inputPayload: Record<string, any> = {
      case_id: id,
      observable_type: typeKey,
      indicator: value,
      observable: {
        ...(observableId ? { id: observableId } : {}),
        type: observable.type,
        value,
        verdict: observable.verdict,
        tags: Array.isArray(observable.tags) ? observable.tags : [],
      },
      ...(context?.extraInput || {}),
    };
    if (observableId) {
      inputPayload.observable_id = observableId;
    }
    const metadataPayload: Record<string, any> = {
      source: context?.source || "case_observable",
      case_id: id,
      observable_type: typeKey,
      connector_method_id: option.methodId,
      ...(context?.extraMetadata || {}),
    };
    if (observableId) {
      metadataPayload.observable_id = observableId;
    }

    executeConnectorHub.mutate(
      {
        connector_id: option.connectorId,
        method_id: option.methodId,
        action: option.action || undefined,
        case_id: id,
        message: undefined,
        input: inputPayload,
        metadata: metadataPayload,
        dry_run: false,
      },
      {
        onSuccess: (result: any) => {
          const runStatus = String(result?.status || "").toLowerCase();
          const executionID = String(result?.id || result?.execution_id || "").trim();
          if (executionID) {
            setSelectedConnectorExecutionID(executionID);
            setConnectorExecutionDrawerOpen(true);
          }
          if (runStatus === "completed" || runStatus === "dry_run") {
            toast.success("Connector execution completed");
            return;
          }
          if (
            runStatus === "accepted" ||
            runStatus === "queued" ||
            runStatus === "dispatching" ||
            runStatus === "retry_scheduled" ||
            runStatus === "provider_accepted"
          ) {
            toast.success("Connector execution queued");
            return;
          }
          toast.error(result?.error || "Connector execution failed");
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to execute connector");
        },
      },
    );
  };

  const renderRelatedMatchedSummary = (related: any, sectionKey: string) => {
    if (Array.isArray(related?.matchedObservables) && related.matchedObservables.length > 0) {
      return (
        <div className="mt-2 flex flex-wrap gap-1.5">
          {related.matchedObservables.slice(0, 2).map((item: any, index: number) => {
            const observableType = String(item?.type || "Observable").trim() || "Observable";
            const value = String(item?.value || "").trim();
            const typeKey = observableTypeKey(observableType);
            const connectorOptions = observableConnectorOptionsByType[typeKey] || [];
            return (
              <div
                key={`related-observable-${sectionKey}-${related?.id || "case"}-${index}`}
                className="inline-flex max-w-full items-center gap-1.5 rounded-lg border border-[#2a2c3c] bg-[#0f1118] px-2 py-1"
              >
                <span className="text-[10px] font-semibold uppercase tracking-wide text-[#60a5fa]">{observableType}</span>
                <span className="max-w-[180px] truncate text-xs text-[#d1d5db]">{value || "-"}</span>
                <ObservableConnectorMenu
                  options={connectorOptions}
                  onSelect={(option) =>
                    handleRunConnectorForObservable(
                      {
                        type: observableType,
                        value,
                        verdict: item?.verdict || "Unknown",
                        tags: Array.isArray(item?.tags) ? item.tags : [],
                      },
                      option,
                      {
                        source: "case_related_observable",
                        extraInput: {
                          matched_case_id: related?.id,
                        },
                        extraMetadata: {
                          matched_case_id: related?.id,
                        },
                      },
                    )
                  }
                  emptyLabel={t("alerts.observableConnectors.noMethods")}
                  disabled={executeConnectorHub.isPending || !value}
                  triggerClassName="h-6 w-6 text-[#60a5fa] hover:bg-[rgba(37,99,235,0.16)] hover:text-[#93c5fd]"
                  triggerTestId={`button-related-observable-connectors-${sectionKey}-${related?.id || "case"}-${index}`}
                  getItemTestId={(option) =>
                    `button-related-observable-connector-option-${sectionKey}-${related?.id || "case"}-${index}-${option.methodId}`
                  }
                />
              </div>
            );
          })}
        </div>
      );
    }
    if (Array.isArray(related?.matchedFields) && related.matchedFields.length > 0) {
      return (
        <div className="mt-1 truncate text-xs text-[#9ca3af]">
          {related.matchedFields
            .slice(0, 2)
            .map((item: any) => `${t(`caseDetail.related.linkBy.${item.field}`)}: ${item.value}`)
            .join(", ")}
        </div>
      );
    }
    return null;
  };

  const relatedActiveRecentCases = useMemo(
    () => (Array.isArray(relatedCasesData?.activeRecent) ? relatedCasesData.activeRecent : []),
    [relatedCasesData?.activeRecent],
  );
  const relatedAllTimeCases = useMemo(
    () => (Array.isArray(relatedCasesData?.allTime) ? relatedCasesData.allTime : []),
    [relatedCasesData?.allTime],
  );
  const closureApprovalConfig = useMemo(
    () => parseClosureApprovalConfig(caseData?.customFields),
    [caseData?.customFields],
  );

  useEffect(() => {
    if (availablePlaybooks.length === 0) {
      setSelectedPlaybookID("");
      setPlaybookInputDraft("{}");
      setPlaybookInputDirty(false);
      return;
    }
    setSelectedPlaybookID((prev) => {
      if (prev && availablePlaybooks.some((item) => item.id === prev)) {
        return prev;
      }
      return availablePlaybooks[0].id;
    });
  }, [availablePlaybooks]);

  useEffect(() => {
    if (!selectedPlaybookID || playbookInputDirty) {
      return;
    }
    const selected = availablePlaybooks.find((item) => item.id === selectedPlaybookID);
    if (!selected) return;
    const payload = {
      ...playbookAutofillInput,
      workflow: {
        id: selected.id,
        name: selected.name,
      },
    };
    setPlaybookInputDraft(JSON.stringify(payload, null, 2));
  }, [availablePlaybooks, playbookAutofillInput, playbookInputDirty, selectedPlaybookID]);

  const renderedDescriptionHTML = useMemo(
    () => sanitizeDescriptionHTML(renderMarkdownToHTML(String(caseData?.description || ""))),
    [caseData?.description],
  );

  useEffect(() => {
    setForumId(caseData?.forumId || "");
  }, [caseData?.forumId]);

  useEffect(() => {
    setCustomFieldRows(toCustomFieldDraftRows(caseData?.customFields));
    setCustomFieldsDirty(false);
  }, [caseData?.id, caseData?.customFields]);

  useEffect(() => {
    const connectorRows = Array.isArray(outboundConnectors) ? outboundConnectors : [];
    if (connectorRows.length === 0) {
      setConnectorHubConnectorID("");
      return;
    }
    setConnectorHubConnectorID((prev) => {
      if (prev && connectorRows.some((item: any) => String(item?.id || "") === prev)) {
        return prev;
      }
      return String(connectorRows[0]?.id || "");
    });
  }, [outboundConnectors]);

  useEffect(() => {
    if (!connectorHubConnectorID) {
      setConnectorHubDraft((prev) => ({ ...prev, methodId: "", action: "" }));
      return;
    }
    if (!Array.isArray(connectorHubMethods) || connectorHubMethods.length === 0) {
      setConnectorHubDraft((prev) => ({ ...prev, methodId: "" }));
      return;
    }
    setConnectorHubDraft((prev) => {
      const methodExists = connectorHubMethods.some((item: any) => String(item?.id || "") === prev.methodId);
      if (methodExists) {
        return prev;
      }
      const firstMethod = connectorHubMethods[0];
      const nextAction = String(firstMethod?.action || firstMethod?.slug || firstMethod?.name || "").trim();
      return {
        ...prev,
        methodId: String(firstMethod?.id || ""),
        action: nextAction || prev.action,
      };
    });
  }, [connectorHubConnectorID, connectorHubMethods]);

  useEffect(() => {
    if (!escalationTargetTenants.length) {
      return;
    }
    setEscalationDraft((prev) => {
      if (prev.targetTenantID) {
        return prev;
      }
      return { ...prev, targetTenantID: String(escalationTargetTenants[0].id) };
    });
    setShareDraft((prev) => {
      if (prev.targetTenantID) {
        return prev;
      }
      return { ...prev, targetTenantID: String(escalationTargetTenants[0].id) };
    });
  }, [escalationTargetTenants]);

  useEffect(() => {
    const observableIdSet = new Set(observableRows.map((item: any) => item.id));
    setObservableTagDrafts((prev) => {
      let changed = false;
      const next: Record<string, string> = {};
      for (const [key, value] of Object.entries(prev)) {
        if (!observableIdSet.has(key)) {
          changed = true;
          continue;
        }
        next[key] = value;
      }
      return changed ? next : prev;
    });
  }, [observableRows]);

  useEffect(() => {
    const { params } = splitLocationPathAndSearch(location);
    const tabParam = (params.get("tab") || "").trim().toLowerCase();
    if (tabParam && CASE_TAB_SET.has(tabParam)) {
      setActiveTab(tabParam);
    }
    const taskFilterParam = params.get("task_filter");
    if (taskFilterParam) {
      setTaskFilter(taskFilterParam);
    }
    const descriptionModeParam = params.get("description_mode");
    if (descriptionModeParam === "raw" || descriptionModeParam === "rendered") {
      setDescriptionViewMode(descriptionModeParam);
    }
    const visualSourceParam = (params.get("visual_source") || "").trim().toLowerCase();
    if (visualSourceParam === "all" || visualSourceParam === "network" || visualSourceParam === "authorization" || visualSourceParam === "other") {
      setVisualSourceFilter(visualSourceParam);
    }
    const visualTypeParam = (params.get("visual_type") || "").trim();
    if (visualTypeParam) {
      setVisualTypeFilter(visualTypeParam);
    }
    const visualFileParam = (params.get("visual_file") || "").trim().toLowerCase();
    if (visualFileParam === "all" || visualFileParam === "images" || visualFileParam === "files") {
      setVisualAttachmentFilter(visualFileParam);
    }
    const visualSearchParam = params.get("visual_q");
    if (visualSearchParam !== null) {
      setVisualSearch(visualSearchParam);
    }
    setRelatedLinkBy(parseRelatedCasesLinkBy(params.get("related_link_by")));
    setAiFocusWorkloadId(String(params.get("ai_workload") || "").trim());
    setAiFocusStageId(String(params.get("ai_stage") || "").trim());
    queryHydratedRef.current = true;
  }, []);

  useEffect(() => {
    if (!queryHydratedRef.current) return;
    const nextLocation = applySearchPatch(location, {
      tab: activeTab !== "overview" ? activeTab : undefined,
      task_filter: activeTab === "tasks" && taskFilter !== "All" ? taskFilter : undefined,
      description_mode: descriptionViewMode !== "rendered" ? descriptionViewMode : undefined,
      visual_source: activeTab === "visuals" && visualSourceFilter !== "all" ? visualSourceFilter : undefined,
      visual_type: activeTab === "visuals" && visualTypeFilter !== "all" ? visualTypeFilter : undefined,
      visual_file: activeTab === "visuals" && visualAttachmentFilter !== "all" ? visualAttachmentFilter : undefined,
      visual_q: activeTab === "visuals" && visualSearch.trim() ? visualSearch.trim() : undefined,
      related_link_by: relatedLinkBy !== "observables" ? relatedLinkBy : undefined,
    });
    const currentLocation =
      typeof window !== "undefined" ? `${window.location.pathname}${window.location.search}` : location;
    if (nextLocation !== currentLocation) {
      setLocation(nextLocation, { replace: true });
    }
  }, [activeTab, taskFilter, descriptionViewMode, visualSourceFilter, visualTypeFilter, visualAttachmentFilter, visualSearch, relatedLinkBy, location, setLocation]);

  const handleStatusChange = (newStatusCode: string) => {
    if (!id) return;
    const matched = caseStatusOptions.find((item: any) => item.code === newStatusCode);
    updateCase.mutate(
      { id, data: { status: newStatusCode } },
      {
        onSuccess: () => {
          toast.success(t("caseDetail.toast.statusChanged", { status: matched?.label || newStatusCode }));
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to update case status");
        },
      },
    );
  };

  const handleCreateForum = () => {
    if (!caseData) return;
    createForumThread.mutate(
      {
        caseId: caseData.id,
        title: `${t("caseDetail.forum.investigationPrefix")}: ${caseData.title}`,
        status: "In Progress",
        tenantId: currentTenantId,
      },
      {
        onSuccess: (thread: any) => {
          if (thread?.id) {
            setForumId(thread.id);
          }
          toast.success(t("caseDetail.toast.forumCreated"));
        },
      },
    );
  };

  const handleAddComment = () => {
    if (!newComment.trim() || !id) return;
    createComment.mutate({
      caseId: id,
      authorId: currentUserId,
      content: newComment.trim(),
      tenantId: currentTenantId,
    });
    setNewComment("");
    toast.success(t("caseDetail.toast.commentAdded"));
  };

  const handleAddTimelineNote = () => {
    if (!id) return;
    const body = timelineNoteBody.trim();
    if (!body) {
      toast.error(t("caseDetail.timeline.noteRequired"));
      return;
    }
    const title = timelineNoteTitle.trim() || t("caseDetail.timeline.noteDefaultTitle");
    createTimelineEvent.mutate(
      {
        caseId: id,
        eventType: "note",
        title,
        body,
        metadata: {
          source: "analyst_note",
        },
      },
      {
        onSuccess: () => {
          setTimelineNoteTitle("");
          setTimelineNoteBody("");
          toast.success(t("caseDetail.timeline.noteSaved"));
        },
        onError: (error: any) => {
          toast.error(error?.message || t("caseDetail.timeline.noteSaveFailed"));
        },
      },
    );
  };

  const handleApproveCaseClosure = () => {
    if (!id || !currentUserId) return;
    const actorName = String(currentUser?.name || currentUser?.username || currentUserId).trim();
    createTimelineEvent.mutate(
      {
        caseId: id,
        eventType: CLOSURE_APPROVAL_EVENT_TYPE,
        title: t("caseDetail.closureApproval.timelineTitle"),
        body: t("caseDetail.closureApproval.timelineBody", { user: actorName }),
        metadata: {
          approved: true,
          approver_id: currentUserId,
          source: "case_detail_closure_approval",
        },
      },
      {
        onSuccess: () => {
          toast.success(t("caseDetail.closureApproval.approvedToast"));
        },
        onError: (error: any) => {
          toast.error(error?.message || t("caseDetail.closureApproval.approvedToastFailed"));
        },
      },
    );
  };

  const handleScheduleReminder = (offsetMinutes: number) => {
    if (!id || offsetMinutes <= 0) return;
    const triggerAt = new Date(Date.now() + offsetMinutes * 60 * 1000);
    const reminderBody = reminderMessage.trim();
    createTimelineEvent.mutate(
      {
        caseId: id,
        eventType: "reminder",
        title: t("caseDetail.timeline.reminderScheduledTitle"),
        body: reminderBody || t("caseDetail.timeline.reminderDefaultBody"),
        metadata: {
          trigger_at: triggerAt.toISOString(),
          offset_minutes: offsetMinutes,
          source: "case_page_quick_reminder",
        },
      },
      {
        onSuccess: () => {
          toast.success(
            t("caseDetail.timeline.reminderScheduled", {
              date: format(triggerAt, "dd.MM.yyyy HH:mm"),
            }),
          );
          setReminderMessage("");
        },
        onError: (error: any) => {
          toast.error(error?.message || t("caseDetail.timeline.reminderScheduleFailed"));
        },
      },
    );
  };

  const applyPlaybookAutofill = (workflowID: string) => {
    const selected = availablePlaybooks.find((item) => item.id === workflowID);
    if (!selected) {
      return;
    }
    const payload = {
      ...playbookAutofillInput,
      workflow: {
        id: selected.id,
        name: selected.name,
      },
    };
    setSelectedPlaybookID(selected.id);
    setPlaybookInputDraft(JSON.stringify(payload, null, 2));
    setPlaybookInputDirty(false);
  };

  const handleRunPlaybookFromCase = () => {
    if (!selectedPlaybookID) {
      toast.error(t("caseDetail.timeline.playbookSelectRequired"));
      return;
    }
    const selected = availablePlaybooks.find((item) => item.id === selectedPlaybookID);
    if (!selected) {
      toast.error(t("caseDetail.timeline.playbookSelectRequired"));
      return;
    }

    let parsedInput: Record<string, any> = {};
    const rawInput = playbookInputDraft.trim();
    if (rawInput) {
      try {
        const candidate = JSON.parse(rawInput);
        if (!candidate || typeof candidate !== "object" || Array.isArray(candidate)) {
          toast.error(t("caseDetail.timeline.playbookInputInvalid"));
          return;
        }
        parsedInput = candidate;
      } catch {
        toast.error(t("caseDetail.timeline.playbookInputInvalid"));
        return;
      }
    }

    const mergedInput: Record<string, any> = {
      ...playbookAutofillInput,
      ...parsedInput,
      case_id: parsedInput.case_id || playbookAutofillInput.case_id,
      case_number: parsedInput.case_number || playbookAutofillInput.case_number,
      node: parsedInput.node || playbookAutofillInput.node,
      host: parsedInput.host || playbookAutofillInput.host,
      indicators: Array.isArray(parsedInput.indicators) && parsedInput.indicators.length > 0
        ? parsedInput.indicators
        : playbookAutofillInput.indicators,
    };
    if (!mergedInput.case || typeof mergedInput.case !== "object" || Array.isArray(mergedInput.case)) {
      mergedInput.case = playbookAutofillInput.case;
    }

    runWorkflow.mutate(
      {
        workflowID: selected.id,
        data: mergedInput,
      },
      {
        onSuccess: () => {
          setPlaybookInputDraft(JSON.stringify(mergedInput, null, 2));
          setPlaybookInputDirty(false);
          toast.success(t("caseDetail.timeline.playbookRunSuccess", { name: selected.name }));
        },
        onError: (error: any) => {
          toast.error(error?.message || t("caseDetail.timeline.playbookRunFailed"));
        },
      },
    );
  };

  const handleCreateTask = () => {
    if (!taskTitle.trim() || !id) return;
    const normalizedDueDate = fromDateTimeLocal(taskDueAt);
    if (!normalizedDueDate) {
      toast.error("Due date is required");
      return;
    }
    createTask.mutate({
      caseId: id,
      title: taskTitle.trim(),
      description: taskDesc.trim(),
      assignee: taskAssignee || null,
      dueDate: normalizedDueDate,
      mandatory: taskMandatory,
      status: "Pending",
      tenantId: currentTenantId,
    });
    setTaskTitle("");
    setTaskDesc("");
    setTaskAssignee("");
    setTaskDueAt("");
    setTaskMandatory(false);
    toast.success(t("caseDetail.toast.taskCreated"));
  };

  const handleCreateObservable = () => {
    if (!obsValue.trim() || !id) return;
    const parsedTags = normalizeObservableTags(parseObservableTagsInput(obsTags));
    createObservable.mutate({
      caseId: id,
      type: obsType,
      value: obsValue.trim(),
      verdict: obsVerdict,
      tags: parsedTags,
      tenantId: currentTenantId,
    });
    setObsValue("");
    setObsTags("");
    toast.success(t("caseDetail.toast.observableAdded"));
  };

  const handleObservableTagDraftChange = (observableID: string, value: string) => {
    setObservableTagDrafts((prev) => ({
      ...prev,
      [observableID]: value,
    }));
  };

  const handleAddObservableTag = async (observable: any) => {
    if (!id) return;
    const draft = (observableTagDrafts[observable.id] || "").trim();
    if (!draft) return;
    const nextTags = normalizeObservableTags([
      ...((Array.isArray(observable.tags) ? observable.tags : []).map((item: string) => String(item))),
      ...parseObservableTagsInput(draft),
    ]);
    if (nextTags.length === (Array.isArray(observable.tags) ? observable.tags.length : 0)) {
      setObservableTagDrafts((prev) => ({ ...prev, [observable.id]: "" }));
      return;
    }
    try {
      await updateObservable.mutateAsync({
        id: observable.id,
        data: {
          caseId: id,
          tags: nextTags,
        },
      });
      setObservableTagDrafts((prev) => ({ ...prev, [observable.id]: "" }));
      toast.success(t("caseDetail.toast.observableUpdated"));
    } catch (error: any) {
      toast.error(error?.message || t("caseDetail.toast.observableUpdateFailed"));
    }
  };

  const handleRemoveObservableTag = async (observable: any, tag: string) => {
    if (!id) return;
    const nextTags = normalizeObservableTags(
      (Array.isArray(observable.tags) ? observable.tags : []).filter(
        (item: string) => normalizeObservableType(item) !== normalizeObservableType(tag),
      ),
    );
    try {
      await updateObservable.mutateAsync({
        id: observable.id,
        data: {
          caseId: id,
          tags: nextTags,
        },
      });
      toast.success(t("caseDetail.toast.observableUpdated"));
    } catch (error: any) {
      toast.error(error?.message || t("caseDetail.toast.observableUpdateFailed"));
    }
  };

  const handleAttachmentInputChange = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file || !id) {
      return;
    }
    uploadAttachment.mutate(
      { caseId: id, file },
      {
        onSuccess: () => {
          toast.success(t("caseDetail.toast.attachmentUploaded"));
        },
        onError: (err: any) => {
          toast.error(err?.message || t("caseDetail.toast.attachmentUploadFailed"));
        },
      },
    );
    event.target.value = "";
  };

  const handleAttachmentDownload = (attachmentId: string) => {
    if (!id || !attachmentId) {
      return;
    }
    downloadAttachment.mutate(
      { caseId: id, attachmentId },
      {
        onSuccess: (url: string) => {
          if (url) {
            window.open(url, "_blank", "noopener,noreferrer");
          }
        },
        onError: (err: any) => {
          toast.error(err?.message || t("caseDetail.toast.attachmentOpenFailed"));
        },
      },
    );
  };

  const handleAddMitre = (type: "tactic" | "technique", value: string) => {
    if (!caseData || !id) return;
    const currentTactics = caseData.tactics || [];
    const currentTechniques = caseData.techniques || [];
    if (type === "tactic" && !currentTactics.includes(value)) {
      updateCase.mutate({ id, data: { tactics: [...currentTactics, value] } });
      toast.success(t("caseDetail.toast.tacticAdded"));
    } else if (type === "technique" && !currentTechniques.includes(value)) {
      updateCase.mutate({ id, data: { techniques: [...currentTechniques, value] } });
      toast.success(t("caseDetail.toast.techniqueAdded"));
    }
  };

  const handleAnalyzeCaseWithAI = () => {
    if (!id || analyzeCaseAI.isPending) return;
    analyzeCaseAI.mutate({ caseId: id, language }, {
      onSuccess: () => {
        toast.success(t("caseDetail.toast.aiCompleted"));
      },
      onError: (err: any) => {
        toast.error(err?.message || t("caseDetail.toast.aiFailed"));
      },
    });
  };

  const handleRunConnectorHubForCase = () => {
    if (!id) {
      return;
    }
    if (!connectorHubConnectorID) {
      toast.error("Select connector first");
      return;
    }

    let inputPayload: Record<string, any>;
    let metadataPayload: Record<string, any>;
    try {
      inputPayload = connectorHubDraft.inputJson.trim() ? JSON.parse(connectorHubDraft.inputJson) : {};
    } catch {
      toast.error("Input JSON is invalid");
      return;
    }
    try {
      metadataPayload = connectorHubDraft.metadataJson.trim() ? JSON.parse(connectorHubDraft.metadataJson) : {};
    } catch {
      toast.error("Metadata JSON is invalid");
      return;
    }

    executeConnectorHub.mutate(
      {
        connector_id: connectorHubConnectorID,
        method_id: connectorHubDraft.methodId || undefined,
        action: connectorHubDraft.action.trim() || undefined,
        case_id: id,
        message: connectorHubDraft.message.trim() || undefined,
        input: inputPayload,
        metadata: metadataPayload,
        dry_run: connectorHubDraft.dryRun,
      },
      {
        onSuccess: (result: any) => {
          const runStatus = String(result?.status || "").toLowerCase();
          const executionID = String(result?.id || result?.execution_id || "").trim();
          if (executionID) {
            setSelectedConnectorExecutionID(executionID);
            setConnectorExecutionDrawerOpen(true);
          }
          if (runStatus === "completed" || runStatus === "dry_run") {
            toast.success(runStatus === "dry_run" ? "Connector hub dry-run completed" : "Connector hub execution completed");
            return;
          }
          if (runStatus === "accepted" || runStatus === "queued" || runStatus === "dispatching" || runStatus === "retry_scheduled" || runStatus === "provider_accepted") {
            toast.success("Connector hub execution queued");
            return;
          }
          toast.error(result?.error || "Connector hub execution failed");
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to execute connector hub run");
        },
      },
    );
  };

  const handleAddCustomFieldRow = () => {
    setCustomFieldRows((prev) => [...prev, { id: createCustomFieldDraftID(), key: "", value: "" }]);
    setCustomFieldsDirty(true);
  };

  const handleAddSOARCustomFieldRows = () => {
    const merged = mergeMissingSOARCaseFields(
      customFieldRows,
      (preset) => ({ id: createCustomFieldDraftID(), key: preset.key, value: "" }),
      normalizeSOARCaseFieldKey,
    );
    setCustomFieldRows(merged);
    setCustomFieldsDirty(true);
    if (merged.length > customFieldRows.length) {
      toast.success(t("caseDetail.customFields.soarAdded"));
      return;
    }
    toast.success(t("caseDetail.customFields.soarAlreadyPresent"));
  };

  const handleUpdateCustomFieldRow = (rowID: string, patch: Partial<CustomFieldDraftRow>) => {
    setCustomFieldRows((prev) => prev.map((row) => (row.id === rowID ? { ...row, ...patch } : row)));
    setCustomFieldsDirty(true);
  };

  const handleRemoveCustomFieldRow = (rowID: string) => {
    setCustomFieldRows((prev) => prev.filter((row) => row.id !== rowID));
    setCustomFieldsDirty(true);
  };

  const handleSaveCustomFields = () => {
    if (!id) return;
    const payload: Record<string, string> = {};
    for (const row of customFieldRows) {
      const key = row.key.trim();
      const value = row.value;
      if (!key) {
        if (value.trim()) {
          toast.error(t("caseDetail.customFields.keyRequired"));
          return;
        }
        continue;
      }
      if (Object.prototype.hasOwnProperty.call(payload, key)) {
        toast.error(t("caseDetail.customFields.duplicate"));
        return;
      }
      payload[key] = value;
    }
    updateCase.mutate(
      { id, data: { customFields: payload } },
      {
        onSuccess: () => {
          setCustomFieldRows(toCustomFieldDraftRows(payload));
          setCustomFieldsDirty(false);
          toast.success(t("caseDetail.customFields.saved"));
        },
        onError: (error: any) => {
          toast.error(error?.message || t("caseDetail.customFields.saveFailed"));
        },
      },
    );
  };

  const handleSetTaskChainMode = (mode: (typeof TASK_CHAIN_MODES)[number]) => {
    if (!id) return;
    const currentCustomFields = (caseData?.customFields && typeof caseData.customFields === "object")
      ? (caseData.customFields as Record<string, any>)
      : {};
    updateCase.mutate(
      {
        id,
        data: {
          customFields: {
            ...currentCustomFields,
            task_chain_mode: mode,
          },
        },
      },
      {
        onSuccess: () => {
          toast.success(t("caseDetail.task.chain.saved"));
        },
        onError: (error: any) => {
          toast.error(error?.message || t("caseDetail.task.chain.saveFailed"));
        },
      },
    );
  };

  const handleCreateCustomCaseCategory = () => {
    const name = newCaseCategoryName.trim();
    if (!name) {
      return;
    }
    createCaseCategory.mutate(
      { name },
      {
        onSuccess: () => {
          patchCaseEditDraft({ category: name });
          setNewCaseCategoryName("");
          toast.success(t("caseDetail.classification.categoryCreated"));
        },
        onError: (error: any) => {
          toast.error(error?.message || t("caseDetail.classification.categoryCreateFailed"));
        },
      },
    );
  };

  const handleRemoveMitre = (type: "tactic" | "technique", value: string) => {
    if (!caseData || !id) return;
    if (type === "tactic") {
      updateCase.mutate({ id, data: { tactics: (caseData.tactics || []).filter((t: string) => t !== value) } });
    } else {
      updateCase.mutate({ id, data: { techniques: (caseData.techniques || []).filter((t: string) => t !== value) } });
    }
  };

  const handleOpenEditCase = () => {
    setCaseEditDraft(createCaseEditDraft(caseData));
    setCaseEditBaseUpdatedAt(String(caseData?.updatedAt || "").trim());
    setEditDialogOpen(true);
  };

  const handleDuplicateCase = () => {
    if (!id || !caseData) return;
    const sourceTitle = String(caseData.title || "").trim();
    const duplicatedTitle = sourceTitle ? `Copy of ${sourceTitle}` : "Case copy";
    copyCase.mutate(
      {
        id,
        data: {
          title: duplicatedTitle,
          assignee: caseData.assignee || "",
          tags: Array.isArray(caseData.tags) ? caseData.tags : [],
          customFields: caseData.customFields || {},
          owner: caseData.owner || "",
          includeObservables: true,
        },
      },
      {
        onSuccess: (payload: any) => {
          const duplicatedCaseID = String(payload?.id || "").trim();
          setDuplicateCaseConfirmOpen(false);
          toast.success("Case duplicated");
          if (duplicatedCaseID) {
            const tenantSlugOrID = currentTenantSlug || currentTenantId;
            if (tenantSlugOrID) {
              setLocation(withTenantPath(tenantSlugOrID, `/cases/${duplicatedCaseID}`));
            } else {
              setLocation(`/cases/${duplicatedCaseID}`);
            }
          }
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to duplicate case");
        },
      },
    );
  };

  const handleSaveCase = () => {
    if (!id || !caseEditDraft) return;
    const confidenceRaw = caseEditDraft.confidence.trim();
    let confidenceValue: number | undefined = undefined;
    if (confidenceRaw !== "") {
      const parsedConfidence = Number(confidenceRaw);
      if (Number.isNaN(parsedConfidence)) {
        toast.error("Confidence must be a number between 0 and 100");
        return;
      }
      confidenceValue = Math.max(0, Math.min(100, Math.round(parsedConfidence)));
    }
    const recommendations = caseEditDraft.recommendations
      .split("\n")
      .map((item) => item.trim())
      .filter(Boolean);

    updateCase.mutate(
      {
        id,
        data: {
          caseNumber: caseEditDraft.caseNumber.trim(),
          title: caseEditDraft.title.trim(),
          description: caseEditDraft.description,
          source: caseEditDraft.source.trim(),
          incidentType: caseEditDraft.incidentType.trim(),
          category: caseEditDraft.category.trim(),
          relatedProduct: caseEditDraft.relatedProduct.trim(),
          status: caseEditDraft.statusCode,
          priority: caseEditDraft.priority.trim(),
          impact: caseEditDraft.impact.trim(),
          confidence: confidenceValue,
          sev: caseEditDraft.severity,
          tlp: normalizeTrafficLight(caseEditDraft.tlp),
          pap: normalizeTrafficLight(caseEditDraft.pap),
          assignee: caseEditDraft.assignee.trim(),
          expectedUpdatedAt: caseEditBaseUpdatedAt || String(caseData?.updatedAt || "").trim(),
          detectedAt: fromDateTimeLocal(caseEditDraft.detectedAt),
          occurredAt: fromDateTimeLocal(caseEditDraft.occurredAt),
          closedAt: fromDateTimeLocal(caseEditDraft.closedAt),
          resolutionSummary: caseEditDraft.resolutionSummary,
          stage: caseEditDraft.stage.trim(),
          verdict: caseEditDraft.verdict,
          recommendations,
        },
      },
      {
        onSuccess: () => {
          setEditDialogOpen(false);
          setCaseEditBaseUpdatedAt("");
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to update case");
        },
      },
    );
  };

  const handleEscalateCase = () => {
    if (!id) return;
    const targetTenantID = String(escalationDraft.targetTenantID || "").trim();
    if (!targetTenantID) {
      toast.error("Select target tenant");
      return;
    }

    escalateCase.mutate(
      {
        caseId: id,
        data: {
          targetTenantId: targetTenantID,
          handoffType: escalationDraft.handoffType,
          summary: escalationDraft.summary,
          includeObservables: escalationDraft.includeObservables,
        },
      },
      {
        onSuccess: (payload: any) => {
          toast.success("Case escalated");
          setEscalateDialogOpen(false);

          const targetCaseID = String(payload?.target_case?.id || "").trim();
          const targetTenantSlug = String(
            payload?.target_tenant_slug ||
              escalationTargetTenants.find((tenant: any) => tenant.id === targetTenantID)?.slug ||
              "",
          ).trim();
          if (targetCaseID && targetTenantSlug) {
            setLocation(withTenantPath(targetTenantSlug, `/cases/${targetCaseID}`));
          }
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to escalate case");
        },
      },
    );
  };

  const handleShareCaseAcrossTenants = () => {
    if (!id) return;
    const targetTenantID = String(shareDraft.targetTenantID || "").trim();
    if (!targetTenantID) {
      toast.error("Select target tenant");
      return;
    }
    shareCaseAcrossTenants.mutate(
      {
        caseId: id,
        data: {
          targetTenantId: targetTenantID,
        },
      },
      {
        onSuccess: () => {
          toast.success("Case is now shared as one entity across tenants");
          setShareDialogOpen(false);
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to share case");
        },
      },
    );
  };

  const patchCaseEditDraft = (patch: Partial<CaseEditDraft>) => {
    setCaseEditDraft((prev) => (prev ? { ...prev, ...patch } : prev));
  };

  const getInlineFieldValue = (field: InlineCaseFieldKey): string => {
    switch (field) {
      case "title":
        return String(caseData.title || "");
      case "incidentType":
        return String(caseData.incidentType || "");
      case "category":
        return String(caseData.category || "");
      case "relatedProduct":
        return String(caseData.relatedProduct || "");
      case "source":
        return String(caseData.source || "");
      case "statusCode":
        return String(caseData.statusCode || "open");
      case "severity":
        return String(caseData.sev || "Medium");
      case "priority":
        return String(caseData.priority || "");
      case "stage":
        return String(caseData.stage || "");
      case "tlp":
        return String(caseData.tlp || "TLP:AMBER");
      case "assignee":
        return String(caseData.assignee || "");
      case "owner":
        return String(caseData.owner || "");
      case "impact":
        return String(caseData.impact || "");
      case "confidence":
        return String(caseData.confidence ?? 0);
      case "detectedAt":
        return toDateTimeLocal(caseData.detectedAt);
      case "tags":
        return Array.isArray(caseData.tags) ? caseData.tags.join(", ") : "";
      case "description":
        return String(caseData.description || "");
      case "resolutionSummary":
        return String(caseData.resolutionSummary || "");
      default:
        return "";
    }
  };

  const beginInlineEdit = (field: InlineCaseFieldKey) => {
    setInlineEditingField(field);
    setInlineEditingValue(getInlineFieldValue(field));
    setInlineEditBaseUpdatedAt(String(caseData?.updatedAt || "").trim());
  };

  const cancelInlineEdit = () => {
    setInlineEditingField(null);
    setInlineEditingValue("");
    setInlineEditBaseUpdatedAt("");
    setInlineSavingField(null);
  };

  const buildInlinePatch = (field: InlineCaseFieldKey, rawValue: string): Record<string, any> | null => {
    const value = String(rawValue ?? "");
    switch (field) {
      case "title": {
        const title = value.trim();
        if (!title) {
          toast.error(t("alerts.field.title"));
          return null;
        }
        return { title };
      }
      case "incidentType":
        return { incidentType: value.trim() };
      case "category":
        return { category: value.trim() };
      case "relatedProduct":
        return { relatedProduct: value.trim() };
      case "source":
        return { source: value.trim() };
      case "statusCode":
        return { status: value.trim().toLowerCase() };
      case "severity":
        return { sev: value.trim() || "Medium" };
      case "priority":
        return { priority: value.trim() };
      case "stage":
        return { stage: value.trim() };
      case "tlp":
        return { tlp: normalizeTrafficLight(value) };
      case "assignee":
        return { assignee: value.trim() };
      case "owner":
        return { owner: value.trim() };
      case "impact":
        return { impact: value.trim() };
      case "confidence": {
        const parsed = Number(value.trim());
        if (Number.isNaN(parsed)) {
          toast.error("Confidence must be a number between 0 and 100");
          return null;
        }
        return { confidence: Math.max(0, Math.min(100, Math.round(parsed))) };
      }
      case "detectedAt":
        return { detectedAt: fromDateTimeLocal(value) };
      case "tags": {
        const tags = value
          .split(",")
          .map((item) => item.trim())
          .filter(Boolean);
        return { tags: Array.from(new Set(tags)) };
      }
      case "description":
        return { description: value };
      case "resolutionSummary":
        return { resolutionSummary: value };
      default:
        return null;
    }
  };

  const saveInlineEdit = (overrideValue?: string) => {
    if (!inlineEditingField || !id || inlineSavingField) return;
    const nextValue = overrideValue ?? inlineEditingValue;
    const patch = buildInlinePatch(inlineEditingField, nextValue);
    if (!patch) return;
    setInlineSavingField(inlineEditingField);
    updateCase.mutate(
      {
        id,
        data: {
          ...patch,
          expectedUpdatedAt: inlineEditBaseUpdatedAt || String(caseData?.updatedAt || "").trim(),
        },
      },
      {
        onSuccess: () => {
          cancelInlineEdit();
        },
        onError: (error: any) => {
          setInlineSavingField(null);
          toast.error(error?.message || "Failed to update case field");
        },
      },
    );
  };

  useEffect(() => {
    if (!inlineEditingField || !INLINE_AUTOSAVE_FIELDS.has(inlineEditingField)) {
      return;
    }
    const handlePointerDown = (event: MouseEvent) => {
      if (!inlineEditorRef.current) return;
      const target = event.target as Node | null;
      if (target && inlineEditorRef.current.contains(target)) return;
      const targetElement = event.target as Element | null;
      if (
        targetElement &&
        (targetElement.closest("[data-radix-popper-content-wrapper]") ||
          targetElement.closest("[data-radix-select-content]"))
      ) {
        return;
      }
      saveInlineEdit();
    };
    document.addEventListener("mousedown", handlePointerDown);
    return () => document.removeEventListener("mousedown", handlePointerDown);
  }, [inlineEditingField, inlineEditingValue, inlineSavingField]);

  const taskArr = tasks || [];
  const doneTasks = taskArr.filter((t: any) => t.status === "Done").length;
  const cancelledTasks = taskArr.filter((t: any) => t.status === "Cancelled").length;
  const taskCompletion = taskArr.length > 0 ? Math.round((doneTasks / taskArr.length) * 100) : 0;
  const openTasks = useMemo(
    () => taskArr.filter((task: any) => task.status !== "Done" && task.status !== "Cancelled"),
    [taskArr],
  );
  const taskChainMode = useMemo(() => {
    const raw = String((caseData?.customFields as any)?.task_chain_mode || "").trim().toLowerCase();
    if (raw === "manual" || raw === "semi_automated" || raw === "automated") {
      return raw as (typeof TASK_CHAIN_MODES)[number];
    }
    return "manual";
  }, [caseData?.customFields]);
  const taskSLA = useMemo(() => {
    const now = Date.now();
    const overdue = openTasks.filter((task: any) => {
      const dueTimestamp = toTimestamp(task?.dueDate);
      return dueTimestamp > 0 && dueTimestamp < now;
    });
    const upcoming = openTasks.filter((task: any) => {
      const dueTimestamp = toTimestamp(task?.dueDate);
      if (dueTimestamp <= 0) return false;
      const delta = dueTimestamp - now;
      return delta >= 0 && delta <= 24 * 60 * 60 * 1000;
    });
    return {
      overdueCount: overdue.length,
      upcomingCount: upcoming.length,
    };
  }, [openTasks]);
  const caseSLA = useMemo(() => {
    const customSlaRaw = Number(
      (caseData?.customFields as any)?.sla_target_minutes ??
      (caseData?.customFields as any)?.sla_minutes ??
      0,
    );
    const targetMinutes = Number.isFinite(customSlaRaw) && customSlaRaw > 0
      ? Math.round(customSlaRaw)
      : CASE_SLA_MINUTES_BY_SEVERITY[String(caseData?.sev || "").toLowerCase()] || 480;
    const startedAtTs = toTimestamp(caseData?.time || caseData?.detectedAt);
    const dueAtTs = startedAtTs > 0 ? startedAtTs + targetMinutes * 60 * 1000 : 0;
    const nowTs = Date.now();
    const statusRaw = String(caseData?.statusCode || caseData?.status || "").toLowerCase();
    const isClosed = statusRaw.includes("closed") || statusRaw.includes("resolved");
    const breached = dueAtTs > 0 && nowTs > dueAtTs && !isClosed;
    const remainingMs = dueAtTs > 0 ? dueAtTs - nowTs : 0;
    const elapsedMs = startedAtTs > 0 ? Math.max(0, nowTs - startedAtTs) : 0;
    const totalMs = targetMinutes * 60 * 1000;
    const progress = totalMs > 0 ? Math.max(0, Math.min(100, Math.round((elapsedMs / totalMs) * 100))) : 0;
    return {
      targetMinutes,
      dueAtTs,
      breached,
      remainingMs,
      progress,
    };
  }, [caseData?.customFields, caseData?.detectedAt, caseData?.sev, caseData?.status, caseData?.statusCode, caseData?.time]);
  const timelineRows = Array.isArray(timeline) ? timeline : [];
  const timelineRowsDesc = useMemo(
    () => [...timelineRows].sort((left: any, right: any) => toTimestamp(right.createdAt) - toTimestamp(left.createdAt)),
    [timelineRows],
  );
  const timelineRowsAsc = useMemo(() => [...timelineRowsDesc].reverse(), [timelineRowsDesc]);
  const closureApprovalsByUser = useMemo(() => {
    const out = new Map<string, any>();
    timelineRowsDesc.forEach((event: any) => {
      const normalizedEventType = String(event?.eventType || "").trim().toLowerCase();
      if (normalizedEventType !== CLOSURE_APPROVAL_EVENT_TYPE) {
        return;
      }
      const actorID = String(event?.userId || event?.metadata?.approver_id || "").trim();
      if (!actorID || out.has(actorID)) {
        return;
      }
      out.set(actorID, event);
    });
    return out;
  }, [timelineRowsDesc]);
  const requiredClosureApproverIDs = useMemo(
    () => Array.from(new Set(closureApprovalConfig.requiredApproverIDs.map((item) => String(item || "").trim()).filter(Boolean))),
    [closureApprovalConfig.requiredApproverIDs],
  );
  const requiredClosureApprovals = useMemo(
    () => Math.max(closureApprovalConfig.requiredApprovals, requiredClosureApproverIDs.length),
    [closureApprovalConfig.requiredApprovals, requiredClosureApproverIDs.length],
  );
  const missingRequiredClosureApprovers = useMemo(
    () => requiredClosureApproverIDs.filter((userID) => !closureApprovalsByUser.has(userID)),
    [requiredClosureApproverIDs, closureApprovalsByUser],
  );
  const hasCurrentUserClosureApproval = useMemo(
    () => closureApprovalsByUser.has(String(currentUserId || "").trim()),
    [closureApprovalsByUser, currentUserId],
  );
  const casePlaybookRuns = useMemo(
    () => (Array.isArray(casePlaybookRunsData?.runs) ? casePlaybookRunsData.runs : []),
    [casePlaybookRunsData?.runs],
  );
  const timelineMetrics = useMemo(() => {
    const durationsByAction = new Map<string, number>();
    const actionCounts = new Map<string, number>();
    const statusCounts = new Map<string, number>();
    let measuredDurationMs = 0;
    let measuredActions = 0;
    let successSignals = 0;
    let failureSignals = 0;

    const nowTs = Date.now();
    timelineRowsAsc.forEach((event: any, index: number) => {
      const currentTs = toTimestamp(event?.createdAt) || nowTs;
      const nextTs =
        index < timelineRowsAsc.length - 1
          ? toTimestamp(timelineRowsAsc[index + 1]?.createdAt) || nowTs
          : nowTs;
      const durationMs = Math.max(0, nextTs - currentTs);
      const actionLabel = String(
        event?.metadata?.stage ||
          event?.metadata?.action ||
          event?.metadata?.step ||
          event?.title ||
          event?.eventType ||
          "event",
      ).trim();
      const normalizedAction = humanizeTimelineEventType(actionLabel);
      durationsByAction.set(normalizedAction, (durationsByAction.get(normalizedAction) || 0) + durationMs);
      actionCounts.set(normalizedAction, (actionCounts.get(normalizedAction) || 0) + 1);
      measuredDurationMs += durationMs;
      measuredActions += 1;

      const statusTarget = String(
        event?.metadata?.to_status ||
          event?.metadata?.status ||
          event?.metadata?.state ||
          "",
      ).trim();
      if (statusTarget) {
        const normalizedStatus = humanizeTimelineEventType(statusTarget);
        statusCounts.set(normalizedStatus, (statusCounts.get(normalizedStatus) || 0) + 1);
      }

      const signal = String(
        event?.metadata?.outcome ||
          event?.metadata?.result ||
          event?.metadata?.status ||
          "",
      ).toLowerCase();
      if (signal.includes("success") || signal.includes("completed") || signal.includes("done") || signal === "ok") {
        successSignals += 1;
      } else if (signal.includes("failed") || signal.includes("error") || signal.includes("cancel")) {
        failureSignals += 1;
      }
    });

    const successRate =
      taskArr.length > 0
        ? Math.round((doneTasks / taskArr.length) * 100)
        : successSignals + failureSignals > 0
          ? Math.round((successSignals / (successSignals + failureSignals)) * 100)
          : 0;

    return {
      totalActions: timelineRowsAsc.length,
      avgActionDurationMs: measuredActions > 0 ? Math.round(measuredDurationMs / measuredActions) : 0,
      successRate,
      statusBreakdown: Array.from(statusCounts.entries())
        .sort((left, right) => right[1] - left[1])
        .slice(0, 6)
        .map(([status, count]) => ({ status, count })),
      actionDurations: Array.from(durationsByAction.entries())
        .sort((left, right) => right[1] - left[1])
        .slice(0, 6)
        .map(([action, durationMs]) => ({
          action,
          durationMs,
          count: actionCounts.get(action) || 0,
        })),
    };
  }, [doneTasks, taskArr, timelineRowsAsc]);
  const visualizationObservableRows = useMemo(
    () =>
      observableRows.map((observable: any) => {
        const source = classifyObservableSource(observable?.type, observable?.value, Array.isArray(observable?.tags) ? observable.tags : []);
        return {
          ...observable,
          source,
        };
      }),
    [observableRows],
  );
  const visualSearchLower = visualSearch.trim().toLowerCase();
  const visualTypeFilterLower = visualTypeFilter === "all" ? "all" : String(visualTypeFilter || "").trim().toLowerCase();
  const visualSearchFilteredObservables = useMemo(
    () =>
      visualizationObservableRows.filter((observable: any) => {
        if (!visualSearchLower) {
          return true;
        }
        const haystack = `${observable?.type || ""} ${observable?.value || ""} ${Array.isArray(observable?.tags) ? observable.tags.join(" ") : ""}`.toLowerCase();
        return haystack.includes(visualSearchLower);
      }),
    [visualSearchLower, visualizationObservableRows],
  );
  const sourceCountsForCurrentVisual = useMemo(() => {
    const counts = {
      network: 0,
      authorization: 0,
      other: 0,
    };
    visualSearchFilteredObservables.forEach((observable: any) => {
      const observableType = String(observable?.type || "").trim().toLowerCase();
      if (visualTypeFilterLower !== "all" && observableType !== visualTypeFilterLower) {
        return;
      }
      const source = String(observable?.source || "").trim() as "network" | "authorization" | "other";
      if (source === "network" || source === "authorization" || source === "other") {
        counts[source] += 1;
      }
    });
    return counts;
  }, [visualSearchFilteredObservables, visualTypeFilterLower]);
  const observablesBySourceChartData = useMemo(() => {
    return [
      { sourceKey: "network", source: t("caseDetail.visual.source.network"), value: sourceCountsForCurrentVisual.network },
      { sourceKey: "authorization", source: t("caseDetail.visual.source.authorization"), value: sourceCountsForCurrentVisual.authorization },
      { sourceKey: "other", source: t("caseDetail.visual.source.other"), value: sourceCountsForCurrentVisual.other },
    ];
  }, [sourceCountsForCurrentVisual, t]);
  const observablesByTypeChartData = useMemo(() => {
    const counts = new Map<string, number>();
    visualSearchFilteredObservables.forEach((observable: any) => {
      if (visualSourceFilter !== "all" && observable.source !== visualSourceFilter) {
        return;
      }
      const type = String(observable?.type || "").trim() || "Unknown";
      counts.set(type, (counts.get(type) || 0) + 1);
    });
    return Array.from(counts.entries())
      .sort((left, right) => right[1] - left[1])
      .slice(0, 8)
      .map(([type, value]) => ({
        type,
        value,
      }));
  }, [visualSearchFilteredObservables, visualSourceFilter]);
  const filteredVisualizationObservables = useMemo(
    () =>
      visualSearchFilteredObservables.filter((observable: any) => {
        if (visualSourceFilter !== "all" && observable.source !== visualSourceFilter) {
          return false;
        }
        const observableType = String(observable?.type || "").trim().toLowerCase();
        if (visualTypeFilterLower !== "all" && observableType !== visualTypeFilterLower) {
          return false;
        }
        return true;
      }),
    [visualSearchFilteredObservables, visualSourceFilter, visualTypeFilterLower],
  );
  const visualizationAttachmentRows = useMemo(
    () =>
      attachments.map((attachment: any) => ({
        ...attachment,
        kind: isImageAttachment(attachment?.contentType, attachment?.fileName) ? "image" : "file",
      })),
    [attachments],
  );
  const filteredVisualizationAttachments = useMemo(
    () =>
      visualizationAttachmentRows.filter((attachment: any) => {
        if (visualAttachmentFilter === "images" && attachment.kind !== "image") {
          return false;
        }
        if (visualAttachmentFilter === "files" && attachment.kind !== "file") {
          return false;
        }
        if (!visualSearchLower) {
          return true;
        }
        const haystack = `${attachment?.fileName || ""} ${attachment?.contentType || ""}`.toLowerCase();
        return haystack.includes(visualSearchLower);
      }),
    [visualAttachmentFilter, visualSearchLower, visualizationAttachmentRows],
  );
  const imageAttachmentCount = useMemo(
    () => visualizationAttachmentRows.filter((attachment: any) => attachment.kind === "image").length,
    [visualizationAttachmentRows],
  );
  const fileAttachmentCount = useMemo(
    () => visualizationAttachmentRows.filter((attachment: any) => attachment.kind === "file").length,
    [visualizationAttachmentRows],
  );
  const visualSourceLabels = useMemo(
    () => ({
      network: t("caseDetail.visual.source.network"),
      authorization: t("caseDetail.visual.source.authorization"),
      other: t("caseDetail.visual.source.other"),
    }),
    [t],
  );
  const observableCount = (observables || []).length;
  const attachmentCount = attachments.length;
  const commentCount = (comments || []).length;
  const latestCaseAIAnalysis = caseAIAnalyses[0];
  const selectedConnectorHubMethod = useMemo(
    () => (Array.isArray(connectorHubMethods) ? connectorHubMethods : []).find((item: any) => String(item?.id || "") === connectorHubDraft.methodId) || null,
    [connectorHubMethods, connectorHubDraft.methodId],
  );
  const selectedPlaybook = useMemo(
    () => availablePlaybooks.find((item) => item.id === selectedPlaybookID) || null,
    [availablePlaybooks, selectedPlaybookID],
  );
  const selectedPlaybookGraph = useMemo(() => {
    const definition = (selectedPlaybook as any)?.definition;
    const nodes = Array.isArray(definition?.nodes) ? definition.nodes : [];
    const edges = Array.isArray(definition?.edges) ? definition.edges : [];
    const nodeByID = new Map<string, any>(nodes.map((node: any) => [String(node.id), node]));
    const orderedNodes = [...nodes].sort((left: any, right: any) => {
      const leftX = Number(left?.position?.x || 0);
      const rightX = Number(right?.position?.x || 0);
      if (leftX !== rightX) {
        return leftX - rightX;
      }
      return Number(left?.position?.y || 0) - Number(right?.position?.y || 0);
    });
    const edgeView = edges
      .map((edge: any) => {
        const sourceID = String(edge?.source || "");
        const targetID = String(edge?.target || "");
        const sourceNode = nodeByID.get(sourceID);
        const targetNode = nodeByID.get(targetID);
        if (!sourceNode || !targetNode) return null;
        return {
          id: edge?.id || `${sourceID}-${targetID}`,
          sourceLabel: sourceNode.label || sourceID,
          targetLabel: targetNode.label || targetID,
          condition: String(edge?.condition || edge?.label || "").trim(),
        };
      })
      .filter(Boolean) as Array<{ id: string; sourceLabel: string; targetLabel: string; condition: string }>;
    return {
      orderedNodes,
      edgeView,
    };
  }, [selectedPlaybook]);
  const recentCaseUpdates = useMemo(() => timelineRowsDesc.slice(0, 5), [timelineRowsDesc]);
  const suggestedPlaybookIDs = useMemo(
    () => new Set(suggestedPlaybooks.map((item) => item.id)),
    [suggestedPlaybooks],
  );
  const caseExportPayload = useMemo(
    () => ({
      exportedAt: new Date().toISOString(),
      case: caseData,
      timeline: timelineRowsDesc,
      tasks: taskArr,
      observables: observables || [],
      attachments,
      comments: comments || [],
      playbookRuns: casePlaybookRuns,
    }),
    [attachments, caseData, casePlaybookRuns, comments, observables, taskArr, timelineRowsDesc],
  );
  const caseDataMismatchedRoute = Boolean(
    caseData && id && String(caseData.id || "").trim() && String(caseData.id || "").trim() !== String(id).trim(),
  );
  const isCaseDetailInitialLoading = Boolean(caseLoading && (!caseData || caseDataMismatchedRoute));
  const showCaseLoadingSkeleton = useMinimumLoading(isCaseDetailInitialLoading);

  const handleExportCase = (formatKind: "json" | "csv" | "md") => {
    if (!caseData) {
      return;
    }
    const safeBaseName = String(caseData.caseNumber || caseData.id || "case")
      .trim()
      .replace(/[^a-zA-Z0-9_-]+/g, "_");
    let fileName = `${safeBaseName}.json`;
    let mimeType = "application/json";
    let content = JSON.stringify(caseExportPayload, null, 2);
    if (formatKind === "csv") {
      fileName = `${safeBaseName}.csv`;
      mimeType = "text/csv;charset=utf-8";
      content = buildCaseExportCSV(caseExportPayload);
    } else if (formatKind === "md") {
      fileName = `${safeBaseName}.md`;
      mimeType = "text/markdown;charset=utf-8";
      content = buildCaseExportMarkdown(caseExportPayload);
    }

    const blob = new Blob([content], { type: mimeType });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = fileName;
    anchor.rel = "noopener";
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
    URL.revokeObjectURL(url);
    toast.success(t("caseDetail.timeline.exportDone", { format: formatKind.toUpperCase() }));
  };

  if (showCaseLoadingSkeleton) {
    return <CaseDetailLoadingSkeleton shellClass={CASE_PAGE_SHELL_CLASS} panelPaddedClass={CASE_PANEL_PADDED_CLASS} />;
  }

  if (!caseData) {
    return (
      <AppLayout>
        <div className="mx-auto flex w-full max-w-[1240px] flex-col items-center justify-center gap-4 py-20" data-testid="case-not-found">
          <AlertTriangle size={48} className="text-[#9ca3af]" />
          <h2 className="text-xl font-bold">{t("caseDetail.notFound.title")}</h2>
          <Link href={withTenantPath(currentTenantSlug, "/cases")}>
            <Button variant="outline" className="rounded-xl gap-2" data-testid="button-back-not-found">
              <ArrowLeft size={16} /> {t("caseDetail.notFound.back")}
            </Button>
          </Link>
        </div>
      </AppLayout>
    );
  }

  const filteredTasks = taskFilter === "All" ? taskArr : taskArr.filter((t: any) => t.status === taskFilter);
  const severityLabel = (() => {
    const severityKey = `severity.${String(caseData.sev || "").toLowerCase()}`;
    const translated = t(severityKey);
    return translated !== severityKey ? translated : (caseData.sev || "—");
  })();

  const startInlineEdit = (field: InlineCaseFieldKey) => {
    if (inlineSavingField) return;
    beginInlineEdit(field);
  };

  const renderInlineEditButton = (field: InlineCaseFieldKey, testId?: string, size: "sm" | "md" = "sm") => (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      className={`shrink-0 opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100 ${
        size === "md" ? CASE_INLINE_EDIT_BUTTON_CLASS : "h-6 w-6 rounded-md text-[#9ca3af] hover:bg-[#1a1f2d] hover:text-[#f3f4f6]"
      }`}
      onClick={(event) => {
        event.preventDefault();
        event.stopPropagation();
        startInlineEdit(field);
      }}
      data-testid={testId || `button-inline-edit-${field}`}
    >
      <Edit size={size === "md" ? 14 : 12} />
    </Button>
  );

  const renderInlineEditorActions = (field: InlineCaseFieldKey, compact = false) => {
    if (compact) {
      return null;
    }
    const saving = inlineSavingField === field;
    return (
      <div className={`flex items-center gap-1 ${compact ? "justify-end" : "mt-2 justify-end"}`}>
        <Button
          type="button"
          size={compact ? "icon" : "sm"}
          className={compact ? "h-7 w-7 rounded-md bg-[#11141d] text-[#f3f4f6] hover:bg-[#1d2433]" : "h-7 rounded-md bg-[#11141d] text-[#f3f4f6] hover:bg-[#1d2433]"}
          disabled={saving}
          onClick={(event) => {
            event.preventDefault();
            event.stopPropagation();
            saveInlineEdit();
          }}
          data-testid={`button-inline-save-${field}`}
        >
          {compact ? <CheckCircle2 size={14} /> : t("common.save")}
        </Button>
        <Button
          type="button"
          variant="outline"
          size={compact ? "icon" : "sm"}
          className={compact ? "h-7 w-7 rounded-md border-[#2a2c3c] bg-[#0f1118] text-[#d1d5db] hover:bg-[#1a1f2d]" : "h-7 rounded-md border-[#2a2c3c] bg-[#0f1118] text-[#d1d5db] hover:bg-[#1a1f2d]"}
          disabled={saving}
          onClick={(event) => {
            event.preventDefault();
            event.stopPropagation();
            cancelInlineEdit();
          }}
          data-testid={`button-inline-cancel-${field}`}
        >
          {compact ? <XCircle size={14} /> : t("common.cancel")}
        </Button>
      </div>
    );
  };

  return (
    <AppLayout>
      <div className={CASE_PAGE_SHELL_CLASS} data-testid="case-detail-page">
        {/* Header */}
        <div className={`${CASE_PANEL_PADDED_CLASS} flex flex-col gap-4`}>
          <div className="flex flex-wrap items-center gap-3">
            <Link href={withTenantPath(currentTenantSlug, "/cases")}>
              <Button
                variant="ghost"
                size="icon"
                className="h-10 w-10 rounded-xl border border-[#2a2c3c] bg-[#0f1118] text-[#d1d5db] hover:bg-[#1a1f2d] hover:text-white"
                data-testid="button-back"
              >
                <ArrowLeft size={20} />
              </Button>
            </Link>
            {inlineEditingField === "title" ? (
              <div ref={inlineEditorRef} className="flex-1 min-w-0 rounded-xl border border-[#2a2c3c] bg-[#0f1118] p-2" data-testid="inline-editor-title">
                <Input
                  autoFocus
                  value={inlineEditingValue}
                  onChange={(event) => setInlineEditingValue(event.target.value)}
                  className={CASE_INPUT_CLASS}
                  data-testid="input-inline-title"
                />
                {renderInlineEditorActions("title")}
              </div>
            ) : (
              <div className="group flex flex-1 min-w-0 items-center gap-1">
                <h1 className="flex-1 min-w-0 text-2xl font-semibold tracking-[-0.01em] text-white" data-testid="text-case-title">
                  <EllipsisText text={caseData.title} className="w-full" />
                </h1>
                {renderInlineEditButton("title")}
              </div>
            )}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  variant="outline"
                  size="sm"
                  className={`h-8 rounded-lg gap-1 text-xs font-semibold ${currentCaseStatus.isClosed ? statusColors.Closed : statusColors.Open}`}
                  data-testid="dropdown-status"
                >
                  {currentCaseStatus.label} <ChevronDown size={12} />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent className="rounded-lg border-[#2a2c3c] bg-[#13141c] text-[#f3f4f6]">
                {caseStatusOptions.map((status: any) => (
                  <DropdownMenuItem
                    key={status.code}
                    onClick={() => handleStatusChange(status.code)}
                    className="text-[#d1d5db] focus:bg-[#1a1f2d] focus:text-white"
                    data-testid={`status-option-${status.code}`}
                  >
                    {status.label}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
            {inlineEditingField === "severity" ? (
              <div ref={inlineEditorRef} className="rounded-lg border border-[#2a2c3c] bg-[#0f1118] p-2" data-testid="inline-editor-severity">
                <Select value={inlineEditingValue || "Medium"} onValueChange={setInlineEditingValue}>
                  <SelectTrigger className={`${CASE_SELECT_TRIGGER_CLASS} min-w-[140px]`} data-testid="select-inline-severity">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent className={CASE_SELECT_CONTENT_CLASS}>
                    {["Critical", "High", "Medium", "Low"].map((severity) => (
                      <SelectItem key={severity} value={severity}>
                        {severity}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {renderInlineEditorActions("severity", true)}
              </div>
            ) : (
              <button
                type="button"
                className="inline-flex cursor-pointer items-center rounded-lg transition-opacity hover:opacity-90"
                onClick={() => startInlineEdit("severity")}
                data-testid="badge-severity-trigger"
              >
                <Badge className={`${sevColors[caseData.sev] || "border border-[rgba(107,114,128,0.3)] bg-[rgba(107,114,128,0.16)] text-[#d1d5db]"} min-w-[84px] justify-center font-semibold`} data-testid="badge-severity">
                  {severityLabel}
                </Badge>
              </button>
            )}
            {inlineEditingField === "priority" ? (
              <div ref={inlineEditorRef} className="rounded-lg border border-[#2a2c3c] bg-[#0f1118] p-2" data-testid="inline-editor-priority">
                <Input
                  autoFocus
                  value={inlineEditingValue}
                  onChange={(event) => setInlineEditingValue(event.target.value)}
                  className={`${CASE_INPUT_CLASS} h-8 w-28`}
                  data-testid="input-inline-priority"
                />
                {renderInlineEditorActions("priority", true)}
              </div>
            ) : (
              <button
                type="button"
                className="inline-flex cursor-pointer items-center rounded-lg transition-colors hover:bg-[#1a1f2d]"
                onClick={() => startInlineEdit("priority")}
                data-testid="badge-priority-trigger"
              >
                <Badge variant="outline" className="rounded-lg border-[#2a2c3c] bg-[#0f1118] font-semibold text-[#d1d5db]" data-testid="badge-priority">
                  {caseData.priority || "P3"}
                </Badge>
              </button>
            )}
            {inlineEditingField === "stage" ? (
              <div ref={inlineEditorRef} className="rounded-lg border border-[#2a2c3c] bg-[#0f1118] p-2" data-testid="inline-editor-stage">
                <Input
                  autoFocus
                  value={inlineEditingValue}
                  onChange={(event) => setInlineEditingValue(event.target.value)}
                  className={`${CASE_INPUT_CLASS} h-8 w-32`}
                  data-testid="input-inline-stage"
                />
                {renderInlineEditorActions("stage", true)}
              </div>
            ) : (
              <button
                type="button"
                className="inline-flex cursor-pointer items-center rounded-lg transition-colors hover:bg-[#1a1f2d]"
                onClick={() => startInlineEdit("stage")}
                data-testid="badge-stage-trigger"
              >
                <Badge variant="outline" className="rounded-lg border-[#2a2c3c] bg-[#0f1118] text-xs text-[#d1d5db]" data-testid="badge-stage">
                  {caseData.stage || t("caseDetail.stage.triage")}
                </Badge>
              </button>
            )}
            {inlineEditingField === "tlp" ? (
              <div ref={inlineEditorRef} className="rounded-lg border border-[#2a2c3c] bg-[#0f1118] p-2" data-testid="inline-editor-tlp">
                <Select value={inlineEditingValue || "TLP:AMBER"} onValueChange={setInlineEditingValue}>
                  <SelectTrigger className={`${CASE_SELECT_TRIGGER_CLASS} min-w-[140px]`} data-testid="select-inline-tlp">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent className={CASE_SELECT_CONTENT_CLASS}>
                    {TLP_VALUES.map((value) => (
                      <SelectItem key={value} value={value}>
                        {value}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {renderInlineEditorActions("tlp", true)}
              </div>
            ) : (
              <button
                type="button"
                className="inline-flex cursor-pointer items-center rounded-lg transition-opacity hover:opacity-90"
                onClick={() => startInlineEdit("tlp")}
                data-testid="badge-tlp-trigger"
              >
                <Badge className={`${tlpColors[caseData.tlp || "TLP:AMBER"]} rounded-lg text-xs font-semibold`} data-testid="badge-tlp">
                  {caseData.tlp || "TLP:AMBER"}
                </Badge>
              </button>
            )}
            {caseShares.length > 0 && (
              <div className="flex flex-wrap items-center gap-1" data-testid="case-share-badges">
                {caseShares.map((share: any, index: number) => {
                  const label = String(share?.shared_tenant_name || share?.shared_tenant_slug || share?.shared_tenant_id || "").trim();
                  if (!label) return null;
                  return (
                    <Badge key={`${label}-${index}`} variant="secondary" className="rounded-lg border border-[#2a2c3c] bg-[#0f1118] text-xs text-[#cbd5e1]" data-testid={`badge-case-shared-tenant-${index}`}>
                      Shared: {label}
                    </Badge>
                  );
                })}
              </div>
            )}
            <div className="flex-1" />
            <Button
              size="sm"
              className="group h-9 rounded-xl gap-2 border border-[rgba(59,130,246,0.34)] bg-[rgba(59,130,246,0.2)] font-semibold text-[#dbeafe] hover:bg-[rgba(59,130,246,0.28)]"
              disabled={analyzeCaseAI.isPending}
              onClick={handleAnalyzeCaseWithAI}
              data-testid="button-analyze-case-ai"
            >
              <Sparkles size={14} className={analyzeCaseAI.isPending ? "animate-pulse" : "transition-transform group-hover:rotate-12"} />
              {analyzeCaseAI.isPending ? t("caseDetail.ai.analyzing") : t("caseDetail.ai.analyze")}
            </Button>
            <Dialog open={shareDialogOpen} onOpenChange={setShareDialogOpen}>
              <DialogTrigger asChild>
                <Button
                  variant="outline"
                  size="sm"
                  className="h-9 rounded-xl gap-2 border-[#2a2c3c] bg-[#0f1118] text-[#d1d5db] hover:bg-[#1a1f2d]"
                  data-testid="button-open-share-case"
                >
                  <Link2 size={14} /> Share Case
                </Button>
              </DialogTrigger>
              <DialogContent className="max-w-xl rounded-2xl border border-[#2a2c3c] bg-[#13141c] text-[#f3f4f6]" data-testid="dialog-share-case">
                <DialogHeader>
                  <DialogTitle>Share Case As One Entity</DialogTitle>
                  <DialogDescription>
                    Teams in selected tenant will work with the same case object (not a copy).
                  </DialogDescription>
                </DialogHeader>
                <div className="space-y-4">
                  <div className="space-y-2">
                    <Label>Target Tenant</Label>
                    <Select
                      value={shareDraft.targetTenantID || "__none__"}
                      onValueChange={(value) =>
                        setShareDraft((prev) => ({
                          ...prev,
                          targetTenantID: value === "__none__" ? "" : value,
                        }))
                      }
                    >
                      <SelectTrigger className={CASE_SELECT_TRIGGER_CLASS} data-testid="select-case-share-tenant">
                        <SelectValue placeholder="Select tenant" />
                      </SelectTrigger>
                      <SelectContent className={CASE_SELECT_CONTENT_CLASS}>
                        <SelectItem value="__none__" disabled>
                          {escalationTargetTenants.length === 0 ? "No target tenants" : "Select tenant"}
                        </SelectItem>
                        {escalationTargetTenants.map((tenant: any) => (
                          <SelectItem key={tenant.id} value={tenant.id}>
                            {tenant.name || tenant.slug || tenant.id}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                </div>
                <div className="mt-2 flex justify-end gap-2">
                  <Button
                    variant="outline"
                    className="border-[#2a2c3c] bg-[#0f1118] text-[#d1d5db] hover:bg-[#1a1f2d]"
                    onClick={() => setShareDialogOpen(false)}
                    data-testid="button-case-share-cancel"
                  >
                    {t("common.cancel")}
                  </Button>
                  <Button
                    onClick={handleShareCaseAcrossTenants}
                    disabled={shareCaseAcrossTenants.isPending || escalationTargetTenants.length === 0}
                    data-testid="button-case-share-confirm"
                  >
                    {shareCaseAcrossTenants.isPending ? "Sharing..." : "Share"}
                  </Button>
                </div>
              </DialogContent>
            </Dialog>
            <Dialog open={escalateDialogOpen} onOpenChange={setEscalateDialogOpen}>
              <DialogTrigger asChild>
                <Button
                  variant="outline"
                  size="sm"
                  className="h-9 rounded-xl gap-2 border-[#2a2c3c] bg-[#0f1118] text-[#d1d5db] hover:bg-[#1a1f2d]"
                  data-testid="button-open-escalate-case"
                >
                  <ChevronDown size={14} /> Escalate
                </Button>
              </DialogTrigger>
              <DialogContent className="max-w-xl rounded-2xl border border-[#2a2c3c] bg-[#13141c] text-[#f3f4f6]" data-testid="dialog-escalate-case">
                <DialogHeader>
                  <DialogTitle>Escalate Case</DialogTitle>
                  <DialogDescription>Create linked case in another tenant and transfer context.</DialogDescription>
                </DialogHeader>
                <div className="space-y-4">
                  <div className="space-y-2">
                    <Label>Target Tenant</Label>
                    <Select
                      value={escalationDraft.targetTenantID || "__none__"}
                      onValueChange={(value) =>
                        setEscalationDraft((prev) => ({
                          ...prev,
                          targetTenantID: value === "__none__" ? "" : value,
                        }))
                      }
                    >
                      <SelectTrigger className={CASE_SELECT_TRIGGER_CLASS} data-testid="select-case-escalate-tenant">
                        <SelectValue placeholder="Select tenant" />
                      </SelectTrigger>
                      <SelectContent className={CASE_SELECT_CONTENT_CLASS}>
                        <SelectItem value="__none__" disabled>
                          {escalationTargetTenants.length === 0 ? "No target tenants" : "Select tenant"}
                        </SelectItem>
                        {escalationTargetTenants.map((tenant: any) => (
                          <SelectItem key={tenant.id} value={tenant.id}>
                            {tenant.name || tenant.slug || tenant.id}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <Label>Handoff Type</Label>
                    <Select
                      value={escalationDraft.handoffType}
                      onValueChange={(value) => setEscalationDraft((prev) => ({ ...prev, handoffType: value }))}
                    >
                      <SelectTrigger className={CASE_SELECT_TRIGGER_CLASS} data-testid="select-case-escalate-handoff-type">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent className={CASE_SELECT_CONTENT_CLASS}>
                        <SelectItem value="employee_client">Employee / Client</SelectItem>
                        <SelectItem value="employee_multi_clients">Employee / Multi Clients</SelectItem>
                        <SelectItem value="employee_without_client">Employee / No Client</SelectItem>
                        <SelectItem value="multi_employee_client">Multi Employee / Client</SelectItem>
                        <SelectItem value="custom">Custom</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <Label>Summary</Label>
                    <Textarea
                      value={escalationDraft.summary}
                      onChange={(event) =>
                        setEscalationDraft((prev) => ({
                          ...prev,
                          summary: event.target.value,
                        }))
                      }
                      className="min-h-[96px] rounded-lg border-[#2a2c3c] bg-[#0f1118] text-[#f3f4f6] placeholder:text-[#6b7280] focus-visible:ring-[#3b82f6]/40"
                      placeholder="What should receiving tenant focus on?"
                      data-testid="input-case-escalate-summary"
                    />
                  </div>
                  <label className="flex items-center gap-2 text-sm">
                    <Checkbox
                      checked={escalationDraft.includeObservables}
                      onCheckedChange={(checked) =>
                        setEscalationDraft((prev) => ({
                          ...prev,
                          includeObservables: checked !== false,
                        }))
                      }
                      data-testid="checkbox-case-escalate-include-observables"
                    />
                    <span>Copy observables to target case</span>
                  </label>
                </div>
                <div className="mt-2 flex justify-end gap-2">
                  <Button
                    variant="outline"
                    className="border-[#2a2c3c] bg-[#0f1118] text-[#d1d5db] hover:bg-[#1a1f2d]"
                    onClick={() => setEscalateDialogOpen(false)}
                    data-testid="button-case-escalate-cancel"
                  >
                    {t("common.cancel")}
                  </Button>
                  <Button
                    onClick={handleEscalateCase}
                    disabled={escalateCase.isPending || escalationTargetTenants.length === 0}
                    data-testid="button-case-escalate-confirm"
                  >
                    {escalateCase.isPending ? "Escalating..." : "Escalate"}
                  </Button>
                </div>
              </DialogContent>
            </Dialog>
            <Button
              variant="outline"
              size="sm"
              className="h-9 rounded-xl gap-2 border-[#2a2c3c] bg-[#0f1118] text-[#d1d5db] hover:bg-[#1a1f2d]"
              onClick={() => setDuplicateCaseConfirmOpen(true)}
              disabled={copyCase.isPending}
              data-testid="button-copy-case"
            >
              <Copy size={14} /> {copyCase.isPending ? "Duplicating..." : "Duplicate"}
            </Button>
            <AlertDialog open={duplicateCaseConfirmOpen} onOpenChange={setDuplicateCaseConfirmOpen}>
              <AlertDialogContent className="border-[#2a2c3c] bg-[#13141c] text-[#f3f4f6]">
                <AlertDialogHeader>
                  <AlertDialogTitle>Duplicate case?</AlertDialogTitle>
                  <AlertDialogDescription className="text-[#9ca3af]">
                    This creates a new case with the same fields and observables. Comments, timeline, communications, forum links, and AI history stay in the original case.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel className="border-[#2a2c3c] bg-[#0f1118] text-[#d1d5db] hover:bg-[#1a1f2d]">Cancel</AlertDialogCancel>
                  <AlertDialogAction
                    className="bg-[#4ed938] text-[#0b0c10] hover:bg-[#66ff4c]"
                    onClick={(event) => {
                      event.preventDefault();
                      handleDuplicateCase();
                    }}
                  >
                    {copyCase.isPending ? "Duplicating..." : "Duplicate case"}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
            <Button
              variant="outline"
              size="sm"
              className="h-9 rounded-xl gap-2 border-[#2a2c3c] bg-[#0f1118] text-[#d1d5db] hover:bg-[#1a1f2d]"
              onClick={handleOpenEditCase}
              data-testid="button-edit-case"
            >
              <Edit size={14} /> {t("common.edit")}
            </Button>
          </div>
        </div>

        <Dialog
          open={editDialogOpen}
          onOpenChange={(open) => {
            setEditDialogOpen(open);
            if (!open) {
              setCaseEditBaseUpdatedAt("");
            }
          }}
        >
          <DialogContent
            className="max-h-[90vh] max-w-4xl overflow-y-auto rounded-2xl border border-[#2a2c3c] bg-[#13141c] text-[#f3f4f6]"
            data-testid="dialog-edit-case"
          >
            <DialogHeader>
              <DialogTitle>{t("common.edit")} {caseData.caseNumber || caseData.id}</DialogTitle>
              <DialogDescription>Edit case fields across all tabs (case ID is read-only).</DialogDescription>
            </DialogHeader>
            {caseEditDraft && (
              <div className="space-y-5">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label htmlFor="case-edit-id">ID</Label>
                    <Input id="case-edit-id" value={caseEditDraft.id} readOnly className="rounded-xl font-mono text-xs" data-testid="input-case-edit-id" />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="case-edit-number">Case Number</Label>
                    <Input
                      id="case-edit-number"
                      value={caseEditDraft.caseNumber}
                      onChange={(event) => patchCaseEditDraft({ caseNumber: event.target.value })}
                      className="rounded-xl"
                      data-testid="input-case-edit-number"
                    />
                  </div>
                  <div className="space-y-2 md:col-span-2">
                    <Label htmlFor="case-edit-title">{t("alerts.field.title")}</Label>
                    <Input
                      id="case-edit-title"
                      value={caseEditDraft.title}
                      onChange={(event) => patchCaseEditDraft({ title: event.target.value })}
                      className="rounded-xl"
                      data-testid="input-case-edit-title"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="case-edit-source">{t("alerts.field.source")}</Label>
                    <Input
                      id="case-edit-source"
                      value={caseEditDraft.source}
                      onChange={(event) => patchCaseEditDraft({ source: event.target.value })}
                      className="rounded-xl"
                      data-testid="input-case-edit-source"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label>{t("caseDetail.overview.category")}</Label>
                    <Select
                      value={caseEditDraft.category || "__none__"}
                      onValueChange={(value) => patchCaseEditDraft({ category: value === "__none__" ? "" : value })}
                    >
                      <SelectTrigger className="rounded-xl" data-testid="select-case-edit-category">
                        <SelectValue placeholder={t("caseDetail.classification.categoryPlaceholder")} />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="__none__">{t("caseDetail.classification.noCategory")}</SelectItem>
                        {caseCategoryOptions.map((value) => (
                          <SelectItem key={value} value={value}>
                            {value}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <div className="flex items-center gap-2 pt-1">
                      <Input
                        value={newCaseCategoryName}
                        onChange={(event) => setNewCaseCategoryName(event.target.value)}
                        placeholder={t("caseDetail.classification.newCategoryPlaceholder")}
                        className="rounded-xl"
                        data-testid="input-case-edit-new-category"
                      />
                      <Button
                        type="button"
                        variant="outline"
                        className="rounded-xl"
                        onClick={handleCreateCustomCaseCategory}
                        disabled={createCaseCategory.isPending || !newCaseCategoryName.trim()}
                        data-testid="button-case-edit-add-category"
                      >
                        {t("caseDetail.classification.addCategory")}
                      </Button>
                    </div>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="case-edit-incident-type">{t("cases.field.incidentType")}</Label>
                    <Input
                      id="case-edit-incident-type"
                      value={caseEditDraft.incidentType}
                      onChange={(event) => patchCaseEditDraft({ incidentType: event.target.value })}
                      className="rounded-xl"
                      data-testid="input-case-edit-incident-type"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="case-edit-related-product">{t("caseDetail.classification.relatedProduct")}</Label>
                    <Input
                      id="case-edit-related-product"
                      value={caseEditDraft.relatedProduct}
                      onChange={(event) => patchCaseEditDraft({ relatedProduct: event.target.value })}
                      className="rounded-xl"
                      data-testid="input-case-edit-related-product"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label>{t("cases.field.status")}</Label>
                    <Select
                      value={caseEditDraft.statusCode}
                      onValueChange={(value) => patchCaseEditDraft({ statusCode: value })}
                    >
                      <SelectTrigger className="rounded-xl" data-testid="select-case-edit-status">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {caseStatusOptions.map((status: any) => (
                          <SelectItem key={status.code} value={status.code}>{status.label}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <Label>{t("alerts.field.severity")}</Label>
                    <Select
                      value={caseEditDraft.severity}
                      onValueChange={(value) => patchCaseEditDraft({ severity: value })}
                    >
                      <SelectTrigger className="rounded-xl" data-testid="select-case-edit-severity">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {["Critical", "High", "Medium", "Low"].map((severity) => (
                          <SelectItem key={severity} value={severity}>{severity}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="case-edit-priority">{t("alerts.field.priority")}</Label>
                    <Input
                      id="case-edit-priority"
                      value={caseEditDraft.priority}
                      onChange={(event) => patchCaseEditDraft({ priority: event.target.value })}
                      className="rounded-xl"
                      data-testid="input-case-edit-priority"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="case-edit-stage">{t("caseDetail.stage.triage")}</Label>
                    <Input
                      id="case-edit-stage"
                      value={caseEditDraft.stage}
                      onChange={(event) => patchCaseEditDraft({ stage: event.target.value })}
                      className="rounded-xl"
                      placeholder={t("caseDetail.stage.triage")}
                      data-testid="input-case-edit-stage"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label>{t("alerts.field.tlp")}</Label>
                    <Select value={caseEditDraft.tlp} onValueChange={(value) => patchCaseEditDraft({ tlp: value })}>
                      <SelectTrigger className="rounded-xl" data-testid="select-case-edit-tlp">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {TLP_VALUES.map((value) => (
                          <SelectItem key={value} value={value}>{value}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <Label>{t("alerts.field.pap")}</Label>
                    <Select value={caseEditDraft.pap} onValueChange={(value) => patchCaseEditDraft({ pap: value })}>
                      <SelectTrigger className="rounded-xl" data-testid="select-case-edit-pap">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {PAP_VALUES.map((value) => (
                          <SelectItem key={value} value={value}>{value}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <Label>{t("caseDetail.overview.assignee")}</Label>
                    <Select
                      value={caseEditDraft.assignee || "__unassigned__"}
                      onValueChange={(value) => patchCaseEditDraft({ assignee: value === "__unassigned__" ? "" : value })}
                    >
                      <SelectTrigger className="rounded-xl" data-testid="select-case-edit-assignee">
                        <SelectValue placeholder={t("caseDetail.overview.unassigned")} />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="__unassigned__">{t("caseDetail.overview.unassigned")}</SelectItem>
                        {(users || []).map((user: any) => (
                          <SelectItem key={user.id} value={user.id}>{user.name}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="case-edit-impact">{t("dashboard.impact")}</Label>
                    <Input
                      id="case-edit-impact"
                      value={caseEditDraft.impact}
                      onChange={(event) => patchCaseEditDraft({ impact: event.target.value })}
                      className="rounded-xl"
                      data-testid="input-case-edit-impact"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="case-edit-confidence">{t("cases.field.confidence")}</Label>
                    <Input
                      id="case-edit-confidence"
                      type="number"
                      min={0}
                      max={100}
                      value={caseEditDraft.confidence}
                      onChange={(event) => patchCaseEditDraft({ confidence: event.target.value })}
                      className="rounded-xl"
                      data-testid="input-case-edit-confidence"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="case-edit-detected">{t("cases.field.detectedAt")}</Label>
                    <Input
                      id="case-edit-detected"
                      type="datetime-local"
                      value={caseEditDraft.detectedAt}
                      onChange={(event) => patchCaseEditDraft({ detectedAt: event.target.value })}
                      className="rounded-xl"
                      data-testid="input-case-edit-detected-at"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="case-edit-occurred">{t("cases.field.occurredAt")}</Label>
                    <Input
                      id="case-edit-occurred"
                      type="datetime-local"
                      value={caseEditDraft.occurredAt}
                      onChange={(event) => patchCaseEditDraft({ occurredAt: event.target.value })}
                      className="rounded-xl"
                      data-testid="input-case-edit-occurred-at"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="case-edit-closed">{t("caseDetail.status.closed")}</Label>
                    <Input
                      id="case-edit-closed"
                      type="datetime-local"
                      value={caseEditDraft.closedAt}
                      onChange={(event) => patchCaseEditDraft({ closedAt: event.target.value })}
                      className="rounded-xl"
                      data-testid="input-case-edit-closed-at"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label>{t("caseDetail.observable.verdict")}</Label>
                    <Select value={caseEditDraft.verdict} onValueChange={(value) => patchCaseEditDraft({ verdict: value })}>
                      <SelectTrigger className="rounded-xl" data-testid="select-case-edit-verdict">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {VERDICTS.map((value) => (
                          <SelectItem key={value} value={value}>{t(VERDICT_KEYS[value] || value)}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2 md:col-span-2">
                    <Label htmlFor="case-edit-description">{t("cases.field.description")}</Label>
                    <Textarea
                      id="case-edit-description"
                      value={caseEditDraft.description}
                      onChange={(event) => patchCaseEditDraft({ description: event.target.value })}
                      className="rounded-xl min-h-[96px]"
                      data-testid="input-case-edit-description"
                    />
                  </div>
                  <div className="space-y-2 md:col-span-2">
                    <Label htmlFor="case-edit-resolution">Resolution Summary</Label>
                    <Textarea
                      id="case-edit-resolution"
                      value={caseEditDraft.resolutionSummary}
                      onChange={(event) => patchCaseEditDraft({ resolutionSummary: event.target.value })}
                      className="rounded-xl min-h-[84px]"
                      data-testid="input-case-edit-resolution-summary"
                    />
                  </div>
                  <div className="space-y-2 md:col-span-2">
                    <Label htmlFor="case-edit-recommendations">{t("layout.ai.recommendations")}</Label>
                    <Textarea
                      id="case-edit-recommendations"
                      value={caseEditDraft.recommendations}
                      onChange={(event) => patchCaseEditDraft({ recommendations: event.target.value })}
                      className="rounded-xl min-h-[84px]"
                      placeholder="One recommendation per line"
                      data-testid="input-case-edit-recommendations"
                    />
                  </div>
                </div>
                <div className="flex justify-end gap-2">
                  <Button
                    variant="outline"
                    onClick={() => {
                      setEditDialogOpen(false);
                      setCaseEditBaseUpdatedAt("");
                    }}
                    data-testid="button-case-edit-cancel"
                  >
                    {t("common.cancel")}
                  </Button>
                  <Button onClick={handleSaveCase} disabled={updateCase.isPending} data-testid="button-case-edit-save">
                    {updateCase.isPending ? "Saving..." : t("common.save")}
                  </Button>
                </div>
              </div>
            )}
          </DialogContent>
        </Dialog>

        {/* Tabs */}
        <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
          <div className="overflow-x-auto rounded-xl border border-[#2a2c3c] bg-[#10121a]" data-testid="tabs-navigation">
            <TabsList className="h-auto w-max min-w-full justify-start overflow-visible rounded-none bg-transparent p-0">
              <TabsTrigger value="overview" className={CASE_TAB_TRIGGER_CLASS} data-testid="tab-overview">{t("caseDetail.tab.overview")}</TabsTrigger>
              <TabsTrigger value="timeline" className={CASE_TAB_TRIGGER_CLASS} data-testid="tab-timeline">{t("caseDetail.tab.timeline")}</TabsTrigger>
              <TabsTrigger value="tasks" className={CASE_TAB_TRIGGER_CLASS} data-testid="tab-tasks">{t("caseDetail.tab.tasks")}</TabsTrigger>
              <TabsTrigger value="observables" className={CASE_TAB_TRIGGER_CLASS} data-testid="tab-observables">{t("caseDetail.tab.observables")}</TabsTrigger>
              <TabsTrigger value="visuals" className={CASE_TAB_TRIGGER_CLASS} data-testid="tab-visuals">{t("caseDetail.tab.visuals")}</TabsTrigger>
              <TabsTrigger value="attachments" className={CASE_TAB_TRIGGER_CLASS} data-testid="tab-attachments">{t("caseDetail.tab.attachments")}</TabsTrigger>
              <TabsTrigger value="comments" className={CASE_TAB_TRIGGER_CLASS} data-testid="tab-comments">{t("caseDetail.tab.comments")}</TabsTrigger>
              <TabsTrigger value="ai" className={CASE_TAB_TRIGGER_CLASS} data-testid="tab-ai">{t("caseDetail.tab.ai")}</TabsTrigger>
              <TabsTrigger value="mitre" className={CASE_TAB_TRIGGER_CLASS} data-testid="tab-mitre">{t("caseDetail.tab.mitre")}</TabsTrigger>
              <TabsTrigger value="connectors" className={CASE_TAB_TRIGGER_CLASS} data-testid="tab-connectors">{t("caseDetail.tab.connectors")}</TabsTrigger>
              <TabsTrigger value="communications" className={`${CASE_TAB_TRIGGER_CLASS} gap-1.5`} data-testid="tab-communications">
                <MessageSquare size={13} /> {t("caseDetail.tab.communications")}
              </TabsTrigger>
            </TabsList>
          </div>

          {/* Overview Tab */}
          <TabsContent value="overview" className="mt-6 space-y-6" data-testid="tab-content-overview">
            <CaseOverviewSummarySection
              t={t}
              panelClass={CASE_PANEL_CLASS}
              taskCompletion={taskCompletion}
              observableCount={observableCount}
              commentCount={commentCount}
              relatedLinkBy={relatedLinkBy}
              setRelatedLinkBy={setRelatedLinkBy}
              parseRelatedCasesLinkBy={parseRelatedCasesLinkBy}
              relatedCasesLinkByOptions={RELATED_CASES_LINK_BY_OPTIONS}
              relatedActiveRecentCases={relatedActiveRecentCases}
              relatedAllTimeCases={relatedAllTimeCases}
              currentTenantSlug={currentTenantSlug}
              renderRelatedMatchedSummary={renderRelatedMatchedSummary}
            />

            <Card className={CASE_PANEL_CLASS} data-testid="card-closure-approvals">
              <CardHeader className="pb-3">
                <CardTitle className="text-sm font-semibold uppercase tracking-[0.12em] text-[#9ca3af] flex items-center gap-2">
                  <CheckCircle2 size={16} className="text-primary" /> {t("caseDetail.closureApproval.title")}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <Badge variant="outline" className="rounded-lg text-[11px]">
                    {t("caseDetail.closureApproval.progress", {
                      approved: String(closureApprovalsByUser.size),
                      required: String(requiredClosureApprovals),
                    })}
                  </Badge>
                  <Button
                    type="button"
                    size="sm"
                    variant={hasCurrentUserClosureApproval ? "secondary" : "default"}
                    className="h-8 rounded-lg"
                    disabled={createTimelineEvent.isPending || hasCurrentUserClosureApproval}
                    onClick={handleApproveCaseClosure}
                    data-testid="button-approve-closure"
                  >
                    {hasCurrentUserClosureApproval ? t("caseDetail.closureApproval.approved") : t("caseDetail.closureApproval.approve")}
                  </Button>
                </div>
                {requiredClosureApprovals > 0 ? (
                  <p className="text-xs text-[#9ca3af]">
                    {t("caseDetail.closureApproval.requiredHint", { count: String(requiredClosureApprovals) })}
                  </p>
                ) : (
                  <p className="text-xs text-[#9ca3af]">{t("caseDetail.closureApproval.optionalHint")}</p>
                )}
                {requiredClosureApproverIDs.length > 0 ? (
                  <div className="flex flex-wrap gap-2" data-testid="closure-required-approvers">
                    {requiredClosureApproverIDs.map((userID) => {
                      const user = usersByID.get(userID);
                      const approved = closureApprovalsByUser.has(userID);
                      return (
                        <Badge
                          key={userID}
                          variant={approved ? "default" : "outline"}
                          className="rounded-lg text-[11px]"
                          data-testid={`closure-required-approver-${userID}`}
                        >
                          {user?.name || user?.username || user?.email || userID}
                          {approved ? ` • ${t("caseDetail.closureApproval.approvedShort")}` : ""}
                        </Badge>
                      );
                    })}
                  </div>
                ) : null}
                {missingRequiredClosureApprovers.length > 0 ? (
                  <p className="text-xs text-amber-700" data-testid="closure-missing-approvals">
                    {t("caseDetail.closureApproval.missing", { count: String(missingRequiredClosureApprovers.length) })}
                  </p>
                ) : requiredClosureApprovals > 0 ? (
                  <p className="text-xs text-emerald-700" data-testid="closure-ready">
                    {t("caseDetail.closureApproval.ready")}
                  </p>
                ) : null}
              </CardContent>
            </Card>

            <div className="space-y-6">
              <Card className={CASE_PANEL_CLASS} data-testid="card-details">
                <CardHeader className="pb-3">
                  <CardTitle className="text-sm font-semibold uppercase tracking-[0.12em] text-[#9ca3af] flex items-center gap-2">
                    <Shield size={16} className="text-primary" /> {t("caseDetail.overview.caseDetails")}
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-2">
                  <div className="grid grid-cols-1 xl:grid-cols-2 gap-2">
                    <div
                      className={`group flex items-center justify-between gap-3 rounded-xl border px-3 py-2 text-sm ${
                        inlineEditingField === "incidentType" ? "border-[#2a2c3c] bg-[#0f1118]" : "border-transparent hover:border-[#2a2c3c] hover:bg-[#1a1f2d]"
                      }`}
                      data-testid="detail-row-type"
                    >
                      <span className="text-[#9ca3af]">{t("caseDetail.overview.type")}</span>
                      {inlineEditingField === "incidentType" ? (
                        <div
                          ref={inlineEditorRef}
                          className="w-full max-w-[280px]"
                          onClick={(event) => event.stopPropagation()}
                          data-testid="inline-editor-incident-type"
                        >
                          <Input
                            autoFocus
                            value={inlineEditingValue}
                            onChange={(event) => setInlineEditingValue(event.target.value)}
                            className="h-8 rounded-md border-[#2a2c3c] bg-[#0f1118] text-right text-[#f3f4f6]"
                            data-testid="input-inline-incident-type"
                          />
                          {renderInlineEditorActions("incidentType", true)}
                        </div>
                      ) : (
                        <div className="flex items-center gap-2 min-w-0">
                          <span className="font-medium text-right" data-testid="detail-type">
                            {caseData.incidentType || t("caseDetail.overview.incident")}
                          </span>
                          {renderInlineEditButton("incidentType")}
                        </div>
                      )}
                    </div>

                    <div
                      className={`group flex items-center justify-between gap-3 rounded-xl border px-3 py-2 text-sm ${
                        inlineEditingField === "category" ? "border-[#2a2c3c] bg-[#0f1118]" : "border-transparent hover:border-[#2a2c3c] hover:bg-[#1a1f2d]"
                      }`}
                      data-testid="detail-row-category"
                    >
                      <span className="text-[#9ca3af]">{t("caseDetail.overview.category")}</span>
                      {inlineEditingField === "category" ? (
                        <div
                          ref={inlineEditorRef}
                          className="w-full max-w-[280px]"
                          onClick={(event) => event.stopPropagation()}
                          data-testid="inline-editor-category"
                        >
                          <Input
                            autoFocus
                            value={inlineEditingValue}
                            onChange={(event) => setInlineEditingValue(event.target.value)}
                            className="h-8 rounded-md border-[#2a2c3c] bg-[#0f1118] text-right text-[#f3f4f6]"
                            data-testid="input-inline-category"
                          />
                          {renderInlineEditorActions("category", true)}
                        </div>
                      ) : (
                        <div className="flex items-center gap-2 min-w-0">
                          <span className="font-medium text-right" data-testid="detail-category">
                            {caseData.category || t("caseDetail.classification.noCategory")}
                          </span>
                          {renderInlineEditButton("category")}
                        </div>
                      )}
                    </div>

                    <div
                      className={`group flex items-center justify-between gap-3 rounded-xl border px-3 py-2 text-sm ${
                        inlineEditingField === "relatedProduct" ? "border-[#2a2c3c] bg-[#0f1118]" : "border-transparent hover:border-[#2a2c3c] hover:bg-[#1a1f2d]"
                      }`}
                      data-testid="detail-row-related-product"
                    >
                      <span className="text-[#9ca3af]">{t("caseDetail.classification.relatedProduct")}</span>
                      {inlineEditingField === "relatedProduct" ? (
                        <div
                          ref={inlineEditorRef}
                          className="w-full max-w-[280px]"
                          onClick={(event) => event.stopPropagation()}
                          data-testid="inline-editor-related-product"
                        >
                          <Input
                            autoFocus
                            value={inlineEditingValue}
                            onChange={(event) => setInlineEditingValue(event.target.value)}
                            className="h-8 rounded-md border-[#2a2c3c] bg-[#0f1118] text-right text-[#f3f4f6]"
                            data-testid="input-inline-related-product"
                          />
                          {renderInlineEditorActions("relatedProduct", true)}
                        </div>
                      ) : (
                        <div className="flex items-center gap-2 min-w-0">
                          <span className="font-medium text-right" data-testid="detail-related-product">
                            {caseData.relatedProduct || "—"}
                          </span>
                          {renderInlineEditButton("relatedProduct")}
                        </div>
                      )}
                    </div>

                    <div
                      className={`group flex items-center justify-between gap-3 rounded-xl border px-3 py-2 text-sm ${
                        inlineEditingField === "impact" ? "border-[#2a2c3c] bg-[#0f1118]" : "border-transparent hover:border-[#2a2c3c] hover:bg-[#1a1f2d]"
                      }`}
                      data-testid="detail-row-impact"
                    >
                      <span className="text-[#9ca3af]">{t("caseDetail.overview.riskScore")}</span>
                      {inlineEditingField === "impact" ? (
                        <div
                          ref={inlineEditorRef}
                          className="w-full max-w-[280px]"
                          onClick={(event) => event.stopPropagation()}
                          data-testid="inline-editor-impact"
                        >
                          <Input
                            autoFocus
                            value={inlineEditingValue}
                            onChange={(event) => setInlineEditingValue(event.target.value)}
                            className="h-8 rounded-md border-[#2a2c3c] bg-[#0f1118] text-right text-[#f3f4f6]"
                            data-testid="input-inline-impact"
                          />
                          {renderInlineEditorActions("impact", true)}
                        </div>
                      ) : (
                        <div className="flex items-center gap-2 min-w-0">
                          <span className="font-medium text-right" data-testid="detail-risk-score">
                            {caseData.impact || "—"}
                          </span>
                          {renderInlineEditButton("impact")}
                        </div>
                      )}
                    </div>

                    <div
                      className={`group flex items-center justify-between gap-3 rounded-xl border px-3 py-2 text-sm ${
                        inlineEditingField === "assignee" ? "border-[#2a2c3c] bg-[#0f1118]" : "border-transparent hover:border-[#2a2c3c] hover:bg-[#1a1f2d]"
                      }`}
                      data-testid="detail-row-assignee"
                    >
                      <span className="text-[#9ca3af]">{t("caseDetail.overview.assignee")}</span>
                      {inlineEditingField === "assignee" ? (
                        <div
                          ref={inlineEditorRef}
                          className="w-full max-w-[280px]"
                          onClick={(event) => event.stopPropagation()}
                          data-testid="inline-editor-assignee"
                        >
                          <Select value={inlineEditingValue || "__unassigned__"} onValueChange={(value) => setInlineEditingValue(value === "__unassigned__" ? "" : value)}>
                            <SelectTrigger className="h-8 rounded-md border-[#2a2c3c] bg-[#0f1118] text-[#f3f4f6]" data-testid="select-inline-assignee">
                              <SelectValue placeholder={t("caseDetail.overview.unassigned")} />
                            </SelectTrigger>
                            <SelectContent className={CASE_SELECT_CONTENT_CLASS}>
                              <SelectItem value="__unassigned__">{t("caseDetail.overview.unassigned")}</SelectItem>
                              {(users || []).map((user: any) => (
                                <SelectItem key={user.id} value={user.id}>
                                  {user.name}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                          {renderInlineEditorActions("assignee", true)}
                        </div>
                      ) : (
                        <div className="flex items-center gap-2 min-w-0">
                          {caseData.assignee ? (
                            <UserAvatar
                              name={assigneeUser?.name || caseData.assignee}
                              avatar={assigneeUser?.avatar}
                              fallback={caseData.assignee}
                              className="h-6 w-6 border border-[#2a2c3c]"
                              fallbackClassName="text-[10px] font-bold"
                            />
                          ) : null}
                          <span className="font-medium text-right" data-testid="detail-assignee">
                            {assigneeUser?.name || caseData.assignee || t("caseDetail.overview.unassigned")}
                          </span>
                          {renderInlineEditButton("assignee")}
                        </div>
                      )}
                    </div>

                    <div
                      className={`group flex items-center justify-between gap-3 rounded-xl border px-3 py-2 text-sm ${
                        inlineEditingField === "confidence" ? "border-[#2a2c3c] bg-[#0f1118]" : "border-transparent hover:border-[#2a2c3c] hover:bg-[#1a1f2d]"
                      }`}
                      data-testid="detail-row-confidence"
                    >
                      <span className="text-[#9ca3af]">{t("cases.field.confidence")}</span>
                      {inlineEditingField === "confidence" ? (
                        <div
                          ref={inlineEditorRef}
                          className="w-full max-w-[180px]"
                          onClick={(event) => event.stopPropagation()}
                          data-testid="inline-editor-confidence"
                        >
                          <Input
                            autoFocus
                            type="number"
                            min={0}
                            max={100}
                            value={inlineEditingValue}
                            onChange={(event) => setInlineEditingValue(event.target.value)}
                            className="h-8 rounded-md border-[#2a2c3c] bg-[#0f1118] text-right text-[#f3f4f6]"
                            data-testid="input-inline-confidence"
                          />
                          {renderInlineEditorActions("confidence", true)}
                        </div>
                      ) : (
                        <div className="flex items-center gap-2 min-w-0">
                          <span className="font-medium text-right" data-testid="detail-confidence">
                            {`${caseData.confidence ?? 0}`}
                          </span>
                          {renderInlineEditButton("confidence")}
                        </div>
                      )}
                    </div>

                    <div
                      className={`group flex items-center justify-between gap-3 rounded-xl border px-3 py-2 text-sm ${
                        inlineEditingField === "detectedAt" ? "border-[#2a2c3c] bg-[#0f1118]" : "border-transparent hover:border-[#2a2c3c] hover:bg-[#1a1f2d]"
                      }`}
                      data-testid="detail-row-detected"
                    >
                      <span className="text-[#9ca3af]">{t("caseDetail.overview.detected")}</span>
                      {inlineEditingField === "detectedAt" ? (
                        <div
                          ref={inlineEditorRef}
                          className="w-full max-w-[280px]"
                          onClick={(event) => event.stopPropagation()}
                          data-testid="inline-editor-detected-at"
                        >
                          <Input
                            autoFocus
                            type="datetime-local"
                            value={inlineEditingValue}
                            onChange={(event) => setInlineEditingValue(event.target.value)}
                            className="h-8 rounded-md border-[#2a2c3c] bg-[#0f1118] text-[#f3f4f6]"
                            data-testid="input-inline-detected-at"
                          />
                          {renderInlineEditorActions("detectedAt", true)}
                        </div>
                      ) : (
                        <div className="flex items-center gap-2 min-w-0">
                          <span className="font-medium text-right" data-testid="detail-detected">
                            {caseData.detectedAt ? format(new Date(caseData.detectedAt), "dd.MM.yyyy HH:mm") : "—"}
                          </span>
                          {renderInlineEditButton("detectedAt")}
                        </div>
                      )}
                    </div>

                    <div
                      className="col-span-1 xl:col-span-2 rounded-xl border border-transparent bg-transparent px-3 py-2 text-sm"
                      data-testid="detail-row-owner-tags"
                    >
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-3 md:gap-6 items-start">
                        <div className="group flex items-start justify-between gap-3" data-testid="detail-row-owner">
                          <span className="text-[#9ca3af]">{t("caseDetail.overview.owner")}</span>
                          {inlineEditingField === "owner" ? (
                            <div
                              ref={inlineEditorRef}
                              className="w-full max-w-[280px]"
                              onClick={(event) => event.stopPropagation()}
                              data-testid="inline-editor-owner"
                            >
                              <Select value={inlineEditingValue || "__unassigned__"} onValueChange={(value) => setInlineEditingValue(value === "__unassigned__" ? "" : value)}>
                                <SelectTrigger className="h-8 rounded-md border-[#2a2c3c] bg-[#0f1118] text-[#f3f4f6]" data-testid="select-inline-owner">
                                  <SelectValue placeholder={t("caseDetail.overview.unassigned")} />
                                </SelectTrigger>
                                <SelectContent className={CASE_SELECT_CONTENT_CLASS}>
                                  <SelectItem value="__unassigned__">{t("caseDetail.overview.unassigned")}</SelectItem>
                                  {(users || []).map((user: any) => (
                                    <SelectItem key={user.id} value={user.id}>
                                      {user.name}
                                    </SelectItem>
                                  ))}
                                </SelectContent>
                              </Select>
                              {renderInlineEditorActions("owner", true)}
                            </div>
                          ) : (
                            <div className="flex flex-1 min-w-0 items-center justify-end gap-2">
                              {caseData.owner ? (
                                <UserAvatar
                                  name={ownerUser?.name || caseData.owner}
                                  avatar={ownerUser?.avatar}
                                  fallback={caseData.owner}
                                  className="h-6 w-6 border border-[#2a2c3c]"
                                  fallbackClassName="text-[10px] font-bold"
                                />
                              ) : null}
                              <span className="font-medium text-right" data-testid="detail-owner">
                                {ownerUser?.name || caseData.owner || t("caseDetail.overview.unassigned")}
                              </span>
                              {renderInlineEditButton("owner")}
                            </div>
                          )}
                        </div>

                        <div className="flex items-start justify-between gap-3" data-testid="detail-row-tags">
                          <span className="text-[#9ca3af]">{t("alerts.filter.tags")}</span>
                          {inlineEditingField === "tags" ? (
                            <div
                              ref={inlineEditorRef}
                              className="w-full max-w-[420px]"
                              onClick={(event) => event.stopPropagation()}
                              data-testid="inline-editor-tags"
                            >
                              <Input
                                autoFocus
                                value={inlineEditingValue}
                                onChange={(event) => setInlineEditingValue(event.target.value)}
                                placeholder={t("caseDetail.observable.tagsPlaceholder")}
                                className="h-8 rounded-md border-[#2a2c3c] bg-[#0f1118] text-right text-[#f3f4f6]"
                                data-testid="input-inline-tags"
                              />
                              {renderInlineEditorActions("tags", true)}
                            </div>
                          ) : (
                            <div className="group flex min-h-6 flex-1 items-start justify-end gap-2 rounded-lg px-1 py-0.5 hover:bg-[#1a1f2d]">
                              {caseData.tags && caseData.tags.length > 0 ? (
                                <div className="flex flex-wrap justify-end gap-1">
                                  {caseData.tags.map((tag: string) => (
                                    <Badge
                                      key={tag}
                                      variant="secondary"
                                      className="rounded-lg border border-[rgba(59,130,246,0.3)] bg-[rgba(59,130,246,0.12)] px-2 py-0.5 text-xs text-[#93c5fd]"
                                      data-testid={`badge-tag-${tag}`}
                                    >
                                      {tag}
                                    </Badge>
                                  ))}
                                </div>
                              ) : (
                                <span className="font-medium text-right">—</span>
                              )}
                              <div className="mt-0.5">
                                {renderInlineEditButton("tags")}
                              </div>
                            </div>
                          )}
                        </div>
                      </div>
                    </div>

                    <div
                      className="col-span-1 xl:col-span-2 flex items-center justify-between gap-3 rounded-xl border border-transparent px-3 py-2 text-sm"
                      data-testid="detail-row-created"
                    >
                      <span className="text-[#9ca3af]">{t("caseDetail.overview.created")}</span>
                      <span className="font-medium tabular-nums text-right pr-8" data-testid="detail-created">
                        {caseData.time ? format(new Date(caseData.time), "dd.MM.yyyy HH:mm") : "—"}
                      </span>
                    </div>
                  </div>
                </CardContent>
              </Card>

              <Card className={CASE_PANEL_CLASS} data-testid="card-custom-fields">
                <CardHeader className="pb-3">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <CardTitle className="text-sm font-semibold uppercase tracking-[0.12em] text-[#9ca3af] flex items-center gap-2">
                      <FileText size={16} className="text-primary" /> {t("caseDetail.customFields.title")}
                    </CardTitle>
                    <div className="flex items-center gap-2">
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        className="h-8 rounded-lg text-xs"
                        onClick={handleAddSOARCustomFieldRows}
                        data-testid="button-add-soar-custom-fields"
                      >
                        <Layers size={12} className="mr-1" />
                        {t("caseDetail.customFields.addSoar")}
                      </Button>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        className="h-8 rounded-lg text-xs"
                        onClick={handleAddCustomFieldRow}
                        data-testid="button-add-custom-field"
                      >
                        <Plus size={12} className="mr-1" />
                        {t("caseDetail.customFields.add")}
                      </Button>
                    </div>
                  </div>
                </CardHeader>
                <CardContent className="space-y-3">
                  {customFieldRows.length === 0 ? (
                    <div className="rounded-xl border border-dashed px-3 py-5 text-sm text-[#9ca3af]" data-testid="custom-fields-empty">
                      {t("caseDetail.customFields.empty")}
                    </div>
                  ) : (
                    <div className="space-y-2" data-testid="custom-fields-list">
                      {customFieldRows.map((row, index) => {
                        const normalizedKey = normalizeSOARCaseFieldKey(row.key);
                        const isSOARField = isSOARCaseFieldKey(normalizedKey);
                        return (
                          <div
                            key={row.id}
                            className="grid grid-cols-1 gap-2 md:grid-cols-[minmax(0,220px)_minmax(0,1fr)_auto] md:items-center"
                            data-testid={`custom-field-row-${index}`}
                          >
                            <Input
                              value={row.key}
                              onChange={(event) => handleUpdateCustomFieldRow(row.id, { key: event.target.value })}
                              placeholder={t("caseDetail.customFields.keyPlaceholder")}
                              className={CASE_INPUT_CLASS}
                              data-testid={`input-custom-field-key-${index}`}
                            />
                            <Input
                              value={row.value}
                              onChange={(event) => handleUpdateCustomFieldRow(row.id, { value: event.target.value })}
                              placeholder={isSOARField ? t(`soarCaseField.${normalizedKey}.placeholder`) : t("caseDetail.customFields.valuePlaceholder")}
                              className={CASE_INPUT_CLASS}
                              data-testid={`input-custom-field-value-${index}`}
                            />
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon"
                              className="h-8 w-8 rounded-full text-[#9ca3af] hover:bg-[rgba(244,63,94,0.16)] hover:text-[#fda4af]"
                              onClick={() => handleRemoveCustomFieldRow(row.id)}
                              data-testid={`button-remove-custom-field-${index}`}
                            >
                              <Trash2 size={14} />
                            </Button>
                          </div>
                        );
                      })}
                    </div>
                  )}
                  <div className="flex justify-end">
                    <Button
                      type="button"
                      className="rounded-lg"
                      disabled={!customFieldsDirty || updateCase.isPending}
                      onClick={handleSaveCustomFields}
                      data-testid="button-save-custom-fields"
                    >
                      {updateCase.isPending ? t("common.loading") : t("caseDetail.customFields.save")}
                    </Button>
                  </div>
                </CardContent>
              </Card>

              <Card className={CASE_PANEL_CLASS} data-testid="card-ai-case-verdict">
                <CardHeader>
                  <div className="flex items-center justify-between gap-3">
                    <CardTitle className="text-sm font-semibold uppercase tracking-[0.12em] text-[#9ca3af] flex items-center gap-2">
                      <Sparkles size={16} className="text-primary" /> {t("caseDetail.ai.verdict")}
                    </CardTitle>
                    <Button
                      variant="outline"
                      size="sm"
                      className="rounded-lg h-8 text-xs"
                      disabled={analyzeCaseAI.isPending}
                      onClick={handleAnalyzeCaseWithAI}
                      data-testid="button-rerun-case-ai"
                    >
                      {analyzeCaseAI.isPending ? t("caseDetail.ai.analyzing") : t("caseDetail.ai.rerun")}
                    </Button>
                  </div>
                </CardHeader>
                <CardContent className="space-y-4">
                  {!latestCaseAIAnalysis ? (
                    <div className="rounded-xl border border-dashed p-4 text-sm text-[#9ca3af]" data-testid="case-ai-empty">
                      {t("caseDetail.ai.empty")}
                    </div>
                  ) : (
                    <>
                      <div className="flex flex-wrap items-center gap-2">
                        <Badge className={`${getAIVerdictBadgeClass(latestCaseAIAnalysis.verdict)} text-xs rounded-lg border`}>
                          {latestCaseAIAnalysis.verdict}
                        </Badge>
                        <Badge className={`${getAIStatusBadgeClass(latestCaseAIAnalysis.status)} text-xs rounded-lg border`}>
                          {latestCaseAIAnalysis.status}
                        </Badge>
                        <Badge variant="outline" className="rounded-lg text-xs">
                          {t("caseDetail.ai.confidence")}: {latestCaseAIAnalysis.confidence.toFixed(2)}%
                        </Badge>
                      </div>
                      <div className="text-xs text-[#9ca3af]" data-testid="case-ai-analysis-time">
                        {t("caseDetail.ai.lastRun")}: {formatAnalysisDate(latestCaseAIAnalysis.createdAt)} · {t("caseDetail.ai.model")}: {latestCaseAIAnalysis.model || "n/a"}
                      </div>
                      <p className="text-sm leading-relaxed" data-testid="case-ai-summary">
                        {latestCaseAIAnalysis.summary || t("caseDetail.ai.noSummary")}
                      </p>
                      {latestCaseAIAnalysis.errorMessage && (
                        <div className="rounded-lg border border-[rgba(244,63,94,0.28)] bg-[rgba(244,63,94,0.16)] p-3 text-xs text-[#fda4af]" data-testid="case-ai-error">
                          {latestCaseAIAnalysis.errorMessage}
                        </div>
                      )}
                      {latestCaseAIAnalysis.recommendations?.length > 0 && (
                        <div>
                          <div className="text-xs font-bold uppercase tracking-wider text-[#9ca3af] mb-2">{t("layout.ai.recommendations")}</div>
                          <ul className="space-y-1 text-xs text-[#d1d5db]">
                            {latestCaseAIAnalysis.recommendations.slice(0, 4).map((item: string, idx: number) => (
                              <li key={`${item}-${idx}`} className="leading-relaxed">• {item}</li>
                            ))}
                          </ul>
                        </div>
                      )}
                    </>
                  )}

                  <Separator />
                  <div>
                    <div className="text-xs font-bold uppercase tracking-wider text-[#9ca3af] mb-2">
                      {t("caseDetail.ai.history")} ({caseAIAnalyses.length})
                    </div>
                    {caseAIAnalyses.length === 0 ? (
                      <p className="text-xs text-[#9ca3af]">{t("caseDetail.ai.historyEmpty")}</p>
                    ) : (
                      <div className="space-y-2 max-h-48 overflow-y-auto pr-1" data-testid="case-ai-history">
                        {caseAIAnalyses.map((analysis: any, idx: number) => (
                          <div key={analysis.id || idx} className="rounded-lg border p-3">
                            <div className="flex flex-wrap items-center gap-2 mb-1">
                              <span className="text-[10px] uppercase tracking-wider text-[#9ca3af]">{t("caseDetail.ai.run")} #{caseAIAnalyses.length - idx}</span>
                              <Badge className={`${getAIVerdictBadgeClass(analysis.verdict)} text-[10px] rounded-md border`}>
                                {analysis.verdict}
                              </Badge>
                              <Badge className={`${getAIStatusBadgeClass(analysis.status)} text-[10px] rounded-md border`}>
                                {analysis.status}
                              </Badge>
                            </div>
                            <div className="text-[11px] text-[#9ca3af] mb-1">
                              {formatAnalysisDate(analysis.createdAt)} · {analysis.confidence.toFixed(2)}%
                            </div>
                            <div className="text-xs leading-relaxed text-[#d1d5db] line-clamp-2">
                              {analysis.summary || t("caseDetail.ai.noSummary")}
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                </CardContent>
              </Card>

              <Card className={`group ${CASE_PANEL_CLASS}`} data-testid="card-resolution-summary">
                <CardHeader>
                  <div className="flex items-start justify-between gap-3">
                    <CardTitle className="text-sm font-semibold uppercase tracking-[0.12em] text-[#9ca3af] flex items-center gap-2">
                      <BarChart3 size={16} className="text-primary" /> Resolution Summary
                    </CardTitle>
                    {inlineEditingField !== "resolutionSummary" && renderInlineEditButton("resolutionSummary", undefined, "md")}
                  </div>
                </CardHeader>
                <CardContent>
                  {inlineEditingField === "resolutionSummary" ? (
                    <div ref={inlineEditorRef} data-testid="inline-editor-resolution-summary">
                      <Textarea
                        autoFocus
                        value={inlineEditingValue}
                        onChange={(event) => setInlineEditingValue(event.target.value)}
                        className="min-h-[100px] rounded-xl"
                        data-testid="input-inline-resolution-summary"
                      />
                      {renderInlineEditorActions("resolutionSummary")}
                    </div>
                  ) : (
                    <div
                      className="group rounded-xl border border-transparent p-2 text-left transition-colors hover:border-[#2a2c3c] hover:bg-[#1a1f2d]"
                      data-testid="text-resolution-summary-trigger"
                    >
                      <p className="text-sm leading-relaxed text-[#d1d5db] whitespace-pre-wrap break-words" data-testid="text-resolution-summary">
                        {caseData.resolutionSummary || "—"}
                      </p>
                    </div>
                  )}
                </CardContent>
              </Card>

              {caseData.summary && (
                <Card className={CASE_PANEL_CLASS} data-testid="card-summary">
                  <CardHeader>
                    <CardTitle className="text-sm font-semibold uppercase tracking-[0.12em] text-[#9ca3af] flex items-center gap-2">
                      <BarChart3 size={16} className="text-primary" /> {t("caseDetail.summary")}
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    <p className="text-sm leading-relaxed text-[#d1d5db]" data-testid="text-summary">
                      {caseData.summary}
                    </p>
                  </CardContent>
                </Card>
              )}

              <CaseOverviewCollaborationSection
                t={t}
                panelClass={CASE_PANEL_CLASS}
                forumThread={forumThread}
                resolvedForumId={resolvedForumId}
                currentTenantSlug={currentTenantSlug}
                handleCreateForum={handleCreateForum}
                inlineEditingField={inlineEditingField}
                descriptionViewMode={descriptionViewMode}
                setDescriptionViewMode={setDescriptionViewMode}
                renderInlineEditButton={renderInlineEditButton}
                inlineEditorRef={inlineEditorRef}
                inlineEditingValue={inlineEditingValue}
                setInlineEditingValue={setInlineEditingValue}
                renderInlineEditorActions={renderInlineEditorActions}
                caseData={caseData}
                renderedDescriptionHTML={renderedDescriptionHTML}
              />
            </div>
          </TabsContent>

          {/* Timeline Tab */}
          <TabsContent value="timeline" className="mt-6 space-y-6" data-testid="tab-content-timeline">
            <Card className={CASE_PANEL_CLASS}>
              <CardHeader>
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <CardTitle className="text-sm font-semibold uppercase tracking-[0.12em] text-[#9ca3af] flex items-center gap-2">
                    <Clock size={16} className="text-primary" /> {t("caseDetail.timeline.title")}
                  </CardTitle>
                  <div className="flex items-center gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      className="h-8 rounded-lg gap-1 border-[#2a2c3c] bg-[#0f1118] text-xs text-[#d1d5db] hover:bg-[#1a1f2d]"
                      onClick={() => handleExportCase("json")}
                      data-testid="button-export-case-json"
                    >
                      <FileDown size={12} /> JSON
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      className="h-8 rounded-lg gap-1 border-[#2a2c3c] bg-[#0f1118] text-xs text-[#d1d5db] hover:bg-[#1a1f2d]"
                      onClick={() => handleExportCase("csv")}
                      data-testid="button-export-case-csv"
                    >
                      <FileDown size={12} /> CSV
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      className="h-8 rounded-lg gap-1 border-[#2a2c3c] bg-[#0f1118] text-xs text-[#d1d5db] hover:bg-[#1a1f2d]"
                      onClick={() => handleExportCase("md")}
                      data-testid="button-export-case-md"
                    >
                      <FileDown size={12} /> MD
                    </Button>
                  </div>
                </div>
              </CardHeader>
              <CardContent className="space-y-6">
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
                  <Card className={CASE_SUBPANEL_CLASS}>
                    <CardContent className="pt-4">
                      <div className="text-xs text-[#9ca3af]">{t("caseDetail.timeline.metrics.totalActions")}</div>
                      <div className="mt-1 text-2xl font-bold" data-testid="timeline-metric-total-actions">{timelineMetrics.totalActions}</div>
                    </CardContent>
                  </Card>
                  <Card className={CASE_SUBPANEL_CLASS}>
                    <CardContent className="pt-4">
                      <div className="text-xs text-[#9ca3af]">{t("caseDetail.timeline.metrics.avgActionTime")}</div>
                      <div className="mt-1 text-2xl font-bold" data-testid="timeline-metric-avg-duration">
                        {formatDurationCompact(timelineMetrics.avgActionDurationMs)}
                      </div>
                    </CardContent>
                  </Card>
                  <Card className={CASE_SUBPANEL_CLASS}>
                    <CardContent className="pt-4">
                      <div className="text-xs text-[#9ca3af]">{t("caseDetail.timeline.metrics.successRate")}</div>
                      <div className="mt-1 text-2xl font-bold" data-testid="timeline-metric-success-rate">
                        {timelineMetrics.successRate}%
                      </div>
                    </CardContent>
                  </Card>
                  <Card className={CASE_SUBPANEL_CLASS}>
                    <CardContent className="pt-4">
                      <div className="text-xs text-[#9ca3af]">{t("caseDetail.timeline.metrics.taskOutcome")}</div>
                      <div className="mt-1 text-sm font-semibold" data-testid="timeline-metric-task-outcome">
                        {doneTasks} {t("caseDetail.task.status.done")} / {cancelledTasks} {t("caseDetail.task.status.cancelled")}
                      </div>
                    </CardContent>
                  </Card>
                </div>

                <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
                  <Card className={CASE_SUBPANEL_CLASS} data-testid="timeline-actions-duration">
                    <CardHeader className="pb-3">
                      <CardTitle className="text-xs font-bold uppercase tracking-widest text-[#9ca3af]">
                        {t("caseDetail.timeline.metrics.actionDurations")}
                      </CardTitle>
                    </CardHeader>
                    <CardContent>
                      {timelineMetrics.actionDurations.length === 0 ? (
                        <p className="text-sm text-[#9ca3af]">{t("caseDetail.timeline.metrics.noData")}</p>
                      ) : (
                        <div className="space-y-2">
                          {timelineMetrics.actionDurations.map((item) => (
                            <div key={item.action} className="flex items-center justify-between gap-3 text-sm">
                              <span className="truncate">{item.action}</span>
                              <span className="shrink-0 text-xs text-[#9ca3af]">
                                {formatDurationCompact(item.durationMs)} · {item.count}x
                              </span>
                            </div>
                          ))}
                        </div>
                      )}
                    </CardContent>
                  </Card>

                  <Card className={CASE_SUBPANEL_CLASS} data-testid="timeline-status-breakdown">
                    <CardHeader className="pb-3">
                      <CardTitle className="text-xs font-bold uppercase tracking-widest text-[#9ca3af]">
                        {t("caseDetail.timeline.metrics.statusBreakdown")}
                      </CardTitle>
                    </CardHeader>
                    <CardContent>
                      {timelineMetrics.statusBreakdown.length === 0 ? (
                        <p className="text-sm text-[#9ca3af]">{t("caseDetail.timeline.metrics.noData")}</p>
                      ) : (
                        <div className="flex flex-wrap gap-2">
                          {timelineMetrics.statusBreakdown.map((item) => (
                            <Badge key={item.status} variant="outline" className="rounded-lg text-xs">
                              {item.status}: {item.count}
                            </Badge>
                          ))}
                        </div>
                      )}
                    </CardContent>
                  </Card>
                </div>

                <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
                  <Card className={CASE_SUBPANEL_CLASS} data-testid="card-case-sla">
                    <CardHeader className="pb-3">
                      <CardTitle className="text-xs font-bold uppercase tracking-widest text-[#9ca3af] flex items-center gap-2">
                        <Timer size={14} className="text-primary" />
                        {t("caseDetail.timeline.slaTitle")}
                      </CardTitle>
                    </CardHeader>
                    <CardContent className="space-y-3">
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <Badge
                          className={`rounded-md border text-[10px] ${
                            caseSLA.breached
                              ? "border-[rgba(244,63,94,0.3)] bg-[rgba(244,63,94,0.16)] text-[#fda4af]"
                              : "border-[rgba(16,185,129,0.3)] bg-[rgba(16,185,129,0.16)] text-[#6ee7b7]"
                          }`}
                          data-testid="badge-case-sla-state"
                        >
                          {caseSLA.breached ? t("caseDetail.timeline.slaBreached") : t("caseDetail.timeline.slaOnTrack")}
                        </Badge>
                        <span className="text-xs text-[#9ca3af]" data-testid="text-case-sla-target">
                          {t("caseDetail.timeline.slaTargetMinutes", { minutes: String(caseSLA.targetMinutes) })}
                        </span>
                      </div>
                      <Progress value={caseSLA.progress} className="h-2" data-testid="progress-case-sla" />
                      <div className="text-xs text-[#9ca3af]" data-testid="text-case-sla-due">
                        {caseSLA.dueAtTs > 0
                          ? t("caseDetail.timeline.slaDueAt", { date: format(new Date(caseSLA.dueAtTs), "dd.MM.yyyy HH:mm") })
                          : t("caseDetail.timeline.slaDueMissing")}
                      </div>
                    </CardContent>
                  </Card>

                  <Card className={CASE_SUBPANEL_CLASS} data-testid="card-case-reminders">
                    <CardHeader className="pb-3">
                      <CardTitle className="text-xs font-bold uppercase tracking-widest text-[#9ca3af] flex items-center gap-2">
                        <Bell size={14} className="text-primary" />
                        {t("caseDetail.timeline.remindersTitle")}
                      </CardTitle>
                    </CardHeader>
                    <CardContent className="space-y-3">
                      <Input
                        value={reminderMessage}
                        onChange={(event) => setReminderMessage(event.target.value)}
                        placeholder={t("caseDetail.timeline.reminderMessagePlaceholder")}
                        className={CASE_INPUT_CLASS}
                        data-testid="input-reminder-message"
                      />
                      <div className="flex flex-wrap gap-2">
                        {QUICK_REMINDER_PRESETS.map((preset) => (
                          <Button
                            key={preset.id}
                            type="button"
                            size="sm"
                            variant="outline"
                            className="rounded-lg border-[#2a2c3c] bg-[#0f1118] text-xs text-[#d1d5db] hover:bg-[#1a1f2d]"
                            onClick={() => handleScheduleReminder(preset.minutes)}
                            disabled={createTimelineEvent.isPending}
                            data-testid={`button-schedule-reminder-${preset.id}`}
                          >
                            {t("caseDetail.timeline.reminderIn", { minutes: String(preset.minutes) })}
                          </Button>
                        ))}
                      </div>
                    </CardContent>
                  </Card>
                </div>

                <Card className={CASE_SUBPANEL_CLASS} data-testid="card-case-updates">
                  <CardHeader className="pb-3">
                    <CardTitle className="text-xs font-bold uppercase tracking-widest text-[#9ca3af] flex items-center gap-2">
                      <Activity size={14} className="text-primary" />
                      {t("caseDetail.timeline.updatesTitle")}
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    {recentCaseUpdates.length === 0 ? (
                      <p className="text-sm text-[#9ca3af]">{t("caseDetail.timeline.empty")}</p>
                    ) : (
                      <div className="space-y-2">
                        {recentCaseUpdates.map((event: any, index: number) => {
                          const eventTypeRaw = String(event?.eventType || event?.title || "").trim();
                          const eventKey = TIMELINE_EVENT_KEYS[eventTypeRaw] || TIMELINE_EVENT_KEYS[eventTypeRaw.toLowerCase()] || "";
                          const title = eventKey ? t(eventKey) : (event.title || humanizeTimelineEventType(eventTypeRaw || "event"));
                          const connectorExecutionID = String(
                            (event.metadata && (event.metadata.execution_id || event.metadata.executionId)) || "",
                          ).trim();
                          const isConnectorExecution = String(event?.eventType || "").trim() === "connector_execution" || Boolean(connectorExecutionID);
                          const handleClick = () => {
                            if (!isConnectorExecution || !connectorExecutionID) return;
                            setSelectedConnectorExecutionID(connectorExecutionID);
                            setConnectorExecutionDrawerOpen(true);
                          };
                          const Container: any = isConnectorExecution ? "button" : "div";
                          return (
                            <Container
                              key={`${event.id || "update"}-${index}`}
                              type={isConnectorExecution ? "button" : undefined}
                              onClick={isConnectorExecution ? handleClick : undefined}
                              className={`rounded-lg border border-[#2a2c3c] px-3 py-2 text-xs ${isConnectorExecution ? "w-full cursor-pointer text-left hover:bg-[#161927]" : ""}`}
                              data-testid={`case-update-${index}`}
                            >
                              <div className="flex items-center justify-between gap-2">
                                <div className="flex items-center gap-2">
                                  <span className="font-semibold">{title}</span>
                                  {isConnectorExecution ? (
                                    <span className="inline-flex items-center rounded-full bg-[rgba(15,23,42,0.9)] px-2 py-0.5 text-[9px] font-medium uppercase tracking-wide text-[#93c5fd]">
                                      Connector Hub
                                    </span>
                                  ) : null}
                                </div>
                                <span className="text-[#9ca3af]">{formatTimelineDate(event.createdAt)}</span>
                              </div>
                              {event.description ? (
                                <p className="mt-1 text-[#9ca3af]">{event.description}</p>
                              ) : null}
                            </Container>
                          );
                        })}
                      </div>
                    )}
                  </CardContent>
                </Card>

                <Card className={CASE_SUBPANEL_CLASS} data-testid="card-timeline-note-form">
                  <CardHeader className="pb-3">
                    <CardTitle className="text-xs font-bold uppercase tracking-widest text-[#9ca3af]">
                      {t("caseDetail.timeline.noteTitle")}
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-3">
                    <Input
                      value={timelineNoteTitle}
                      onChange={(event) => setTimelineNoteTitle(event.target.value)}
                      placeholder={t("caseDetail.timeline.noteTitlePlaceholder")}
                      className={CASE_INPUT_CLASS}
                      data-testid="input-timeline-note-title"
                    />
                    <Textarea
                      value={timelineNoteBody}
                      onChange={(event) => setTimelineNoteBody(event.target.value)}
                      placeholder={t("caseDetail.timeline.noteBodyPlaceholder")}
                      className="min-h-[96px] rounded-lg border-[#2a2c3c] bg-[#0f1118] text-[#f3f4f6] placeholder:text-[#6b7280] focus-visible:ring-[#3b82f6]/40"
                      data-testid="input-timeline-note-body"
                    />
                    <div className="flex justify-end">
                      <Button
                        onClick={handleAddTimelineNote}
                        disabled={!timelineNoteBody.trim() || createTimelineEvent.isPending}
                        className="rounded-lg gap-2 bg-[#11141d] text-[#f3f4f6] hover:bg-[#1d2433]"
                        data-testid="button-add-timeline-note"
                      >
                        <MessageSquare size={14} />
                        {createTimelineEvent.isPending ? t("common.loading") : t("caseDetail.timeline.noteSave")}
                      </Button>
                    </div>
                  </CardContent>
                </Card>

                <Card className={CASE_SUBPANEL_CLASS} data-testid="card-case-playbook-catalog">
                  <CardHeader className="pb-3">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <CardTitle className="text-xs font-bold uppercase tracking-widest text-[#9ca3af]">
                        {t("caseDetail.timeline.playbookCatalogTitle")}
                      </CardTitle>
                      <Badge variant="outline" className="rounded-lg text-[10px]">
                        {t("caseDetail.timeline.playbookCount", { count: String(availablePlaybooks.length) })}
                      </Badge>
                    </div>
                    {suggestedPlaybooks.length > 0 ? (
                      <p className="text-xs text-[#9ca3af]" data-testid="text-playbook-suggested-caption">
                        {t("caseDetail.timeline.playbookSuggestedHint")}
                      </p>
                    ) : null}
                  </CardHeader>
                  <CardContent className="space-y-4">
                    {availablePlaybooks.length === 0 ? (
                      <p className="text-sm text-[#9ca3af]">{t("caseDetail.timeline.playbookCatalogEmpty")}</p>
                    ) : (
                      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
                        <div className="space-y-2 max-h-80 overflow-y-auto pr-1" data-testid="list-case-playbooks">
                          {availablePlaybooks.map((playbook) => {
                            const isSelected = selectedPlaybookID === playbook.id;
                            const suggestedMeta = suggestedPlaybookMeta.find((item) => item.id === playbook.id);
                            return (
                              <button
                                key={playbook.id}
                                type="button"
                                className={`w-full rounded-lg border px-3 py-2 text-left transition-colors ${
                                  isSelected ? "border-[#3b82f6] bg-[rgba(59,130,246,0.12)]" : "border-[#2a2c3c] hover:bg-[#1a1f2d]"
                                }`}
                                onClick={() => applyPlaybookAutofill(playbook.id)}
                                data-testid={`button-select-playbook-${playbook.id}`}
                              >
                                <div className="flex items-start justify-between gap-2">
                                  <span className="text-sm font-semibold">{playbook.name}</span>
                                  <div className="flex items-center gap-1">
                                    {suggestedPlaybookIDs.has(playbook.id) ? (
                                      <Badge variant="secondary" className="rounded-md text-[10px]" data-testid={`badge-playbook-suggested-${playbook.id}`}>
                                        {t("caseDetail.timeline.playbookSuggested")}
                                      </Badge>
                                    ) : null}
                                  </div>
                                </div>
                                {playbook.description ? (
                                  <div className="mt-1 text-xs text-[#9ca3af] line-clamp-2">{playbook.description}</div>
                                ) : null}
                                {suggestedMeta?.reasons?.length ? (
                                  <div className="mt-1 text-[11px] text-[#9ca3af]">
                                    {suggestedMeta.reasons.join(" · ")}
                                  </div>
                                ) : null}
                              </button>
                            );
                          })}
                        </div>

                        <Card className={CASE_SUBPANEL_CLASS} data-testid="card-playbook-launcher">
                          <CardHeader className="pb-2">
                            <CardTitle className="text-xs font-bold uppercase tracking-widest text-[#9ca3af]">
                              {t("caseDetail.timeline.playbookInputTitle")}
                            </CardTitle>
                          </CardHeader>
                          <CardContent className="space-y-3">
                            <div className="text-sm font-semibold" data-testid="text-selected-playbook">
                              {selectedPlaybook?.name || t("caseDetail.timeline.playbookSelectRequired")}
                            </div>
                            <Textarea
                              value={playbookInputDraft}
                              onChange={(event) => {
                                setPlaybookInputDraft(event.target.value);
                                setPlaybookInputDirty(true);
                              }}
                              placeholder={t("caseDetail.timeline.playbookInputPlaceholder")}
                              className="min-h-[210px] rounded-lg border-[#2a2c3c] bg-[#0f1118] font-mono text-xs text-[#f3f4f6] placeholder:text-[#6b7280] focus-visible:ring-[#3b82f6]/40"
                              data-testid="textarea-playbook-input"
                            />
                            <div className="flex flex-wrap items-center justify-between gap-2">
                              <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                className="rounded-lg border-[#2a2c3c] bg-[#0f1118] text-[#d1d5db] hover:bg-[#1a1f2d]"
                                onClick={() => {
                                  if (!selectedPlaybookID) return;
                                  applyPlaybookAutofill(selectedPlaybookID);
                                }}
                                disabled={!selectedPlaybookID}
                                data-testid="button-playbook-autofill"
                              >
                                {t("caseDetail.timeline.playbookAutofill")}
                              </Button>
                              <Button
                                type="button"
                                size="sm"
                                className="rounded-lg gap-2 bg-[#11141d] text-[#f3f4f6] hover:bg-[#1d2433]"
                                onClick={handleRunPlaybookFromCase}
                                disabled={!selectedPlaybookID || runWorkflow.isPending}
                                data-testid="button-run-playbook-case"
                              >
                                <Sparkles size={14} />
                                {runWorkflow.isPending ? t("common.loading") : t("caseDetail.timeline.playbookRun")}
                              </Button>
                            </div>
                          </CardContent>
                        </Card>
                      </div>
                    )}
                  </CardContent>
                </Card>

                <Card className={CASE_SUBPANEL_CLASS} data-testid="card-case-playbook-graph">
                  <CardHeader className="pb-3">
                    <CardTitle className="text-xs font-bold uppercase tracking-widest text-[#9ca3af] flex items-center gap-2">
                      <GitBranch size={14} className="text-primary" />
                      {t("caseDetail.timeline.playbookGraphTitle")}
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-3">
                    {!selectedPlaybook ? (
                      <p className="text-sm text-[#9ca3af]">{t("caseDetail.timeline.playbookSelectRequired")}</p>
                    ) : selectedPlaybookGraph.orderedNodes.length === 0 ? (
                      <p className="text-sm text-[#9ca3af]">{t("caseDetail.timeline.playbookGraphEmpty")}</p>
                    ) : (
                      <>
                        <div className="flex flex-wrap gap-2" data-testid="list-playbook-graph-nodes">
                          {selectedPlaybookGraph.orderedNodes.map((node: any, index: number) => (
                            <div
                              key={node.id || index}
                              className="inline-flex items-center gap-1 rounded-lg border border-[#2a2c3c] px-2 py-1 text-xs"
                              data-testid={`playbook-graph-node-${index}`}
                            >
                              <span className="font-semibold">{node.label || node.id}</span>
                              <Badge variant="outline" className="rounded text-[10px]">
                                {node.type || "task"}
                              </Badge>
                            </div>
                          ))}
                        </div>
                        <div className="space-y-1" data-testid="list-playbook-graph-edges">
                          {selectedPlaybookGraph.edgeView.length === 0 ? (
                            <p className="text-xs text-[#9ca3af]">{t("caseDetail.timeline.playbookGraphNoEdges")}</p>
                          ) : (
                            selectedPlaybookGraph.edgeView.map((edge) => (
                              <div key={edge.id} className="text-xs text-[#9ca3af]">
                                {edge.sourceLabel} {"->"} {edge.targetLabel}
                                {edge.condition ? ` (${edge.condition})` : ""}
                              </div>
                            ))
                          )}
                        </div>
                      </>
                    )}
                  </CardContent>
                </Card>

                <Card className={CASE_SUBPANEL_CLASS} data-testid="card-case-playbook-runs">
                  <CardHeader className="pb-3">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <CardTitle className="text-xs font-bold uppercase tracking-widest text-[#9ca3af]">
                        {t("caseDetail.timeline.playbookTitle")}
                      </CardTitle>
                      <Badge variant="outline" className="rounded-lg text-[10px]">
                        {t("caseDetail.timeline.playbookCount", { count: String(casePlaybookRuns.length) })}
                      </Badge>
                    </div>
                  </CardHeader>
                  <CardContent>
                    {casePlaybookRuns.length === 0 ? (
                      <p className="text-sm text-[#9ca3af]">{t("caseDetail.timeline.playbookEmpty")}</p>
                    ) : (
                      <div className="space-y-2 max-h-72 overflow-y-auto pr-1">
                        {casePlaybookRuns.map((run: any, index: number) => {
                          const status = String(run?.status || "unknown").toLowerCase();
                          const badgeClass =
                            status === "success"
                              ? "border-[rgba(16,185,129,0.3)] bg-[rgba(16,185,129,0.16)] text-[#6ee7b7]"
                              : status === "failed"
                                ? "border-[rgba(244,63,94,0.3)] bg-[rgba(244,63,94,0.16)] text-[#fda4af]"
                                : "border-[rgba(245,158,11,0.3)] bg-[rgba(245,158,11,0.16)] text-[#fbbf24]";
                          return (
                            <div key={run.id || index} className="rounded-lg border px-3 py-2" data-testid={`case-playbook-run-${index}`}>
                              <div className="flex flex-wrap items-center gap-2">
                                <span className="text-sm font-semibold">{run.workflowName || run.workflowId || "Workflow"}</span>
                                <Badge className={`${badgeClass} rounded-md border text-[10px]`}>{status}</Badge>
                                <span className="text-[11px] text-[#9ca3af]">
                                  {run.durationMs ? formatDurationCompact(run.durationMs) : "—"}
                                </span>
                              </div>
                              <div className="mt-1 text-[11px] text-[#9ca3af]">
                                {formatDateTimeWithSeconds(run.startedAt || run.createdAt)}
                              </div>
                              {run.error ? (
                                <div className="mt-1 text-xs text-[#fda4af]">{run.error}</div>
                              ) : null}
                              {summarizeWorkflowRunPayload(run.result) ? (
                                <pre className="mt-1 overflow-x-auto whitespace-pre-wrap break-words rounded-md bg-[#171b28] p-2 text-[11px] text-[#d1d5db]">
                                  {summarizeWorkflowRunPayload(run.result)}
                                </pre>
                              ) : null}
                            </div>
                          );
                        })}
                      </div>
                    )}
                  </CardContent>
                </Card>

                {timelineRowsDesc.length === 0 ? (
                  <div className="text-center py-12 text-[#9ca3af]" data-testid="timeline-empty">
                    <Clock size={32} className="mx-auto mb-3 opacity-50" />
                    <p className="text-sm">{t("caseDetail.timeline.empty")}</p>
                  </div>
                ) : (
                  <div className="space-y-4">
                    {timelineRowsDesc.map((event: any, idx: number) => {
                      const eventType = String(event.eventType || "").trim();
                      const icon = timelineIcons[eventType] || timelineIcons.default;
                      const colorClass = timelineColors[eventType] || timelineColors.default;
                      const eventLabel = t(TIMELINE_EVENT_KEYS[eventType] || humanizeTimelineEventType(eventType));
                      const title = String(event.title || "").trim() || eventLabel;
                      const description = String(event.description || "").trim();
                      const connectorExecutionID = String(
                        (event.metadata && (event.metadata.execution_id || event.metadata.executionId)) || "",
                      ).trim();
                      const isConnectorExecution = eventType === "connector_execution" || Boolean(connectorExecutionID);
                      const eventActor = event?.actor && typeof event.actor === "object" ? event.actor : null;
                      const eventUser = event.userId ? usersByID.get(String(event.userId)) : null;
                      const eventAuthorName = String(eventActor?.authorName || eventUser?.name || event.userId || "").trim();
                      const handleClick = () => {
                        if (!isConnectorExecution || !connectorExecutionID) return;
                        setSelectedConnectorExecutionID(connectorExecutionID);
                        setConnectorExecutionDrawerOpen(true);
                      };
                      const Container: any = isConnectorExecution ? "button" : "div";
                      return (
                        <Container
                          key={event.id || idx}
                          type={isConnectorExecution ? "button" : undefined}
                          onClick={isConnectorExecution ? handleClick : undefined}
                          className={`flex ${isConnectorExecution ? "w-full cursor-pointer text-left hover:bg-[#161927]" : ""} items-start gap-4 p-3 rounded-xl border ${colorClass}`}
                          data-testid={`timeline-event-${idx}`}
                        >
                          <div className="mt-0.5 shrink-0">{icon}</div>
                          <div className="flex-1 min-w-0">
                            <div className="text-sm font-medium" data-testid={`timeline-event-desc-${idx}`}>{title}</div>
                            {description ? (
                              <div className="mt-1 text-xs text-[#d1d5db] whitespace-pre-wrap break-words">{description}</div>
                            ) : null}
                            <div className="mt-1 flex flex-wrap items-center gap-2">
                              <Badge variant="outline" className="text-[10px] rounded-lg">{eventLabel}</Badge>
                              {isConnectorExecution ? (
                                <Badge variant="outline" className="rounded-lg border-[rgba(59,130,246,0.45)] bg-[rgba(15,23,42,0.9)] px-1.5 text-[9px] uppercase tracking-wide text-[#bfdbfe]">
                                  Connector Hub
                                </Badge>
                              ) : null}
                              {eventAuthorName ? (
                                <span className="inline-flex items-center gap-1.5 text-xs text-[#9ca3af]">
                                  <UserAvatar
                                    name={eventAuthorName}
                                    avatar={eventActor ? undefined : eventUser?.avatar}
                                    fallback={eventActor?.authorId || event.userId}
                                    authorKind={eventActor?.authorKind}
                                    authorAvatarKey={eventActor?.authorAvatarKey}
                                    className="h-5 w-5 rounded-lg border border-[#2a2c3c]"
                                    fallbackClassName="text-[9px]"
                                  />
                                  <span>{eventAuthorName}</span>
                                </span>
                              ) : null}
                            </div>
                          </div>
                          <div className="flex items-center gap-1 text-xs text-[#9ca3af] whitespace-nowrap" data-testid={`timeline-event-time-${idx}`}>
                            <span>{formatTimelineDate(event.createdAt)}</span>
                            {isConnectorExecution ? (
                              <ArrowUpRight size={12} className="text-[#93c5fd] opacity-80" />
                            ) : null}
                          </div>
                        </Container>
                      );
                    })}
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          {/* Tasks Tab */}
          <TabsContent value="tasks" className="mt-6 space-y-6" data-testid="tab-content-tasks">
            <Card className={CASE_PANEL_CLASS}>
              <CardHeader>
                <CardTitle className="text-sm font-semibold uppercase tracking-[0.12em] text-[#9ca3af] flex items-center gap-2">
                  <Plus size={16} className="text-primary" /> {t("caseDetail.task.create")}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
                  <Input
                    placeholder={t("caseDetail.task.titlePlaceholder")}
                    value={taskTitle}
                    onChange={(e) => setTaskTitle(e.target.value)}
                    className={CASE_INPUT_CLASS}
                    data-testid="input-task-title"
                  />
                  <Select value={taskAssignee} onValueChange={setTaskAssignee}>
                    <SelectTrigger className={CASE_SELECT_TRIGGER_CLASS} data-testid="select-task-assignee">
                      <SelectValue placeholder={t("caseDetail.overview.assignee")} />
                    </SelectTrigger>
                    <SelectContent className={CASE_SELECT_CONTENT_CLASS}>
                      {(users || []).map((u: any) => (
                        <SelectItem key={u.id} value={u.id}>{u.name}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <Input
                    type="datetime-local"
                    value={taskDueAt}
                    onChange={(e) => setTaskDueAt(e.target.value)}
                    className={CASE_INPUT_CLASS}
                    data-testid="input-task-due-date"
                  />
                </div>
                <div className="mt-2 flex flex-wrap items-center gap-2" data-testid="task-due-presets">
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    className="h-8 rounded-lg text-xs"
                    onClick={() => setTaskDueAt(toDateTimeLocal(new Date(Date.now() + 60 * 60 * 1000).toISOString()))}
                  >
                    +1h
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    className="h-8 rounded-lg text-xs"
                    onClick={() => setTaskDueAt(toDateTimeLocal(new Date(Date.now() + 4 * 60 * 60 * 1000).toISOString()))}
                  >
                    +4h
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    className="h-8 rounded-lg text-xs"
                    onClick={() => setTaskDueAt(toDateTimeLocal(new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString()))}
                  >
                    +1d
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    className="h-8 rounded-lg text-xs"
                    onClick={() => setTaskDueAt(toDateTimeLocal(new Date(Date.now() + 3 * 24 * 60 * 60 * 1000).toISOString()))}
                  >
                    +3d
                  </Button>
                </div>
                <Textarea
                  placeholder={t("caseDetail.task.descriptionPlaceholder")}
                  value={taskDesc}
                  onChange={(e) => setTaskDesc(e.target.value)}
                  className="mt-3 rounded-xl border-[#2a2c3c] bg-[#0f1118] text-[#f3f4f6] placeholder:text-[#6b7280] focus-visible:ring-[#3b82f6]/40"
                  rows={2}
                  data-testid="input-task-description"
                />
                <div className="flex items-center justify-between mt-3">
                  <div className="flex items-center gap-2">
                    <Checkbox
                      checked={taskMandatory}
                      onCheckedChange={(c) => setTaskMandatory(!!c)}
                      data-testid="checkbox-task-mandatory"
                    />
                    <span className="text-sm text-[#9ca3af]">{t("caseDetail.task.mandatory")}</span>
                  </div>
                  <Button onClick={handleCreateTask} className="rounded-xl gap-2 bg-[#11141d] font-semibold text-[#f3f4f6] hover:bg-[#1d2433]" disabled={!taskTitle.trim() || !taskDueAt.trim()} data-testid="button-create-task">
                    <Plus size={14} /> {t("caseDetail.task.add")}
                  </Button>
                </div>
              </CardContent>
            </Card>

            <div className="flex items-center gap-3">
              <Progress value={taskCompletion} className="flex-1 h-3" />
              <span className="text-sm font-bold text-[#9ca3af]" data-testid="text-task-progress">{t("caseDetail.task.progress", { percent: String(taskCompletion) })}</span>
            </div>

            <Card className={CASE_SUBPANEL_CLASS} data-testid="card-task-chain-mode">
              <CardHeader className="pb-3">
                <CardTitle className="text-xs font-bold uppercase tracking-widest text-[#9ca3af] flex items-center gap-2">
                  <GitBranch size={14} className="text-primary" />
                  {t("caseDetail.task.chain.title")}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="flex flex-wrap gap-2">
                  {TASK_CHAIN_MODES.map((mode) => (
                    <Button
                      key={mode}
                      type="button"
                      size="sm"
                      variant={taskChainMode === mode ? "default" : "outline"}
                      className="rounded-lg text-xs"
                      onClick={() => handleSetTaskChainMode(mode)}
                      data-testid={`button-task-chain-mode-${mode}`}
                    >
                      {t(TASK_CHAIN_MODE_KEYS[mode])}
                    </Button>
                  ))}
                </div>
                <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
                  <div className="rounded-lg border border-[#2a2c3c] px-3 py-2 text-xs">
                    <div className="text-[#9ca3af]">{t("caseDetail.task.chain.openTasks")}</div>
                    <div className="mt-1 text-sm font-semibold" data-testid="text-open-task-count">{openTasks.length}</div>
                  </div>
                  <div className="rounded-lg border border-[#2a2c3c] px-3 py-2 text-xs">
                    <div className="text-[#9ca3af]">{t("caseDetail.task.chain.overdueTasks")}</div>
                    <div className="mt-1 text-sm font-semibold" data-testid="text-overdue-task-count">{taskSLA.overdueCount}</div>
                  </div>
                </div>
              </CardContent>
            </Card>

            <div className="flex gap-2 flex-wrap">
              {["All", ...TASK_STATUSES].map((s) => (
                <Button
                  key={s}
                  variant={taskFilter === s ? "default" : "outline"}
                  size="sm"
                  className={`rounded-xl text-xs ${taskFilter === s ? "bg-[#11141d] text-[#f3f4f6] hover:bg-[#1d2433]" : "border-[#2a2c3c] bg-[#0f1118] text-[#d1d5db] hover:bg-[#1a1f2d]"}`}
                  onClick={() => setTaskFilter(s)}
                  data-testid={`filter-task-${s.toLowerCase().replace(/\s/g, "-")}`}
                >
                  {s === "All" ? t("caseDetail.task.filter.all") : t(TASK_STATUS_KEYS[s] || s)}
                </Button>
              ))}
            </div>

            <div className="space-y-3">
              {filteredTasks.length === 0 ? (
                <div className="text-center py-12 text-[#9ca3af]" data-testid="tasks-empty">
                  <ListChecks size={32} className="mx-auto mb-3 opacity-50" />
                  <p className="text-sm">{t("caseDetail.task.empty")}</p>
                </div>
              ) : (
                filteredTasks.map((task: any) => (
                  <Card key={task.id} className={`${CASE_PANEL_CLASS} rounded-xl p-4`} data-testid={`task-card-${task.id}`}>
                    <div className="flex items-start gap-3">
                      <div className="mt-1">{taskStatusIcons[task.status] || taskStatusIcons.Pending}</div>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2">
                          <span className="font-bold text-sm" data-testid={`task-title-${task.id}`}>{task.title}</span>
                          {task.mandatory && (
                            <Badge variant="destructive" className="text-[10px] rounded-lg" data-testid={`task-mandatory-${task.id}`}>{t("caseDetail.task.mandatory")}</Badge>
                          )}
                        </div>
                        {task.description && (
                          <p className="text-xs text-[#9ca3af] mt-1">{task.description}</p>
                        )}
                        {task.assignee ? (() => {
                          const taskAssigneeUser = usersByID.get(String(task.assignee));
                          const taskAssigneeLabel = taskAssigneeUser?.name || task.assignee;
                          return (
                            <span className="text-xs text-[#9ca3af] mt-1 block">
                              {t("caseDetail.task.assigned")}:{" "}
                              <span className="inline-flex items-center gap-1">
                                <UserAvatar
                                  name={taskAssigneeLabel}
                                  avatar={taskAssigneeUser?.avatar}
                                  fallback={task.assignee}
                                  className="h-4 w-4 border border-[#2a2c3c]"
                                  fallbackClassName="text-[8px] font-bold"
                                />
                                <span>{taskAssigneeLabel}</span>
                              </span>
                            </span>
                          );
                        })() : null}
                        {task.dueDate ? (() => {
                          const dueTs = toTimestamp(task.dueDate);
                          const deltaMs = dueTs > 0 ? dueTs - nowTs : 0;
                          const isOverdue = deltaMs < 0 && task.status !== "Done" && task.status !== "Cancelled";
                          const countdownLabel = isOverdue
                            ? `Overdue by ${formatDurationCompact(Math.abs(deltaMs))}`
                            : `Remaining ${formatDurationCompact(deltaMs)}`;
                          return (
                            <div
                              className="mt-1 flex flex-wrap items-center gap-2 text-xs"
                              data-testid={`task-due-date-${task.id}`}
                            >
                              <span className={isOverdue ? "text-rose-600" : "text-[#9ca3af]"}>
                                {t("caseDetail.task.dueAt")}: {format(new Date(task.dueDate), "dd.MM.yyyy HH:mm")}
                              </span>
                              {dueTs > 0 ? (
                                <Badge
                                  variant="outline"
                                  className={`h-5 rounded-lg px-2 text-[10px] font-semibold ${isOverdue
                                    ? "border-[rgba(244,63,94,0.35)] bg-[rgba(244,63,94,0.12)] text-[#fda4af]"
                                    : "border-[rgba(34,197,94,0.3)] bg-[rgba(34,197,94,0.12)] text-[#86efac]"}`}
                                >
                                  {countdownLabel}
                                </Badge>
                              ) : null}
                            </div>
                          );
                        })() : null}
                      </div>
                      <div className="flex items-center gap-2 shrink-0">
                        <Select
                          value={task.status}
                          onValueChange={(val) => updateTask.mutate({ id: task.id, data: { status: val } })}
                        >
                          <SelectTrigger className="h-8 w-32 rounded-lg border-[#2a2c3c] bg-[#0f1118] text-xs text-[#f3f4f6]" data-testid={`select-task-status-${task.id}`}>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent className={CASE_SELECT_CONTENT_CLASS}>
                            {TASK_STATUSES.map((s) => (
                              <SelectItem key={s} value={s}>{t(TASK_STATUS_KEYS[s] || s)}</SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8 text-[#fda4af] hover:bg-[rgba(244,63,94,0.16)] hover:text-[#fda4af]"
                          onClick={() => { deleteTask.mutate(task.id); toast.success(t("caseDetail.toast.taskDeleted")); }}
                          data-testid={`button-delete-task-${task.id}`}
                        >
                          <Trash2 size={14} />
                        </Button>
                      </div>
                    </div>
                  </Card>
                ))
              )}
            </div>
          </TabsContent>

          <CaseObservablesTab
            t={t}
            obsType={obsType}
            setObsType={setObsType}
            obsValue={obsValue}
            setObsValue={setObsValue}
            obsVerdict={obsVerdict}
            setObsVerdict={setObsVerdict}
            obsTags={obsTags}
            setObsTags={setObsTags}
            onCreateObservable={handleCreateObservable}
            observableRows={observableRows}
            observableConnectorOptionsByType={observableConnectorOptionsByType}
            observableTagDrafts={observableTagDrafts}
            onObservableTagDraftChange={handleObservableTagDraftChange}
            onAddObservableTag={(observable) => handleAddObservableTag(observable)}
            onRemoveObservableTag={(observable, tag) => handleRemoveObservableTag(observable, tag)}
            onRunConnectorForObservable={(observable, option) => handleRunConnectorForObservable(observable, option)}
            onUpdateObservableType={(observable, value) => {
              updateObservable.mutate(
                {
                  id: observable.id,
                  data: {
                    caseId: id,
                    type: value,
                  },
                },
                {
                  onSuccess: () => toast.success("Observable type updated"),
                  onError: (error: any) => toast.error(error?.message || "Failed to update observable type"),
                },
              );
            }}
            onDeleteObservable={(observable) => {
              deleteObservable.mutate(observable.id);
              toast.success(t("caseDetail.toast.observableDeleted"));
            }}
            executePending={executeConnectorHub.isPending}
            panelClass={CASE_PANEL_CLASS}
            inputClass={CASE_INPUT_CLASS}
            selectTriggerClass={CASE_SELECT_TRIGGER_CLASS}
            selectContentClass={CASE_SELECT_CONTENT_CLASS}
          />

          {/* Visualizations Tab */}
          <TabsContent value="visuals" className="mt-6 space-y-6" data-testid="tab-content-visuals">
            <Card className={CASE_PANEL_CLASS}>
              <CardHeader>
                <CardTitle className="text-sm font-semibold uppercase tracking-[0.12em] text-[#9ca3af] flex items-center gap-2">
                  <BarChart3 size={16} className="text-primary" /> {t("caseDetail.visual.title")}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <p className="text-sm text-[#9ca3af]">{t("caseDetail.visual.subtitle")}</p>
                <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-[220px_220px_220px_minmax(0,1fr)_auto]">
                  <div className="space-y-1">
                    <Label className="text-xs">{t("caseDetail.visual.filters.source")}</Label>
                    <Select value={visualSourceFilter} onValueChange={(value) => setVisualSourceFilter(value as VisualizationSourceFilter)}>
                      <SelectTrigger className={CASE_SELECT_TRIGGER_CLASS} data-testid="select-visual-source-filter">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent className={CASE_SELECT_CONTENT_CLASS}>
                        <SelectItem value="all" data-testid="option-visual-source-all">{t("caseDetail.visual.filter.all")}</SelectItem>
                        <SelectItem value="network" data-testid="option-visual-source-network">{t("caseDetail.visual.source.network")}</SelectItem>
                        <SelectItem value="authorization" data-testid="option-visual-source-authorization">{t("caseDetail.visual.source.authorization")}</SelectItem>
                        <SelectItem value="other" data-testid="option-visual-source-other">{t("caseDetail.visual.source.other")}</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-1">
                    <Label className="text-xs">{t("caseDetail.visual.filters.type")}</Label>
                    <Select value={visualTypeFilter} onValueChange={(value) => setVisualTypeFilter(value as VisualizationTypeFilter)}>
                      <SelectTrigger className={CASE_SELECT_TRIGGER_CLASS} data-testid="select-visual-type-filter">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent className={CASE_SELECT_CONTENT_CLASS}>
                        <SelectItem value="all" data-testid="option-visual-type-all">{t("caseDetail.visual.filter.all")}</SelectItem>
                        {observablesByTypeChartData.map((item) => (
                          <SelectItem key={`visual-type-filter-${item.type}`} value={item.type} data-testid={`option-visual-type-${item.type}`}>
                            {item.type}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-1">
                    <Label className="text-xs">{t("caseDetail.visual.filters.files")}</Label>
                    <Select value={visualAttachmentFilter} onValueChange={(value) => setVisualAttachmentFilter(value as VisualizationAttachmentFilter)}>
                      <SelectTrigger className={CASE_SELECT_TRIGGER_CLASS} data-testid="select-visual-file-filter">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent className={CASE_SELECT_CONTENT_CLASS}>
                        <SelectItem value="all" data-testid="option-visual-file-all">{t("caseDetail.visual.filter.all")}</SelectItem>
                        <SelectItem value="images" data-testid="option-visual-file-images">{t("caseDetail.visual.filter.images")}</SelectItem>
                        <SelectItem value="files" data-testid="option-visual-file-files">{t("caseDetail.visual.filter.files")}</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-1">
                    <Label className="text-xs">{t("caseDetail.visual.filters.search")}</Label>
                    <Input
                      value={visualSearch}
                      onChange={(event) => setVisualSearch(event.target.value)}
                      placeholder={t("caseDetail.visual.filters.searchPlaceholder")}
                      className={CASE_INPUT_CLASS}
                      data-testid="input-visual-search"
                    />
                  </div>
                  <div className="flex items-end">
                    <Button
                      type="button"
                      variant="outline"
                      className="w-full rounded-xl border-[#2a2c3c] bg-[#0f1118] text-[#d1d5db] hover:bg-[#1a1f2d]"
                      onClick={() => {
                        setVisualSourceFilter("all");
                        setVisualTypeFilter("all");
                      }}
                      disabled={visualSourceFilter === "all" && visualTypeFilter === "all"}
                      data-testid="button-visual-reset-linked-filters"
                    >
                      {t("caseDetail.visual.filters.resetLinked")}
                    </Button>
                  </div>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-4 gap-3">
                  <Card className={CASE_SUBPANEL_CLASS}>
                    <CardContent className="py-4">
                      <div className="text-xs text-[#9ca3af]">{t("caseDetail.visual.metrics.observables")}</div>
                      <div className="text-xl font-bold mt-1" data-testid="visual-metric-observables">{filteredVisualizationObservables.length}</div>
                    </CardContent>
                  </Card>
                  <Card className={CASE_SUBPANEL_CLASS}>
                    <CardContent className="py-4">
                      <div className="text-xs text-[#9ca3af]">{t("caseDetail.visual.metrics.network")}</div>
                      <div className="text-xl font-bold mt-1" data-testid="visual-metric-network">
                        {filteredVisualizationObservables.filter((observable: any) => observable.source === "network").length}
                      </div>
                    </CardContent>
                  </Card>
                  <Card className={CASE_SUBPANEL_CLASS}>
                    <CardContent className="py-4">
                      <div className="text-xs text-[#9ca3af]">{t("caseDetail.visual.metrics.authorization")}</div>
                      <div className="text-xl font-bold mt-1" data-testid="visual-metric-authorization">
                        {filteredVisualizationObservables.filter((observable: any) => observable.source === "authorization").length}
                      </div>
                    </CardContent>
                  </Card>
                  <Card className={CASE_SUBPANEL_CLASS}>
                    <CardContent className="py-4">
                      <div className="text-xs text-[#9ca3af]">{t("caseDetail.visual.metrics.files")}</div>
                      <div className="text-xl font-bold mt-1" data-testid="visual-metric-files">
                        {filteredVisualizationAttachments.length}
                      </div>
                    </CardContent>
                  </Card>
                </div>
              </CardContent>
            </Card>

            <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
              <Card className={CASE_SUBPANEL_CLASS}>
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm">{t("caseDetail.visual.chart.bySource")}</CardTitle>
                </CardHeader>
                <CardContent>
                  {observablesBySourceChartData.every((item) => item.value === 0) ? (
                    <p className="text-sm text-[#9ca3af]">{t("caseDetail.visual.empty")}</p>
                  ) : (
                    <div className="h-[240px]" data-testid="visual-chart-by-source">
                      <ResponsiveContainer width="100%" height="100%">
                        <BarChart data={observablesBySourceChartData}>
                          <CartesianGrid strokeDasharray="3 3" />
                          <XAxis dataKey="source" tick={{ fontSize: 12 }} />
                          <YAxis allowDecimals={false} />
                          <ChartTooltip />
                          <Legend />
                          <Bar
                            dataKey="value"
                            fill="hsl(var(--primary))"
                            radius={[6, 6, 0, 0]}
                            name={t("caseDetail.visual.metrics.observables")}
                            onClick={(entry: any) => {
                              const sourceKey = String(entry?.sourceKey || entry?.payload?.sourceKey || "").trim().toLowerCase();
                              if (sourceKey === "network" || sourceKey === "authorization" || sourceKey === "other") {
                                setVisualSourceFilter(sourceKey as VisualizationSourceFilter);
                              }
                            }}
                            cursor="pointer"
                          />
                        </BarChart>
                      </ResponsiveContainer>
                    </div>
                  )}
                </CardContent>
              </Card>
              <Card className={CASE_SUBPANEL_CLASS}>
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm">{t("caseDetail.visual.chart.byType")}</CardTitle>
                </CardHeader>
                <CardContent>
                  {observablesByTypeChartData.length === 0 ? (
                    <p className="text-sm text-[#9ca3af]">{t("caseDetail.visual.empty")}</p>
                  ) : (
                    <div className="h-[240px]" data-testid="visual-chart-by-type">
                      <ResponsiveContainer width="100%" height="100%">
                        <BarChart data={observablesByTypeChartData}>
                          <CartesianGrid strokeDasharray="3 3" />
                          <XAxis dataKey="type" tick={{ fontSize: 12 }} />
                          <YAxis allowDecimals={false} />
                          <ChartTooltip />
                          <Legend />
                          <Bar
                            dataKey="value"
                            fill="#16a34a"
                            radius={[6, 6, 0, 0]}
                            name={t("caseDetail.visual.chart.count")}
                            onClick={(entry: any) => {
                              const nextType = String(entry?.type || entry?.payload?.type || "").trim();
                              if (nextType) {
                                setVisualTypeFilter(nextType);
                              }
                            }}
                            cursor="pointer"
                          />
                        </BarChart>
                      </ResponsiveContainer>
                    </div>
                  )}
                </CardContent>
              </Card>
            </div>

            <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
              <Card className={CASE_SUBPANEL_CLASS}>
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm">{t("caseDetail.visual.applets.sources")}</CardTitle>
                </CardHeader>
                <CardContent>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t("caseDetail.visual.table.source")}</TableHead>
                        <TableHead className="text-right">{t("caseDetail.visual.chart.count")}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {observablesBySourceChartData.map((item) => {
                        const sourceKey = String(item?.sourceKey || "").trim() as VisualizationSourceFilter;
                        const isSelected = visualSourceFilter === sourceKey;
                        return (
                          <TableRow
                            key={`visual-source-applet-${sourceKey}`}
                            className={isSelected ? "bg-[rgba(59,130,246,0.12)]" : undefined}
                            data-testid={`visual-source-applet-row-${sourceKey}`}
                            onClick={() => {
                              setVisualSourceFilter(isSelected ? "all" : sourceKey);
                            }}
                          >
                            <TableCell className="font-medium">{item.source}</TableCell>
                            <TableCell className="text-right">{item.value}</TableCell>
                          </TableRow>
                        );
                      })}
                    </TableBody>
                  </Table>
                </CardContent>
              </Card>

              <Card className={CASE_SUBPANEL_CLASS}>
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm">{t("caseDetail.visual.applets.types")}</CardTitle>
                </CardHeader>
                <CardContent>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t("caseDetail.overview.type")}</TableHead>
                        <TableHead className="text-right">{t("caseDetail.visual.chart.count")}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {observablesByTypeChartData.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={2} className="text-center py-6 text-[#9ca3af]">
                            {t("caseDetail.visual.empty")}
                          </TableCell>
                        </TableRow>
                      ) : (
                        observablesByTypeChartData.map((item) => {
                          const isSelected = visualTypeFilter !== "all" && String(visualTypeFilter) === String(item.type);
                          return (
                            <TableRow
                              key={`visual-type-applet-${item.type}`}
                              className={isSelected ? "bg-[rgba(59,130,246,0.12)]" : undefined}
                              data-testid={`visual-type-applet-row-${item.type}`}
                              onClick={() => {
                                setVisualTypeFilter(isSelected ? "all" : item.type);
                              }}
                            >
                              <TableCell className="font-medium">{item.type}</TableCell>
                              <TableCell className="text-right">{item.value}</TableCell>
                            </TableRow>
                          );
                        })
                      )}
                    </TableBody>
                  </Table>
                </CardContent>
              </Card>
            </div>

            <Card className={CASE_SUBPANEL_CLASS}>
              <CardHeader>
                <CardTitle className="text-sm">{t("caseDetail.visual.applets.observables")}</CardTitle>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("caseDetail.visual.table.source")}</TableHead>
                      <TableHead>{t("caseDetail.overview.type")}</TableHead>
                      <TableHead>{t("caseDetail.observable.value")}</TableHead>
                      <TableHead>{t("caseDetail.observable.verdict")}</TableHead>
                      <TableHead>{t("caseDetail.observable.tagsWithEnter")}</TableHead>
                      <TableHead className="w-[72px] text-right">{t("common.actions")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {filteredVisualizationObservables.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={6} className="text-center py-8 text-[#9ca3af]">
                          {t("caseDetail.visual.empty")}
                        </TableCell>
                      </TableRow>
                    ) : (
                      filteredVisualizationObservables.map((observable: any) => {
                        const observableType = String(observable?.type || "").trim() || "Unknown";
                        const typeKey = observableTypeKey(observableType);
                        const connectorOptions = observableConnectorOptionsByType[typeKey] || [];
                        const rowSelected = visualSourceFilter !== "all"
                          && visualTypeFilter !== "all"
                          && observable.source === visualSourceFilter
                          && String(visualTypeFilter) === observableType;
                        return (
                        <TableRow
                          key={observable.id}
                          data-testid={`visual-observable-row-${observable.id}`}
                          className={rowSelected ? "bg-[rgba(59,130,246,0.12)]" : undefined}
                          onClick={() => {
                            setVisualSourceFilter((observable?.source || "all") as VisualizationSourceFilter);
                            setVisualTypeFilter(observableType);
                          }}
                        >
                          <TableCell>
                            <Badge variant="outline" className="rounded-lg">
                              {visualSourceLabels[observable.source as keyof typeof visualSourceLabels] || t("caseDetail.visual.source.other")}
                            </Badge>
                          </TableCell>
                          <TableCell>{observableType}</TableCell>
                          <TableCell>
                            <code className="rounded bg-[#171b28] px-2 py-1 font-mono text-xs text-[#d1d5db]">{observable.value}</code>
                          </TableCell>
                          <TableCell>
                            <Badge className={`${getAIVerdictBadgeClass(observable.verdict)} text-xs rounded-lg border`}>
                              {t(VERDICT_KEYS[observable.verdict] || observable.verdict)}
                            </Badge>
                          </TableCell>
                          <TableCell>{Array.isArray(observable.tags) && observable.tags.length > 0 ? observable.tags.join(", ") : "—"}</TableCell>
                          <TableCell>
                            <div className="flex justify-end">
                              <ObservableConnectorMenu
                                options={connectorOptions}
                                onSelect={(option) => handleRunConnectorForObservable(observable, option)}
                                emptyLabel={t("alerts.observableConnectors.noMethods")}
                                disabled={executeConnectorHub.isPending}
                                triggerTestId={`button-visual-observable-connectors-${observable.id}`}
                                getItemTestId={(option) => `button-visual-observable-connector-option-${observable.id}-${option.methodId}`}
                              />
                            </div>
                          </TableCell>
                        </TableRow>
                      );
                      })
                    )}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>

            <Card className={CASE_SUBPANEL_CLASS}>
              <CardHeader>
                <CardTitle className="text-sm">{t("caseDetail.visual.applets.files")}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="text-xs text-[#9ca3af]" data-testid="visual-files-summary">
                  {t("caseDetail.visual.files.summary", {
                    images: String(imageAttachmentCount),
                    files: String(fileAttachmentCount),
                  })}
                </div>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("caseDetail.attachment.name")}</TableHead>
                      <TableHead>{t("caseDetail.attachment.type")}</TableHead>
                      <TableHead>{t("caseDetail.attachment.size")}</TableHead>
                      <TableHead>{t("caseDetail.overview.created")}</TableHead>
                      <TableHead>{t("common.actions")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {filteredVisualizationAttachments.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={5} className="text-center py-8 text-[#9ca3af]">
                          {t("caseDetail.visual.empty")}
                        </TableCell>
                      </TableRow>
                    ) : (
                      filteredVisualizationAttachments.map((attachment: any) => (
                        <TableRow key={attachment.id} data-testid={`visual-attachment-row-${attachment.id}`}>
                          <TableCell className="font-medium">{attachment.fileName}</TableCell>
                          <TableCell>
                            <div className="flex flex-wrap items-center gap-2">
                              <span>{attachment.contentType}</span>
                              <Badge variant="outline" className="rounded-md text-[10px] uppercase tracking-wide">
                                {attachment.kind === "image" ? t("caseDetail.attachment.image") : t("caseDetail.attachment.file")}
                              </Badge>
                            </div>
                          </TableCell>
                          <TableCell>{Math.max(1, Math.round((attachment.fileSizeBytes || 0) / 1024))} KB</TableCell>
                          <TableCell>{attachment.createdAt ? format(new Date(attachment.createdAt), "dd.MM.yyyy HH:mm") : "—"}</TableCell>
                          <TableCell>
                            <Button
                              variant="outline"
                              size="sm"
                              className="rounded-lg h-8 gap-1"
                              onClick={() => handleAttachmentDownload(attachment.id)}
                              disabled={downloadAttachment.isPending}
                              data-testid={`button-visual-attachment-open-${attachment.id}`}
                            >
                              <Download size={12} />
                              {attachment.kind === "image" ? t("caseDetail.attachment.preview") : t("caseDetail.attachment.open")}
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </TabsContent>

          {/* Attachments Tab */}
          <TabsContent value="attachments" className="mt-6 space-y-6" data-testid="tab-content-attachments">
            <Card className={CASE_PANEL_CLASS}>
              <CardHeader>
                <CardTitle className="text-sm font-semibold uppercase tracking-[0.12em] text-[#9ca3af] flex items-center gap-2">
                  <Paperclip size={16} className="text-primary" /> {t("caseDetail.attachment.title", { count: String(attachmentCount) })}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <input
                  ref={attachmentInputRef}
                  type="file"
                  className="hidden"
                  onChange={handleAttachmentInputChange}
                  data-testid="input-case-attachment"
                />
                <div className="flex items-center justify-between gap-3">
                  <p className="text-sm text-[#9ca3af]">{t("caseDetail.attachment.subtitle")}</p>
                  <Button
                    className="rounded-xl gap-2 font-bold"
                    onClick={() => attachmentInputRef.current?.click()}
                    disabled={uploadAttachment.isPending}
                    data-testid="button-upload-case-attachment"
                  >
                    <Paperclip size={14} />
                    {uploadAttachment.isPending ? t("caseDetail.attachment.uploading") : t("caseDetail.attachment.upload")}
                  </Button>
                </div>

                <Card className={CASE_SUBPANEL_CLASS}>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t("caseDetail.attachment.name")}</TableHead>
                        <TableHead>{t("caseDetail.attachment.type")}</TableHead>
                        <TableHead>{t("caseDetail.attachment.size")}</TableHead>
                        <TableHead>{t("caseDetail.overview.created")}</TableHead>
                        <TableHead className="w-24">{t("common.actions")}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {attachments.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={5} className="text-center py-12 text-[#9ca3af]" data-testid="attachments-empty">
                            <Paperclip size={32} className="mx-auto mb-3 opacity-50" />
                            <p className="text-sm">{t("caseDetail.attachment.empty")}</p>
                          </TableCell>
                        </TableRow>
                      ) : (
                        attachments.map((attachment: any) => (
                          <TableRow key={attachment.id} data-testid={`attachment-row-${attachment.id}`}>
                            <TableCell className="font-medium">{attachment.fileName}</TableCell>
                            <TableCell>
                              <div className="flex flex-wrap items-center gap-2">
                                <span>{attachment.contentType}</span>
                                <Badge variant="outline" className="rounded-md text-[10px] uppercase tracking-wide">
                                  {isImageAttachment(attachment.contentType, attachment.fileName)
                                    ? t("caseDetail.attachment.image")
                                    : t("caseDetail.attachment.file")}
                                </Badge>
                              </div>
                            </TableCell>
                            <TableCell>{Math.max(1, Math.round((attachment.fileSizeBytes || 0) / 1024))} KB</TableCell>
                            <TableCell>{attachment.createdAt ? format(new Date(attachment.createdAt), "dd.MM.yyyy HH:mm") : "—"}</TableCell>
                            <TableCell>
                              <Button
                                variant="outline"
                                size="sm"
                                className="h-8 rounded-lg gap-1 border-[#2a2c3c] bg-[#0f1118] text-[#d1d5db] hover:bg-[#1a1f2d]"
                                onClick={() => handleAttachmentDownload(attachment.id)}
                                disabled={downloadAttachment.isPending}
                                data-testid={`button-download-attachment-${attachment.id}`}
                              >
                                <Download size={12} /> {isImageAttachment(attachment.contentType, attachment.fileName) ? t("caseDetail.attachment.preview") : t("caseDetail.attachment.open")}
                              </Button>
                            </TableCell>
                          </TableRow>
                        ))
                      )}
                    </TableBody>
                  </Table>
                </Card>
              </CardContent>
            </Card>
          </TabsContent>


          <CaseCommentsTab
            t={t}
            commentCount={commentCount}
            comments={comments}
            currentUser={currentUser}
            newComment={newComment}
            setNewComment={setNewComment}
            onAddComment={handleAddComment}
            panelClass={CASE_PANEL_CLASS}
          />

          {/* AI Workloads Tab */}
          <TabsContent value="ai" className="mt-6 space-y-6" data-testid="tab-content-ai">
            <CaseAIWorkloadsTab
              trace={caseAITrace}
              isLoading={caseAITraceLoading}
              isFetching={caseAITraceFetching}
              focusedWorkloadId={aiFocusWorkloadId}
              focusedStageId={aiFocusStageId}
              onRefresh={() => {
                void refetchCaseAITrace();
              }}
              canManageWorkloads={canManageAIWorkloads}
              pendingWorkloadAction={pendingAITraceAction}
              onRestartWorkload={(workloadId) => handleAITraceWorkloadAction(workloadId, "restart")}
              onCloseWorkload={(workloadId) => handleAITraceWorkloadAction(workloadId, "close")}
              onConnectorExecutionClick={(executionId) => {
                setSelectedConnectorExecutionID(executionId);
                setConnectorExecutionDrawerOpen(true);
              }}
              renderExtraActions={({ workload, runResult }) => {
                const isAlreadyAssignedToCurrent = String(caseData?.assignee || "").trim() === currentUserId;
                return (
                  <>
                    {!isAlreadyAssignedToCurrent && currentUserId ? (
                      <Button
                        type="button"
                        size="sm"
                        className="border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db] hover:bg-[#171b2a] hover:text-white"
                        data-testid={`case-ai-workloads-assign-${String(workload?.id || "")}`}
                        onClick={handleAssignCurrentAnalystFromAITrace}
                      >
                        Assign to me
                      </Button>
                    ) : null}
                    <Button
                      type="button"
                      size="sm"
                      className="border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db] hover:bg-[#171b2a] hover:text-white"
                      data-testid={`case-ai-workloads-escalate-${String(workload?.id || "")}`}
                      onClick={() => handleEscalateFromAITrace({ workload, runResult })}
                    >
                      Escalate
                    </Button>
                  </>
                );
              }}
            />
          </TabsContent>

          <CaseMitreTab
            t={t}
            caseData={caseData}
            mitreDialogOpen={mitreDialogOpen}
            setMitreDialogOpen={setMitreDialogOpen}
            onAddMitre={handleAddMitre}
            onRemoveMitre={handleRemoveMitre}
            panelClass={CASE_PANEL_CLASS}
            subpanelClass={CASE_SUBPANEL_CLASS}
          />

          <CaseConnectorsTab
            outboundConnectors={outboundConnectors}
            connectorHubConnectorID={connectorHubConnectorID}
            setConnectorHubConnectorID={setConnectorHubConnectorID}
            connectorHubMethods={connectorHubMethods}
            connectorHubDraft={connectorHubDraft}
            setConnectorHubDraft={setConnectorHubDraft}
            selectedConnectorHubMethod={selectedConnectorHubMethod}
            connectorHubRuns={connectorHubRuns}
            executePending={executeConnectorHub.isPending}
            onRun={handleRunConnectorHubForCase}
            onOpenExecution={(executionID) => {
              setSelectedConnectorExecutionID(executionID);
              setConnectorExecutionDrawerOpen(true);
            }}
            panelClass={CASE_PANEL_CLASS}
            inputClass={CASE_INPUT_CLASS}
            selectContentClass={CASE_SELECT_CONTENT_CLASS}
          />

          <TabsContent value="communications" className="space-y-4">
            <CaseCommunicationsTab
              caseId={id || ""}
              tenantId={currentTenantId}
              currentUserId={currentUserId}
              currentUserName={currentUser?.name}
            />
          </TabsContent>
        </Tabs>
        <ConnectorExecutionDrawer
          executionId={selectedConnectorExecutionID}
          open={connectorExecutionDrawerOpen && !!selectedConnectorExecutionID}
          onOpenChange={(nextOpen) => {
            setConnectorExecutionDrawerOpen(nextOpen);
            if (!nextOpen) {
              setSelectedConnectorExecutionID("");
            }
          }}
          title="Case Connector Execution"
          description="Durable connector execution details for this case, including attempts, retries, and live event log."
        />
      </div>
    </AppLayout>
  );
}
