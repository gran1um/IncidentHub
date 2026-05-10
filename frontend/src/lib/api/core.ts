import { keepPreviousData, useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { create } from "zustand";
import { normalizeConnectorDirection } from "@/lib/connectors";
import { useAsyncOpsTracker } from "@/lib/async-ops-tracker";

const API_URL = (import.meta.env.VITE_API_URL as string | undefined) ?? "";

interface Identity {
  user_id: string;
  username: string;
  is_platform_admin: boolean;
  tenant_id?: string;
  tenant_role?: string;
}

interface Membership {
  id: string;
  tenant_id: string;
  user_id: string;
  role: string;
  is_active: boolean;
}

interface Session {
  accessToken: string;
  identity: Identity;
  memberships: Membership[];
  selectedTenantId?: string;
}

interface AppState {
  session: Session | null;
  currentTenantId: string;
  currentTenantSlug: string;
  currentUserId: string;
  setCurrentTenant: (id: string) => void;
  setCurrentTenantSlug: (slug: string) => void;
  setSession: (session: Session) => void;
  clearSession: () => void;
}

const PATH_TENANT_UUID_RE = /^\/([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12})(?=\/|$)/;

function readTenantIDFromBrowserURL(): string {
  if (typeof window === "undefined") {
    return "";
  }
  try {
    const match = String(window.location.pathname || "").match(PATH_TENANT_UUID_RE);
    return match?.[1] || "";
  } catch {
    return "";
  }
}

type SessionLike = {
  identity?: Identity;
  memberships?: Membership[];
  selectedTenantId?: string;
};

function listActiveMembershipTenantIDs(sessionLike?: SessionLike | null): string[] {
  const memberships = Array.isArray(sessionLike?.memberships) ? sessionLike?.memberships : [];
  return memberships
    .filter((membership) => membership?.is_active && membership?.tenant_id)
    .map((membership) => String(membership.tenant_id).trim())
    .filter(Boolean);
}

function resolvePreferredTenantID(sessionLike?: SessionLike | null, preferredTenantID?: string): string {
  const activeTenantIDs = listActiveMembershipTenantIDs(sessionLike);
  const fallbackTenantID =
    String(sessionLike?.identity?.tenant_id || "").trim() ||
    activeTenantIDs[0] ||
    "";
  const requestedTenantID = String(preferredTenantID || "").trim();
  const selectedTenantID = String(sessionLike?.selectedTenantId || "").trim();

  if (!sessionLike?.identity?.is_platform_admin) {
    return fallbackTenantID;
  }
  if (requestedTenantID) {
    return requestedTenantID;
  }
  if (selectedTenantID) {
    return selectedTenantID;
  }
  return fallbackTenantID;
}

export const useAppState = create<AppState>()((set, get) => ({
  session: null,
  currentTenantId: "",
  currentTenantSlug: "",
  currentUserId: "",
  setCurrentTenant: (id: string) => {
    const nextTenantID = (id || "").trim();
    const current = get().session;
    if (current) {
      const activeTenantIDs = listActiveMembershipTenantIDs(current);
      const lockedTenantID = String(current.identity?.tenant_id || "").trim() || activeTenantIDs[0] || "";
      if (!current.identity?.is_platform_admin) {
        const next = { ...current, selectedTenantId: lockedTenantID || nextTenantID };
        set({ session: next, currentTenantId: next.selectedTenantId || "" });
        return;
      }
      const requestedTenantID = nextTenantID || current.selectedTenantId || lockedTenantID;
      const selectedTenantID = requestedTenantID || resolvePreferredTenantID(current, lockedTenantID);
      const next = { ...current, selectedTenantId: selectedTenantID };
      set({ session: next, currentTenantId: next.selectedTenantId || "" });
      return;
    }
    set({ currentTenantId: nextTenantID });
  },
  setCurrentTenantSlug: (slug: string) => {
    set({ currentTenantSlug: String(slug || "").trim() });
  },
  setSession: (session: Session) => {
    useAsyncOpsTracker.getState().clearAll();
    const urlTenantID = readTenantIDFromBrowserURL();
    const selectedTenantID = resolvePreferredTenantID(session, urlTenantID);
    const normalizedSession: Session = { ...session, selectedTenantId: selectedTenantID };
    set({
      session: normalizedSession,
      currentTenantId: selectedTenantID,
      currentTenantSlug: "",
      currentUserId: normalizedSession.identity.user_id,
    });
  },
  clearSession: () => {
    useAsyncOpsTracker.getState().clearAll();
    set({ session: null, currentTenantId: "", currentTenantSlug: "", currentUserId: "" });
  },
}));

function normalizeRole(role?: string): string {
  const raw = (role || "").toLowerCase();
  if (raw.includes("platform")) return "platform_admin";
  if (raw.includes("tenant") && raw.includes("admin")) return "tenant_admin";
  if (raw.includes("admin") || raw.includes("manager")) return "tenant_admin";
  if (raw.includes("view")) return "viewer";
  return "analyst";
}

function roleToLabel(role?: string, isPlatformAdmin?: boolean): string {
  if (isPlatformAdmin || role === "platform_admin") return "Platform Admin";
  if (role === "tenant_admin") return "Tenant Admin";
  if (role === "viewer") return "Viewer";
  return "Analyst";
}

function toUIStrSeverity(severity?: string): string {
  const s = (severity || "").toLowerCase();
  if (s === "critical") return "Critical";
  if (s === "high") return "High";
  if (s === "low") return "Low";
  return "Medium";
}

function toCoreSeverity(severity?: string): string {
  const s = (severity || "").toLowerCase();
  if (["critical", "high", "medium", "low"].includes(s)) {
    return s;
  }
  return "medium";
}

function toUIAlertStatus(status?: string): string {
  const s = (status || "").toLowerCase();
  if (s === "triaged") return "Triaged";
  if (s === "closed") return "Closed";
  return "New";
}

function toCoreAlertStatus(status?: string): string {
  const s = (status || "").toLowerCase();
  if (["new", "triaged", "closed"].includes(s)) {
    return s;
  }
  return "new";
}

function toUICaseStatus(status?: string): string {
  const s = (status || "").toLowerCase();
  if (s === "investigating" || s === "in_progress") return "Investigating";
  if (s === "contained") return "Contained";
  if (s === "resolved") return "Resolved";
  if (s === "closed") return "Closed";
  if (s === "open" || s === "new") return "Open";
  return humanizeCaseStatusCode(s || "open");
}

function toCoreCaseStatus(status?: string): string {
  const normalized = (status || "").trim().toLowerCase().replace(/\s+/g, "_");
  return normalized || "new";
}

function humanizeCaseStatusCode(raw: string): string {
  const normalized = raw.trim().replace(/_/g, " ");
  if (!normalized) return "Open";
  return normalized
    .split(/\s+/)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

function toUITaskStatus(status?: string): string {
  const s = (status || "").toLowerCase();
  if (s === "done" || s === "completed") return "Done";
  if (s === "in progress" || s === "in_progress") return "In Progress";
  if (s === "canceled" || s === "cancelled") return "Canceled";
  return "Pending";
}

function toCoreTaskStatus(status?: string): string {
  const s = (status || "").toLowerCase();
  if (s === "done" || s === "completed") return "done";
  if (s === "in progress" || s === "in_progress") return "in_progress";
  if (s === "canceled" || s === "cancelled") return "canceled";
  return "new";
}

function toUIVerdict(verdict?: string): string {
  const v = (verdict || "").toLowerCase();
  if (v === "malicious") return "Malicious";
  if (v === "suspicious") return "Suspicious";
  if (v === "benign") return "Benign";
  return "Unknown";
}

function toCoreVerdict(verdict?: string): string {
  const v = (verdict || "").toLowerCase();
  if (["malicious", "suspicious", "benign", "unknown"].includes(v)) return v;
  return "unknown";
}

function sanitizeSlug(value?: string): string {
  const base = (value || "").trim().toLowerCase();
  const cleaned = base.replace(/[^a-z0-9-]+/g, "-").replace(/^-+|-+$/g, "");
  return cleaned || `tenant-${Date.now()}`;
}

function ensureArray(value: any): any[] {
  return Array.isArray(value) ? value : [];
}

function timestampToMillis(value?: string): number {
  if (!value) return 0;
  const ts = Date.parse(value);
  return Number.isFinite(ts) ? ts : 0;
}

function compareChronological(
  leftTime?: string,
  rightTime?: string,
  leftID?: string,
  rightID?: string,
): number {
  const diff = timestampToMillis(leftTime) - timestampToMillis(rightTime);
  if (diff !== 0) {
    return diff;
  }
  return String(leftID || "").localeCompare(String(rightID || ""));
}

async function tryRefreshSession(existingSession?: Session | null, tenantOverride?: string): Promise<Session | null> {
  const state = useAppState.getState();
  const urlTenantID = readTenantIDFromBrowserURL();
  const requestedTenantID = tenantOverride || state.currentTenantId || existingSession?.selectedTenantId || urlTenantID || existingSession?.identity?.tenant_id || "";
  const refreshHeaders = new Headers();
  if (requestedTenantID) {
    refreshHeaders.set("X-Tenant-ID", requestedTenantID);
  }

  const refreshResponse = await fetch(`${API_URL}/api/v1/auth/refresh`, {
    method: "POST",
    credentials: "include",
    headers: refreshHeaders,
  });
  if (!refreshResponse.ok) {
    useAppState.getState().clearSession();
    return null;
  }

  const refreshed = await refreshResponse.json();
  if (!refreshed?.access_token || !refreshed?.identity?.user_id) {
    useAppState.getState().clearSession();
    return null;
  }

  const nextSession: Session = {
    accessToken: refreshed.access_token,
    identity: refreshed.identity,
    memberships: refreshed.memberships || existingSession?.memberships || [],
    selectedTenantId: resolvePreferredTenantID(
      {
        identity: refreshed.identity,
        memberships: refreshed.memberships || existingSession?.memberships || [],
        selectedTenantId: existingSession?.selectedTenantId || requestedTenantID,
      },
      requestedTenantID,
    ),
  };
  useAppState.getState().setSession(nextSession);
  return nextSession;
}

type AsyncOperationEnvelope = {
  operation_id: string;
  status: string;
  resource?: string;
  resource_id?: string;
  operation_type?: string;
  error?: string;
};

function isAsyncOperationEnvelope(payload: any): payload is AsyncOperationEnvelope {
  return Boolean(
    payload &&
      typeof payload === "object" &&
      typeof payload.operation_id === "string" &&
      typeof payload.status === "string",
  );
}

type WaitForAsyncOperationOptions = {
  tenantOverride?: string;
  operationType?: string;
  resource?: string;
  resourceId?: string;
};

function toAsyncUIStatus(status: string): "queued" | "processing" | "done" | "failed" {
  const normalized = status.trim().toLowerCase();
  if (normalized === "done") return "done";
  if (normalized === "failed") return "failed";
  if (normalized === "queued") return "queued";
  return "processing";
}

async function waitForAsyncOperation(operationID: string, options?: WaitForAsyncOperationOptions): Promise<AsyncOperationEnvelope> {
  const startedAt = Date.now();
  const timeoutMS = 45_000;
  const pollIntervalMS = 300;
  const tracker = useAsyncOpsTracker.getState();
  tracker.upsertOperation({
    operationId: operationID,
    operationType: options?.operationType || "",
    resource: options?.resource || "",
    resourceId: options?.resourceId || "",
    status: "queued",
  });

  try {
    while (Date.now() - startedAt < timeoutMS) {
      const payload = await coreFetch(
        `/api/v1/operations/${encodeURIComponent(operationID)}?no_cache=1`,
        { method: "GET" },
        true,
        options?.tenantOverride,
      );
      if (!isAsyncOperationEnvelope(payload)) {
        throw new Error("Invalid async operation status payload");
      }
      const status = payload.status.trim().toLowerCase();
      tracker.upsertOperation({
        operationId: operationID,
        operationType: payload.operation_type || options?.operationType || "",
        resource: payload.resource || options?.resource || "",
        resourceId: payload.resource_id || options?.resourceId || "",
        status: toAsyncUIStatus(status),
        error: payload.error || "",
      });
      if (status === "done") {
        return payload;
      }
      if (status === "failed") {
        throw new Error(payload.error || "Async operation failed");
      }
      await new Promise((resolve) => window.setTimeout(resolve, pollIntervalMS));
    }
  } catch (error: any) {
    tracker.upsertOperation({
      operationId: operationID,
      operationType: options?.operationType || "",
      resource: options?.resource || "",
      resourceId: options?.resourceId || "",
      status: "failed",
      error: error?.message || "Async operation failed",
    });
    throw error;
  }

  const timeoutError = new Error("Async operation timed out");
  tracker.upsertOperation({
    operationId: operationID,
    operationType: options?.operationType || "",
    resource: options?.resource || "",
    resourceId: options?.resourceId || "",
    status: "failed",
    error: timeoutError.message,
  });
  throw timeoutError;
}

async function coreFetch(path: string, options?: RequestInit, includeTenantHeader = true, tenantOverride?: string): Promise<any> {
  const state = useAppState.getState();
  let session = state.session;
  const headers = new Headers(options?.headers || {});
  const isFormDataBody = typeof FormData !== "undefined" && options?.body instanceof FormData;

  if (!headers.has("Content-Type") && options?.body && !isFormDataBody) {
    headers.set("Content-Type", "application/json");
  }
  if (session?.accessToken) {
    headers.set("Authorization", `Bearer ${session.accessToken}`);
  }

  const tenantID = tenantOverride || state.currentTenantId || session?.selectedTenantId || session?.identity.tenant_id;
  if (includeTenantHeader && tenantID) {
    headers.set("X-Tenant-ID", tenantID);
  }

  const request = async () =>
    fetch(`${API_URL}${path}`, {
      credentials: "include",
      ...options,
      headers,
    });

  let response = await request();
  if (response.status === 401 && path !== "/api/v1/auth/refresh") {
    const refreshedSession = await tryRefreshSession(session, tenantID);
    if (refreshedSession?.accessToken) {
      session = refreshedSession;
      headers.set("Authorization", `Bearer ${session.accessToken}`);
      response = await request();
    } else {
      throw new Error("Unauthorized");
    }
  }

  if (!response.ok) {
    const text = await response.text();
    try {
      const parsed = JSON.parse(text);
      throw new Error(parsed.message || parsed.error || `HTTP ${response.status}`);
    } catch {
      throw new Error(text || `HTTP ${response.status}`);
    }
  }

  if (response.status === 204) {
    return { success: true };
  }

  const contentType = response.headers.get("Content-Type") || "";
  if (contentType.includes("application/json")) {
    return response.json();
  }
  return response.text();
}

async function login(email: string, password: string): Promise<Session> {
  const response = await fetch(`${API_URL}/api/v1/auth/login`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || "Login failed");
  }
  const payload = await response.json();
  const session: Session = {
    accessToken: payload.access_token,
    identity: payload.identity,
    memberships: payload.memberships || [],
    selectedTenantId: resolvePreferredTenantID(
      {
        identity: payload.identity,
        memberships: payload.memberships || [],
        selectedTenantId: payload.identity?.tenant_id || payload.memberships?.[0]?.tenant_id,
      },
      readTenantIDFromBrowserURL(),
    ),
  };
  useAppState.getState().setSession(session);
  return session;
}

async function logout(): Promise<void> {
  try {
    await coreFetch("/api/v1/auth/logout", { method: "POST" }, false);
  } catch {
    // ignore and clear in-memory session anyway
  }
  useAppState.getState().clearSession();
}

export function useSessionBootstrap() {
  const hasSession = useAppState((state) => Boolean(state.session?.accessToken));
  return useQuery({
    queryKey: ["sessionBootstrap"],
    enabled: !hasSession,
    retry: false,
    staleTime: 30_000,
    queryFn: async () => {
      const existing = useAppState.getState().session;
      return tryRefreshSession(existing);
    },
  });
}

function mapTenant(item: any): any {
  return {
    id: item.id,
    slug: item.slug || item.id,
    name: item.name,
    description: item.description || "",
    active: item.is_active ?? item.active ?? true,
    maxUsers: item.max_users ?? item.maxUsers ?? 50,
    responsibleUserId: item.responsible_user_id ?? item.responsibleUserId,
  };
}

function mapUser(item: any, tenantId?: string): any {
  const session = useAppState.getState().session;
  const roleRaw = (item.role || session?.identity?.tenant_role || "analyst").toString();
  const mappedRole = normalizeRole(roleRaw);
  const isAdmin = Boolean(item.is_platform_admin || mappedRole === "tenant_admin");

  return {
    id: item.id,
    name: item.full_name || item.name || item.username || item.email || "User",
    email: item.email || "",
    username: item.username || "",
    role: roleToLabel(mappedRole, item.is_platform_admin),
    team: item.team || "SOC",
    tenantId: tenantId || item.tenant_id || session?.selectedTenantId || session?.identity.tenant_id,
    isAdmin,
    isPlatformAdmin: Boolean(item.is_platform_admin),
    avatar: item.avatar_url || item.avatar || "",
    coverImage: item.cover_image_url || item.cover_image || "",
    personalLink: item.personal_link || "",
    createdAt: item.created_at || item.createdAt || "",
    experiencePoints: Number(item.experience_points ?? item.experiencePoints ?? 0),
    timeRecipient: item.time_recipient || item.timeRecipient || "",
  };
}

function mapDutyAnalyst(item: any): any {
  return {
    id: item?.id || "",
    tenantId: item?.tenant_id || item?.tenantId || "",
    name: item?.full_name || item?.name || item?.username || item?.email || item?.id || "User",
    role: item?.role || "analyst",
    username: item?.username || "",
    email: item?.email || "",
    team: item?.team || "",
    avatar: item?.avatar_url || item?.avatar || "",
    isActive: Boolean(item?.is_active ?? true),
  };
}

function mapDutyShift(item: any): any {
  return {
    id: item?.id || "",
    analystId: item?.analyst_id || "",
    analyst: item?.analyst ? mapDutyAnalyst(item.analyst) : undefined,
    startsAt: item?.starts_at || item?.startsAt || "",
    endsAt: item?.ends_at || item?.endsAt || "",
    isOvernight: Boolean(item?.is_overnight ?? item?.isOvernight ?? false),
    durationMinutes: Number(item?.duration_minutes ?? item?.durationMinutes ?? 0),
  };
}

function mapDutySection(section: any): any {
  return {
    count: Number(section?.count ?? 0),
    shifts: ensureArray(section?.shifts).map(mapDutyShift),
    analysts: ensureArray(section?.analysts).map(mapDutyAnalyst),
    startsAt: section?.starts_at || section?.startsAt || "",
  };
}

function mapDutyOverview(payload: any): any {
  return {
    tenantId: payload?.tenant_id || payload?.tenantId || "",
    asOf: payload?.as_of || payload?.asOf || "",
    onDuty: ensureArray(payload?.on_duty || payload?.onDuty).map(mapDutyAnalyst),
    current: mapDutySection(payload?.current || {}),
    next: mapDutySection(payload?.next || {}),
    meta: {
      loadedShifts: Number(payload?.meta?.loaded_shifts ?? payload?.meta?.loadedShifts ?? 0),
      parsedShifts: Number(payload?.meta?.parsed_shifts ?? payload?.meta?.parsedShifts ?? 0),
      invalidShifts: Number(payload?.meta?.invalid_shifts ?? payload?.meta?.invalidShifts ?? 0),
      limit: Number(payload?.meta?.limit ?? 0),
    },
  };
}

function fallbackExperienceEventDescription(item: any): string {
  const eventType = String(item?.event_type || item?.eventType || "").toLowerCase();
  if (eventType === "case_closed") {
    const rawSeverity = String(item?.details?.severity || "").toLowerCase();
    const severity = ["low", "medium", "high", "critical"].includes(rawSeverity) ? rawSeverity : "medium";
    return `Closed case (${severity} severity)`;
  }
  if (eventType === "achievement_granted") return "Achievement granted";
  if (eventType === "connector_invoked") return "Connector invoked";
  if (eventType === "analyzer_invoked") return "Analyzer invoked";
  if (eventType === "responder_invoked") return "Responder invoked";
  if (eventType === "incident_first_message") return "First incident message sent";
  if (eventType === "manual_grant") return "Manual experience grant";
  return "Experience rewarded";
}

function mapExperienceEvent(item: any): any {
  const details = item?.details && typeof item.details === "object" ? item.details : {};
  const pointsRaw = Number(item?.points ?? 0);
  const points = Number.isFinite(pointsRaw) ? Math.trunc(pointsRaw) : 0;
  const description = String(item?.description || "").trim() || fallbackExperienceEventDescription(item);
  return {
    id: item?.id || "",
    tenantId: item?.tenant_id || item?.tenantId || "",
    userId: item?.user_id || item?.userId || "",
    eventKey: item?.event_key || item?.eventKey || "",
    eventType: String(item?.event_type || item?.eventType || "unknown"),
    points,
    description,
    details,
    createdAt: item?.created_at || item?.createdAt || "",
  };
}

function asNullableNumber(value: any): number | null {
  if (value === null || value === undefined || value === "") {
    return null;
  }
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    return null;
  }
  return parsed;
}

