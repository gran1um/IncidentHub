import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { Activity, ArrowUpRight, Bot, CheckCircle2, Loader2, RefreshCcw, XCircle } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";

type PendingAction = {
  workloadId: string;
  action: "restart" | "close";
} | null;

type ExtraActionsContext = {
  event: any;
  workload: any;
  runResult: any;
};

function normalizeTestIdSegment(value: string): string {
  return String(value || "")
    .trim()
    .replace(/[^a-zA-Z0-9_-]+/g, "_");
}

function formatTimestamp(value: any): string {
  const raw = String(value || "").trim();
  if (!raw) {
    return "";
  }
  const date = new Date(raw);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return date.toLocaleString();
}

export function CaseAIWorkloadsTab({
  trace,
  isLoading,
  isFetching,
  onRefresh,
  canManageWorkloads,
  pendingWorkloadAction,
  onRestartWorkload,
  onCloseWorkload,
  onConnectorExecutionClick,
  renderExtraActions,
  focusedWorkloadId,
  focusedStageId,
}: {
  trace: any;
  isLoading?: boolean;
  isFetching?: boolean;
  onRefresh?: () => void;
  canManageWorkloads?: boolean;
  pendingWorkloadAction?: PendingAction;
  onRestartWorkload?: (workloadId: string) => void;
  onCloseWorkload?: (workloadId: string) => void;
  onConnectorExecutionClick?: (executionId: string) => void;
  renderExtraActions?: (context: ExtraActionsContext) => ReactNode;
  focusedWorkloadId?: string;
  focusedStageId?: string;
}) {
  const events = useMemo(() => (Array.isArray(trace?.events) ? trace.events : []), [trace?.events]);
  const activeEventId = String(trace?.active_event_id || "").trim();
  const deepLinkWorkloadId = String(focusedWorkloadId || "").trim();
  const deepLinkStageId = String(focusedStageId || "").trim();

  const appliedEventDeepLinkRef = useRef(false);
  const appliedWorkloadDeepLinkRef = useRef(false);

  const [selectedEventId, setSelectedEventId] = useState("");

  useEffect(() => {
    if (selectedEventId) {
      return;
    }
    const initial = activeEventId || String(events[0]?.id || "").trim();
    setSelectedEventId(initial);
  }, [activeEventId, events, selectedEventId]);

  useEffect(() => {
    if (appliedEventDeepLinkRef.current) {
      return;
    }
    if (!deepLinkWorkloadId || events.length === 0) {
      return;
    }
    const eventWithWorkload = events.find((event: any) => {
      const workloads = Array.isArray(event?.workloads) ? event.workloads : [];
      return workloads.some((workload: any) => String(workload?.id || "").trim() === deepLinkWorkloadId);
    });
    if (eventWithWorkload?.id) {
      setSelectedEventId(String(eventWithWorkload.id).trim());
    }
    appliedEventDeepLinkRef.current = true;
  }, [deepLinkWorkloadId, events]);

  useEffect(() => {
    if (!selectedEventId) {
      return;
    }
    const exists = events.some((event: any) => String(event?.id || "").trim() === selectedEventId);
    if (!exists) {
      setSelectedEventId(String(activeEventId || events[0]?.id || "").trim());
    }
  }, [activeEventId, events, selectedEventId]);

  const selectedEvent = useMemo(() => {
    if (!selectedEventId) {
      return events[0] || null;
    }
    return events.find((event: any) => String(event?.id || "").trim() === selectedEventId) || events[0] || null;
  }, [events, selectedEventId]);

  const workloads = useMemo(() => {
    const items = selectedEvent?.workloads;
    return Array.isArray(items) ? items : [];
  }, [selectedEvent?.workloads]);

  const [selectedWorkloadId, setSelectedWorkloadId] = useState("");

  useEffect(() => {
    if (!workloads.length) {
      setSelectedWorkloadId("");
      return;
    }
    if (!appliedWorkloadDeepLinkRef.current && deepLinkWorkloadId) {
      const exists = workloads.some((workload: any) => String(workload?.id || "") === deepLinkWorkloadId);
      if (exists) {
        setSelectedWorkloadId(deepLinkWorkloadId);
        appliedWorkloadDeepLinkRef.current = true;
        return;
      }
    }
    if (!selectedWorkloadId) {
      setSelectedWorkloadId(String(workloads[0]?.id || ""));
      return;
    }
    const exists = workloads.some((workload: any) => String(workload?.id || "") === selectedWorkloadId);
    if (!exists) {
      setSelectedWorkloadId(String(workloads[0]?.id || ""));
    }
  }, [deepLinkWorkloadId, selectedWorkloadId, workloads]);

  const selectedWorkload = useMemo(
    () => workloads.find((workload: any) => String(workload?.id || "") === selectedWorkloadId) || null,
    [workloads, selectedWorkloadId],
  );

  const selectedRunResult = useMemo(() => {
    const raw = selectedWorkload?.run?.result;
    return raw && typeof raw === "object" ? raw : {};
  }, [selectedWorkload?.run?.result]);

  const stageTimeline = useMemo(() => {
    const items = selectedRunResult?.stage_timeline ?? selectedRunResult?.stageTimeline;
    return Array.isArray(items) ? items : [];
  }, [selectedRunResult]);

  const connectorTimeline = useMemo(() => {
    const items = selectedRunResult?.connector_timeline ?? selectedRunResult?.connectorTimeline;
    return Array.isArray(items) ? items : [];
  }, [selectedRunResult]);

  const isLive = Boolean(events.length > 0);

  if (isLoading) {
    return (
      <Card className="rounded-2xl border border-[#2a2c3c] bg-[#13141c] p-5">
        <div className="space-y-3">
          <Skeleton className="h-5 w-56 rounded-lg bg-[rgba(255,255,255,0.05)]" />
          <Skeleton className="h-4 w-full rounded-lg bg-[rgba(255,255,255,0.04)]" />
          <div className="grid gap-4 md:grid-cols-2">
            <Skeleton className="h-56 w-full rounded-xl bg-[rgba(255,255,255,0.04)]" />
            <Skeleton className="h-56 w-full rounded-xl bg-[rgba(255,255,255,0.04)]" />
          </div>
        </div>
      </Card>
    );
  }

  return (
    <Card className="rounded-2xl border border-[#2a2c3c] bg-[#13141c] p-5" data-testid="case-ai-workloads">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="flex items-center gap-2">
            <Bot size={16} className="text-primary" />
            <h3 className="text-sm font-semibold text-white">AI Workloads</h3>
          </div>
          <p className="mt-1 text-xs text-[#8b91a3]">Workload list with stage/connector details and actions.</p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Badge
            className={
              isFetching
                ? "border border-[rgba(59,130,246,0.3)] bg-[rgba(59,130,246,0.14)] text-[#93c5fd]"
                : "border border-[rgba(34,197,94,0.28)] bg-[rgba(34,197,94,0.16)] text-[#86efac]"
            }
          >
            {isFetching ? <Loader2 size={12} className="mr-1.5 animate-spin" /> : <Activity size={12} className="mr-1.5" />}
            {isFetching ? "Refreshing" : "Live"}
          </Badge>
          <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db]">{events.length} events</Badge>
          {onRefresh ? (
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="h-8 rounded-lg border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db] hover:bg-[#171b2a] hover:text-white"
              onClick={onRefresh}
              data-testid="case-ai-refresh"
            >
              <RefreshCcw size={14} className="mr-1.5" />
              Refresh
            </Button>
          ) : null}
        </div>
      </div>

      <Separator className="my-4 bg-[#23283a]" />

      {!isLive ? (
        <div className="rounded-xl border border-dashed border-[#2a2c3c] bg-[#0f131d] px-4 py-6 text-sm text-[#9ca3af]" data-testid="case-ai-empty">
          No AI workloads were recorded for this case yet.
        </div>
      ) : (
        <div className="space-y-4">
          {events.length > 1 ? (
            <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto]">
              <div className="space-y-2">
                <div className="text-xs font-semibold text-[#9ca3af]">Queue event</div>
                <Select value={selectedEventId || ""} onValueChange={(value) => setSelectedEventId(value)}>
                  <SelectTrigger className="h-10 rounded-xl border-[#2a2c3c] bg-[#0f131d] text-[#f3f4f6]" data-testid="case-ai-select-event">
                    <SelectValue placeholder="Select event" />
                  </SelectTrigger>
                  <SelectContent className="rounded-xl border border-[#2a2c3c] bg-[#13141c] text-[#f3f4f6]">
                    {events.map((event: any) => {
                      const id = String(event?.id || "").trim();
                      const createdAt = String(event?.created_at || event?.createdAt || "").trim();
                      const status = String(event?.status || "").trim();
                      return (
                        <SelectItem key={id} value={id}>
                          {id.slice(0, 8)}… {status || ""} {createdAt ? `· ${createdAt}` : ""}
                        </SelectItem>
                      );
                    })}
                  </SelectContent>
                </Select>
              </div>
            </div>
          ) : null}

          <div className="grid gap-4 lg:grid-cols-[minmax(0,0.85fr)_minmax(0,1.15fr)]">
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <p className="text-xs font-semibold uppercase tracking-[0.16em] text-[#9ca3af]">Workloads</p>
                <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db]">{workloads.length}</Badge>
              </div>

              {workloads.length === 0 ? (
                <div className="rounded-xl border border-dashed border-[#2a2c3c] bg-[#0f131d] px-4 py-6 text-sm text-[#9ca3af]">
                  No workloads for this event yet.
                </div>
              ) : (
                <div className="space-y-2">
                  {workloads.map((workload: any) => {
                    const id = String(workload?.id || "");
                    const selected = id === selectedWorkloadId;
                    const agentName = String(workload?.agent_name || workload?.agentName || workload?.agent_id || "AI Agent").trim();
                    const status = String(workload?.status || "unknown").trim().toLowerCase();
                    const verdict = String(workload?.run?.result?.verdict || "").trim();
                    const error = String(workload?.last_error || workload?.run?.result?.error || "").trim();
                    const createdAt = formatTimestamp(workload?.created_at || workload?.createdAt);
                    const startedAt = formatTimestamp(workload?.started_at || workload?.startedAt);
                    const finishedAt = formatTimestamp(workload?.finished_at || workload?.finishedAt);
                    const timeSummary = [createdAt ? `Created ${createdAt}` : "", startedAt ? `Started ${startedAt}` : "", finishedAt ? `Finished ${finishedAt}` : ""]
                      .filter(Boolean)
                      .join(" · ");
                    return (
                      <button
                        key={id}
                        type="button"
                        className={
                          selected
                            ? "w-full rounded-xl border border-[rgba(102,255,76,0.35)] bg-[rgba(102,255,76,0.08)] px-3 py-3 text-left"
                            : "w-full rounded-xl border border-[#23283a] bg-[#0f131d] px-3 py-3 text-left hover:border-[#3b425a]"
                        }
                        onClick={() => setSelectedWorkloadId(id)}
                        data-testid={`case-ai-workload-${id}`}
                        data-ai-focused={selected ? "true" : "false"}
                      >
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0">
                            <p className="truncate text-sm font-semibold text-white">{agentName}</p>
                            {verdict || timeSummary ? (
                              <p className="mt-1 text-xs text-[#8b91a3]">
                                {verdict ? `Verdict: ${verdict}` : ""}
                                {verdict && timeSummary ? " · " : ""}
                                {timeSummary}
                              </p>
                            ) : null}
                            {error ? <p className="mt-1 line-clamp-2 text-xs text-[#fda4af]">{error}</p> : null}
                          </div>
                          <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db]">{status}</Badge>
                        </div>
                      </button>
                    );
                  })}
                </div>
              )}
            </div>

            <div className="space-y-4">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-[#9ca3af]">Details</p>
                  {selectedWorkload ? (
                    <>
                      <p className="mt-1 text-sm font-semibold text-white">
                        {String(selectedWorkload?.agent_name || selectedWorkload?.agentName || selectedWorkload?.agent_id || "AI Agent")}
                      </p>
                      {(() => {
                        const createdAt = formatTimestamp(selectedWorkload?.created_at || selectedWorkload?.createdAt);
                        const startedAt = formatTimestamp(selectedWorkload?.started_at || selectedWorkload?.startedAt);
                        const finishedAt = formatTimestamp(selectedWorkload?.finished_at || selectedWorkload?.finishedAt);
                        const summary = [createdAt ? `Created ${createdAt}` : "", startedAt ? `Started ${startedAt}` : "", finishedAt ? `Finished ${finishedAt}` : ""]
                          .filter(Boolean)
                          .join(" · ");
                        return summary ? <p className="mt-1 text-xs text-[#8b91a3]">{summary}</p> : null;
                      })()}
                    </>
                  ) : null}
                </div>

                {selectedWorkload ? (
                  <div className="flex flex-wrap items-center gap-2">
                    {renderExtraActions ? (
                      <div className="flex flex-wrap items-center gap-2">
                        {renderExtraActions({ event: selectedEvent, workload: selectedWorkload, runResult: selectedRunResult })}
                      </div>
                    ) : null}

                    {canManageWorkloads && onRestartWorkload ? (
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        className="h-8 rounded-lg border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db] hover:bg-[#171b2a] hover:text-white"
                        onClick={() => onRestartWorkload(String(selectedWorkload.id || ""))}
                        disabled={
                          pendingWorkloadAction?.workloadId === String(selectedWorkload.id || "") &&
                          pendingWorkloadAction?.action === "restart"
                        }
                        data-testid="case-ai-workload-restart"
                      >
                        {pendingWorkloadAction?.workloadId === String(selectedWorkload.id || "") &&
                        pendingWorkloadAction?.action === "restart" ? (
                          <Loader2 size={14} className="mr-1.5 animate-spin" />
                        ) : (
                          <CheckCircle2 size={14} className="mr-1.5" />
                        )}
                        Restart
                      </Button>
                    ) : null}

                    {canManageWorkloads && onCloseWorkload ? (
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        className="h-8 rounded-lg border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.12)] text-[#fda4af] hover:bg-[rgba(239,68,68,0.18)]"
                        onClick={() => onCloseWorkload(String(selectedWorkload.id || ""))}
                        disabled={
                          pendingWorkloadAction?.workloadId === String(selectedWorkload.id || "") &&
                          pendingWorkloadAction?.action === "close"
                        }
                        data-testid="case-ai-workload-close"
                      >
                        {pendingWorkloadAction?.workloadId === String(selectedWorkload.id || "") &&
                        pendingWorkloadAction?.action === "close" ? (
                          <Loader2 size={14} className="mr-1.5 animate-spin" />
                        ) : (
                          <XCircle size={14} className="mr-1.5" />
                        )}
                        Close
                      </Button>
                    ) : null}
                  </div>
                ) : null}
              </div>

              {!selectedWorkload ? (
                <div className="rounded-xl border border-dashed border-[#2a2c3c] bg-[#0f131d] px-4 py-6 text-sm text-[#9ca3af]">
                  Select a workload to inspect its trace.
                </div>
              ) : (
                <>
                  <div className="grid gap-3 md:grid-cols-3">
                    <div className="rounded-xl border border-[#23283a] bg-[#0f131d] p-3">
                      <p className="text-xs font-semibold text-[#9ca3af]">Status</p>
                      <p className="mt-1 text-sm font-semibold text-white">{String(selectedWorkload?.status || "unknown")}</p>
                    </div>
                    <div className="rounded-xl border border-[#23283a] bg-[#0f131d] p-3">
                      <p className="text-xs font-semibold text-[#9ca3af]">Verdict</p>
                      <p className="mt-1 text-sm font-semibold text-white">{String(selectedRunResult?.verdict || "-")}</p>
                    </div>
                    <div className="rounded-xl border border-[#23283a] bg-[#0f131d] p-3">
                      <p className="text-xs font-semibold text-[#9ca3af]">Confidence</p>
                      <p className="mt-1 text-sm font-semibold text-white">
                        {Number.isFinite(Number(selectedRunResult?.confidence)) ? Number(selectedRunResult.confidence).toFixed(2) : "-"}
                      </p>
                    </div>
                  </div>

                  {String(selectedRunResult?.summary || "").trim() ? (
                    <div className="rounded-xl border border-[#23283a] bg-[#0f131d] p-4">
                      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-[#9ca3af]">Summary</p>
                      <p className="mt-2 whitespace-pre-wrap text-sm leading-relaxed text-[#d1d5db]">{String(selectedRunResult.summary)}</p>
                    </div>
                  ) : null}

                  {String(selectedRunResult?.error || selectedWorkload?.last_error || "").trim() ? (
                    <div className="rounded-xl border border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.12)] p-4 text-sm text-[#fda4af]">
                      {String(selectedRunResult?.error || selectedWorkload?.last_error)}
                    </div>
                  ) : null}

                  {Array.isArray(selectedRunResult?.action_blockers) && selectedRunResult.action_blockers.length > 0 ? (
                    <div className="rounded-xl border border-[#23283a] bg-[#0f131d] p-4">
                      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-[#9ca3af]">Action Blockers</p>
                      <ul className="mt-2 space-y-1 text-sm text-[#d1d5db]">
                        {selectedRunResult.action_blockers.slice(0, 10).map((item: any, idx: number) => (
                          <li key={`${item}-${idx}`} className="flex gap-2">
                            <span className="mt-1.5 h-1.5 w-1.5 rounded-full bg-[#fcd34d]" />
                            <span className="min-w-0 flex-1 break-words">{String(item)}</span>
                          </li>
                        ))}
                      </ul>
                    </div>
                  ) : null}

                  <div className="grid gap-4 lg:grid-cols-2">
                    <div className="rounded-xl border border-[#23283a] bg-[#0f131d] p-4">
                      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-[#9ca3af]">Stage Timeline</p>
                      {stageTimeline.length === 0 ? (
                        <p className="mt-2 text-sm text-[#9ca3af]">No stage data.</p>
                      ) : (
                        <div className="mt-3 space-y-2">
                          {stageTimeline.slice(0, 20).map((stage: any, idx: number) => {
                            const stageName = String(stage?.name || stage?.stage_name || stage?.stage || "Stage").trim();
                            const status = String(stage?.status || "").trim();
                            const durationMs = Number(stage?.duration_ms ?? stage?.durationMs ?? 0);
                            const connectorCount = Number(stage?.connector_count ?? stage?.connectorCount ?? 0);
                            const okCount = Number(stage?.connector_success_count ?? stage?.connectorSuccessCount ?? 0);
                            const errCount = Number(stage?.connector_error_count ?? stage?.connectorErrorCount ?? 0);
                            const rawStageId = String(stage?.id || stageName || idx).trim();
                            const stageIdSegment = normalizeTestIdSegment(rawStageId);
                            const workloadIdSegment = normalizeTestIdSegment(selectedWorkloadId || selectedWorkload?.id || "workload");
                            const stageFocused =
                              deepLinkStageId &&
                              (rawStageId === deepLinkStageId || stageName.toLowerCase() === deepLinkStageId.toLowerCase());
                            return (
                              <div
                                key={String(stage?.id || stageName)}
                                className="rounded-lg border border-[#1f2433] bg-[#0b1019] px-3 py-2"
                                data-testid={`case-ai-stage-${workloadIdSegment}-${stageIdSegment}`}
                                data-ai-stage-focused={stageFocused ? "true" : "false"}
                              >
                                <div className="flex flex-wrap items-center justify-between gap-2">
                                  <p className="text-sm font-semibold text-white">{stageName}</p>
                                  <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db]">{status || "completed"}</Badge>
                                </div>
                                <p className="mt-1 text-xs text-[#8b91a3]">
                                  connectors: {connectorCount} (ok {okCount} / err {errCount}){durationMs > 0 ? ` · ${Math.round(durationMs)}ms` : ""}
                                </p>
                              </div>
                            );
                          })}
                        </div>
                      )}
                    </div>

                    <div className="rounded-xl border border-[#23283a] bg-[#0f131d] p-4">
                      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-[#9ca3af]">Connector Timeline</p>
                      {connectorTimeline.length === 0 ? (
                        <p className="mt-2 text-sm text-[#9ca3af]">No connector calls.</p>
                      ) : (
                        <div className="mt-3 space-y-2">
                          {connectorTimeline.slice(0, 30).map((entry: any, idx: number) => {
                            const name = String(entry?.name || entry?.connector_name || entry?.connector_id || "Connector").trim();
                            const status = String(entry?.status || "").trim().toLowerCase() || "completed";
                            const executionId = String(entry?.execution_id || entry?.executionId || "").trim();
                            const error = String(entry?.error || "").trim();
                            const reply = String(entry?.reply || "").trim();
                            const durationMs = Number(entry?.duration_ms ?? entry?.durationMs ?? 0);
                            const clickable = Boolean(executionId && onConnectorExecutionClick);
                            return (
                              <button
                                key={`${executionId || name}-${idx}`}
                                type="button"
                                className={
                                  clickable
                                    ? "w-full rounded-lg border border-[#1f2433] bg-[#0b1019] px-3 py-2 text-left hover:border-[#3b425a]"
                                    : "w-full rounded-lg border border-[#1f2433] bg-[#0b1019] px-3 py-2 text-left"
                                }
                                onClick={() => {
                                  if (!clickable) return;
                                  onConnectorExecutionClick?.(executionId);
                                }}
                                disabled={!clickable}
                                data-testid={`case-ai-connector-${idx}`}
                              >
                                <div className="flex flex-wrap items-start justify-between gap-2">
                                  <div className="min-w-0">
                                    <div className="flex flex-wrap items-center gap-2">
                                      <p className="truncate text-sm font-semibold text-white">{name}</p>
                                      <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db]">{status}</Badge>
                                      {clickable ? <ArrowUpRight size={14} className="text-[#9ca3af]" /> : null}
                                    </div>
                                    {error ? <p className="mt-1 text-xs text-[#fda4af]">{error}</p> : null}
                                    {!error && reply ? <p className="mt-1 text-xs text-[#86efac]">{reply.slice(0, 140)}{reply.length > 140 ? "…" : ""}</p> : null}
                                  </div>
                                  {durationMs > 0 ? (
                                    <div className="text-xs text-[#8b91a3]">{Math.round(durationMs)}ms</div>
                                  ) : null}
                                </div>
                              </button>
                            );
                          })}
                        </div>
                      )}
                    </div>
                  </div>
                </>
              )}
            </div>
          </div>
        </div>
      )}
    </Card>
  );
}
