import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const hoisted = vi.hoisted(() => ({
  useCasesPageMock: vi.fn(),
  updateCaseMutate: vi.fn(),
  createCaseMutate: vi.fn(),
  deleteCaseMutate: vi.fn(),
  deleteCasesBulkMutate: vi.fn(),
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
  setLocation: vi.fn(),
  location: "/cases",
  currentUser: { id: "user-1", name: "Alice" } as { id: string; name: string } | undefined,
  totalPages: 4,
  totalCases: 120,
  caseStatuses: [
    { code: "open", label: "Open", isClosed: false },
    { code: "closed", label: "Closed", isClosed: true },
  ],
  cases: [
    {
      id: "case-1",
      title: "Credential leak",
      owner: undefined,
      sev: "High",
      incidentType: "credential_access",
      stage: "analysis",
      customFields: { inbound_event: "event_credential", observables_count: "1", client: "alpha" },
      status: "Open",
      time: "2026-02-18T10:00:00.000Z",
      tags: ["AMBER"],
    },
    {
      id: "case-2",
      title: "Malware triage",
      owner: "user-2",
      sev: "Critical",
      incidentType: "malware",
      stage: "response",
      customFields: { inbound_event: "event_malware", observables_count: "0", client: "beta" },
      status: "Open",
      time: "2026-02-18T12:00:00.000Z",
      tags: ["RED"],
    },
    {
      id: "case-3",
      title: "Resolved access review",
      owner: "user-2",
      sev: "Low",
      incidentType: "access_review",
      stage: "closed",
      customFields: { inbound_event: "event_access_review", observables_count: "0", client: "gamma" },
      status: "Closed",
      time: "2026-02-17T12:00:00.000Z",
      tags: ["GREEN"],
    },
  ],
}));

vi.mock("@/components/layout", () => ({
  AppLayout: ({ children }: { children: any }) => <div data-testid="layout">{children}</div>,
}));

vi.mock("wouter", () => ({
  useLocation: () => [hoisted.location, hoisted.setLocation],
  Link: ({ children, href }: { children: any; href: string }) => <a href={href}>{children}</a>,
}));

vi.mock("sonner", () => ({
  toast: {
    success: hoisted.toastSuccess,
    error: hoisted.toastError,
  },
}));

vi.mock("@/lib/api", () => ({
  useAppState: () => ({ currentTenantId: "tenant-1", currentTenantSlug: "tenant-1", currentUserId: "user-1" }),
  useCaseStatuses: () => ({ data: hoisted.caseStatuses }),
  useCaseTemplates: () => ({ data: [] }),
  useCases: () => ({ data: hoisted.cases }),
  useCasesPage: hoisted.useCasesPageMock,
  useCasesSummaries: (_tenantId: string, caseIds: string[]) => ({
    data: caseIds.map((caseId: string) => ({
      caseId,
      openTasks: caseId === "case-1" ? 1 : 0,
      closedTasks: caseId === "case-1" ? 0 : 1,
      totalTasks: 1,
      taskProgressPercent: caseId === "case-1" ? 0 : 100,
      responderStatuses: { queued: 1, running: 0, completed: 0, failed: 0 },
    })),
  }),
  useUser: () => ({ data: hoisted.currentUser }),
  useUsers: () => ({ data: [{ id: "user-1", name: "Alice", team: "SOC" }, { id: "user-2", name: "Bob", team: "Fraud" }] }),
  useUpdateCase: () => ({ mutate: hoisted.updateCaseMutate, isPending: false }),
  useCreateCase: () => ({ mutate: hoisted.createCaseMutate, isPending: false }),
  useCreateCaseTask: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteCase: () => ({ mutate: hoisted.deleteCaseMutate, isPending: false }),
  useDeleteCasesBulk: () => ({ mutate: hoisted.deleteCasesBulkMutate, isPending: false }),
}));

import CasesPage from "@/pages/cases";