function mapUserCasePerformance(item: any): any {
  return {
    closedCasesTotal: Number(item?.closed_cases_total ?? item?.closedCasesTotal ?? 0),
    avgInvestigationMinutes: Number(item?.avg_investigation_minutes ?? item?.avgInvestigationMinutes ?? 0),
    currentMonthClosedCases: Number(item?.current_month_closed_cases ?? item?.currentMonthClosedCases ?? 0),
    previousMonthClosedCases: Number(item?.previous_month_closed_cases ?? item?.previousMonthClosedCases ?? 0),
    currentMonthAvgInvestigationMinutes: asNullableNumber(item?.current_month_avg_investigation_minutes ?? item?.currentMonthAvgInvestigationMinutes),
    previousMonthAvgInvestigationMinutes: asNullableNumber(item?.previous_month_avg_investigation_minutes ?? item?.previousMonthAvgInvestigationMinutes),
  };
}

function mapAlert(item: any, meta?: any): any {
  const m = meta || {};
  const fallbackTags = ensureArray(item.tags || [item.tlp?.toUpperCase(), item.pap?.toUpperCase()].filter(Boolean));
  const resolvedTags = ensureArray(m.tags ?? fallbackTags);
  return {
    id: item.id,
    title: item.title,
    source: item.source,
    sev: toUIStrSeverity(item.severity),
    time: item.created_at || item.updated_at,
    tags: resolvedTags.length > 0 ? resolvedTags : fallbackTags,
    owner: item.assigned_to || item.owner || undefined,
    status: toUIAlertStatus(item.status),
    statusCode: (item.status || "").toString().toLowerCase(),
    tenantId: item.tenant_id,
    caseId: item.case_id || item.caseId || undefined,
    tlp: item.tlp || "amber",
    pap: item.pap || "amber",
    description: item.description || "",
    createdAt: item.created_at,
    updatedAt: item.updated_at,
  };
}

function normalizeCustomFields(value: any): Record<string, string> {
  const customFields: Record<string, string> = {};
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return customFields;
  }
  Object.entries(value).forEach(([key, rawValue]) => {
    const normalizedKey = String(key || "").trim();
    if (!normalizedKey) return;
    if (rawValue === null || rawValue === undefined) {
      customFields[normalizedKey] = "";
      return;
    }
    if (typeof rawValue === "string") {
      customFields[normalizedKey] = rawValue;
      return;
    }
    try {
      customFields[normalizedKey] = JSON.stringify(rawValue);
    } catch {
      customFields[normalizedKey] = String(rawValue);
    }
  });
  return customFields;
}

function mapCase(item: any, meta?: any): any {
  const m = meta || {};
  const customFields = normalizeCustomFields(m.custom_fields ?? m.customFields);
  const resolvedTags = ensureArray(m.tags ?? item.tags);
  const fallbackTags = [item.tlp?.toUpperCase(), item.pap?.toUpperCase()].filter(Boolean);
  const hasMetaOwner = Object.prototype.hasOwnProperty.call(m, "owner") || Object.prototype.hasOwnProperty.call(m, "owner_id");
  const resolvedOwner = hasMetaOwner
    ? (m.owner || m.owner_id || undefined)
    : (item.owner_id || item.created_by || item.createdBy || undefined);
  return {
    id: item.id,
    caseNumber: item.case_number || m.caseNumber || m.case_number || item.id,
    title: item.title,
    description: item.description || "",
    source: item.source || "manual",
    incidentType: item.incident_type || m.incidentType || "",
    priority: (item.priority || "medium").toString(),
    impact: item.impact || "",
    confidence: Number(item.confidence ?? m.confidence ?? 0),
    statusCode: (item.status || "new").toString().toLowerCase(),
    sev: toUIStrSeverity(item.severity),
    status: toUICaseStatus(item.status),
    tlp: (item.tlp || "amber").toString().toUpperCase(),
    pap: (item.pap || "amber").toString().toUpperCase(),
    owner: resolvedOwner,
    assignee: item.assigned_to || undefined,
    detectedAt: item.detected_at || m.detectedAt || undefined,
    occurredAt: item.occurred_at || m.occurredAt || undefined,
    closedAt: item.closed_at || m.closedAt || undefined,
    resolutionSummary: item.resolution_summary || m.resolutionSummary || "",
    createdAt: item.created_at || "",
    updatedAt: item.updated_at || "",
    time: item.created_at || item.updated_at,
    tags: resolvedTags.length > 0 ? resolvedTags : fallbackTags,
    forumId: m.forumId || m.forum_id,
    tactics: ensureArray(m.tactics),
    techniques: ensureArray(m.techniques),
    category: m.category || m.caseCategory || "",
    relatedProduct: m.relatedProduct || m.related_product || "",
    verdict: m.verdict || "Unknown",
    recommendations: ensureArray(m.recommendations),
    stage: m.stage || "",
    customFields,
    tenantId: item.tenant_id,
  };
}

function mapCaseListSummary(item: any): any {
  const statuses = item?.responder_statuses ?? item?.responderStatuses ?? {};
  return {
    caseId: String(item?.case_id || item?.caseId || "").trim(),
    openTasks: Number(item?.open_tasks ?? item?.openTasks ?? 0),
    closedTasks: Number(item?.closed_tasks ?? item?.closedTasks ?? 0),
    totalTasks: Number(item?.total_tasks ?? item?.totalTasks ?? 0),
    taskProgressPercent: Number(item?.task_progress_percent ?? item?.taskProgressPercent ?? 0),
    nextOpenTaskDueAt: String(item?.next_open_task_due_at ?? item?.nextOpenTaskDueAt ?? "").trim(),
    overdueOpenTasks: Number(item?.overdue_open_tasks ?? item?.overdueOpenTasks ?? 0),
    responderStatuses: {
      queued: Number(statuses?.queued ?? 0),
      running: Number(statuses?.running ?? 0),
      completed: Number(statuses?.completed ?? 0),
      failed: Number(statuses?.failed ?? 0),
    },
  };
}

function mapActivityLivestreamItem(item: any): any {
  return {
    id: String(item?.id || "").trim(),
    entity: String(item?.entity || "").trim().toLowerCase(),
    entityId: String(item?.entity_id || item?.entityId || "").trim(),
    action: String(item?.action || "").trim().toLowerCase() || "updated",
    title: String(item?.title || "").trim(),
    description: String(item?.description || "").trim(),
    severity: String(item?.severity || "").trim(),
    status: String(item?.status || "").trim(),
    source: String(item?.source || "").trim(),
    assigneeId: String(item?.assignee_id || item?.assigneeId || "").trim(),
    caseId: String(item?.case_id || item?.caseId || "").trim(),
    createdAt: String(item?.created_at || item?.createdAt || "").trim(),
    updatedAt: String(item?.updated_at || item?.updatedAt || "").trim(),
  };
}

function mapDashboardMetricBucket(item: any): any {
  return {
    key: String(item?.key || "").trim() || "unknown",
    count: Number(item?.count ?? 0),
  };
}

function mapDashboardSLA(item: any): any {
  return {
    severity: String(item?.severity || "").trim().toLowerCase() || "medium",
    targetMinutes: Number(item?.targetMinutes ?? item?.target_minutes ?? 0),
    openCases: Number(item?.openCases ?? item?.open_cases ?? 0),
    resolvedCases: Number(item?.resolvedCases ?? item?.resolved_cases ?? 0),
    avgResolutionMinutes: Number(item?.avgResolutionMinutes ?? item?.avg_resolution_minutes ?? 0),
    breachedCases: Number(item?.breachedCases ?? item?.breached_cases ?? 0),
  };
}

function mapDashboardAnalystMetric(item: any): any {
  return {
    userId: String(item?.userId ?? item?.user_id ?? "").trim(),
    username: String(item?.username || "").trim(),
    displayName: String((item?.displayName ?? item?.display_name) || "").trim(),
    resolvedCases: Number(item?.resolvedCases ?? item?.resolved_cases ?? 0),
  };
}

function mapDashboardCustomMetric(item: any): any {
  const filters = item?.filters && typeof item.filters === "object" ? item.filters : {};
  return {
    id: String(item?.id || "").trim(),
    name: String(item?.name || "").trim(),
    description: String(item?.description || "").trim(),
    source: String(item?.source || "cases").trim().toLowerCase(),
    measure: String(item?.measure || "count").trim().toLowerCase(),
    enabled: Boolean(item?.enabled ?? true),
    value: Number(item?.value ?? 0),
    error: String(item?.error || "").trim(),
    createdAt: String(item?.created_at || item?.createdAt || "").trim(),
    updatedAt: String(item?.updated_at || item?.updatedAt || "").trim(),
    filters: {
      statuses: ensureArray(filters?.statuses).map((entry: any) => String(entry || "").trim()).filter(Boolean),
      severities: ensureArray(filters?.severities).map((entry: any) => String(entry || "").trim()).filter(Boolean),
      categories: ensureArray(filters?.categories).map((entry: any) => String(entry || "").trim()).filter(Boolean),
      createdFrom: String((filters?.created_from ?? filters?.createdFrom) || "").trim(),
      createdTo: String((filters?.created_to ?? filters?.createdTo) || "").trim(),
      closedFrom: String((filters?.closed_from ?? filters?.closedFrom) || "").trim(),
      closedTo: String((filters?.closed_to ?? filters?.closedTo) || "").trim(),
      createdWithinHours: Number(filters?.created_within_hours ?? filters?.createdWithinHours ?? 0),
      closedWithinHours: Number(filters?.closed_within_hours ?? filters?.closedWithinHours ?? 0),
      overdueMinutes: Number(filters?.overdue_minutes ?? filters?.overdueMinutes ?? 0),
    },
  };
}

function mapRelatedCase(item: any): any {
  const mappedCase = mapCase(item);
  return {
    ...mappedCase,
    matchCount: Number(item?.match_count ?? item?.matchCount ?? 0),
    matchedObservables: ensureArray(item?.matched_observables ?? item?.matchedObservables).map((observable: any) => ({
      type: String(observable?.type || "").trim(),
      value: String(observable?.value || "").trim(),
    })).filter((observable: any) => observable.type && observable.value),
    matchedFields: ensureArray(item?.matched_fields ?? item?.matchedFields).map((field: any) => ({
      field: String(field?.field || "").trim().toLowerCase(),
      value: String(field?.value || "").trim(),
    })).filter((field: any) => field.field && field.value),
  };
}

function mapTask(item: any): any {
  return {
    id: item.id,
    caseId: item.case_id,
    tenantId: item.tenant_id,
    title: item.title,
    description: item.description || "",
    status: toUITaskStatus(item.status),
    assignee: item.assignee_id || undefined,
    mandatory: Boolean(item.mandatory),
    dueDate: item.due_date || item.dueDate || "",
    createdAt: item.created_at,
    updatedAt: item.updated_at,
  };
}

function mapObservable(item: any): any {
  return {
    id: item.id,
    caseId: item.case_id,
    tenantId: item.tenant_id,
    type: item.type,
    value: item.value,
    verdict: toUIVerdict(item.verdict),
    source: item.source,
    tags: ensureArray(item.tags),
    createdAt: item.created_at,
    updatedAt: item.updated_at,
  };
}

function mapCaseAttachment(item: any): any {
  return {
    id: item.id,
    caseId: item.case_id || item.caseId,
    tenantId: item.tenant_id || item.tenantId,
    fileName: item.file_name || item.fileName || "",
    contentType: item.content_type || item.contentType || "application/octet-stream",
    fileSizeBytes: Number(item.file_size_bytes ?? item.fileSizeBytes ?? 0),
    storageKey: item.storage_key || item.storageKey || "",
    checksumSHA256: item.checksum_sha256 || item.checksumSHA256 || "",
    uploadedBy: item.uploaded_by || item.uploadedBy || "",
    createdAt: item.created_at || item.createdAt,
    updatedAt: item.updated_at || item.updatedAt,
  };
}

function mapCasePage(item: any): any {
  return {
    id: item?.id || "",
    caseId: item?.case_id || item?.caseId || "",
    tenantId: item?.tenant_id || item?.tenantId || "",
    title: item?.title || "",
    body: item?.body || "",
    createdBy: item?.created_by || item?.createdBy || "",
    createdAt: item?.created_at || item?.createdAt || "",
    updatedAt: item?.updated_at || item?.updatedAt || "",
  };
}

function mapForumAttachment(item: any): any {
  return {
    id: item.id || "",
    fileName: item.file_name || item.fileName || "",
    contentType: item.content_type || item.contentType || "application/octet-stream",
    sizeBytes: Number(item.size_bytes ?? item.sizeBytes ?? item.file_size_bytes ?? 0),
    storageKey: item.storage_key || item.storageKey || "",
    storageURI: item.storage_uri || item.storageUri || "",
    checksumSHA256: item.checksum_sha256 || item.checksumSHA256 || "",
    url: item.download_url || item.url || "",
    previewUrl: item.preview_url || item.download_url || item.url || "",
  };
}

function mapCatalogItem(item: any): any {
  return {
    ...item,
    id: item.id,
    tenantId: item.tenant_id || item.tenantId,
    ownerId: item.owner_id || item.ownerId,
    refId: item.ref_id || item.refId,
    createdAt: item.created_at || item.createdAt,
    updatedAt: item.updated_at || item.updatedAt,
  };
}

function catalogDataView(item: any): Record<string, any> {
  const base = item && typeof item === "object" ? item : {};
  const nested = base?.data && typeof base.data === "object" && !Array.isArray(base.data) ? base.data : {};
  return { ...base, ...nested };
}

function stringSliceFromUnknown(value: any): string[] {
  return ensureArray(value)
    .map((entry: any) => String(entry || "").trim())
    .filter(Boolean);
}

function boolFromUnknown(value: any, fallback: boolean): boolean {
  if (typeof value === "boolean") {
    return value;
  }
  if (typeof value === "string") {
    const normalized = value.trim().toLowerCase();
    if (normalized === "true") return true;
    if (normalized === "false") return false;
  }
  if (typeof value === "number") {
    return value > 0;
  }
  return fallback;
}

function numberFromUnknown(value: any, fallback: number): number {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    return fallback;
  }
  return parsed;
}

function mapAIAgentCatalogItem(item: any): any {
  const base = mapCatalogItem(item);
  const view = catalogDataView(item);
  const maxCasesPerRun = Math.max(1, Math.min(100, Math.round(numberFromUnknown(view.max_cases_per_run ?? view.maxCasesPerRun ?? view.max_cases ?? view.maxCases, 10))));
  const autoActionMinConfidence = Math.max(
    0,
    Math.min(100, numberFromUnknown(view.auto_action_min_confidence ?? view.autoActionMinConfidence ?? view.auto_action_confidence ?? view.autoActionConfidence, 85)),
  );
  const investigationPlan = ensureArray(view.investigation_plan ?? view.investigationPlan ?? view.plan ?? view.stages).map((stage: any) => ({
    id: String(stage?.id || "").trim(),
    name: String(stage?.name || stage?.title || "").trim(),
    description: String(stage?.description || "").trim(),
    prompt: String(stage?.prompt || stage?.instruction || stage?.instructions || "").trim(),
    caseTags: stringSliceFromUnknown(stage?.case_tags ?? stage?.caseTags ?? stage?.add_tags ?? stage?.addTags),
    enrichmentConnectorIds: stringSliceFromUnknown(
      stage?.enrichment_connector_ids ?? stage?.enrichmentConnectorIds ?? stage?.connector_ids ?? stage?.connectorIds,
    ),
  }));
  return {
    ...base,
    name: String(view.name || view.title || "AI Agent").trim(),
    description: String(view.description || "").trim(),
    prompt: String(view.prompt || view.instructions || view.system_prompt || "").trim(),
    model: String(view.model || "").trim(),
    provider: String(view.provider || view.ai_provider || view.aiProvider || "").trim(),
    endpoint: String(view.endpoint || view.base_url || view.baseURL || view.api_base || view.apiBase || "").trim(),
    language: String(view.language || "").trim(),
    enabled: boolFromUnknown(view.enabled ?? view.is_enabled ?? view.isEnabled, true),
    targetTypes: stringSliceFromUnknown(view.target_types ?? view.targetTypes ?? view.targets),
    caseTags: stringSliceFromUnknown(view.case_tags ?? view.caseTags ?? view.tags),
    alertSources: stringSliceFromUnknown(view.alert_sources ?? view.alertSources ?? view.sources),
    autoCaseTags: stringSliceFromUnknown(view.auto_case_tags ?? view.autoCaseTags),
    autoCloseCase: boolFromUnknown(view.auto_close_case ?? view.autoCloseCase ?? view.auto_close, false),
    autoCloseVerdicts: stringSliceFromUnknown(view.auto_close_verdicts ?? view.autoCloseVerdicts),
    autoCreateCaseFromAlert: boolFromUnknown(view.auto_create_case_from_alert ?? view.autoCreateCaseFromAlert, false),
    enrichmentConnectorIds: stringSliceFromUnknown(view.enrichment_connector_ids ?? view.enrichmentConnectorIds ?? view.connector_ids ?? view.connectorIds),
    notificationConnectorIds: stringSliceFromUnknown(
      view.notification_connector_ids ?? view.notificationConnectorIds ?? view.notify_connector_ids ?? view.notifyConnectorIds,
    ),
    investigationPlan,
    maxCasesPerRun,
    autoCreateTasks: boolFromUnknown(view.auto_create_tasks ?? view.autoCreateTasks, true),
    autoComment: boolFromUnknown(view.auto_comment ?? view.autoComment, true),
    taskAssigneeId: String(view.task_assignee_id ?? view.taskAssigneeId ?? view.assignee_user_id ?? view.assigneeUserId ?? "").trim(),
    triadEnabled: boolFromUnknown(view.triad_enabled ?? view.triadEnabled ?? view.three_agent_mode ?? view.threeAgentMode, false),
    triadCriticalOnly: boolFromUnknown(view.triad_critical_only ?? view.triadCriticalOnly ?? view.critical_only_triad ?? view.criticalOnlyTriad, true),
    investigatorPrompt: String(view.investigator_prompt ?? view.investigatorPrompt ?? "").trim(),
    reviewerPrompt: String(view.reviewer_prompt ?? view.reviewerPrompt ?? "").trim(),
    arbiterPrompt: String(view.arbiter_prompt ?? view.arbiterPrompt ?? "").trim(),
    requireReviewerConsensus: boolFromUnknown(
      view.require_reviewer_consensus ?? view.requireReviewerConsensus ?? view.reviewer_consensus_required ?? view.reviewerConsensusRequired,
      true,
    ),
    executionPolicy: String(view.execution_policy ?? view.executionPolicy ?? view.queue_execution_policy ?? view.queueExecutionPolicy ?? "all_matching").trim() || "all_matching",
    executionPriority: Math.max(-1000, Math.min(1000, Math.round(numberFromUnknown(view.execution_priority ?? view.executionPriority ?? view.queue_execution_priority ?? view.queueExecutionPriority, 0)))),
    autoActionMinConfidence,
  };
}

function mapAIAgentRunCatalogItem(item: any): any {
  const base = mapCatalogItem(item);
  const view = catalogDataView(item);
  return {
    ...base,
    runId: String(view.run_id || base.id || "").trim(),
    agentId: String(view.agent_id || view.agentId || base.refId || "").trim(),
    agentName: String(view.agent_name || view.agentName || "").trim(),
    startedAt: String(view.started_at || view.startedAt || base.createdAt || "").trim(),
    finishedAt: String(view.finished_at || view.finishedAt || "").trim(),
    dryRun: boolFromUnknown(view.dry_run ?? view.dryRun, false),
    matchedCasesTotal: Math.max(0, Math.round(numberFromUnknown(view.matched_cases_total ?? view.matchedCasesTotal, 0))),
    processedCases: Math.max(0, Math.round(numberFromUnknown(view.processed_cases ?? view.processedCases, 0))),
    successfulCases: Math.max(0, Math.round(numberFromUnknown(view.successful_cases ?? view.successfulCases, 0))),
    results: ensureArray(view.results),
  };
}

