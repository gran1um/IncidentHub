export function formatModuleResponseMs(value: number, status?: string): string {
  if (String(status || "").toLowerCase() === "disabled") {
    return "n/a";
  }
  if (!Number.isFinite(value) || value < 0) {
    return "n/a";
  }
  if (value === 0) {
    return "0 ms";
  }
  return `${Math.max(1, Math.round(value))} ms`;
}

export function formatModuleOperationLatency(value: number, operations?: number): string {
  if (!Number.isFinite(value) || value < 0 || !Number.isFinite(operations) || Number(operations || 0) <= 0) {
    return "n/a";
  }
  return `${Math.max(1, Math.round(value))} ms avg`;
}

export function formatOperationWindow(windowSeconds: number): string {
  if (!Number.isFinite(windowSeconds) || windowSeconds <= 0) {
    return "";
  }
  if (windowSeconds % 60 === 0) {
    return `${Math.round(windowSeconds / 60)}m`;
  }
  return `${Math.round(windowSeconds)}s`;
}
