import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

  const hoisted = vi.hoisted(() => ({
    createUserMutate: vi.fn(),
    createNotificationBotMutate: vi.fn(),
    saveNotificationSettingMutate: vi.fn(),
    createServiceAlertRuleMutate: vi.fn(),
    updateServiceAlertRuleMutate: vi.fn(),
    createConnectorMutate: vi.fn(),
    executeConnectorHubMutate: vi.fn(),
    upsertSOCAccessPolicyMutate: vi.fn(),
    factoryResetLocalMutate: vi.fn(),
    setLocation: vi.fn(),
  location: "/administration",
  serviceAlertRules: [] as any[],
  systemResources: {
    env: "prod",
    timestamp: "2026-02-18T10:00:00.000Z",
    modules: {
      api: { status: "ok", responseMs: 12, operation: { avgLatencyMs: 118.6, operations: 3, errors: 1, windowSeconds: 300 } },
      postgres: { status: "ok", responseMs: 7 },
      redis: { status: "ok", responseMs: 5 },
      elasticsearch: { status: "disabled", responseMs: 0 },
      s3: { status: "disabled", responseMs: 0 },
      ai_model: { status: "ok", responseMs: 1, operation: { avgLatencyMs: 142.3, operations: 2, errors: 0, windowSeconds: 300 } },
    },
    tenant: { apiRequests24h: 10, activeUsers: 2, openCases: 1, observables: 4 },
    host: {
      cpuPercent: 10,
      cpuCoresUsed: 0.8,
      cpuCoresTotal: 8,
      memoryUsedBytes: 8 * 1024 * 1024 * 1024,
      memoryTotalBytes: 32 * 1024 * 1024 * 1024,
      memoryUsedMB: 8192,
      memoryTotalMB: 32768,
      memoryUsedGB: 8,
      memoryTotalGB: 32,
      memoryUsedPercent: 25,
      diskUsedBytes: 10 * 1024 * 1024 * 1024,
      diskTotalBytes: 100 * 1024 * 1024 * 1024,
      diskUsedPercent: 12,
      diskUsedGB: 10,
      diskTotalGB: 100,
    },
    postgresPool: {},
  },
}));

vi.mock("@/components/layout", () => ({
  AppLayout: ({ children }: { children: any }) => <div data-testid="layout">{children}</div>,
}));
vi.mock("@/components/connector-execution-drawer", () => ({
  ConnectorExecutionDrawer: () => null,
}));

vi.mock("wouter", () => ({
  useLocation: () => [hoisted.location, hoisted.setLocation],
}));

vi.mock("@/components/admin/case-statuses-panel", () => ({
  CaseStatusesPanel: () => <div data-testid="case-statuses-panel" />,
}));

vi.mock("recharts", () => ({
  ResponsiveContainer: ({ children }: { children: any }) => <div>{children}</div>,
  AreaChart: ({ children }: { children: any }) => <div>{children}</div>,
  Area: () => null,
  LineChart: ({ children }: { children: any }) => <div>{children}</div>,
  Line: () => null,
  XAxis: () => null,
  YAxis: () => null,
  Tooltip: () => null,
  CartesianGrid: () => null,
  BarChart: ({ children }: { children: any }) => <div>{children}</div>,
  Bar: () => null,
}));

