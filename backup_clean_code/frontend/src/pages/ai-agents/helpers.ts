import { withTenantPath } from "@/lib/tenant-url";

export function normalizeEntityType(value: any): string {
  const normalized = String(value || "").trim().toLowerCase();
  if (normalized === "cases") return "case";
  if (normalized === "alerts") return "alert";
  return normalized;
}

export function buildEntityTraceLocation(currentTenantSlug: string, entityType: any, entityId: any, workloadId?: any, stageKey?: any): string {
  const normalizedEntityType = normalizeEntityType(entityType);
  const normalizedEntityId = String(entityId || "").trim();
  if (!currentTenantSlug || !normalizedEntityId) {
    return "";
  }
  let basePath = "";
  if (normalizedEntityType === "case") {
    basePath = withTenantPath(currentTenantSlug, `/cases/${normalizedEntityId}`);
  } else if (normalizedEntityType === "alert") {
    basePath = withTenantPath(currentTenantSlug, `/alerts/${normalizedEntityId}`);
  }
  if (!basePath) {
    return "";
  }
  const params = new URLSearchParams();
  if (normalizedEntityType === "case") {
    params.set("tab", "ai");
  }
  const normalizedWorkloadId = String(workloadId || "").trim();
  const normalizedStageKey = String(stageKey || "").trim();
  if (normalizedWorkloadId) {
    params.set("ai_workload", normalizedWorkloadId);
  }
  if (normalizedStageKey) {
    params.set("ai_stage", normalizedStageKey);
  }
  const query = params.toString();
  return query ? `${basePath}?${query}` : basePath;
}

export function buildEntityOpenLabel(entityType: any): string {
  return normalizeEntityType(entityType) === "alert" ? "Open alert" : "Open case";
}

export function buildEntityLabel(entityType: any): string {
  return normalizeEntityType(entityType) === "alert" ? "Alert" : "Case";
}

export function buildEventTracePayload(event: any): any {
  return {
    events: event ? [event] : [],
    active_event_id:
      event && ["processing", "queued", "running", "in_progress"].includes(String(event?.status || "").trim().toLowerCase())
        ? String(event?.id || "")
        : "",
  };
}

export function buildOperationsTraceTitle(event: any): string {
  return String(event?.entity_reference || event?.entity_title || event?.id || "AI workload trace").trim() || "AI workload trace";
}

export function buildOperationsTraceSubtitle(event: any): string {
  const parts = [
    buildEntityLabel(event?.entity_type),
    String(event?.entity_title || "").trim(),
    String(event?.workflow_id || "").trim() ? `workflow ${String(event.workflow_id).trim()}` : "",
  ].filter(Boolean);
  return parts.join(" • ");
}

export function formatTraceStatusLabel(value: any): string {
  const normalized = String(value || "").trim().toLowerCase();
  if (!normalized) return "Unknown";
  return normalized.replace(/_/g, " ").replace(/\b\w/g, (char) => char.toUpperCase());
}

export function traceStatusBadgeClass(status: any): string {
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
    case "in_progress":
      return "border border-[rgba(59,130,246,0.3)] bg-[rgba(59,130,246,0.14)] text-[#93c5fd]";
    case "queued":
    case "pending":
    case "retry_scheduled":
      return "border border-[rgba(245,158,11,0.28)] bg-[rgba(245,158,11,0.14)] text-[#fcd34d]";
    case "cancelled":
      return "border border-[rgba(148,163,184,0.28)] bg-[rgba(51,65,85,0.4)] text-[#cbd5e1]";
    default:
      return "border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db]";
  }
}

export function parseTraceTimestamp(value: any): number {
  const raw = String(value || "").trim();
  if (!raw) return 0;
  const parsed = Date.parse(raw);
  return Number.isFinite(parsed) ? parsed : 0;
}

