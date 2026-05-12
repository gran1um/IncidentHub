import { create } from "zustand";
import { persist } from "zustand/middleware";

export type Theme = "light" | "dark";

interface ThemeState {
  theme: Theme;
  setTheme: (theme: Theme) => void;
  toggleTheme: () => void;
}

function getInitialTheme(): Theme {
  if (typeof window === "undefined") {
    return "dark";
  }
  const saved = localStorage.getItem("theme-storage");
  if (saved) {
    try {
      const parsed = JSON.parse(saved);
      const value = parsed?.state?.theme;
      if (value === "light" || value === "dark") {
        return value;
      }
    } catch {
      // ignored
    }
  }
  return "dark";
}

export const useTheme = create<ThemeState>()(
  persist(
    (set, get) => ({
      theme: getInitialTheme(),
      setTheme: (theme) => {
        set({ theme });
        applyTheme(theme);
      },
      toggleTheme: () => {
        const next = get().theme === "light" ? "dark" : "light";
        set({ theme: next });
        applyTheme(next);
      },
    }),
    { name: "theme-storage" }
  )
);

function applyTheme(theme: Theme) {
  if (theme === "dark") {
    document.documentElement.classList.add("dark");
  } else {
    document.documentElement.classList.remove("dark");
  }
}

if (typeof window !== "undefined") {
  applyTheme(getInitialTheme());
}
