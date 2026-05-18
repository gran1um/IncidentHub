import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const hoisted = vi.hoisted(() => ({
  createMutate: vi.fn(),
  updateMutate: vi.fn(),
  deleteMutate: vi.fn(),
  runMutate: vi.fn(),
  restartWorkloadMutate: vi.fn(),
  closeWorkloadMutate: vi.fn(),
  refetchOperationsOverview: vi.fn(),
  refetchEntityTrace: vi.fn(),
  setLocation: vi.fn(),
  agents: [
    {
      id: "agent-1",
      name: "Tag Hunter",
      description: "Investigates tagged incidents",
      prompt: "Investigate quickly",
      model: "",
      provider: "openai",
      endpoint: "",
      language: "auto",
      targetTypes: ["case"],
      caseTags: ["phishing", "critical"],
      alertSources: [],
      autoCaseTags: [],
      autoCloseCase: false,
      autoCloseVerdicts: [],
      autoCreateCaseFromAlert: false,
      enrichmentConnectorIds: [],
      notificationConnectorIds: [],
      investigationPlan: [],
      maxCasesPerRun: 10,
      enabled: true,
      autoCreateTasks: true,
      autoComment: true,
      taskAssigneeId: "",
      executionPolicy: "all_matching",
      executionPriority: 0,
      triadEnabled: false,
      triadCriticalOnly: true,
      investigatorPrompt: "",
      reviewerPrompt: "",
      arbiterPrompt: "",
      requireReviewerConsensus: true,
      autoActionMinConfidence: 85,
    },
  ],
  connectors: [
    { id: "connector-1", name: "Threat Intel", channel: "http", enabled: true },
  ],
  users: [
    { id: "user-1", name: "Sarah Connor", email: "sarah@example.com", role: "Analyst" },
  ],
  runsFeed: [] as any[],
  operationsOverview: {
    queue: {},
    workers: {},
    processing_events: [],
    queued_events: [],
    recent_failed_events: [],
    agent_runtime: [],
  } as any,
  entityTrace: null as any,
}));

vi.mock("@/components/layout", () => ({
  AppLayout: ({ children }: { children: any }) => <div data-testid="layout">{children}</div>,
}));

vi.mock("wouter", () => ({
  useLocation: () => ["/tenant-1/ai-agents", hoisted.setLocation],
}));

vi.mock("@/lib/api", () => ({
  useAppState: () => ({ currentTenantId: "tenant-1", currentTenantSlug: "tenant-1" }),
  useAIAgents: () => ({ data: hoisted.agents, isLoading: false }),
  useOutboundConnectors: () => ({ data: hoisted.connectors }),
  useAIAgentRunsFeed: () => ({ data: hoisted.runsFeed, isLoading: false }),
  useAIAgentOperationsOverview: () => ({
    data: hoisted.operationsOverview,
    isLoading: false,
    isFetching: false,
    refetch: hoisted.refetchOperationsOverview,
  }),
  useAIAgentEntityTrace: () => ({
    data: hoisted.entityTrace,
    isLoading: false,
    isFetching: false,
    dataUpdatedAt: Date.parse("2026-03-06T12:00:00Z"),
    refetch: hoisted.refetchEntityTrace,
  }),
  useUsers: () => ({ data: hoisted.users, isLoading: false }),
  useCreateAIAgent: () => ({ mutate: hoisted.createMutate, isPending: false }),
  useUpdateAIAgent: () => ({ mutate: hoisted.updateMutate, isPending: false }),
  useDeleteAIAgent: () => ({ mutate: hoisted.deleteMutate, isPending: false }),
  useRunAIAgent: () => ({ mutate: hoisted.runMutate, isPending: false }),
  useRestartAIAgentWorkload: () => ({ mutate: hoisted.restartWorkloadMutate, isPending: false }),
  useCloseAIAgentWorkload: () => ({ mutate: hoisted.closeWorkloadMutate, isPending: false }),
  useAIAgentRuns: () => ({ data: [], isLoading: false }),
}));

import AIAgentsPage from "@/pages/ai-agents";

