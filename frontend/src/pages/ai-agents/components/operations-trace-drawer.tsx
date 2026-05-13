import type { ReactNode } from "react";

import { Activity, ArrowUpRight, Loader2, X } from "lucide-react";

import { AIEntityTrace, type AIEntityTraceActionContext, type AIEntityTraceLabels } from "@/components/ai-entity-trace";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

import {
  buildEntityLabel,
  buildEntityOpenLabel,
  formatTraceEventTime,
  formatTraceStatusLabel,
  traceStatusBadgeClass,
  type OperationsDrawerEventLogEntry,
  type OperationsTraceDrawerState,
} from "../helpers";

export function OperationsTraceDrawer({
  drawer,
  onClose,
  subtitleFallback,
  subpanelClass,
  outlineButtonClass,
  mutedTextClass,
  isRefreshing,
  lastUpdatedLabel,
  entityLocation,
  onOpenEntity,
  onRefresh,
  focusedWorkload,
  eventLog,
  resolvedTrace,
  entityTraceLoading,
  traceLabels,
  pendingWorkloadAction,
  onRestartWorkload,
  onCloseWorkload,
  renderExtraWorkloadActions,
  focusWorkloadId,
  focusStageKey,
}: {
  drawer: OperationsTraceDrawerState | null;
  onClose: () => void;
  subtitleFallback: string;
  subpanelClass: string;
  outlineButtonClass: string;
  mutedTextClass: string;
  isRefreshing: boolean;
  lastUpdatedLabel: string;
  entityLocation: string;
  onOpenEntity: () => void;
  onRefresh: () => void;
  focusedWorkload: any;
  eventLog: OperationsDrawerEventLogEntry[];
  resolvedTrace: any;
  entityTraceLoading: boolean;
  traceLabels: AIEntityTraceLabels;
  pendingWorkloadAction: { workloadId: string; action: "restart" | "close" } | null;
  onRestartWorkload: (workloadId: string) => void;
  onCloseWorkload: (workloadId: string) => void;
  renderExtraWorkloadActions?: (context: AIEntityTraceActionContext) => ReactNode;
  focusWorkloadId: string;
  focusStageKey: string;
}) {
  if (!drawer) {
    return null;
  }

  return (
    <>
      <button
        type="button"
        aria-label="Close operations trace drawer"
        className="fixed inset-0 z-40 bg-black/45 backdrop-blur-[1px]"
        onClick={onClose}
      />
      <div className="fixed inset-y-0 right-0 z-50 flex w-full justify-end p-3 sm:p-4" data-testid="operations-trace-drawer">
        <div className="flex h-full w-full max-w-2xl flex-col rounded-[24px] border border-[#2a2c3c] bg-[#111622] shadow-[0_20px_60px_rgba(0,0,0,0.45)]">
          <div className="relative border-b border-[#2a2c3c] px-5 py-5 pr-14">
            <button
              type="button"
              className="absolute right-8 top-8 rounded-md border border-[#2a2c3c] bg-[#0f131d] p-2 text-[#c7cedf] transition-colors hover:bg-[#171b2a] hover:text-white"
              data-testid="button-operations-trace-drawer-close"
              onClick={onClose}
            >
              <X size={16} />
            </button>
            <div className="space-y-1 text-left">
              <p className="text-base font-semibold text-white">{drawer.title || "AI workload trace"}</p>
              <p className="text-sm text-[#8b91a3]">{drawer.subtitle || subtitleFallback}</p>
            </div>
            <div className="mt-3 flex flex-wrap items-center gap-2">
              <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db]">{buildEntityLabel(drawer.entityType)}</Badge>
              {drawer.entityId ? (
                <Badge className="border border-[#2a2c3c] bg-[#0f131d] font-mono text-[#9ca3af]">{drawer.entityId}</Badge>
              ) : null}
              {entityLocation ? (
                <Button
                  type="button"
                  size="sm"
                  className={outlineButtonClass}
                  data-testid="button-operations-trace-drawer-open-entity"
                  onClick={onOpenEntity}
                >
                  <ArrowUpRight size={14} className="mr-1.5" />
                  {buildEntityOpenLabel(drawer.entityType)}
                </Button>
              ) : null}
            </div>
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto p-5">
            <div className={`${subpanelClass} mb-4 p-4`} data-testid="operations-trace-drawer-event-log">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <p className="text-sm font-semibold text-white">Live Event Log</p>
                  <p className={mutedTextClass}>Auto-refresh every 2s while this drawer stays open.</p>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <Badge className={isRefreshing ? "border border-[rgba(59,130,246,0.3)] bg-[rgba(59,130,246,0.14)] text-[#93c5fd]" : "border border-[rgba(34,197,94,0.28)] bg-[rgba(34,197,94,0.16)] text-[#86efac]"}>
                    {isRefreshing ? <Loader2 size={12} className="mr-1.5 animate-spin" /> : <Activity size={12} className="mr-1.5" />}
                    {isRefreshing ? "Refreshing" : "Live"}
                  </Badge>
                  <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#9ca3af] font-mono tabular-nums w-[176px] justify-center">
                    {lastUpdatedLabel ? `Updated ${lastUpdatedLabel}` : "Waiting for update"}
                  </Badge>
                  <Button type="button" size="sm" className={outlineButtonClass} data-testid="button-operations-trace-drawer-refresh" onClick={onRefresh}>
                    {isRefreshing ? <Loader2 size={14} className="mr-1.5 animate-spin" /> : <Activity size={14} className="mr-1.5" />}
                    Refresh
                  </Button>
                </div>
              </div>
              {focusedWorkload ? (
                <div className="mt-3 flex flex-wrap items-center gap-2">
                  <Badge className={traceStatusBadgeClass(focusedWorkload?.status)}>{formatTraceStatusLabel(focusedWorkload?.status)}</Badge>
                  <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db]">
                    {String(focusedWorkload?.agent_name || focusedWorkload?.agent_id || "AI agent")}
                  </Badge>
                  <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#9ca3af]">
                    attempts {Number(focusedWorkload?.attempt_count ?? 0)}/{Number(focusedWorkload?.max_attempts ?? 0)}
                  </Badge>
                  {String(focusedWorkload?.last_stage || "").trim() ? (
                    <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#9ca3af]">
                      stage {String(focusedWorkload.last_stage).trim()}
                    </Badge>
                  ) : null}
                </div>
              ) : null}
              <div className="mt-3 max-h-[220px] space-y-2 overflow-y-auto pr-1">
                {eventLog.length > 0 ? (
                  eventLog.map((entry, index) => (
                    <div key={entry.id} className="rounded-lg border border-[#23283a] bg-[#0b1019] px-3 py-2.5" data-testid={`operations-trace-event-entry-${index}`}>
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0">
                          <div className="flex flex-wrap items-center gap-2">
                            <Badge className={traceStatusBadgeClass(entry.status)}>{formatTraceStatusLabel(entry.status)}</Badge>
                            <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#9ca3af]">{entry.kind}</Badge>
                            <p className="truncate text-sm font-medium text-white">{entry.title}</p>
                          </div>
                          <p className="mt-1 text-xs text-[#9ca3af]">{entry.description}</p>
                        </div>
                        <p className={`${mutedTextClass} whitespace-nowrap font-mono tabular-nums w-[88px] text-right`}>{formatTraceEventTime(entry.timestamp)}</p>
                      </div>
                    </div>
                  ))
                ) : (
                  <div className="rounded-lg border border-dashed border-[#2a2c3c] bg-[#0b1019] px-3 py-4 text-sm text-[#9ca3af]">
                    Waiting for workload events. New stages and connector calls will appear here automatically.
                  </div>
                )}
              </div>
            </div>
            <AIEntityTrace
              trace={resolvedTrace}
              isLoading={entityTraceLoading && !Array.isArray(resolvedTrace?.events)}
              labels={traceLabels}
              testIdPrefix="ops-drawer-ai-trace"
              canManageWorkloads
              pendingWorkloadAction={pendingWorkloadAction}
              onRestartWorkload={onRestartWorkload}
              onCloseWorkload={onCloseWorkload}
              renderExtraWorkloadActions={renderExtraWorkloadActions}
              focusWorkloadId={focusWorkloadId}
              focusStageKey={focusStageKey}
            />
          </div>
        </div>
      </div>
    </>
  );
}
