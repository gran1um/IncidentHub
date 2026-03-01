import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

const hoisted = vi.hoisted(() => ({
  createThreadMutate: vi.fn(),
  createPostMutate: vi.fn(),
  setLocation: vi.fn(),
  threads: [
    {
      id: "thread-1",
      caseId: "case-1",
      title: "Suspicious outbound traffic",
      status: "In Progress",
      posts: [{ id: "post-1", timestamp: "2026-03-01T10:00:00.000Z" }],
    },
  ],
  cases: [
    {
      id: "case-1",
      sev: "Critical",
      title: "Potential C2 activity",
    },
  ],
}));

vi.mock("@/components/layout", () => ({
  AppLayout: ({ children }: { children: any }) => <div data-testid="layout">{children}</div>,
}));

vi.mock("@/lib/api", () => ({
  useAppState: () => ({ currentTenantId: "tenant-1", currentTenantSlug: "tenant-1" }),
  useForumThreads: () => ({ data: hoisted.threads, isLoading: false }),
  useCases: () => ({ data: hoisted.cases, isLoading: false }),
  useCreateForumThread: () => ({ mutate: hoisted.createThreadMutate, isPending: false }),
  useCreateForumPost: () => ({ mutate: hoisted.createPostMutate, isPending: false }),
  useUser: () => ({ data: { id: "user-1", name: "Current User" } }),
}));

vi.mock("wouter", () => ({
  Link: ({ href, children }: { href: string; children: any }) => <a href={href}>{children}</a>,
  useLocation: () => ["/forum", hoisted.setLocation],
}));

import ForumPage from "@/pages/forum";

describe("ForumPage", () => {
  it("renders security forum list and cards", () => {
    render(<ForumPage />);

    expect(screen.getByText("Security Forum")).toBeInTheDocument();
    expect(screen.getByText("Suspicious outbound traffic")).toBeInTheDocument();
    expect(screen.getByText("Potential C2 activity")).toBeInTheDocument();
    expect(screen.getByText("Open Thread")).toBeInTheDocument();
  });
});
