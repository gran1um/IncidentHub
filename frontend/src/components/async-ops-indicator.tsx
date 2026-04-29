import { AlertCircle, CheckCircle2, Loader2, X } from "lucide-react";
import { useAsyncOpsTracker, type AsyncUIOperation } from "@/lib/async-ops-tracker";
import { Button } from "@/components/ui/button";

function operationLabel(item: AsyncUIOperation): string {
  const op = String(item.operationType || "").trim().toLowerCase();
  const map: Record<string, string> = {
    "alert.create": "Alert creation",
    "alert.update": "Alert update",
    "alert.delete": "Alert deletion",
    "case.create": "Case creation",
    "case.update": "Case update",
    "case.delete": "Case deletion",
  };
  if (map[op]) {
    return map[op];
  }
  const resource = String(item.resource || "").trim();
  if (resource) {
    return `${resource[0].toUpperCase()}${resource.slice(1)} operation`;
  }
  return "Background operation";
}

function statusLabel(status: AsyncUIOperation["status"]): string {
  switch (status) {
    case "done":
      return "Completed";
    case "failed":
      return "Failed";
    case "queued":
      return "Queued";
    default:
      return "Processing";
  }
}

export function AsyncOpsIndicator() {
  const operations = useAsyncOpsTracker((state) => state.operations);
  const dismissOperation = useAsyncOpsTracker((state) => state.dismissOperation);
  const visibleOperations = operations.filter((item) => item.status !== "done");

  if (visibleOperations.length === 0) {
    return null;
  }

  return (
    <div className="fixed top-20 right-4 z-[80] flex w-[320px] max-w-[calc(100vw-2rem)] flex-col gap-2 pointer-events-none">
      {visibleOperations.map((item) => {
        const toneClass =
          item.status === "done"
            ? "border-emerald-300/80 bg-emerald-50/95 text-emerald-900 dark:border-emerald-500/50 dark:bg-emerald-950/80 dark:text-emerald-100"
            : item.status === "failed"
              ? "border-red-300/80 bg-red-50/95 text-red-900 dark:border-red-500/50 dark:bg-red-950/80 dark:text-red-100"
              : "border-primary/40 bg-background/95 text-foreground";
        return (
          <div
            key={item.operationId}
            className={`pointer-events-auto rounded-xl border px-3 py-2 shadow-lg backdrop-blur-sm ${toneClass}`}
            data-testid={`async-op-indicator-${item.operationId}`}
          >
            <div className="flex items-start gap-2">
              <div className="mt-0.5 shrink-0">
                {item.status === "done" ? (
                  <CheckCircle2 size={16} />
                ) : item.status === "failed" ? (
                  <AlertCircle size={16} />
                ) : (
                  <Loader2 size={16} className="animate-spin" />
                )}
              </div>
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-semibold">{operationLabel(item)}</p>
                <p className="truncate text-xs opacity-80">{statusLabel(item.status)}</p>
                {item.error && item.status === "failed" && (
                  <p className="mt-1 line-clamp-2 text-xs opacity-90">{item.error}</p>
                )}
              </div>
              <Button
                variant="ghost"
                size="icon"
                className="h-6 w-6 shrink-0"
                onClick={() => dismissOperation(item.operationId)}
                data-testid={`async-op-dismiss-${item.operationId}`}
              >
                <X size={12} />
              </Button>
            </div>
          </div>
        );
      })}
    </div>
  );
}
