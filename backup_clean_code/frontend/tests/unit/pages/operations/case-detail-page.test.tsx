import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const hoisted = vi.hoisted(() => ({
  updateCaseMutate: vi.fn(),
  copyCaseMutate: vi.fn(),
  escalateCaseMutate: vi.fn(),
  shareCaseMutate: vi.fn(),
  createTaskMutate: vi.fn(),
  createCasePageMutate: vi.fn(),
  createTimelineEventMutate: vi.fn(),
  runWorkflowMutate: vi.fn(),
  updateObservableMutateAsync: vi.fn(),
  executeConnectorHubMutate: vi.fn(),
  restartWorkloadMutate: vi.fn(),
  closeWorkloadMutate: vi.fn(),
  refetchAITrace: vi.fn(),
  caseData: {} as any,
  outboundConnectorsData: [] as any[],
  connectorMethodsData: [] as any[],
  connectorHubRunsData: [] as any[],
  aiTraceData: { events: [] } as any,
  workflowCatalogData: [
    {
      id: "wf-1",
      name: "Containment Playbook",
      description: "Contain and isolate affected host using indicators",
      enabled: true,
      updatedAt: "2026-02-23T10:00:00.000Z",
    },
    {
      id: "wf-2",
      name: "IOC Enrichment",
      description: "Enrich IOC and map observables to context",
      enabled: true,
      updatedAt: "2026-02-23T10:05:00.000Z",
    },
  ] as any[],
  playbookRunsData: {
    runs: [
      {
        id: "run-1",
        workflowId: "wf-1",
        workflowName: "Containment Playbook",
        status: "success",
        startedAt: "2026-02-23T10:20:00.000Z",
        durationMs: 1400,
        result: { action: "contained" },
      },
    ],
  } as any,
  observablesData: [] as any[],
  attachmentsData: [] as any[],
  timelineData: [] as any[],
  location: "/cases/case-1",
  relatedCasesData: {
    activeRecent: [
      {
        id: "case-2",
        caseNumber: "CASE-002",
        title: "Active related chain",
        matchCount: 2,
        matchedObservables: [{ type: "ip", value: "1.1.1.1" }],
      },
    ],
    allTime: [
      {
        id: "case-2",
        caseNumber: "CASE-002",
        title: "Active related chain",
        matchCount: 2,
        matchedObservables: [{ type: "ip", value: "1.1.1.1" }],
      },
      {
        id: "case-3",
        caseNumber: "CASE-003",
        title: "Historical related chain",
        matchCount: 1,
        matchedObservables: [{ type: "domain", value: "malicious.example" }],
      },
    ],
  } as any,
  setLocation: vi.fn(),
}));

vi.mock("@/components/layout", () => ({
  AppLayout: ({ children }: { children: any }) => <div data-testid="layout">{children}</div>,
}));
vi.mock("@/components/connector-execution-drawer", () => ({
  ConnectorExecutionDrawer: () => null,
}));

vi.mock("@/components/case-communications-tab", () => ({
  CaseCommunicationsTab: () => <div data-testid="communications-tab" />,
}));