function mapWorkflowRun(item: any): any {
  return {
    id: item?.id || "",
    tenantId: item?.tenant_id || item?.tenantId || "",
    workflowId: item?.workflow_id || item?.workflowId || "",
    workflowKind: item?.workflow_kind || item?.workflowKind || "",
    trigger: item?.trigger || "manual_test",
    status: String(item?.status || "queued").toLowerCase(),
    startedAt: item?.started_at || item?.startedAt || "",
    finishedAt: item?.finished_at || item?.finishedAt || null,
    durationMs: Number(item?.duration_ms ?? item?.durationMs ?? 0),
    input: item?.input && typeof item.input === "object" ? item.input : {},
    result: item?.result && typeof item.result === "object" ? item.result : {},
    error: item?.error || "",
    createdBy: item?.created_by || item?.createdBy || "",
    createdAt: item?.created_at || item?.createdAt || "",
    updatedAt: item?.updated_at || item?.updatedAt || "",
  };
}

function extractCaseReferenceFromPayload(payload: any): string {
  if (!payload || typeof payload !== "object") {
    return "";
  }
  const direct = [
    payload.case_id,
    payload.caseId,
    payload.caseID,
    payload.ref_id,
    payload.refId,
  ];
  for (const candidate of direct) {
    const normalized = String(candidate || "").trim();
    if (normalized) {
      return normalized;
    }
  }

  const caseObject = payload.case;
  if (caseObject && typeof caseObject === "object") {
    const nested = [
      caseObject.id,
      caseObject.case_id,
      caseObject.caseId,
      caseObject.caseID,
    ];
    for (const candidate of nested) {
      const normalized = String(candidate || "").trim();
      if (normalized) {
        return normalized;
      }
    }
  }

  const contextObject = payload.context;
  if (contextObject && typeof contextObject === "object") {
    const nested = [
      contextObject.case_id,
      contextObject.caseId,
      contextObject.caseID,
      contextObject.ref_id,
      contextObject.refId,
    ];
    for (const candidate of nested) {
      const normalized = String(candidate || "").trim();
      if (normalized) {
        return normalized;
      }
    }
  }

  return "";
}

function workflowRunReferencesCase(run: any, caseId: string): boolean {
  const normalizedCaseID = String(caseId || "").trim().toLowerCase();
  if (!normalizedCaseID) {
    return false;
  }
  const candidates = [
    run?.case_id,
    run?.caseId,
    extractCaseReferenceFromPayload(run?.input),
    extractCaseReferenceFromPayload(run?.result),
  ];
  return candidates.some((candidate) => String(candidate || "").trim().toLowerCase() === normalizedCaseID);
}

function mapCaseCommunicationMessage(item: any): any {
  return {
    ...item,
    id: item.id,
    threadId: item.thread_id || item.threadId,
    caseId: item.case_id || item.caseId,
    tenantId: item.tenant_id || item.tenantId,
    connectorId: item.connector_id || item.connectorId,
    direction: item.direction || "inbound",
    authorId: item.author_id || item.authorId || "",
    authorName: item.authorName || item.author_name || "",
    content: item.content || "",
    timestamp: item.timestamp || item.created_at || item.createdAt,
    externalId: item.external_id || item.externalId || "",
    deliveryStatus: item.delivery_status || item.deliveryStatus || "",
    metadata: item.metadata || {},
    createdAt: item.created_at || item.createdAt,
    updatedAt: item.updated_at || item.updatedAt,
  };
}

function mapCaseCommunicationThread(item: any): any {
  const messages = ensureArray(item.messages).map(mapCaseCommunicationMessage);
  messages.sort((left: any, right: any) =>
    compareChronological(left.timestamp || left.createdAt, right.timestamp || right.createdAt, left.id, right.id),
  );
  return {
    ...item,
    id: item.id,
    caseId: item.case_id || item.caseId || item.ref_id || item.refId,
    tenantId: item.tenant_id || item.tenantId,
    title: item.title || "Communication",
    status: item.status || "open",
    channel: item.channel || "",
    communicationMode: item.communication_mode || item.communicationMode || "",
    connectorId: item.connector_id || item.connectorId || "",
    participant: item.participant || {},
    metadata: item.metadata || {},
    subject: item.subject || "",
    lastMessageAt: item.last_message_at || item.lastMessageAt || item.updated_at || item.updatedAt,
    lastMessagePreview: item.last_message_preview || item.lastMessagePreview || "",
    lastSyncedAt: item.last_synced_at || item.lastSyncedAt || "",
    lastSyncCreatedCount: Number(item.last_sync_created_count ?? item.lastSyncCreatedCount ?? 0) || 0,
    messages,
    createdAt: item.created_at || item.createdAt,
    updatedAt: item.updated_at || item.updatedAt,
  };
}

function mapSyntheticActor(input: any): any | null {
  const actor = input && typeof input === "object" ? input : {};
  const authorKind = String(actor?.author_kind || actor?.authorKind || "").trim();
  const authorId = String(actor?.author_id || actor?.authorId || "").trim();
  const authorName = String(actor?.author_name || actor?.authorName || actor?.agent_name || actor?.agentName || "").trim();
  const authorAvatarKey = String(actor?.author_avatar_key || actor?.authorAvatarKey || "").trim();
  const agentId = String(actor?.agent_id || actor?.agentId || "").trim();
  if (!authorKind && !authorId && !authorName && !authorAvatarKey && !agentId) {
    return null;
  }
  return {
    authorKind: authorKind || (authorId.startsWith("ai-agent") ? "ai_agent" : ""),
    authorId: authorId,
    authorName: authorName || authorId || "AI Agent",
    authorAvatarKey: authorAvatarKey,
    agentId,
  };
}

function mapAIMessage(item: any): any {
  return {
    id: item.id,
    sessionId: item.session_id || item.sessionId,
    tenantId: item.tenant_id || item.tenantId,
    userId: item.user_id || item.userId,
    role: item.role || "assistant",
    content: item.content || "",
    sources: ensureArray(item.sources),
    metadata: item.metadata || {},
    createdAt: item.created_at || item.createdAt,
  };
}

function mapAISession(item: any): any {
  return {
    id: item?.id || "",
    tenantId: item?.tenant_id || item?.tenantId || "",
    userId: item?.user_id || item?.userId || "",
    title: item?.title || "SOC Assistant",
    isDefault: Boolean(item?.is_default ?? item?.isDefault),
    sortOrder: Number(item?.sort_order ?? item?.sortOrder ?? 0),
    lastMessagePreview: item?.last_message_preview || item?.lastMessagePreview || "",
    lastMessageRole: item?.last_message_role || item?.lastMessageRole || "",
    lastMessageAt: item?.last_message_at || item?.lastMessageAt || item?.updated_at || item?.updatedAt || "",
    createdAt: item?.created_at || item?.createdAt || "",
    updatedAt: item?.updated_at || item?.updatedAt || "",
  };
}

function mapCaseAIAnalysis(item: any): any {
  return {
    id: item.id,
    tenantId: item.tenant_id || item.tenantId,
    caseId: item.case_id || item.caseId,
    requestedBy: item.requested_by || item.requestedBy,
    model: item.model || "",
    status: item.status || "completed",
    verdict: toUIVerdict(item.verdict),
    confidence: Number(item.confidence ?? 0),
    summary: item.summary || "",
    recommendations: ensureArray(item.recommendations),
    findings: ensureArray(item.findings),
    sources: ensureArray(item.sources),
    errorMessage: item.error_message || item.errorMessage || "",
    createdAt: item.created_at || item.createdAt,
  };
}

function mapAuthSession(item: any): any {
  return {
    id: item.id,
    userAgent: item.user_agent || item.userAgent || "",
    ipAddress: item.ip_address || item.ipAddress || "",
    createdAt: item.created_at || item.createdAt,
    expiresAt: item.expires_at || item.expiresAt,
    revokedAt: item.revoked_at || item.revokedAt || null,
    status: (item.status || "expired").toString().toLowerCase(),
    isCurrent: Boolean(item.is_current ?? item.isCurrent),
    durationSeconds: Number(item.duration_seconds ?? item.durationSeconds ?? 0),
    remainingSeconds: Number(item.remaining_seconds ?? item.remainingSeconds ?? 0),
  };
}

function mapCaseStatus(item: any): any {
  const code = (item?.code || "").toString().trim().toLowerCase();
  return {
    id: item?.id || "",
    code: code || "new",
    label: item?.label || humanizeCaseStatusCode(code || "new"),
    order: Number(item?.order ?? 0),
    isClosed: Boolean(item?.is_closed ?? item?.isClosed),
    color: item?.color || "",
    tenantId: item?.tenant_id || item?.tenantId || "",
  };
}

function mapNotificationBot(item: any): any {
  return {
    id: item?.id || "",
    tenantId: item?.tenant_id || item?.tenantId || "",
    name: item?.name || "",
    botId: Number(item?.bot_id ?? item?.botId ?? 0),
    botUsername: item?.bot_username || item?.botUsername || "",
    botFirstName: item?.bot_first_name || item?.botFirstName || "",
    canJoinGroups: Boolean(item?.can_join_groups ?? item?.canJoinGroups),
    canReadAllGroupMessages: Boolean(item?.can_read_all_group_messages ?? item?.canReadAllGroupMessages),
    supportsInlineQueries: Boolean(item?.supports_inline_queries ?? item?.supportsInlineQueries),
    enabled: Boolean(item?.enabled ?? true),
    createdBy: item?.created_by || item?.createdBy || "",
    createdAt: item?.created_at || item?.createdAt || "",
    updatedAt: item?.updated_at || item?.updatedAt || "",
  };
}

function mapMyNotificationSettings(item: any): any {
  return {
    id: item?.id || "",
    tenantId: item?.tenant_id || item?.tenantId || "",
    userId: item?.user_id || item?.userId || "",
    deliveryEnabled: Boolean(item?.delivery_enabled ?? item?.deliveryEnabled),
    deliveryChannel: String(item?.delivery_channel || item?.deliveryChannel || "in_app").toLowerCase(),
    telegramBotId: item?.telegram_bot_id || item?.telegramBotId || "",
    telegramChatId: item?.telegram_chat_id || item?.telegramChatId || "",
    telegramUsername: item?.telegram_username || item?.telegramUsername || "",
    notificationEmail: item?.notification_email || item?.notificationEmail || "",
    timeRecipient: item?.time_recipient || item?.timeRecipient || "",
    updatedBy: item?.updated_by || item?.updatedBy || "",
    createdAt: item?.created_at || item?.createdAt || "",
    updatedAt: item?.updated_at || item?.updatedAt || "",
  };
}

function mapAdminNotificationSetting(item: any): any {
  return {
    ...mapMyNotificationSettings(item),
    userName: item?.user_name || item?.userName || "",
    userEmail: item?.user_email || item?.userEmail || "",
    userRole: item?.user_role || item?.userRole || "",
    userActive: Boolean(item?.user_active ?? item?.userActive ?? true),
    isPlatformAdmin: Boolean(item?.is_platform_admin ?? item?.isPlatformAdmin ?? false),
  };
}

function mapServiceAlertRule(item: any): any {
  const rawCriticalityLevels = item?.criticality_levels ?? item?.criticalityLevels ?? item?.critical_severities ?? item?.criticalSeverities ?? [];
  const rawEscalationEventTypes = item?.escalation_event_types ?? item?.escalationEventTypes ?? item?.event_types ?? item?.eventTypes ?? [];
  return {
    id: item?.id || "",
    tenantId: item?.tenant_id || item?.tenantId || "",
    name: String(item?.name || "Service alert"),
    description: String(item?.description || ""),
    metricType: String(item?.metric_type || item?.metricType || "low_rps").toLowerCase(),
    module: String(item?.module || "api").toLowerCase(),
    minRps: Number(item?.min_rps ?? item?.minRps ?? 0),
    maxLatencyMs: Number(item?.max_latency_ms ?? item?.maxLatencyMs ?? 0),
    slaSeconds: Number(item?.sla_seconds ?? item?.slaSeconds ?? item?.max_age_seconds ?? item?.maxAgeSeconds ?? 0),
    casesThreshold: Number(item?.cases_threshold ?? item?.casesThreshold ?? item?.case_threshold ?? item?.caseThreshold ?? item?.threshold ?? 0),
    thresholdMode: String(item?.threshold_mode || item?.thresholdMode || item?.scope || "in_work").toLowerCase(),
    criticalityLevels: ensureArray(rawCriticalityLevels).map((value: any) => String(value || "").trim().toLowerCase()).filter(Boolean),
    escalationEventTypes: ensureArray(rawEscalationEventTypes).map((value: any) => String(value || "").trim().toLowerCase()).filter(Boolean),
    pingInactivitySeconds: Number(item?.ping_inactivity_seconds ?? item?.pingInactivitySeconds ?? item?.ping_seconds ?? item?.pingSeconds ?? item?.stale_seconds ?? item?.staleSeconds ?? 0),
    openStatuses: ensureArray(item?.open_statuses ?? item?.openStatuses ?? item?.statuses ?? []).map((value: any) => String(value || "").trim().toLowerCase()).filter(Boolean),
    windowSeconds: Number(item?.window_seconds ?? item?.windowSeconds ?? 300),
    cooldownSeconds: Number(item?.cooldown_seconds ?? item?.cooldownSeconds ?? 900),
    severity: String(item?.severity || "warning").toLowerCase(),
    enabled: Boolean(item?.enabled ?? true),
    createdAt: item?.created_at || item?.createdAt || "",
    updatedAt: item?.updated_at || item?.updatedAt || "",
  };
}

function catalogQuery(params?: Record<string, string | number | boolean | undefined>): string {
  if (!params) {
    return "";
  }
  const searchParams = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value === undefined || value === null || value === "") return;
    searchParams.set(key, String(value));
  });
  const query = searchParams.toString();
  return query ? `?${query}` : "";
}

async function listCatalog(kind: string, params?: Record<string, string | number | boolean | undefined>): Promise<any[]> {
  const data = await coreFetch(`/api/v1/catalog/${encodeURIComponent(kind)}${catalogQuery(params)}`, { method: "GET" });
  return (data || []).map(mapCatalogItem);
}

async function createCatalog(kind: string, data: any, ownerId?: string, refId?: string): Promise<any> {
  const created = await coreFetch(`/api/v1/catalog/${encodeURIComponent(kind)}`, {
    method: "POST",
    body: JSON.stringify({ owner_id: ownerId || "", ref_id: refId || "", data }),
  });
  return mapCatalogItem(created);
}

async function updateCatalog(kind: string, id: string, data: any): Promise<any> {
  const updated = await coreFetch(`/api/v1/catalog/${encodeURIComponent(kind)}/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify({ data }),
  });
  return mapCatalogItem(updated);
}

async function deleteCatalog(kind: string, id: string): Promise<any> {
  return coreFetch(`/api/v1/catalog/${encodeURIComponent(kind)}/${encodeURIComponent(id)}`, { method: "DELETE" });
}

async function findCaseMeta(caseId: string): Promise<any | null> {
  const items = await listCatalog("case_meta", { ref_id: caseId, limit: 1 });
  return items[0] || null;
}

async function findAlertMeta(alertId: string): Promise<any | null> {
  const items = await listCatalog("alert_meta", { ref_id: alertId, limit: 1 });
  return items[0] || null;
}

async function upsertCaseMeta(caseId: string, data: Record<string, any>): Promise<any> {
  const current = await findCaseMeta(caseId);
  if (current) {
    return updateCatalog("case_meta", current.id, data);
  }
	return createCatalog("case_meta", data, undefined, caseId);
}

async function upsertAlertMeta(alertId: string, data: Record<string, any>): Promise<any> {
  const current = await findAlertMeta(alertId);
  if (current) {
    return updateCatalog("alert_meta", current.id, data);
  }
  return createCatalog("alert_meta", data, undefined, alertId);
}

type PagedCollection<T> = {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
};

export type AssignedFilterMode = "all" | "assigned" | "unassigned" | "mine";
export type ListSortOrder = "asc" | "desc";

type ListAssignmentFilter = {
  assigned?: AssignedFilterMode;
  assignedTo?: string;
  sortBy?: string;
  sortOrder?: ListSortOrder;
  q?: string;
  qNot?: string;
  searchMode?: "plain" | "regex" | "fulltext";
  searchLogic?: "all" | "any";
};

function normalizePagedCollection<T>(payload: any, mapItem: (item: any) => T, fallbackPage: number, fallbackPageSize: number): PagedCollection<T> {
  if (Array.isArray(payload)) {
    const items = payload.map(mapItem);
    const total = items.length;
    return {
      items,
      page: 1,
      pageSize: Math.max(fallbackPageSize, total || fallbackPageSize),
      total,
      totalPages: 1,
    };
  }
  const rawItems = ensureArray(payload?.items ?? payload?.data ?? []);
  const items = rawItems.map(mapItem);
  const total = Number(payload?.total ?? items.length);
  const page = Math.max(1, Number(payload?.page ?? fallbackPage));
  const pageSize = Math.max(1, Number(payload?.page_size ?? payload?.pageSize ?? fallbackPageSize));
  const fallbackTotalPages = Math.ceil(total / pageSize) || 1;
  const totalPages = Math.max(1, Number(payload?.total_pages ?? payload?.totalPages ?? fallbackTotalPages));
  return {
    items,
    page,
    pageSize,
    total,
    totalPages,
  };
}

async function listCasesFromCore(): Promise<any[]> {
  const [cases, metas] = await Promise.all([
    coreFetch("/api/v1/cases", { method: "GET" }),
    listCatalog("case_meta").catch(() => []),
  ]);
  const metaByCase: Record<string, any> = {};
  (metas || []).forEach((item: any) => {
    const key = item.ref_id || item.refId || item.case_id || item.caseId;
    if (key) {
      metaByCase[key] = item;
    }
  });
  return (cases || []).map((item: any) => mapCase(item, metaByCase[item.id]));
}

async function listCasesPageFromCore(page: number, pageSize: number, filters?: ListAssignmentFilter): Promise<PagedCollection<any>> {
  const params = new URLSearchParams();
  params.set("page", String(page));
  params.set("page_size", String(pageSize));
  const assigned = filters?.assigned || "all";
  if (assigned !== "all") {
    params.set("assigned", assigned);
  }
  const assignedTo = String(filters?.assignedTo || "").trim();
  if (assignedTo) {
    params.set("assigned_to", assignedTo);
  }
  const query = String(filters?.q || "").trim();
  if (query) {
    params.set("q", query);
  }
  const excludeQuery = String(filters?.qNot || "").trim();
  if (excludeQuery) {
    params.set("q_not", excludeQuery);
  }
  const searchMode = filters?.searchMode === "regex"
    ? "regex"
    : filters?.searchMode === "fulltext"
      ? "fulltext"
      : filters?.searchMode === "plain"
        ? "plain"
        : "";
  if (searchMode) {
    params.set("search_mode", searchMode);
  }
  const searchLogic = filters?.searchLogic === "any" ? "any" : filters?.searchLogic === "all" ? "all" : "";
  if (searchLogic) {
    params.set("search_logic", searchLogic);
  }
  const sortBy = String(filters?.sortBy || "").trim();
  if (sortBy) {
    params.set("sort_by", sortBy);
  }
  const sortOrder = filters?.sortOrder === "asc" ? "asc" : filters?.sortOrder === "desc" ? "desc" : "";
  if (sortOrder) {
    params.set("sort_order", sortOrder);
  }
  const [payload, metas] = await Promise.all([
    coreFetch(`/api/v1/cases?${params.toString()}`, { method: "GET" }),
    listCatalog("case_meta").catch(() => []),
  ]);
  const metaByCase: Record<string, any> = {};
  (metas || []).forEach((item: any) => {
    const key = item.ref_id || item.refId || item.case_id || item.caseId;
    if (key) {
      metaByCase[key] = item;
    }
  });
  return normalizePagedCollection(payload, (item) => mapCase(item, metaByCase[item?.id]), page, pageSize);
}

async function listAlertsFromCore(): Promise<any[]> {
  const [alerts, metas] = await Promise.all([
    coreFetch("/api/v1/alerts", { method: "GET" }),
    listCatalog("alert_meta").catch(() => []),
  ]);
  const metaByAlert: Record<string, any> = {};
  (metas || []).forEach((item: any) => {
    const key = item.ref_id || item.refId || item.alert_id || item.alertId;
    if (!key) return;
    metaByAlert[key] = item;
  });
  return (alerts || []).map((item: any) => mapAlert(item, metaByAlert[item?.id]));
}

async function listAlertsPageFromCore(page: number, pageSize: number, filters?: ListAssignmentFilter): Promise<PagedCollection<any>> {
  const params = new URLSearchParams();
  params.set("page", String(page));
  params.set("page_size", String(pageSize));
  const assigned = filters?.assigned || "all";
  if (assigned !== "all") {
    params.set("assigned", assigned);
  }
  const assignedTo = String(filters?.assignedTo || "").trim();
  if (assignedTo) {
    params.set("assigned_to", assignedTo);
  }
  const query = String(filters?.q || "").trim();
  if (query) {
    params.set("q", query);
  }
  const excludeQuery = String(filters?.qNot || "").trim();
  if (excludeQuery) {
    params.set("q_not", excludeQuery);
  }
  const searchMode = filters?.searchMode === "regex"
    ? "regex"
    : filters?.searchMode === "fulltext"
      ? "fulltext"
      : filters?.searchMode === "plain"
        ? "plain"
        : "";
  if (searchMode) {
    params.set("search_mode", searchMode);
  }
  const searchLogic = filters?.searchLogic === "any" ? "any" : filters?.searchLogic === "all" ? "all" : "";
  if (searchLogic) {
    params.set("search_logic", searchLogic);
  }
  const sortBy = String(filters?.sortBy || "").trim();
  if (sortBy) {
    params.set("sort_by", sortBy);
  }
  const sortOrder = filters?.sortOrder === "asc" ? "asc" : filters?.sortOrder === "desc" ? "desc" : "";
  if (sortOrder) {
    params.set("sort_order", sortOrder);
  }
  const [payload, metas] = await Promise.all([
    coreFetch(`/api/v1/alerts?${params.toString()}`, { method: "GET" }),
    listCatalog("alert_meta").catch(() => []),
  ]);
  const metaByAlert: Record<string, any> = {};
  (metas || []).forEach((item: any) => {
    const key = item.ref_id || item.refId || item.alert_id || item.alertId;
    if (!key) return;
    metaByAlert[key] = item;
  });
  return normalizePagedCollection(payload, (item) => mapAlert(item, metaByAlert[item?.id]), page, pageSize);
}

async function listUsersForTenant(tenantId: string): Promise<any[]> {
  const data = await coreFetch(`/api/v1/users?tenant_id=${encodeURIComponent(tenantId)}`, { method: "GET" }, false);
  return (data || []).map((u: any) => mapUser(u, tenantId));
}

export { login, logout };

// ── Tenants ──
export function useTenants(enabled = true) {
  return useQuery({
    queryKey: ["tenants"],
    queryFn: async () => {
      const tenants = await coreFetch("/api/v1/tenants", { method: "GET" }, false);
      return (tenants || []).map(mapTenant);
    },
    enabled,
  });
}

// ── Users ──
export function useUsers(tenantId: string) {
  return useQuery({
    queryKey: ["users", tenantId],
    queryFn: () => listUsersForTenant(tenantId),
    enabled: !!tenantId,
  });
}

export function useUser(id: string) {
  return useQuery({
    queryKey: ["user", id],
    queryFn: async () => {
      const user = await coreFetch(`/api/v1/users/${encodeURIComponent(id)}`, { method: "GET" });
      return mapUser(user);
    },
    enabled: !!id,
  });
}

export function useUserCasePerformance(userId: string) {
  return useQuery({
    queryKey: ["userCasePerformance", userId],
    queryFn: async () => {
      const payload = await coreFetch(`/api/v1/users/${encodeURIComponent(userId)}/performance`, { method: "GET" });
      return mapUserCasePerformance(payload);
    },
    enabled: !!userId,
  });
}

export function useUserExperienceEvents(userId: string, limit = 20) {
  const safeLimit = Number.isFinite(limit) ? Math.max(1, Math.min(50, Math.floor(limit))) : 20;
  return useQuery({
    queryKey: ["userExperienceEvents", userId, safeLimit],
    queryFn: async () => {
      const payload = await coreFetch(
        `/api/v1/users/${encodeURIComponent(userId)}/experience-events?limit=${encodeURIComponent(String(safeLimit))}`,
        { method: "GET" },
      );
      return Array.isArray(payload) ? payload.map(mapExperienceEvent) : [];
    },
    enabled: !!userId,
  });
}

type ExperienceEventsPage = {
  items: any[];
  offset: number;
  limit: number;
  hasMore: boolean;
};

async function listUserExperienceEventsPageFromCore(userId: string, limit: number, offset: number): Promise<ExperienceEventsPage> {
  const safeLimit = Number.isFinite(limit) ? Math.max(1, Math.min(50, Math.floor(limit))) : 20;
  const safeOffset = Number.isFinite(offset) ? Math.max(0, Math.floor(offset)) : 0;
  const payload = await coreFetch(
    `/api/v1/users/${encodeURIComponent(userId)}/experience-events?limit=${encodeURIComponent(String(safeLimit))}&offset=${encodeURIComponent(String(safeOffset))}`,
    { method: "GET" },
  );
  const rawItems = Array.isArray(payload) ? payload : ensureArray(payload?.items ?? payload?.data ?? []);
  const items = rawItems.map(mapExperienceEvent);
  const hasMore = typeof payload?.has_more === "boolean"
    ? Boolean(payload.has_more)
    : items.length >= safeLimit;
  return {
    items,
    offset: safeOffset,
    limit: safeLimit,
    hasMore,
  };
}

export function useInfiniteUserExperienceEvents(userId: string, pageSize = 30) {
  const safePageSize = Number.isFinite(pageSize) ? Math.max(1, Math.min(50, Math.floor(pageSize))) : 20;
  return useInfiniteQuery({
    queryKey: ["userExperienceEvents", userId, "infinite", safePageSize],
    initialPageParam: 0,
    queryFn: async ({ pageParam }) => {
      const offset = Number(pageParam ?? 0);
      return listUserExperienceEventsPageFromCore(userId, safePageSize, Number.isFinite(offset) ? offset : 0);
    },
    getNextPageParam: (lastPage, pages) => {
      if (!lastPage.hasMore) {
        return undefined;
      }
      return pages.reduce((sum, page) => sum + page.items.length, 0);
    },
    enabled: !!userId,
  });
}

export function useUpdateUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, data }: { id: string; data: any }) => {
      const payload: any = {
        name: data?.name,
        full_name: data?.full_name,
        email: data?.email,
        team: data?.team,
        avatar_url: data?.avatar_url || data?.avatar,
        cover_image_url: data?.cover_image_url || data?.coverImage || data?.cover_image,
        personal_link: data?.personalLink || data?.personal_link,
        role: data?.role,
        password: data?.password,
        current_password: data?.currentPassword || data?.current_password,
      };
      const updated = await coreFetch(`/api/v1/users/${encodeURIComponent(id)}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }, true);
      return mapUser(updated);
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ["user", vars.id] });
      qc.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