vi.mock("@/lib/api", () => ({
  useAppState: () => ({ currentTenantId: "tenant-1", currentUserId: "user-1", session: { identity: { is_platform_admin: true } } }),
  useUser: () => ({ data: { id: "user-1", name: "Alice" } }),
  useUsers: () => ({
    data: [
      { id: "user-1", name: "Alice", email: "alice@example.com", role: "Admin", team: "SOC", isAdmin: true },
      { id: "user-2", name: "Bob", email: "bob@example.com", role: "Analyst", team: "SOC", isAdmin: false },
    ],
    isLoading: false,
  }),
  useTenants: () => ({
    data: [
      {
        id: "tenant-1",
        slug: "tenant-1",
        name: "Tenant One",
        description: "Main tenant",
        maxUsers: 50,
        active: true,
        responsibleUserId: "user-1",
      },
      {
        id: "tenant-2",
        slug: "tenant-2",
        name: "Tenant Two",
        description: "Secondary tenant",
        maxUsers: 25,
        active: true,
        responsibleUserId: "user-2",
      },
    ],
    isLoading: false,
  }),
  useCreateTenant: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateTenant: () => ({ mutate: vi.fn(), isPending: false }),
  useCreateUser: () => ({ mutate: hoisted.createUserMutate, isPending: false }),
  useUpdateUser: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteUser: () => ({ mutate: vi.fn(), isPending: false }),
  useRateLimits: () => ({ data: [], isLoading: false }),
  useCreateRateLimit: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateRateLimit: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteRateLimit: () => ({ mutate: vi.fn(), isPending: false }),
  useSOCAccessPolicy: () => ({ data: { id: "policy-1", allowedCaseTags: [], maxCasesInWork: 0 }, isLoading: false }),
  useUpsertSOCAccessPolicy: () => ({ mutate: hoisted.upsertSOCAccessPolicyMutate, isPending: false }),
  useAchievements: () => ({ data: [], isLoading: false }),
  useCreateAchievement: () => ({ mutate: vi.fn(), isPending: false }),
  useUploadAchievementIcon: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteAchievement: () => ({ mutate: vi.fn(), isPending: false }),
  useUserAchievements: () => ({ data: [] }),
  useGrantAchievement: () => ({ mutate: vi.fn(), isPending: false }),
  useRevokeAchievement: () => ({ mutate: vi.fn(), isPending: false }),
  useInfiniteUserExperienceEvents: () => ({ data: { pages: [{ items: [] }] }, isLoading: false, isFetchingNextPage: false, hasNextPage: false, fetchNextPage: vi.fn() }),
  useAwardUserExperience: () => ({ mutate: vi.fn(), isPending: false }),
  useAdminApiTokens: () => ({ data: [], isLoading: false }),
  useCreateAdminApiToken: () => ({ mutate: vi.fn(), isPending: false }),
  useRevokeAdminApiToken: () => ({ mutate: vi.fn(), isPending: false }),
  useAdminNotificationBots: () => ({ data: [], isLoading: false }),
  useCreateAdminNotificationBot: () => ({ mutate: hoisted.createNotificationBotMutate, isPending: false }),
  useUpdateAdminNotificationBot: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteAdminNotificationBot: () => ({ mutate: vi.fn(), isPending: false }),
  useAdminNotificationSettings: () => ({
    data: [
      {
        userId: "user-1",
        userName: "Alice",
        userEmail: "alice@example.com",
        userRole: "tenant_admin",
        deliveryEnabled: false,
        deliveryChannel: "in_app",
        telegramBotId: "",
        telegramChatId: "",
        telegramUsername: "",
      },
      {
        userId: "user-2",
        userName: "Bob",
        userEmail: "bob@example.com",
        userRole: "analyst",
        deliveryEnabled: false,
        deliveryChannel: "in_app",
        telegramBotId: "",
        telegramChatId: "",
        telegramUsername: "",
      },
    ],
    isLoading: false,
  }),
  useSaveAdminNotificationSetting: () => ({ mutate: hoisted.saveNotificationSettingMutate, isPending: false }),
  useServiceAlertRules: () => ({ data: hoisted.serviceAlertRules, isLoading: false }),
  useCreateServiceAlertRule: () => ({ mutate: hoisted.createServiceAlertRuleMutate, isPending: false }),
  useUpdateServiceAlertRule: () => ({ mutate: hoisted.updateServiceAlertRuleMutate, isPending: false }),
  useDeleteServiceAlertRule: () => ({ mutate: vi.fn(), isPending: false }),
  useOutboundConnectors: () => ({ data: [], isLoading: false }),
  useTenantConnectorMethods: () => ({ data: [], isLoading: false }),
  useCreateConnector: () => ({ mutate: hoisted.createConnectorMutate, isPending: false }),
  useUpdateConnector: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteConnector: () => ({ mutate: vi.fn(), isPending: false }),
  useConnectorMethods: () => ({ data: [] }),
  useCreateConnectorMethod: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteConnectorMethod: () => ({ mutate: vi.fn(), isPending: false }),
  useConnectorHubExecutions: () => ({ data: [] }),
  useExecuteConnectorHub: () => ({ mutate: hoisted.executeConnectorHubMutate, isPending: false }),
  useAutomations: () => ({ data: [] }),
  useCreateAutomation: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteAutomation: () => ({ mutate: vi.fn(), isPending: false }),
  useSystemResources: () => ({
    data: hoisted.systemResources,
  }),
  useFactoryResetLocal: () => ({ mutate: hoisted.factoryResetLocalMutate, isPending: false }),
}));

import AdministrationPage from "@/pages/administration";

