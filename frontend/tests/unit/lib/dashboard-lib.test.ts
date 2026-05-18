import { describe, expect, it } from "vitest";

import { buildResolutionBySeverity } from "@/lib/dashboard";

describe("buildResolutionBySeverity weekly mode", () => {
  it("uses static Monday-to-Sunday ordering for current week", () => {
    const now = new Date(2026, 1, 18, 12, 0, 0, 0); // Wed, 2026-02-18
    const data = buildResolutionBySeverity([], "week", now, { locale: "en-US" });

    expect(data).toHaveLength(7);
    expect(data.map((item) => item.name)).toEqual(["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]);
  });

  it("counts only resolved/closed cases that belong to current Monday-Sunday week", () => {
    const now = new Date(2026, 1, 18, 12, 0, 0, 0); // Wed, 2026-02-18
    const data = buildResolutionBySeverity(
      [
        { status: "resolved", severity: "high", updatedAt: "2026-02-16T10:00:00" }, // Mon
        { status: "closed", severity: "low", updatedAt: "2026-02-18T11:00:00" }, // Wed
        { status: "resolved", severity: "critical", updatedAt: "2026-02-15T09:00:00" }, // prev week
        { status: "new", severity: "critical", updatedAt: "2026-02-17T09:00:00" }, // non-resolved
      ],
      "week",
      now,
      { locale: "en-US" },
    );

    expect(data[0].High).toBe(1);
    expect(data[2].Low).toBe(1);
    expect(data[0].Critical).toBe(0);
    expect(data[1].Critical).toBe(0);
    expect(data[2].Critical).toBe(0);
    expect(data[6].Critical).toBe(0);
  });
});
