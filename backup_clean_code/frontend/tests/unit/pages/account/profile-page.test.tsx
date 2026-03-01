import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

const hoisted = vi.hoisted(() => ({
  updateUserMutate: vi.fn(),
  currentUser: {
    id: "user-1",
    tenantId: "tenant-1",
    name: "Alex Popov",
    role: "analyst",
    team: "SOC",
    email: "alex@example.com",
    personalLink: "example.com/me",
    isAdmin: true,
    experiencePoints: 1200,
    createdAt: "2026-03-01T10:00:00Z",
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
  useAppState: () => ({ currentUserId: "user-1" }),
  useUser: () => ({
    data: hoisted.currentUser,
    isLoading: false,
  }),
  useUserAchievements: () => ({ data: [] }),
  useUserProfileBio: () => ({ data: "", isLoading: false }),
  useSaveUserProfileBio: () => ({ mutate: vi.fn(), isPending: false }),
  useUserSpecializationBadges: () => ({ data: [], isLoading: false }),
  useSaveUserSpecializationBadges: () => ({ mutate: vi.fn(), isPending: false }),
  useTenants: () => ({ data: [{ id: "tenant-1", slug: "tenant-alpha" }] }),
  useUpdateUser: () => ({ mutate: hoisted.updateUserMutate, isPending: false }),
  useUploadUserMedia: () => ({ mutate: vi.fn(), isPending: false }),
  useInfiniteUserExperienceEvents: () => ({
    data: { pages: [{ items: [] }] },
    isLoading: false,
    isFetchingNextPage: false,
    hasNextPage: false,
    fetchNextPage: vi.fn(),
  }),
  useUserCasePerformance: () => ({
    data: {
      closedCasesTotal: 12,
      currentMonthClosedCases: 4,
      previousMonthClosedCases: 2,
      avgInvestigationMinutes: 11.2,
      currentMonthAvgInvestigationMinutes: 10.4,
      previousMonthAvgInvestigationMinutes: 12.7,
    },
    isLoading: false,
  }),
}));

import ProfilePage from "@/pages/profile";

describe("ProfilePage", () => {
  beforeAll(() => {
    vi.stubGlobal(
      "IntersectionObserver",
      class {
        observe() {}
        disconnect() {}
      },
    );
  });

  beforeEach(() => {
    hoisted.updateUserMutate.mockReset();
    hoisted.updateUserMutate.mockImplementation((_payload: any, options?: { onSuccess?: () => void }) => {
      options?.onSuccess?.();
    });
  });

  it("saves profile fields through header and block edit controls", async () => {
    render(<ProfilePage />);

    fireEvent.mouseOver(screen.getByTestId("text-username"));
    fireEvent.click(screen.getByTestId("button-inline-edit-name"));
    fireEvent.change(screen.getByTestId("input-inline-name"), { target: { value: "  New Name  " } });
    fireEvent.click(screen.getByTestId("button-inline-save-name"));

    await waitFor(() => {
      expect(hoisted.updateUserMutate).toHaveBeenCalledTimes(1);
    });

    fireEvent.click(screen.getByTestId("button-profile-info-edit"));
    fireEvent.change(screen.getByTestId("input-profile-info-email"), { target: { value: "  new@example.com  " } });
    fireEvent.click(screen.getByTestId("button-profile-info-save"));

    await waitFor(() => {
      expect(hoisted.updateUserMutate).toHaveBeenCalledTimes(2);
    });

    expect(hoisted.updateUserMutate.mock.calls[0][0]).toEqual({
      id: "user-1",
      data: {
        name: "New Name",
      },
    });
    expect(hoisted.updateUserMutate.mock.calls[1][0]).toEqual({
      id: "user-1",
      data: {
        email: "new@example.com",
        team: "SOC",
        personalLink: "example.com/me",
      },
    });
  });
});
