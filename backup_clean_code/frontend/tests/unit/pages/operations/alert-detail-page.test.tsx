import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const hoisted = vi.hoisted(() => ({
  setLocation: vi.fn(),
  location: "/tenant-1/alerts/alert-1",
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
  updateAlertMutate: vi.fn(),
  bindAlertsMutate: vi.fn(),
  createCaseMutate: vi.fn(),
  deleteAlertMutate: vi.fn(),
  restartWorkloadMutate: vi.fn(),
  closeWorkloadMutate: vi.fn(),
  refetchAITrace: vi.fn(),
  executeConnectorHubMutate: vi.fn(),
  outboundConnectors: [
    { id: "connector-1", name: "Threat Intel HTTP", enabled: true, direction: "outbound" },
  ],
  tenantConnectorMethods: [
    { id: "method-ip-1", ref_id: "connector-1", name: "Enrich IP", action: "lookup_ip", observable_types: ["ip"] },
  ],
  connectorExecutions: [],
  alertData: {
    id: "alert-1",
    title: "Ransomware Behavior Detected",
    description: "Suspicious encryption activity on FIN-SRV-02 from 192.168.10.45",
    source: "edr",
    sev: "Critical",
    status: "New",
    statusCode: "new",
    tags: ["ransomware", "critical-asset"],
    owner: undefined,
    caseId: "case-1",
    createdAt: "2026-02-18T10:00:00.000Z",
    updatedAt: "2026-02-18T10:01:00.000Z",
    time: "2026-02-18T10:00:00.000Z",
  },
  cases: [{ id: "case-1", caseNumber: "CASE-1", title: "Existing case" }],
  aiTraceData: {
    events: [
      {
        id: "queue-event-1",
        entity_title: "Ransomware Behavior Detected",
        status: "processing",
        source: "api",
        updated_at: "2026-03-05T10:05:00Z",
        execution_policy: "first_match",
        workloads: [
          {
            id: "workload-1",
            agent_name: "Alert Responder",
            status: "processing",
            execution_policy: "first_match",
            execution_priority: 20,
            attempt_count: 1,
            max_attempts: 3,
            last_stage: "enrichment",
            run: {
              result: {
                verdict: "suspicious",
                summary: "Awaiting analyst confirmation",
                stage_timeline: [
                  { id: "stage-1", name: "Alert triage", status: "completed", connector_count: 1, connector_success_count: 1 },
                  { id: "analysis", name: "Analysis", status: "completed" },
                ],
                connector_timeline: [
                  { connector_id: "connector-telegram", name: "Telegram Connector", status: "completed", channel: "telegram", stage_name: "Alert triage", reply: "Target user confirmed activity." },
                ],
              },
            },
          },
        ],
      },
    ],
  },
  relatedAlerts: [
    {
      id: "alert-2",
      title: "Credential Dump Attempt",
      source: "edr",
      sev: "High",
      time: "2026-02-18T09:50:00.000Z",
      caseId: "case-1",
    },
  ],
}));

vi.mock("@/components/layout", () => ({
  AppLayout: ({ children }: { children: any }) => <div data-testid="layout">{children}</div>,
}));

vi.mock("@/components/connector-execution-drawer", () => ({
  ConnectorExecutionDrawer: ({ open, executionId, title }: { open: boolean; executionId: string; title?: string }) =>
    open ? <div data-testid="connector-execution-drawer">{title || "Connector Execution"}:{executionId}</div> : null,
}));

vi.mock("@/components/ui/dropdown-menu", () => ({
  DropdownMenu: ({ children }: { children: any }) => <div>{children}</div>,
  DropdownMenuTrigger: ({ children }: { children: any }) => <div>{children}</div>,
  DropdownMenuContent: ({ children }: { children: any }) => <div>{children}</div>,
  DropdownMenuItem: ({ children, onSelect, ...props }: { children: any; onSelect?: () => void; [key: string]: any }) => (
    <button type="button" onClick={() => onSelect?.()} {...props}>{children}</button>
  ),
}));

vi.mock("wouter", () => ({
  Link: ({ href, children }: { href: string; children: any }) => <a href={href}>{children}</a>,
  useLocation: () => [hoisted.location, hoisted.setLocation],
  useParams: () => ({ id: "alert-1" }),
}));

vi.mock("sonner", () => ({
  toast: {
    success: hoisted.toastSuccess,
    error: hoisted.toastError,
  },
}));

