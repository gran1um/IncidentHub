import { describe, expect, it } from "vitest";
import { applySearchPatch, splitLocationPathAndSearch } from "@/lib/url-state";

describe("url-state", () => {
  it("parses query directly from location string when present", () => {
    const { path, params } = splitLocationPathAndSearch("/admin/cases?page=2&page_size=30");
    expect(path).toBe("/admin/cases");
    expect(params.get("page")).toBe("2");
    expect(params.get("page_size")).toBe("30");
  });

  it("falls back to window.location.search when router location has no query", () => {
    window.history.replaceState({}, "", "/admin/alerts?page=3&page_size=50");
    const { path, params } = splitLocationPathAndSearch("/admin/alerts");
    expect(path).toBe("/admin/alerts");
    expect(params.get("page")).toBe("3");
    expect(params.get("page_size")).toBe("50");
  });

  it("can clear query params using browser search fallback", () => {
    window.history.replaceState({}, "", "/admin/cases?page=2");
    const next = applySearchPatch("/admin/cases", { page: undefined });
    expect(next).toBe("/admin/cases");
  });
});
