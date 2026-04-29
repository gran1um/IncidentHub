import { useEffect, useMemo, type ReactNode } from "react";
import { Activity, AlertTriangle, Bot, Clock3, Loader2, RotateCcw, Route, X, ArrowUpRight } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

export type AIEntityTraceLabels = {
  title: string;
  subtitle: string;
  empty: string;
  queueEvent: string;
  source: string;
  lastUpdated: string;
  attempts: string;
  execution: string;
  stage: string;
  verdict: string;
  blockers: string;
  error: string;
  noWorkloads: string;
  stageTimeline: string;
  connectorTimeline: string;
  connector: string;
  duration: string;
  reply: string;
  caseTags: string;
  restart: string;
  close: string;
  liveEventLog: string;
  liveEventLogSubtitle: string;
  live: string;
  refreshing: string;
  refresh: string;
  waitingForUpdate: string;
};

type AIEntityTracePendingAction = {
  workloadId: string;
  action: "restart" | "close";
} | null;

export type AIEntityTraceActionContext = {
  event: any;
  workload: any;
  runResult: any;
  eventIndex: number;
  workloadIndex: number;
};

type AIEntityTraceProps = {
  trace: any;
  isLoading?: boolean;
  labels: AIEntityTraceLabels;
  compact?: boolean;
  testIdPrefix?: string;
  canManageWorkloads?: boolean;
  pendingWorkloadAction?: AIEntityTracePendingAction;
  onRestartWorkload?: (workloadId: string) => void;
  onCloseWorkload?: (workloadId: string) => void;
  renderExtraWorkloadActions?: (context: AIEntityTraceActionContext) => ReactNode;
  focusWorkloadId?: string;
  focusStageKey?: string;
  showLiveEventLog?: boolean;
  liveEventLogRefreshing?: boolean;
  liveEventLogUpdatedAt?: number | string;
  onRefreshLiveEventLog?: () => void;
  onConnectorExecutionClick?: (executionId: string) => void;
};

const PANEL_CLASS = "rounded-2xl border border-[#2a2c3c] bg-[linear-gradient(180deg,rgba(19,20,28,0.96),rgba(15,19,29,0.96))]";
const SUBPANEL_CLASS = "rounded-xl border border-[#23283a] bg-[#101623]";
const MUTED_TEXT_CLASS = "text-xs text-[#8b91a3]";
const ACTION_BUTTON_CLASS = "border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db] hover:bg-[#171b2a] hover:text-white";
const DANGER_BUTTON_CLASS = "border border-[rgba(239,68,68,0.35)] bg-[rgba(127,29,29,0.2)] text-[#fecaca] hover:bg-[rgba(153,27,27,0.32)]";

function formatDateTime(value?: string): string {
  if (!value) return "-";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
}

function formatExecutionPolicy(value?: string): string {
  switch (String(value || "").trim().toLowerCase()) {
    case "exclusive":
      return "Exclusive";
    case "first_match":
      return "First match";
    case "fallback_chain":
      return "Fallback chain";
    default:
      return "All matching";
  }
}

function capitalizeStatus(value?: string): string {
  const normalized = String(value || "").trim().toLowerCase();
  if (!normalized) return "Unknown";
  return normalized.replace(/_/g, " ").replace(/\b\w/g, (char) => char.toUpperCase());
}

function statusBadgeClass(status?: string): string {
  switch (String(status || "").trim().toLowerCase()) {
    case "done":
    case "completed":
      return "border border-[rgba(34,197,94,0.28)] bg-[rgba(34,197,94,0.16)] text-[#86efac]";
    case "partial":
      return "border border-[rgba(245,158,11,0.28)] bg-[rgba(245,158,11,0.14)] text-[#fcd34d]";
    case "failed":
    case "error":
      return "border border-[rgba(239,68,68,0.3)] bg-[rgba(239,68,68,0.16)] text-[#fca5a5]";
    case "processing":
    case "running":
      return "border border-[rgba(59,130,246,0.3)] bg-[rgba(59,130,246,0.14)] text-[#93c5fd]";
    case "queued":
    case "pending":
      return "border border-[rgba(245,158,11,0.28)] bg-[rgba(245,158,11,0.14)] text-[#fcd34d]";
    case "cancelled":
      return "border border-[rgba(148,163,184,0.28)] bg-[rgba(51,65,85,0.4)] text-[#cbd5e1]";
    default:
      return "border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db]";
  }
}

function toArray(value: any): any[] {
  return Array.isArray(value) ? value : [];
}