describe("AIAgentsPage", () => {
  beforeEach(() => {
    hoisted.createMutate.mockReset();
    hoisted.updateMutate.mockReset();
    hoisted.deleteMutate.mockReset();
    hoisted.runMutate.mockReset();
    hoisted.restartWorkloadMutate.mockReset();
    hoisted.closeWorkloadMutate.mockReset();
    hoisted.refetchOperationsOverview.mockReset();
    hoisted.refetchEntityTrace.mockReset();
    hoisted.setLocation.mockReset();
    hoisted.runsFeed = [];
    hoisted.entityTrace = null;
    hoisted.operationsOverview = {
      queue: {},
      workers: {},
      processing_events: [],
      queued_events: [],
      recent_failed_events: [],
      agent_runtime: [],
    };
  });

  it("runs configured agent from card action", async () => {
    render(<AIAgentsPage />);

    expect(screen.getByText("AI Agents")).toBeInTheDocument();
    expect(screen.getByText("Tag Hunter")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("button-run-agent-agent-1"));

    await waitFor(() => {
      expect(hoisted.runMutate).toHaveBeenCalledWith(
        {
          agentId: "agent-1",
          payload: { dry_run: false },
        },
        expect.any(Object),
      );
    });
  });

  it("creates a new ai agent with tag targeting", async () => {
    render(<AIAgentsPage />);

    fireEvent.click(screen.getByTestId("button-new-ai-agent"));
    fireEvent.change(screen.getByLabelText("Agent Name"), { target: { value: "Auto Triager" } });
    fireEvent.change(screen.getByLabelText("Case Filter Tags (optional)"), { target: { value: "malware, vip" } });
    fireEvent.change(screen.getByLabelText("Provider"), { target: { value: "openai" } });
    fireEvent.change(screen.getByLabelText("Endpoint (optional)"), { target: { value: "http://192.168.31.190:8000/v1" } });
    fireEvent.change(screen.getByLabelText("Model (optional)"), { target: { value: "cyankiwi/Qwen3.5-27B-AWQ-4bit" } });
    fireEvent.click(screen.getByTestId("button-save-ai-agent"));

    await waitFor(() => {
        expect(hoisted.createMutate).toHaveBeenCalledWith(
          expect.objectContaining({
            name: "Auto Triager",
            targetTypes: ["case"],
            caseTags: ["malware", "vip"],
            provider: "openai",
            endpoint: "http://192.168.31.190:8000/v1",
          model: "cyankiwi/Qwen3.5-27B-AWQ-4bit",
        }),
        expect.any(Object),
      );
    });
  });

  it("saves alert-targeted flow with auto-case and stage plan", async () => {
    render(<AIAgentsPage />);

    fireEvent.click(screen.getByTestId("button-new-ai-agent"));
    fireEvent.change(screen.getByLabelText("Agent Name"), { target: { value: "Alert Responder" } });
    fireEvent.click(screen.getByText("Alerts"));
    fireEvent.change(screen.getByLabelText("Alert Sources (optional)"), { target: { value: "edr, siem" } });
    fireEvent.click(screen.getByText("Add Stage"));
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Enrichment stage" } });
    fireEvent.change(screen.getByLabelText("Prompt"), { target: { value: "Check IOC reputation" } });
    fireEvent.click(screen.getByTestId("button-save-ai-agent"));

    await waitFor(() => {
      expect(hoisted.createMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Alert Responder",
          targetTypes: ["case", "alert"],
          alertSources: ["edr", "siem"],
          investigationPlan: [
            expect.objectContaining({
              name: "Enrichment stage",
              prompt: "Check IOC reputation",
            }),
          ],
        }),
        expect.any(Object),
      );
    });
  });

  it("saves execution policy fields for agent scheduling", async () => {
    render(<AIAgentsPage />);

    fireEvent.click(screen.getByTestId("button-new-ai-agent"));
    fireEvent.change(screen.getByLabelText("Agent Name"), { target: { value: "Fallback Analyst" } });
    fireEvent.click(screen.getByLabelText("Execution Policy"));
    fireEvent.click(screen.getByText("Fallback chain"));
    fireEvent.change(screen.getByLabelText("Execution Priority"), { target: { value: "25" } });
    fireEvent.click(screen.getByTestId("button-save-ai-agent"));

    await waitFor(() => {
      expect(hoisted.createMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Fallback Analyst",
          executionPolicy: "fallback_chain",
          executionPriority: 25,
        }),
        expect.any(Object),
      );
    });
  });

  it("saves triad policy fields for critical investigations", async () => {
    render(<AIAgentsPage />);

    fireEvent.click(screen.getByTestId("button-new-ai-agent"));
    fireEvent.change(screen.getByLabelText("Agent Name"), { target: { value: "Critical Triad" } });
    fireEvent.click(screen.getByTestId("switch-triad-enabled"));
    fireEvent.change(screen.getByLabelText("Auto-action minimum confidence"), { target: { value: "90" } });
    fireEvent.change(screen.getByLabelText("Investigator prompt (optional)"), { target: { value: "Investigate deeply" } });
    fireEvent.change(screen.getByLabelText("Reviewer prompt (optional)"), { target: { value: "Challenge assumptions" } });
    fireEvent.change(screen.getByLabelText("Arbiter prompt (optional)"), { target: { value: "Produce final verdict" } });
    fireEvent.click(screen.getByTestId("button-save-ai-agent"));

    await waitFor(() => {
      expect(hoisted.createMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Critical Triad",
          triadEnabled: true,
          triadCriticalOnly: true,
          requireReviewerConsensus: true,
          autoActionMinConfidence: 90,
          investigatorPrompt: "Investigate deeply",
          reviewerPrompt: "Challenge assumptions",
          arbiterPrompt: "Produce final verdict",
        }),
        expect.any(Object),
      );
    });
  });

  it("opens case detail from triad analytics card", async () => {
    hoisted.runsFeed = [
      {
        id: "run-triad-1",
        agent_id: "agent-1",
        agent_name: "Tag Hunter",
        started_at: "2026-03-06T10:00:00Z",
        results: [
          {
            case_id: "case-triad-1",
            case_number: "CASE-TRIAD-1",
            title: "Critical malware case",
            verdict: "malicious",
            confidence: 96.2,
            auto_actions_allowed: false,
            requires_human_review: true,
            reviewer_consensus: false,
            action_blockers: ["manual review required"],
            triad: {
              investigator: { verdict: "malicious", confidence: 94.4 },
              reviewer: { verdict: "suspicious", confidence: 82.1 },
              arbiter: { verdict: "malicious", confidence: 96.2 },
            },
          },
        ],
      },
    ];

    const user = userEvent.setup();
    render(<AIAgentsPage />);

    await user.click(screen.getByTestId("tab-ai-triad"));
    await user.click(screen.getByTestId("button-triad-open-case-run-triad-1"));

    expect(hoisted.setLocation).toHaveBeenCalledWith("/tenant-1/cases/case-triad-1?tab=ai");
  });

  it("opens operations trace drawer from active workload summary card", async () => {
    hoisted.operationsOverview = {
      queue: { processing: 1 },
      workers: { poll_interval_ms: 3000, batch_size: 20 },
      processing_events: [
        {
          id: "queue-event-drawer-1",
          entity_type: "case",
          entity_id: "case-drawer-1",
          entity_reference: "CASE-20260306-DRW1",
          entity_title: "Drawer workload case",
          status: "processing",
          workflow_id: "ai-queue-drawer-1",
          candidate_count: 1,
          candidate_agents: [{ id: "agent-1", name: "Tag Hunter" }],
          workloads: [
            {
              id: "workload-drawer-1",
              agent_id: "agent-1",
              agent_name: "Tag Hunter",
              status: "failed",
              attempt_count: 1,
              max_attempts: 3,
              last_stage: "analysis",
              last_error: "timeout",
              run: {
                result: {
                  summary: "Connector timed out during enrichment",
                  stage_timeline: [
                    { id: "analysis", name: "Analysis", status: "failed" },
                  ],
                },
              },
            },
          ],
          workload_count: 1,
        },
      ],
      queued_events: [],
      recent_failed_events: [],
      agent_runtime: [],
    };

    const user = userEvent.setup();
    render(<AIAgentsPage />);

    await user.click(screen.getByTestId("tab-ai-operations"));
    await waitFor(() => {
      expect(screen.getByTestId("tab-ai-operations")).toHaveAttribute("data-state", "active");
      expect(screen.getByTestId("button-workload-inspect-queue-event-drawer-1")).toBeInTheDocument();
    });
    await user.click(screen.getByTestId("button-workload-inspect-queue-event-drawer-1"));

    await waitFor(() => {
      expect(screen.getByTestId("operations-trace-drawer")).toBeInTheDocument();
      expect(screen.getByTestId("operations-trace-drawer-event-log")).toBeInTheDocument();
      expect(screen.getByText("Live Event Log")).toBeInTheDocument();
      expect(screen.getByTestId("ops-drawer-ai-trace-card")).toBeInTheDocument();
      expect(screen.getAllByText("Drawer workload case").length).toBeGreaterThan(0);
      expect(screen.getAllByText("Analysis").length).toBeGreaterThan(0);
    });

    await user.click(screen.getByTestId("button-operations-trace-drawer-open-entity"));
    expect(hoisted.setLocation).toHaveBeenCalledWith("/tenant-1/cases/case-drawer-1?tab=ai&ai_workload=workload-drawer-1&ai_stage=analysis");
  });

  it("expands active workload diagnostics with run-level error details", async () => {
    hoisted.operationsOverview = {
      queue: { processing: 1 },
      workers: { poll_interval_ms: 3000, batch_size: 20 },
      processing_events: [
        {
          id: "queue-event-1",
          entity_type: "case",
          entity_id: "case-1",
          entity_reference: "CASE-20260305-9DF6FC",
          entity_title: "Bulk Created Case",
          status: "processing",
          workflow_id: "ai-queue-1",
          matched_agents: 1,
          processed_agents: 0,
          candidate_count: 1,
          candidate_agents: [{ id: "agent-1", name: "Tag Hunter" }],
          workloads: [
            {
              id: "workload-1",
              agent_id: "agent-1",
              agent_name: "Tag Hunter",
              status: "failed",
              run_id: "run-1",
              attempt_count: 1,
              max_attempts: 3,
              last_stage: "analysis",
              last_error: "model is overloaded",
              started_at: "2026-03-05T10:02:00Z",
              finished_at: "2026-03-05T10:03:00Z",
              run: {
                id: "run-1",
                status: "completed_with_error",
                started_at: "2026-03-05T10:02:00Z",
                finished_at: "2026-03-05T10:03:00Z",
                result: {
                  error: "model is overloaded",
                  stage_timeline: [
                    { id: "stage-1", name: "Enrichment", status: "partial", connector_count: 1, connector_success_count: 0, connector_error_count: 1 },
                    { id: "analysis", name: "Analysis", status: "failed", error: "model is overloaded" },
                  ],
                  connector_timeline: [
                    { connector_id: "connector-1", name: "Threat Intel", status: "failed", stage_name: "Enrichment", error: "connector timed out" },
                  ],
                },
              },
            },
          ],
          workload_count: 1,
          created_at: "2026-03-05T10:00:00Z",
          updated_at: "2026-03-05T10:05:00Z",
          started_at: "2026-03-05T10:01:00Z",
          last_error: "llm timeout after 60s",
        },
      ],
      queued_events: [],
      recent_failed_events: [],
      agent_runtime: [],
    };
    hoisted.runsFeed = [];

    const user = userEvent.setup();

    render(<AIAgentsPage />);

    await user.click(screen.getByTestId("tab-ai-operations"));

    await waitFor(() => {
      expect(screen.getByTestId("tab-ai-operations")).toHaveAttribute("data-state", "active");
      expect(screen.getByTestId("button-workload-expand-queue-event-1")).toBeInTheDocument();
    });
    await user.click(screen.getByTestId("button-workload-expand-queue-event-1"));

    await waitFor(() => {
      expect(screen.getByText("Workload Diagnostics")).toBeInTheDocument();
      expect(screen.getByText("llm timeout after 60s")).toBeInTheDocument();
      expect(screen.getAllByText("model is overloaded").length).toBeGreaterThan(0);
      expect(screen.getByText("Stage Timeline")).toBeInTheDocument();
      expect(screen.getByText("Connector Timeline")).toBeInTheDocument();
      expect(screen.getByText("Threat Intel")).toBeInTheDocument();
    });
  });

  it("opens entity trace and targeted stage from operations workload trace", async () => {
    hoisted.operationsOverview = {
      queue: { processing: 1 },
      workers: { poll_interval_ms: 3000, batch_size: 20 },
      processing_events: [
        {
          id: "queue-event-link-1",
          entity_type: "case",
          entity_id: "case-42",
          entity_reference: "CASE-20260306-LINK",
          entity_title: "Deep-link case",
          status: "processing",
          workloads: [
            {
              id: "workload-link-1",
              agent_id: "agent-1",
              agent_name: "Tag Hunter",
              status: "failed",
              attempt_count: 1,
              max_attempts: 3,
              last_stage: "analysis",
              run: {
                result: {
                  stage_timeline: [
                    { id: "enrichment", name: "Enrichment", status: "completed" },
                    { id: "analysis", name: "Analysis", status: "failed" },
                  ],
                },
              },
            },
          ],
          workload_count: 1,
        },
      ],
      queued_events: [],
      recent_failed_events: [],
      agent_runtime: [],
    };

    const user = userEvent.setup();
    render(<AIAgentsPage />);

    await user.click(screen.getByTestId("tab-ai-operations"));
    await user.click(screen.getByTestId("button-workload-expand-queue-event-link-1"));

    await waitFor(() => {
      expect(screen.getByTestId("ops-ai-trace-open-entity-workload-link-1")).toBeInTheDocument();
      expect(screen.getByTestId("ops-ai-trace-open-stage-workload-link-1")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("ops-ai-trace-open-entity-workload-link-1"));
    expect(hoisted.setLocation).toHaveBeenCalledWith("/tenant-1/cases/case-42?tab=ai&ai_workload=workload-link-1");

    await user.click(screen.getByTestId("ops-ai-trace-open-stage-workload-link-1"));
    expect(hoisted.setLocation).toHaveBeenCalledWith("/tenant-1/cases/case-42?tab=ai&ai_workload=workload-link-1&ai_stage=analysis");
  });

  it("triggers workload restart and close actions from diagnostics controls", async () => {
    hoisted.operationsOverview = {
      queue: { processing: 1 },
      workers: { poll_interval_ms: 3000, batch_size: 20, max_retries: 3, llm_max_concurrent: 2 },
      processing_events: [
        {
          id: "queue-event-action-1",
          entity_type: "case",
          entity_reference: "CASE-20260305-ABCD",
          entity_title: "Action case",
          status: "processing",
          workflow_id: "ai-queue-action-1",
          matched_agents: 1,
          processed_agents: 0,
          candidate_count: 1,
          candidate_agents: [{ id: "agent-1", name: "Tag Hunter" }],
          workloads: [
            {
              id: "workload-action-1",
              agent_id: "agent-1",
              agent_name: "Tag Hunter",
              status: "failed",
              run_id: "run-action-1",
              attempt_count: 1,
              max_attempts: 3,
              last_stage: "analysis",
              last_error: "temporary failure",
            },
          ],
          workload_count: 1,
          created_at: "2026-03-05T10:00:00Z",
          updated_at: "2026-03-05T10:05:00Z",
          started_at: "2026-03-05T10:01:00Z",
          last_error: "temporary failure",
        },
      ],
      queued_events: [],
      recent_failed_events: [],
      agent_runtime: [],
    };

    const user = userEvent.setup();

    render(<AIAgentsPage />);
    await user.click(screen.getByTestId("tab-ai-operations"));

    await waitFor(() => {
      expect(screen.getByTestId("tab-ai-operations")).toHaveAttribute("data-state", "active");
      expect(screen.getByTestId("button-workload-expand-queue-event-action-1")).toBeInTheDocument();
    });
    await user.click(screen.getByTestId("button-workload-expand-queue-event-action-1"));
    fireEvent.click(screen.getByTestId("button-workload-restart-workload-action-1"));
    fireEvent.click(screen.getByTestId("button-workload-close-workload-action-1"));

    await waitFor(() => {
      expect(hoisted.restartWorkloadMutate).toHaveBeenCalledWith({ workloadId: "workload-action-1" }, expect.any(Object));
      expect(hoisted.closeWorkloadMutate).toHaveBeenCalledWith({ workloadId: "workload-action-1" }, expect.any(Object));
    });
  });
});