export function formatTraceEventTime(value: any): string {
  const raw = String(value || "").trim();
  if (!raw) return "Pending";
  const parsed = new Date(raw);
  if (Number.isNaN(parsed.getTime())) return raw;
  return parsed.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

export function readTraceTimeline(result: any, primaryKey: string, secondaryKey: string): any[] {
  if (Array.isArray(result?.[primaryKey])) {
    return result[primaryKey];
  }
  if (Array.isArray(result?.[secondaryKey])) {
    return result[secondaryKey];
  }
  return [];
}

export function resolveOperationsTraceEvent(trace: any): any {
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

export function resolveOperationsTraceWorkload(trace: any, focusWorkloadId: string): any {
  const event = resolveOperationsTraceEvent(trace);
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

export type OperationsDrawerEventLogEntry = {
  id: string;
  kind: "queue" | "stage" | "connector";
  status: string;
  title: string;
  description: string;
  timestamp: string;
  sortTime: number;
};

export function buildOperationsDrawerEventLog(trace: any, focusWorkloadId: string): OperationsDrawerEventLogEntry[] {
  const event = resolveOperationsTraceEvent(trace);
  if (!event) {
    return [];
  }
  const workload = resolveOperationsTraceWorkload(trace, focusWorkloadId);
  const runResult = workload?.run?.result || {};
  const stageTimeline = readTraceTimeline(runResult, "stage_timeline", "stageTimeline");
  const connectorTimeline = readTraceTimeline(runResult, "connector_timeline", "connectorTimeline");
  const entries: OperationsDrawerEventLogEntry[] = [];

  const pushEntry = (entry: OperationsDrawerEventLogEntry) => {
    if (!entry.title.trim()) {
      return;
    }
    entries.push(entry);
  };

  pushEntry({
    id: `queue-created-${String(event?.id || "event")}`,
    kind: "queue",
    status: String(event?.status || "accepted").trim() || "accepted",
    title: `${buildEntityLabel(event?.entity_type)} accepted into queue`,
    description: String(event?.entity_title || event?.entity_reference || event?.entity_id || "AI workload").trim(),
    timestamp: String(event?.created_at || "").trim(),
    sortTime: parseTraceTimestamp(event?.created_at),
  });

  if (workload) {
    const workloadTime = String(workload?.updated_at || workload?.started_at || event?.updated_at || "").trim();
    pushEntry({
      id: `workload-${String(workload?.id || "current")}`,
      kind: "queue",
      status: String(workload?.status || event?.status || "processing").trim() || "processing",
      title: `${String(workload?.agent_name || "AI agent").trim() || "AI agent"} workload ${formatTraceStatusLabel(workload?.status || event?.status || "processing").toLowerCase()}`,
      description: String(workload?.last_stage || workload?.last_error || event?.last_error || "Waiting for the next stage update").trim(),
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

export type AgentDraft = {
  id: string;
  name: string;
  description: string;
  prompt: string;
  model: string;
  provider: string;
  endpoint: string;
  language: string;
  targetTypes: string[];
  tagsInput: string;
  alertSourcesInput: string;
  autoCaseTagsInput: string;
  autoCloseVerdictsInput: string;
  autoCloseCase: boolean;
  autoCreateCaseFromAlert: boolean;
  maxCasesPerRun: string;
  enabled: boolean;
  autoCreateTasks: boolean;
  autoComment: boolean;
  taskAssigneeId: string;
  executionPolicy: string;
  executionPriority: string;
  triadEnabled: boolean;
  triadCriticalOnly: boolean;
  investigatorPrompt: string;
  reviewerPrompt: string;
  arbiterPrompt: string;
  requireReviewerConsensus: boolean;
  autoActionMinConfidence: string;
  enrichmentConnectorIds: string[];
  notificationConnectorIds: string[];
  stages: InvestigationStageDraft[];
};

export type InvestigationStageDraft = {
  id: string;
  name: string;
  description: string;
  prompt: string;
  caseTagsInput: string;
  enrichmentConnectorIds: string[];
};

export type ConnectorOption = {
  id: string;
  name: string;
  channel: string;
  enabled: boolean;
};

export type AIAgentsTab = "builder" | "triad" | "operations";

export type TriadCaseEntry = {
  runId: string;
  agentId: string;
  agentName: string;
  startedAt: string;
  caseId: string;
  caseNumber: string;
  title: string;
  verdict: string;
  confidence: number;
  autoActionsAllowed: boolean;
  requiresHumanReview: boolean;
  reviewerConsensus: boolean;
  actionBlockers: string[];
  triadWarnings: string[];
  triad: Record<string, any>;
};

export type WorkloadAgentProgress = {
  workloadId: string;
  agentId: string;
  agentName: string;
  status: "pending" | "running" | "done" | "error";
  statusLabel: string;
  runId: string;
  startedAt: string;
  finishedAt: string;
  verdict: string;
  confidence: number | null;
  summary: string;
  error: string;
  workloadStatus: string;
  attemptCount: number;
  maxAttempts: number;
  lastStage: string;
  workflowId: string;
  executionPolicy: string;
  executionPriority: number;
  executionIndex: number;
};

export type OperationsTraceDrawerState = {
  source: "processing" | "failed";
  eventId: string;
  entityType: string;
  entityId: string;
  title: string;
  subtitle: string;
  focusWorkloadId: string;
  focusStageKey: string;
  snapshot: any;
};

export function readRunField(run: any, ...keys: string[]): any {
  const views = [run, run?.data];
  for (const key of keys) {
    for (const view of views) {
      if (!view || typeof view !== "object") continue;
      if (Object.prototype.hasOwnProperty.call(view, key)) {
        const value = view[key];
        if (value !== undefined && value !== null && value !== "") {
          return value;
        }
      }
    }
  }
  return undefined;
}

export function readRunErrorText(run: any): string {
  const fromResult = Array.isArray(readRunField(run, "results"))
    ? readRunField(run, "results")
        .map((item: any) => String(item?.error || item?.error_message || "").trim())
        .find((item: string) => item.length > 0)
    : "";
  return String(
    fromResult ||
      readRunField(run, "error", "last_error", "lastError", "run_error", "runError") ||
      "",
  ).trim();
}

export function normalizeWorkloadStatus(raw: string, hasError: boolean): { key: WorkloadAgentProgress["status"]; label: string } {
  const normalized = String(raw || "").trim().toLowerCase();
  if (hasError || normalized === "failed" || normalized === "error" || normalized === "completed_with_error") {
    return { key: "error", label: normalized === "failed" ? "failed" : "error" };
  }
  if (normalized === "completed" || normalized === "done" || normalized === "success") {
    return { key: "done", label: normalized === "completed" ? "done" : normalized };
  }
  if (normalized === "queued" || normalized === "pending") {
    return { key: "pending", label: normalized || "pending" };
  }
  if (normalized === "cancelled" || normalized === "canceled") {
    return { key: "pending", label: "cancelled" };
  }
  if (normalized === "processing" || normalized === "running" || normalized === "in_progress") {
    return { key: "running", label: normalized.replace(/_/g, " ") };
  }
  if (normalized) {
    return { key: "running", label: normalized.replace(/_/g, " ") };
  }
  return { key: "pending", label: "pending" };
}

export function workloadStatusBadgeClass(status: WorkloadAgentProgress["status"]): string {
  if (status === "done") {
    return "border border-[rgba(34,197,94,0.3)] bg-[rgba(34,197,94,0.16)] text-[#86efac]";
  }
  if (status === "error") {
    return "border border-[rgba(239,68,68,0.3)] bg-[rgba(239,68,68,0.16)] text-[#fca5a5]";
  }
  if (status === "running") {
    return "border border-[rgba(59,130,246,0.35)] bg-[rgba(59,130,246,0.12)] text-[#93c5fd]";
  }
  return "border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db]";
}

export function formatDuration(valueMs: number): string {
  if (!Number.isFinite(valueMs) || valueMs <= 0) {
    return "0s";
  }
  const totalSeconds = Math.floor(valueMs / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) {
    return `${hours}h ${minutes}m`;
  }
  if (minutes > 0) {
    return `${minutes}m ${seconds}s`;
  }
  return `${seconds}s`;
}

export function buildWorkloadAgentProgress(workload: any, allRunsFeed: any[]): WorkloadAgentProgress[] {
  const queueEventID = String(workload?.id || "").trim();
  const entityID = String(workload?.entity_id || "").trim();
  const entityType = String(workload?.entity_type || "").trim().toLowerCase();

  const relatedRuns = (Array.isArray(allRunsFeed) ? allRunsFeed : [])
    .filter((run: any) => {
      const runQueueEventID = String(readRunField(run, "queue_event_id", "queueEventId") || "").trim();
      if (queueEventID && runQueueEventID) {
        return runQueueEventID === queueEventID;
      }
      const runEntityID = String(readRunField(run, "queue_entity_id", "queueEntityId") || "").trim();
      const runEntityType = String(readRunField(run, "queue_entity_type", "queueEntityType") || "").trim().toLowerCase();
      if (!entityID || runEntityID !== entityID) {
        return false;
      }
      return !entityType || !runEntityType || runEntityType === entityType;
    })
    .sort((left: any, right: any) => {
      const leftTime = new Date(String(readRunField(left, "started_at", "startedAt", "created_at", "createdAt") || "")).getTime();
      const rightTime = new Date(String(readRunField(right, "started_at", "startedAt", "created_at", "createdAt") || "")).getTime();
      return rightTime - leftTime;
    });

  const workloadItems = Array.isArray(workload?.workloads) ? workload.workloads : [];
  if (workloadItems.length > 0) {
    return workloadItems.map((item: any) => {
      const workloadId = String(item?.id || "").trim();
      const agentId = String(item?.agent_id || item?.agentId || "").trim();
      const agentName = String(item?.agent_name || item?.agentName || agentId || "agent").trim();
      const embeddedRun = item?.run && typeof item.run === "object" ? item.run : null;
      const run = embeddedRun || relatedRuns.find((candidate: any) => {
        const expectedRunId = String(item?.run_id || item?.runId || "").trim();
        const candidateRunId = String(candidate?.id || readRunField(candidate, "run_id", "runId") || "").trim();
        if (expectedRunId && candidateRunId) {
          return expectedRunId === candidateRunId;
        }
        const runAgentId = String(readRunField(candidate, "agent_id", "agentId") || "").trim();
        if (agentId && runAgentId) {
          return runAgentId === agentId;
        }
        const runAgentName = String(readRunField(candidate, "agent_name", "agentName") || "").trim().toLowerCase();
        return runAgentName !== "" && runAgentName === agentName.toLowerCase();
      });
      const runResult = embeddedRun?.result && typeof embeddedRun.result === "object"
        ? embeddedRun.result
        : Array.isArray(readRunField(run, "results"))
          ? readRunField(run, "results")[0] || {}
          : {};
      const errorText = String(runResult?.error || item?.last_error || readRunErrorText(run) || "").trim();
      const status = normalizeWorkloadStatus(String(item?.status || readRunField(run, "status", "run_status") || ""), Boolean(errorText));
      return {
        workloadId,
        agentId,
        agentName,
        status: status.key,
        statusLabel: status.label,
        runId: String(item?.run_id || item?.runId || embeddedRun?.id || run?.id || readRunField(run, "run_id", "runId") || "").trim(),
        startedAt: String(item?.started_at || item?.startedAt || embeddedRun?.started_at || embeddedRun?.startedAt || readRunField(run, "started_at", "startedAt", "created_at", "createdAt") || "").trim(),
        finishedAt: String(item?.finished_at || item?.finishedAt || embeddedRun?.finished_at || embeddedRun?.finishedAt || readRunField(run, "finished_at", "finishedAt") || "").trim(),
        verdict: String(runResult?.verdict || "").trim(),
        confidence: typeof runResult?.confidence === "number" ? runResult.confidence : null,
        summary: String(runResult?.summary || "").trim(),
        error: errorText,
        workloadStatus: String(item?.status || "").trim().toLowerCase(),
        attemptCount: Number(item?.attempt_count ?? item?.attemptCount ?? 0),
        maxAttempts: Number(item?.max_attempts ?? item?.maxAttempts ?? 0),
        lastStage: String(item?.last_stage || item?.lastStage || "").trim(),
        workflowId: String(item?.workflow_id || item?.workflowId || "").trim(),
        executionPolicy: String(item?.execution_policy || item?.executionPolicy || "all_matching").trim() || "all_matching",
        executionPriority: Number(item?.execution_priority ?? item?.executionPriority ?? 0),
        executionIndex: Number(item?.execution_index ?? item?.executionIndex ?? 0),
      };
    });
  }

  const candidates = Array.isArray(workload?.candidate_agents) ? workload.candidate_agents : [];
  return candidates.map((candidate: any) => {
    const agentId = String(candidate?.id || candidate?.agent_id || "").trim();
    const candidateName = String(candidate?.name || candidate?.agent_name || agentId || "agent").trim();
    const run = relatedRuns.find((item: any) => {
      const runAgentId = String(readRunField(item, "agent_id", "agentId") || "").trim();
      if (agentId && runAgentId) {
        return runAgentId === agentId;
      }
      const runAgentName = String(readRunField(item, "agent_name", "agentName") || "").trim().toLowerCase();
      return runAgentName !== "" && runAgentName === candidateName.toLowerCase();
    });

    if (!run) {
      return {
        workloadId: "",
        agentId,
        agentName: candidateName,
        status: "pending",
        statusLabel: "pending",
        runId: "",
        startedAt: "",
        finishedAt: "",
        verdict: "",
        confidence: null,
        summary: "",
        error: "",
        workloadStatus: "",
        attemptCount: 0,
        maxAttempts: 0,
        lastStage: "",
        workflowId: "",
        executionPolicy: String(candidate?.execution_policy || candidate?.executionPolicy || "all_matching").trim() || "all_matching",
        executionPriority: Number(candidate?.execution_priority ?? candidate?.executionPriority ?? 0),
        executionIndex: Number(candidate?.execution_index ?? candidate?.executionIndex ?? 0),
      };
    }

    const firstResult = Array.isArray(readRunField(run, "results")) ? readRunField(run, "results")[0] || {} : {};
    const errorText = readRunErrorText(run);
    const status = normalizeWorkloadStatus(String(readRunField(run, "status", "run_status") || ""), Boolean(errorText));

    return {
      workloadId: "",
      agentId,
      agentName: candidateName,
      status: status.key,
      statusLabel: status.label,
      runId: String(run?.id || readRunField(run, "run_id", "runId") || "").trim(),
      startedAt: String(readRunField(run, "started_at", "startedAt", "created_at", "createdAt") || "").trim(),
      finishedAt: String(readRunField(run, "finished_at", "finishedAt") || "").trim(),
      verdict: String(firstResult?.verdict || "").trim(),
      confidence: typeof firstResult?.confidence === "number" ? firstResult.confidence : null,
      summary: String(firstResult?.summary || "").trim(),
      error: errorText,
      workloadStatus: String(readRunField(run, "status", "run_status") || "").trim().toLowerCase(),
      attemptCount: 0,
      maxAttempts: 0,
      lastStage: "",
      workflowId: "",
      executionPolicy: String(candidate?.execution_policy || candidate?.executionPolicy || "all_matching").trim() || "all_matching",
      executionPriority: Number(candidate?.execution_priority ?? candidate?.executionPriority ?? 0),
      executionIndex: Number(candidate?.execution_index ?? candidate?.executionIndex ?? 0),
    };
  });
}


export function readTriadCaseEntries(runs: any[]): TriadCaseEntry[] {
  const out: TriadCaseEntry[] = [];
  for (const run of Array.isArray(runs) ? runs : []) {
    const runResults = Array.isArray(run?.results) ? run.results : [];
    for (const result of runResults) {
      const triad = result?.triad && typeof result.triad === "object" ? result.triad : null;
      if (!triad) continue;
      const actionBlockersRaw = result?.action_blockers ?? result?.actionBlockers;
      const triadWarningsRaw = result?.triad_warnings ?? result?.triadWarnings;
      out.push({
        runId: String(run?.id || run?.runId || "").trim(),
        agentId: String(run?.agentId || run?.agent_id || "").trim(),
        agentName: String(run?.agentName || run?.agent_name || "").trim(),
        startedAt: String(run?.startedAt || run?.started_at || "").trim(),
        caseId: String(result?.case_id || result?.caseId || "").trim(),
        caseNumber: String(result?.case_number || result?.caseNumber || "").trim(),
        title: String(result?.title || "").trim(),
        verdict: String(result?.verdict || "unknown").trim().toLowerCase(),
        confidence: Number(result?.confidence ?? 0),
        autoActionsAllowed: Boolean(result?.auto_actions_allowed ?? result?.autoActionsAllowed ?? true),
        requiresHumanReview: Boolean(result?.requires_human_review ?? result?.requiresHumanReview ?? false),
        reviewerConsensus: Boolean(result?.reviewer_consensus ?? result?.reviewerConsensus ?? false),
        actionBlockers: Array.isArray(actionBlockersRaw)
          ? actionBlockersRaw.map((item: any) => String(item || "").trim()).filter(Boolean)
          : [],
        triadWarnings: Array.isArray(triadWarningsRaw)
          ? triadWarningsRaw.map((item: any) => String(item || "").trim()).filter(Boolean)
          : [],
        triad,
      });
    }
  }
  return out;
}

export function defaultAgentDraft(): AgentDraft {
  return {
    id: "",
    name: "",
    description: "",
    prompt: "",
    model: "",
    provider: "openai",
    endpoint: "",
    language: "auto",
    targetTypes: ["case"],
    tagsInput: "",
    alertSourcesInput: "",
    autoCaseTagsInput: "",
    autoCloseVerdictsInput: "benign, resolved, closed",
    autoCloseCase: false,
    autoCreateCaseFromAlert: false,
    maxCasesPerRun: "10",
    enabled: true,
    autoCreateTasks: true,
    autoComment: true,
    taskAssigneeId: "",
    executionPolicy: "all_matching",
    executionPriority: "0",
    triadEnabled: false,
    triadCriticalOnly: true,
    investigatorPrompt: "",
    reviewerPrompt: "",
    arbiterPrompt: "",
    requireReviewerConsensus: true,
    autoActionMinConfidence: "85",
    enrichmentConnectorIds: [],
    notificationConnectorIds: [],
    stages: [],
  };
}

export function normalizeCommaSeparated(value: string): string[] {
  return String(value || "")
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

export function positiveInt(value: string, fallback: number): number {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return fallback;
  }
  return Math.max(1, Math.min(100, Math.round(parsed)));
}

export function parseIdentifierList(value: string): string[] {
  return String(value || "")
    .split(/[\s,]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

export function formatDateTime(value?: string): string {
  if (!value) return "n/a";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
}

export function formatExecutionPolicy(value?: string): string {
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

export function toAgentDraft(agent: any): AgentDraft {
  const targetTypesRaw = Array.isArray(agent?.targetTypes) ? agent.targetTypes : [];
  const normalizedTargetTypes = targetTypesRaw
    .map((item: any) => String(item || "").trim().toLowerCase())
    .filter((item: string) => item === "case" || item === "alert");
  const targetTypes: string[] = normalizedTargetTypes.length > 0 ? Array.from(new Set(normalizedTargetTypes)) : ["case"];
  const stages = Array.isArray(agent?.investigationPlan)
    ? agent.investigationPlan
        .map((stage: any, index: number) => ({
          id: String(stage?.id || `stage-${index + 1}`).trim() || `stage-${index + 1}`,
          name: String(stage?.name || "").trim(),
          description: String(stage?.description || "").trim(),
          prompt: String(stage?.prompt || "").trim(),
          caseTagsInput: Array.isArray(stage?.caseTags) ? stage.caseTags.join(", ") : "",
          enrichmentConnectorIds: Array.isArray(stage?.enrichmentConnectorIds)
            ? stage.enrichmentConnectorIds.map((item: any) => String(item || "").trim()).filter(Boolean)
            : [],
        }))
        .filter((stage: InvestigationStageDraft) => stage.name || stage.description || stage.prompt || stage.caseTagsInput || stage.enrichmentConnectorIds.length > 0)
    : [];
  return {
    id: String(agent?.id || ""),
    name: String(agent?.name || "").trim(),
    description: String(agent?.description || "").trim(),
    prompt: String(agent?.prompt || "").trim(),
    model: String(agent?.model || "").trim(),
    provider: String(agent?.provider || "").trim() || "openai",
    endpoint: String(agent?.endpoint || "").trim(),
    language: String(agent?.language || "").trim() || "auto",
    targetTypes,
    tagsInput: Array.isArray(agent?.caseTags) ? agent.caseTags.join(", ") : "",
    alertSourcesInput: Array.isArray(agent?.alertSources) ? agent.alertSources.join(", ") : "",
    autoCaseTagsInput: Array.isArray(agent?.autoCaseTags) ? agent.autoCaseTags.join(", ") : "",
    autoCloseVerdictsInput: Array.isArray(agent?.autoCloseVerdicts) ? agent.autoCloseVerdicts.join(", ") : "benign, resolved, closed",
    autoCloseCase: Boolean(agent?.autoCloseCase ?? false),
    autoCreateCaseFromAlert: Boolean(agent?.autoCreateCaseFromAlert ?? false),
    maxCasesPerRun: String(agent?.maxCasesPerRun || 10),
    enabled: Boolean(agent?.enabled ?? true),
    autoCreateTasks: Boolean(agent?.autoCreateTasks ?? true),
    autoComment: Boolean(agent?.autoComment ?? true),
    taskAssigneeId: String(agent?.taskAssigneeId || "").trim(),
    executionPolicy: String(agent?.executionPolicy || "all_matching").trim() || "all_matching",
    executionPriority: String(agent?.executionPriority ?? 0),
    triadEnabled: Boolean(agent?.triadEnabled ?? false),
    triadCriticalOnly: Boolean(agent?.triadCriticalOnly ?? true),
    investigatorPrompt: String(agent?.investigatorPrompt || "").trim(),
    reviewerPrompt: String(agent?.reviewerPrompt || "").trim(),
    arbiterPrompt: String(agent?.arbiterPrompt || "").trim(),
    requireReviewerConsensus: Boolean(agent?.requireReviewerConsensus ?? true),
    autoActionMinConfidence: String(agent?.autoActionMinConfidence ?? 85),
    enrichmentConnectorIds: Array.isArray(agent?.enrichmentConnectorIds)
      ? agent.enrichmentConnectorIds.map((item: any) => String(item || "").trim()).filter(Boolean)
      : [],
    notificationConnectorIds: Array.isArray(agent?.notificationConnectorIds)
      ? agent.notificationConnectorIds.map((item: any) => String(item || "").trim()).filter(Boolean)
      : [],
    stages,
  };
}

export function normalizeConnectorOption(item: any): ConnectorOption | null {
  const id = String(item?.id || "").trim();
  if (!id) {
    return null;
  }
  const data = item?.data && typeof item.data === "object" ? item.data : {};
  const name = String(item?.name || data?.name || item?.title || id).trim() || id;
  const channel = String(item?.channel || item?.type || data?.channel || data?.type || data?.provider || "").trim().toLowerCase();
  const enabled = Boolean(item?.enabled ?? data?.enabled ?? true);
  return { id, name, channel, enabled };
}

export function buildAgentPayload(draft: AgentDraft): any {
  const targetTypes = draft.targetTypes
    .map((item) => String(item || "").trim().toLowerCase())
    .filter((item) => item === "case" || item === "alert");
  const stages = draft.stages
    .map((stage, index) => ({
      id: String(stage.id || `stage-${index + 1}`).trim() || `stage-${index + 1}`,
      name: String(stage.name || "").trim(),
      description: String(stage.description || "").trim(),
      prompt: String(stage.prompt || "").trim(),
      caseTags: normalizeCommaSeparated(stage.caseTagsInput),
      enrichmentConnectorIds: stage.enrichmentConnectorIds.map((item) => String(item || "").trim()).filter(Boolean),
    }))
    .filter((stage) => stage.name || stage.description || stage.prompt || stage.caseTags.length > 0 || stage.enrichmentConnectorIds.length > 0);
  return {
    name: draft.name.trim(),
    description: draft.description.trim(),
    prompt: draft.prompt.trim(),
    model: draft.model.trim(),
    provider: draft.provider.trim(),
    endpoint: draft.endpoint.trim(),
    language: draft.language === "auto" ? "" : draft.language.trim(),
    targetTypes,
    caseTags: normalizeCommaSeparated(draft.tagsInput),
    alertSources: normalizeCommaSeparated(draft.alertSourcesInput),
    autoCaseTags: normalizeCommaSeparated(draft.autoCaseTagsInput),
    autoCloseVerdicts: normalizeCommaSeparated(draft.autoCloseVerdictsInput),
    autoCloseCase: draft.autoCloseCase,
    autoCreateCaseFromAlert: draft.autoCreateCaseFromAlert,
    enrichmentConnectorIds: draft.enrichmentConnectorIds,
    notificationConnectorIds: draft.notificationConnectorIds,
    investigationPlan: stages,
    maxCasesPerRun: positiveInt(draft.maxCasesPerRun, 10),
    enabled: draft.enabled,
    autoCreateTasks: draft.autoCreateTasks,
    autoComment: draft.autoComment,
    taskAssigneeId: draft.taskAssigneeId.trim(),
    executionPolicy: (draft.executionPolicy || "all_matching").trim() || "all_matching",
    executionPriority: Math.max(-1000, Math.min(1000, Number(draft.executionPriority || "0") || 0)),
    triadEnabled: draft.triadEnabled,
    triadCriticalOnly: draft.triadCriticalOnly,
    investigatorPrompt: draft.investigatorPrompt.trim(),
    reviewerPrompt: draft.reviewerPrompt.trim(),
    arbiterPrompt: draft.arbiterPrompt.trim(),
    requireReviewerConsensus: draft.requireReviewerConsensus,
    autoActionMinConfidence: Math.max(0, Math.min(100, Number(draft.autoActionMinConfidence || "85") || 85)),
  };
}
