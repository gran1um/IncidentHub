import * as React from "react";

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const hoisted = vi.hoisted(() => ({
  setLocation: vi.fn(),
  setLanguage: vi.fn(),
  toggleTheme: vi.fn(),
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
  createAlertMutate: vi.fn(),
  createCaseMutate: vi.fn(),
  executeConnectorHubMutate: vi.fn(),
}));

vi.mock("wouter", () => ({
  useLocation: () => ["/", hoisted.setLocation],
  Link: ({ children, href }: { children: any; href: string }) => <a href={href}>{children}</a>,
}));

vi.mock("sonner", () => ({
  toast: {
    success: hoisted.toastSuccess,
    error: hoisted.toastError,
  },
}));

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({ language: "en", setLanguage: hoisted.setLanguage }),
  useT: () => (key: string) => key,
}));

vi.mock("@/lib/theme", () => ({
  useTheme: () => ({ theme: "light", toggleTheme: hoisted.toggleTheme }),
}));

vi.mock("@/components/ui/dropdown-menu", () => ({
  DropdownMenu: ({ children }: { children: any }) => <div>{children}</div>,
  DropdownMenuTrigger: ({ children }: { children: any }) => <div>{children}</div>,
  DropdownMenuContent: ({ children }: { children: any }) => <div>{children}</div>,
  DropdownMenuItem: ({ children, onClick, ...rest }: { children: any; onClick?: () => void }) => (
    <button type="button" onClick={onClick} {...rest}>
      {children}
    </button>
  ),
  DropdownMenuLabel: ({ children }: { children: any }) => <div>{children}</div>,
  DropdownMenuSeparator: () => <hr />,
}));

vi.mock("@/components/ui/select", () => {
  type SelectContextValue = { value: string; onValueChange: (value: string) => void };
  const SelectContext = React.createContext<SelectContextValue>({ value: "", onValueChange: () => undefined });

  return {
    Select: ({ children, value, onValueChange }: { children: any; value?: string; onValueChange?: (value: string) => void }) => (
      <SelectContext.Provider value={{ value: String(value ?? ""), onValueChange: onValueChange ?? (() => undefined) }}>
        <div>{children}</div>
      </SelectContext.Provider>
    ),
    SelectTrigger: ({ children, ...props }: { children: any }) => (
      <button type="button" {...props}>
        {children}
      </button>
    ),
    SelectValue: ({ placeholder }: { placeholder?: string }) => <span>{placeholder ?? ""}</span>,
    SelectContent: ({ children }: { children: any }) => <div>{children}</div>,
    SelectItem: ({ children, value }: { children: any; value: string }) => {
      const ctx = React.useContext(SelectContext);
      return (
        <button type="button" onClick={() => ctx.onValueChange(value)}>
          {children}
        </button>
      );
    },
  };
});

vi.mock("@/components/ui/popover", () => {
  type PopoverContextValue = { open: boolean; setOpen: (next: boolean) => void };
  const PopoverContext = React.createContext<PopoverContextValue>({ open: false, setOpen: () => undefined });

  const composeClickHandler = (originalOnClick: ((event: any) => void) | undefined, toggle: () => void) => (event: any) => {
    originalOnClick?.(event);
    toggle();
  };

  return {
    Popover: ({ children, open, onOpenChange }: { children: any; open?: boolean; onOpenChange?: (open: boolean) => void }) => {
      const [internalOpen, setInternalOpen] = React.useState(false);
      const isControlled = typeof open === "boolean";
      const resolvedOpen = isControlled ? Boolean(open) : internalOpen;
      const setOpen = (next: boolean) => {
        if (!isControlled) {
          setInternalOpen(next);
        }
        onOpenChange?.(next);
      };
      return <PopoverContext.Provider value={{ open: resolvedOpen, setOpen }}>{children}</PopoverContext.Provider>;
    },
    PopoverTrigger: ({ children, asChild }: { children: any; asChild?: boolean }) => {
      const ctx = React.useContext(PopoverContext);
      const toggle = () => ctx.setOpen(!ctx.open);
      if (asChild && React.isValidElement(children)) {
        const typedChild = children as React.ReactElement<{ onClick?: (event: any) => void }>;
        return React.cloneElement(typedChild, {
          onClick: composeClickHandler(typedChild.props.onClick, toggle),
        });
      }
      return (
        <button type="button" onClick={toggle}>
          {children}
        </button>
      );
    },
    PopoverContent: ({
      children,
      sideOffset: _sideOffset,
      side: _side,
      align: _align,
      ...props
    }: {
      children: any;
      sideOffset?: number;
      side?: string;
      align?: string;
    }) => {
      const ctx = React.useContext(PopoverContext);
      if (!ctx.open) {
        return null;
      }
      return <div {...props}>{children}</div>;
    },
    PopoverAnchor: ({ children }: { children: any }) => <>{children}</>,
  };
});

