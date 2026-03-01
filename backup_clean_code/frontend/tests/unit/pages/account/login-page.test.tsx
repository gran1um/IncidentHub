import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

const hoisted = vi.hoisted(() => ({
  loginMock: vi.fn(),
  setLocationMock: vi.fn(),
  setThemeMock: vi.fn(),
  setLanguageMock: vi.fn(),
  useAppStateMock: Object.assign(
    (selector?: (state: { currentTenantSlug: string }) => string) => {
      const state = { currentTenantSlug: "tenant-alpha" };
      return selector ? selector(state) : state;
    },
    {
      getState: () => ({ currentTenantSlug: "tenant-alpha" }),
    },
  ),
}));

vi.mock("wouter", () => ({
  useLocation: () => ["/login", hoisted.setLocationMock],
}));

vi.mock("@/lib/theme", () => ({
  useTheme: () => ({ theme: "dark", setTheme: hoisted.setThemeMock }),
}));

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({ language: "en", setLanguage: hoisted.setLanguageMock }),
  useT: () => (key: string) => key,
}));

vi.mock("@/lib/api", () => ({
  login: hoisted.loginMock,
  useAppState: hoisted.useAppStateMock,
}));

import LoginPage from "@/pages/login";

describe("LoginPage", () => {
  it("submits credentials and redirects to tenant dashboard", async () => {
    const user = userEvent.setup();
    hoisted.loginMock.mockReset();
    hoisted.setLocationMock.mockReset();
    hoisted.loginMock.mockResolvedValue(undefined);

    render(<LoginPage />);

    await user.type(screen.getByTestId("login-email"), "admin@example.com");
    await user.type(screen.getByTestId("login-password"), "secret");
    await user.click(screen.getByTestId("login-submit"));

    await waitFor(() => {
      expect(hoisted.loginMock).toHaveBeenCalledWith("admin@example.com", "secret");
    });
    expect(hoisted.setLocationMock).toHaveBeenCalledWith("/tenant-alpha/dashboard", { replace: true });
  });

  it("shows backend error when login fails", async () => {
    const user = userEvent.setup();
    hoisted.loginMock.mockReset();
    hoisted.setLocationMock.mockReset();
    hoisted.loginMock.mockRejectedValue(new Error("invalid credentials"));

    render(<LoginPage />);

    await user.type(screen.getByTestId("login-email"), "admin@example.com");
    await user.type(screen.getByTestId("login-password"), "bad-secret");
    await user.click(screen.getByTestId("login-submit"));

    await screen.findByText("invalid credentials");
    expect(hoisted.setLocationMock).not.toHaveBeenCalled();
  });
});
