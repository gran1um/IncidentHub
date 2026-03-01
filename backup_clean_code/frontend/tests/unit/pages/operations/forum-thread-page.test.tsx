import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

const hoisted = vi.hoisted(() => ({
  createPostMutate: vi.fn(),
  createPostWithAttachmentsMutate: vi.fn(),
  proxySendMutate: vi.fn(),
  proxySyncMutate: vi.fn(),
  thread: {
    id: "thread-1",
    title: "Suspicious outbound traffic",
    status: "In Progress",
    caseId: "case-1",
    proxyProfiles: [
      {
        id: "profile-1",
        name: "SOC bridge",
        connectorId: "connector-1",
        bindingKey: "C123456789:1741439188.021300",
        lastSyncedAt: "2026-03-01T10:05:00.000Z",
        hasBinding: true,
        cursorPresent: true,
        metadata: {
          channel: "slack",
          channel_id: "C123456789",
          thread_ts: "1741439188.021300",
          participant: {
            channel_id: "C123456789",
            thread_ts: "1741439188.021300",
          },
        },
      },
    ],
    posts: [
      {
        id: "post-1",
        authorId: "user-2",
        authorName: "Alice Analyst",
        content: "We need to isolate the host.",
        timestamp: "2026-03-01T10:00:00.000Z",
      },
    ],
  },
  cases: [
    {
      id: "case-1",
      sev: "Critical",
      title: "Potential C2 activity",
      description: "Outbound beaconing detected",
      owner: "user-2",
      time: "2026-03-01T09:30:00.000Z",
      tags: ["c2", "network"],
    },
  ],
  users: [
    { id: "user-1", name: "Current User", avatar: "" },
    { id: "user-2", name: "Alice Analyst", avatar: "" },
  ],
}));

vi.mock("@/components/layout", () => ({
  AppLayout: ({ children }: { children: any }) => <div data-testid="layout">{children}</div>,
}));

vi.mock("wouter", () => ({
  Link: ({ href, children }: { href: string; children: any }) => <a href={href}>{children}</a>,
  useParams: () => ({ id: "thread-1" }),
}));

vi.mock("@/lib/api", () => ({
  useAppState: () => ({
    currentTenantId: "tenant-1",
    currentTenantSlug: "tenant-1",
    currentUserId: "user-1",
  }),
  useForumThread: () => ({ data: hoisted.thread }),
  useCreateForumPost: () => ({ mutate: hoisted.createPostMutate, isPending: false }),
  useCreateForumPostWithAttachments: () => ({ mutate: hoisted.createPostWithAttachmentsMutate, isPending: false }),
  useProxyForumSend: () => ({ mutate: hoisted.proxySendMutate, isPending: false }),
  useProxyForumSync: () => ({ mutate: hoisted.proxySyncMutate, isPending: false }),
  useUser: () => ({ data: hoisted.users[0] }),
  useUsers: () => ({ data: hoisted.users }),
  useCases: () => ({ data: hoisted.cases }),
  useCaseCommunicationConnectors: () => ({
    data: [
      {
        id: "connector-1",
        name: "SOC bridge",
        channel: "slack",
        communicationMode: "chat",
        direction: "outbound",
      },
    ],
  }),
}));

import ForumThreadPage from "@/pages/forum-thread";

describe("ForumThreadPage", () => {
  it("renders case context, posts a local reply, and sends via a saved connector route", async () => {
    const user = userEvent.setup();
    hoisted.createPostMutate.mockReset();
    hoisted.proxySendMutate.mockReset();
    hoisted.proxySendMutate.mockImplementation((_payload: any, options?: { onSuccess?: (payload?: any) => void }) => {
      options?.onSuccess?.({ profile_id: "profile-1" });
    });

    render(<ForumThreadPage />);

    expect(screen.getByText("Suspicious outbound traffic")).toBeInTheDocument();
    expect(screen.getByText("We need to isolate the host.")).toBeInTheDocument();
    await user.click(screen.getByTestId("button-toggle-forum-case-context"));
    expect(screen.getByText("Potential C2 activity")).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText("Write a reply..."), {
      target: { value: "Starting containment actions now." },
    });
    await user.click(screen.getByTestId("button-forum-send"));

    await waitFor(() => {
      expect(hoisted.createPostMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          threadId: "thread-1",
          authorId: "user-1",
          content: "Starting containment actions now.",
        }),
        expect.any(Object),
      );
    });

    fireEvent.change(screen.getByPlaceholderText("Write a reply..."), {
      target: { value: "Sync this with external channel" },
    });
    await user.click(screen.getByTestId("button-forum-send-via-connector"));
    expect(await screen.findByText("Linked external route")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText(/Last synced:/)).toBeInTheDocument());
    await waitFor(() => expect(screen.getByTestId("input-forum-thread-connector-chat-id")).toHaveValue("C123456789"));
    await waitFor(() => expect(screen.getByDisplayValue("1741439188.021300")).toBeInTheDocument());
    await user.click(screen.getByTestId("button-forum-send-via-connector-confirm"));

    await waitFor(() => {
      expect(hoisted.proxySendMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          threadId: "thread-1",
          connectorId: "connector-1",
          profileId: "profile-1",
          bindingKey: "C123456789:1741439188.021300",
          content: "Sync this with external channel",
          metadata: expect.objectContaining({
            channel_id: "C123456789",
            thread_ts: "1741439188.021300",
          }),
        }),
        expect.any(Object),
      );
    });
  });

  it("syncs replies through the selected saved proxy profile", async () => {
    const user = userEvent.setup();
    hoisted.proxySyncMutate.mockReset();
    hoisted.proxySyncMutate.mockImplementation((_payload: any, options?: { onSuccess?: (payload?: any) => void }) => {
      options?.onSuccess?.({ created_count: 2, synced_at: "2026-03-01T10:10:00.000Z" });
    });

    render(<ForumThreadPage />);

    await user.click(screen.getByTestId("button-forum-send-via-connector"));
    expect(await screen.findByText("Linked external route")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText(/Last synced:/)).toBeInTheDocument());
    await user.click(screen.getByTestId("button-forum-sync-replies"));

    await waitFor(() => {
      expect(hoisted.proxySyncMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          threadId: "thread-1",
          connectorId: "connector-1",
          profileId: "profile-1",
          bindingKey: "C123456789:1741439188.021300",
        }),
        expect.any(Object),
      );
    });
  });
});