export function useDeleteUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) =>
      coreFetch(`/api/v1/users/${encodeURIComponent(id)}`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["users"] });
      qc.invalidateQueries({ queryKey: ["tenants"] });
    },
  });
}

export function useAwardUserExperience() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: { userId: string; points: number; description: string }) => {
      return coreFetch(`/api/v1/users/${encodeURIComponent(data.userId)}/experience-awards`, {
        method: "POST",
        body: JSON.stringify({
          points: Math.max(1, Math.floor(Number(data.points) || 0)),
          description: String(data.description || "").trim(),
        }),
      });
    },
    onSuccess: (_payload, vars) => {
      qc.invalidateQueries({ queryKey: ["user", vars.userId] });
      qc.invalidateQueries({ queryKey: ["users"] });
      qc.invalidateQueries({ queryKey: ["userExperienceEvents", vars.userId] });
    },
  });
}

export function useUploadUserMedia() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, kind, file }: { id: string; kind: "avatar" | "cover"; file: File }) => {
      const form = new FormData();
      form.set("file", file);
      const payload = await coreFetch(
        `/api/v1/users/${encodeURIComponent(id)}/media/${encodeURIComponent(kind)}/upload`,
        { method: "POST", body: form },
        false,
      );
      return {
        kind: payload.kind,
        url: payload.url || "",
        user: payload.user ? mapUser(payload.user) : null,
      };
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ["user", vars.id] });
      qc.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

// ── Alerts ──
export function useAlerts(tenantId: string) {
  return useQuery({
    queryKey: ["alerts", tenantId],
    queryFn: () => listAlertsFromCore(),
    enabled: !!tenantId,
  });
}

export function useAlertsTotal(tenantId: string) {
  return useQuery({
    queryKey: ["alerts", tenantId, "total"],
    queryFn: async () => {
      const payload = await listAlertsPageFromCore(1, 10);
      return payload.total;
    },
    enabled: !!tenantId,
  });
}

export function useAlertsPage(
  tenantId: string,
  page: number,
  pageSize: number,
  assigned: AssignedFilterMode = "all",
  q = "",
  sortBy = "updated_at",
  sortOrder: ListSortOrder = "desc",
  searchMode: "plain" | "regex" | "fulltext" = "plain",
  searchLogic: "all" | "any" = "all",
  assignedTo = "",
) {
  return useQuery({
    queryKey: [
      "alerts",
      tenantId,
      "page",
      page,
      pageSize,
      "assigned",
      assigned,
      "assignedTo",
      assignedTo,
      "q",
      q,
      "searchMode",
      searchMode,
      "searchLogic",
      searchLogic,
      "sort",
      sortBy,
      sortOrder,
    ],
    queryFn: () => listAlertsPageFromCore(page, pageSize, { assigned, assignedTo, q, searchMode, searchLogic, sortBy, sortOrder }),
    enabled: !!tenantId,
    placeholderData: keepPreviousData,
  });
}

export function useAlert(id: string) {
  return useQuery({
    queryKey: ["alert", id],
    queryFn: async () => {
      const [item, meta] = await Promise.all([
        coreFetch(`/api/v1/alerts/${encodeURIComponent(id)}`, { method: "GET" }),
        findAlertMeta(id).catch(() => null),
      ]);
      return mapAlert(item, meta || undefined);
    },
    enabled: !!id,
  });
}

export function useCreateAlert() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: any) => {
      const created = await coreFetch("/api/v1/alerts", {
        method: "POST",
        body: JSON.stringify({
          title: data?.title,
          description: data?.description || "",
          source: data?.source || "manual",
          status: toCoreAlertStatus(data?.status),
          severity: toCoreSeverity(data?.sev || data?.severity),
          tlp: data?.tlp || "amber",
          pap: data?.pap || "amber",
        }),
      });
      let createdAlertPayload = created;
      if (isAsyncOperationEnvelope(created) && created.status.toLowerCase() === "queued") {
        const operation = await waitForAsyncOperation(created.operation_id, {
          operationType: created.operation_type,
          resource: created.resource || "alert",
          resourceId: created.resource_id,
        });
        const alertID = operation.resource_id || created.resource_id;
        if (!alertID) {
          throw new Error("Async alert creation completed without resource id");
        }
        createdAlertPayload = await coreFetch(`/api/v1/alerts/${encodeURIComponent(alertID)}`, { method: "GET" });
      }
      const alertID = String(createdAlertPayload?.id || "").trim();
      if (alertID && data?.tags !== undefined) {
        await upsertAlertMeta(alertID, { tags: ensureArray(data.tags) }).catch((error: any) => {
          console.warn("alert_meta upsert failed after alert create", error?.message || error);
        });
      }
      const meta = alertID ? await findAlertMeta(alertID).catch(() => null) : null;
      return mapAlert(createdAlertPayload, meta || undefined);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["alerts"] }),
  });
}

export function useUpdateAlert() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, data }: { id: string; data: any }) => {
      const updated = await coreFetch(`/api/v1/alerts/${encodeURIComponent(id)}`, {
        method: "PATCH",
        body: JSON.stringify({
          title: data?.title,
          description: data?.description,
          source: data?.source,
          status: data?.status ? toCoreAlertStatus(data.status) : undefined,
          severity: data?.sev || data?.severity ? toCoreSeverity(data?.sev || data?.severity) : undefined,
          tlp: data?.tlp,
          pap: data?.pap,
          assigned_to: data?.owner || data?.assigned_to,
        }),
      });
      let updatedAlertPayload = updated;
      if (isAsyncOperationEnvelope(updated) && updated.status.toLowerCase() === "queued") {
        const operation = await waitForAsyncOperation(updated.operation_id, {
          operationType: updated.operation_type,
          resource: updated.resource || "alert",
          resourceId: updated.resource_id || id,
        });
        const alertID = operation.resource_id || updated.resource_id || id;
        updatedAlertPayload = await coreFetch(`/api/v1/alerts/${encodeURIComponent(alertID)}`, { method: "GET" });
      }
      if (data?.tags !== undefined) {
        await upsertAlertMeta(id, { tags: ensureArray(data.tags) }).catch((error: any) => {
          console.warn("alert_meta upsert failed after alert patch", error?.message || error);
        });
      }
      const meta = await findAlertMeta(id).catch(() => null);
      return mapAlert(updatedAlertPayload, meta || undefined);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["alerts"] }),
  });
}

export function useDeleteAlert() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      const payload = await coreFetch(`/api/v1/alerts/${encodeURIComponent(id)}`, { method: "DELETE" });
      if (isAsyncOperationEnvelope(payload) && payload.status.toLowerCase() === "queued") {
        await waitForAsyncOperation(payload.operation_id, {
          operationType: payload.operation_type,
          resource: payload.resource || "alert",
          resourceId: payload.resource_id || id,
        });
      }
      return { success: true };
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["alerts"] });
      qc.invalidateQueries({ queryKey: ["alert"] });
      qc.invalidateQueries({ queryKey: ["dashboardStats"] });
    },
  });
}

export function useDeleteAlertsBulk() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (ids: string[]) => {
      const uniqueIDs = Array.from(new Set(ensureArray(ids).map((item: any) => String(item || "").trim()).filter(Boolean)));
      const results = await Promise.allSettled(
        uniqueIDs.map(async (id) => {
          const payload = await coreFetch(`/api/v1/alerts/${encodeURIComponent(id)}`, { method: "DELETE" });
          if (isAsyncOperationEnvelope(payload) && payload.status.toLowerCase() === "queued") {
            await waitForAsyncOperation(payload.operation_id, {
              operationType: payload.operation_type,
              resource: payload.resource || "alert",
              resourceId: payload.resource_id || id,
            });
          }
          return true;
        }),
      );
      const failed = results.filter((item) => item.status === "rejected").length;
      return {
        total: uniqueIDs.length,
        deleted: uniqueIDs.length - failed,
        failed,
      };
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["alerts"] });
      qc.invalidateQueries({ queryKey: ["alert"] });
      qc.invalidateQueries({ queryKey: ["dashboardStats"] });
    },
  });
}

export function useBindAlertsToCase() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: { alertIds: string[]; caseId: string }) =>
      coreFetch("/api/v1/alerts/bulk/link-case", {
        method: "POST",
        body: JSON.stringify({
          alert_ids: data.alertIds,
          case_id: data.caseId,
        }),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["alerts"] });
      qc.invalidateQueries({ queryKey: ["alert"] });
      qc.invalidateQueries({ queryKey: ["cases"] });
      qc.invalidateQueries({ queryKey: ["case"] });
    },
  });
}

export function useCreateCaseFromAlerts() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: { alertIds: string[]; case: any }) => {
      const payload = await coreFetch("/api/v1/alerts/bulk/create-case", {
        method: "POST",
        body: JSON.stringify({
          alert_ids: data.alertIds,
          case: {
            case_number: data.case?.caseNumber || data.case?.case_number,
            title: data.case?.title,
            description: data.case?.description,
            source: data.case?.source,
            incident_type: data.case?.incidentType || data.case?.incident_type,
            status: data.case?.status ? toCoreCaseStatus(data.case.status) : undefined,
            priority: data.case?.priority,
            impact: data.case?.impact,
            confidence: data.case?.confidence,
            severity: data.case?.severity ? toCoreSeverity(data.case.severity) : undefined,
            tlp: data.case?.tlp,
            pap: data.case?.pap,
            detected_at: data.case?.detectedAt || data.case?.detected_at,
            occurred_at: data.case?.occurredAt || data.case?.occurred_at,
            closed_at: data.case?.closedAt || data.case?.closed_at,
            resolution_summary: data.case?.resolutionSummary || data.case?.resolution_summary,
            assigned_to: data.case?.assignee || data.case?.assigned_to,
          },
        }),
      });
      return {
        case: payload?.case ? mapCase(payload.case) : null,
        alerts: ensureArray(payload?.alerts).map(mapAlert),
        updatedCount: Number(payload?.updated_count ?? 0),
      };
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["alerts"] });
      qc.invalidateQueries({ queryKey: ["alert"] });
      qc.invalidateQueries({ queryKey: ["cases"] });
      qc.invalidateQueries({ queryKey: ["case"] });
      qc.invalidateQueries({ queryKey: ["caseTimeline"] });
    },
  });
}

// ── Cases ──
export function useCaseStatuses(tenantId: string) {
  return useQuery({
    queryKey: ["caseStatuses", tenantId],
    queryFn: async () => {
      const payload = await coreFetch("/api/v1/case-statuses", { method: "GET" }, true, tenantId);
      const items = ensureArray(payload?.statuses || payload);
      return items.map(mapCaseStatus);
    },
    enabled: !!tenantId,
  });
}

export function useSaveCaseStatuses() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: any[] | { tenantId?: string; statuses: any[] }) => {
      const tenantId = Array.isArray(input) ? undefined : input?.tenantId;
      const statuses = Array.isArray(input) ? input : input?.statuses;
      return coreFetch("/api/v1/case-statuses", {
        method: "PUT",
        body: JSON.stringify({
          statuses: ensureArray(statuses).map((item: any, index: number) => ({
            code: toCoreCaseStatus(item?.code),
            label: (item?.label || "").toString().trim(),
            order: Number(item?.order ?? (index + 1) * 10),
            is_closed: Boolean(item?.isClosed ?? item?.is_closed),
            color: (item?.color || "").toString().trim(),
          })),
        }),
      }, true, tenantId);
    },
    onSuccess: (_result, input) => {
      const tenantId = Array.isArray(input) ? undefined : input?.tenantId;
      if (tenantId) {
        qc.invalidateQueries({ queryKey: ["caseStatuses", tenantId] });
      }
      qc.invalidateQueries({ queryKey: ["caseStatuses"] });
      qc.invalidateQueries({ queryKey: ["cases"] });
      qc.invalidateQueries({ queryKey: ["case"] });
    },
  });
}

export function useCases(tenantId: string) {
  return useQuery({
    queryKey: ["cases", tenantId],
    queryFn: () => listCasesFromCore(),
    enabled: !!tenantId,
  });
}

export function useCasesTotal(tenantId: string) {
  return useQuery({
    queryKey: ["cases", tenantId, "total"],
    queryFn: async () => {
      const payload = await listCasesPageFromCore(1, 10);
      return payload.total;
    },
    enabled: !!tenantId,
  });
}

export function useCasesPage(
  tenantId: string,
  page: number,
  pageSize: number,
  assigned: AssignedFilterMode = "all",
  q = "",
  sortBy = "updated_at",
  sortOrder: ListSortOrder = "desc",
  qNot = "",
  searchMode: "plain" | "regex" | "fulltext" = "plain",
  searchLogic: "all" | "any" = "all",
  assignedTo = "",
) {
  return useQuery({
    queryKey: [
      "cases",
      tenantId,
      "page",
      page,
      pageSize,
      "assigned",
      assigned,
      "assignedTo",
      assignedTo,
      "q",
      q,
      "qNot",
      qNot,
      "searchMode",
      searchMode,
      "searchLogic",
      searchLogic,
      "sort",
      sortBy,
      sortOrder,
    ],
    queryFn: () => listCasesPageFromCore(page, pageSize, { assigned, assignedTo, q, qNot, searchMode, searchLogic, sortBy, sortOrder }),
    enabled: !!tenantId,
    placeholderData: keepPreviousData,
  });
}

export function useCasesSummaries(tenantId: string, caseIds: string[]) {
  const normalizedCaseIds = Array.from(new Set(ensureArray(caseIds).map((item: any) => String(item || "").trim()).filter(Boolean)));
  const queryCaseIDs = normalizedCaseIds.join(",");
  return useQuery({
    queryKey: ["cases", tenantId, "summaries", queryCaseIDs],
    queryFn: async () => {
      if (!queryCaseIDs) return [];
      const payload = await coreFetch(`/api/v1/cases/summary?case_ids=${encodeURIComponent(queryCaseIDs)}`, { method: "GET" });
      return ensureArray(payload).map(mapCaseListSummary);
    },
    enabled: !!tenantId && normalizedCaseIds.length > 0,
  });
}

