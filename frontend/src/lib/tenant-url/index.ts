const TENANT_UUID_PREFIX_RE = /^\/([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12})(?=\/|$)/;

function normalizePath(path: string): string {
  const trimmed = String(path || "").trim();
  if (!trimmed) {
    return "/";
  }
  return trimmed.startsWith("/") ? trimmed : `/${trimmed}`;
}

function normalizeSlug(value: string): string {
  return String(value || "").trim().toLowerCase();
}

export function extractLeadingPathSegment(path: string): string {
  const normalized = normalizePath(path);
  if (normalized === "/") {
    return "";
  }
  const segments = normalized.split("/").filter(Boolean);
  return segments[0] || "";
}

export function stripTenantPrefix(path: string, knownTenantSlugs?: Iterable<string>): string {
  const normalized = normalizePath(path);
  if (normalized === "/") {
    return "/";
  }

  if (TENANT_UUID_PREFIX_RE.test(normalized)) {
    const next = normalized.replace(TENANT_UUID_PREFIX_RE, "");
    return next && next !== "/" ? (next.startsWith("/") ? next : `/${next}`) : "/";
  }

  const firstSegment = extractLeadingPathSegment(normalized);
  if (!firstSegment) {
    return "/";
  }

  const known = new Set(Array.from(knownTenantSlugs || []).map(normalizeSlug).filter(Boolean));
  if (known.size > 0 && known.has(normalizeSlug(firstSegment))) {
    const next = normalized.replace(new RegExp(`^/${firstSegment}(?=/|$)`), "");
    return next && next !== "/" ? (next.startsWith("/") ? next : `/${next}`) : "/";
  }

  return normalized;
}

export function withTenantPath(tenantSlug: string, path: string): string {
  const normalizedTenantSlug = normalizeSlug(tenantSlug);
  const normalizedPath = normalizePath(path);
  if (!normalizedTenantSlug) {
    return normalizedPath;
  }
  if (normalizedPath === `/${normalizedTenantSlug}` || normalizedPath.startsWith(`/${normalizedTenantSlug}/`)) {
    return normalizedPath;
  }
  const withoutTenant = stripTenantPrefix(normalizedPath, [normalizedTenantSlug]);
  if (withoutTenant === "/") {
    return `/${normalizedTenantSlug}`;
  }
  return `/${normalizedTenantSlug}${withoutTenant}`;
}

export function withTenantLocation(
  tenantSlug: string,
  location: string,
  knownTenantSlugs?: Iterable<string>,
): string {
  const raw = String(location || "").trim();
  const queryIndex = raw.indexOf("?");
  const path = queryIndex >= 0 ? raw.slice(0, queryIndex) : raw;
  const search = queryIndex >= 0 ? raw.slice(queryIndex) : "";
  const basePath = stripTenantPrefix(path, knownTenantSlugs);
  return `${withTenantPath(tenantSlug, basePath)}${search}`;
}