vi.mock("@/lib/api", () => ({
  useAppState: () => ({
    currentTenantId: "tenant-1",
    currentTenantSlug: "tenant-1",
    currentUserId: "user-1",
    setCurrentTenant: vi.fn(),
    session: {
      identity: { user_id: "user-1", is_platform_admin: false },
      memberships: [{ tenant_id: "tenant-1", role: "tenant_admin", is_active: true }],
    },
  }),
  useAlerts: () => ({ data: [] }),
  useAlertsTotal: () => ({ data: 0 }),
  useCases: () => ({ data: [] }),
  useCasesTotal: () => ({ data: 0 }),
  useUser: () => ({ data: { id: "user-1", name: "User One", avatar: "", isAdmin: true } }),
  useTenants: () => ({ data: [{ id: "tenant-1", slug: "tenant-1", name: "Tenant 1" }] }),
  useConfig: () => ({ data: {} }),
  useHealth: () => ({ data: { status: "ok", modules: { api: { status: "ok", responseMs: 10 } } } }),
  useSearch: () => ({ data: { alerts: [], cases: [], threads: [] }, isLoading: false }),
  useNotifications: () => ({ data: [] }),
  useMarkNotificationRead: () => ({ mutate: vi.fn() }),
  useMarkAllNotificationsRead: () => ({ mutate: vi.fn() }),
  useDeleteNotification: () => ({ mutate: vi.fn() }),
  useClearNotifications: () => ({ mutate: vi.fn() }),
  useAskAI: () => ({ mutate: vi.fn(), isPending: false }),
  useAISessions: () => ({ data: [], refetch: vi.fn() }),
  useCreateAISession: () => ({ mutate: vi.fn(), isPending: false }),
  useAIMessages: () => ({ data: [], refetch: vi.fn() }),
  useCreateAlert: () => ({ mutate: hoisted.createAlertMutate, isPending: false }),
  useCreateCase: () => ({ mutate: hoisted.createCaseMutate, isPending: false }),
  useExecuteConnectorHub: () => ({ mutate: hoisted.executeConnectorHubMutate, isPending: false }),
  useOutboundConnectors: () => ({ data: [{ id: "connector-1", name: "Smoke Connector", enabled: true }] }),
  useConnectorMethods: (connectorId?: string) => ({
    data: connectorId
      ? [{ id: "method-1", enabled: true, data: { name: "Ping" } }]
      : [],
  }),
  logout: vi.fn(),
}));

import { AppLayout } from "@/components/layout";

