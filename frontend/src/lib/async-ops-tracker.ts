import { create } from "zustand";

export type AsyncUIStatus = "queued" | "processing" | "done" | "failed";

export type AsyncUIOperation = {
  operationId: string;
  operationType: string;
  resource: string;
  resourceId: string;
  status: AsyncUIStatus;
  error?: string;
  createdAt: number;
  updatedAt: number;
};

type AsyncOpsTrackerState = {
  operations: AsyncUIOperation[];
  upsertOperation: (entry: Partial<AsyncUIOperation> & { operationId: string }) => void;
  dismissOperation: (operationId: string) => void;
  clearAll: () => void;
};

const SUCCESS_AUTO_DISMISS_MS = 2800;
const FAILURE_AUTO_DISMISS_MS = 7000;
const MAX_TRACKED_OPERATIONS = 6;

const dismissTimers = new Map<string, ReturnType<typeof setTimeout>>();

function clearDismissTimer(operationId: string) {
  const handle = dismissTimers.get(operationId);
  if (handle) {
    clearTimeout(handle);
    dismissTimers.delete(operationId);
  }
}

function scheduleDismiss(operationId: string, timeoutMs: number) {
  clearDismissTimer(operationId);
  dismissTimers.set(
    operationId,
    setTimeout(() => {
      useAsyncOpsTracker.getState().dismissOperation(operationId);
    }, timeoutMs),
  );
}

export const useAsyncOpsTracker = create<AsyncOpsTrackerState>()((set) => ({
  operations: [],
  upsertOperation: (entry) =>
    set((state) => {
      const now = Date.now();
      const operationType = String(entry.operationType || "").trim();
      const resource = String(entry.resource || "").trim();
      const resourceId = String(entry.resourceId || "").trim();
      const status: AsyncUIStatus =
        entry.status === "done" || entry.status === "failed" || entry.status === "processing" || entry.status === "queued"
          ? entry.status
          : "queued";
      const error = String(entry.error || "").trim() || undefined;

      const next: AsyncUIOperation = {
        operationId: entry.operationId,
        operationType,
        resource,
        resourceId,
        status,
        error,
        createdAt: now,
        updatedAt: now,
      };
      const index = state.operations.findIndex((item) => item.operationId === entry.operationId);
      const operations = [...state.operations];
      if (index >= 0) {
        const current = operations[index];
        operations[index] = {
          ...current,
          ...next,
          createdAt: current.createdAt || now,
          updatedAt: now,
        };
      } else {
        operations.unshift(next);
      }

      if (status === "done") {
        scheduleDismiss(entry.operationId, SUCCESS_AUTO_DISMISS_MS);
      } else if (status === "failed") {
        scheduleDismiss(entry.operationId, FAILURE_AUTO_DISMISS_MS);
      } else {
        clearDismissTimer(entry.operationId);
      }

      return { operations: operations.slice(0, MAX_TRACKED_OPERATIONS) };
    }),
  dismissOperation: (operationId) =>
    set((state) => {
      clearDismissTimer(operationId);
      return { operations: state.operations.filter((item) => item.operationId !== operationId) };
    }),
  clearAll: () =>
    set(() => {
      for (const [operationId] of dismissTimers) {
        clearDismissTimer(operationId);
      }
      return { operations: [] };
    }),
}));