export function useActivityLivestream(
  tenantId: string,
  options?: {
    q?: string;
    types?: string[];
    assigneeId?: string;
    since?: string;
    limit?: number;
    offset?: number;
    refetchInterval?: number;
    enabled?: boolean;
    refetchOnWindowFocus?: boolean;
  },
) {
  const normalizedTypes = Array.from(
    new Set(
      ensureArray(options?.types)
        .map((item: any) => String(item || "").trim().toLowerCase())
        .filter((item: string) => item === "case" || item === "alert" || item === "task"),
    ),
  );
  const q = String(options?.q || "").trim();
  const assigneeId = String(options?.assigneeId || "").trim();
  const since = String(options?.since || "").trim();
  const limitRaw = Number(options?.limit ?? 100);
  const limit = Number.isFinite(limitRaw) ? Math.max(1, Math.min(500, Math.floor(limitRaw))) : 100;
  const offsetRaw = Number(options?.offset ?? 0);
  const offset = Number.isFinite(offsetRaw) ? Math.max(0, Math.floor(offsetRaw)) : 0;
  const refetchInterval = Number(options?.refetchInterval ?? 3000);
  const enabled = options?.enabled ?? true;
  const refetchOnWindowFocus = options?.refetchOnWindowFocus ?? true;
  return useQuery({
    queryKey: ["activityLivestream", tenantId, q, normalizedTypes.join(","), assigneeId, since, limit, offset],
    queryFn: async () => {
      const params = new URLSearchParams();
      params.set("limit", String(limit));
      if (offset > 0) {
        params.set("offset", String(offset));
      }
      if (q) {
        params.set("q", q);
      }
      if (assigneeId) {
        params.set("assignee_id", assigneeId);
      }
      if (since) {
        params.set("since", since);
      }
      if (normalizedTypes.length > 0) {
        params.set("types", normalizedTypes.join(","));
      }
      const payload = await coreFetch(`/api/v1/activity/livestream?${params.toString()}`, { method: "GET" });
      const items = ensureArray(payload?.items).map(mapActivityLivestreamItem);
      const payloadLimit = Number(payload?.limit ?? limit);
      const payloadOffset = Number(payload?.offset ?? offset);
      const hasMoreRaw = payload?.has_more ?? payload?.hasMore;
      return {
        items,
        generatedAt: String(payload?.generated_at || payload?.generatedAt || ""),
        limit: Number.isFinite(payloadLimit) ? Math.max(1, Math.floor(payloadLimit)) : limit,
        offset: Number.isFinite(payloadOffset) ? Math.max(0, Math.floor(payloadOffset)) : offset,
        hasMore:
          typeof hasMoreRaw === "boolean"
            ? hasMoreRaw
            : Number.isFinite(payloadOffset) && Number.isFinite(payloadLimit)
              ? items.length >= Number(payloadLimit)
              : false,
      };
    },
    enabled: !!tenantId && enabled,
    refetchInterval: refetchInterval > 0 ? refetchInterval : false,
    refetchOnWindowFocus,
    placeholderData: keepPreviousData,
  });
}

export function useCase(id: string, options?: { refetchInterval?: number }) {
  const refetchInterval = Number(options?.refetchInterval ?? 0);
  return useQuery({
    queryKey: ["case", id],
    queryFn: async () => {
      const [item, meta] = await Promise.all([
        coreFetch(`/api/v1/cases/${encodeURIComponent(id)}`, { method: "GET" }),
        findCaseMeta(id).catch(() => null),
      ]);
      return mapCase(item, meta || undefined);
    },
    enabled: !!id,
    refetchInterval: refetchInterval > 0 ? refetchInterval : false,
    placeholderData: keepPreviousData,
  });
}

export function useCaseRelatedCases(caseId: string, linkBy: string = "observables") {
  const normalizedLinkBy = String(linkBy || "").trim().toLowerCase() || "observables";
  return useQuery({
    queryKey: ["caseRelatedCases", caseId, normalizedLinkBy],
    queryFn: async () => {
      const params = new URLSearchParams();
      params.set("link_by", normalizedLinkBy);
      const payload = await coreFetch(`/api/v1/cases/${encodeURIComponent(caseId)}/related?${params.toString()}`, {
        method: "GET",
      });
      return {
        linkBy: String(payload?.link_by || normalizedLinkBy).trim().toLowerCase() || "observables",
        activeRecent: ensureArray(payload?.active_recent).map(mapRelatedCase),
        allTime: ensureArray(payload?.all_time).map(mapRelatedCase),
      };
    },
    enabled: !!caseId,
  });
}

export function useCreateCase() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: any) => {
      const createdPayload = await coreFetch("/api/v1/cases", {
        method: "POST",
        body: JSON.stringify({
          case_number: data?.caseNumber || data?.case_number,
          title: data?.title,
          description: data?.description || "",
          source: data?.source || "manual",
          incident_type: data?.incidentType || data?.incident_type || "",
          status: data?.status ? toCoreCaseStatus(data?.status) : undefined,
          priority: (data?.priority || "medium").toString().toLowerCase(),
          impact: data?.impact || "",
          confidence: Number(data?.confidence || 0),
          severity: toCoreSeverity(data?.sev || data?.severity),
          tlp: data?.tlp || "amber",
          pap: data?.pap || "amber",
          detected_at: data?.detectedAt || data?.detected_at || "",
          occurred_at: data?.occurredAt || data?.occurred_at || "",
          closed_at: data?.closedAt || data?.closed_at || "",
          resolution_summary: data?.resolutionSummary || data?.resolution_summary || "",
          assigned_to: data?.assignee || data?.assigned_to || "",
        }),
      });

      let resolvedCasePayload = createdPayload;
      if (isAsyncOperationEnvelope(createdPayload) && createdPayload.status.toLowerCase() === "queued") {
        const operation = await waitForAsyncOperation(createdPayload.operation_id, {
          operationType: createdPayload.operation_type,
          resource: createdPayload.resource || "case",
          resourceId: createdPayload.resource_id,
        });
        const caseID = operation.resource_id || createdPayload.resource_id;
        if (!caseID) {
          throw new Error("Async case creation completed without resource id");
        }
        resolvedCasePayload = await coreFetch(`/api/v1/cases/${encodeURIComponent(caseID)}`, { method: "GET" });
      }

      const caseID = String(resolvedCasePayload?.id || "").trim();
      if (caseID) {
        const caseMetaPatch: Record<string, any> = {};
        if (data?.tags !== undefined) {
          caseMetaPatch.tags = ensureArray(data.tags);
        }
        if (data?.customFields !== undefined || data?.custom_fields !== undefined) {
          const nextCustomFields = data?.customFields ?? data?.custom_fields;
          caseMetaPatch.customFields = normalizeCustomFields(nextCustomFields);
          caseMetaPatch.custom_fields = normalizeCustomFields(nextCustomFields);
        }
        if (Object.keys(caseMetaPatch).length > 0) {
          await upsertCaseMeta(caseID, caseMetaPatch).catch((error: any) => {
            console.warn("case_meta upsert failed after case create", error?.message || error);
          });
        }
        const meta = await findCaseMeta(caseID).catch(() => null);
        return mapCase(resolvedCasePayload, meta || undefined);
      }

      return mapCase(resolvedCasePayload);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["cases"] }),
  });
}

export function useUpdateCase() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, data }: { id: string; data: any }) => {
      const assignedToValue = data?.assignee !== undefined ? data.assignee : data?.assigned_to;
      const updated = await coreFetch(`/api/v1/cases/${encodeURIComponent(id)}`, {
        method: "PATCH",
        body: JSON.stringify({
          case_number: data?.caseNumber || data?.case_number,
          title: data?.title,
          description: data?.description,
          source: data?.source,
          incident_type: data?.incidentType || data?.incident_type,
          status: data?.status ? toCoreCaseStatus(data.status) : undefined,
          priority: data?.priority ? String(data.priority).toLowerCase() : undefined,
          impact: data?.impact,
          confidence: data?.confidence !== undefined ? Number(data.confidence) : undefined,
          severity: data?.sev || data?.severity ? toCoreSeverity(data?.sev || data?.severity) : undefined,
          tlp: data?.tlp,
          pap: data?.pap,
          detected_at: data?.detectedAt || data?.detected_at,
          occurred_at: data?.occurredAt || data?.occurred_at,
          closed_at: data?.closedAt || data?.closed_at,
          expected_updated_at: data?.expectedUpdatedAt || data?.expected_updated_at,
          resolution_summary: data?.resolutionSummary || data?.resolution_summary,
          assigned_to: assignedToValue,
        }),
      });
      let updatedCasePayload = updated;
      if (isAsyncOperationEnvelope(updated) && updated.status.toLowerCase() === "queued") {
        const operation = await waitForAsyncOperation(updated.operation_id, {
          operationType: updated.operation_type,
          resource: updated.resource || "case",
          resourceId: updated.resource_id || id,
        });
        const caseID = operation.resource_id || updated.resource_id || id;
        updatedCasePayload = await coreFetch(`/api/v1/cases/${encodeURIComponent(caseID)}`, { method: "GET" });
      }

      const caseMetaPatch: Record<string, any> = {};
      if (data?.forumId !== undefined) caseMetaPatch.forumId = data.forumId;
      if (data?.tactics !== undefined) caseMetaPatch.tactics = data.tactics;
      if (data?.techniques !== undefined) caseMetaPatch.techniques = data.techniques;
      if (data?.verdict !== undefined) caseMetaPatch.verdict = data.verdict;
      if (data?.recommendations !== undefined) caseMetaPatch.recommendations = data.recommendations;
      if (data?.stage !== undefined) caseMetaPatch.stage = data.stage;
      if (data?.category !== undefined) caseMetaPatch.category = data.category;
      if (data?.relatedProduct !== undefined || data?.related_product !== undefined) {
        caseMetaPatch.relatedProduct = data?.relatedProduct ?? data?.related_product;
        caseMetaPatch.related_product = data?.relatedProduct ?? data?.related_product;
      }
      if (data?.tags !== undefined) caseMetaPatch.tags = ensureArray(data.tags);
      if (data?.owner !== undefined) caseMetaPatch.owner = data.owner || "";
      if (data?.customFields !== undefined || data?.custom_fields !== undefined) {
        const nextCustomFields = data?.customFields ?? data?.custom_fields;
        caseMetaPatch.customFields = nextCustomFields;
        caseMetaPatch.custom_fields = nextCustomFields;
      }

      if (Object.keys(caseMetaPatch).length > 0) {
        try {
          await upsertCaseMeta(id, caseMetaPatch);
        } catch (error: any) {
          // Core case fields were already updated. Keep edit flow usable even if metadata sync fails.
          console.warn("case_meta upsert failed after case patch", error?.message || error);
        }
      }

      const meta = await findCaseMeta(id).catch(() => null);
      return mapCase(updatedCasePayload, meta || undefined);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["cases"] });
      qc.invalidateQueries({ queryKey: ["case"] });
      qc.invalidateQueries({ queryKey: ["forum"] });
    },
  });
}

export function useCopyCase() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, data }: { id: string; data?: any }) => {
      const payload = await coreFetch(`/api/v1/cases/${encodeURIComponent(id)}/copy`, {
        method: "POST",
        body: JSON.stringify({
          title: data?.title,
          case_number: data?.caseNumber || data?.case_number,
          assigned_to: data?.assignee || data?.assigned_to,
          include_observables: data?.includeObservables ?? data?.include_observables ?? true,
        }),
      });

      let copiedCasePayload = payload;
      if (isAsyncOperationEnvelope(payload) && payload.status.toLowerCase() === "queued") {
        const operation = await waitForAsyncOperation(payload.operation_id, {
          operationType: payload.operation_type,
          resource: payload.resource || "case",
          resourceId: payload.resource_id,
        });
        const caseID = operation.resource_id || payload.resource_id;
        if (!caseID) {
          throw new Error("Async case copy completed without resource id");
        }
        copiedCasePayload = await coreFetch(`/api/v1/cases/${encodeURIComponent(caseID)}`, { method: "GET" });
      }

      const copiedCaseID = String(copiedCasePayload?.id || "").trim();
      if (copiedCaseID) {
        const caseMetaPatch: Record<string, any> = {};
        if (data?.tags !== undefined) {
          caseMetaPatch.tags = ensureArray(data.tags);
        }
        if (data?.customFields !== undefined || data?.custom_fields !== undefined) {
          const nextCustomFields = data?.customFields ?? data?.custom_fields;
          caseMetaPatch.customFields = normalizeCustomFields(nextCustomFields);
          caseMetaPatch.custom_fields = normalizeCustomFields(nextCustomFields);
        }
        if (data?.owner !== undefined) {
          caseMetaPatch.owner = data.owner || "";
        }
        if (Object.keys(caseMetaPatch).length > 0) {
          await upsertCaseMeta(copiedCaseID, caseMetaPatch).catch((error: any) => {
            console.warn("case_meta upsert failed after case copy", error?.message || error);
          });
        }
      }

      const meta = copiedCaseID ? await findCaseMeta(copiedCaseID).catch(() => null) : null;
      return mapCase(copiedCasePayload, meta || undefined);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["cases"] });
      qc.invalidateQueries({ queryKey: ["dashboardStats"] });
      qc.invalidateQueries({ queryKey: ["caseTimeline"] });
    },
  });
}

export function useDeleteCase() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      const payload = await coreFetch(`/api/v1/cases/${encodeURIComponent(id)}`, { method: "DELETE" });
      if (isAsyncOperationEnvelope(payload) && payload.status.toLowerCase() === "queued") {
        await waitForAsyncOperation(payload.operation_id, {
          operationType: payload.operation_type,
          resource: payload.resource || "case",
          resourceId: payload.resource_id || id,
        });
      }
      return { success: true };
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["cases"] });
      qc.invalidateQueries({ queryKey: ["case"] });
      qc.invalidateQueries({ queryKey: ["alerts"] });
      qc.invalidateQueries({ queryKey: ["alert"] });
      qc.invalidateQueries({ queryKey: ["dashboardStats"] });
      qc.invalidateQueries({ queryKey: ["caseTimeline"] });
    },
  });
}

export function useEscalateCase() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ caseId, data }: { caseId: string; data: any }) =>
      coreFetch(`/api/v1/cases/${encodeURIComponent(caseId)}/escalate`, {
        method: "POST",
        body: JSON.stringify({
          target_tenant_id: data?.targetTenantId || data?.target_tenant_id || "",
          target_tenant_slug: data?.targetTenantSlug || data?.target_tenant_slug || "",
          target_assignee_id: data?.targetAssigneeId || data?.target_assignee_id || "",
          handoff_type: data?.handoffType || data?.handoff_type || "employee_client",
          summary: data?.summary || "",
          include_observables: data?.includeObservables ?? data?.include_observables ?? true,
        }),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["cases"] });
      qc.invalidateQueries({ queryKey: ["case"] });
      qc.invalidateQueries({ queryKey: ["caseTimeline"] });
    },
  });
}

export function useCaseShares(caseId: string) {
  return useQuery({
    queryKey: ["caseShares", caseId],
    queryFn: async () => {
      const payload = await coreFetch(`/api/v1/cases/${encodeURIComponent(caseId)}/shares`, {
        method: "GET",
      });
      return ensureArray(payload?.shares);
    },
    enabled: !!caseId,
  });
}

export function useShareCaseAcrossTenants() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ caseId, data }: { caseId: string; data: any }) =>
      coreFetch(`/api/v1/cases/${encodeURIComponent(caseId)}/share`, {
        method: "POST",
        body: JSON.stringify({
          target_tenant_id: data?.targetTenantId || data?.target_tenant_id || "",
          target_tenant_slug: data?.targetTenantSlug || data?.target_tenant_slug || "",
        }),
      }),
    onSuccess: (_payload, vars) => {
      qc.invalidateQueries({ queryKey: ["cases"] });
      qc.invalidateQueries({ queryKey: ["case", vars.caseId] });
      qc.invalidateQueries({ queryKey: ["caseShares", vars.caseId] });
    },
  });
}

export function useDeleteCasesBulk() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (ids: string[]) => {
      const uniqueIDs = Array.from(new Set(ensureArray(ids).map((item: any) => String(item || "").trim()).filter(Boolean)));
      const results = await Promise.allSettled(
        uniqueIDs.map(async (id) => {
          const payload = await coreFetch(`/api/v1/cases/${encodeURIComponent(id)}`, { method: "DELETE" });
          if (isAsyncOperationEnvelope(payload) && payload.status.toLowerCase() === "queued") {
            await waitForAsyncOperation(payload.operation_id, {
              operationType: payload.operation_type,
              resource: payload.resource || "case",
              resourceId: payload.resource_id || id,
            });
          }
          return true;
        }),
      );
      const failed = results.filter((item) => item.status === "rejected").length;
      return {
        total: uniqueIDs.length,
        deleted: uniqueIDs.length - failed,
        failed,
      };
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["cases"] });
      qc.invalidateQueries({ queryKey: ["case"] });
      qc.invalidateQueries({ queryKey: ["alerts"] });
      qc.invalidateQueries({ queryKey: ["alert"] });
      qc.invalidateQueries({ queryKey: ["dashboardStats"] });
      qc.invalidateQueries({ queryKey: ["caseTimeline"] });
    },
  });
}

// ── Forum ──
function mapForumProxyProfile(profile: any) {
  const metadata = profile?.metadata && typeof profile.metadata === "object" ? profile.metadata : {};
  return {
    id: String(profile?.id || "").trim(),
    connectorId: String(profile?.connector_id || profile?.connectorId || "").trim(),
    name: String(profile?.name || "").trim(),
    bindingKey: String(profile?.binding_key || profile?.bindingKey || "").trim(),
    metadata,
    createdAt: String(profile?.created_at || profile?.createdAt || "").trim(),
    updatedAt: String(profile?.updated_at || profile?.updatedAt || "").trim(),
    lastSyncedAt: String(profile?.last_synced_at || profile?.lastSyncedAt || "").trim(),
    hasBinding: Boolean(profile?.has_binding ?? profile?.hasBinding),
    conversationId: String(profile?.conversation_id || profile?.conversationId || "").trim(),
    cursorPresent: Boolean(profile?.cursor_present ?? profile?.cursorPresent),
  };
}

export function useForumThreads(tenantId: string) {
  return useQuery({
    queryKey: ["forum", tenantId],
    queryFn: async () => {
      const threads = await coreFetch("/api/v1/forum/threads", { method: "GET" });
      return (threads || []).map((item: any) => ({
        id: item.id,
        caseId: item.case_id || item.caseId,
        title: item.title || "Thread",
        status: item.status || "In Progress",
        tenantId: item.tenant_id || item.tenantId,
        postsCount: Number(item.posts_count ?? item.postsCount ?? 0) || 0,
        lastPostAt: String(item.last_post_at || item.lastPostAt || item.last_activity_at || item.lastActivityAt || ""),
        lastPostPreview: String(item.last_post_preview || item.lastPostPreview || ""),
        lastPostAuthorID: String(item.last_post_author_id || item.lastPostAuthorId || ""),
        lastPostAuthorName: String(item.last_post_author_name || item.lastPostAuthorName || ""),
      }));
    },
    enabled: !!tenantId,
  });
}

export function useForumThread(id: string) {
  return useQuery({
    queryKey: ["forumThread", id],
    queryFn: async () => {
      const thread = await coreFetch(`/api/v1/forum/threads/${encodeURIComponent(id)}`, { method: "GET" });
      const posts = ensureArray(thread.posts).map((p: any) => ({
        id: p.id,
        threadId: p.thread_id || p.threadId,
        authorId: p.author_id || p.authorId,
        authorName: p.author_name || p.authorName || "",
        content: p.content,
        timestamp: p.timestamp || p.created_at || p.createdAt,
        attachments: ensureArray(p.attachments).map(mapForumAttachment),
      }));
      posts.sort((left: any, right: any) => compareChronological(left.timestamp, right.timestamp, left.id, right.id));
      return {
        id: thread.id,
        caseId: thread.case_id || thread.caseId,
        title: thread.title,
        status: thread.status,
        tenantId: thread.tenant_id,
        postsCount: Number(thread.posts_count ?? thread.postsCount ?? posts.length) || posts.length,
        lastPostAt: String(thread.last_post_at || thread.lastPostAt || thread.last_activity_at || thread.lastActivityAt || ""),
        proxyProfiles: ensureArray(thread.proxy_profiles || thread.proxyProfiles).map(mapForumProxyProfile).filter((item: any) => item.id),
        posts,
      };
    },
    enabled: !!id,
  });
}

export function useForumProxyProfiles(threadId: string) {
  return useQuery({
    queryKey: ["forumProxyProfiles", threadId],
    queryFn: async () => {
      const payload = await coreFetch(`/api/v1/forum/threads/${encodeURIComponent(threadId)}/proxy/profiles`, { method: "GET" });
      return ensureArray(payload?.profiles).map(mapForumProxyProfile).filter((item: any) => item.id);
    },
    enabled: !!threadId,
  });
}