describe("CasesPage delete flows", () => {
  beforeEach(() => {
    hoisted.updateCaseMutate.mockReset();
    hoisted.createCaseMutate.mockReset();
    hoisted.deleteCaseMutate.mockReset();
    hoisted.deleteCasesBulkMutate.mockReset();
    hoisted.toastSuccess.mockReset();
    hoisted.toastError.mockReset();
    hoisted.currentUser = { id: "user-1", name: "Alice" };
    hoisted.totalPages = 4;
    hoisted.totalCases = 120;
    hoisted.location = "/cases";
    hoisted.useCasesPageMock.mockReset();
    hoisted.useCasesPageMock.mockImplementation((_tenantId: string, page: number, pageSize: number) => ({
      data: {
        items: hoisted.cases,
        page,
        pageSize,
        total: hoisted.totalCases,
        totalPages: hoisted.totalPages,
      },
    }));
    window.localStorage.clear();

    hoisted.updateCaseMutate.mockImplementation((_payload: any, opts?: { onSuccess?: () => void }) => {
      opts?.onSuccess?.();
    });
    hoisted.deleteCaseMutate.mockImplementation((_id: string, opts?: { onSuccess?: () => void }) => {
      opts?.onSuccess?.();
    });
    hoisted.deleteCasesBulkMutate.mockImplementation((ids: string[], opts?: { onSuccess?: (result: any) => void }) => {
      opts?.onSuccess?.({ deleted: ids.length, failed: 0 });
    });
  });

  it("requires integration fields before creating case", async () => {
    render(<CasesPage />);

    fireEvent.click(screen.getByTestId("button-new-case"));
    fireEvent.change(screen.getByTestId("input-new-case-title"), { target: { value: "Case with integration fields" } });
    fireEvent.click(screen.getByTestId("button-create-case-confirm"));

    await waitFor(() => {
      expect(hoisted.toastError).toHaveBeenCalled();
    });
    expect(hoisted.createCaseMutate).not.toHaveBeenCalled();
  });

  it("adds SOAR preset fields to create-case custom fields", async () => {
    render(<CasesPage />);

    fireEvent.click(screen.getByTestId("button-new-case"));
    fireEvent.click(screen.getByTestId("button-add-new-case-soar-fields"));

    expect(screen.getByDisplayValue("analyst")).toBeInTheDocument();
    expect(screen.getByDisplayValue("indicators")).toBeInTheDocument();
    expect(screen.getByDisplayValue("status")).toBeInTheDocument();
    expect(screen.getByDisplayValue("stages")).toBeInTheDocument();
    expect(screen.getByDisplayValue("description")).toBeInTheDocument();
    expect(screen.getByDisplayValue("inbound_event")).toBeInTheDocument();
    expect(screen.getByDisplayValue("criticality")).toBeInTheDocument();
    expect(screen.getByDisplayValue("role_types")).toBeInTheDocument();
  });

  it("deletes single case from card action", async () => {
    render(<CasesPage />);

    fireEvent.click(screen.getByTestId("button-delete-case-case-1"));
    expect(screen.getByText("Delete this case?")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));

    await waitFor(() => {
      expect(hoisted.deleteCaseMutate).toHaveBeenCalled();
    });
    expect(hoisted.deleteCaseMutate.mock.calls[0][0]).toBe("case-1");
  });

  it("deletes selected cases in bulk", async () => {
    render(<CasesPage />);

    fireEvent.click(screen.getByTestId("checkbox-select-all-cases"));
    fireEvent.click(screen.getByTestId("button-delete-selected-cases"));
    expect(screen.getByText("Delete 2 selected case(s)?")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));

    await waitFor(() => {
      expect(hoisted.deleteCasesBulkMutate).toHaveBeenCalled();
    });
    const ids = hoisted.deleteCasesBulkMutate.mock.calls[0][0] as string[];
    expect(ids).toContain("case-1");
    expect(ids).toContain("case-2");
  });

  it("skips delete when confirmation is cancelled", async () => {
    render(<CasesPage />);

    fireEvent.click(screen.getByTestId("button-delete-case-case-1"));
    expect(screen.getByText("Delete this case?")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await waitFor(() => {
      expect(screen.queryByText("Delete this case?")).not.toBeInTheDocument();
    });
    expect(hoisted.deleteCaseMutate).not.toHaveBeenCalled();
  });

  it("keeps cases list visible when current user profile is not loaded yet", () => {
    hoisted.currentUser = undefined;

    render(<CasesPage />);

    expect(screen.getByTestId("button-delete-case-case-1")).toBeInTheDocument();
    expect(screen.getByText("Credential leak")).toBeInTheDocument();
  });

  it("renders pagination at top and supports manual page input", async () => {
    render(<CasesPage />);

    expect(screen.getByTestId("cases-pagination-top")).toBeInTheDocument();
    const topInput = screen.getByTestId("input-cases-page-top");
    fireEvent.change(topInput, { target: { value: "3" } });
    fireEvent.blur(topInput);

    await waitFor(() => {
      expect(screen.getByTestId("input-cases-page-top")).toHaveValue("3");
    });

    fireEvent.click(screen.getByTestId("button-cases-prev-page"));
    await waitFor(() => {
      expect(screen.getByTestId("input-cases-page-top")).toHaveValue("2");
    });
  });

  it("hides closed cases by default and loads them by button", async () => {
    render(<CasesPage />);

    expect(screen.queryByText("Resolved access review")).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId("button-cases-toggle-closed"));
    await waitFor(() => {
      expect(screen.getByText("Resolved access review")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("button-cases-toggle-closed"));
    await waitFor(() => {
      expect(screen.queryByText("Resolved access review")).not.toBeInTheDocument();
    });
  });

  it("applies include/exclude and rule filters on cases list", async () => {
    render(<CasesPage />);

    expect(screen.getByText("Credential leak")).toBeInTheDocument();
    expect(screen.getByText("Malware triage")).toBeInTheDocument();

    fireEvent.change(screen.getByTestId("input-cases-search-exclude"), { target: { value: "credential" } });

    await waitFor(() => {
      expect(screen.queryByText("Credential leak")).not.toBeInTheDocument();
      expect(screen.getByText("Malware triage")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByTestId("input-cases-rule"), { target: { value: "malware" } });

    await waitFor(() => {
      expect(screen.queryByText("Credential leak")).not.toBeInTheDocument();
      expect(screen.getByText("Malware triage")).toBeInTheDocument();
    });
  });

  it("renders case task progress and supports tag filter from all-tags panel", async () => {
    render(<CasesPage />);

    expect(screen.getByTestId("case-task-progress-case-1")).toBeInTheDocument();
    expect(screen.getByTestId("case-task-progress-case-2")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /AMBER/i }));

    await waitFor(() => {
      expect(screen.getByText("Credential leak")).toBeInTheDocument();
      expect(screen.queryByText("Malware triage")).not.toBeInTheDocument();
    });
  });

  it("supports visible fields configuration for case cards", async () => {
    render(<CasesPage />);

    expect(screen.getByText("case-1")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("button-case-visible-fields"));
    fireEvent.click(screen.getByText("ID"));

    await waitFor(() => {
      expect(screen.queryByText("case-1")).not.toBeInTheDocument();
    });
  });

  it("sorts by custom field from URL state", async () => {
    hoisted.location = "/cases?sort_by=cf:client&sort_order=desc";
    render(<CasesPage />);

    await waitFor(() => {
      const cards = screen.getAllByTestId(/case-card-/);
      expect(cards[0]).toHaveAttribute("data-testid", "case-card-case-2");
      expect(cards[1]).toHaveAttribute("data-testid", "case-card-case-1");
    });
  });

  it("shows count of non-empty custom field values in case card", async () => {
    render(<CasesPage />);

    await waitFor(() => {
      expect(screen.getAllByText("Custom values: 3")).toHaveLength(2);
    });
  });

  it("uses backend fulltext mode without local include/exclude fallback filtering", async () => {
    hoisted.location = "/cases?search_mode=fulltext&q=credential%20OR%20malware";
    render(<CasesPage />);

    await waitFor(() => {
      expect(screen.getByText("Credential leak")).toBeInTheDocument();
      expect(screen.getByText("Malware triage")).toBeInTheDocument();
    });
  });

  it("hydrates assignee query and forwards assigned_to filter to backend list hook", async () => {
    hoisted.location = "/cases?assignee=user-2";
    render(<CasesPage />);

    await waitFor(() => {
      const lastCall = hoisted.useCasesPageMock.mock.calls.at(-1);
      expect(lastCall?.[10]).toBe("user-2");
    });
  });

  it("applies bulk close for selected cases", async () => {
    render(<CasesPage />);

    fireEvent.click(screen.getByTestId("checkbox-select-all-cases"));
    fireEvent.click(screen.getByTestId("button-cases-bulk-close"));

    await waitFor(() => {
      expect(hoisted.updateCaseMutate).toHaveBeenCalledTimes(2);
    });
    const firstCallPayload = hoisted.updateCaseMutate.mock.calls[0][0];
    expect(firstCallPayload.data.status).toBe("closed");
  });

  it("adds tag for selected cases in bulk", async () => {
    render(<CasesPage />);

    fireEvent.click(screen.getByTestId("checkbox-select-all-cases"));
    fireEvent.change(screen.getByTestId("input-cases-bulk-tag"), { target: { value: "investigation" } });
    fireEvent.click(screen.getByTestId("button-cases-bulk-add-tag"));

    await waitFor(() => {
      expect(hoisted.updateCaseMutate).toHaveBeenCalled();
    });
    const firstCallPayload = hoisted.updateCaseMutate.mock.calls[0][0];
    expect(firstCallPayload.data.tags).toContain("investigation");
  });
});