describe("Layout quick actions", () => {
  beforeEach(() => {
    hoisted.setLocation.mockReset();
    hoisted.toastSuccess.mockReset();
    hoisted.toastError.mockReset();
    hoisted.createAlertMutate.mockReset();
    hoisted.createCaseMutate.mockReset();
    hoisted.executeConnectorHubMutate.mockReset();

    hoisted.createAlertMutate.mockImplementation((_payload: any, opts?: { onSuccess?: (result: any) => void }) => {
      opts?.onSuccess?.({ id: "alert-123" });
    });
    hoisted.createCaseMutate.mockImplementation((_payload: any, opts?: { onSuccess?: (result: any) => void }) => {
      opts?.onSuccess?.({ id: "case-123" });
    });
    hoisted.executeConnectorHubMutate.mockImplementation((_payload: any, opts?: { onSuccess?: () => void }) => {
      opts?.onSuccess?.();
    });
  });

  it("creates alert from quick action", async () => {
    render(
      <AppLayout>
        <div>child</div>
      </AppLayout>,
    );

    fireEvent.click(screen.getByTestId("quick-action-item-add-alert"));
    fireEvent.change(screen.getByTestId("quick-action-alert-title"), { target: { value: "Suspicious login" } });
    fireEvent.click(screen.getByTestId("quick-action-alert-submit"));

    await waitFor(() => {
      expect(hoisted.createAlertMutate).toHaveBeenCalled();
    });
    expect(hoisted.setLocation).toHaveBeenCalledWith("/tenant-1/alerts/alert-123");
  });

  it("creates case from quick action", async () => {
    render(
      <AppLayout>
        <div>child</div>
      </AppLayout>,
    );

    fireEvent.click(screen.getByTestId("quick-action-item-add-case"));
    fireEvent.change(screen.getByTestId("quick-action-case-title"), { target: { value: "Quick case" } });
    fireEvent.click(screen.getByTestId("quick-action-case-submit"));

    await waitFor(() => {
      expect(hoisted.createCaseMutate).toHaveBeenCalled();
    });
    expect(hoisted.setLocation).toHaveBeenCalledWith("/tenant-1/cases/case-123");
  });

  it("runs connector from quick action", async () => {
    render(
      <AppLayout>
        <div>child</div>
      </AppLayout>,
    );

    fireEvent.click(screen.getByTestId("quick-action-item-run-connector"));
    fireEvent.click(screen.getByText("Smoke Connector"));
    fireEvent.click(screen.getByText("Ping"));
    fireEvent.click(screen.getByTestId("quick-action-connector-submit"));

    await waitFor(() => {
      expect(hoisted.executeConnectorHubMutate).toHaveBeenCalled();
    });
    expect(hoisted.setLocation).toHaveBeenCalledWith("/tenant-1/connectors");
  });

  it("renders redesigned AI chat composer", async () => {
    render(
      <AppLayout>
        <div>child</div>
      </AppLayout>,
    );

    fireEvent.click(screen.getByTestId("ai-toggle"));

    await waitFor(() => {
      expect(screen.getByTestId("ai-chat-input")).toBeInTheDocument();
    });
    expect(screen.getByTestId("ai-chat-panel").className).toContain("h-[min(78dvh,760px)]");
    expect(screen.getByTestId("ai-chat-body").className).toContain("min-h-0");
    expect(screen.getByTestId("ai-chat-messages").className).toContain("overflow-y-auto");
    expect(screen.getByTestId("ai-chat-messages-stack").className).toContain("justify-end");
    expect(screen.getByTestId("ai-send-toggle")).toBeDisabled();
  });

  it("renders service status popover without an inner scroll cap", async () => {
    render(
      <AppLayout>
        <div>child</div>
      </AppLayout>,
    );

    fireEvent.click(screen.getByTestId("sidebar-status-trigger"));

    await waitFor(() => {
      expect(screen.getByTestId("sidebar-status-content")).toBeInTheDocument();
    });

    expect(screen.getByText("api")).toBeInTheDocument();
    expect(screen.getByTestId("sidebar-status-list").className).not.toContain("overflow-y-auto");
    expect(screen.getByTestId("sidebar-status-list").className).not.toContain("max-h-[280px]");
  });

  it("locks browser scrolling and uses the layout scroll region", () => {
    const { unmount } = render(
      <AppLayout>
        <div>child</div>
      </AppLayout>,
    );

    expect(document.documentElement.classList.contains("app-shell-scroll-lock")).toBe(true);
    expect(document.body.classList.contains("app-shell-scroll-lock")).toBe(true);
    expect(screen.getByTestId("app-layout-shell").className).toContain("overflow-hidden");
    expect(screen.getByTestId("layout-scroll-region").className).toContain("overflow-y-auto");

    unmount();

    expect(document.documentElement.classList.contains("app-shell-scroll-lock")).toBe(false);
    expect(document.body.classList.contains("app-shell-scroll-lock")).toBe(false);
  });
});