export function useUpsertForumProxyProfile() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: any) => {
      const threadId = data?.threadId || data?.thread_id;
      const payload = await coreFetch(`/api/v1/forum/threads/${encodeURIComponent(threadId)}/proxy/profiles`, {
        method: "POST",
        body: JSON.stringify({
          id: data?.id || "",
          connector_id: data?.connectorId || data?.connector_id || "",
          name: data?.name || "",
          binding_key: data?.bindingKey || data?.binding_key || "",
          metadata: data?.metadata || {},
        }),
      });
      return payload;
    },
    onSuccess: (_payload, vars) => {
      const threadId = vars?.threadId || vars?.thread_id;
      qc.invalidateQueries({ queryKey: ["forumProxyProfiles", threadId] });
      qc.invalidateQueries({ queryKey: ["forumThread", threadId] });
      qc.invalidateQueries({ queryKey: ["forum"] });
    },
  });
}

export function useDeleteForumProxyProfile() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: any) => {
      const threadId = data?.threadId || data?.thread_id;
      const profileId = data?.profileId || data?.profile_id;
      const payload = await coreFetch(
        `/api/v1/forum/threads/${encodeURIComponent(threadId)}/proxy/profiles/${encodeURIComponent(profileId)}`,
        { method: "DELETE" },
      );
      return payload;
    },
    onSuccess: (_payload, vars) => {
      const threadId = vars?.threadId || vars?.thread_id;
      qc.invalidateQueries({ queryKey: ["forumProxyProfiles", threadId] });
      qc.invalidateQueries({ queryKey: ["forumThread", threadId] });
      qc.invalidateQueries({ queryKey: ["forum"] });
    },
  });
}

export function useCreateForumThread() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: any) => {
      const created = await coreFetch("/api/v1/forum/threads", {
        method: "POST",
        body: JSON.stringify({
          case_id: data?.caseId || data?.case_id,
          title: data?.title,
          status: data?.status,
          initial_message: data?.initialMessage || data?.initial_message || "",
          author_id: data?.authorId || data?.author_id || "",
        }),
      });
      const caseId = created.case_id || created.caseId;
      if (caseId) {
        await upsertCaseMeta(caseId, { forumId: created.id, forum_id: created.id }).catch(() => undefined);
      }
      return {
        id: created.id,
        caseId,
        title: created.title,
        status: created.status,
        tenantId: created.tenant_id,
        postsCount: Number(created.posts_count ?? created.postsCount ?? 0) || 0,
        lastPostAt: String(created.last_post_at || created.lastPostAt || created.last_activity_at || created.lastActivityAt || ""),
      };
    },
    onSuccess: (created) => {
      if (created?.caseId) {
        qc.setQueryData(["case", created.caseId], (prev: any) => {
          if (!prev) {
            return prev;
          }
          return { ...prev, forumId: created.id };
        });
      }
      qc.invalidateQueries({ queryKey: ["forum"] });
      qc.invalidateQueries({ queryKey: ["cases"] });
      qc.invalidateQueries({ queryKey: ["case"] });
    },
  });
}

export function useCreateForumPost() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: any) => {
      const created = await coreFetch("/api/v1/forum/posts", {
        method: "POST",
        body: JSON.stringify({
          thread_id: data?.threadId || data?.thread_id,
          author_id: data?.authorId || data?.author_id,
          content: data?.content,
        }),
      });
      return {
        id: created.id,
        threadId: created.thread_id,
        authorId: created.author_id,
        authorName: created.author_name || created.authorName || "",
        content: created.content,
        timestamp: created.timestamp || created.created_at,
        attachments: ensureArray(created.attachments).map(mapForumAttachment),
      };
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ["forumThread", vars.threadId] });
    },
  });
}

export function useCreateForumPostWithAttachments() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: { threadId: string; content?: string; authorId?: string; files: File[] }) => {
      const form = new FormData();
      if (data?.content) {
        form.set("content", data.content);
      }
      if (data?.authorId) {
        form.set("author_id", data.authorId);
      }
      ensureArray(data?.files).forEach((file: File) => {
        form.append("files", file);
      });
      const created = await coreFetch(`/api/v1/forum/threads/${encodeURIComponent(data.threadId)}/posts/upload`, {
        method: "POST",
        body: form,
      });
      return {
        id: created.id,
        threadId: created.thread_id || created.threadId || data.threadId,
        authorId: created.author_id || created.authorId || data.authorId || "",
        authorName: created.author_name || created.authorName || "",
        content: created.content || "",
        timestamp: created.timestamp || created.created_at || created.createdAt,
        attachments: ensureArray(created.attachments).map(mapForumAttachment),
      };
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ["forumThread", vars.threadId] });
      qc.invalidateQueries({ queryKey: ["forumProxyProfiles", vars.threadId] });
      qc.invalidateQueries({ queryKey: ["forum"] });
    },
  });
}

export function useProxyForumSend() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: any) => {
      const threadId = data?.threadId || data?.thread_id;
      const payload = await coreFetch(`/api/v1/forum/threads/${encodeURIComponent(threadId)}/proxy/send`, {
        method: "POST",
        body: JSON.stringify({
          connector_id: data?.connectorId || data?.connector_id,
          profile_id: data?.profileId || data?.profile_id || "",
          binding_key: data?.bindingKey || data?.binding_key || "",
          content: data?.content,
          author: data?.author,
          metadata: data?.metadata || {},
        }),
      });
      return payload;
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ["forumThread", vars.threadId] });
      qc.invalidateQueries({ queryKey: ["forumProxyProfiles", vars.threadId] });
      qc.invalidateQueries({ queryKey: ["forum"] });
    },
  });
}

export function useProxyForumSync() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: any) => {
      const threadId = data?.threadId || data?.thread_id;
      const payload = await coreFetch(`/api/v1/forum/threads/${encodeURIComponent(threadId)}/proxy/sync`, {
        method: "POST",
        body: JSON.stringify({
          connector_id: data?.connectorId || data?.connector_id,
          profile_id: data?.profileId || data?.profile_id || "",
          binding_key: data?.bindingKey || data?.binding_key || "",
          metadata: data?.metadata || {},
        }),
      });
      return payload;
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ["forumThread", vars.threadId] });
      qc.invalidateQueries({ queryKey: ["forumProxyProfiles", vars.threadId] });
      qc.invalidateQueries({ queryKey: ["forum"] });
    },
  });
}

// ── Case Communications ──
export function useCaseCommunications(caseId: string) {
  return useQuery({
    queryKey: ["caseCommunications", caseId],
    queryFn: async () => {
      const items = await coreFetch(`/api/v1/cases/${encodeURIComponent(caseId)}/communications`, { method: "GET" });
      return ensureArray(items).map(mapCaseCommunicationThread);
    },
    enabled: !!caseId,
  });
}

export function useCaseCommunicationConnectors(tenantId: string) {
  return useQuery({
    queryKey: ["caseCommunicationConnectors", tenantId],
    queryFn: async () => {
      const items = await coreFetch("/api/v1/communications/connectors", { method: "GET" }, true, tenantId);
      return ensureArray(items).map((item: any) => {
        const base = mapCatalogItem(item);
        const data = item?.data && typeof item.data === "object" ? item.data : {};
        return {
          ...base,
          ...data,
          name: String(data?.name || item?.name || item?.title || "").trim(),
          description: String(data?.description || item?.description || "").trim(),
          enabled: boolFromUnknown(data?.enabled ?? item?.enabled, true),
          channel: String(data?.channel || item?.channel || "").trim().toLowerCase(),
          type: String(data?.type || item?.type || data?.channel || item?.channel || "").trim(),
          category: String(data?.category || item?.category || "standard").trim().toLowerCase(),
          communicationMode: String(data?.communication_mode || data?.communicationMode || item?.communication_mode || item?.communicationMode || "").trim().toLowerCase(),
          capabilities: stringSliceFromUnknown(data?.capabilities || item?.capabilities),
          config: data?.config && typeof data.config === "object" ? data.config : (item?.config && typeof item.config === "object" ? item.config : {}),
        };
      });
    },
    enabled: !!tenantId,
  });
}

export function useCaseCommunication(caseId: string, threadId: string) {
  return useQuery({
    queryKey: ["caseCommunication", caseId, threadId],
    queryFn: async () => {
      const item = await coreFetch(`/api/v1/cases/${encodeURIComponent(caseId)}/communications/${encodeURIComponent(threadId)}`, { method: "GET" });
      return mapCaseCommunicationThread(item);
    },
    enabled: !!caseId && !!threadId,
  });
}

export function useCreateCaseCommunication() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: any) => {
      const caseId = data?.caseId || data?.case_id;
      const payload = await coreFetch(`/api/v1/cases/${encodeURIComponent(caseId)}/communications`, {
        method: "POST",
        body: JSON.stringify({
          title: data?.title || "",
          status: data?.status || "",
          channel: data?.channel || "",
          connector_id: data?.connectorId || data?.connector_id || "",
          participant: data?.participant || {},
          metadata: data?.metadata || {},
        }),
      });
      return mapCaseCommunicationThread(payload);
    },
    onSuccess: (_, vars) => {
      const caseId = vars?.caseId || vars?.case_id;
      qc.invalidateQueries({ queryKey: ["caseCommunications", caseId] });
      qc.invalidateQueries({ queryKey: ["case", caseId] });
    },
  });
}

export function useSendCaseCommunicationMessage() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: any) => {
      const caseId = data?.caseId || data?.case_id;
      const threadId = data?.threadId || data?.thread_id;
      const payload = await coreFetch(`/api/v1/cases/${encodeURIComponent(caseId)}/communications/${encodeURIComponent(threadId)}/messages`, {
        method: "POST",
        body: JSON.stringify({
          connector_id: data?.connectorId || data?.connector_id || "",
          content: data?.content || "",
          author: data?.author || "",
          metadata: data?.metadata || {},
          template_id: data?.templateId || data?.template_id || "",
          template_vars: data?.templateVars || data?.template_vars || {},
          subject: data?.subject || "",
          notify_managers: data?.notifyManagers ?? data?.notify_managers,
        }),
      });
      return payload;
    },
    onSuccess: (_, vars) => {
      const caseId = vars?.caseId || vars?.case_id;
      const threadId = vars?.threadId || vars?.thread_id;
      qc.invalidateQueries({ queryKey: ["caseCommunication", caseId, threadId] });
      qc.invalidateQueries({ queryKey: ["caseCommunications", caseId] });
      qc.invalidateQueries({ queryKey: ["case", caseId] });
    },
  });
}

export function useSyncCaseCommunication() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: any) => {
      const caseId = data?.caseId || data?.case_id;
      const threadId = data?.threadId || data?.thread_id;
      const payload = await coreFetch(`/api/v1/cases/${encodeURIComponent(caseId)}/communications/${encodeURIComponent(threadId)}/sync`, {
        method: "POST",
        body: JSON.stringify({
          connector_id: data?.connectorId || data?.connector_id || "",
          subject: data?.subject || "",
        }),
      });
      return payload;
    },
    onSuccess: (_, vars) => {
      const caseId = vars?.caseId || vars?.case_id;
      const threadId = vars?.threadId || vars?.thread_id;
      qc.invalidateQueries({ queryKey: ["caseCommunication", caseId, threadId] });
      qc.invalidateQueries({ queryKey: ["caseCommunications", caseId] });
      qc.invalidateQueries({ queryKey: ["case", caseId] });
    },
  });
}

// ── Observable Types ──
export function useObservableTypes(tenantId: string) {
  return useQuery({
    queryKey: ["observableTypes", tenantId],
    queryFn: async () => {
      const items = await listCatalog("observable_types", { include_global: true, limit: 500 });
      return items.map((item: any) => ({
        id: item.id,
        name: item.name,
        dataType: item.dataType || item.data_type,
        validatorRegex: item.validatorRegex || item.validator_regex,
        tenantId: item.tenant_id || item.tenantId,
      }));
    },
    enabled: !!tenantId,
  });
}

export function useCreateObservableType() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: any) => createCatalog("observable_types", {
      id: data?.id,
      name: data?.name,
      dataType: data?.dataType || data?.data_type,
      validatorRegex: data?.validatorRegex || data?.validator_regex,
    }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["observableTypes"] }),
  });
}

export function useUpdateObservableType() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: any }) => updateCatalog("observable_types", id, {
      name: data?.name,
      dataType: data?.dataType || data?.data_type,
      validatorRegex: data?.validatorRegex || data?.validator_regex,
    }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["observableTypes"] }),
  });
}

export function useDeleteObservableType() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteCatalog("observable_types", id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["observableTypes"] }),
  });
}

// ── Case Categories ──
export function useCaseCategories(tenantId: string) {
  return useQuery({
    queryKey: ["caseCategories", tenantId],
    queryFn: async () => {
      const items = await listCatalog("case_categories", { include_global: true, limit: 500 });
      return items
        .map((item: any) => ({
          id: String(item?.id || ""),
          name: String(item?.name || item?.title || "").trim(),
          description: String(item?.description || "").trim(),
          tenantId: String(item?.tenant_id || item?.tenantId || ""),
        }))
        .filter((item: any) => item.name.length > 0);
    },
    enabled: !!tenantId,
  });
}

export function useCreateCaseCategory() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: { name: string; description?: string }) => createCatalog("case_categories", {
      name: String(data?.name || "").trim(),
      description: String(data?.description || "").trim(),
    }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["caseCategories"] }),
  });
}

// ── Templates ──
export function useTemplates(tenantId: string) {
  return useQuery({
    queryKey: ["templates", tenantId],
    queryFn: async () => {
      const items = await listCatalog("templates", { include_global: true });
      return items.map((item: any) => ({
        id: item.id,
        name: item.name,
        type: item.type,
        content: item.content,
        tenantId: item.tenant_id,
      }));
    },
    enabled: !!tenantId,
  });
}

export function useCreateTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: any) => createCatalog("templates", {
      id: data?.id,
      name: data?.name,
      type: data?.type,
      content: data?.content,
    }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["templates"] }),
  });
}

function mapConnectorTemplateCatalogItem(item: any) {
  const configSchema = ensureArray(item?.config_schema ?? item?.configSchema);
  const forms = item?.forms && typeof item.forms === "object" ? item.forms : {};
  return {
    id: String(item?.id || "").trim(),
    name: String(item?.name || item?.title || "").trim(),
    description: String(item?.description || "").trim(),
    type: String(item?.type || item?.provider || "HTTP").trim(),
    channel: String(item?.channel || "").trim().toLowerCase(),
    category: String(item?.category || "standard").trim().toLowerCase(),
    communicationMode: String(item?.communication_mode || item?.communicationMode || "").trim().toLowerCase(),
    capabilities: stringSliceFromUnknown(item?.capabilities),
    tags: stringSliceFromUnknown(item?.tags),
    icon: item?.icon || item?.icon_key || item?.iconKey || "",
    configSchema,
    forms: {
      manualRun: ensureArray(forms?.manual_run ?? forms?.manualRun ?? item?.manual_run ?? item?.manualRun),
      forumSend: ensureArray(forms?.forum_send ?? forms?.forumSend ?? item?.forum_send ?? item?.forumSend),
      caseCommunication: ensureArray(forms?.case_communication ?? forms?.caseCommunication ?? item?.case_communication ?? item?.caseCommunication),
    },
    tenantId: String(item?.tenant_id || item?.tenantId || "").trim(),
  };
}

export function useConnectorTemplates(tenantId: string) {
  return useQuery({
    queryKey: ["connectorTemplates", tenantId],
    queryFn: async () => {
      const items = await listCatalog("connector_templates", { include_global: true, limit: 500 });
      return ensureArray(items).map(mapConnectorTemplateCatalogItem).filter((item: any) => item.id || item.name);
    },
    enabled: !!tenantId,
  });
}

export function useCreateConnectorTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: any) => createCatalog("connector_templates", {
      name: data?.name,
      description: data?.description || "",
      type: data?.type || "HTTP",
      channel: data?.channel || "",
      category: data?.category || "standard",
      communication_mode: data?.communicationMode || data?.communication_mode || "",
      capabilities: ensureArray(data?.capabilities),
      tags: ensureArray(data?.tags),
      icon: data?.icon || "",
      config_schema: ensureArray(data?.configSchema ?? data?.config_schema),
      forms: {
        manual_run: ensureArray(data?.forms?.manualRun ?? data?.forms?.manual_run ?? data?.manualRun ?? data?.manual_run),
        forum_send: ensureArray(data?.forms?.forumSend ?? data?.forms?.forum_send ?? data?.forumSend ?? data?.forum_send),
        case_communication: ensureArray(data?.forms?.caseCommunication ?? data?.forms?.case_communication ?? data?.caseCommunication ?? data?.case_communication),
      },
    }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["connectorTemplates"] }),
  });
}

export function useUpdateConnectorTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: any }) => updateCatalog("connector_templates", id, {
      name: data?.name,
      description: data?.description || "",
      type: data?.type || "HTTP",
      channel: data?.channel || "",
      category: data?.category || "standard",
      communication_mode: data?.communicationMode || data?.communication_mode || "",
      capabilities: ensureArray(data?.capabilities),
      tags: ensureArray(data?.tags),
      icon: data?.icon || "",
      config_schema: ensureArray(data?.configSchema ?? data?.config_schema),
      forms: {
        manual_run: ensureArray(data?.forms?.manualRun ?? data?.forms?.manual_run ?? data?.manualRun ?? data?.manual_run),
        forum_send: ensureArray(data?.forms?.forumSend ?? data?.forms?.forum_send ?? data?.forumSend ?? data?.forum_send),
        case_communication: ensureArray(data?.forms?.caseCommunication ?? data?.forms?.case_communication ?? data?.caseCommunication ?? data?.case_communication),
      },
    }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["connectorTemplates"] }),
  });
}

export function useDeleteConnectorTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteCatalog("connector_templates", id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["connectorTemplates"] }),
  });
}

export function useCommunicationTemplates(tenantId: string) {
  return useQuery({
    queryKey: ["communicationTemplates", tenantId],
    queryFn: async () => {
      const items = await listCatalog("communication_templates", { include_global: true, limit: 500 });
      return items.map((item: any) => ({
        id: String(item?.id || "").trim(),
        name: String(item?.name || item?.title || "").trim(),
        description: String(item?.description || "").trim(),
        channel: String(item?.channel || item?.type || "email").trim().toLowerCase(),
        subjectTemplate: String(item?.subject_template || item?.subjectTemplate || item?.subject || "").trim(),
        bodyTemplate: String(item?.body_template || item?.bodyTemplate || item?.message_template || item?.content_template || item?.content || "").trim(),
        tenantId: String(item?.tenant_id || item?.tenantId || "").trim(),
      })).filter((item: any) => item.id);
    },
    enabled: !!tenantId,
  });
}

export function useCreateCommunicationTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: any) => createCatalog("communication_templates", {
      id: data?.id,
      name: data?.name,
      description: data?.description || "",
      channel: data?.channel || "email",
      subject_template: data?.subjectTemplate || data?.subject_template || "",
      body_template: data?.bodyTemplate || data?.body_template || "",
    }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["communicationTemplates"] }),
  });
}


export function useUpdateCommunicationTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: any }) =>
      updateCatalog("communication_templates", id, {
        name: data?.name,
        description: data?.description || "",
        channel: data?.channel || "email",
        subject_template: data?.subjectTemplate || data?.subject_template || "",
        body_template: data?.bodyTemplate || data?.body_template || "",
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["communicationTemplates"] }),
  });
}

export function useDeleteCommunicationTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteCatalog("communication_templates", id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["communicationTemplates"] }),
  });
}

export type WorkflowCatalogKind = "workflows";
export type WorkflowBuilderType = "workflow";

export function workflowTypeToCatalogKind(_type: WorkflowBuilderType): WorkflowCatalogKind {
  return "workflows";
}

function workflowTypeFromKindAndData(_kind: WorkflowCatalogKind, _data: any): WorkflowBuilderType {
  return "workflow";
}

function normalizeWorkflowDefinition(definition: any) {
  const raw = definition && typeof definition === "object" ? definition : {};
  const nodes = ensureArray(raw.nodes)
    .map((item: any) => ({
      id: String(item?.id || ""),
      type: String(item?.type || "task"),
      label: String(item?.label || item?.name || "Step"),
      position: {
        x: Number(item?.position?.x || 120),
        y: Number(item?.position?.y || 120),
      },
      config: item?.config && typeof item.config === "object" ? item.config : {},
    }))
    .filter((item: any) => item.id);
  const edges = ensureArray(raw.edges)
    .map((item: any) => ({
      id: String(item?.id || `${item?.source || ""}->${item?.target || ""}`),
      source: String(item?.source || ""),
      target: String(item?.target || ""),
      label: String(item?.label || ""),
      condition: String(item?.condition || ""),
    }))
    .filter((item: any) => item.source && item.target);

  const firstNodeId = nodes[0]?.id || "";
  const entryNodeId = String(raw.entryNodeId || raw.entry_node_id || firstNodeId);
  return {
    version: Number(raw.version || 1),
    entryNodeId: entryNodeId || firstNodeId,
    nodes,
    edges,
  };
}

