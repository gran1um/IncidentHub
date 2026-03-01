import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const hoisted = vi.hoisted(() => ({
  createThreadMutate: vi.fn(),
  sendMessageMutate: vi.fn(),
  syncThreadMutate: vi.fn(),
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
  connectors: [] as any[],
  users: [] as any[],
  threads: [] as any[],
  threadDetails: {} as Record<string, any>,
  communicationTemplates: [] as any[],
}));

vi.mock("sonner", () => ({
  toast: {
    success: hoisted.toastSuccess,
    error: hoisted.toastError,
  },
}));

vi.mock("@/lib/api", () => ({
  useCaseCommunicationConnectors: () => ({ data: hoisted.connectors }),
  useCommunicationTemplates: () => ({ data: hoisted.communicationTemplates }),
  useUsers: () => ({ data: hoisted.users }),
  useCaseCommunications: () => ({ data: hoisted.threads, isLoading: false }),
  useCaseCommunication: (_caseId: string, threadId: string) => {
    const id = threadId || hoisted.threads[0]?.id || "";
    return {
      data: id ? hoisted.threadDetails[id] || null : null,
      isLoading: false,
    };
  },
  useCreateCaseCommunication: () => ({ mutate: hoisted.createThreadMutate, isPending: false }),
  useSendCaseCommunicationMessage: () => ({ mutate: hoisted.sendMessageMutate, isPending: false }),
  useSyncCaseCommunication: () => ({ mutate: hoisted.syncThreadMutate, isPending: false }),
}));

import { CaseCommunicationsTab } from "@/components/case-communications-tab";

