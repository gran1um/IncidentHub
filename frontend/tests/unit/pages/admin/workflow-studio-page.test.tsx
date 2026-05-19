import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const hoisted = vi.hoisted(() => ({
  createMutate: vi.fn(),
  updateMutate: vi.fn(),
  deleteMutate: vi.fn(),
  testRunMutate: vi.fn(),
  askAIMutateAsync: vi.fn(),
  refetchRuns: vi.fn(),
  setLocation: vi.fn(),
  location: "/workflow-studio",
  workflowItems: [] as any[],
  workflowRuns: [] as any[],
  workflowCatalogLoading: false,
}));

vi.mock("@/components/layout", () => ({
  AppLayout: ({ children }: { children: any }) => <div data-testid="layout">{children}</div>,
}));

vi.mock("wouter", () => ({
  useLocation: () => [hoisted.location, hoisted.setLocation],
}));

vi.mock("@/lib/api", () => ({
  useAppState: () => ({ currentTenantId: "tenant-1" }),
  useWorkflowCatalog: () => ({ data: hoisted.workflowItems, isLoading: hoisted.workflowCatalogLoading }),
  useCreateWorkflowCatalogItem: () => ({ mutate: hoisted.createMutate, isPending: false }),
  useUpdateWorkflowCatalogItem: () => ({ mutate: hoisted.updateMutate, isPending: false }),
  useDeleteWorkflowCatalogItem: () => ({ mutate: hoisted.deleteMutate, isPending: false }),
  useWorkflowTestRun: () => ({ mutate: hoisted.testRunMutate, isPending: false }),
  useAskAI: () => ({ mutateAsync: hoisted.askAIMutateAsync, isPending: false }),
  useWorkflowRunHistory: () => ({ data: { runs: hoisted.workflowRuns, retentionLimit: 1000 }, refetch: hoisted.refetchRuns, isFetching: false }),
  useOutboundConnectors: () => ({ data: [] }),
  useConnectorMethods: () => ({ data: [] }),
  useWorkflowCredentials: () => ({ data: [] }),
  useCreateWorkflowCredential: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateWorkflowCredential: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteWorkflowCredential: () => ({ mutate: vi.fn(), isPending: false }),
}));

import WorkflowStudioPage from "@/pages/workflow-studio";

