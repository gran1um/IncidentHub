import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const hoisted = vi.hoisted(() => ({
  createConnectorTemplateMutate: vi.fn(),
  createCaseTemplateMutate: vi.fn(),
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
}));

vi.mock("@/components/layout", () => ({
  AppLayout: ({ children }: { children: any }) => <div data-testid="layout">{children}</div>,
}));

vi.mock("sonner", () => ({
  toast: {
    success: hoisted.toastSuccess,
    error: hoisted.toastError,
  },
}));

vi.mock("@/lib/api", () => ({
  useAppState: () => ({ currentTenantId: "tenant-1" }),
  useConnectorTemplates: () => ({ data: [], isLoading: false }),
  useCreateConnectorTemplate: () => ({ mutate: hoisted.createConnectorTemplateMutate, isPending: false }),
  useUpdateConnectorTemplate: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteConnectorTemplate: () => ({ mutate: vi.fn(), isPending: false }),
  useCommunicationTemplates: () => ({ data: [], isLoading: false }),
  useCreateCommunicationTemplate: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateCommunicationTemplate: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteCommunicationTemplate: () => ({ mutate: vi.fn(), isPending: false }),
  useCaseTemplates: () => ({ data: [], isLoading: false }),
  useCreateCaseTemplate: () => ({ mutate: hoisted.createCaseTemplateMutate, isPending: false }),
  useUpdateCaseTemplate: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteCaseTemplate: () => ({ mutate: vi.fn(), isPending: false }),
  useObservableTypes: () => ({ data: [], isLoading: false }),
  useCreateObservableType: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateObservableType: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteObservableType: () => ({ mutate: vi.fn(), isPending: false }),
  useUsers: () => ({ data: [{ id: "user-1", name: "Alice" }], isLoading: false }),
  useCaseStatuses: () => ({ data: [{ code: "new", label: "New", isClosed: false }], isLoading: false }),
}));

import TemplatesPage from "@/pages/templates";

async function chooseSelectOption(user: ReturnType<typeof userEvent.setup>, triggerTestId: string, optionLabel: string) {
  await user.click(screen.getByTestId(triggerTestId));
  await user.click(await screen.findByRole("option", { name: optionLabel }));
}

describe("TemplatesPage", () => {
  beforeEach(() => {
    hoisted.createConnectorTemplateMutate.mockReset();
    hoisted.createCaseTemplateMutate.mockReset();
    hoisted.toastSuccess.mockReset();
    hoisted.toastError.mockReset();
  });

  it("adds SOAR preset fields and includes them in the case template payload", async () => {
    const user = userEvent.setup();
    render(<TemplatesPage />);

    await user.click(screen.getByTestId("tab-case-templates"));
    await user.click(screen.getByTestId("button-new-case-template"));
    await user.click(screen.getByTestId("button-add-case-template-soar-fields"));

    expect(screen.getByDisplayValue("analyst")).toBeInTheDocument();
    expect(screen.getByDisplayValue("indicators")).toBeInTheDocument();
    expect(screen.getByDisplayValue("status")).toBeInTheDocument();
    expect(screen.getByDisplayValue("stages")).toBeInTheDocument();
    expect(screen.getByDisplayValue("description")).toBeInTheDocument();
    expect(screen.getByDisplayValue("inbound_event")).toBeInTheDocument();
    expect(screen.getByDisplayValue("criticality")).toBeInTheDocument();
    expect(screen.getByDisplayValue("role_types")).toBeInTheDocument();

    fireEvent.change(screen.getByTestId("input-case-template-name"), {
      target: { value: "SOAR Investigation Template" },
    });

    const analystKeyInput = screen.getByDisplayValue("analyst");
    const analystRow = analystKeyInput.closest("div[data-testid^='case-template-custom-field-row-']");
    if (!analystRow) {
      throw new Error("Analyst custom field row was not found");
    }
    const analystValueInput = analystRow.querySelector('input[data-testid^="input-case-template-custom-field-value-"]') as HTMLInputElement | null;
    if (!analystValueInput) {
      throw new Error("Analyst value input was not found");
    }
    fireEvent.change(analystValueInput, { target: { value: "SOC Analyst" } });

    fireEvent.click(screen.getByTestId("button-save-case-template"));

    expect(hoisted.createCaseTemplateMutate).toHaveBeenCalledTimes(1);
    const [payload] = hoisted.createCaseTemplateMutate.mock.calls[0];
    expect(payload.name).toBe("SOAR Investigation Template");
    expect(payload.status).toBe("new");
    expect(payload.priority).toBe("medium");
    expect(payload.customFields.analyst).toBe("SOC Analyst");
    expect(Object.keys(payload.customFields)).toEqual(
      expect.arrayContaining(["analyst", "indicators", "status", "stages", "description", "inbound_event", "criticality", "role_types"]),
    );
  });

  it("creates connector templates with tags, icon, and schema-driven forum fields", async () => {
    const user = userEvent.setup();
    render(<TemplatesPage />);

    await user.click(screen.getByTestId("tab-connector-templates"));
    await user.click(screen.getByTestId("button-new-connector-template"));
    await chooseSelectOption(user, "select-connector-template-preset", "Slack Chat");
    await chooseSelectOption(user, "select-connector-template-icon", "Shield");
    await user.type(screen.getByTestId("input-connector-template-tags"), "security, chat");
    await user.keyboard("{Enter}");
    await user.click(screen.getByTestId("button-save-connector-template"));

    expect(hoisted.createConnectorTemplateMutate).toHaveBeenCalledTimes(1);
    const [payload] = hoisted.createConnectorTemplateMutate.mock.calls[0];
    expect(payload.name).toBe("Slack Chat");
    expect(payload.type).toBe("Slack");
    expect(payload.channel).toBe("slack");
    expect(payload.icon).toBe("shield");
    expect(payload.tags).toEqual(expect.arrayContaining(["security", "chat"]));
    expect(payload.forms.forumSend).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ id: "chatId", label: "Channel ID" }),
        expect.objectContaining({ id: "externalUserId", label: "Thread TS" }),
      ]),
    );
  });
});
