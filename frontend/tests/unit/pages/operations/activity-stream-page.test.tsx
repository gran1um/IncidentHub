import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

const hoisted = vi.hoisted(() => ({
  location: "/tenant-1/activity",
  setLocation: vi.fn(),
  useActivityLivestreamMock: vi.fn(),
  items: [
    {
      id: "case:1",
      entity: "case",
      entityId: "case-1",
      action: "updated",
      title: "Case updated",
      description: "desc",
      severity: "high",
      status: "open",
      source: "manual",
      assigneeId: "user-1",
      caseId: "",
      createdAt: "2026-02-20T10:00:00Z",
      updatedAt: "2026-02-20T10:05:00Z",
    },
  ],
}));

vi.mock("@/components/layout", () => ({
  AppLayout: ({ children }: { children: any }) => <div data-testid="layout">{children}</div>,
}));

vi.mock("wouter", () => ({
  useLocation: () => [hoisted.location, hoisted.setLocation],
}));

vi.mock("@/lib/api", () => ({
  useAppState: () => ({ currentTenantId: "tenant-1", currentTenantSlug: "tenant-1" }),
  useUsers: () => ({
    data: [
      { id: "user-1", name: "Alice Analyst", email: "alice@example.com" },
      { id: "user-2", name: "Bob Analyst", email: "bob@example.com" },
    ],
  }),
  useActivityLivestream: hoisted.useActivityLivestreamMock,
}));

import ActivityStreamPage from "@/pages/activity-stream";

describe("ActivityStreamPage", () => {
  it("renders livestream block with events and opens linked object", async () => {
    const user = userEvent.setup();
    hoisted.location = "/tenant-1/activity";
    hoisted.setLocation.mockReset();
    hoisted.useActivityLivestreamMock.mockReset();
    hoisted.useActivityLivestreamMock.mockImplementation(() => ({
      data: { items: hoisted.items, generatedAt: "2026-02-20T10:05:00Z" },
    }));

    render(<ActivityStreamPage />);

    expect(screen.getByText("Live Activity Stream")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Filter events...")).toBeInTheDocument();
    expect(screen.getByText("Case updated")).toBeInTheDocument();
    expect(screen.getByText("MANUAL")).toBeInTheDocument();

    await user.click(screen.getByTestId("activity-open-case:1"));
    expect(hoisted.setLocation).toHaveBeenCalledWith("/tenant-1/cases/case-1");
  });

  it("passes filter values into activity query", async () => {
    const user = userEvent.setup();
    hoisted.location = "/tenant-1/activity";
    hoisted.useActivityLivestreamMock.mockReset();
    hoisted.useActivityLivestreamMock.mockImplementation(() => ({
      data: { items: hoisted.items, generatedAt: "2026-02-20T10:05:00Z" },
    }));

    render(<ActivityStreamPage />);

    fireEvent.change(screen.getByTestId("activity-filter-search"), { target: { value: "credential" } });
    await user.click(screen.getByTestId("activity-filter-assignee"));
    await user.click(screen.getByRole("option", { name: "Bob Analyst" }));
    await user.click(screen.getByTestId("activity-filter-limit"));
    await user.click(screen.getByRole("option", { name: "50 events" }));
    await user.click(screen.getByTestId("activity-filter-type-task"));

    const lastCall = hoisted.useActivityLivestreamMock.mock.calls.at(-1);
    expect(lastCall?.[0]).toBe("tenant-1");
    expect(lastCall?.[1]).toMatchObject({
      q: "credential",
      assigneeId: "user-2",
      limit: 50,
      types: ["case", "alert"],
      refetchInterval: 3000,
      refetchOnWindowFocus: true,
    });
  });

  it("hydrates filter state from url query and updates query options", async () => {
    hoisted.location = "/tenant-1/activity?q=dns&assignee=user-2&limit=50&types=case,task&paused=1";
    hoisted.useActivityLivestreamMock.mockReset();
    hoisted.useActivityLivestreamMock.mockImplementation(() => ({
      data: { items: hoisted.items, generatedAt: "2026-02-20T10:05:00Z" },
    }));

    render(<ActivityStreamPage />);

    await waitFor(() => {
      const lastCall = hoisted.useActivityLivestreamMock.mock.calls.at(-1);
      expect(lastCall?.[1]).toMatchObject({
        q: "dns",
        assigneeId: "user-2",
        limit: 50,
        types: ["case", "task"],
        refetchInterval: 0,
        refetchOnWindowFocus: false,
      });
    });
  });
});