vi.mock("wouter", () => ({
  useParams: () => ({ id: "case-1" }),
  useLocation: () => [hoisted.location, hoisted.setLocation],
  Link: ({ children, href }: { children: any; href: string }) => <a href={href}>{children}</a>,
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

vi.mock("@/lib/api", () => ({
  useAppState: () => ({
    currentUserId: "user-1",
    currentTenantId: "tenant-1",
    currentTenantSlug: "tenant-1",
    session: {
      identity: { user_id: "user-1", is_platform_admin: false },
      memberships: [{ tenant_id: "tenant-1", role: "tenant_admin", is_active: true }],
    },
  }),
  useCase: () => ({
    data: hoisted.caseData,
    isLoading: false,
  }),
  useUser: (id?: string) => ({
    data: id ? { id, name: id === "user-2" ? "SOC Analyst" : id === "user-3" ? "Incident Commander" : "Current User" } : undefined,
  }),
  useTenants: () => ({
    data: [
      { id: "tenant-1", slug: "tenant-1", name: "Tenant One", active: true },
      { id: "tenant-2", slug: "tenant-2", name: "Tenant Two", active: true },
    ],
  }),
  useUsers: () => ({ data: [{ id: "user-2", name: "SOC Analyst" }, { id: "user-3", name: "Incident Commander" }] }),
  useOutboundConnectors: () => ({ data: hoisted.outboundConnectorsData }),
  useConnectorMethods: () => ({ data: hoisted.connectorMethodsData }),
  useTenantConnectorMethods: () => ({ data: hoisted.connectorMethodsData }),
  useExecuteConnectorHub: () => ({ mutate: hoisted.executeConnectorHubMutate, isPending: false }),
  useConnectorHubExecutions: () => ({ data: hoisted.connectorHubRunsData }),
  useCaseStatuses: () => ({ data: [{ code: "open", label: "Open", isClosed: false }, { code: "closed", label: "Closed", isClosed: true }] }),
  useCaseCategories: () => ({ data: [{ id: "cat-1", name: "True Positive" }] }),
  useCreateCaseCategory: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateCase: () => ({ mutate: hoisted.updateCaseMutate, isPending: false }),
  useCopyCase: () => ({ mutate: hoisted.copyCaseMutate, isPending: false }),
  useEscalateCase: () => ({ mutate: hoisted.escalateCaseMutate, isPending: false }),
  useCaseShares: () => ({ data: [] }),
  useShareCaseAcrossTenants: () => ({ mutate: hoisted.shareCaseMutate, isPending: false }),
  useCaseRelatedCases: () => ({ data: hoisted.relatedCasesData }),
  useCreateForumThread: () => ({ mutate: vi.fn(), isPending: false }),
  useForumThread: () => ({ data: null }),
  useCaseTimeline: () => ({ data: hoisted.timelineData }),
  useCaseTasks: () => ({ data: [] }),
  useCreateCaseTask: () => ({ mutate: hoisted.createTaskMutate, isPending: false }),
  useUpdateCaseTask: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteCaseTask: () => ({ mutate: vi.fn(), isPending: false }),
  useCaseObservables: () => ({ data: hoisted.observablesData }),
  useCreateCaseObservable: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateCaseObservable: () => ({ mutateAsync: hoisted.updateObservableMutateAsync, isPending: false }),
  useDeleteCaseObservable: () => ({ mutate: vi.fn(), isPending: false }),
  useCaseAttachments: () => ({ data: hoisted.attachmentsData }),
  useUploadCaseAttachment: () => ({ mutate: vi.fn(), isPending: false }),
  useCaseAttachmentDownloadURL: () => ({ mutate: vi.fn(), isPending: false }),
  useCaseComments: () => ({ data: [] }),
  useCreateCaseComment: () => ({ mutate: vi.fn(), isPending: false }),
  useCreateCaseTimelineEvent: () => ({ mutate: hoisted.createTimelineEventMutate, isPending: false }),
  useCasePages: () => ({ data: [] }),
  useCreateCasePage: () => ({ mutate: hoisted.createCasePageMutate, isPending: false }),
  useCasePlaybookRuns: () => ({ data: hoisted.playbookRunsData }),
  useWorkflowCatalog: () => ({ data: hoisted.workflowCatalogData }),
  useWorkflowRun: () => ({ mutate: hoisted.runWorkflowMutate, isPending: false }),
  useCaseAIAnalyses: () => ({ data: [] }),
  useAnalyzeCaseAI: () => ({ mutate: vi.fn(), isPending: false }),
  useAIAgentEntityTrace: () => ({ data: hoisted.aiTraceData, isLoading: false, isFetching: false, dataUpdatedAt: Date.parse("2026-03-05T10:05:00Z"), refetch: hoisted.refetchAITrace }),
  useRestartAIAgentWorkload: () => ({ mutate: hoisted.restartWorkloadMutate, isPending: false }),
  useCloseAIAgentWorkload: () => ({ mutate: hoisted.closeWorkloadMutate, isPending: false }),
}));

import CaseDetailPage from "@/pages/case-detail";

describe("CaseDetailPage edit flow", () => {
  beforeEach(() => {
    hoisted.updateCaseMutate.mockReset();
    hoisted.copyCaseMutate.mockReset();
    hoisted.escalateCaseMutate.mockReset();
    hoisted.shareCaseMutate.mockReset();
    hoisted.createTaskMutate.mockReset();
    hoisted.createCasePageMutate.mockReset();
    hoisted.createTimelineEventMutate.mockReset();
    hoisted.runWorkflowMutate.mockReset();
    hoisted.updateObservableMutateAsync.mockReset();
    hoisted.executeConnectorHubMutate.mockReset();
    hoisted.location = "/cases/case-1";
    hoisted.restartWorkloadMutate.mockReset();
    hoisted.closeWorkloadMutate.mockReset();
    hoisted.refetchAITrace.mockReset();
    hoisted.setLocation.mockReset();
    hoisted.caseData = {
      id: "case-1",
      caseNumber: "CASE-001",
      title: "Credential leak",
      description: "Original description",
      source: "SIEM",
      incidentType: "Credential Access",
      category: "True Positive",
      relatedProduct: "DIB Core",
      priority: "high",
      impact: "medium",
      confidence: 75,
      statusCode: "open",
      status: "Open",
      sev: "High",
      tlp: "TLP:AMBER",
      pap: "AMBER",
      owner: "user-2",
      assignee: "user-2",
      detectedAt: "2026-02-23T10:00:00.000Z",
      occurredAt: "2026-02-23T09:30:00.000Z",
      closedAt: "",
      resolutionSummary: "",
      time: "2026-02-23T10:00:00.000Z",
      tags: ["AMBER"],
      forumId: "",
      tactics: [],
      techniques: [],
      verdict: "Unknown",
      recommendations: [],
      stage: "Triage",
      customFields: {},
    };
    hoisted.workflowCatalogData = [
      {
        id: "wf-1",
        name: "Containment Playbook",
        description: "Contain and isolate affected host using indicators",
        enabled: true,
        updatedAt: "2026-02-23T10:00:00.000Z",
      },
      {
        id: "wf-2",
        name: "IOC Enrichment",
        description: "Enrich IOC and map observables to context",
        enabled: true,
        updatedAt: "2026-02-23T10:05:00.000Z",
      },
    ];
    hoisted.playbookRunsData = {
      runs: [
        {
          id: "run-1",
          workflowId: "wf-1",
          workflowName: "Containment Playbook",
          status: "success",
          startedAt: "2026-02-23T10:20:00.000Z",
          durationMs: 1400,
          result: { action: "contained" },
        },
      ],
    };
    hoisted.observablesData = [];
    hoisted.attachmentsData = [];
    hoisted.timelineData = [];
    hoisted.outboundConnectorsData = [{ id: "connector-1", name: "Telegram Connector" }];
    hoisted.connectorMethodsData = [{ id: "method-1", name: "Send Message", action: "send_message" }];
    hoisted.connectorHubRunsData = [];
    hoisted.aiTraceData = {
      events: [
        {
          id: "queue-event-1",
          entity_title: "Credential leak",
          status: "processing",
          source: "api",
          updated_at: "2026-03-05T10:05:00Z",
          execution_policy: "fallback_chain",
          workloads: [
            {
              id: "workload-1",
              agent_name: "Queue Analyst",
              status: "failed",
              execution_policy: "fallback_chain",
              execution_index: 0,
              attempt_count: 1,
              max_attempts: 3,
              last_stage: "analysis",
              last_error: "Connector timeout",
              run: {
                result: {
                  verdict: "suspicious",
                  summary: "IOC requires manual review",
                  action_blockers: ["needs human review"],
                  stage_timeline: [
                    {
                      id: "stage-1",
                      name: "Threat Intel",
                      status: "completed",
                      connector_count: 1,
                      connector_success_count: 1,
                      case_tags: ["ai-reviewed"],
                    },
                    {
                      id: "analysis",
                      name: "Analysis",
                      status: "failed",
                      error: "Connector timeout",
                    },
                  ],
                  connector_timeline: [
                    {
                      connector_id: "connector-1",
                      name: "Threat Intel Connector",
                      status: "completed",
                      channel: "http",
                      stage_name: "Threat Intel",
                      reply: "Observed IP reputation is high risk.",
                    },
                  ],
                },
              },
            },
          ],
        },
      ],
    };
    hoisted.relatedCasesData = {
      activeRecent: [
        {
          id: "case-2",
          caseNumber: "CASE-002",
          title: "Active related chain",
          matchCount: 2,
          matchedObservables: [{ type: "ip", value: "1.1.1.1" }],
        },
      ],
      allTime: [
        {
          id: "case-2",
          caseNumber: "CASE-002",
          title: "Active related chain",
          matchCount: 2,
          matchedObservables: [{ type: "ip", value: "1.1.1.1" }],
        },
        {
          id: "case-3",
          caseNumber: "CASE-003",
          title: "Historical related chain",
          matchCount: 1,
          matchedObservables: [{ type: "domain", value: "malicious.example" }],
        },
      ],
    };
    hoisted.updateCaseMutate.mockImplementation((_payload: any, options?: { onSuccess?: () => void }) => {
      options?.onSuccess?.();
    });
    hoisted.copyCaseMutate.mockImplementation((_payload: any, options?: { onSuccess?: (payload?: any) => void }) => {
      options?.onSuccess?.({ id: "case-copy-1" });
    });
    hoisted.escalateCaseMutate.mockImplementation((_payload: any, options?: { onSuccess?: (payload?: any) => void }) => {
      options?.onSuccess?.({
        target_tenant_slug: "tenant-2",
        target_case: { id: "case-2" },
      });
    });
    hoisted.shareCaseMutate.mockImplementation((_payload: any, options?: { onSuccess?: () => void }) => {
      options?.onSuccess?.();
    });
    hoisted.createCasePageMutate.mockImplementation((_payload: any, options?: { onSuccess?: () => void }) => {
      options?.onSuccess?.();
    });
    hoisted.createTimelineEventMutate.mockImplementation((_payload: any, options?: { onSuccess?: () => void }) => {
      options?.onSuccess?.();
    });
    hoisted.runWorkflowMutate.mockImplementation((_payload: any, options?: { onSuccess?: () => void }) => {
      options?.onSuccess?.();
    });
    hoisted.executeConnectorHubMutate.mockImplementation((_payload: any, options?: { onSuccess?: (payload?: any) => void }) => {
      options?.onSuccess?.({ status: "completed" });
    });
    hoisted.updateObservableMutateAsync.mockResolvedValue({});
    hoisted.createTaskMutate.mockImplementation(() => undefined);
  });

  it("opens edit dialog and saves updated case fields", async () => {
    render(<CaseDetailPage />);

    fireEvent.click(screen.getByTestId("button-edit-case"));
    expect(screen.getByTestId("dialog-edit-case")).toBeInTheDocument();

    fireEvent.change(screen.getByTestId("input-case-edit-title"), {
      target: { value: "Credential leak updated" },
    });
    fireEvent.change(screen.getByTestId("input-case-edit-description"), {
      target: { value: "Updated description" },
    });

    fireEvent.click(screen.getByTestId("button-case-edit-save"));

    await waitFor(() => {
      expect(hoisted.updateCaseMutate).toHaveBeenCalled();
    });

    const [payload] = hoisted.updateCaseMutate.mock.calls[0];
    expect(payload.id).toBe("case-1");
    expect(payload.data.title).toBe("Credential leak updated");
    expect(payload.data.description).toBe("Updated description");
  });

  it("duplicates case from header action", async () => {
    render(<CaseDetailPage />);

    fireEvent.click(screen.getByTestId("button-copy-case"));
    fireEvent.click(await screen.findByText("Duplicate case"));

    await waitFor(() => {
      expect(hoisted.copyCaseMutate).toHaveBeenCalled();
    });

    const [payload] = hoisted.copyCaseMutate.mock.calls[0];
    expect(payload.id).toBe("case-1");
    expect(payload.data.title).toBe("Copy of Credential leak");
    expect(payload.data.includeObservables).toBe(true);
    expect(hoisted.setLocation).toHaveBeenCalledWith("/tenant-1/cases/case-copy-1");
  });

  it("edits case title inline and saves by save button", async () => {
    render(<CaseDetailPage />);

    fireEvent.click(screen.getByTestId("button-inline-edit-title"));
    fireEvent.change(screen.getByTestId("input-inline-title"), {
      target: { value: "Inline updated title" },
    });
    fireEvent.click(screen.getByTestId("button-inline-save-title"));

    await waitFor(() => {
      expect(hoisted.updateCaseMutate).toHaveBeenCalled();
    });

    const [payload] = hoisted.updateCaseMutate.mock.calls[0];
    expect(payload).toMatchObject({
      id: "case-1",
      data: {
        title: "Inline updated title",
      },
    });
  });

  it("edits category inline and autosaves on click outside", async () => {
    render(<CaseDetailPage />);

    fireEvent.click(screen.getByTestId("button-inline-edit-category"));
    fireEvent.change(screen.getByTestId("input-inline-category"), {
      target: { value: "False Positive" },
    });
    fireEvent.mouseDown(document.body);

    await waitFor(() => {
      expect(hoisted.updateCaseMutate).toHaveBeenCalled();
    });

    const [payload] = hoisted.updateCaseMutate.mock.calls[0];
    expect(payload).toMatchObject({
      id: "case-1",
      data: {
        category: "False Positive",
      },
    });
  });

  it("autosaves inline description on click outside editor", async () => {
    render(<CaseDetailPage />);

    fireEvent.click(screen.getByTestId("button-inline-edit-description"));
    fireEvent.change(screen.getByTestId("input-inline-description"), {
      target: { value: "Description autosaved" },
    });

    fireEvent.mouseDown(document.body);

    await waitFor(() => {
      expect(hoisted.updateCaseMutate).toHaveBeenCalled();
    });

    const [payload] = hoisted.updateCaseMutate.mock.calls[0];
    expect(payload).toMatchObject({
      id: "case-1",
      data: {
        description: "Description autosaved",
      },
    });
  });

  it("edits owner inline via dropdown and autosaves", async () => {
    const user = userEvent.setup();
    render(<CaseDetailPage />);

    await user.click(screen.getByTestId("button-inline-edit-owner"));
    await user.click(screen.getByTestId("select-inline-owner"));
    await user.click(await screen.findByText("Incident Commander"));

    fireEvent.mouseDown(document.body);

    await waitFor(() => {
      expect(hoisted.updateCaseMutate).toHaveBeenCalled();
    });

    const [payload] = hoisted.updateCaseMutate.mock.calls[0];
    expect(payload).toMatchObject({
      id: "case-1",
      data: {
        owner: "user-3",
      },
    });
  });

  it("clears assignee inline via unassigned option and autosaves", async () => {
    const user = userEvent.setup();
    render(<CaseDetailPage />);

    await user.click(screen.getByTestId("button-inline-edit-assignee"));
    await user.click(screen.getByTestId("select-inline-assignee"));
    await user.click(await screen.findByText(/Unassigned/i));

    fireEvent.mouseDown(document.body);

    await waitFor(() => {
      expect(hoisted.updateCaseMutate).toHaveBeenCalled();
    });

    const [payload] = hoisted.updateCaseMutate.mock.calls[0];
    expect(payload).toMatchObject({
      id: "case-1",
      data: {
        assignee: "",
      },
    });
  });

  it("clears owner inline via unassigned option and autosaves", async () => {
    const user = userEvent.setup();
    render(<CaseDetailPage />);

    await user.click(screen.getByTestId("button-inline-edit-owner"));
    await user.click(screen.getByTestId("select-inline-owner"));
    await user.click(await screen.findByText(/Unassigned/i));

    fireEvent.mouseDown(document.body);

    await waitFor(() => {
      expect(hoisted.updateCaseMutate).toHaveBeenCalled();
    });

    const [payload] = hoisted.updateCaseMutate.mock.calls[0];
    expect(payload).toMatchObject({
      id: "case-1",
      data: {
        owner: "",
      },
    });
  });

  it("adds and saves custom fields", async () => {
    const user = userEvent.setup();
    render(<CaseDetailPage />);

    await user.click(screen.getByTestId("button-add-custom-field"));
    fireEvent.change(screen.getByTestId("input-custom-field-key-0"), {
      target: { value: "ticket_id" },
    });
    fireEvent.change(screen.getByTestId("input-custom-field-value-0"), {
      target: { value: "IR-42" },
    });
    await user.click(screen.getByTestId("button-save-custom-fields"));

    await waitFor(() => {
      expect(hoisted.updateCaseMutate).toHaveBeenCalled();
    });
    const [payload] = hoisted.updateCaseMutate.mock.calls[0];
    expect(payload).toMatchObject({
      id: "case-1",
      data: {
        customFields: {
          ticket_id: "IR-42",
        },
      },
    });
  });

  it("renders related cases chains on overview tab", () => {
    render(<CaseDetailPage />);

    expect(screen.getByTestId("card-related-cases")).toBeInTheDocument();
    expect(screen.getByTestId("related-active-recent")).toHaveTextContent("CASE-002");
    expect(screen.getByTestId("related-all-time")).toHaveTextContent("CASE-003");
  });

  it("runs generic connector action from matched related observables", async () => {
    const user = userEvent.setup();
    hoisted.outboundConnectorsData = [{ id: "connector-1", name: "Threat Intel Connector", enabled: true, direction: "outbound" }];
    hoisted.connectorMethodsData = [
      { id: "method-ip", ref_id: "connector-1", name: "Lookup IP", action: "scan_ip", observable_types: ["ip"] },
    ];

    render(<CaseDetailPage />);

    const trigger = screen.getByTestId("button-related-observable-connectors-active-case-2-0");
    expect(trigger).not.toBeDisabled();

    await user.click(trigger);
    await user.click(await screen.findByTestId("button-related-observable-connector-option-active-case-2-0-method-ip"));

    await waitFor(() => {
      expect(hoisted.executeConnectorHubMutate).toHaveBeenCalled();
    });

    const [payload] = hoisted.executeConnectorHubMutate.mock.calls.at(-1) || [];
    expect(payload).toMatchObject({
      connector_id: "connector-1",
      method_id: "method-ip",
      case_id: "case-1",
      action: "scan_ip",
      input: {
        indicator: "1.1.1.1",
        matched_case_id: "case-2",
        observable_type: "ip",
        observable: {
          type: "ip",
          value: "1.1.1.1",
        },
      },
      metadata: {
        source: "case_related_observable",
        matched_case_id: "case-2",
        observable_type: "ip",
      },
    });
  });

  it("runs connector hub action from connectors tab", async () => {
    const user = userEvent.setup();
    render(<CaseDetailPage />);

    await user.click(screen.getByTestId("tab-connectors"));
    expect(screen.getByTestId("tab-content-connectors")).toBeInTheDocument();

    const runButton = screen.getByTestId("button-case-connector-hub-run");
    await waitFor(() => {
      expect(runButton).not.toHaveAttribute("disabled");
    });

    await user.click(runButton);

    await waitFor(() => {
      expect(hoisted.executeConnectorHubMutate).toHaveBeenCalled();
    });

    const [payload] = hoisted.executeConnectorHubMutate.mock.calls[0];
    expect(payload).toMatchObject({
      connector_id: "connector-1",
      method_id: "method-1",
      case_id: "case-1",
      action: "send_message",
    });
    expect(payload.input).toEqual({ query: "" });
    expect(payload.metadata).toEqual({ source: "case_detail" });
  });

  it("records closure approval event from overview card", async () => {
    const user = userEvent.setup();
    hoisted.caseData = {
      ...hoisted.caseData,
      customFields: {
        closure_required_approvals: 2,
        closure_approver_ids: "user-2,user-3",
      },
    };
    hoisted.timelineData = [
      {
        id: "evt-approval-1",
        eventType: "closure_approval",
        title: "Closure approved",
        description: "Approved by SOC analyst",
        userId: "user-2",
        metadata: {},
        createdAt: "2026-02-23T10:40:00.000Z",
      },
    ];

    render(<CaseDetailPage />);

    await user.click(screen.getByTestId("button-approve-closure"));
    expect(hoisted.createTimelineEventMutate).toHaveBeenCalledTimes(1);
    const [payload] = hoisted.createTimelineEventMutate.mock.calls[0];
    expect(payload).toMatchObject({
      caseId: "case-1",
      eventType: "closure_approval",
    });
  });

  it("shows generic connector actions inside the visual observables applet", async () => {
    const user = userEvent.setup();
    hoisted.observablesData = [
      { id: "obs-net", type: "IP", value: "10.0.0.1", verdict: "Suspicious", tags: ["network"] },
      { id: "obs-auth", type: "Email", value: "user@example.com", verdict: "Unknown", tags: ["authorization"] },
    ];
    hoisted.outboundConnectorsData = [{ id: "connector-1", name: "Threat Intel Connector", enabled: true, direction: "outbound" }];
    hoisted.connectorMethodsData = [
      { id: "method-ip", ref_id: "connector-1", name: "Lookup IP", action: "scan_ip", observable_types: ["ip"] },
    ];

    render(<CaseDetailPage />);

    await user.click(screen.getByTestId("tab-visuals"));

    expect(screen.getByTestId("button-visual-observable-connectors-obs-net")).not.toBeDisabled();
    expect(screen.getByTestId("button-visual-observable-connectors-obs-auth")).toBeDisabled();
  });

  it("renders visualizations tab with source and files filters", async () => {
    const user = userEvent.setup();
    hoisted.observablesData = [
      { id: "obs-net", type: "IP", value: "10.0.0.1", verdict: "Suspicious", tags: ["network"] },
      { id: "obs-auth", type: "Email", value: "user@example.com", verdict: "Unknown", tags: ["authorization"] },
    ];
    hoisted.attachmentsData = [
      { id: "att-image", fileName: "packet.png", contentType: "image/png", fileSizeBytes: 12000, createdAt: "2026-02-23T10:10:00.000Z" },
      { id: "att-file", fileName: "auth.log", contentType: "text/plain", fileSizeBytes: 22000, createdAt: "2026-02-23T10:12:00.000Z" },
    ];

    render(<CaseDetailPage />);

    await user.click(screen.getByTestId("tab-visuals"));
    expect(screen.getByTestId("tab-content-visuals")).toBeInTheDocument();
    expect(screen.getByTestId("visual-observable-row-obs-net")).toBeInTheDocument();
    expect(screen.getByTestId("visual-observable-row-obs-auth")).toBeInTheDocument();

    await user.click(screen.getByTestId("select-visual-source-filter"));
    await user.click(await screen.findByTestId("option-visual-source-authorization"));

    await waitFor(() => {
      expect(screen.queryByTestId("visual-observable-row-obs-net")).not.toBeInTheDocument();
      expect(screen.getByTestId("visual-observable-row-obs-auth")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("select-visual-file-filter"));
    await user.click(await screen.findByTestId("option-visual-file-images"));

    await waitFor(() => {
      expect(screen.getByTestId("visual-attachment-row-att-image")).toBeInTheDocument();
      expect(screen.queryByTestId("visual-attachment-row-att-file")).not.toBeInTheDocument();
    });

    await user.click(screen.getByTestId("button-visual-reset-linked-filters"));
    await waitFor(() => {
      expect(screen.getByTestId("visual-observable-row-obs-net")).toBeInTheDocument();
      expect(screen.getByTestId("visual-observable-row-obs-auth")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("visual-source-applet-row-network"));
    await waitFor(() => {
      expect(screen.getByTestId("visual-observable-row-obs-net")).toBeInTheDocument();
      expect(screen.queryByTestId("visual-observable-row-obs-auth")).not.toBeInTheDocument();
    });

    await user.click(screen.getByTestId("button-visual-reset-linked-filters"));
    await user.click(screen.getByTestId("visual-type-applet-row-Email"));
    await waitFor(() => {
      expect(screen.getByTestId("visual-observable-row-obs-auth")).toBeInTheDocument();
      expect(screen.queryByTestId("visual-observable-row-obs-net")).not.toBeInTheDocument();
    });
  });

  it("shares case as one entity with another tenant", async () => {
    const user = userEvent.setup();
    render(<CaseDetailPage />);

    await user.click(screen.getByTestId("button-open-share-case"));
    await user.click(screen.getByTestId("select-case-share-tenant"));
    const tenantTwoOptions = await screen.findAllByText("Tenant Two");
    await user.click(tenantTwoOptions[tenantTwoOptions.length - 1]);
    await user.click(screen.getByTestId("button-case-share-confirm"));

    expect(hoisted.shareCaseMutate).toHaveBeenCalledTimes(1);
    const [payload] = hoisted.shareCaseMutate.mock.calls[0];
    expect(payload).toMatchObject({
      caseId: "case-1",
      data: {
        targetTenantId: "tenant-2",
      },
    });
  });

  it("escalates case to another tenant from dialog", async () => {
    const user = userEvent.setup();
    render(<CaseDetailPage />);

    await user.click(screen.getByTestId("button-open-escalate-case"));
    await user.click(screen.getByTestId("select-case-escalate-tenant"));
    const tenantTwoOptions = await screen.findAllByText("Tenant Two");
    await user.click(tenantTwoOptions[tenantTwoOptions.length - 1]);
    await user.type(screen.getByTestId("input-case-escalate-summary"), "Need handoff to DataSec");
    await user.click(screen.getByTestId("button-case-escalate-confirm"));

    expect(hoisted.escalateCaseMutate).toHaveBeenCalledTimes(1);
    const [payload] = hoisted.escalateCaseMutate.mock.calls[0];
    expect(payload).toMatchObject({
      caseId: "case-1",
      data: {
        targetTenantId: "tenant-2",
        summary: "Need handoff to DataSec",
        includeObservables: true,
      },
    });
    expect(hoisted.setLocation).toHaveBeenCalled();
  });

  it("does not render case pages tab anymore", async () => {
    render(<CaseDetailPage />);
    expect(screen.queryByTestId("tab-pages")).not.toBeInTheDocument();
  });

  it("adds observable tag by Enter", async () => {
    hoisted.observablesData = [
      { id: "obs-1", caseId: "case-1", type: "ip", value: "1.1.1.1", verdict: "Unknown", tags: [] },
      { id: "obs-2", caseId: "case-1", type: "ip", value: "8.8.8.8", verdict: "Unknown", tags: [] },
    ];

    const user = userEvent.setup();
    render(<CaseDetailPage />);

    await user.click(screen.getByTestId("tab-observables"));

    const tagInput = await screen.findByTestId("input-observable-tags-obs-1");
    fireEvent.change(tagInput, { target: { value: "blocklist" } });
    fireEvent.keyDown(tagInput, { key: "Enter", code: "Enter" });

    await waitFor(() => {
      expect(hoisted.updateObservableMutateAsync).toHaveBeenCalled();
    });
    expect(hoisted.updateObservableMutateAsync.mock.calls[0][0]).toMatchObject({
      id: "obs-1",
      data: {
        caseId: "case-1",
        tags: ["blocklist"],
      },
    });
  });

  it("shows suggested playbooks and autofills case context", async () => {
    hoisted.observablesData = [
      { id: "obs-1", caseId: "case-1", type: "Domain", value: "malicious.example", verdict: "Suspicious", tags: [] },
      { id: "obs-2", caseId: "case-1", type: "IP", value: "1.1.1.1", verdict: "Malicious", tags: [] },
    ];
    const user = userEvent.setup();
    render(<CaseDetailPage />);

    await user.click(screen.getByTestId("tab-timeline"));
    expect(screen.getByTestId("badge-playbook-suggested-wf-1")).toBeInTheDocument();

    await user.click(screen.getByTestId("button-select-playbook-wf-2"));
    const input = screen.getByTestId("textarea-playbook-input") as HTMLTextAreaElement;
    expect(input.value).toContain("\"case_id\": \"case-1\"");
    expect(input.value).toContain("\"host\": \"malicious.example\"");
    expect(input.value).toContain("\"indicators\"");
  });

  it("runs selected playbook with case autofill payload", async () => {
    hoisted.observablesData = [
      { id: "obs-1", caseId: "case-1", type: "Domain", value: "malicious.example", verdict: "Suspicious", tags: [] },
      { id: "obs-2", caseId: "case-1", type: "IP", value: "1.1.1.1", verdict: "Malicious", tags: [] },
    ];
    const user = userEvent.setup();
    render(<CaseDetailPage />);

    await user.click(screen.getByTestId("tab-timeline"));
    await user.click(screen.getByTestId("button-select-playbook-wf-2"));
    await user.click(screen.getByTestId("button-run-playbook-case"));

    await waitFor(() => {
      expect(hoisted.runWorkflowMutate).toHaveBeenCalledTimes(1);
    });
    const [payload] = hoisted.runWorkflowMutate.mock.calls[0];
    expect(payload.workflowID).toBe("wf-2");
    expect(payload.data).toMatchObject({
      case_id: "case-1",
      host: "malicious.example",
    });
    expect(Array.isArray(payload.data.indicators)).toBe(true);
    expect(payload.data.indicators).toContain("1.1.1.1");
  });

  it("adds analyst note into timeline", async () => {
    const user = userEvent.setup();
    render(<CaseDetailPage />);

    await user.click(screen.getByTestId("tab-timeline"));
    await user.type(screen.getByTestId("input-timeline-note-title"), "Containment update");
    await user.type(screen.getByTestId("input-timeline-note-body"), "Host isolated and credentials rotated");
    await user.click(screen.getByTestId("button-add-timeline-note"));

    await waitFor(() => {
      expect(hoisted.createTimelineEventMutate).toHaveBeenCalledTimes(1);
    });
    expect(hoisted.createTimelineEventMutate.mock.calls[0][0]).toMatchObject({
      caseId: "case-1",
      eventType: "note",
      title: "Containment update",
      body: "Host isolated and credentials rotated",
    });
  });

  it("exports case timeline as JSON", async () => {
    const user = userEvent.setup();
    const originalCreateObjectURL = URL.createObjectURL;
    const originalRevokeObjectURL = URL.revokeObjectURL;
    const createObjectURLMock = vi.fn(() => "blob:case-export");
    const revokeObjectURLMock = vi.fn();
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
    Object.defineProperty(URL, "createObjectURL", { configurable: true, writable: true, value: createObjectURLMock });
    Object.defineProperty(URL, "revokeObjectURL", { configurable: true, writable: true, value: revokeObjectURLMock });

    render(<CaseDetailPage />);
    await user.click(screen.getByTestId("tab-timeline"));
    await user.click(screen.getByTestId("button-export-case-json"));

    expect(createObjectURLMock).toHaveBeenCalledTimes(1);
    expect(revokeObjectURLMock).toHaveBeenCalledTimes(1);
    expect(clickSpy).toHaveBeenCalled();

    clickSpy.mockRestore();
    Object.defineProperty(URL, "createObjectURL", { configurable: true, writable: true, value: originalCreateObjectURL });
    Object.defineProperty(URL, "revokeObjectURL", { configurable: true, writable: true, value: originalRevokeObjectURL });
  });

  it("renders markdown description with image, file link and strips unsafe tags", () => {
    hoisted.caseData = {
      ...hoisted.caseData,
      description: "# Header\n**bold** text\n![Packet](https://example.com/a.png)\n[Report](https://example.com/report.pdf)\n<span style=\"color:#ff0000\">red</span>\n<script>alert(1)</script>",
    };

    render(<CaseDetailPage />);

    const rendered = screen.getByTestId("text-description-rendered");
    expect(rendered.innerHTML).toContain("<h1>Header</h1>");
    expect(rendered.innerHTML).toContain("<img");
    expect(rendered.innerHTML).toContain("report.pdf");
    expect(rendered.innerHTML).toContain("color:#ff0000");
    expect(rendered.innerHTML).not.toContain("<script");
  });

  it("creates task with due date from tasks tab", async () => {
    const user = userEvent.setup();
    render(<CaseDetailPage />);

    await user.click(screen.getByTestId("tab-tasks"));
    await user.type(screen.getByTestId("input-task-title"), "Follow up with endpoint team");
    fireEvent.change(screen.getByTestId("input-task-due-date"), { target: { value: "2026-03-01T12:30" } });
    await user.click(screen.getByTestId("button-create-task"));

    expect(hoisted.createTaskMutate).toHaveBeenCalledTimes(1);
    const [payload] = hoisted.createTaskMutate.mock.calls[0];
    expect(payload.title).toBe("Follow up with endpoint team");
    expect(typeof payload.dueDate).toBe("string");
    expect(String(payload.dueDate)).toMatch(/^20\d\d-\d\d-\d\dT/);
  });

  it("switches task chain mode and persists in custom fields", async () => {
    const user = userEvent.setup();
    render(<CaseDetailPage />);

    await user.click(screen.getByTestId("tab-tasks"));
    await user.click(screen.getByTestId("button-task-chain-mode-automated"));

    expect(hoisted.updateCaseMutate).toHaveBeenCalled();
    const [payload] = hoisted.updateCaseMutate.mock.calls[0];
    expect(payload).toMatchObject({
      id: "case-1",
      data: {
        customFields: {
          task_chain_mode: "automated",
        },
      },
    });
  });

  it("focuses linked ai workload stage from query params", async () => {
    const user = userEvent.setup();
    hoisted.location = "/cases/case-1?tab=ai&ai_workload=workload-1&ai_stage=analysis";

    render(<CaseDetailPage />);
    await user.click(screen.getByTestId("tab-ai"));

    await waitFor(() => {
      expect(screen.getByTestId("case-ai-workload-workload-1")).toHaveAttribute("data-ai-focused", "true");
      expect(screen.getByTestId("case-ai-stage-workload-1-analysis")).toHaveAttribute("data-ai-stage-focused", "true");
    });
  });

  it("renders ai workload trace inside dedicated tab", async () => {
    const user = userEvent.setup();
    render(<CaseDetailPage />);

    await user.click(screen.getByTestId("tab-ai"));

    expect(screen.getByTestId("case-ai-workloads")).toBeInTheDocument();
    expect(screen.getAllByText("Queue Analyst").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Connector timeout").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Threat Intel").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Threat Intel Connector").length).toBeGreaterThan(0);

    await user.click(screen.getByTestId("case-ai-refresh"));
    expect(hoisted.refetchAITrace).toHaveBeenCalled();
  });

  it("restarts and closes ai workloads from case trace", async () => {
    const user = userEvent.setup();
    hoisted.restartWorkloadMutate.mockImplementation((_payload: any, options?: { onSuccess?: () => void }) => {
      options?.onSuccess?.();
    });
    hoisted.closeWorkloadMutate.mockImplementation((_payload: any, options?: { onSuccess?: () => void }) => {
      options?.onSuccess?.();
    });

    render(<CaseDetailPage />);

    await user.click(screen.getByTestId("tab-ai"));
    await user.click(screen.getByTestId("case-ai-workload-workload-1"));
    await user.click(screen.getByTestId("case-ai-workload-restart"));
    await user.click(screen.getByTestId("case-ai-workload-close"));

    expect(hoisted.restartWorkloadMutate).toHaveBeenCalledWith({ workloadId: "workload-1" }, expect.any(Object));
    expect(hoisted.closeWorkloadMutate).toHaveBeenCalledWith({ workloadId: "workload-1" }, expect.any(Object));
  });

  it("assigns current analyst and opens escalation dialog from case ai trace", async () => {
    const user = userEvent.setup();
    hoisted.updateCaseMutate.mockImplementation((_payload: any, options?: { onSuccess?: () => void }) => {
      options?.onSuccess?.();
    });

    render(<CaseDetailPage />);

    await user.click(screen.getByTestId("tab-ai"));
    await user.click(screen.getByTestId("case-ai-workloads-assign-workload-1"));

    expect(hoisted.updateCaseMutate).toHaveBeenCalledWith(
      {
        id: "case-1",
        data: expect.objectContaining({ assignee: "user-1" }),
      },
      expect.any(Object),
    );

    await user.click(screen.getByTestId("case-ai-workloads-escalate-workload-1"));
    expect(screen.getByTestId("dialog-escalate-case")).toBeInTheDocument();
    expect(screen.getByDisplayValue(/IOC requires manual review/i)).toBeInTheDocument();
  });

  it("creates quick reminder event from timeline", async () => {
    const user = userEvent.setup();
    render(<CaseDetailPage />);

    await user.click(screen.getByTestId("tab-timeline"));
    await user.type(screen.getByTestId("input-reminder-message"), "Check containment confirmation");
    await user.click(screen.getByTestId("button-schedule-reminder-15m"));

    expect(hoisted.createTimelineEventMutate).toHaveBeenCalledTimes(1);
    const [payload] = hoisted.createTimelineEventMutate.mock.calls[0];
    expect(payload).toMatchObject({
      caseId: "case-1",
      eventType: "reminder",
      body: "Check containment confirmation",
    });
    expect(payload.metadata.offset_minutes).toBe(15);
  });
});