describe("WorkflowStudioPage", () => {
  beforeEach(() => {
    hoisted.createMutate.mockReset();
    hoisted.updateMutate.mockReset();
    hoisted.deleteMutate.mockReset();
    hoisted.testRunMutate.mockReset();
    hoisted.askAIMutateAsync.mockReset();
    hoisted.askAIMutateAsync.mockResolvedValue({
      session_id: "session-1",
      answer: JSON.stringify({
        name: "AI generated flow",
        description: "Generated for test",
        definition: {
          version: 1,
          entryNodeId: "node-1",
          nodes: [{ id: "node-1", type: "trigger", label: "Trigger", position: { x: 120, y: 80 }, config: {} }],
          edges: [],
        },
      }),
    });
    hoisted.refetchRuns.mockReset();
    hoisted.setLocation.mockReset();
    hoisted.location = "/workflow-studio";
    hoisted.workflowItems = [];
    hoisted.workflowRuns = [];
    hoisted.workflowCatalogLoading = false;
  });

  it("does not crash when loading state switches from skeleton to content", async () => {
    hoisted.workflowCatalogLoading = true;
    const { rerender } = render(<WorkflowStudioPage />);
    expect(screen.getByTestId("layout")).toBeInTheDocument();

    hoisted.workflowCatalogLoading = false;
    rerender(<WorkflowStudioPage />);

    expect(screen.getByTestId("text-workflow-studio-title")).toBeInTheDocument();
  });

  it("creates a workflow from block canvas", async () => {
    const user = userEvent.setup();
    render(<WorkflowStudioPage />);

    expect(screen.getByTestId("text-workflow-studio-title")).toHaveTextContent("Workflow Studio");
    expect(screen.getByTestId("card-workflow-list")).toBeInTheDocument();

    await user.click(screen.getByTestId("button-create-workflow"));
    await user.click(screen.getByTestId("button-open-block-library"));
    await user.click(screen.getByTestId("button-add-node-trigger"));
    await user.click(screen.getByTestId("button-open-workflow-settings"));
    await user.type(screen.getByTestId("input-workflow-name"), "IOC enrichment");
    await user.click(screen.getByTestId("button-save-workflow"));

    expect(hoisted.createMutate).toHaveBeenCalledTimes(1);
    expect(hoisted.createMutate.mock.calls[0][0]).toMatchObject({
      name: "IOC enrichment",
      type: "workflow",
      enabled: false,
    });
  });

  it("persists enabled switch immediately and allows test run for existing workflow", async () => {
    hoisted.workflowItems = [
      {
        id: "wf-1",
        name: "Existing workflow",
        description: "Saved workflow",
        status: "draft",
        enabled: false,
        definition: {
          version: 1,
          entryNodeId: "node-1",
          nodes: [{ id: "node-1", type: "trigger", label: "Trigger", position: { x: 10, y: 10 }, config: {} }],
          edges: [],
        },
      },
    ];

    const user = userEvent.setup();
    render(<WorkflowStudioPage />);

    expect(screen.getByTestId("card-workflow-list")).toBeInTheDocument();
    await user.click(screen.getByTestId("item-workflow-wf-1"));
    expect(screen.getByTestId("button-back-to-workflows")).toBeInTheDocument();
    await user.click(screen.getByTestId("button-open-workflow-settings"));
    await user.click(screen.getByTestId("switch-workflow-enabled"));

    expect(hoisted.updateMutate).toHaveBeenCalledTimes(1);
    expect(hoisted.updateMutate.mock.calls[0][0]).toMatchObject({
      id: "wf-1",
      data: { enabled: true },
    });

    await user.click(screen.getByTestId("button-test-workflow"));
    expect(hoisted.testRunMutate).toHaveBeenCalledTimes(1);
    expect(hoisted.testRunMutate.mock.calls[0][0]).toMatchObject({
      workflowID: "wf-1",
    });

    await user.click(screen.getByTestId("button-back-to-workflows"));
    expect(screen.getByTestId("card-workflow-list")).toBeInTheDocument();
  });

  it("paginates workflows with default page size 10", async () => {
    hoisted.workflowItems = Array.from({ length: 12 }, (_, index) => ({
      id: `wf-${index + 1}`,
      name: `Workflow ${index + 1}`,
      description: "Page check",
      status: "draft",
      enabled: false,
      definition: {
        version: 1,
        entryNodeId: "",
        nodes: [],
        edges: [],
      },
    }));

    const user = userEvent.setup();
    render(<WorkflowStudioPage />);

    expect(screen.getByTestId("item-workflow-wf-1")).toBeInTheDocument();
    expect(screen.getByTestId("item-workflow-wf-10")).toBeInTheDocument();
    expect(screen.queryByTestId("item-workflow-wf-11")).not.toBeInTheDocument();

    await user.click(screen.getByTestId("button-workflow-next-page"));

    expect(screen.getByTestId("item-workflow-wf-11")).toBeInTheDocument();
    expect(screen.getByTestId("item-workflow-wf-12")).toBeInTheDocument();
    expect(screen.queryByTestId("item-workflow-wf-1")).not.toBeInTheDocument();
  });

  it("generates workflow draft through AI assistant chat", async () => {
    const user = userEvent.setup();
    render(<WorkflowStudioPage />);

    await user.click(screen.getByTestId("button-open-workflow-ai-assistant"));
    await user.type(screen.getByTestId("textarea-workflow-ai-prompt"), "build phishing triage");
    await user.click(screen.getByTestId("button-workflow-ai-generate"));

    expect(hoisted.askAIMutateAsync).toHaveBeenCalledTimes(1);
    expect(await screen.findByText("JSON parsed")).toBeInTheDocument();

    const applyButtons = await screen.findAllByRole("button", { name: "Apply to canvas" });
    await user.click(applyButtons[0]);
    expect(screen.getByTestId("button-save-workflow")).toBeInTheDocument();
  });

});
