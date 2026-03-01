export type APISession = {
  token: string;
  tenantId: string;
  userId: string;
};

export async function apiLogin(baseURL: string, email: string, password: string): Promise<APISession> {
  const resp = await fetch(`${baseURL}/api/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  if (!resp.ok) {
    const text = await resp.text();
    throw new Error(`login failed (${resp.status}): ${text || resp.statusText}`);
  }

  const payload = (await resp.json()) as any;
  const token = String(payload?.access_token || "").trim();
  const tenantId = String(payload?.memberships?.[0]?.tenant_id || "").trim();
  const userId = String(payload?.identity?.user_id || "").trim();
  if (!token || !tenantId || !userId) {
    throw new Error("login response missing token/tenant/user");
  }
  return { token, tenantId, userId };
}

async function apiFetch<T>(
  baseURL: string,
  session: APISession,
  path: string,
  init: RequestInit & { tenantId?: string } = {},
): Promise<T> {
  const tenantId = String(init.tenantId || session.tenantId || "").trim();
  const headers = new Headers(init.headers);
  headers.set("Authorization", `Bearer ${session.token}`);
  if (tenantId) {
    headers.set("X-Tenant-ID", tenantId);
  }
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  const resp = await fetch(`${baseURL}${path}`, {
    ...init,
    headers,
  });

  const text = await resp.text();
  if (!resp.ok) {
    throw new Error(`api ${init.method || "GET"} ${path} failed (${resp.status}): ${text || resp.statusText}`);
  }
  if (!text) {
    return {} as T;
  }
  return JSON.parse(text) as T;
}

export async function apiListTenants(baseURL: string, session: APISession): Promise<any[]> {
  return apiFetch<any[]>(baseURL, session, "/api/v1/tenants", { method: "GET" });
}

export async function apiCreateOutboundConnector(baseURL: string, session: APISession, data: any): Promise<any> {
  return apiFetch<any>(baseURL, session, "/api/v1/catalog/outbound_connectors", {
    method: "POST",
    body: JSON.stringify({ data }),
  });
}