async function chooseSelectOption(user: ReturnType<typeof userEvent.setup>, triggerTestId: string, optionLabel: string) {
  await user.click(screen.getByTestId(triggerTestId));
  await user.click(await screen.findByRole("option", { name: optionLabel }));
}

describe("AdministrationPage", () => {
  it("passes entered password when creating a user", async () => {
    const user = userEvent.setup();
    hoisted.createUserMutate.mockReset();
    hoisted.serviceAlertRules = [];
    hoisted.updateServiceAlertRuleMutate.mockReset();

    render(<AdministrationPage initialTab="users" />);

    await user.click(screen.getByTestId("button-new-user"));
    await screen.findByTestId("input-user-name");
    await user.type(screen.getByTestId("input-user-name"), "Charlie");
    await user.type(screen.getByTestId("input-user-email"), "charlie@example.com");
    await user.type(screen.getByTestId("input-user-password"), "TempPass123!");
    await user.click(screen.getByTestId("button-create-user"));

    expect(hoisted.createUserMutate).toHaveBeenCalledTimes(1);
    const payload = hoisted.createUserMutate.mock.calls[0][0];
    expect(payload.name).toBe("Charlie");
    expect(payload.email).toBe("charlie@example.com");
    expect(payload.password).toBe("TempPass123!");
  });

  it("creates notification bot for selected tenant", async () => {
    const user = userEvent.setup();
    hoisted.createNotificationBotMutate.mockReset();
    hoisted.saveNotificationSettingMutate.mockReset();
    hoisted.serviceAlertRules = [];
    hoisted.updateServiceAlertRuleMutate.mockReset();

    render(<AdministrationPage initialTab="notification_bots" />);

    await chooseSelectOption(user, "select-notification-bot-tenant", "tenant-2");
    await user.click(screen.getByTestId("button-new-notification-bot"));
    await screen.findByTestId("select-notification-bot-tenant-dialog");
    await chooseSelectOption(user, "select-notification-bot-tenant-dialog", "tenant-2");
    await user.type(screen.getByTestId("input-notification-bot-name"), "Tenant 2 Bot");
    await user.type(screen.getByTestId("input-notification-bot-token"), "123456:tenant2");
    await user.click(screen.getByTestId("button-create-notification-bot"));

    expect(hoisted.createNotificationBotMutate).toHaveBeenCalled();
    const payload = hoisted.createNotificationBotMutate.mock.calls[0][0];
    expect(payload.tenantId).toBe("tenant-2");
    expect(payload.name).toBe("Tenant 2 Bot");
  });

  it("creates service alert rule for selected tenant", async () => {
    const user = userEvent.setup();
    hoisted.createServiceAlertRuleMutate.mockReset();
    hoisted.updateServiceAlertRuleMutate.mockReset();
    hoisted.serviceAlertRules = [];

    render(<AdministrationPage initialTab="service_alerts" />);

    await chooseSelectOption(user, "select-service-alert-tenant", "tenant-2");
    await user.click(screen.getByTestId("button-new-service-alert-rule"));
    await screen.findByTestId("select-service-alert-tenant-dialog");
    await chooseSelectOption(user, "select-service-alert-tenant-dialog", "tenant-2");
    await user.clear(screen.getByTestId("input-service-alert-name"));
    await user.type(screen.getByTestId("input-service-alert-name"), "API low RPS");
    await user.click(screen.getByTestId("button-create-service-alert"));

    expect(hoisted.createServiceAlertRuleMutate).toHaveBeenCalled();
    const payload = hoisted.createServiceAlertRuleMutate.mock.calls[0][0];
    expect(payload.tenantId).toBe("tenant-2");
    expect(payload.name).toBe("API low RPS");
  });

  it("saves tenant user notification settings for selected tenant", async () => {
    const user = userEvent.setup();
    hoisted.saveNotificationSettingMutate.mockReset();
    hoisted.serviceAlertRules = [];
    hoisted.updateServiceAlertRuleMutate.mockReset();

    render(<AdministrationPage initialTab="notification_bots" />);

    await user.click(screen.getByTestId("tab-notification-bots"));
    await chooseSelectOption(user, "select-notification-bot-tenant", "tenant-2");
    await chooseSelectOption(user, "select-notification-setting-channel-user-2", "Telegram");
    await user.type(screen.getByTestId("input-notification-setting-chat-user-2"), "777100200");
    await user.click(screen.getByTestId("button-save-notification-setting-user-2"));

    expect(hoisted.saveNotificationSettingMutate).toHaveBeenCalled();
    const payload = hoisted.saveNotificationSettingMutate.mock.calls[0][0];
    expect(payload.userId).toBe("user-2");
    expect(payload.data.tenantId).toBe("tenant-2");
    expect(payload.data.deliveryChannel).toBe("telegram");
  });

  it("renders without runtime error and shows tenant actions", async () => {
    const user = userEvent.setup();
    hoisted.serviceAlertRules = [];
    hoisted.updateServiceAlertRuleMutate.mockReset();
    render(<AdministrationPage />);

    expect(screen.getByTestId("text-page-title")).toHaveTextContent("Administration");
    await user.click(screen.getByTestId("tab-tenants"));
    expect(await screen.findByTestId("button-edit-tenant-tenant-1")).toBeInTheDocument();
  });

  it("persists tenant user delivery toggle immediately", async () => {
    const user = userEvent.setup();
    hoisted.saveNotificationSettingMutate.mockReset();
    hoisted.serviceAlertRules = [];
    hoisted.updateServiceAlertRuleMutate.mockReset();

    render(<AdministrationPage initialTab="notification_bots" />);

    await user.click(screen.getByTestId("switch-notification-setting-enabled-user-2"));

    expect(hoisted.saveNotificationSettingMutate).toHaveBeenCalledTimes(1);
    const payload = hoisted.saveNotificationSettingMutate.mock.calls[0][0];
    expect(payload.userId).toBe("user-2");
    expect(payload.data.tenantId).toBe("tenant-1");
    expect(payload.data.deliveryEnabled).toBe(true);
  });

  it("persists service alert enabled toggle immediately", async () => {
    const user = userEvent.setup();
    hoisted.updateServiceAlertRuleMutate.mockReset();
    hoisted.serviceAlertRules = [
      {
        id: "rule-1",
        tenantId: "tenant-1",
        name: "API low RPS",
        description: "",
        metricType: "low_rps",
        module: "api",
        minRps: 1,
        maxLatencyMs: 300,
        windowSeconds: 300,
        cooldownSeconds: 900,
        severity: "warning",
        enabled: true,
      },
    ];

    render(<AdministrationPage initialTab="service_alerts" />);

    await user.click(screen.getByTestId("switch-service-alert-enabled-rule-1"));

    expect(hoisted.updateServiceAlertRuleMutate).toHaveBeenCalledTimes(1);
    const payload = hoisted.updateServiceAlertRuleMutate.mock.calls[0][0];
    expect(payload.id).toBe("rule-1");
    expect(payload.data.enabled).toBe(false);
    expect(payload.data.tenantId).toBe("tenant-1");
  });

  it("filters tenant users from the live search bar", async () => {
    const user = userEvent.setup();
    hoisted.serviceAlertRules = [];
    hoisted.updateServiceAlertRuleMutate.mockReset();

    render(<AdministrationPage initialTab="users" />);

    await user.type(screen.getByTestId("input-user-search"), "bob");

    expect(screen.getByTestId("card-user-user-2")).toBeInTheDocument();
    expect(screen.queryByTestId("card-user-user-1")).not.toBeInTheDocument();
  });

  it("filters tenants from the tenant directory search bar", async () => {
    const user = userEvent.setup();
    hoisted.serviceAlertRules = [];
    hoisted.updateServiceAlertRuleMutate.mockReset();

    render(<AdministrationPage initialTab="tenants" />);

    await user.type(screen.getByTestId("input-tenant-search"), "secondary");

    expect(screen.getByTestId("card-tenant-tenant-2")).toBeInTheDocument();
    expect(screen.queryByTestId("card-tenant-tenant-1")).not.toBeInTheDocument();
  });

  it("hides factory reset controls in prod", () => {
    hoisted.serviceAlertRules = [];
    hoisted.updateServiceAlertRuleMutate.mockReset();
    hoisted.systemResources = {
      ...hoisted.systemResources,
      env: "prod",
    };

    render(<AdministrationPage />);

    expect(screen.queryByTestId("button-open-factory-reset-local")).not.toBeInTheDocument();
  });

  it("does not render the legacy connectors tab in administration", () => {
    hoisted.serviceAlertRules = [];
    hoisted.updateServiceAlertRuleMutate.mockReset();

    render(<AdministrationPage />);

    expect(screen.queryByTestId("tab-connectors")).not.toBeInTheDocument();
    expect(screen.getByTestId("tab-users")).toBeInTheDocument();
    expect(screen.getByTestId("text-page-title")).toHaveTextContent("Administration");
  });
});
