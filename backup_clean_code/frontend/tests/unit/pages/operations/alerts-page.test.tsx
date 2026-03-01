import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const hoisted = vi.hoisted(() => ({
  useAlertsPageMock: vi.fn(),
  updateAlertMutate: vi.fn(),
  deleteAlertMutate: vi.fn(),
  deleteAlertsBulkMutate: vi.fn(),
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
  setLocation: vi.fn(),
  location: "/alerts",
  createAlertMutate: vi.fn(),
  currentUser: { id: "user-1", name: "Alice" } as { id: string; name: string } | undefined,
  totalPages: 5,
  totalAlerts: 200,
  alerts: [
    {
      id: "alert-1",
      title: "Suspicious login",
      source: "siem",
      sev: "High",
      time: "2026-02-18T10:00:00.000Z",
      tags: ["AMBER"],
      owner: undefined,
      status: "New",
      caseId: undefined,
    },
    {
      id: "alert-2",
      title: "DNS anomaly",
      source: "edr",
      sev: "Medium",
      time: "2026-02-18T11:00:00.000Z",
      tags: ["GREEN"],
      owner: "user-2",
      status: "New",
      caseId: "case-1",
    },
  ],
  cases: [{ id: "case-1", title: "Case 1", caseNumber: "CASE-1" }],
  caseStatuses: [{ code: "open", label: "Open", isClosed: false }],
}));

vi.mock("@/components/layout", () => ({
  AppLayout: ({ children }: { children: any }) => <div data-testid="layout">{children}</div>,
}));

vi.mock("wouter", () => ({
  useLocation: () => [hoisted.location, hoisted.setLocation],
}));

vi.mock("sonner", () => ({
  toast: {
    success: hoisted.toastSuccess,
    error: hoisted.toastError,
  },
}));

vi.mock("@/lib/api", () => ({
  useAppState: () => ({ currentTenantId: "tenant-1", currentTenantSlug: "tenant-1", currentUserId: "user-1" }),
  useAlertsPage: hoisted.useAlertsPageMock,
  useUser: () => ({ data: hoisted.currentUser }),
  useUsers: () => ({ data: [{ id: "user-1", name: "Alice" }, { id: "user-2", name: "Bob" }] }),
  useUpdateAlert: () => ({ mutate: hoisted.updateAlertMutate, isPending: false }),
  useCreateAlert: () => ({ mutate: hoisted.createAlertMutate, isPending: false }),
  useCases: () => ({ data: hoisted.cases }),
  useBindAlertsToCase: () => ({ mutate: vi.fn(), isPending: false }),
  useCreateCaseFromAlerts: () => ({ mutate: vi.fn(), isPending: false }),
  useCaseStatuses: () => ({ data: hoisted.caseStatuses }),
  useDeleteAlert: () => ({ mutate: hoisted.deleteAlertMutate, isPending: false }),
  useDeleteAlertsBulk: () => ({ mutate: hoisted.deleteAlertsBulkMutate, isPending: false }),
}));

import AlertsPage from "@/pages/alerts";

