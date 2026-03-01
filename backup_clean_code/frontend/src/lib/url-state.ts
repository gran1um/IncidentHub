export function splitLocationPathAndSearch(location: string): { path: string; params: URLSearchParams } {
  const raw = String(location || "");
  const index = raw.indexOf("?");
  if (index < 0) {
    const path = raw || "/";
    if (typeof window !== "undefined") {
      const browserPath = String(window.location.pathname || "");
      const browserSearch = String(window.location.search || "");
      if (browserPath && browserPath === path && browserSearch.startsWith("?")) {
        return { path, params: new URLSearchParams(browserSearch.slice(1)) };
      }
    }
    return { path, params: new URLSearchParams() };
  }
  const path = raw.slice(0, index) || "/";
  const search = raw.slice(index + 1);
  return { path, params: new URLSearchParams(search) };
}

export function applySearchPatch(
  location: string,
  patch: Record<string, string | undefined | null>,
): string {
  const { path, params } = splitLocationPathAndSearch(location);
  Object.entries(patch).forEach(([key, value]) => {
    if (value === undefined || value === null || value === "") {
      params.delete(key);
      return;
    }
    params.set(key, value);
  });
  const search = params.toString();
  return search ? `${path}?${search}` : path;
}

export function parseDateParam(value: string | null): Date | undefined {
  if (!value) return undefined;
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return undefined;
  }
  return parsed;
}

export function formatDateParam(value?: Date): string | undefined {
  if (!value || Number.isNaN(value.getTime())) {
    return undefined;
  }
  return value.toISOString();
}

export function parsePositiveIntParam(value: string | null, fallback: number): number {
  const parsed = Number.parseInt(String(value || "").trim(), 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return fallback;
  }
  return parsed;
}
