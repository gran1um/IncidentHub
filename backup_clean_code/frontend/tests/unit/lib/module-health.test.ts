import { describe, expect, it } from "vitest";

import { formatModuleOperationLatency, formatModuleResponseMs, formatOperationWindow } from "@/lib/module-health";

describe("formatModuleResponseMs", () => {
  it("returns n/a for disabled modules", () => {
    expect(formatModuleResponseMs(42, "disabled")).toBe("n/a");
  });

  it("returns n/a for invalid values", () => {
    expect(formatModuleResponseMs(Number.NaN, "ok")).toBe("n/a");
    expect(formatModuleResponseMs(-1, "ok")).toBe("n/a");
  });

  it("keeps zero as zero", () => {
    expect(formatModuleResponseMs(0, "ok")).toBe("0 ms");
  });

  it("rounds positive latencies to whole milliseconds", () => {
    expect(formatModuleResponseMs(0.23, "ok")).toBe("1 ms");
    expect(formatModuleResponseMs(4.6, "ok")).toBe("5 ms");
    expect(formatModuleResponseMs(12.2, "ok")).toBe("12 ms");
  });

  it("formats operation latency only when operations exist", () => {
    expect(formatModuleOperationLatency(118.6, 3)).toBe("119 ms avg");
    expect(formatModuleOperationLatency(118.6, 0)).toBe("n/a");
  });

  it("formats operation windows", () => {
    expect(formatOperationWindow(300)).toBe("5m");
    expect(formatOperationWindow(45)).toBe("45s");
  });
});