vi.mock("@/lib/api", () => ({
  useAppState: () => ({
    currentTenantId: "tenant-1",
    currentTenantSlug: "tenant-1",
    currentUserId: "user-1",
    session: {
      identity: { user_id: "user-1", is_platform_admin: false },
      memberships: [{ tenant_id: "tenant-1", role: "tenant_admin", is_active: true }],
    },
  }),
  useAlert: () => ({ data: hoisted.alertData, isLoading: false }),
  useAIAgentEntityTrace: () => ({ data: hoisted.aiTraceData, isLoading: false, isFetching: false, dataUpdatedAt: Date.parse("2026-03-05T10:05:00Z"), refetch: hoisted.refetchAITrace }),
  useRestartAIAgentWorkload: () => ({ mutate: hoisted.restartWorkloadMutate, isPending: false }),
  useCloseAIAgentWorkload: () => ({ mutate: hoisted.closeWorkloadMutate, isPending: false }),
  useAlertsPage: () => ({ data: { items: hoisted.relatedAlerts, total: hoisted.relatedAlerts.length } }),
  useUser: (id: string) => {
    if (id === "user-owner") {
      return { data: { id: "user-owner", name: "Owner User", email: "owner@example.com" } };
    }
    return { data: { id: "user-1", name: "Current User", email: "current@example.com" } };
  },
  useCases: () => ({ data: hoisted.cases }),
  useUpdateAlert: () => ({ mutate: hoisted.updateAlertMutate, isPending: false }),
  useBindAlertsToCase: () => ({ mutate: hoisted.bindAlertsMutate, isPending: false }),
  useCreateCaseFromAlerts: () => ({ mutate: hoisted.createCaseMutate, isPending: false }),
  useDeleteAlert: () => ({ mutate: hoisted.deleteAlertMutate, isPending: false }),
  useOutboundConnectors: () => ({ data: hoisted.outboundConnectors }),
  useTenantConnectorMethods: () => ({ data: hoisted.tenantConnectorMethods }),
  useExecuteConnectorHub: () => ({ mutate: hoisted.executeConnectorHubMutate, isPending: false }),
  useConnectorHubExecutions: () => ({ data: hoisted.connectorExecutions }),
}));

import AlertDetailPage from "@/pages/alert-detail";

