import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const hoisted = vi.hoisted(() => ({
  users: [] as any[],
  cases: [] as any[],
  shifts: [] as any[],
  stats: {
    activeCases: 0,
    alertsCount: 0,
    avgResponseMin: 0,
    resolvedToday: 0,
  },
  dutyOverview: {
    onDuty: [] as any[],
    current: { count: 0, shifts: [], analysts: [] },
    next: { count: 0, startsAt: "", shifts: [], analysts: [] },
  },
  dashboardMetrics: {
    openCases: 0,
    openAlerts: 0,
    overdueCases: 0,
    overdueThresholdMinutes: 1440,
    slaBySeverity: [] as any[],
    resolvedByAnalyst: [] as any[],
    casesByStatus: [] as any[],
    casesByCategory: [] as any[],
    alertsByStatus: [] as any[],
    alertsByCategory: [] as any[],
    customMetrics: [] as any[],
  },
  customMetrics: [] as any[],
}));

vi.mock("@/components/layout", () => ({
  AppLayout: ({ children }: { children: any }) => <div data-testid="layout">{children}</div>,
}));

vi.mock("wouter", () => ({
  Link: ({ children, href }: { children: any; href: string }) => <a href={href}>{children}</a>,
}));

vi.mock("recharts", () => {
  const Mock = ({ children }: { children?: any }) => <div>{children}</div>;
  return {
    ResponsiveContainer: Mock,
    BarChart: Mock,
    Bar: () => null,
    XAxis: () => null,
    YAxis: () => null,
    Tooltip: () => null,
    CartesianGrid: () => null,
    Legend: () => null,
  };
});

vi.mock("@/lib/api", () => ({
  useAppState: () => ({ currentTenantId: "tenant-1", currentTenantSlug: "tenant-1" }),
  useUsers: () => ({ data: hoisted.users, isLoading: false }),
  useCases: () => ({ data: hoisted.cases }),
  useShifts: () => ({ data: hoisted.shifts }),
  useDashboardStats: () => ({ data: hoisted.stats, isLoading: false }),
  useDashboardMetrics: () => ({ data: hoisted.dashboardMetrics, isLoading: false }),
  useDashboardCustomMetrics: () => ({ data: hoisted.customMetrics, isLoading: false }),
  useCreateDashboardCustomMetric: () => ({ mutate: vi.fn() }),
  useUpdateDashboardCustomMetric: () => ({ mutate: vi.fn() }),
  useDeleteDashboardCustomMetric: () => ({ mutate: vi.fn() }),
  useDutyOverview: () => ({ data: hoisted.dutyOverview, isLoading: false }),
  useActivityLivestream: () => ({ data: { items: [] }, isLoading: false }),
  useCreateShift: () => ({ mutate: vi.fn() }),
  useDeleteShift: () => ({ mutate: vi.fn() }),
}));

import Dashboard from "@/pages/dashboard";

describe("Dashboard duty overview", () => {
  beforeEach(() => {
    window.localStorage.clear();
    hoisted.users = [
      { id: "user-1", name: "Alice Analyst", role: "Analyst", avatar: "" },
    ];
    hoisted.cases = [];
    hoisted.shifts = [];
    hoisted.stats = {
      activeCases: 4,
      alertsCount: 18,
      avgResponseMin: 14,
      resolvedToday: 3,
    };
    hoisted.dutyOverview = {
      onDuty: [
        { id: "user-1", name: "Alice Analyst", role: "Analyst", avatar: "" },
      ],
      current: { count: 2, shifts: [], analysts: [] },
      next: { count: 1, startsAt: "2026-02-27T18:00:00Z", shifts: [], analysts: [] },
    };
    hoisted.dashboardMetrics = {
      openCases: 4,
      openAlerts: 3,
      overdueCases: 1,
      overdueThresholdMinutes: 1440,
      slaBySeverity: [{ severity: "high", openCases: 1, resolvedCases: 2, breachedCases: 0, avgResolutionMinutes: 30, targetMinutes: 240 }],
      resolvedByAnalyst: [{ userId: "user-1", username: "alice", displayName: "Alice Analyst", resolvedCases: 2 }],
      casesByStatus: [{ key: "open", count: 4 }],
      casesByCategory: [{ key: "phishing", count: 2 }],
      alertsByStatus: [{ key: "new", count: 3 }],
      alertsByCategory: [{ key: "edr", count: 3 }],
      customMetrics: [{ id: "m1", name: "Open critical", source: "cases", measure: "count", enabled: true, value: 1, filters: {} }],
    };
    hoisted.customMetrics = hoisted.dashboardMetrics.customMetrics;
  });

  it("renders current and next duty summary from API", () => {
    render(<Dashboard />);

    expect(screen.getByTestId("duty-overview-current").textContent).toContain("2");
    expect(screen.getByTestId("duty-overview-next").textContent).toContain("Next shift:");
    expect(screen.getByTestId("duty-overview-next-count").textContent).toContain("1");
    expect(screen.getByTestId("dashboard-livestream-widget")).toBeInTheDocument();
    expect(screen.getByTestId("dashboard-metrics-widget")).toBeInTheDocument();
  });

  it("renders no-upcoming-shift text when next shift is absent", () => {
    hoisted.dutyOverview = {
      onDuty: [],
      current: { count: 0, shifts: [], analysts: [] },
      next: { count: 0, startsAt: "", shifts: [], analysts: [] },
    };
    hoisted.users = [];

    render(<Dashboard />);

    expect(screen.getByText("No active analyst shift right now.")).toBeInTheDocument();
    expect(screen.getByTestId("duty-overview-next").textContent).toContain("No upcoming shifts");
  });

  it("opens schedule and customize dialogs from dashboard actions", () => {
    render(<Dashboard />);

    fireEvent.click(screen.getByTestId("button-duty-schedule-open"));
    expect(screen.getByText("Shift Management")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("button-dashboard-customize-open"));
    expect(screen.getByText("New metric")).toBeInTheDocument();
  });

  it("persists stream pause toggle per tenant", () => {
    const setItemSpy = vi.spyOn(window.localStorage.__proto__, "setItem");
    render(<Dashboard />);

    fireEvent.click(screen.getByTestId("button-dashboard-stream-pause"));

    expect(setItemSpy).toHaveBeenCalledWith("incidenthub-dashboard-stream-paused:tenant-1", "1");
    setItemSpy.mockRestore();
  });

  it("renders overnight continuation on next day in schedule grid", () => {
    hoisted.shifts = [
      {
        id: "shift-1",
        data: {
          analystId: "user-1",
          day: 2,
          start: "22:00",
          end: "06:00",
        },
      },
    ];

    render(<Dashboard />);
    fireEvent.click(screen.getByTestId("button-duty-schedule-open"));

    expect(screen.getByTestId("schedule-day-continuation-3-shift-1")).toBeInTheDocument();
  });
});
