import { useMemo } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import {
  useCancelConnectorHubExecution,
  useConnectorHubExecutionDetail,
  useConnectorHubExecutionEvents,
  useRestartConnectorHubExecution,
  useRetryConnectorHubExecution,
} from "@/lib/api";
import { toast } from "sonner";
import { Loader2, RefreshCcw, RotateCcw, Square } from "lucide-react";

const TERMINAL_CONNECTOR_EXECUTION_STATUSES = new Set(["completed", "dry_run", "failed", "cancelled", "dead_letter"]);

function connectorExecutionStatusClass(status: string): string {
  switch ((status || "").toLowerCase()) {
    case "completed":
      return "border-[rgba(34,197,94,0.35)] bg-[rgba(34,197,94,0.16)] text-[#86efac]";
    case "dry_run":
      return "border-[rgba(59,130,246,0.35)] bg-[rgba(59,130,246,0.16)] text-[#93c5fd]";
    case "provider_accepted":
      return "border-[rgba(59,130,246,0.35)] bg-[rgba(59,130,246,0.16)] text-[#bfdbfe]";
    case "queued":
    case "retry_scheduled":
    case "dispatching":
    case "accepted":
      return "border-[rgba(245,158,11,0.35)] bg-[rgba(245,158,11,0.16)] text-[#fbbf24]";
    case "failed":
    case "dead_letter":
    case "cancelled":
      return "border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.16)] text-[#fda4af]";
    default:
      return "border-[#2a2c3c] bg-[#0f1118] text-[#d1d5db]";
  }
}

function formatConnectorExecutionTime(value?: string): string {
  if (!value) return "-";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
}