function formatDuration(value: any): string {
  const durationMs = Number(value ?? 0);
  if (!Number.isFinite(durationMs) || durationMs <= 0) return "-";
  if (durationMs < 1000) return `${Math.round(durationMs)} ms`;
  if (durationMs < 60_000) return `${(durationMs / 1000).toFixed(durationMs >= 10_000 ? 0 : 1)} s`;
  const minutes = Math.floor(durationMs / 60_000);
  const seconds = Math.round((durationMs % 60_000) / 1000);
  return `${minutes}m ${seconds}s`;
}

function isWorkloadBusy(status?: string): boolean {
  const normalized = String(status || "").trim().toLowerCase();
  return normalized === "processing" || normalized === "running" || normalized === "in_progress";
}

function truncateText(value: any, limit: number): string {
  const text = String(value || "").trim();
  if (!text || text.length <= limit) return text;
  return `${text.slice(0, limit)}...`;
}

function normalizeFocusKey(value: any): string {
  return String(value || "").trim().toLowerCase();
}

function buildFocusToken(...values: any[]): string {
  const normalized = values
    .map((value) => normalizeFocusKey(value))
    .filter(Boolean)
    .join("-")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
  return normalized || "item";
}

function matchesStageFocus(stage: any, target: string): boolean {
  const normalizedTarget = normalizeFocusKey(target);
  if (!normalizedTarget) return false;
  return [stage?.id, stage?.name].some((candidate) => normalizeFocusKey(candidate) === normalizedTarget);
}

type AIEntityTraceLogEntry = {
  id: string;
  kind: "queue" | "stage" | "connector";
  status: string;
  title: string;
  description: string;
  timestamp: string;
  sortTime: number;
};

function parseTraceTimestamp(value: any): number {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  const raw = String(value || "").trim();
  if (!raw) return 0;
  const parsed = Date.parse(raw);
  return Number.isFinite(parsed) ? parsed : 0;
}

