import { useMemo, useState } from "react";
import { Link, useLocation, useParams } from "wouter";
import { toast } from "sonner";
import {
  AlertCircle,
  ArrowLeft,
  Bell,
  CheckCircle2,
  Circle,
  Clock3,
  FileWarning,
  History,
  Link2,
  Network,
  Plus,
  Plug,
  Server,
  Trash2,
  UserRound,
  Zap,
} from "lucide-react";
import { AppLayout } from "@/components/layout";
import { AIEntityTrace } from "@/components/ai-entity-trace";
import { ConnectorExecutionDrawer } from "@/components/connector-execution-drawer";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import {
  useAlert,
  useAlertsPage,
  useAppState,
  useAIAgentEntityTrace,
  useRestartAIAgentWorkload,
  useCloseAIAgentWorkload,
  useBindAlertsToCase,
  useCases,
  useCreateCaseFromAlerts,
  useDeleteAlert,
  useUpdateAlert,
  useUser,
  useOutboundConnectors,
  useTenantConnectorMethods,
  useExecuteConnectorHub,
  useConnectorHubExecutions,
} from "@/lib/api";
import { useI18n, useT } from "@/lib/i18n";
import { UserAvatar } from "@/components/user-avatar";
import { withTenantPath } from "@/lib/tenant-url";
import { splitLocationPathAndSearch } from "@/lib/url-state";
import { useMinimumLoading } from "@/lib/use-minimum-loading";
import { buildObservableConnectorOptionsByType, normalizeObservableConnectorType } from "@/lib/connectors";
import { ObservableConnectorMenu } from "@/features/connectors";

const PAGE_SHELL_CLASS = "mx-auto w-full max-w-[1144px] space-y-6 pb-6";
const PANEL_CLASS =
  "rounded-2xl border border-[rgba(255,255,255,0.06)] bg-[linear-gradient(180deg,rgba(19,20,28,0.97),rgba(17,20,32,0.97))] shadow-[0_14px_34px_rgba(0,0,0,0.28)]";
const PANEL_BORDER_CLASS = "border-b border-[#2a2c3c]";
const MUTED_LABEL_CLASS = "text-[12px] font-normal leading-[15px] tracking-[0.1px] text-[#8b91a3]";
const OUTLINE_BUTTON_CLASS = "border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]";
const SELECT_TRIGGER_CLASS = "h-11 rounded-lg border border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] focus:ring-1 focus:ring-[#3b4a79]";
const SELECT_CONTENT_CLASS = "rounded-xl border border-[#2a2c3c] bg-[#13141c] text-white";
const ALERT_AI_TRACE_REFRESH_MS = 2000;

const SEVERITY_BADGE_CLASS: Record<string, string> = {
  critical: "border border-[rgba(239,68,68,0.2)] bg-[rgba(239,68,68,0.1)] text-[#f87171]",
  high: "border border-[rgba(245,158,11,0.25)] bg-[rgba(245,158,11,0.12)] text-[#f59e0b]",
  medium: "border border-[rgba(59,130,246,0.25)] bg-[rgba(59,130,246,0.12)] text-[#60a5fa]",
  low: "border border-[rgba(34,197,94,0.25)] bg-[rgba(34,197,94,0.12)] text-[#4ade80]",
};

const STATUS_BADGE_CLASS: Record<string, string> = {
  new: "border border-[rgba(234,179,8,0.2)] bg-[rgba(234,179,8,0.1)] text-[#facc15]",
  triaged: "border border-[rgba(59,130,246,0.2)] bg-[rgba(59,130,246,0.1)] text-[#60a5fa]",
  closed: "border border-[rgba(34,197,94,0.2)] bg-[rgba(34,197,94,0.1)] text-[#4ade80]",
};

function normalizeKey(value: string): string {
  return String(value || "").trim().toLowerCase();
}

function formatDate(value: string, locale: string): string {
  const timestamp = Date.parse(String(value || ""));
  if (!Number.isFinite(timestamp)) return "-";
  return new Date(timestamp).toLocaleDateString(locale, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

function formatTime(value: string, locale: string): string {
  const timestamp = Date.parse(String(value || ""));
  if (!Number.isFinite(timestamp)) return "-";
  return new Date(timestamp).toLocaleTimeString(locale, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
    timeZoneName: "short",
  });
}

function formatRelative(value: string, locale: string): string {
  const timestamp = Date.parse(String(value || ""));
  const isRu = locale.startsWith("ru");
  if (!Number.isFinite(timestamp)) return isRu ? "только что" : "just now";

  const diffSeconds = Math.max(1, Math.floor((Date.now() - timestamp) / 1000));
  if (diffSeconds < 60) return isRu ? `${diffSeconds}с назад` : `${diffSeconds}s ago`;

  const diffMinutes = Math.floor(diffSeconds / 60);
  if (diffMinutes < 60) return isRu ? `${diffMinutes}м назад` : `${diffMinutes}m ago`;

  const diffHours = Math.floor(diffMinutes / 60);
  if (diffHours < 24) return isRu ? `${diffHours}ч назад` : `${diffHours}h ago`;

  const diffDays = Math.floor(diffHours / 24);
  return isRu ? `${diffDays}д назад` : `${diffDays}d ago`;
}

function stableHash(value: string): number {
  let hash = 0;
  for (let i = 0; i < value.length; i += 1) {
    hash = (hash << 5) - hash + value.charCodeAt(i);
    hash |= 0;
  }
  return Math.abs(hash);
}

function confidenceFromSeverity(severity: string): number {
  const key = normalizeKey(severity);
  if (key === "critical") return 95;
  if (key === "high") return 88;
  if (key === "medium") return 76;
  return 64;
}

function extractIp(description: string): string {
  const match = String(description || "").match(/\b\d{1,3}(?:\.\d{1,3}){3}\b/);
  return match ? match[0] : "-";
}

function extractHost(title: string, description: string): string {
  const text = `${title || ""} ${description || ""}`;
  const match = text.match(/\b[A-Z]{2,}(?:-[A-Z0-9]{2,})+\b/);
  return match ? match[0] : "-";
}

function pickProcessName(title: string, description: string): string {
  const match = `${title || ""} ${description || ""}`.match(/\b[a-z0-9_.-]+\.exe\b/i);
  if (match) return match[0].toLowerCase();
  return "unknown.exe";
}

type DerivedAlertObservable = {
  id: string;
  type: string;
  typeKey: string;
  value: string;
};

function extractUrl(text: string): string {
  const match = String(text || "").match(/https?:\/\/[^\s)]+/i);
  return match ? match[0] : "";
}