function jsonPreview(value: unknown): string {
  if (value == null) return "{}";
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

export function ConnectorExecutionDrawer({
  executionId,
  open,
  onOpenChange,
  title = "Connector Execution",
  description = "Execution details, retry history, provider response, and live event log.",
}: {
  executionId: string;
  open: boolean;
  onOpenChange: (nextOpen: boolean) => void;
  title?: string;
  description?: string;
}) {
  const detailQuery = useConnectorHubExecutionDetail(executionId, {
    enabled: open && !!executionId,
    refetchInterval: open ? 2000 : false,
  });
  const execution = detailQuery.data?.execution;
  const executionStatus = String(execution?.status || "").toLowerCase();
  const isTerminal = TERMINAL_CONNECTOR_EXECUTION_STATUSES.has(executionStatus);
  const eventsQuery = useConnectorHubExecutionEvents(executionId, 100, {
    enabled: open && !!executionId,
    refetchInterval: open && !isTerminal ? 2000 : false,
  });
  const retryExecution = useRetryConnectorHubExecution();
  const cancelExecution = useCancelConnectorHubExecution();
  const restartExecution = useRestartConnectorHubExecution();
  const attempts = useMemo(() => (Array.isArray(detailQuery.data?.attempts) ? detailQuery.data.attempts : []), [detailQuery.data]);
  const events = useMemo(() => {
    if (Array.isArray(eventsQuery.data) && eventsQuery.data.length > 0) {
      return eventsQuery.data;
    }
    return Array.isArray(detailQuery.data?.events) ? detailQuery.data.events : [];
  }, [detailQuery.data, eventsQuery.data]);

  const connectorChannel = String(execution?.connector_channel || "").toLowerCase();
  const response = execution?.response || {};
  const responseMetadata = (response && typeof response === "object" ? (response as any).metadata : undefined) || {};
  const sqlRows = Array.isArray((responseMetadata as any).rows) ? (responseMetadata as any).rows as any[] : [];
  const sqlRowCount = Number(((responseMetadata as any).row_count ?? (responseMetadata as any).rowCount ?? sqlRows.length) || 0);
  const s3Meta = connectorChannel === "object_storage" ? (responseMetadata as any) : {};

  const pendingAction = retryExecution.isPending || cancelExecution.isPending || restartExecution.isPending;

  const handleRetry = () => {
    retryExecution.mutate(executionId, {
      onSuccess: () => toast.success("Execution re-queued"),
      onError: (error: any) => toast.error(error?.message || "Failed to retry execution"),
    });
  };

  const handleCancel = () => {
    cancelExecution.mutate(executionId, {
      onSuccess: () => toast.success("Execution cancelled"),
      onError: (error: any) => toast.error(error?.message || "Failed to cancel execution"),
    });
  };

  const handleRestart = () => {
    restartExecution.mutate(executionId, {
      onSuccess: () => toast.success("Execution restarted"),
      onError: (error: any) => toast.error(error?.message || "Failed to restart execution"),
    });
  };

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full border-l border-[#2a2c3c] bg-[#111622] p-0 text-[#f3f4f6] sm:max-w-2xl">
        <div className="flex h-full min-h-0 flex-col">
          <div className="border-b border-[#2a2c3c] px-6 py-5 pr-12">
            <SheetHeader className="space-y-1 text-left">
              <SheetTitle className="text-base font-semibold text-[#f3f4f6]">{title}</SheetTitle>
              <SheetDescription className="text-sm text-[#9ca3af]">{description}</SheetDescription>
            </SheetHeader>
          </div>

          <div className="flex min-h-0 flex-1 flex-col overflow-hidden px-6 py-5">
            {detailQuery.isLoading && !execution ? (
              <div className="flex flex-1 items-center justify-center text-sm text-[#9ca3af]">
                <Loader2 size={18} className="mr-2 animate-spin" /> Loading execution details...
              </div>
            ) : !execution ? (
              <div className="flex flex-1 items-center justify-center text-sm text-[#9ca3af]">Execution details are unavailable.</div>
            ) : (
              <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-hidden">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="space-y-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <Badge className={`rounded-lg border text-[10px] uppercase tracking-[0.14em] ${connectorExecutionStatusClass(executionStatus)}`}>
                        {execution.status}
                      </Badge>
                      <span className="text-xs text-[#9ca3af]">{execution.connector_name || execution.connector_id}</span>
                    </div>
                    <div className="text-lg font-semibold text-[#f3f4f6]">{execution.method_name || execution.action || execution.method_key || "connector_action"}</div>
                    <div className="text-xs text-[#9ca3af]">Execution ID: {execution.id}</div>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="h-9 rounded-xl border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db] hover:bg-[#171b2a]"
                      onClick={handleRetry}
                      disabled={pendingAction}
                    >
                      <RefreshCcw size={14} className="mr-2" /> Retry
                    </Button>
                    {!isTerminal ? (
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        className="h-9 rounded-xl border-[rgba(239,68,68,0.28)] bg-[rgba(239,68,68,0.14)] text-[#fda4af] hover:bg-[rgba(239,68,68,0.2)]"
                        onClick={handleCancel}
                        disabled={pendingAction}
                      >
                        <Square size={14} className="mr-2" /> Cancel
                      </Button>
                    ) : null}
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="h-9 rounded-xl border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db] hover:bg-[#171b2a]"
                      onClick={handleRestart}
                      disabled={pendingAction}
                    >
                      <RotateCcw size={14} className="mr-2" /> Restart
                    </Button>
                  </div>
                </div>

                <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                  {[
                    ["Connector", execution.connector_name || execution.connector_id],
                    ["Mode", execution.execution_mode || "manual"],
                    ["Provider status", execution.provider_status || "-"],
                    ["Attempts", `${execution.attempt_count || 0} / ${execution.max_attempts || 0}`],
                    ["External ID", execution.external_id || "-"],
                    ["Correlation ID", execution.correlation_id || "-"],
                    ["Created", formatConnectorExecutionTime(execution.created_at)],
                    ["Updated", formatConnectorExecutionTime(execution.updated_at)],
                    ["Next retry", formatConnectorExecutionTime(execution.next_attempt_at)],
                  ].map(([label, value]) => (
                    <div key={label} className="rounded-xl border border-[#2a2c3c] bg-[#0f1118] px-3 py-2.5">
                      <div className="text-[10px] uppercase tracking-[0.16em] text-[#6b7280]">{label}</div>
                      <div className="mt-1 break-all text-sm text-[#f3f4f6]">{value}</div>
                    </div>
                  ))}
                </div>

                {connectorChannel === "sql" && sqlRows.length > 0 ? (
                  <div className="rounded-xl border border-[#2a2c3c] bg-[#0f1118] px-4 py-3 text-sm text-[#e5e7eb]">
                    <div className="mb-2 flex items-center justify-between gap-2">
                      <div className="text-xs font-semibold uppercase tracking-[0.16em] text-[#9ca3af]">SQL result</div>
                      <Badge variant="outline" className="rounded-lg text-[10px]">{sqlRowCount} rows</Badge>
                    </div>
                    <div className="overflow-auto rounded-lg border border-[#202534] bg-[#0b0d13]">
                      <table className="min-w-full border-collapse text-left text-[11px] text-[#e5e7eb]">
                        <thead className="bg-[#111827]">
                          <tr>
                            {Object.keys(sqlRows[0] || {}).map((col) => (
                              <th key={col} className="border-b border-[#1f2937] px-2 py-1 font-semibold">{col}</th>
                            ))}
                          </tr>
                        </thead>
                        <tbody>
                          {sqlRows.slice(0, 5).map((row: any, rowIndex: number) => (
                            <tr key={rowIndex} className={rowIndex % 2 === 0 ? "bg-[#020617]" : "bg-[#020617]/80"}>
                              {Object.keys(sqlRows[0] || {}).map((col) => (
                                <td key={col} className="border-b border-[#111827] px-2 py-1 align-top">
                                  {String(row[col] ?? "").slice(0, 160) || "—"}
                                </td>
                              ))}
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                    {sqlRows.length > 5 ? (
                      <div className="mt-2 text-[11px] text-[#9ca3af]">Showing first 5 rows.</div>
                    ) : null}
                  </div>
                ) : null}

                {connectorChannel === "object_storage" ? (
                  <div className="rounded-xl border border-[#2a2c3c] bg-[#0f1118] px-4 py-3 text-sm text-[#e5e7eb]">
                    <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                      <div className="text-xs font-semibold uppercase tracking-[0.16em] text-[#9ca3af]">Object storage result</div>
                      <Badge variant="outline" className="rounded-lg text-[10px]">{String(s3Meta.content_type || s3Meta.contentType || "").substring(0, 40) || "unknown"}</Badge>
                    </div>
                    <div className="grid gap-2 sm:grid-cols-2 md:grid-cols-3 text-[11px] text-[#d1d5db]">
                      <div><span className="text-[#9ca3af]">Bucket:</span> {s3Meta.bucket || "-"}</div>
                      <div><span className="text-[#9ca3af]">Key:</span> {s3Meta.key || "-"}</div>
                      <div><span className="text-[#9ca3af]">Size:</span> {String(s3Meta.size ?? "-")}</div>
                      <div><span className="text-[#9ca3af]">ETag:</span> {s3Meta.etag || "-"}</div>
                    </div>
                    {response?.reply ? (
                      <div className="mt-2">
                        <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[#9ca3af]">Body preview</div>
                        <pre className="mt-1 max-h-[140px] overflow-auto whitespace-pre-wrap break-words rounded-lg border border-[#202534] bg-[#0b0d13] px-3 py-2 text-[11px] text-[#cbd5e1]">
                          {String(response.reply).slice(0, 2000)}
                        </pre>
                      </div>
                    ) : null}
                  </div>
                ) : null}

                {execution.error ? (
                  <div className="rounded-xl border border-[rgba(239,68,68,0.28)] bg-[rgba(239,68,68,0.12)] px-4 py-3 text-sm text-[#fda4af]">
                    {execution.error}
                  </div>
                ) : null}

                <div className="grid min-h-0 flex-1 gap-5 lg:grid-cols-[minmax(0,1.15fr)_minmax(300px,0.85fr)]">
                  <div className="flex min-h-0 flex-col gap-5 overflow-hidden">
                    <section className="min-h-0 flex-1 overflow-hidden rounded-2xl border border-[#2a2c3c] bg-[#0f1118]">
                      <div className="flex items-center justify-between border-b border-[#2a2c3c] px-4 py-3">
                        <div className="text-xs font-semibold uppercase tracking-[0.16em] text-[#9ca3af]">Event log</div>
                        <Badge variant="outline" className="rounded-lg text-[10px]">{events.length}</Badge>
                      </div>
                      <div className="max-h-[320px] space-y-2 overflow-y-auto px-4 py-3">
                        {events.length === 0 ? (
                          <div className="text-sm text-[#6b7280]">No events yet.</div>
                        ) : (
                          events.map((event: any) => (
                            <div key={event.id} className="rounded-xl border border-[#2a2c3c] bg-[#13141c] px-3 py-2.5">
                              <div className="flex items-center justify-between gap-3">
                                <div className="min-w-0">
                                  <div className="text-sm font-medium text-[#f3f4f6]">{event.message || event.event_type || "event"}</div>
                                  <div className="mt-1 text-[11px] text-[#6b7280]">{event.event_type || "event"}</div>
                                </div>
                                <Badge className={`rounded-lg border text-[10px] ${connectorExecutionStatusClass(String(event.status || ""))}`}>
                                  {event.status || "unknown"}
                                </Badge>
                              </div>
                              <div className="mt-2 text-[11px] text-[#9ca3af]">{formatConnectorExecutionTime(event.created_at)}</div>
                              {event.data && Object.keys(event.data).length > 0 ? (
                                <pre className="mt-2 overflow-x-auto whitespace-pre-wrap break-words rounded-lg border border-[#202534] bg-[#0b0d13] px-3 py-2 text-[11px] text-[#a5b4fc]">
                                  {jsonPreview(event.data)}
                                </pre>
                              ) : null}
                            </div>
                          ))
                        )}
                      </div>
                    </section>
                  </div>

                  <div className="flex min-h-0 flex-col gap-5 overflow-hidden">
                    <section className="overflow-hidden rounded-2xl border border-[#2a2c3c] bg-[#0f1118]">
                      <div className="flex items-center justify-between border-b border-[#2a2c3c] px-4 py-3">
                        <div className="text-xs font-semibold uppercase tracking-[0.16em] text-[#9ca3af]">Attempts</div>
                        <Badge variant="outline" className="rounded-lg text-[10px]">{attempts.length}</Badge>
                      </div>
                      <div className="max-h-[220px] space-y-2 overflow-y-auto px-4 py-3">
                        {attempts.length === 0 ? (
                          <div className="text-sm text-[#6b7280]">No attempts recorded.</div>
                        ) : (
                          attempts.map((attempt: any) => (
                            <div key={attempt.id} className="rounded-xl border border-[#2a2c3c] bg-[#13141c] px-3 py-2.5">
                              <div className="flex items-center justify-between gap-2">
                                <div className="text-sm font-medium text-[#f3f4f6]">Attempt #{attempt.attempt_no || attempt.attemptNo || "-"}</div>
                                <Badge className={`rounded-lg border text-[10px] ${connectorExecutionStatusClass(String(attempt.status || ""))}`}>
                                  {attempt.status || "unknown"}
                                </Badge>
                              </div>
                              <div className="mt-1 text-[11px] text-[#9ca3af]">{formatConnectorExecutionTime(attempt.finished_at || attempt.updated_at || attempt.started_at)}</div>
                              {attempt.error ? <div className="mt-2 text-[11px] text-[#fda4af]">{attempt.error}</div> : null}
                            </div>
                          ))
                        )}
                      </div>
                    </section>

                    <section className="min-h-0 overflow-hidden rounded-2xl border border-[#2a2c3c] bg-[#0f1118]">
                      <div className="px-4 py-3">
                        <div className="text-xs font-semibold uppercase tracking-[0.16em] text-[#9ca3af]">Request</div>
                        <pre className="mt-2 max-h-[180px] overflow-auto whitespace-pre-wrap break-words rounded-xl border border-[#202534] bg-[#0b0d13] px-3 py-3 text-[11px] text-[#cbd5e1]">
                          {jsonPreview(execution.request)}
                        </pre>
                      </div>
                      <Separator className="bg-[#2a2c3c]" />
                      <div className="px-4 py-3">
                        <div className="text-xs font-semibold uppercase tracking-[0.16em] text-[#9ca3af]">Response</div>
                        <pre className="mt-2 max-h-[180px] overflow-auto whitespace-pre-wrap break-words rounded-xl border border-[#202534] bg-[#0b0d13] px-3 py-3 text-[11px] text-[#cbd5e1]">
                          {jsonPreview(execution.response)}
                        </pre>
                      </div>
                    </section>
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}