describe("AlertsPage delete flows", () => {
  beforeEach(() => {
    hoisted.updateAlertMutate.mockReset();
    hoisted.createAlertMutate.mockReset();
    hoisted.deleteAlertMutate.mockReset();
    hoisted.deleteAlertsBulkMutate.mockReset();
    hoisted.toastSuccess.mockReset();
    hoisted.toastError.mockReset();
    hoisted.currentUser = { id: "user-1", name: "Alice" };
    hoisted.totalPages = 5;
    hoisted.totalAlerts = 200;
    hoisted.location = "/alerts";
    hoisted.useAlertsPageMock.mockReset();
    hoisted.useAlertsPageMock.mockImplementation((_tenantId: string, page: number, pageSize: number) => ({
      data: {
        items: hoisted.alerts,
        page,
        pageSize,
        total: hoisted.totalAlerts,
        totalPages: hoisted.totalPages,
      },
    }));

    hoisted.updateAlertMutate.mockImplementation((_payload: any, opts?: { onSuccess?: () => void }) => {
      opts?.onSuccess?.();
    });
    hoisted.deleteAlertMutate.mockImplementation((_id: string, opts?: { onSuccess?: () => void }) => {
      opts?.onSuccess?.();
    });
    hoisted.deleteAlertsBulkMutate.mockImplementation((ids: string[], opts?: { onSuccess?: (result: any) => void }) => {
      opts?.onSuccess?.({ deleted: ids.length, failed: 0 });
    });
  });

  it("deletes a single alert from card action", async () => {
    render(<AlertsPage />);

    fireEvent.click(screen.getByTestId("button-delete-alert-alert-1"));
    expect(screen.getByText("Delete this alert?")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));

    await waitFor(() => {
      expect(hoisted.deleteAlertMutate).toHaveBeenCalled();
    });
    expect(hoisted.deleteAlertMutate.mock.calls[0][0]).toBe("alert-1");
  });

  it("deletes selected alerts in bulk", async () => {
    render(<AlertsPage />);

    fireEvent.click(screen.getByTestId("checkbox-select-all-alerts"));
    fireEvent.click(screen.getByTestId("button-delete-selected-alerts"));
    expect(screen.getByText("Delete 2 selected alert(s)?")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));

    await waitFor(() => {
      expect(hoisted.deleteAlertsBulkMutate).toHaveBeenCalled();
    });
    const ids = hoisted.deleteAlertsBulkMutate.mock.calls[0][0] as string[];
    expect(ids).toContain("alert-1");
    expect(ids).toContain("alert-2");
  });

  it("skips alert delete when confirmation is cancelled", async () => {
    render(<AlertsPage />);

    fireEvent.click(screen.getByTestId("button-delete-alert-alert-1"));
    expect(screen.getByText("Delete this alert?")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await waitFor(() => {
      expect(screen.queryByText("Delete this alert?")).not.toBeInTheDocument();
    });
    expect(hoisted.deleteAlertMutate).not.toHaveBeenCalled();
  });

  it("keeps alerts list visible when current user profile is not loaded yet", () => {
    hoisted.currentUser = undefined;

    render(<AlertsPage />);

    expect(screen.getByTestId("button-delete-alert-alert-1")).toBeInTheDocument();
    expect(screen.getByText("Suspicious login")).toBeInTheDocument();
  });

  it("renders pagination at top and supports manual page input", async () => {
    render(<AlertsPage />);

    expect(screen.getByTestId("alerts-pagination-top")).toBeInTheDocument();
    const topInput = screen.getByTestId("input-alerts-page-top");
    fireEvent.change(topInput, { target: { value: "4" } });
    fireEvent.blur(topInput);

    await waitFor(() => {
      expect(screen.getByTestId("input-alerts-page-top")).toHaveValue("4");
    });

    fireEvent.click(screen.getByTestId("button-alerts-prev-page"));
    await waitFor(() => {
      expect(screen.getByTestId("input-alerts-page-top")).toHaveValue("3");
    });
  });

  it("creates alert from manual dialog", async () => {
    render(<AlertsPage />);

    fireEvent.click(screen.getByTestId("button-new-alert"));
    fireEvent.change(screen.getByTestId("input-new-alert-title"), { target: { value: "Manual alert" } });
    fireEvent.click(screen.getByTestId("button-create-alert-confirm"));

    await waitFor(() => {
      expect(hoisted.createAlertMutate).toHaveBeenCalled();
    });
  });

  it("uses backend fulltext mode without local token fallback filtering", async () => {
    hoisted.location = "/alerts?search_mode=fulltext&q=suspicious%20OR%20dns";
    render(<AlertsPage />);

    await waitFor(() => {
      expect(screen.getByText("Suspicious login")).toBeInTheDocument();
      expect(screen.getByText("DNS anomaly")).toBeInTheDocument();
    });
  });

  it("hydrates assignee query and forwards assigned_to filter to backend list hook", async () => {
    hoisted.location = "/alerts?assignee=user-2";
    render(<AlertsPage />);

    await waitFor(() => {
      const lastCall = hoisted.useAlertsPageMock.mock.calls.at(-1);
      expect(lastCall?.[9]).toBe("user-2");
    });
  });

  it("applies bulk close for selected alerts", async () => {
    render(<AlertsPage />);

    fireEvent.click(screen.getByTestId("checkbox-select-all-alerts"));
    fireEvent.click(screen.getByTestId("button-alerts-bulk-close"));

    await waitFor(() => {
      expect(hoisted.updateAlertMutate).toHaveBeenCalledTimes(2);
    });
    const firstCallPayload = hoisted.updateAlertMutate.mock.calls[0][0];
    expect(firstCallPayload.data.status).toBe("closed");
  });

  it("adds tag for selected alerts in bulk", async () => {
    render(<AlertsPage />);

    fireEvent.click(screen.getByTestId("checkbox-select-all-alerts"));
    fireEvent.change(screen.getByTestId("input-alerts-bulk-tag"), { target: { value: "priority" } });
    fireEvent.click(screen.getByTestId("button-alerts-bulk-add-tag"));

    await waitFor(() => {
      expect(hoisted.updateAlertMutate).toHaveBeenCalled();
    });
    const firstCallPayload = hoisted.updateAlertMutate.mock.calls[0][0];
    expect(firstCallPayload.data.tags).toContain("priority");
  });
});
