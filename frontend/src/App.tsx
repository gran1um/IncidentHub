import { Switch, Route, useLocation } from "wouter";
import { useEffect, useMemo } from "react";
import Dashboard from "@/pages/dashboard";
import AlertsPage from "@/pages/alerts";
import AlertDetailPage from "@/pages/alert-detail";
import CasesPage from "@/pages/cases";
import CaseDetailPage from "@/pages/case-detail";
import TemplatesPage from "@/pages/templates";
import AdministrationPage from "@/pages/administration";
import ConnectorsPage from "@/pages/connectors";
import ForumPage from "@/pages/forum";
import ForumThreadPage from "@/pages/forum-thread";
import ProfilePage from "@/pages/profile";
import SecurityPage from "@/pages/security";
import UserProfilePage from "@/pages/user-profile";
import WorkflowStudioPage from "@/pages/workflow-studio";
import ActivityStreamPage from "@/pages/activity-stream";
import AIAgentsPage from "@/pages/ai-agents";
import LoginPage from "@/pages/login";
import { useAppState, useSessionBootstrap, useTenants } from "@/lib/api";
import { splitLocationPathAndSearch } from "@/lib/url-state";
import { extractLeadingPathSegment, stripTenantPrefix, withTenantLocation, withTenantPath } from "@/lib/tenant-url";

const TENANT_SCOPED_ROUTE_ROOTS = new Set([
  "dashboard",
  "alerts",
  "cases",
  "forum",
  "templates",
  "administration",
  "ai-agents",
  "workflow-studio",
  "profile",
  "security",
  "activity",
  "users",
  "connectors",
]);

function Redirect({ to }: { to: string }) {
  const [, setLocation] = useLocation();
  useEffect(() => {
    setLocation(to, { replace: true });
  }, [setLocation, to]);
  return null;
}