describe("CaseCommunicationsTab", () => {
  beforeEach(() => {
    hoisted.createThreadMutate.mockReset();
    hoisted.sendMessageMutate.mockReset();
    hoisted.syncThreadMutate.mockReset();
    hoisted.toastSuccess.mockReset();
    hoisted.toastError.mockReset();
    hoisted.connectors = [
      { id: "time-1", name: "Time Primary", channel: "time" },
      { id: "tg-1", name: "Telegram Main", channel: "telegram", communicationMode: "chat" },
      { id: "slack-1", name: "Slack SOC", channel: "slack", communicationMode: "chat" },
      { id: "outlook-1", name: "Outlook Mail", channel: "outlook", communicationMode: "email" },
    ];
    hoisted.users = [
      { id: "u-1", name: "Alice Analyst", username: "alice", email: "alice@example.com", timeRecipient: "time-user-1" },
      { id: "u-2", name: "Bob Responder", username: "bob", email: "bob@example.com", timeRecipient: "time-user-2" },
    ];
    hoisted.threads = [];
    hoisted.threadDetails = {};
    hoisted.communicationTemplates = [];
    hoisted.createThreadMutate.mockImplementation((_payload: any, options?: { onSuccess?: (created: any) => void }) => {
      options?.onSuccess?.({
        id: "thread-time-1",
        connectorId: "time-1",
        channel: "time",
        title: "Alice Analyst · Time",
        participant: {
          user_id: "u-1",
          target: "time-user-1",
        },
        messages: [],
      });
    });
  });

  it("starts Time chat with selected user", async () => {
    render(
      <CaseCommunicationsTab
        caseId="case-1"
        tenantId="tenant-1"
        currentUserId="u-analyst"
        currentUserName="Analyst"
      />,
    );

    const button = await screen.findByTestId("button-start-time-chat");
    await userEvent.click(button);

    await waitFor(() => expect(hoisted.createThreadMutate).toHaveBeenCalledTimes(1));
    const payload = hoisted.createThreadMutate.mock.calls[0][0];
    expect(payload.channel).toBe("time");
    expect(payload.connectorId).toBe("time-1");
    expect(payload.participant.user_id).toBe("u-1");
    expect(payload.participant.target).toBe("time-user-1");
    expect(payload.metadata.recipient).toBe("time-user-1");
  });

  it("renders persisted message history for selected thread", async () => {
    hoisted.threads = [
      {
        id: "thread-time-1",
        title: "Alice Analyst · Time",
        channel: "time",
        connectorId: "time-1",
        participant: {
          user_id: "u-1",
          target: "time-user-1",
          name: "Alice Analyst",
        },
      },
    ];
    hoisted.threadDetails = {
      "thread-time-1": {
        id: "thread-time-1",
        title: "Alice Analyst · Time",
        channel: "time",
        connectorId: "time-1",
        participant: {
          user_id: "u-1",
          target: "time-user-1",
          name: "Alice Analyst",
        },
        lastMessageAt: "2026-02-27T11:00:00Z",
        messages: [
          {
            id: "msg-1",
            direction: "outbound",
            authorId: "u-analyst",
            authorName: "Analyst",
            content: "Need update on isolation.",
            timestamp: "2026-02-27T11:00:00Z",
          },
          {
            id: "msg-2",
            direction: "inbound",
            authorId: "u-1",
            authorName: "Alice Analyst",
            content: "Host isolated, collecting evidence.",
            timestamp: "2026-02-27T11:01:00Z",
          },
        ],
      },
    };

    render(
      <CaseCommunicationsTab
        caseId="case-1"
        tenantId="tenant-1"
        currentUserId="u-analyst"
        currentUserName="Analyst"
      />,
    );

    expect(await screen.findByText("Need update on isolation.")).toBeInTheDocument();
    expect(await screen.findByText("Host isolated, collecting evidence.")).toBeInTheDocument();
    expect(screen.getByTestId("communication-messages-list")).toBeInTheDocument();
  });

  it("sends message using selected communication template", async () => {
    hoisted.communicationTemplates = [
      { id: "tpl-1", name: "Escalation Email" },
    ];
    hoisted.threads = [
      {
        id: "thread-email-1",
        title: "Email Thread",
        channel: "email",
        connectorId: "outlook-1",
        participant: {
          email: "user@example.com",
        },
        subject: "Account blocked: CASE-1",
        communicationMode: "email",
        metadata: {
          subject: "Account blocked: CASE-1",
          to: "user@example.com",
        },
      },
    ];
    hoisted.threadDetails = {
      "thread-email-1": {
        id: "thread-email-1",
        title: "Email Thread",
        channel: "email",
        connectorId: "outlook-1",
        participant: {
          email: "user@example.com",
        },
        subject: "Account blocked: CASE-1",
        communicationMode: "email",
        metadata: {
          subject: "Account blocked: CASE-1",
          to: "user@example.com",
        },
        messages: [],
      },
    };
    hoisted.sendMessageMutate.mockImplementation((_payload: any, options?: { onSuccess?: () => void }) => {
      options?.onSuccess?.();
    });

    render(
      <CaseCommunicationsTab
        caseId="case-1"
        tenantId="tenant-1"
        currentUserId="u-analyst"
        currentUserName="Analyst"
      />,
    );

    await userEvent.click(await screen.findByTestId("select-communication-template"));
    await userEvent.click(await screen.findByText("Escalation Email"));
    fireEvent.change(screen.getByTestId("input-communication-template-vars"), {
      target: { value: "{\"case_ref\":\"CASE-1\"}" },
    });
    await userEvent.type(screen.getByTestId("textarea-communication-message"), "Please review and confirm.");
    await userEvent.click(screen.getByTestId("button-send-communication-message"));

    await waitFor(() => expect(hoisted.sendMessageMutate).toHaveBeenCalledTimes(1));
    const payload = hoisted.sendMessageMutate.mock.calls[0][0];
    expect(payload.templateId).toBe("tpl-1");
    expect(payload.subject).toBe("Account blocked: CASE-1");
    expect(payload.templateVars).toEqual({ case_ref: "CASE-1" });
  });

  it("renders channel-aware route summary and sync freshness metadata", async () => {
    hoisted.threads = [
      {
        id: "thread-slack-1",
        title: "Slack Thread",
        channel: "slack",
        connectorId: "slack-1",
        communicationMode: "chat",
        lastSyncedAt: "2026-03-02T12:00:00Z",
        lastSyncCreatedCount: 2,
        participant: {
          channel_id: "C123456789",
          target: "C123456789",
        },
        metadata: {
          channel_id: "C123456789",
          thread_ts: "1741439188.021300",
        },
      },
    ];
    hoisted.threadDetails = {
      "thread-slack-1": {
        id: "thread-slack-1",
        title: "Slack Thread",
        channel: "slack",
        connectorId: "slack-1",
        communicationMode: "chat",
        lastSyncedAt: "2026-03-02T12:00:00Z",
        lastSyncCreatedCount: 2,
        participant: {
          channel_id: "C123456789",
          target: "C123456789",
        },
        metadata: {
          channel_id: "C123456789",
          thread_ts: "1741439188.021300",
        },
        messages: [],
      },
    };

    render(
      <CaseCommunicationsTab
        caseId="case-1"
        tenantId="tenant-1"
        currentUserId="u-analyst"
        currentUserName="Analyst"
      />,
    );

    expect((await screen.findAllByText(/Channel: C123456789/)).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/Last synced:/).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/Last pull: 2/).length).toBeGreaterThan(0);
  });

  it("syncs communication thread by subject", async () => {
    hoisted.threads = [
      {
        id: "thread-email-2",
        title: "Email Thread",
        channel: "email",
        connectorId: "outlook-1",
        subject: "Case follow-up",
        communicationMode: "email",
        metadata: {
          subject: "Case follow-up",
          to: "user@example.com",
        },
      },
    ];
    hoisted.threadDetails = {
      "thread-email-2": {
        id: "thread-email-2",
        title: "Email Thread",
        channel: "email",
        connectorId: "outlook-1",
        subject: "Case follow-up",
        communicationMode: "email",
        metadata: {
          subject: "Case follow-up",
          to: "user@example.com",
        },
        messages: [],
      },
    };
    hoisted.syncThreadMutate.mockImplementation((_payload: any, options?: { onSuccess?: (payload?: any) => void }) => {
      options?.onSuccess?.({ created_count: 1 });
    });

    render(
      <CaseCommunicationsTab
        caseId="case-1"
        tenantId="tenant-1"
        currentUserId="u-analyst"
        currentUserName="Analyst"
      />,
    );

    await userEvent.click(await screen.findByTestId("button-sync-communication-thread"));

    await waitFor(() => expect(hoisted.syncThreadMutate).toHaveBeenCalledTimes(1));
    const payload = hoisted.syncThreadMutate.mock.calls[0][0];
    expect(payload.subject).toBe("Case follow-up");
  });
});