describe("AlertDetailPage", () => {
  beforeEach(() => {
    hoisted.setLocation.mockReset();
    hoisted.toastSuccess.mockReset();
    hoisted.toastError.mockReset();
    hoisted.updateAlertMutate.mockReset();
    hoisted.bindAlertsMutate.mockReset();
    hoisted.createCaseMutate.mockReset();
    hoisted.deleteAlertMutate.mockReset();
    hoisted.restartWorkloadMutate.mockReset();
    hoisted.closeWorkloadMutate.mockReset();
    hoisted.refetchAITrace.mockReset();
    hoisted.executeConnectorHubMutate.mockReset();
    hoisted.location = "/tenant-1/alerts/alert-1";
    hoisted.outboundConnectors = [
      { id: "connector-1", name: "Threat Intel HTTP", enabled: true, direction: "outbound" },
    ];
    hoisted.tenantConnectorMethods = [
      { id: "method-ip-1", ref_id: "connector-1", name: "Enrich IP", action: "lookup_ip", observable_types: ["ip"] },
    ];
    hoisted.connectorExecutions = [];
    hoisted.alertData = {
      id: "alert-1",
      title: "Ransomware Behavior Detected",
      description: "Suspicious encryption activity on FIN-SRV-02 from 192.168.10.45",
      source: "edr",
      sev: "Critical",
      status: "New",
      statusCode: "new",
      tags: ["ransomware", "critical-asset"],
      owner: undefined,
      caseId: "case-1",
      createdAt: "2026-02-18T10:00:00.000Z",
      updatedAt: "2026-02-18T10:01:00.000Z",
      time: "2026-02-18T10:00:00.000Z",
    };

    hoisted.updateAlertMutate.mockImplementation((_payload: any, opts?: { onSuccess?: () => void }) => {
      opts?.onSuccess?.();
    });
    hoisted.bindAlertsMutate.mockImplementation((_payload: any, opts?: { onSuccess?: () => void }) => {
      opts?.onSuccess?.();
    });
    hoisted.createCaseMutate.mockImplementation((_payload: any, opts?: { onSuccess?: (result: any) => void }) => {
      opts?.onSuccess?.({ case: { id: "case-new" } });
    });
    hoisted.deleteAlertMutate.mockImplementation((_id: string, opts?: { onSuccess?: () => void }) => {
      opts?.onSuccess?.();
    });
  });

  it("renders alert detail sections", () => {
    render(<AlertDetailPage />);

    expect(screen.getByTestId("alert-detail-page")).toBeInTheDocument();
    expect(screen.getByText("Summary")).toBeInTheDocument();
    expect(screen.getByText("Activity Timeline")).toBeInTheDocument();
    expect(screen.getByText("Metadata")).toBeInTheDocument();
    expect(screen.getByText("Related Alerts")).toBeInTheDocument();
    expect(screen.getByTestId("alert-ai-trace-card")).toBeInTheDocument();
    expect(screen.getByTestId("alert-ai-trace-event-log")).toBeInTheDocument();
    expect(screen.getByText("Alert Responder")).toBeInTheDocument();
    expect(screen.getAllByText("Alert triage").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Telegram Connector").length).toBeGreaterThan(0);

    fireEvent.click(screen.getByTestId("alert-ai-trace-refresh-live-event-log"));
    expect(hoisted.refetchAITrace).toHaveBeenCalled();
  });

  it("shows inline alert trace actions, opens linked case, and focuses targeted stage", async () => {
    hoisted.location = "/tenant-1/alerts/alert-1?ai_workload=workload-1&ai_stage=analysis";
    render(<AlertDetailPage />);

    expect(screen.getByTestId("alert-ai-trace-workload-0-0")).toHaveAttribute("data-ai-focused", "true");
    expect(screen.getByTestId("alert-ai-trace-stage-workload-1-analysis")).toHaveAttribute("data-ai-stage-focused", "true");

    fireEvent.click(screen.getByTestId("alert-ai-trace-assign-workload-1"));
    expect(hoisted.updateAlertMutate).toHaveBeenCalledWith(
      { id: "alert-1", data: { owner: "user-1", status: "Triaged" } },
      expect.any(Object),
    );

    fireEvent.click(screen.getByTestId("alert-ai-trace-open-case-workload-1"));
    expect(hoisted.setLocation).toHaveBeenCalledWith("/tenant-1/cases/case-1?tab=ai");
  });

  it("creates case from inline alert trace action when alert is not linked", async () => {
    hoisted.alertData = {
      ...hoisted.alertData,
      caseId: undefined,
    };

    render(<AlertDetailPage />);
    fireEvent.click(screen.getByTestId("alert-ai-trace-create-case-workload-1"));

    await waitFor(() => {
      expect(hoisted.createCaseMutate).toHaveBeenCalledWith(
        { alertIds: ["alert-1"], case: {} },
        expect.any(Object),
      );
    });
  });

  it("restarts and closes ai workloads from alert trace", async () => {
    hoisted.restartWorkloadMutate.mockImplementation((_payload: any, options?: { onSuccess?: () => void }) => {
      options?.onSuccess?.();
    });
    hoisted.closeWorkloadMutate.mockImplementation((_payload: any, options?: { onSuccess?: () => void }) => {
      options?.onSuccess?.();
    });
    hoisted.aiTraceData.events[0].workloads[0].status = "failed";
    hoisted.aiTraceData.events[0].workloads[0].last_error = "Needs manual restart";

    render(<AlertDetailPage />);

    fireEvent.click(screen.getByTestId("alert-ai-trace-restart-workload-1"));
    fireEvent.click(screen.getByTestId("alert-ai-trace-close-workload-1"));

    await waitFor(() => {
      expect(hoisted.restartWorkloadMutate).toHaveBeenCalledWith({ workloadId: "workload-1" }, expect.any(Object));
      expect(hoisted.closeWorkloadMutate).toHaveBeenCalledWith({ workloadId: "workload-1" }, expect.any(Object));
    });
  });

  it("derives observables and executes matched connector methods", async () => {
    hoisted.executeConnectorHubMutate.mockImplementation((_payload: any, opts?: { onSuccess?: (result: any) => void }) => {
      opts?.onSuccess?.({ id: "execution-1", status: "accepted" });
    });

    render(<AlertDetailPage />);

    expect(screen.getByText("Derived Observables")).toBeInTheDocument();
    expect(screen.getAllByText("192.168.10.45").length).toBeGreaterThan(0);

    fireEvent.click(screen.getByTestId("button-alert-observable-connectors-ip:192.168.10.45"));
    fireEvent.click(await screen.findByTestId("button-alert-observable-connector-option-ip:192.168.10.45-method-ip-1"));

    await waitFor(() => {
      expect(hoisted.executeConnectorHubMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          connector_id: "connector-1",
          method_id: "method-ip-1",
          action: "lookup_ip",
          alert_id: "alert-1",
          metadata: expect.objectContaining({
            source: "alert_observable",
            observable_type: "ip",
          }),
          input: expect.objectContaining({
            indicator: "192.168.10.45",
            observable_type: "ip",
          }),
        }),
        expect.any(Object),
      );
    });

    expect(hoisted.toastSuccess).toHaveBeenCalledWith("Connector execution queued");
    expect(screen.getByTestId("connector-execution-drawer")).toHaveTextContent("execution-1");
  });

  it("assigns alert to current user", async () => {
    render(<AlertDetailPage />);

    fireEvent.click(screen.getByTestId("button-alert-detail-assign"));

    await waitFor(() => {
      expect(hoisted.updateAlertMutate).toHaveBeenCalled();
    });
    const payload = hoisted.updateAlertMutate.mock.calls[0][0];
    expect(payload.id).toBe("alert-1");
    expect(payload.data.owner).toBe("user-1");
    expect(payload.data.status).toBe("Triaged");
  });

  it("creates case from alert and navigates", async () => {
    render(<AlertDetailPage />);

    fireEvent.click(screen.getByTestId("button-alert-detail-create-case"));

    await waitFor(() => {
      expect(hoisted.createCaseMutate).toHaveBeenCalled();
    });
    expect(hoisted.setLocation).toHaveBeenCalledWith("/tenant-1/cases/case-new");
  });

  it("deletes alert after confirmation", async () => {
    render(<AlertDetailPage />);

    fireEvent.click(screen.getByTestId("button-alert-detail-delete"));
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));

    await waitFor(() => {
      expect(hoisted.deleteAlertMutate).toHaveBeenCalledWith("alert-1", expect.any(Object));
    });
    expect(hoisted.setLocation).toHaveBeenCalledWith("/tenant-1/alerts");
  });
});