function App() {
  const [location, setLocation] = useLocation();
  const session = useAppState((state) => state.session);
  const currentTenantId = useAppState((state) => state.currentTenantId);
  const currentTenantSlug = useAppState((state) => state.currentTenantSlug);
  const setCurrentTenant = useAppState((state) => state.setCurrentTenant);
  const setCurrentTenantSlug = useAppState((state) => state.setCurrentTenantSlug);
  const sessionBootstrap = useSessionBootstrap();
  const { data: tenants = [] } = useTenants(Boolean(session));

  const { path: locationPath } = useMemo(() => splitLocationPathAndSearch(location), [location]);
  const leadingPathSegment = useMemo(() => extractLeadingPathSegment(locationPath), [locationPath]);

  const tenantSlugById = useMemo(() => {
    const out = new Map<string, string>();
    tenants.forEach((tenant: any) => {
      const id = String(tenant?.id || "").trim();
      const slug = String(tenant?.slug || "").trim().toLowerCase();
      if (!id || !slug) {
        return;
      }
      out.set(id, slug);
    });
    return out;
  }, [tenants]);

  const tenantIdBySlug = useMemo(() => {
    const out = new Map<string, string>();
    tenantSlugById.forEach((slug, id) => {
      out.set(slug, id);
    });
    return out;
  }, [tenantSlugById]);

  const knownTenantSlugs = useMemo(() => Array.from(tenantIdBySlug.keys()), [tenantIdBySlug]);
  const knownTenantIDs = useMemo(() => Array.from(tenantSlugById.keys()), [tenantSlugById]);
  const provisionalTenantSlugFromPath = useMemo(() => {
    const candidate = String(leadingPathSegment || "").trim().toLowerCase();
    if (!candidate || candidate === "login" || TENANT_SCOPED_ROUTE_ROOTS.has(candidate)) {
      return "";
    }
    return candidate;
  }, [leadingPathSegment]);
  const tenantIdFromPath = useMemo(
    () => tenantIdBySlug.get(String(leadingPathSegment || "").trim().toLowerCase()) || "",
    [tenantIdBySlug, leadingPathSegment],
  );
  const routePathWithoutTenant = useMemo(
    () => stripTenantPrefix(locationPath, knownTenantSlugs),
    [locationPath, knownTenantSlugs],
  );

  const activeMembershipTenantIDs = useMemo(
    () =>
      (session?.memberships || [])
        .filter((membership) => membership?.is_active && membership?.tenant_id)
        .map((membership) => String(membership.tenant_id).trim())
        .filter(Boolean),
    [session?.memberships],
  );

  const fallbackTenantID = useMemo(
    () =>
      String(session?.identity?.tenant_id || "").trim() ||
      activeMembershipTenantIDs[0] ||
      "",
    [session?.identity?.tenant_id, activeMembershipTenantIDs],
  );
  const effectiveFallbackTenantID = useMemo(() => {
    if (fallbackTenantID) {
      return fallbackTenantID;
    }
    if (session?.identity?.is_platform_admin) {
      return knownTenantIDs[0] || "";
    }
    return "";
  }, [fallbackTenantID, session?.identity?.is_platform_admin, knownTenantIDs]);

  const resolvedTenantId = currentTenantId || effectiveFallbackTenantID;
  const resolvedTenantSlug = useMemo(
    () => tenantSlugById.get(resolvedTenantId) || "",
    [tenantSlugById, resolvedTenantId],
  );
  const effectiveTenantSlug = resolvedTenantSlug || currentTenantSlug || provisionalTenantSlugFromPath;

  useEffect(() => {
    if (!session) {
      return;
    }
    if (session.identity?.is_platform_admin) {
      if (tenantIdFromPath && tenantIdFromPath !== currentTenantId && knownTenantIDs.includes(tenantIdFromPath)) {
        setCurrentTenant(tenantIdFromPath);
        return;
      }
      if ((!tenantIdFromPath || !knownTenantIDs.includes(tenantIdFromPath)) && effectiveFallbackTenantID && currentTenantId !== effectiveFallbackTenantID) {
        setCurrentTenant(effectiveFallbackTenantID);
      }
      return;
    }
    if (effectiveFallbackTenantID && currentTenantId !== effectiveFallbackTenantID) {
      setCurrentTenant(effectiveFallbackTenantID);
    }
  }, [session, tenantIdFromPath, currentTenantId, knownTenantIDs, effectiveFallbackTenantID, setCurrentTenant]);

  useEffect(() => {
    if (!resolvedTenantSlug) {
      return;
    }
    if (currentTenantSlug !== resolvedTenantSlug) {
      setCurrentTenantSlug(resolvedTenantSlug);
    }
  }, [resolvedTenantSlug, currentTenantSlug, setCurrentTenantSlug]);

  useEffect(() => {
    if (!session || !provisionalTenantSlugFromPath || currentTenantSlug) {
      return;
    }
    setCurrentTenantSlug(provisionalTenantSlugFromPath);
  }, [session, provisionalTenantSlugFromPath, currentTenantSlug, setCurrentTenantSlug]);

  useEffect(() => {
    if (!session || routePathWithoutTenant === "/login" || !resolvedTenantSlug) {
      return;
    }
    if (
      session.identity?.is_platform_admin &&
      tenantIdFromPath &&
      tenantIdFromPath !== resolvedTenantId
    ) {
      return;
    }
    const pathSegments = String(locationPath || "")
      .split("/")
      .filter(Boolean);
    const firstSegment = String(pathSegments[0] || "").trim().toLowerCase();
    const secondSegment = String(pathSegments[1] || "").trim().toLowerCase();
    const looksLikeUnknownTenantPrefix =
      pathSegments.length >= 2 &&
      Boolean(firstSegment) &&
      !knownTenantSlugs.includes(firstSegment) &&
      TENANT_SCOPED_ROUTE_ROOTS.has(secondSegment);
    const knownSlugsForRewrite = looksLikeUnknownTenantPrefix
      ? [...knownTenantSlugs, firstSegment]
      : knownTenantSlugs;

    const nextLocation = withTenantLocation(resolvedTenantSlug, location, knownSlugsForRewrite);
    if (nextLocation !== location) {
      setLocation(nextLocation, { replace: true });
    }
  }, [session, routePathWithoutTenant, resolvedTenantSlug, locationPath, location, setLocation, knownTenantSlugs, tenantIdFromPath, resolvedTenantId]);

  if (!session && sessionBootstrap.isPending) {
    return null;
  }

  if (!session) {
    return (
      <Switch>
        <Route path="/login" component={LoginPage} />
        <Route component={() => <LoginPage />} />
      </Switch>
    );
  }

  const rootPath = withTenantPath(effectiveTenantSlug, "/dashboard");
  const scopedTenantId = currentTenantId || resolvedTenantId;
  const activeMembership = session.memberships?.find((m) => m.tenant_id === scopedTenantId && m.is_active);
  const isTenantAdmin = Boolean(activeMembership?.role === "tenant_admin");
  const isAdmin = Boolean(session.identity?.is_platform_admin || isTenantAdmin);
  const isAnalystOrHigher = Boolean(
    session.identity?.is_platform_admin || activeMembership?.role === "tenant_admin" || activeMembership?.role === "analyst",
  );

  return (
    <Switch>
      <Route path="/login" component={() => <Redirect to={rootPath} />} />

      <Route path="/:tenantSlug/dashboard" component={Dashboard} />
      <Route path="/:tenantSlug" component={() => <Redirect to={rootPath} />} />
      <Route path="/:tenantSlug/alerts" component={() => (isAnalystOrHigher ? <AlertsPage /> : <Redirect to={rootPath} />)} />
      <Route path="/:tenantSlug/alerts/:id" component={() => (isAnalystOrHigher ? <AlertDetailPage /> : <Redirect to={rootPath} />)} />
      <Route path="/:tenantSlug/cases" component={() => (isAnalystOrHigher ? <CasesPage /> : <Redirect to={rootPath} />)} />
      <Route path="/:tenantSlug/cases/:id" component={() => (isAnalystOrHigher ? <CaseDetailPage /> : <Redirect to={rootPath} />)} />
      <Route path="/:tenantSlug/forum" component={() => (isAnalystOrHigher ? <ForumPage /> : <Redirect to={rootPath} />)} />
      <Route path="/:tenantSlug/forum/:id" component={() => (isAnalystOrHigher ? <ForumThreadPage /> : <Redirect to={rootPath} />)} />
      <Route path="/:tenantSlug/ai-agents" component={() => (isAdmin ? <AIAgentsPage /> : <Redirect to={rootPath} />)} />
      <Route path="/:tenantSlug/templates" component={() => (isAnalystOrHigher ? <TemplatesPage /> : <Redirect to={rootPath} />)} />
      <Route path="/:tenantSlug/administration" component={() => (isAdmin ? <AdministrationPage /> : <Redirect to={rootPath} />)} />
      <Route path="/:tenantSlug/connectors" component={() => (isAdmin ? <ConnectorsPage /> : <Redirect to={rootPath} />)} />
      <Route path="/:tenantSlug/workflow-studio" component={() => (isAnalystOrHigher ? <WorkflowStudioPage /> : <Redirect to={rootPath} />)} />
      <Route path="/:tenantSlug/activity" component={() => (isAnalystOrHigher ? <ActivityStreamPage /> : <Redirect to={rootPath} />)} />
      <Route path="/:tenantSlug/profile" component={ProfilePage} />
      <Route path="/:tenantSlug/security" component={SecurityPage} />
      <Route path="/:tenantSlug/users/:id" component={UserProfilePage} />

      <Route path="/" component={() => <Redirect to={rootPath} />} />
      <Route path="/dashboard" component={() => <Redirect to={withTenantPath(effectiveTenantSlug, "/dashboard")} />} />
      <Route path="/alerts" component={() => <Redirect to={withTenantPath(effectiveTenantSlug, "/alerts")} />} />
      <Route path="/alerts/:id" component={(params: any) => <Redirect to={withTenantPath(effectiveTenantSlug, `/alerts/${params.id}`)} />} />
      <Route path="/cases" component={() => <Redirect to={withTenantPath(effectiveTenantSlug, "/cases")} />} />
      <Route path="/cases/:id" component={(params: any) => <Redirect to={withTenantPath(effectiveTenantSlug, `/cases/${params.id}`)} />} />
      <Route path="/forum" component={() => <Redirect to={withTenantPath(effectiveTenantSlug, "/forum")} />} />
      <Route path="/forum/:id" component={(params: any) => <Redirect to={withTenantPath(effectiveTenantSlug, `/forum/${params.id}`)} />} />
      <Route path="/ai-agents" component={() => <Redirect to={withTenantPath(effectiveTenantSlug, "/ai-agents")} />} />
      <Route path="/templates" component={() => <Redirect to={withTenantPath(effectiveTenantSlug, "/templates")} />} />
      <Route path="/administration" component={() => <Redirect to={withTenantPath(effectiveTenantSlug, "/administration")} />} />
      <Route path="/connectors" component={() => <Redirect to={withTenantPath(effectiveTenantSlug, "/connectors")} />} />
      <Route path="/workflow-studio" component={() => <Redirect to={withTenantPath(effectiveTenantSlug, "/workflow-studio")} />} />
      <Route path="/activity" component={() => <Redirect to={withTenantPath(effectiveTenantSlug, "/activity")} />} />
      <Route path="/profile" component={() => <Redirect to={withTenantPath(effectiveTenantSlug, "/profile")} />} />
      <Route path="/security" component={() => <Redirect to={withTenantPath(effectiveTenantSlug, "/security")} />} />
      <Route path="/users/:id" component={(params: any) => <Redirect to={withTenantPath(effectiveTenantSlug, `/users/${params.id}`)} />} />

      <Route component={() => <Redirect to={rootPath} />} />
    </Switch>
  );
}

export default App;