export function useWorkflowCatalog(tenantId: string, kind: WorkflowCatalogKind = "workflows") {
  return useQuery({
    queryKey: ["workflowCatalog", tenantId, kind],
    queryFn: async () => {
      const items = await listCatalog("workflows", { limit: 500 });
      const workflows = ensureArray(items)
        .map((item: any) => ({
          id: item.id,
          kind: "workflows",
          type: workflowTypeFromKindAndData("workflows", item),
          name: item.name || "Untitled workflow",
          description: item.description || "",
          status: String(item.status || "draft").toLowerCase() === "published" ? "published" : "draft",
          enabled: Boolean(item.enabled ?? item.is_enabled ?? item.isEnabled ?? (String(item.status || "").toLowerCase() === "published")),
          definition: normalizeWorkflowDefinition(item.definition || item.workflow_definition || item.workflow),
          createdAt: item.created_at || item.createdAt,
          updatedAt: item.updated_at || item.updatedAt,
          tenantId: item.tenant_id || item.tenantId,
        }))
        .sort((left: any, right: any) => compareChronological(right.updatedAt, left.updatedAt, right.id, left.id));
      return kind === "workflows" ? workflows : [];
    },
    enabled: !!tenantId,
  });
}

export function useCreateWorkflowCatalogItem(kind: WorkflowCatalogKind = "workflows") {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: any) => {
      const definition = normalizeWorkflowDefinition(data?.definition);
      return createCatalog(kind, {
        name: data?.name,
        description: data?.description || "",
        type: data?.type || workflowTypeFromKindAndData(kind, data),
        status: String(data?.status || "draft").toLowerCase() === "published" ? "published" : "draft",
        enabled: Boolean(data?.enabled),
        is_enabled: Boolean(data?.enabled),
        definition,
        node_count: definition.nodes.length,
      });
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["workflowCatalog"] }),
  });
}

export function useUpdateWorkflowCatalogItem(defaultKind: WorkflowCatalogKind = "workflows") {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, kind, data }: { id: string; kind?: WorkflowCatalogKind; data: any }) => {
      const effectiveKind = kind || defaultKind;
      const input = data && typeof data === "object" ? data : {};
      const hasOwn = (key: string) => Object.prototype.hasOwnProperty.call(input, key);
      const payload: Record<string, any> = {};

      if (hasOwn("name")) {
        payload.name = input?.name;
      }
      if (hasOwn("description")) {
        payload.description = input?.description || "";
      }
      if (hasOwn("type")) {
        payload.type = input?.type || workflowTypeFromKindAndData(effectiveKind, input);
      }
      if (hasOwn("status")) {
        payload.status = String(input?.status || "draft").toLowerCase() === "published" ? "published" : "draft";
      }
      if (hasOwn("enabled") || hasOwn("is_enabled") || hasOwn("isEnabled")) {
        const enabled = Boolean(input?.enabled ?? input?.is_enabled ?? input?.isEnabled);
        payload.enabled = enabled;
        payload.is_enabled = enabled;
      }
      if (hasOwn("definition")) {
        const definition = normalizeWorkflowDefinition(input?.definition);
        payload.definition = definition;
        payload.node_count = definition.nodes.length;
      }

      return updateCatalog(effectiveKind, id, payload);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["workflowCatalog"] }),
  });
}

export function useDeleteWorkflowCatalogItem(defaultKind: WorkflowCatalogKind = "workflows") {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, kind }: { id: string; kind?: WorkflowCatalogKind }) => deleteCatalog(kind || defaultKind, id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["workflowCatalog"] }),
  });
}

export function useWorkflowRunHistory(workflowID: string, limit = 100) {
  return useQuery({
    queryKey: ["workflowRuns", workflowID, limit],
    queryFn: async () => {
      const payload = await coreFetch(
        `/api/v1/workflows/${encodeURIComponent(workflowID)}/runs${catalogQuery({ limit })}`,
        { method: "GET" },
      );
      return {
        runs: ensureArray(payload?.runs).map(mapWorkflowRun),
        retentionLimit: Number(payload?.retention_limit ?? payload?.retentionLimit ?? 1000),
      };
    },
    enabled: !!workflowID,
  });
}

export function useCasePlaybookRuns(caseId: string, tenantId: string, limit = 30) {
  return useQuery({
    queryKey: ["casePlaybookRuns", tenantId, caseId, limit],
    queryFn: async () => {
      const workflowItems = await listCatalog("workflows", { limit: 200 });
      const workflows = ensureArray(workflowItems)
        .map((item: any) => ({
          id: String(item?.id || "").trim(),
          name: String(item?.name || item?.title || "Workflow").trim(),
          updatedAt: item?.updated_at || item?.updatedAt || item?.created_at || item?.createdAt || "",
        }))
        .filter((item: any) => item.id)
        .sort((left: any, right: any) => compareChronological(right.updatedAt, left.updatedAt, right.id, left.id))
        .slice(0, 80);

      if (workflows.length === 0) {
        return {
          runs: [],
          retentionLimit: 1000,
          scannedWorkflows: 0,
        };
      }

      const runLimit = Math.max(10, Math.min(limit, 100));
      const settled = await Promise.allSettled(
        workflows.map(async (workflowItem: any) => {
          const payload = await coreFetch(
            `/api/v1/workflows/${encodeURIComponent(workflowItem.id)}/runs${catalogQuery({ limit: runLimit })}`,
            { method: "GET" },
          );
          return {
            workflow: workflowItem,
            payload,
          };
        }),
      );

      let retentionLimit = 1000;
      const runs: any[] = [];
      settled.forEach((entry) => {
        if (entry.status !== "fulfilled") {
          return;
        }
        const retentionCandidate = Number(
          entry.value?.payload?.retention_limit ??
            entry.value?.payload?.retentionLimit ??
            retentionLimit,
        );
        if (Number.isFinite(retentionCandidate) && retentionCandidate > 0) {
          retentionLimit = retentionCandidate;
        }
        const workflowID = entry.value.workflow.id;
        const workflowName = entry.value.workflow.name;
        ensureArray(entry.value?.payload?.runs)
          .map((run: any) => mapWorkflowRun(run))
          .forEach((run: any) => {
            if (!workflowRunReferencesCase(run, caseId)) {
              return;
            }
            runs.push({
              ...run,
              workflowId: run.workflowId || workflowID,
              workflowName,
            });
          });
      });

      runs.sort((left: any, right: any) =>
        compareChronological(right.startedAt || right.createdAt, left.startedAt || left.createdAt, right.id, left.id),
      );

      return {
        runs: runs.slice(0, Math.max(1, limit)),
        retentionLimit,
        scannedWorkflows: workflows.length,
      };
    },
    enabled: !!caseId && !!tenantId,
  });
}

export function useWorkflowTestRun() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ workflowID, data }: { workflowID: string; data?: any }) => {
      const payload = await coreFetch(`/api/v1/workflows/${encodeURIComponent(workflowID)}/test-run`, {
        method: "POST",
        body: JSON.stringify(data || {}),
      });
      return {
        ok: Boolean(payload?.ok),
        run: mapWorkflowRun(payload?.run || {}),
        retentionLimit: Number(payload?.retention_limit ?? payload?.retentionLimit ?? 1000),
      };
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ["workflowRuns", vars.workflowID] });
      qc.invalidateQueries({ queryKey: ["casePlaybookRuns"] });
    },
  });
}

export function useWorkflowRun() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ workflowID, data }: { workflowID: string; data?: any }) => {
      const payload = await coreFetch(`/api/v1/workflows/${encodeURIComponent(workflowID)}/run`, {
        method: "POST",
        body: JSON.stringify(data || {}),
      });
      return {
        ok: Boolean(payload?.ok),
        run: mapWorkflowRun(payload?.run || {}),
        retentionLimit: Number(payload?.retention_limit ?? payload?.retentionLimit ?? 1000),
      };
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ["workflowRuns", vars.workflowID] });
    },
  });
}

// ── Case Templates ──
export function useCaseTemplates(tenantId: string) {
  return useQuery({
    queryKey: ["caseTemplates", tenantId],
    queryFn: async () => {
      const items = await listCatalog("case_templates", { include_global: true, limit: 500 });
      return items.map((item: any) => ({
        id: item.id,
        name: item.name,
        description: item.description,
        status: item.status || "",
        severity: item.severity,
        source: item.source || "",
        incidentType: item.incident_type || item.incidentType || "",
        priority: item.priority || "medium",
        confidence: Number(item.confidence ?? 0),
        tlp: item.tlp || "amber",
        pap: item.pap || "amber",
        detectedAt: item.detected_at || item.detectedAt || "",
        occurredAt: item.occurred_at || item.occurredAt || "",
        assigneeId: String(item.assignee_id || item.assigneeId || "").trim(),
        tags: ensureArray(item.tags),
        tasks: ensureArray(item.tasks),
        customFields: normalizeCustomFields(item.customFields ?? item.custom_fields),
        tenantId: item.tenant_id,
      }));
    },
    enabled: !!tenantId,
  });
}

export function useCreateCaseTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: any) => {
      const normalizedCustomFields = normalizeCustomFields(data?.customFields ?? data?.custom_fields);
      return createCatalog("case_templates", {
        id: data?.id,
        name: data?.name,
        description: data?.description,
        status: data?.status || "",
        severity: data?.severity,
        source: data?.source || "",
        incident_type: data?.incidentType || data?.incident_type || "",
        priority: data?.priority || "medium",
        confidence: Number(data?.confidence ?? 0),
        tlp: data?.tlp || "amber",
        pap: data?.pap || "amber",
        detected_at: data?.detectedAt || data?.detected_at || "",
        occurred_at: data?.occurredAt || data?.occurred_at || "",
        assignee_id: data?.assigneeId || data?.assignee_id || "",
        tags: ensureArray(data?.tags),
        tasks: ensureArray(data?.tasks),
        customFields: normalizedCustomFields,
        custom_fields: normalizedCustomFields,
      });
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["caseTemplates"] }),
  });
}

export function useUpdateCaseTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: any }) => {
      const normalizedCustomFields = normalizeCustomFields(data?.customFields ?? data?.custom_fields);
      return updateCatalog("case_templates", id, {
        name: data?.name,
        description: data?.description,
        status: data?.status || "",
        severity: data?.severity,
        source: data?.source || "",
        incident_type: data?.incidentType || data?.incident_type || "",
        priority: data?.priority || "medium",
        confidence: Number(data?.confidence ?? 0),
        tlp: data?.tlp || "amber",
        pap: data?.pap || "amber",
        detected_at: data?.detectedAt || data?.detected_at || "",
        occurred_at: data?.occurredAt || data?.occurred_at || "",
        assignee_id: data?.assigneeId || data?.assignee_id || "",
        tags: ensureArray(data?.tags),
        tasks: ensureArray(data?.tasks),
        customFields: normalizedCustomFields,
        custom_fields: normalizedCustomFields,
      });
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["caseTemplates"] }),
  });
}

export function useDeleteCaseTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteCatalog("case_templates", id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["caseTemplates"] }),
  });
}

// ── Shifts ──
export function useShifts(tenantId: string, month: number, year: number) {
  return useQuery({
    queryKey: ["shifts", tenantId, month, year],
    queryFn: async () => {
      const items = await listCatalog("shifts", { month, year });
      return items;
    },
    enabled: !!tenantId,
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  });
}

export function useCreateShift() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: any) => createCatalog("shifts", {
      ...data,
      month: Number(data?.month),
      year: Number(data?.year),
    }, data?.userId || data?.user_id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["shifts"] }),
  });
}

export function useDeleteShift() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteCatalog("shifts", id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["shifts"] }),
  });
}

// ── Dashboard ──
export function useDashboardStats(tenantId: string) {
  return useQuery({
    queryKey: ["dashboard", tenantId],
    queryFn: async () => coreFetch("/api/v1/dashboard/stats", { method: "GET" }),
    refetchInterval: 30000,
    enabled: !!tenantId,
  });
}

export function useDashboardMetrics(tenantId: string, overdueMinutes = 24 * 60) {
  return useQuery({
    queryKey: ["dashboardMetrics", tenantId, overdueMinutes],
    queryFn: async () => {
      const params = new URLSearchParams();
      if (overdueMinutes > 0) {
        params.set("overdue_minutes", String(Math.floor(overdueMinutes)));
      }
      const query = params.size > 0 ? `?${params.toString()}` : "";
      const payload = await coreFetch(`/api/v1/dashboard/metrics${query}`, { method: "GET" });
      return {
        openCases: Number(payload?.open_cases ?? payload?.openCases ?? 0),
        openAlerts: Number(payload?.open_alerts ?? payload?.openAlerts ?? 0),
        overdueCases: Number(payload?.overdue_cases ?? payload?.overdueCases ?? 0),
        overdueThresholdMinutes: Number(payload?.overdue_threshold_m ?? payload?.overdueThresholdMinutes ?? overdueMinutes),
        generatedAt: String(payload?.generated_at ?? payload?.generatedAt ?? ""),
        slaBySeverity: ensureArray(payload?.sla_by_severity ?? payload?.slaBySeverity).map(mapDashboardSLA),
        resolvedByAnalyst: ensureArray(payload?.resolved_by_analyst ?? payload?.resolvedByAnalyst).map(mapDashboardAnalystMetric),
        casesByStatus: ensureArray(payload?.cases_by_status ?? payload?.casesByStatus).map(mapDashboardMetricBucket),
        casesByCategory: ensureArray(payload?.cases_by_category ?? payload?.casesByCategory).map(mapDashboardMetricBucket),
        alertsByStatus: ensureArray(payload?.alerts_by_status ?? payload?.alertsByStatus).map(mapDashboardMetricBucket),
        alertsByCategory: ensureArray(payload?.alerts_by_category ?? payload?.alertsByCategory).map(mapDashboardMetricBucket),
        customMetrics: ensureArray(payload?.custom_metrics ?? payload?.customMetrics).map(mapDashboardCustomMetric),
      };
    },
    enabled: !!tenantId,
    refetchInterval: 30000,
  });
}

export function useDashboardCustomMetrics(tenantId: string) {
  return useQuery({
    queryKey: ["dashboardCustomMetrics", tenantId],
    queryFn: async () => {
      const payload = await coreFetch("/api/v1/dashboard/metrics/custom", { method: "GET" });
      return ensureArray(payload?.items).map(mapDashboardCustomMetric);
    },
    enabled: !!tenantId,
  });
}

export function useCreateDashboardCustomMetric() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: any) => {
      const payload = await coreFetch("/api/v1/dashboard/metrics/custom", {
        method: "POST",
        body: JSON.stringify({
          name: String(data?.name || "").trim(),
          description: String(data?.description || "").trim(),
          source: String(data?.source || "cases").trim().toLowerCase(),
          measure: String(data?.measure || "count").trim().toLowerCase(),
          enabled: Boolean(data?.enabled ?? true),
          filters: {
            statuses: ensureArray(data?.filters?.statuses).map((entry: any) => String(entry || "").trim()).filter(Boolean),
            severities: ensureArray(data?.filters?.severities).map((entry: any) => String(entry || "").trim()).filter(Boolean),
            categories: ensureArray(data?.filters?.categories).map((entry: any) => String(entry || "").trim()).filter(Boolean),
            created_from: String(data?.filters?.createdFrom ?? data?.filters?.created_from ?? "").trim(),
            created_to: String(data?.filters?.createdTo ?? data?.filters?.created_to ?? "").trim(),
            closed_from: String(data?.filters?.closedFrom ?? data?.filters?.closed_from ?? "").trim(),
            closed_to: String(data?.filters?.closedTo ?? data?.filters?.closed_to ?? "").trim(),
            created_within_hours: Number(data?.filters?.createdWithinHours ?? data?.filters?.created_within_hours ?? 0),
            closed_within_hours: Number(data?.filters?.closedWithinHours ?? data?.filters?.closed_within_hours ?? 0),
            overdue_minutes: Number(data?.filters?.overdueMinutes ?? data?.filters?.overdue_minutes ?? 0),
          },
        }),
      });
      return mapDashboardCustomMetric(payload);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["dashboardMetrics"] });
      qc.invalidateQueries({ queryKey: ["dashboardCustomMetrics"] });
    },
  });
}

export function useUpdateDashboardCustomMetric() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, data }: { id: string; data: any }) => {
      const payload = await coreFetch(`/api/v1/dashboard/metrics/custom/${encodeURIComponent(id)}`, {
        method: "PATCH",
        body: JSON.stringify({
          name: String(data?.name || "").trim(),
          description: String(data?.description || "").trim(),
          source: String(data?.source || "cases").trim().toLowerCase(),
          measure: String(data?.measure || "count").trim().toLowerCase(),
          enabled: Boolean(data?.enabled ?? true),
          filters: {
            statuses: ensureArray(data?.filters?.statuses).map((entry: any) => String(entry || "").trim()).filter(Boolean),
            severities: ensureArray(data?.filters?.severities).map((entry: any) => String(entry || "").trim()).filter(Boolean),
            categories: ensureArray(data?.filters?.categories).map((entry: any) => String(entry || "").trim()).filter(Boolean),
            created_from: String(data?.filters?.createdFrom ?? data?.filters?.created_from ?? "").trim(),
            created_to: String(data?.filters?.createdTo ?? data?.filters?.created_to ?? "").trim(),
            closed_from: String(data?.filters?.closedFrom ?? data?.filters?.closed_from ?? "").trim(),
            closed_to: String(data?.filters?.closedTo ?? data?.filters?.closed_to ?? "").trim(),
            created_within_hours: Number(data?.filters?.createdWithinHours ?? data?.filters?.created_within_hours ?? 0),
            closed_within_hours: Number(data?.filters?.closedWithinHours ?? data?.filters?.closed_within_hours ?? 0),
            overdue_minutes: Number(data?.filters?.overdueMinutes ?? data?.filters?.overdue_minutes ?? 0),
          },
        }),
      });
      return mapDashboardCustomMetric(payload);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["dashboardMetrics"] });
      qc.invalidateQueries({ queryKey: ["dashboardCustomMetrics"] });
    },
  });
}

export function useDeleteDashboardCustomMetric() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => coreFetch(`/api/v1/dashboard/metrics/custom/${encodeURIComponent(id)}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["dashboardMetrics"] });
      qc.invalidateQueries({ queryKey: ["dashboardCustomMetrics"] });
    },
  });
}

export function useDutyOverview(tenantId: string, at?: string) {
  return useQuery({
    queryKey: ["dutyOverview", tenantId, at || ""],
    queryFn: async () => {
      const params = new URLSearchParams();
      if (at) {
        params.set("at", at);
      }
      const query = params.size > 0 ? `?${params.toString()}` : "";
      const payload = await coreFetch(`/api/v1/duty/overview${query}`, { method: "GET" });
      return mapDutyOverview(payload);
    },
    refetchInterval: 30000,
    enabled: !!tenantId,
  });
}

// ── Admin API Tokens ──
export function useAdminApiTokens(tenantId: string) {
  return useQuery({
    queryKey: ["adminApiTokens", tenantId],
    queryFn: async () => {
      const items = await coreFetch("/api/v1/admin/api-tokens", { method: "GET" });
      return items.map((item: any) => ({
        id: item.id,
        name: item.name || "",
        description: item.description || "",
        tokenPrefix: item.token_prefix || item.tokenPrefix || "",
        scopes: ensureArray(item.scopes).map((value: any) => String(value || "").trim()).filter(Boolean),
        fullAccess: Boolean(item.full_access ?? item.fullAccess),
        createdBy: item.created_by || item.createdBy || "",
        tenantId: item.tenant_id,
        expiresAt: item.expiresAt || item.expires_at,
        lastUsedAt: item.lastUsedAt || item.last_used_at,
        createdAt: item.createdAt || item.created_at,
      }));
    },
    enabled: !!tenantId,
  });
}

export function useCreateAdminApiToken() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: any) => {
      const payload = await coreFetch("/api/v1/admin/api-tokens", {
        method: "POST",
        body: JSON.stringify({
          name: data?.name || "",
          description: data?.description || "",
          scopes: ensureArray(data?.scopes),
          full_access: Boolean(data?.fullAccess ?? data?.full_access),
          expires_at: data?.expiresAt || data?.expires_at || "",
        }),
      });
      return {
        id: payload?.id,
        name: payload?.name || "",
        description: payload?.description || "",
        tokenPrefix: payload?.token_prefix || payload?.tokenPrefix || "",
        token: payload?.token || "",
        scopes: ensureArray(payload?.scopes),
        fullAccess: Boolean(payload?.full_access ?? payload?.fullAccess),
        createdBy: payload?.created_by || payload?.createdBy || "",
        expiresAt: payload?.expires_at || payload?.expiresAt || null,
        createdAt: payload?.created_at || payload?.createdAt || null,
      };
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["adminApiTokens"] }),
  });
}

export function useRevokeAdminApiToken() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => coreFetch(`/api/v1/admin/api-tokens/${encodeURIComponent(id)}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["adminApiTokens"] }),
  });
}