function extractEmail(text: string): string {
  const match = String(text || "").match(/[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}/i);
  return match ? match[0] : "";
}

function extractHash(text: string): string {
  const sha256 = String(text || "").match(/\b[a-f0-9]{64}\b/i);
  if (sha256) return sha256[0];
  const md5 = String(text || "").match(/\b[a-f0-9]{32}\b/i);
  return md5 ? md5[0] : "";
}

function buildAlertDerivedObservables(alertData: any): DerivedAlertObservable[] {
  if (!alertData) return [];
  const title = String(alertData?.title || "");
  const description = String(alertData?.description || "");
  const candidates: Array<{ type: string; typeKey: string; value: string }> = [
    { type: "IP", typeKey: normalizeObservableConnectorType("ip"), value: extractIp(description) },
    { type: "Host", typeKey: normalizeObservableConnectorType("host"), value: extractHost(title, description) },
    { type: "Process", typeKey: normalizeObservableConnectorType("process"), value: pickProcessName(title, description) },
    { type: "URL", typeKey: normalizeObservableConnectorType("url"), value: extractUrl(`${title} ${description}`) },
    { type: "Email", typeKey: normalizeObservableConnectorType("email"), value: extractEmail(`${title} ${description}`) },
    { type: "Hash", typeKey: normalizeObservableConnectorType("hash"), value: extractHash(`${title} ${description}`) },
  ];

  const seen = new Set<string>();
  return candidates
    .filter((item) => item.typeKey && item.value && item.value !== "-" && item.value !== "unknown.exe")
    .filter((item) => {
      const key = `${item.typeKey}:${item.value}`.toLowerCase();
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    })
    .map((item) => ({
      id: `${item.typeKey}:${item.value}`.toLowerCase(),
      type: item.type,
      typeKey: item.typeKey,
      value: item.value,
    }));
}

function timelineToneClasses(index: number): { ring: string; icon: string } {
  if (index === 0) return { ring: "border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.12)]", icon: "text-[#f87171]" };
  if (index === 1) return { ring: "border-[rgba(59,130,246,0.35)] bg-[rgba(59,130,246,0.12)]", icon: "text-[#60a5fa]" };
  if (index === 2) return { ring: "border-[rgba(168,85,247,0.35)] bg-[rgba(168,85,247,0.12)]", icon: "text-[#c084fc]" };
  return { ring: "border-[rgba(102,255,76,0.35)] bg-[rgba(102,255,76,0.12)]", icon: "text-[#66ff4c]" };
}

function connectorExecutionStatusClass(status: string): string {
  switch (String(status || "").toLowerCase()) {
    case "completed":
      return "border-[rgba(34,197,94,0.35)] bg-[rgba(34,197,94,0.16)] text-[#86efac]";
    case "dry_run":
    case "provider_accepted":
      return "border-[rgba(59,130,246,0.35)] bg-[rgba(59,130,246,0.16)] text-[#93c5fd]";
    case "accepted":
    case "queued":
    case "dispatching":
    case "retry_scheduled":
      return "border-[rgba(245,158,11,0.35)] bg-[rgba(245,158,11,0.16)] text-[#fbbf24]";
    case "failed":
    case "cancelled":
    case "dead_letter":
      return "border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.16)] text-[#fda4af]";
    default:
      return "border-[#2a2c3c] bg-[#0f1118] text-[#d1d5db]";
  }
}