function formatTraceEventTime(value: any): string {
  if (typeof value === "number" && Number.isFinite(value)) {
    return new Date(value).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
  }
  const raw = String(value || "").trim();
  if (!raw) return "";
  const parsed = new Date(raw);
  if (Number.isNaN(parsed.getTime())) return raw;
  return parsed.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

function readTraceTimeline(result: any, primaryKey: string, secondaryKey: string): any[] {
  if (Array.isArray(result?.[primaryKey])) {
    return result[primaryKey];
  }
  if (Array.isArray(result?.[secondaryKey])) {
    return result[secondaryKey];
  }
  return [];
}

function resolvePrimaryTraceEvent(trace: any): any {
  const events = Array.isArray(trace?.events) ? trace.events : [];
  const activeEventId = String(trace?.active_event_id || trace?.activeEventId || "").trim();
  if (activeEventId) {
    const matched = events.find((item: any) => String(item?.id || "").trim() === activeEventId);
    if (matched) {
      return matched;
    }
  }
  return events[0] || null;
}

function resolveFocusedTraceWorkload(trace: any, focusWorkloadId: string): any {
  const event = resolvePrimaryTraceEvent(trace);
  const workloads = Array.isArray(event?.workloads) ? event.workloads : [];
  const normalizedFocusWorkloadId = String(focusWorkloadId || "").trim();
  if (normalizedFocusWorkloadId) {
    const matched = workloads.find((item: any) => String(item?.id || "").trim() === normalizedFocusWorkloadId);
    if (matched) {
      return matched;
    }
  }
  return workloads[0] || null;
}

function buildAIEntityTraceEventLog(trace: any, focusWorkloadId: string): AIEntityTraceLogEntry[] {
  const event = resolvePrimaryTraceEvent(trace);
  if (!event) {
    return [];
  }
  const workload = resolveFocusedTraceWorkload(trace, focusWorkloadId);
  const runResult = workload?.run?.result || {};
  const stageTimeline = readTraceTimeline(runResult, "stage_timeline", "stageTimeline");
  const connectorTimeline = readTraceTimeline(runResult, "connector_timeline", "connectorTimeline");
  const entries: AIEntityTraceLogEntry[] = [];
  const entityLabel = String(event?.entity_type || "case").trim().toLowerCase() === "alert" ? "Alert" : "Case";

  const pushEntry = (entry: AIEntityTraceLogEntry) => {
    if (!String(entry.title || "").trim()) {
      return;
    }
    entries.push(entry);
  };

  pushEntry({
    id: `queue-${String(event?.id || "event")}`,
    kind: "queue",
    status: String(event?.status || "accepted").trim() || "accepted",
    title: `${entityLabel} queued for AI processing`,
    description: String(event?.entity_title || event?.entity_reference || event?.id || "AI workload").trim(),
    timestamp: String(event?.created_at || event?.updated_at || "").trim(),
    sortTime: parseTraceTimestamp(event?.created_at || event?.updated_at),
  });

  if (workload) {
    const workloadTime = String(workload?.updated_at || workload?.started_at || event?.updated_at || "").trim();
    pushEntry({
      id: `workload-${String(workload?.id || "current")}`,
      kind: "queue",
      status: String(workload?.status || event?.status || "processing").trim() || "processing",
      title: `${String(workload?.agent_name || workload?.agent_id || "AI agent").trim() || "AI agent"} ${capitalizeStatus(workload?.status || event?.status || "processing").toLowerCase()}`,
      description: String(workload?.last_stage || workload?.last_error || runResult?.summary || "Waiting for the next stage update").trim(),
      timestamp: workloadTime,
      sortTime: parseTraceTimestamp(workloadTime),
    });
  }

  stageTimeline.forEach((stage: any, index: number) => {
    const timestamp = String(stage?.finished_at || stage?.finishedAt || stage?.started_at || stage?.startedAt || workload?.updated_at || "").trim();
    const connectorCount = Number(stage?.connector_count ?? stage?.connectorCount ?? 0);
    const connectorSuccessCount = Number(stage?.connector_success_count ?? stage?.connectorSuccessCount ?? 0);
    const summaryParts = [
      String(stage?.description || "").trim(),
      String(stage?.error || "").trim(),
      connectorCount > 0 ? `${connectorSuccessCount}/${connectorCount} connectors` : "",
      String(stage?.verdict || "").trim() ? `verdict: ${String(stage?.verdict).trim()}` : "",
    ].filter(Boolean);
    pushEntry({
      id: `stage-${String(stage?.id || index)}`,
      kind: "stage",
      status: String(stage?.status || "completed").trim() || "completed",
      title: String(stage?.name || stage?.id || `Stage ${index + 1}`).trim() || `Stage ${index + 1}`,
      description: summaryParts.join(" • ") || "Stage update received",
      timestamp,
      sortTime: parseTraceTimestamp(timestamp) || index + 1,
    });
  });

  connectorTimeline.forEach((connector: any, index: number) => {
    const timestamp = String(connector?.finished_at || connector?.finishedAt || connector?.started_at || connector?.startedAt || workload?.updated_at || "").trim();
    const summaryParts = [
      String(connector?.stage_name || connector?.stageName || "").trim(),
      String(connector?.reply || "").trim(),
      String(connector?.error || "").trim(),
    ].filter(Boolean);
    pushEntry({
      id: `connector-${String(connector?.execution_id || connector?.connector_id || index)}`,
      kind: "connector",
      status: String(connector?.status || "completed").trim() || "completed",
      title: String(connector?.name || connector?.connector_id || `Connector ${index + 1}`).trim() || `Connector ${index + 1}`,
      description: summaryParts.join(" • ") || "Connector activity recorded",
      timestamp,
      sortTime: parseTraceTimestamp(timestamp) || index + 1,
    });
  });

  return entries
    .sort((left, right) => right.sortTime - left.sortTime)
    .slice(0, 12);
}

export function AIEntityTrace({
  trace,
  isLoading = false,
  labels,
  compact = false,
  testIdPrefix = "ai-trace",
  canManageWorkloads = false,
  pendingWorkloadAction = null,
  onRestartWorkload,
  onCloseWorkload,
  renderExtraWorkloadActions,
  focusWorkloadId = "",
  focusStageKey = "",
  showLiveEventLog = false,
  liveEventLogRefreshing = false,
  liveEventLogUpdatedAt,
  onRefreshLiveEventLog,
  onConnectorExecutionClick,
}: AIEntityTraceProps) {
  const events = toArray(trace?.events);
  const normalizedFocusWorkloadId = normalizeFocusKey(focusWorkloadId);
  const normalizedFocusStageKey = normalizeFocusKey(focusStageKey);
  const liveEventLogEntries = useMemo(
    () => (showLiveEventLog ? buildAIEntityTraceEventLog(trace, focusWorkloadId) : []),
    [focusWorkloadId, showLiveEventLog, trace],
  );
  const liveEventLogUpdatedLabel = useMemo(() => formatTraceEventTime(liveEventLogUpdatedAt), [liveEventLogUpdatedAt]);

  useEffect(() => {
    if (typeof document === "undefined" || !normalizedFocusWorkloadId) {
      return;
    }
    const targetStageId = normalizedFocusStageKey
      ? `ai-trace-stage-${buildFocusToken(normalizedFocusWorkloadId, normalizedFocusStageKey)}`
      : "";
    const targetWorkloadId = `ai-trace-workload-${buildFocusToken(normalizedFocusWorkloadId)}`;
    const target = (targetStageId ? document.getElementById(targetStageId) : null) || document.getElementById(targetWorkloadId);
    target?.scrollIntoView?.({ behavior: "smooth", block: "center", inline: "nearest" });
  }, [normalizedFocusStageKey, normalizedFocusWorkloadId, events.length]);

  if (isLoading) {
    return (
      <Card className={PANEL_CLASS} data-testid={`${testIdPrefix}-loading`}>
        <div className={compact ? "space-y-4 p-4" : "space-y-4 p-5"}>
          <div className="space-y-2">
            <Skeleton className="h-5 w-40 bg-[rgba(255,255,255,0.06)]" />
            <Skeleton className="h-4 w-64 bg-[rgba(255,255,255,0.05)]" />
          </div>
          <div className="space-y-3">
            {[0, 1].map((item) => (
              <div key={item} className={`${SUBPANEL_CLASS} space-y-3 p-4`}>
                <Skeleton className="h-4 w-48 bg-[rgba(255,255,255,0.06)]" />
                <Skeleton className="h-16 w-full bg-[rgba(255,255,255,0.05)]" />
              </div>
            ))}
          </div>
        </div>
      </Card>
    );
  }

  return (
    <Card className={PANEL_CLASS} data-testid={`${testIdPrefix}-card`}>
      <div className={compact ? "space-y-4 p-4" : "space-y-5 p-5"}>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <Bot size={16} className="text-primary" />
              <h3 className="text-sm font-semibold text-white">{labels.title}</h3>
            </div>
            <p className={`${MUTED_TEXT_CLASS} max-w-2xl`}>{labels.subtitle}</p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db]">{events.length} events</Badge>
            {trace?.active_event_id ? (
              <Badge className="border border-[rgba(59,130,246,0.3)] bg-[rgba(59,130,246,0.14)] text-[#93c5fd]">Active</Badge>
            ) : null}
          </div>
        </div>

        {showLiveEventLog ? (
          <div className={`${SUBPANEL_CLASS} p-4`} data-testid={`${testIdPrefix}-event-log`}>
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <p className="text-sm font-semibold text-white">{labels.liveEventLog}</p>
                <p className={MUTED_TEXT_CLASS}>{labels.liveEventLogSubtitle}</p>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <Badge className={liveEventLogRefreshing ? "border border-[rgba(59,130,246,0.3)] bg-[rgba(59,130,246,0.14)] text-[#93c5fd]" : "border border-[rgba(34,197,94,0.28)] bg-[rgba(34,197,94,0.16)] text-[#86efac]"}>
                  {liveEventLogRefreshing ? <Loader2 size={12} className="mr-1.5 animate-spin" /> : <Activity size={12} className="mr-1.5" />}
                  {liveEventLogRefreshing ? labels.refreshing : labels.live}
                </Badge>
                <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#9ca3af]">
                  {liveEventLogUpdatedLabel ? `${labels.lastUpdated}: ${liveEventLogUpdatedLabel}` : labels.waitingForUpdate}
                </Badge>
                {onRefreshLiveEventLog ? (
                  <Button
                    type="button"
                    size="sm"
                    className={ACTION_BUTTON_CLASS}
                    data-testid={`${testIdPrefix}-refresh-live-event-log`}
                    onClick={onRefreshLiveEventLog}
                  >
                    {liveEventLogRefreshing ? <Loader2 size={14} className="mr-1.5 animate-spin" /> : <Activity size={14} className="mr-1.5" />}
                    {labels.refresh}
                  </Button>
                ) : null}
              </div>
            </div>
            <div className="mt-3 max-h-[220px] space-y-2 overflow-y-auto pr-1">
              {liveEventLogEntries.length > 0 ? (
                liveEventLogEntries.map((entry, index) => (
                  <div
                    key={entry.id}
                    className="rounded-lg border border-[#23283a] bg-[#0b1019] px-3 py-2.5"
                    data-testid={`${testIdPrefix}-event-log-entry-${index}`}
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <Badge className={statusBadgeClass(entry.status)}>{capitalizeStatus(entry.status)}</Badge>
                          <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#9ca3af]">{entry.kind}</Badge>
                          <p className="truncate text-sm font-medium text-white">{entry.title}</p>
                        </div>
                        <p className="mt-1 text-xs text-[#9ca3af]">{entry.description}</p>
                      </div>
                      <p className={`${MUTED_TEXT_CLASS} whitespace-nowrap`}>{formatTraceEventTime(entry.timestamp) || labels.waitingForUpdate}</p>
                    </div>
                  </div>
                ))
              ) : (
                <div className="rounded-lg border border-dashed border-[#2a2c3c] bg-[#0b1019] px-3 py-4 text-sm text-[#9ca3af]">
                  {labels.waitingForUpdate}
                </div>
              )}
            </div>
          </div>
        ) : null}

        {events.length === 0 ? (
          <div className={`${SUBPANEL_CLASS} px-4 py-6 text-sm text-[#9ca3af]`} data-testid={`${testIdPrefix}-empty`}>
            {labels.empty}
          </div>
        ) : (
          <div className="space-y-3">
            {events.map((event: any, eventIndex: number) => {
              const workloads = toArray(event?.workloads);
              return (
                <div key={String(event?.id || eventIndex)} className={`${SUBPANEL_CLASS} space-y-3 p-4`} data-testid={`${testIdPrefix}-event-${eventIndex}`}>
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div className="space-y-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <p className="text-sm font-semibold text-white">{String(event?.entity_title || event?.entity_reference || event?.id || labels.title)}</p>
                        <Badge className={statusBadgeClass(event?.status)}>{capitalizeStatus(event?.status)}</Badge>
                        <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db]">{formatExecutionPolicy(event?.execution_policy)}</Badge>
                      </div>
                      <p className={`${MUTED_TEXT_CLASS} break-all`}>
                        {labels.queueEvent}: <span className="font-mono text-[#cbd5e1]">{String(event?.id || "-")}</span>
                      </p>
                    </div>
                    <div className="grid gap-2 sm:grid-cols-3">
                      <div className="rounded-lg border border-[#1f2433] bg-[#0d1119] px-3 py-2">
                        <p className={MUTED_TEXT_CLASS}>{labels.source}</p>
                        <p className="mt-1 text-sm text-[#e5e7eb]">{String(event?.source || "-")}</p>
                      </div>
                      <div className="rounded-lg border border-[#1f2433] bg-[#0d1119] px-3 py-2">
                        <p className={MUTED_TEXT_CLASS}>{labels.lastUpdated}</p>
                        <p className="mt-1 text-sm text-[#e5e7eb]">{formatDateTime(String(event?.updated_at || event?.created_at || ""))}</p>
                      </div>
                      <div className="rounded-lg border border-[#1f2433] bg-[#0d1119] px-3 py-2">
                        <p className={MUTED_TEXT_CLASS}>{labels.attempts}</p>
                        <p className="mt-1 text-sm text-[#e5e7eb]">{`${Number(event?.attempt_count ?? 0)}/${Number(event?.max_attempts ?? 0) || 0}`}</p>
                      </div>
                    </div>
                  </div>

                  {workloads.length === 0 ? (
                    <p className="text-sm text-[#9ca3af]">{labels.noWorkloads}</p>
                  ) : (
                    <div className="space-y-2">
                      {workloads.map((workload: any, workloadIndex: number) => {
                        const runResult = workload?.run?.result && typeof workload.run.result === "object" ? workload.run.result : {};
                        const blockers = toArray(runResult?.action_blockers);
                        const executionPolicy = String(workload?.execution_policy || workload?.executionPolicy || "all_matching").trim() || "all_matching";
                        const executionPriority = Number(workload?.execution_priority ?? workload?.executionPriority ?? 0);
                        const executionIndex = Number(workload?.execution_index ?? workload?.executionIndex ?? 0);
                        const stageTimeline = toArray(runResult?.stage_timeline ?? runResult?.stageTimeline);
                        const connectorTimeline = toArray(runResult?.connector_timeline ?? runResult?.connectorTimeline);
                        const workloadId = String(workload?.id || "").trim();
                        const workloadKey = workloadId || `${eventIndex}-${workloadIndex}`;
                        const normalizedWorkloadKey = normalizeFocusKey(workloadKey);
                        const isFocusedWorkload = Boolean(normalizedFocusWorkloadId) && normalizedWorkloadKey === normalizedFocusWorkloadId;
                        const isRestartPending = pendingWorkloadAction?.workloadId === workloadId && pendingWorkloadAction?.action === "restart";
                        const isClosePending = pendingWorkloadAction?.workloadId === workloadId && pendingWorkloadAction?.action === "close";
                        const busy = isWorkloadBusy(workload?.status);
                        const canRestart = Boolean(canManageWorkloads && workloadId && onRestartWorkload);
                        const canClose = Boolean(canManageWorkloads && workloadId && onCloseWorkload);
                        const extraWorkloadActions = renderExtraWorkloadActions?.({ event, workload, runResult, eventIndex, workloadIndex });
                        return (
                          <div
                            key={String(workload?.id || workloadIndex)}
                            id={`ai-trace-workload-${buildFocusToken(workloadKey)}`}
                            className={`rounded-lg border bg-[#0d1119] p-3 ${isFocusedWorkload ? "border-[rgba(102,255,76,0.45)] shadow-[0_0_0_1px_rgba(102,255,76,0.22)]" : "border-[#1f2433]"}`}
                            data-testid={`${testIdPrefix}-workload-${eventIndex}-${workloadIndex}`}
                            data-ai-workload-id={normalizedWorkloadKey || undefined}
                            data-ai-focused={isFocusedWorkload ? "true" : "false"}
                            data-ai-stage-target={normalizedFocusStageKey || undefined}
                          >
                            <div className="flex flex-wrap items-start justify-between gap-2">
                              <div>
                                <div className="flex flex-wrap items-center gap-2">
                                  <Route size={13} className="text-[#93c5fd]" />
                                  <p className="text-sm font-medium text-white">{String(workload?.agent_name || workload?.agentName || workload?.agent_id || workload?.id || "AI Agent")}</p>
                                  <Badge className={statusBadgeClass(workload?.status)}>{capitalizeStatus(workload?.status)}</Badge>
                                </div>
                                <p className={`${MUTED_TEXT_CLASS} mt-1`}>
                                  {labels.execution}: {formatExecutionPolicy(executionPolicy)}
                                  {executionPriority !== 0 ? ` · priority ${executionPriority}` : ""}
                                  {executionPolicy === "fallback_chain" ? ` · step ${executionIndex + 1}` : ""}
                                </p>
                              </div>
                              <p className={`${MUTED_TEXT_CLASS} font-mono break-all`}>{workloadId}</p>
                            </div>

                            <div className="mt-3 grid gap-2 text-xs text-[#9ca3af] md:grid-cols-4">
                              <div>
                                <p className={MUTED_TEXT_CLASS}>{labels.stage}</p>
                                <p className="mt-1 text-sm text-[#e5e7eb]">{String(workload?.last_stage || workload?.lastStage || "-")}</p>
                              </div>
                              <div>
                                <p className={MUTED_TEXT_CLASS}>{labels.attempts}</p>
                                <p className="mt-1 text-sm text-[#e5e7eb]">{`${Number(workload?.attempt_count ?? workload?.attemptCount ?? 0)}/${Number(workload?.max_attempts ?? workload?.maxAttempts ?? 0) || 0}`}</p>
                              </div>
                              <div>
                                <p className={MUTED_TEXT_CLASS}>{labels.verdict}</p>
                                <p className="mt-1 text-sm text-[#e5e7eb]">{String(runResult?.verdict || "-")}</p>
                              </div>
                              <div>
                                <p className={MUTED_TEXT_CLASS}>{labels.duration}</p>
                                <p className="mt-1 text-sm text-[#e5e7eb]">{formatDuration(runResult?.duration_ms ?? runResult?.durationMs ?? workload?.duration_ms)}</p>
                              </div>
                            </div>

                            {String(runResult?.summary || "").trim() ? (
                              <p className="mt-3 text-sm text-[#cbd5e1]">{String(runResult.summary)}</p>
                            ) : null}

                            {(canRestart || canClose || extraWorkloadActions) ? (
                              <div className="mt-3 flex flex-wrap gap-2">
                                {canRestart ? (
                                  <Button
                                    type="button"
                                    size="sm"
                                    className={ACTION_BUTTON_CLASS}
                                    data-testid={`${testIdPrefix}-restart-${workloadId}`}
                                    disabled={busy || isClosePending || isRestartPending}
                                    onClick={() => onRestartWorkload?.(workloadId)}
                                  >
                                    {isRestartPending ? <Loader2 size={14} className="mr-1.5 animate-spin" /> : <RotateCcw size={14} className="mr-1.5" />}
                                    {labels.restart}
                                  </Button>
                                ) : null}
                                {canClose ? (
                                  <Button
                                    type="button"
                                    size="sm"
                                    className={DANGER_BUTTON_CLASS}
                                    data-testid={`${testIdPrefix}-close-${workloadId}`}
                                    disabled={busy || isRestartPending || isClosePending}
                                    onClick={() => onCloseWorkload?.(workloadId)}
                                  >
                                    {isClosePending ? <Loader2 size={14} className="mr-1.5 animate-spin" /> : <X size={14} className="mr-1.5" />}
                                    {labels.close}
                                  </Button>
                                ) : null}
                                {extraWorkloadActions}
                              </div>
                            ) : null}

                            {blockers.length > 0 ? (
                              <div className="mt-3 rounded-lg border border-[rgba(245,158,11,0.28)] bg-[rgba(245,158,11,0.08)] px-3 py-2 text-xs text-[#fde68a]">
                                <p className="font-semibold uppercase tracking-wide">{labels.blockers}</p>
                                <p className="mt-1">{blockers.join(" · ")}</p>
                              </div>
                            ) : null}

                            {String(workload?.last_error || workload?.lastError || runResult?.error || "").trim() ? (
                              <div className="mt-3 rounded-lg border border-[rgba(239,68,68,0.28)] bg-[rgba(239,68,68,0.08)] px-3 py-2 text-xs text-[#fecaca]">
                                <div className="flex items-start gap-2">
                                  <AlertTriangle size={14} className="mt-0.5 shrink-0" />
                                  <div>
                                    <p className="font-semibold uppercase tracking-wide">{labels.error}</p>
                                    <p className="mt-1 break-words">{String(workload?.last_error || workload?.lastError || runResult?.error || "")}</p>
                                  </div>
                                </div>
                              </div>
                            ) : null}

                            {(stageTimeline.length > 0 || connectorTimeline.length > 0) ? (
                              <div className={`mt-3 grid gap-3 ${compact ? "grid-cols-1" : "xl:grid-cols-2"}`}>
                                {stageTimeline.length > 0 ? (
                                  <div className="rounded-lg border border-[#1f2433] bg-[#0b1019] p-3" data-testid={`${testIdPrefix}-stage-timeline-${workloadId || workloadIndex}`}>
                                    <p className="text-xs font-semibold uppercase tracking-wide text-[#9ca3af]">{labels.stageTimeline}</p>
                                    <div className="mt-2 space-y-2">
                                      {stageTimeline.map((stage: any, stageIndex: number) => {
                                        const stageTags = toArray(stage?.case_tags ?? stage?.caseTags).map((item) => String(item || "").trim()).filter(Boolean);
                                        const stageKey = String(stage?.id || stage?.name || stageIndex).trim() || `stage-${stageIndex + 1}`;
                                        const normalizedStageKey = normalizeFocusKey(stageKey);
                                        const isFocusedStage = Boolean(normalizedFocusStageKey) && (!normalizedFocusWorkloadId || isFocusedWorkload) && matchesStageFocus(stage, normalizedFocusStageKey);
                                        return (
                                          <div
                                            key={String(stage?.id || stageIndex)}
                                            id={`ai-trace-stage-${buildFocusToken(workloadKey, stageKey)}`}
                                            className={`rounded-md border bg-[#101623] p-2.5 ${isFocusedStage ? "border-[rgba(102,255,76,0.45)] shadow-[0_0_0_1px_rgba(102,255,76,0.2)]" : "border-[#23283a]"}`}
                                            data-testid={`${testIdPrefix}-stage-${workloadKey}-${buildFocusToken(stageKey)}`}
                                            data-ai-stage-key={normalizedStageKey || undefined}
                                            data-ai-workload-id={normalizedWorkloadKey || undefined}
                                            data-ai-stage-focused={isFocusedStage ? "true" : "false"}
                                          >
                                            <div className="flex flex-wrap items-start justify-between gap-2">
                                              <div>
                                                <div className="flex flex-wrap items-center gap-2">
                                                  <Clock3 size={12} className="text-[#93c5fd]" />
                                                  <p className="text-sm font-medium text-white">{String(stage?.name || stage?.id || `Stage ${stageIndex + 1}`)}</p>
                                                  <Badge className={statusBadgeClass(stage?.status)}>{capitalizeStatus(stage?.status || "completed")}</Badge>
                                                </div>
                                                {String(stage?.description || "").trim() ? (
                                                  <p className="mt-1 text-xs text-[#9ca3af]">{String(stage.description)}</p>
                                                ) : null}
                                              </div>
                                              <p className={`${MUTED_TEXT_CLASS} whitespace-nowrap`}>{formatDuration(stage?.duration_ms ?? stage?.durationMs)}</p>
                                            </div>
                                            <div className="mt-2 grid gap-2 text-xs text-[#9ca3af] sm:grid-cols-2">
                                              <p>{labels.attempts}: <span className="text-[#e5e7eb]">{String(stage?.connector_success_count ?? stage?.connectorSuccessCount ?? 0)}/{String(stage?.connector_count ?? stage?.connectorCount ?? 0)}</span></p>
                                              <p>{labels.lastUpdated}: <span className="text-[#e5e7eb]">{formatDateTime(String(stage?.finished_at || stage?.finishedAt || stage?.started_at || stage?.startedAt || ""))}</span></p>
                                            </div>
                                            {stageTags.length > 0 ? (
                                              <div className="mt-2 flex flex-wrap gap-1.5">
                                                <span className={`${MUTED_TEXT_CLASS} mr-1`}>{labels.caseTags}</span>
                                                {stageTags.map((tag) => (
                                                  <Badge key={`${String(stage?.id || stageIndex)}-${tag}`} className="border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db]">{tag}</Badge>
                                                ))}
                                              </div>
                                            ) : null}
                                          </div>
                                        );
                                      })}
                                    </div>
                                  </div>
                                ) : null}

                                {connectorTimeline.length > 0 ? (
                                  <div className="rounded-lg border border-[#1f2433] bg-[#0b1019] p-3" data-testid={`${testIdPrefix}-connector-timeline-${workloadId || workloadIndex}`}>
                                    <p className="flex items-center justify-between text-xs font-semibold uppercase tracking-wide text-[#9ca3af]">
                                      <span>{labels.connectorTimeline}</span>
                                      {onConnectorExecutionClick ? (
                                        <span className="inline-flex items-center gap-1 text-[10px] font-normal text-[#9ca3af]">
                                          <span className="h-1.5 w-1.5 rounded-full bg-[rgba(102,255,76,0.9)]" />
                                          Live details
                                        </span>
                                      ) : null}
                                    </p>
                                    <div className="mt-2 space-y-2">
                                      {connectorTimeline.map((connector: any, connectorIndex: number) => {
                                        const executionId = String(connector?.execution_id || connector?.executionId || "").trim();
                                        const isClickable = Boolean(onConnectorExecutionClick && executionId);
                                        const handleConnectorClick = () => {
                                          if (!onConnectorExecutionClick || !executionId) return;
                                          onConnectorExecutionClick(executionId);
                                        };
                                        return (
                                          <div
                                            key={`${String(connector?.connector_id || connectorIndex)}-${connectorIndex}`}
                                            className={`rounded-md border border-[#23283a] bg-[#101623] p-2.5 ${isClickable ? "cursor-pointer hover:bg-[#151a2a]" : ""}`}
                                            onClick={isClickable ? handleConnectorClick : undefined}
                                          >
                                            <div className="flex flex-wrap items-start justify-between gap-2">
                                              <div>
                                                <div className="flex flex-wrap items-center gap-2">
                                                  <Route size={12} className="text-[#60a5fa]" />
                                                  <p className="text-sm font-medium text-white">{String(connector?.name || connector?.connector_id || labels.connector)}</p>
                                                  <Badge className={statusBadgeClass(connector?.status)}>{capitalizeStatus(connector?.status || "completed")}</Badge>
                                                </div>
                                                <p className="mt-1 text-xs text-[#9ca3af]">
                                                  {labels.connector}: <span className="font-mono text-[#cbd5e1]">{String(connector?.connector_id || "-")}</span>
                                                  {String(connector?.channel || "").trim() ? ` · ${String(connector.channel).trim()}` : ""}
                                                  {String(connector?.stage_name || connector?.stageName || "").trim() ? ` · ${String(connector?.stage_name || connector?.stageName).trim()}` : ""}
                                                </p>
                                              </div>
                                              <div className="flex items-center gap-1.5">
                                                <p className={`${MUTED_TEXT_CLASS} whitespace-nowrap`}>{formatDuration(connector?.duration_ms ?? connector?.durationMs)}</p>
                                                {isClickable ? (
                                                  <ArrowUpRight size={12} className="text-[#93c5fd] opacity-80" />
                                                ) : null}
                                              </div>
                                            </div>
                                            {String(connector?.reply || "").trim() ? (
                                              <p className="mt-2 text-xs text-[#cbd5e1]">
                                                {labels.reply}: {truncateText(connector.reply, compact ? 160 : 240)}
                                              </p>
                                            ) : null}
                                            {String(connector?.error || "").trim() ? (
                                              <p className="mt-2 text-xs text-[#fca5a5]">{String(connector.error)}</p>
                                            ) : null}
                                          </div>
                                        );
                                      })}
                                    </div>
                                  </div>
                                ) : null}
                              </div>
                            ) : null}
                          </div>
                        );
                      })}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </Card>
  );
}
