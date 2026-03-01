import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

const hoisted = vi.hoisted(() => ({
  saveNotificationSettingsMutate: vi.fn(),
  updateUserMutate: vi.fn(),
  revokeOtherSessionsMutate: vi.fn(),
  authSessionsData: {
    sessions: [
      {
        id: "session-current",
        userAgent: "Chrome",
        ipAddress: "127.0.0.1",
        status: "active",
        isCurrent: true,
        createdAt: "2026-01-01T00:00:00Z",
        expiresAt: "2126-01-01T00:00:00Z",
      },
      {
        id: "session-other",
        userAgent: "Firefox",
        ipAddress: "10.10.10.10",
        status: "active",
        isCurrent: false,
        createdAt: "2026-01-01T00:00:00Z",
        expiresAt: "2126-01-01T00:00:00Z",
      },
    ],
    sessionTimeoutSeconds: 3600,
  },
}));

vi.mock("@/components/layout", () => ({
  AppLayout: ({ children }: { children: any }) => <div data-testid="layout">{children}</div>,
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

vi.mock("@/lib/api", () => ({
  useAppState: () => ({ currentTenantId: "tenant-1", currentUserId: "user-1" }),
  useAuthSessions: () => ({
    data: hoisted.authSessionsData,
    isLoading: false,
  }),
  useRevokeOtherSessions: () => ({ mutate: hoisted.revokeOtherSessionsMutate, isPending: false }),
  useUpdateUser: () => ({ mutate: hoisted.updateUserMutate, isPending: false }),
  useNotificationBots: () => ({
    data: [
      {
        id: "bot-1",
        name: "Tenant Bot",
        botUsername: "tenant_bot",
      },
    ],
    isLoading: false,
  }),
  useMyNotificationSettings: () => ({
    data: {
      deliveryEnabled: false,
      deliveryChannel: "in_app",
      telegramBotId: "",
      telegramChatId: "",
      telegramUsername: "",
    },
    isLoading: false,
  }),
  useSaveMyNotificationSettings: () => ({
    mutate: hoisted.saveNotificationSettingsMutate,
    isPending: false,
  }),
}));

import SecurityPage from "@/pages/security";

describe("SecurityPage notification settings", () => {
  it("saves password via update user mutation", async () => {
    const user = userEvent.setup();
    hoisted.updateUserMutate.mockReset();
    hoisted.updateUserMutate.mockImplementation((_payload: any, opts?: { onSuccess?: () => void }) => {
      opts?.onSuccess?.();
    });

    render(<SecurityPage />);

    await user.type(screen.getByTestId("input-current-password"), "old-password");
    await user.type(screen.getByTestId("input-new-password"), "new-password");
    await user.type(screen.getByTestId("input-confirm-password"), "new-password");
    await user.click(screen.getByTestId("button-save-password"));

    await waitFor(() => {
      expect(hoisted.updateUserMutate).toHaveBeenCalledWith(
        {
          id: "user-1",
          data: {
            password: "new-password",
            currentPassword: "old-password",
          },
        },
        expect.any(Object),
      );
    });
  });

  it("terminates other sessions from sessions tab", async () => {
    const user = userEvent.setup();
    hoisted.revokeOtherSessionsMutate.mockReset();
    hoisted.revokeOtherSessionsMutate.mockImplementation((_payload: any, opts?: { onSuccess?: (data?: any) => void }) => {
      opts?.onSuccess?.({ revoked: 1 });
    });

    render(<SecurityPage />);

    await user.click(screen.getByTestId("tab-sessions"));
    await user.click(screen.getByTestId("button-terminate-sessions"));

    await waitFor(() => {
      expect(hoisted.revokeOtherSessionsMutate).toHaveBeenCalledWith(undefined, expect.any(Object));
    });
  });

  it("saves telegram notification settings", async () => {
    const user = userEvent.setup();
    hoisted.saveNotificationSettingsMutate.mockReset();
    hoisted.saveNotificationSettingsMutate.mockImplementation((_payload: any, opts?: { onSuccess?: () => void }) => {
      opts?.onSuccess?.();
    });

    render(<SecurityPage />);

    await user.click(screen.getByTestId("tab-notifications"));
    await screen.findByTestId("button-save-notification-settings");
    await user.click(screen.getByTestId("tab-delivery-telegram"));
    await user.click(screen.getByTestId("select-notification-bot"));
    await user.click(await screen.findByRole("option", { name: /Tenant Bot/i }));
    fireEvent.change(screen.getByTestId("input-telegram-chat-id"), { target: { value: "123456789" } });
    fireEvent.click(screen.getByTestId("button-save-notification-settings"));

    await waitFor(() => {
      expect(hoisted.saveNotificationSettingsMutate).toHaveBeenCalled();
    });
    const payload = hoisted.saveNotificationSettingsMutate.mock.calls[0][0];
    expect(payload.deliveryEnabled).toBe(false);
    expect(payload.deliveryChannel).toBe("telegram");
    expect(payload.telegramBotId).toBe("bot-1");
    expect(payload.telegramChatId).toBe("123456789");
  });

  it("validates email channel when external delivery is enabled", async () => {
    const user = userEvent.setup();
    hoisted.saveNotificationSettingsMutate.mockReset();

    render(<SecurityPage />);

    await user.click(screen.getByTestId("tab-notifications"));
    await screen.findByTestId("button-save-notification-settings");
    await user.click(screen.getByTestId("tab-delivery-email"));
    await user.click(screen.getByTestId("checkbox-delivery-enabled"));
    await user.click(screen.getByTestId("button-save-notification-settings"));

    await waitFor(() => {
      expect(hoisted.saveNotificationSettingsMutate).not.toHaveBeenCalled();
    });
  });

  it("saves time channel recipient", async () => {
    const user = userEvent.setup();
    hoisted.saveNotificationSettingsMutate.mockReset();
    hoisted.saveNotificationSettingsMutate.mockImplementation((_payload: any, opts?: { onSuccess?: () => void }) => {
      opts?.onSuccess?.();
    });

    render(<SecurityPage />);

    await user.click(screen.getByTestId("tab-notifications"));
    await screen.findByTestId("button-save-notification-settings");
    await user.click(screen.getByTestId("tab-delivery-time"));
    await user.click(screen.getByTestId("checkbox-delivery-enabled"));
    fireEvent.change(screen.getByTestId("input-time-recipient"), { target: { value: "secops-room" } });
    fireEvent.click(screen.getByTestId("button-save-notification-settings"));

    await waitFor(() => {
      expect(hoisted.saveNotificationSettingsMutate).toHaveBeenCalled();
    });
    const payload = hoisted.saveNotificationSettingsMutate.mock.calls[0][0];
    expect(payload.deliveryEnabled).toBe(true);
    expect(payload.deliveryChannel).toBe("time");
    expect(payload.timeRecipient).toBe("secops-room");
  });
});