export default function AlertDetailPage() {
  const t = useT();
  const { language } = useI18n();
  const activeLocale = typeof language === "string" && language.trim().length > 0 ? language : "en";
  const [location, setLocation] = useLocation();
  const { id } = useParams<{ id: string }>();
  const { currentTenantId, currentTenantSlug, currentUserId, session } = useAppState();

  const { data: alertData, isLoading } = useAlert(id || "");
  const {
    data: alertAITrace,
    isLoading: alertAITraceLoading,
    isFetching: alertAITraceFetching,
    dataUpdatedAt: alertAITraceUpdatedAt,
    refetch: refetchAlertAITrace,
  } = useAIAgentEntityTrace("alert", id || "", currentTenantId, 8, {
    refetchInterval: ALERT_AI_TRACE_REFRESH_MS,
  });
  const restartAIAgentWorkload = useRestartAIAgentWorkload();
  const closeAIAgentWorkload = useCloseAIAgentWorkload();
  const { data: currentUser } = useUser(currentUserId);
  const { data: ownerUser } = useUser(alertData?.owner || "");
  const { data: cases = [] } = useCases(currentTenantId);
  const { data: relatedAlertsData } = useAlertsPage(currentTenantId, 1, 50, "all", "", "updated_at", "desc");
  const { data: outboundConnectors = [] } = useOutboundConnectors(currentTenantId);
  const { data: tenantConnectorMethods = [] } = useTenantConnectorMethods(currentTenantId);
  const { data: alertConnectorRuns = [] } = useConnectorHubExecutions(undefined, {
    alertId: id || "",
    limit: 8,
  });

  const updateAlert = useUpdateAlert();
  const bindAlertsToCase = useBindAlertsToCase();
  const createCaseFromAlerts = useCreateCaseFromAlerts();
  const deleteAlert = useDeleteAlert();

  const [targetCaseId, setTargetCaseId] = useState("");
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [selectedConnectorExecutionID, setSelectedConnectorExecutionID] = useState("");
  const [connectorExecutionDrawerOpen, setConnectorExecutionDrawerOpen] = useState(false);
  const [pendingAITraceAction, setPendingAITraceAction] = useState<{ workloadId: string; action: "restart" | "close" } | null>(null);
  const showAlertDetailSkeleton = useMinimumLoading(isLoading);
  const currentTenantMembership = useMemo(
    () => (Array.isArray(session?.memberships) ? session.memberships : []).find((membership: any) => membership?.tenant_id === currentTenantId && membership?.is_active),
    [currentTenantId, session?.memberships],
  );
  const canManageAIWorkloads = Boolean(session?.identity?.is_platform_admin || currentTenantMembership?.role === "tenant_admin");
  const aiTraceLabels = useMemo(() => ({
    title: t("aiTrace.title"),
    subtitle: t("alerts.ai.subtitle"),
    empty: t("alerts.ai.empty"),
    queueEvent: t("aiTrace.queueEvent"),
    source: t("aiTrace.source"),
    lastUpdated: t("aiTrace.lastUpdated"),
    attempts: t("aiTrace.attempts"),
    execution: t("aiTrace.execution"),
    stage: t("aiTrace.stage"),
    verdict: t("aiTrace.verdict"),
    blockers: t("aiTrace.blockers"),
    error: t("aiTrace.error"),
    noWorkloads: t("aiTrace.noWorkloads"),
    stageTimeline: t("aiTrace.stageTimeline"),
    connectorTimeline: t("aiTrace.connectorTimeline"),
    connector: t("aiTrace.connector"),
    duration: t("aiTrace.duration"),
    reply: t("aiTrace.reply"),
    caseTags: t("aiTrace.caseTags"),
    restart: t("aiTrace.restart"),
    close: t("aiTrace.close"),
    liveEventLog: t("aiTrace.liveEventLog"),
    liveEventLogSubtitle: t("aiTrace.liveEventLogSubtitle"),
    live: t("aiTrace.live"),
    refreshing: t("aiTrace.refreshing"),
    refresh: t("aiTrace.refresh"),
    waitingForUpdate: t("aiTrace.waitingForUpdate"),
  }), [t]);
  const aiTraceFocus = useMemo(() => {
    const { params } = splitLocationPathAndSearch(location);
    return {
      workloadId: String(params.get("ai_workload") || "").trim(),
      stageKey: String(params.get("ai_stage") || "").trim(),
    };
  }, [location]);

  const availableCases = useMemo(
    () => cases.filter((item: any) => item.id !== alertData?.caseId),
    [cases, alertData?.caseId],
  );

  const linkedCase = useMemo(
    () => cases.find((item: any) => item.id === alertData?.caseId),
    [cases, alertData?.caseId],
  );
  const alertObservables = useMemo(() => buildAlertDerivedObservables(alertData), [alertData]);
  const observableConnectorOptionsByType = useMemo(
    () => buildObservableConnectorOptionsByType(outboundConnectors || [], tenantConnectorMethods || []),
    [outboundConnectors, tenantConnectorMethods],
  );
  const executeConnectorHub = useExecuteConnectorHub();

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

  const relatedAlerts = useMemo(() => {
    if (!alertData?.id) return [];
    const items = relatedAlertsData?.items || [];
    const matched = items.filter((item: any) => {
      if (!item?.id || item.id === alertData.id) return false;
      if (alertData.caseId && item.caseId && item.caseId === alertData.caseId) return true;
      if (item.source && item.source === alertData.source) return true;
      if (item.sev && item.sev === alertData.sev) return true;
      return false;
    });
    return matched.slice(0, 3);
  }, [alertData?.caseId, alertData?.id, alertData?.sev, alertData?.source, relatedAlertsData?.items]);

  const detectionRule = useMemo(() => {
    if (!alertData) return "-";
    const source = String(alertData.source || "alert").trim().toUpperCase();
    const titlePart = String(alertData.title || "rule")
      .trim()
      .replace(/[^a-z0-9]+/gi, "_")
      .replace(/^_+|_+$/g, "")
      .slice(0, 32)
      .toLowerCase();
    return `${source}_${titlePart || "rule"}`;
  }, [alertData]);

  const confidenceScore = useMemo(
    () => confidenceFromSeverity(String(alertData?.sev || "")),
    [alertData?.sev],
  );

  const filesAffected = useMemo(() => {
    if (!alertData) return 0;
    const fromText = String(alertData.description || "").match(/(\d+)\s+files?/i);
    if (fromText) return Number(fromText[1]);
    return 50 + (stableHash(`${alertData.id}:${alertData.source}`) % 250);
  }, [alertData]);

  const processId = useMemo(() => {
    if (!alertData) return 0;
    return 1000 + (stableHash(`${alertData.id}:${alertData.title}`) % 9000);
  }, [alertData]);

  const timeline = useMemo(() => {
    if (!alertData) return [];
    const createdAt = alertData.createdAt || alertData.time;
    const events: Array<{ id: string; title: string; description: string; at: string; icon: any }> = [
      {
        id: "created",
        title: t("alerts.timeline.created"),
        description: t("alerts.timeline.createdDescription", {
          source: String(alertData.source || t("alerts.detail.unknown")),
        }),
        at: createdAt,
        icon: AlertCircle,
      },
    ];

    if (alertData.owner) {
      events.push({
        id: "assigned",
        title: t("alerts.timeline.assigned", {
          user: String(ownerUser?.name || ownerUser?.email || alertData.owner),
        }),
        description: t("alerts.timeline.assignedDescription"),
        at: alertData.updatedAt || createdAt,
        icon: UserRound,
      });
    }

    if (alertData.caseId) {
      events.push({
        id: "linked",
        title: t("alerts.timeline.linked", {
          case: String(linkedCase?.caseNumber || alertData.caseId),
        }),
        description: t("alerts.timeline.linkedDescription"),
        at: alertData.updatedAt || createdAt,
        icon: Link2,
      });
    }

    if (alertData.updatedAt && alertData.updatedAt !== createdAt) {
      events.push({
        id: "updated",
        title: t("alerts.timeline.updated"),
        description: t("alerts.timeline.updatedDescription"),
        at: alertData.updatedAt,
        icon: CheckCircle2,
      });
    }

    return events.slice(0, 4);
  }, [alertData, linkedCase?.caseNumber, ownerUser?.email, ownerUser?.name, t]);

  const handleRunConnectorForAlertObservable = (
    observable: DerivedAlertObservable,
    option: { connectorId: string; methodId: string; action: string },
  ) => {
    if (!id) return;
    const value = String(observable?.value || "").trim();
    if (!value) {
      toast.error("Observable value is empty");
      return;
    }

    executeConnectorHub.mutate(
      {
        connector_id: option.connectorId,
        method_id: option.methodId,
        action: option.action || undefined,
        alert_id: id,
        message: undefined,
        input: {
          alert_id: id,
          observable_id: observable.id,
          observable_type: observable.typeKey,
          indicator: value,
          observable: {
            id: observable.id,
            type: observable.type,
            value,
          },
        },
        metadata: {
          source: "alert_observable",
          alert_id: id,
          observable_id: observable.id,
          observable_type: observable.typeKey,
          connector_method_id: option.methodId,
        },
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
            toast.success(t("alerts.connectorExecution.completed"));
            return;
          }
          if (
            runStatus === "accepted" ||
            runStatus === "queued" ||
            runStatus === "dispatching" ||
            runStatus === "retry_scheduled" ||
            runStatus === "provider_accepted"
          ) {
            toast.success(t("caseDetail.toast.connectorQueued"));
            return;
          }
          toast.error(result?.error || t("caseDetail.toast.connectorRunFailed"));
        },
        onError: (error: any) => {
          toast.error(error?.message || t("caseDetail.toast.connectorRunFailed"));
        },
      },
    );
  };

  const handleAssignToMe = () => {
    if (!alertData?.id || !currentUser?.id) return;
    updateAlert.mutate(
      { id: alertData.id, data: { owner: currentUser.id, status: "Triaged" } },
      { onSuccess: () => toast.success(t("alerts.toast.assigned")) },
    );
  };

  const handleBindToCase = () => {
    if (!alertData?.id) return;
    if (!targetCaseId) {
      toast.error(t("alerts.bulk.selectCaseRequired"));
      return;
    }
    bindAlertsToCase.mutate(
      { alertIds: [alertData.id], caseId: targetCaseId },
      {
        onSuccess: () => {
          toast.success(t("alerts.bulk.bindSuccess"));
        },
      },
    );
  };

  const handleCreateCase = () => {
    if (!alertData?.id) return;
    createCaseFromAlerts.mutate(
      {
        alertIds: [alertData.id],
        case: {},
      },
      {
        onSuccess: (payload: any) => {
          toast.success(t("alerts.bulk.createCaseSuccess"));
          if (payload?.case?.id) {
            setLocation(withTenantPath(currentTenantSlug, `/cases/${payload.case.id}`));
          }
        },
      },
    );
  };

  const handleOpenLinkedCaseFromAITrace = () => {
    if (!alertData?.caseId) return;
    setLocation(`${withTenantPath(currentTenantSlug, `/cases/${alertData.caseId}`)}?tab=ai`);
  };

  const handleDeleteAlert = () => {
    if (!alertData?.id) return;
    deleteAlert.mutate(alertData.id, {
      onSuccess: () => {
        toast.success(t("alerts.toast.deleted"));
        setLocation(withTenantPath(currentTenantSlug, "/alerts"));
      },
      onError: (error: any) => {
        toast.error(error?.message || t("alerts.toast.deleteFailed"));
      },
    });
  };

  if (showAlertDetailSkeleton) {
    return (
      <AppLayout>
        <div className={PAGE_SHELL_CLASS}>
          <Skeleton className="h-20 w-full rounded-xl" />
          <Skeleton className="h-[480px] w-full rounded-xl" />
          <Skeleton className="h-[260px] w-full rounded-xl" />
        </div>
      </AppLayout>
    );
  }

  if (!alertData) {
    return (
      <AppLayout>
        <div className={`${PAGE_SHELL_CLASS} flex min-h-[70vh] flex-col items-center justify-center gap-4`}>
          <Bell size={44} className="text-[#6b7280]" />
          <h2 className="text-xl font-medium text-white">{t("alerts.notFoundTitle")}</h2>
          <Link href={withTenantPath(currentTenantSlug, "/alerts")}>
            <Button variant="outline" className={`rounded-xl ${OUTLINE_BUTTON_CLASS}`}>
              <ArrowLeft size={14} className="mr-1" />
              {t("alerts.backToList")}
            </Button>
          </Link>
        </div>
      </AppLayout>
    );
  }

  const severityKey = normalizeKey(String(alertData.sev || ""));
  const statusKey = normalizeKey(String(alertData.statusCode || alertData.status || ""));
  const createdAt = alertData.createdAt || alertData.time;
  const updatedAt = alertData.updatedAt || alertData.time;
  const host = extractHost(alertData.title, alertData.description);
  const ipAddress = extractIp(alertData.description);

  return (
    <AppLayout>
      <div className={PAGE_SHELL_CLASS} data-testid="alert-detail-page">
        <section className="space-y-2">
          <div className="flex items-center gap-4">
            <Link href={withTenantPath(currentTenantSlug, "/alerts")}>
              <Button
                variant="outline"
                size="icon"
                className={`h-9 w-8 rounded-lg ${OUTLINE_BUTTON_CLASS}`}
                data-testid="button-alert-detail-back"
              >
                <ArrowLeft size={14} />
              </Button>
            </Link>
            <div className="flex flex-wrap items-center gap-2">
              <span className="font-mono text-[12px] text-[#6b7280]">{alertData.id}</span>
              <span className={`inline-flex h-[22px] items-center rounded px-2 text-[12px] font-medium ${SEVERITY_BADGE_CLASS[severityKey] || SEVERITY_BADGE_CLASS.low}`}>
                {alertData.sev}
              </span>
            </div>
          </div>
          <h1 className="text-[30px] font-semibold leading-9 tracking-[-0.5px] text-white">{alertData.title}</h1>
          <p className="text-sm text-[#9ca3af]">{t("alerts.card.createdAt")} {formatRelative(createdAt, activeLocale)}</p>
        </section>

        <div className="grid gap-6 xl:grid-cols-[minmax(0,738px)_minmax(0,357px)]">
          <div className="space-y-4">
            <Card className={`${PANEL_CLASS} p-0`}>
              <div className={`${PANEL_BORDER_CLASS} px-5 py-5`}>
                <h2 className="flex items-center gap-2 text-sm font-semibold uppercase tracking-[0.12em] text-[#9ca3af]">
                  <FileWarning size={16} className="text-[#66ff4c]" />
                  {t("alerts.detail.summary")}
                </h2>
              </div>
              <div className="space-y-4 px-5 py-5">
                <div>
                  <p className={MUTED_LABEL_CLASS}>{t("alerts.detail.description")}</p>
                  <p className="mt-2 whitespace-pre-wrap text-[14px] leading-[23px] tracking-[-0.5px] text-[#d1d5db]">
                    {alertData.description || t("alerts.noDescription")}
                  </p>
                </div>

                <div className="border-t border-[#2a2c3c] pt-4">
                  <div className="grid gap-3 sm:grid-cols-2">
                    <div className="rounded-lg px-3 py-2">
                      <p className={MUTED_LABEL_CLASS}>{t("alerts.detail.source")}</p>
                      <span className="mt-1 inline-flex h-[26px] items-center gap-1 rounded border border-[rgba(168,85,247,0.2)] bg-[rgba(168,85,247,0.1)] px-2 text-[12px] text-[#c084fc]">
                        <Circle size={7} className="fill-current" />
                        {String(alertData.source || t("alerts.detail.unknown")).toUpperCase()}
                      </span>
                    </div>
                    <div className="rounded-lg px-3 py-2">
                      <p className={MUTED_LABEL_CLASS}>{t("alerts.filter.status")}</p>
                      <span className={`mt-1 inline-flex h-[26px] items-center gap-1 rounded px-2 text-[12px] ${STATUS_BADGE_CLASS[statusKey] || STATUS_BADGE_CLASS.new}`}>
                        <Circle size={7} className="fill-current" />
                        {String(alertData.status || "new")}
                      </span>
                    </div>
                    <div className="rounded-lg px-3 py-2">
                      <p className={MUTED_LABEL_CLASS}>{t("alerts.detail.assignedTo")}</p>
                      {alertData.owner ? (
                        <div className="mt-1 inline-flex items-center gap-2 text-sm text-white">
                          <UserAvatar
                            name={ownerUser?.name || ownerUser?.email || alertData.owner}
                            avatar={ownerUser?.avatar}
                            fallback={alertData.owner}
                            className="h-6 w-6 border border-[#2a2c3c]"
                            fallbackClassName="bg-[#1f2f4a] text-[9px] font-semibold text-[#d1d5db]"
                          />
                          <span>{ownerUser?.name || ownerUser?.email || alertData.owner}</span>
                        </div>
                      ) : (
                        <span className="mt-1 inline-flex text-sm text-[#9ca3af]">{t("cases.unassigned")}</span>
                      )}
                    </div>
                    <div className="rounded-lg px-3 py-2">
                      <p className={MUTED_LABEL_CLASS}>{t("alerts.detail.linkedCase")}</p>
                      {alertData.caseId ? (
                        <button
                          type="button"
                          className="mt-1 inline-flex h-[26px] items-center gap-1 rounded border border-[rgba(59,130,246,0.2)] bg-[rgba(59,130,246,0.1)] px-2 text-[12px] text-[#60a5fa]"
                          onClick={() => setLocation(withTenantPath(currentTenantSlug, `/cases/${alertData.caseId}`))}
                        >
                          <Link2 size={11} />
                          {linkedCase?.caseNumber || alertData.caseId}
                        </button>
                      ) : (
                        <span className="mt-1 inline-flex text-sm text-[#9ca3af]">{t("alerts.caseNotLinked")}</span>
                      )}
                    </div>
                    <div className="rounded-lg px-3 py-2">
                      <p className={MUTED_LABEL_CLASS}>{t("alerts.detail.host")}</p>
                      <p className="mt-1 flex items-center gap-1 text-sm text-white">
                        <Server size={12} className="text-[#9ca3af]" />
                        {host}
                      </p>
                    </div>
                    <div className="rounded-lg px-3 py-2">
                      <p className={MUTED_LABEL_CLASS}>{t("alerts.detail.ipAddress")}</p>
                      <p className="mt-1 flex items-center gap-1 text-sm text-white">
                        <Network size={12} className="text-[#9ca3af]" />
                        {ipAddress}
                      </p>
                    </div>
                  </div>
                </div>

                <div className="border-t border-[#2a2c3c] pt-4">
                  <p className={MUTED_LABEL_CLASS}>{t("alerts.detail.tags")}</p>
                  <div className="mt-2 flex flex-wrap gap-2">
                    {(Array.isArray(alertData.tags) ? alertData.tags : []).map((tag: string) => (
                      <Badge
                        key={tag}
                        variant="outline"
                        className="h-[26px] border-[#2a2c3c] bg-[#1d1e29] px-2 text-[11px] font-normal text-[#9ca3af]"
                      >
                        <Circle size={7} className="mr-1 fill-current text-[#6b7280]" />
                        {tag}
                      </Badge>
                    ))}
                  </div>
                </div>

                <div className="border-t border-[#2a2c3c] pt-4">
                  <div className="grid gap-3 sm:grid-cols-3">
                    <div className="px-3 py-2">
                      <p className={MUTED_LABEL_CLASS}>{t("alerts.card.createdAt")}</p>
                      <p className="mt-1 text-sm text-white">{formatDate(createdAt, activeLocale)}</p>
                      <p className="text-xs text-[#6b7280]">{formatTime(createdAt, activeLocale)}</p>
                    </div>
                    <div className="px-3 py-2">
                      <p className={MUTED_LABEL_CLASS}>{t("alerts.detail.lastUpdated")}</p>
                      <p className="mt-1 text-sm text-white">{formatDate(updatedAt, activeLocale)}</p>
                      <p className="text-xs text-[#6b7280]">{formatTime(updatedAt, activeLocale)}</p>
                    </div>
                    <div className="px-3 py-2">
                      <p className={MUTED_LABEL_CLASS}>{t("alerts.detail.firstSeen")}</p>
                      <p className="mt-1 text-sm text-white">{formatRelative(createdAt, activeLocale)}</p>
                      <p className="text-xs text-[#6b7280]">{formatTime(createdAt, activeLocale)}</p>
                    </div>
                  </div>
                </div>
              </div>
            </Card>

            <Card className={`${PANEL_CLASS} p-0`}>
              <div className={`${PANEL_BORDER_CLASS} px-5 py-5`}>
                <h2 className="flex items-center gap-2 text-sm font-semibold uppercase tracking-[0.12em] text-[#9ca3af]">
                  <Plug size={16} className="text-[#66ff4c]" />
                  {t("alerts.observableConnectors.title")}
                </h2>
                <p className="mt-2 text-sm text-[#6b7280]">
                  {t("alerts.observableConnectors.subtitle")}
                </p>
              </div>
              <div className="space-y-3 px-5 py-5">
                {alertObservables.length === 0 ? (
                  <p className="text-sm text-[#9ca3af]">{t("alerts.observableConnectors.empty")}</p>
                ) : (
                  alertObservables.map((observable) => {
                    const connectorOptions = observableConnectorOptionsByType[observable.typeKey] || [];
                    return (
                      <div
                        key={observable.id}
                        className="flex items-center gap-3 rounded-xl border border-[#252a3d] bg-[#10131c] px-3 py-3"
                      >
                        <Badge variant="outline" className="rounded-lg border-[#2a2c3c] bg-[#0b0c10] text-[10px] uppercase tracking-[0.14em] text-[#9ca3af]">
                          {observable.type}
                        </Badge>
                        <div className="min-w-0 flex-1">
                          <div className="truncate font-mono text-[13px] text-white">{observable.value}</div>
                          <div className="mt-1 text-[11px] text-[#6b7280]">{observable.typeKey}</div>
                        </div>
                        <ObservableConnectorMenu
                          options={connectorOptions}
                          onSelect={(option) => handleRunConnectorForAlertObservable(observable, option)}
                          emptyLabel={t("alerts.observableConnectors.noMethods")}
                          disabled={executeConnectorHub.isPending}
                          triggerTestId={`button-alert-observable-connectors-${observable.id}`}
                          getItemTestId={(option) => `button-alert-observable-connector-option-${observable.id}-${option.methodId}`}
                        />
                      </div>
                    );
                  })
                )}

                {Array.isArray(alertConnectorRuns) && alertConnectorRuns.length > 0 ? (
                  <div className="border-t border-[#2a2c3c] pt-4">
                    <div className="flex items-center justify-between gap-3">
                      <p className={MUTED_LABEL_CLASS}>{t("alerts.observableConnectors.recent")}</p>
                      <Badge variant="outline" className="rounded-lg text-[10px]">{alertConnectorRuns.length}</Badge>
                    </div>
                    <div className="mt-3 space-y-2">
                      {alertConnectorRuns.slice(0, 4).map((execution: any) => {
                        const executionId = String(execution?.id || execution?.execution_id || "").trim();
                        return (
                          <button
                            key={executionId || `${execution?.connector_id || 'connector'}-${execution?.action || 'run'}`}
                            type="button"
                            className="w-full rounded-xl border border-[#252a3d] bg-[#10131c] px-3 py-2.5 text-left transition-colors hover:bg-[#171a24]"
                            onClick={() => {
                              if (!executionId) return;
                              setSelectedConnectorExecutionID(executionId);
                              setConnectorExecutionDrawerOpen(true);
                            }}
                            data-testid={executionId ? `button-alert-connector-run-${executionId}` : undefined}
                          >
                            <div className="flex items-center justify-between gap-3">
                              <div className="min-w-0">
                                <div className="truncate text-sm font-medium text-white">
                                  {execution?.method_name || execution?.action || executionId || "Connector execution"}
                                </div>
                                <div className="mt-1 text-[11px] text-[#6b7280]">
                                  {execution?.connector_name || execution?.connector_id || "Connector"}
                                  {execution?.action ? ` · ${execution.action}` : ""}
                                </div>
                              </div>
                              <Badge className={`rounded-lg border text-[10px] ${connectorExecutionStatusClass(String(execution?.status || ""))}`}>
                                {execution?.status || "unknown"}
                              </Badge>
                            </div>
                          </button>
                        );
                      })}
                    </div>
                  </div>
                ) : null}
              </div>
            </Card>

            <Card className={`${PANEL_CLASS} p-0`}>
              <div className={`${PANEL_BORDER_CLASS} px-5 py-5`}>
                <h2 className="flex items-center gap-2 text-sm font-semibold uppercase tracking-[0.12em] text-[#9ca3af]">
                  <History size={16} className="text-[#66ff4c]" />
                  {t("alerts.detail.activity")}
                </h2>
              </div>
              <div className="space-y-4 px-5 py-5">
                {timeline.map((event, index) => {
                  const Icon = event.icon;
                  const tone = timelineToneClasses(index);
                  return (
                    <div key={event.id} className="flex gap-4">
                      <div className={`mt-[2px] flex h-10 w-10 items-center justify-center rounded-full border ${tone.ring}`}>
                        <Icon size={15} className={tone.icon} />
                      </div>
                      <div className="min-w-0">
                        <p className="text-sm text-white">
                          {event.title}
                          <span className="ml-2 text-xs text-[#6b7280]">{formatRelative(event.at, activeLocale)}</span>
                        </p>
                        <p className="text-[13px] text-[#9ca3af]">{event.description}</p>
                      </div>
                    </div>
                  );
                })}
              </div>
            </Card>

            <AIEntityTrace
              trace={alertAITrace}
              isLoading={alertAITraceLoading}
              labels={aiTraceLabels}
              compact
              testIdPrefix="alert-ai-trace"
              canManageWorkloads={canManageAIWorkloads}
              pendingWorkloadAction={pendingAITraceAction}
              onRestartWorkload={(workloadId) => handleAITraceWorkloadAction(workloadId, "restart")}
              onCloseWorkload={(workloadId) => handleAITraceWorkloadAction(workloadId, "close")}
              focusWorkloadId={aiTraceFocus.workloadId}
              focusStageKey={aiTraceFocus.stageKey}
              showLiveEventLog
              liveEventLogRefreshing={alertAITraceFetching}
              liveEventLogUpdatedAt={alertAITraceUpdatedAt}
              onRefreshLiveEventLog={() => {
                void refetchAlertAITrace();
              }}
              onConnectorExecutionClick={(executionId) => {
                setSelectedConnectorExecutionID(executionId);
                setConnectorExecutionDrawerOpen(true);
              }}
              renderExtraWorkloadActions={(context) => {
                const workloadId = String(context.workload?.id || "").trim();
                const isAssignedToCurrent = String(alertData?.owner || "").trim() === currentUserId;
                return (
                  <>
                    {!isAssignedToCurrent && currentUserId ? (
                      <Button
                        type="button"
                        size="sm"
                        className={OUTLINE_BUTTON_CLASS}
                        data-testid={`alert-ai-trace-assign-${workloadId}`}
                        onClick={handleAssignToMe}
                      >
                        {t("alerts.assignToMe")}
                      </Button>
                    ) : null}
                    {alertData?.caseId ? (
                      <Button
                        type="button"
                        size="sm"
                        className={OUTLINE_BUTTON_CLASS}
                        data-testid={`alert-ai-trace-open-case-${workloadId}`}
                        onClick={handleOpenLinkedCaseFromAITrace}
                      >
                        {t("alerts.openLinkedCase")}
                      </Button>
                    ) : (
                      <Button
                        type="button"
                        size="sm"
                        className={OUTLINE_BUTTON_CLASS}
                        data-testid={`alert-ai-trace-create-case-${workloadId}`}
                        onClick={handleCreateCase}
                      >
                        {t("alerts.bulk.createCase")}
                      </Button>
                    )}
                  </>
                );
              }}
            />
          </div>

          <div className="space-y-4">
            <Card className={`${PANEL_CLASS} p-0`}>
              <div className={`${PANEL_BORDER_CLASS} px-5 py-5`}>
                <h2 className="flex items-center gap-2 text-sm font-semibold uppercase tracking-[0.12em] text-[#9ca3af]">
                  <Zap size={16} className="text-[#66ff4c]" />
                  {t("alerts.detail.actions")}
                </h2>
              </div>
              <div className="space-y-3 px-5 py-5">
                {!alertData.owner && (
                  <Button
                    className="h-11 w-full rounded-lg border border-[#4adf37] bg-[#4adf37] text-[14px] font-semibold text-[#0b0c10] hover:bg-[#61f44f]"
                    onClick={handleAssignToMe}
                    disabled={updateAlert.isPending}
                    data-testid="button-alert-detail-assign"
                  >
                    <UserRound size={14} className="mr-2" />
                    {t("alerts.assignToMe")}
                  </Button>
                )}

                <Select value={targetCaseId} onValueChange={setTargetCaseId}>
                  <SelectTrigger
                    className={SELECT_TRIGGER_CLASS}
                    data-testid="select-alert-detail-case"
                  >
                    <SelectValue placeholder={t("alerts.bulk.selectCase")} />
                  </SelectTrigger>
                  <SelectContent className={SELECT_CONTENT_CLASS}>
                    {availableCases.map((item: any) => (
                      <SelectItem key={item.id} value={item.id}>
                        {(item.caseNumber || item.id).toString()} · {item.title}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>

                <Button
                  className="h-11 w-full rounded-lg border border-[#3b82f6] bg-[#3b82f6] text-[14px] font-medium text-white hover:bg-[#4f8ff0]"
                  onClick={handleBindToCase}
                  disabled={bindAlertsToCase.isPending}
                  data-testid="button-alert-detail-bind"
                >
                  <Link2 size={14} className="mr-2" />
                  {bindAlertsToCase.isPending ? t("alerts.bulk.binding") : t("alerts.bulk.bindToCase")}
                </Button>

                <Button
                  variant="outline"
                  className={`h-11 w-full rounded-lg text-[14px] font-medium ${OUTLINE_BUTTON_CLASS}`}
                  onClick={handleCreateCase}
                  disabled={createCaseFromAlerts.isPending}
                  data-testid="button-alert-detail-create-case"
                >
                  <Plus size={14} className="mr-2" />
                  {createCaseFromAlerts.isPending ? t("cases.button.creating") : t("alerts.bulk.createCase")}
                </Button>

                <Button
                  variant="outline"
                  className={`h-11 w-full rounded-lg text-[14px] font-medium ${OUTLINE_BUTTON_CLASS}`}
                  onClick={() => alertData.caseId && setLocation(withTenantPath(currentTenantSlug, `/cases/${alertData.caseId}`))}
                  disabled={!alertData.caseId}
                  data-testid="button-alert-detail-open-case"
                >
                  <Link2 size={14} className="mr-2" />
                  {t("alerts.openLinkedCase")}
                </Button>

                <Button
                  variant="outline"
                  className="h-11 w-full rounded-lg border border-[rgba(239,68,68,0.3)] bg-[rgba(239,68,68,0.15)] text-[14px] font-medium text-[#f87171] hover:bg-[rgba(239,68,68,0.2)]"
                  onClick={() => setDeleteDialogOpen(true)}
                  disabled={deleteAlert.isPending}
                  data-testid="button-alert-detail-delete"
                >
                  <Trash2 size={14} className="mr-2" />
                  {t("alerts.detail.delete")}
                </Button>
              </div>
            </Card>

            <Card className={`${PANEL_CLASS} p-0`}>
              <div className={`${PANEL_BORDER_CLASS} px-5 py-5`}>
                <h2 className="flex items-center gap-2 text-sm font-semibold uppercase tracking-[0.12em] text-[#9ca3af]">
                  <CheckCircle2 size={16} className="text-[#66ff4c]" />
                  {t("alerts.detail.metadata")}
                </h2>
              </div>
              <div className="space-y-4 px-5 py-5">
                <div>
                  <p className={MUTED_LABEL_CLASS}>{t("alerts.detail.alertId")}</p>
                  <p className="mt-1 text-sm text-white">{alertData.id}</p>
                </div>
                <div>
                  <p className={MUTED_LABEL_CLASS}>{t("alerts.detail.detectionRule")}</p>
                  <p className="mt-1 text-sm text-white">{detectionRule}</p>
                </div>
                <div>
                  <div className="flex items-center justify-between">
                    <p className={MUTED_LABEL_CLASS}>{t("alerts.detail.confidenceScore")}</p>
                    <p className="text-sm text-white">{confidenceScore}%</p>
                  </div>
                  <div className="mt-2 h-1.5 rounded-full bg-[#0b0c10]">
                    <div
                      className="h-full rounded-full bg-[#ff4d4f]"
                      style={{ width: `${confidenceScore}%` }}
                    />
                  </div>
                </div>
                <div>
                  <p className={MUTED_LABEL_CLASS}>{t("alerts.detail.filesAffected")}</p>
                  <p className="mt-1 text-sm text-white">{filesAffected}</p>
                </div>
                <div>
                  <p className={MUTED_LABEL_CLASS}>{t("alerts.detail.processName")}</p>
                  <p className="mt-1 text-sm text-white">{pickProcessName(alertData.title, alertData.description)}</p>
                </div>
                <div>
                  <p className={MUTED_LABEL_CLASS}>{t("alerts.detail.processId")}</p>
                  <p className="mt-1 text-sm text-white">{processId}</p>
                </div>
              </div>
            </Card>

            <Card className={`${PANEL_CLASS} p-0`}>
              <div className={`${PANEL_BORDER_CLASS} px-5 py-5`}>
                <h2 className="flex items-center gap-2 text-sm font-semibold uppercase tracking-[0.12em] text-[#9ca3af]">
                  <Link2 size={16} className="text-[#66ff4c]" />
                  {t("alerts.detail.related")}
                </h2>
              </div>
              <div className="space-y-2 px-5 py-5">
                {relatedAlerts.length === 0 ? (
                  <p className="text-sm text-[#9ca3af]">{t("alerts.detail.noRelated")}</p>
                ) : (
                  relatedAlerts.map((item: any) => (
                    <button
                      key={item.id}
                      type="button"
                      className="w-full rounded-xl border border-[#2a2c3c] bg-[#0f121b] px-3 py-3 text-left transition-colors hover:bg-[#171a24]"
                      onClick={() => setLocation(withTenantPath(currentTenantSlug, `/alerts/${item.id}`))}
                    >
                      <div className="flex items-center gap-2">
                        <span className="font-mono text-[11px] text-[#6b7280]">{item.id}</span>
                        <span className={`inline-flex h-[18px] items-center rounded px-1.5 text-[10px] ${SEVERITY_BADGE_CLASS[normalizeKey(item.sev)] || SEVERITY_BADGE_CLASS.low}`}>
                          {item.sev}
                        </span>
                      </div>
                      <p className="mt-1 text-sm text-white">{item.title}</p>
                      <p className="mt-1 flex items-center gap-1 text-xs text-[#6b7280]">
                        <Clock3 size={12} />
                        {formatRelative(item.time || item.createdAt, activeLocale)}
                      </p>
                    </button>
                  ))
                )}
              </div>
            </Card>
          </div>
        </div>
      </div>

      <ConnectorExecutionDrawer
        executionId={selectedConnectorExecutionID}
        open={connectorExecutionDrawerOpen && !!selectedConnectorExecutionID}
        onOpenChange={(nextOpen) => {
          setConnectorExecutionDrawerOpen(nextOpen);
          if (!nextOpen) {
            setSelectedConnectorExecutionID("");
          }
        }}
        title={t("alerts.connectorExecution.title")}
        description={t("alerts.connectorExecution.description")}
      />

      <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <AlertDialogContent className="rounded-xl border border-[#2a2c3c] bg-[#13141c] text-white">
          <AlertDialogHeader>
            <AlertDialogTitle>{t("common.delete")}</AlertDialogTitle>
            <AlertDialogDescription>{t("alerts.deleteConfirmOne")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel className={OUTLINE_BUTTON_CLASS}>
              {t("common.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              className="border border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.2)] text-[#f87171] hover:bg-[rgba(239,68,68,0.3)]"
              onClick={handleDeleteAlert}
              disabled={deleteAlert.isPending}
            >
              {t("common.delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </AppLayout>
  );
}
