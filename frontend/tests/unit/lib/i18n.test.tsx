import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";

import { useI18n, useT } from "@/lib/i18n";

describe("i18n", () => {
  beforeEach(() => {
    useI18n.setState({ language: "en" });
    localStorage.removeItem("i18n-storage");
  });

  it("returns english translations by default", () => {
    const { result } = renderHook(() => useT());
    expect(result.current("alerts.title")).toBe("Alerts");
  });

  it("switches to russian language", () => {
    useI18n.getState().setLanguage("ru");
    const { result } = renderHook(() => useT());
    expect(result.current("alerts.title")).toBe("Алерты");
  });

  it("interpolates params and falls back to key if missing", () => {
    const { result } = renderHook(() => useT());
    expect(result.current("alerts.bulk.selected", { count: "3" })).toBe("Selected: 3");
    expect(result.current("non.existent.key")).toBe("non.existent.key");
  });
});