// ── Rate Limits ──
export function useRateLimits(tenantId: string) {
  return useQuery({
    queryKey: ["rateLimits", tenantId],
    queryFn: () => listCatalog("rate_limits"),
    enabled: !!tenantId,
  });
}

export function useCreateRateLimit() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: any) => createCatalog("rate_limits", {
      targetType: data?.targetType,
      targetId: data?.targetId,
      maxRequests: Number(data?.maxRequests || 0),
      windowSeconds: Number(data?.windowSeconds || 0),
      enabled: data?.enabled ?? true,
    }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["rateLimits"] }),
  });
}

export function useUpdateRateLimit() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: any }) => updateCatalog("rate_limits", id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["rateLimits"] }),
  });
}

export function useDeleteRateLimit() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteCatalog("rate_limits", id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["rateLimits"] }),
  });
}

export type SOCAccessPolicy = {
  id: string;
  allowedCaseTags: string[];
  maxCasesInWork: number;
};

export function useSOCAccessPolicy(tenantId: string) {
  return useQuery({
    queryKey: ["socAccessPolicy", tenantId],
    queryFn: async (): Promise<SOCAccessPolicy> => {
      const items = await listCatalog("soc_access_policies", { limit: 1 });
      const first = ensureArray(items)[0] || {};
      const allowedCaseTags = ensureArray(
        first?.allowed_case_tags ?? first?.allowedCaseTags ?? first?.case_tags ?? first?.caseTags,
      )
        .map((item: any) => String(item || "").trim().toLowerCase())
        .filter(Boolean);
      const maxCandidate = Number(
        first?.max_cases_in_work ??
          first?.maxCasesInWork ??
          first?.in_work_limit ??
          first?.inWorkLimit ??
          0,
      );
      const maxCasesInWork = Number.isFinite(maxCandidate) && maxCandidate > 0 ? Math.floor(maxCandidate) : 0;
      return {
        id: String(first?.id || "").trim(),
        allowedCaseTags: Array.from(new Set(allowedCaseTags)),
        maxCasesInWork,
      };
    },
    enabled: !!tenantId,
  });
}

export function useUpsertSOCAccessPolicy() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: { id?: string; allowedCaseTags?: string[]; maxCasesInWork?: number }) => {
      const payload = {
        allowed_case_tags: ensureArray(data?.allowedCaseTags)
          .map((item: any) => String(item || "").trim().toLowerCase())
          .filter(Boolean),
        max_cases_in_work:
          Number.isFinite(Number(data?.maxCasesInWork)) && Number(data?.maxCasesInWork) > 0
            ? Math.floor(Number(data?.maxCasesInWork))
            : 0,
      };
      if (String(data?.id || "").trim()) {
        return updateCatalog("soc_access_policies", String(data?.id), payload);
      }
      return createCatalog("soc_access_policies", payload);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["socAccessPolicy"] });
      qc.invalidateQueries({ queryKey: ["cases"] });
      qc.invalidateQueries({ queryKey: ["case"] });
    },
  });
}

// ── Tenant Management ──
export function useCreateTenant() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: any) => coreFetch("/api/v1/tenants", {
      method: "POST",
      body: JSON.stringify({
        slug: sanitizeSlug(data?.slug || data?.id || data?.name),
        name: data?.name,
        description: data?.description || "",
        max_users: data?.maxUsers,
        responsible_user_id: data?.responsibleUserId,
        is_active: data?.active,
      }),
    }, false),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["tenants"] }),
  });
}

export function useUpdateTenant() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: any }) => coreFetch(`/api/v1/tenants/${encodeURIComponent(id)}`, {
      method: "PATCH",
      body: JSON.stringify({
        name: data?.name,
        description: data?.description,
        max_users: data?.maxUsers,
        responsible_user_id: data?.responsibleUserId,
        active: data?.active,
        is_active: data?.is_active,
      }),
    }, false),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["tenants"] }),
  });
}

// ── User Creation ──
export function useCreateUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: any) => coreFetch("/api/v1/users", {
      method: "POST",
      body: JSON.stringify({
        username: sanitizeSlug(data?.username || data?.id || data?.name || data?.email?.split("@")[0]),
        email: data?.email,
        full_name: data?.name || data?.full_name,
        password: data?.password || `Tmp-${Date.now()}-Pa55w0rd!`,
        tenant_id: data?.tenantId,
        role: normalizeRole(data?.role),
        is_platform_admin: Boolean(data?.isAdmin),
        ldap_enabled: false,
      }),
    }, false),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["users"] }),
  });
}

// ── Health ──
export function useHealth(refetchInterval?: number) {
  return useQuery({
    queryKey: ["health"],
    queryFn: async () => coreFetch("/api/v1/system/health", { method: "GET" }),
    refetchInterval: refetchInterval || 30000,
  });
}

export function useSystemResources(tenantId: string) {
  return useQuery({
    queryKey: ["systemResources", tenantId],
    queryFn: async () => coreFetch("/api/v1/system/resources", { method: "GET" }, true, tenantId),
    enabled: !!tenantId,
    refetchInterval: 15000,
  });
}

// ── Auth Sessions ──
export function useAuthSessions(limit = 25) {
  return useQuery({
    queryKey: ["authSessions", limit],
    queryFn: async () => {
      const safeLimit = Number.isFinite(limit) ? Math.min(Math.max(Number(limit), 1), 200) : 25;
      const payload = await coreFetch(`/api/v1/auth/sessions?limit=${encodeURIComponent(String(safeLimit))}`, { method: "GET" }, false);
      return {
        sessions: ensureArray(payload?.sessions).map(mapAuthSession),
        activeSessions: Number(payload?.active_sessions ?? payload?.activeSessions ?? 0),
        sessionTimeoutSeconds: Number(payload?.session_timeout_seconds ?? payload?.sessionTimeoutSeconds ?? 0),
      };
    },
    refetchInterval: 10000,
  });
}

export function useRevokeOtherSessions() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => coreFetch("/api/v1/auth/sessions/revoke-others", { method: "POST" }, false),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["authSessions"] });
    },
  });
}

// ── Config ──
export function useFactoryResetLocal() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (confirmation: string) => coreFetch("/api/v1/admin/factory-reset-local", {
      method: "POST",
      body: JSON.stringify({ confirmation }),
    }),
    onSuccess: () => {
      qc.invalidateQueries();
    },
  });
}

export function useConfig() {
  return useQuery({
    queryKey: ["config"],
    queryFn: async () => ({
      supportUrl: (import.meta.env.VITE_SUPPORT_URL as string | undefined) || "https://support.example.com",
    }),
    staleTime: Infinity,
  });
}

// ── Search ──
export function useSearch(tenantId: string, query: string) {
  const normalizedQuery = query.trim();
  return useQuery({
    queryKey: ["search", tenantId, normalizedQuery],
    queryFn: async () => coreFetch(`/api/v1/search?q=${encodeURIComponent(normalizedQuery)}`, { method: "GET" }),
    enabled: !!tenantId && normalizedQuery.length >= 2,
  });
}

export function useCaseAIAnalyses(caseId: string, limit = 30) {
  return useQuery({
    queryKey: ["caseAIAnalyses", caseId, limit],
    queryFn: async () => {
      const response = await coreFetch(`/api/v1/cases/${encodeURIComponent(caseId)}/ai/analyses?limit=${encodeURIComponent(String(limit))}`, { method: "GET" });
      return ensureArray(response).map(mapCaseAIAnalysis);
    },
    enabled: !!caseId,
  });
}

export function useAnalyzeCaseAI() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: string | { caseId: string; language?: "en" | "ru" | string }) => {
      const caseId = typeof input === "string" ? input : String(input?.caseId || "").trim();
      const language = typeof input === "string" ? "" : String(input?.language || "").trim().toLowerCase();
      if (!caseId) {
        throw new Error("case id is required");
      }
      const body = language ? JSON.stringify({ language }) : undefined;
      return coreFetch(
        `/api/v1/cases/${encodeURIComponent(caseId)}/ai/analyze`,
        body ? { method: "POST", body } : { method: "POST" },
      );
    },
    onSuccess: (_, input) => {
      const caseId = typeof input === "string" ? input : String(input?.caseId || "").trim();
      qc.invalidateQueries({ queryKey: ["caseAIAnalyses", caseId] });
      qc.invalidateQueries({ queryKey: ["case", caseId] });
      qc.invalidateQueries({ queryKey: ["cases"] });
    },
  });
}

// ── Case Tasks ──
export function useCaseTasks(caseId: string) {
  return useQuery({
    queryKey: ["caseTasks", caseId],
    queryFn: async () => {
      const tasks = await coreFetch(`/api/v1/tasks?case_id=${encodeURIComponent(caseId)}`, { method: "GET" });
      return (tasks || []).map(mapTask).filter((task: any) => task.caseId === caseId);
    },
    enabled: !!caseId,
  });
}

export function useCreateCaseTask() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: any) => {
      const created = await coreFetch("/api/v1/tasks", {
        method: "POST",
        body: JSON.stringify({
          case_id: data?.caseId,
          title: data?.title,
          description: data?.description || "",
          status: toCoreTaskStatus(data?.status),
          assignee_id: data?.assignee || data?.assigneeId || data?.assignee_id || "",
          due_date: data?.dueDate || data?.due_date || undefined,
        }),
      });
      if (data?.mandatory) {
        await createCatalog("task_flags", { mandatory: true }, undefined, created.id).catch(() => undefined);
      }
      return mapTask(created);
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ["caseTasks", vars.caseId] });
      qc.invalidateQueries({ queryKey: ["caseTimeline", vars.caseId] });
    },
  });
}

export function useUpdateCaseTask() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, data }: { id: string; data: any }) => {
      const updated = await coreFetch(`/api/v1/tasks/${encodeURIComponent(id)}`, {
        method: "PATCH",
        body: JSON.stringify({
          title: data?.title,
          description: data?.description,
          status: data?.status ? toCoreTaskStatus(data.status) : undefined,
          assignee_id: data?.assignee || data?.assigneeId || data?.assignee_id,
          due_date: data?.dueDate !== undefined ? data?.dueDate : data?.due_date,
        }),
      });
      return mapTask(updated);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["caseTasks"] }),
  });
}

export function useDeleteCaseTask() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => coreFetch(`/api/v1/tasks/${encodeURIComponent(id)}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["caseTasks"] }),
  });
}

// ── Case Observables ──
export function useCaseObservables(caseId: string) {
  return useQuery({
    queryKey: ["caseObservables", caseId],
    queryFn: async () => {
      const observables = await coreFetch(`/api/v1/cases/${encodeURIComponent(caseId)}/observables`, { method: "GET" });
      return (observables || []).map(mapObservable);
    },
    enabled: !!caseId,
  });
}

export function useCreateCaseObservable() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: any) => {
      const created = await coreFetch(`/api/v1/cases/${encodeURIComponent(data?.caseId)}/observables`, {
        method: "POST",
        body: JSON.stringify({
          type: String(data?.type || "").toLowerCase(),
          value: data?.value,
          verdict: toCoreVerdict(data?.verdict),
          source: data?.source || "manual",
          tags: ensureArray(data?.tags),
        }),
      });
      return mapObservable(created);
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ["caseObservables", vars.caseId] });
      qc.invalidateQueries({ queryKey: ["caseTimeline", vars.caseId] });
      qc.invalidateQueries({ queryKey: ["caseRelatedCases", vars.caseId] });
    },
  });
}

export function useUpdateCaseObservable() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, data }: { id: string; data: any }) => {
      const caseId = data?.caseId || data?.case_id;
      const updated = await coreFetch(`/api/v1/cases/${encodeURIComponent(caseId)}/observables/${encodeURIComponent(id)}`, {
        method: "PATCH",
        body: JSON.stringify({
          type: data?.type,
          value: data?.value,
          verdict: data?.verdict ? toCoreVerdict(data.verdict) : undefined,
          source: data?.source,
          tags: data?.tags,
        }),
      });
      return mapObservable(updated);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["caseObservables"] });
      qc.invalidateQueries({ queryKey: ["caseRelatedCases"] });
    },
  });
}

export function useDeleteCaseObservable() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { id: string; caseId?: string } | string) => {
      let id: string;
      let caseId: string | undefined;
      if (typeof input === "string") {
        id = input;
      } else {
        id = input.id;
        caseId = input.caseId;
      }
      if (!caseId) {
        const cached = qc.getQueriesData({ queryKey: ["caseObservables"] });
        for (const [, value] of cached) {
          const observables = Array.isArray(value) ? value : [];
          const found = observables.find((obs: any) => obs.id === id);
          if (found?.caseId) {
            caseId = found.caseId;
            break;
          }
        }
      }
      if (!caseId) {
        throw new Error("caseId is required to delete observable");
      }
      return coreFetch(`/api/v1/cases/${encodeURIComponent(caseId)}/observables/${encodeURIComponent(id)}`, { method: "DELETE" });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["caseObservables"] });
      qc.invalidateQueries({ queryKey: ["caseRelatedCases"] });
    },
  });
}

// ── Case Pages ──
export function useCasePages(caseId: string) {
  return useQuery({
    queryKey: ["casePages", caseId],
    queryFn: async () => {
      const pages = await coreFetch(`/api/v1/cases/${encodeURIComponent(caseId)}/pages`, { method: "GET" });
      return ensureArray(pages).map(mapCasePage);
    },
    enabled: !!caseId,
  });
}

export function useCreateCasePage() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: any) => {
      const created = await coreFetch(`/api/v1/cases/${encodeURIComponent(data?.caseId)}/pages`, {
        method: "POST",
        body: JSON.stringify({
          title: String(data?.title || "").trim(),
          body: String(data?.body || "").trim(),
        }),
      });
      return mapCasePage(created);
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ["casePages", vars.caseId] });
      qc.invalidateQueries({ queryKey: ["caseTimeline", vars.caseId] });
    },
  });
}

// ── Case Attachments ──
export function useCaseAttachments(caseId: string) {
  return useQuery({
    queryKey: ["caseAttachments", caseId],
    queryFn: async () => {
      const attachments = await coreFetch(`/api/v1/cases/${encodeURIComponent(caseId)}/attachments`, { method: "GET" });
      return ensureArray(attachments).map(mapCaseAttachment);
    },
    enabled: !!caseId,
  });
}

export function useUploadCaseAttachment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ caseId, file }: { caseId: string; file: File }) => {
      const form = new FormData();
      form.set("file", file);
      const payload = await coreFetch(`/api/v1/cases/${encodeURIComponent(caseId)}/attachments/upload`, {
        method: "POST",
        body: form,
      });
      return {
        attachment: payload?.attachment ? mapCaseAttachment(payload.attachment) : null,
        downloadUrl: payload?.download_url || "",
      };
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ["caseAttachments", vars.caseId] });
      qc.invalidateQueries({ queryKey: ["caseTimeline", vars.caseId] });
    },
  });
}

export function useCaseAttachmentDownloadURL() {
  return useMutation({
    mutationFn: async ({ caseId, attachmentId }: { caseId: string; attachmentId: string }) => {
      const payload = await coreFetch(
        `/api/v1/cases/${encodeURIComponent(caseId)}/attachments/${encodeURIComponent(attachmentId)}/download`,
        { method: "GET" },
      );
      return payload?.url || "";
    },
  });
}

// ── Case Comments ──
export function useCaseComments(caseId: string) {
  return useQuery({
    queryKey: ["caseComments", caseId],
    queryFn: async () => {
      const comments = await coreFetch(`/api/v1/case-comments?case_id=${encodeURIComponent(caseId)}`, { method: "GET" });
      const mapped = ensureArray(comments).map((comment: any) => {
        const data = comment?.data && typeof comment.data === "object" ? comment.data : {};
        const actor = mapSyntheticActor(data);
        return {
          id: comment.id,
          caseId: data?.case_id || data?.caseId || comment.case_id || comment.caseId,
          authorId: actor?.authorId || data?.author_id || data?.authorId || comment.author_id || comment.authorId || comment.owner_id || comment.ownerId || "",
          authorName: actor?.authorName || data?.author_name || data?.authorName || comment.author_name || comment.authorName || "",
          authorKind: actor?.authorKind || data?.author_kind || data?.authorKind || "",
          authorAvatarKey: actor?.authorAvatarKey || data?.author_avatar_key || data?.authorAvatarKey || "",
          agentId: actor?.agentId || data?.agent_id || data?.agentId || "",
          content: data?.content || comment.content || "",
          createdAt: data?.created_at || data?.createdAt || comment.created_at || comment.createdAt,
          tenantId: data?.tenant_id || data?.tenantId || comment.tenant_id || comment.tenantId,
          actor,
        };
      });
      mapped.sort((left: any, right: any) => compareChronological(left.createdAt, right.createdAt, left.id, right.id));
      return mapped;
    },
    enabled: !!caseId,
  });
}

export function useCreateCaseComment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: any) => coreFetch("/api/v1/case-comments", {
      method: "POST",
      body: JSON.stringify({
        case_id: data?.caseId,
        author_id: data?.authorId,
        content: data?.content,
      }),
    }),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ["caseComments", vars.caseId] });
      qc.invalidateQueries({ queryKey: ["caseTimeline", vars.caseId] });
    },
  });
}

export function useCreateCaseTimelineEvent() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: any) =>
      coreFetch(`/api/v1/cases/${encodeURIComponent(data?.caseId)}/events`, {
        method: "POST",
        body: JSON.stringify({
          event_type: data?.eventType || "note",
          title: data?.title || "",
          body: data?.body || "",
          metadata: data?.metadata || {},
        }),
      }),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ["caseTimeline", vars.caseId] });
    },
  });
}

// ── Case Timeline ──
export function useCaseTimeline(caseId: string) {
  return useQuery({
    queryKey: ["caseTimeline", caseId],
    queryFn: async () => {
      const [events, comments] = await Promise.all([
        coreFetch(`/api/v1/cases/${encodeURIComponent(caseId)}/events`, { method: "GET" }),
        coreFetch(`/api/v1/case-comments?case_id=${encodeURIComponent(caseId)}`, { method: "GET" }).catch(() => []),
      ]);

      const mappedCoreEvents = ensureArray(events).map((event: any) => {
        const metadata = event?.metadata && typeof event.metadata === "object" ? event.metadata : {};
        const actor = mapSyntheticActor(metadata?.actor);
        return {
          id: event.id,
          eventType: String(event.event_type || event.eventType || "note").toLowerCase(),
          title: event.title || event.body || "Timeline event",
          description: event.body || "",
          userId: event.actor_id,
          actor,
          metadata,
          source: "event",
          createdAt: event.created_at,
        };
      });

      const commentEvents = ensureArray(comments).map((comment: any) => {
        const data = comment?.data && typeof comment.data === "object" ? comment.data : {};
        const actor = mapSyntheticActor(data) || mapSyntheticActor(data?.actor);
        return {
          id: `comment-${comment.id}`,
          eventType: "comment_added",
          title: "Comment added",
          description: data?.content || comment.content || "",
          userId: actor?.authorId || data?.author_id || data?.authorId || comment.author_id || comment.authorId || "",
          actor,
          metadata: {
            comment_id: comment.id,
            actor: actor ? {
              author_kind: actor.authorKind,
              author_id: actor.authorId,
              author_name: actor.authorName,
              author_avatar_key: actor.authorAvatarKey,
              agent_id: actor.agentId,
            } : undefined,
          },
          source: "comment",
          createdAt: data?.created_at || data?.createdAt || comment.created_at || comment.createdAt,
        };
      });

      return [...mappedCoreEvents, ...commentEvents].sort(
        (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime(),
      );
    },
    enabled: !!caseId,
  });
}


export {
  boolFromUnknown,
  catalogQuery,
  coreFetch,
  createCatalog,
  deleteCatalog,
  ensureArray,
  listCatalog,
  mapAdminNotificationSetting,
  mapAIAgentCatalogItem,
  mapAIAgentRunCatalogItem,
  mapAIMessage,
  mapAISession,
  mapCatalogItem,
  mapMyNotificationSettings,
  mapNotificationBot,
  mapServiceAlertRule,
  normalizeConnectorDirection,
  numberFromUnknown,
  stringSliceFromUnknown,
  updateCatalog,
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
};
