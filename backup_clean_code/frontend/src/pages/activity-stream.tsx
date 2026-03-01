import { useEffect, useMemo, useRef, useState } from "react";
import { useLocation } from "wouter";
import { ChevronLeft, ChevronRight, Pause, Play, Search } from "lucide-react";
import { AppLayout } from "@/components/layout";
import { useActivityLivestream, useAppState, useUsers } from "@/lib/api";
import { withTenantPath } from "@/lib/tenant-url";
import { applySearchPatch, splitLocationPathAndSearch } from "@/lib/url-state";
import { Skeleton } from "@/components/ui/skeleton";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useMinimumLoading } from "@/lib/use-minimum-loading";

type StreamPalette = {
  dot: string;
  text: string;
};

const ACTIVITY_PAGE_SHELL_CLASS = "mx-auto w-full max-w-[1144px] pb-6";
const ACTIVITY_PANEL_CLASS =
  "rounded-2xl border border-[rgba(255,255,255,0.06)] bg-[linear-gradient(180deg,rgba(19,20,28,0.97),rgba(17,20,32,0.97))] p-[25px] shadow-[0_14px_34px_rgba(0,0,0,0.28)]";
const ACTIVITY_MUTED_TEXT_CLASS = "text-xs text-[#8b91a3]";
const FILTER_INPUT_CLASS =
  "h-[38px] rounded-lg border border-[#2a2c3c] bg-[#0b0c10] px-3 text-sm text-[#d1d5db] outline-none transition-colors placeholder:text-[rgba(209,213,219,0.5)] focus:border-[#3b4a79]";
const FILTER_SELECT_TRIGGER_CLASS =
  "h-[38px] rounded-lg border border-[#2a2c3c] bg-[#0b0c10] px-3 text-sm text-[#d1d5db] focus:ring-1 focus:ring-[#66ff4c]/55";
const TYPE_TOGGLE_ACTIVE_CLASS = "border-[#3b4a79] bg-[#1a2338] text-white";
const TYPE_TOGGLE_INACTIVE_CLASS = "border-[#2a2c3c] bg-[#0f1118] text-[#9ca3af] hover:border-[#4b5168] hover:bg-[#1a1f2d]";
const STREAM_LIMIT_OPTIONS = [50, 100, 200, 500] as const;
const STREAM_ENTITY_TYPES = ["case", "alert", "task"] as const;
type StreamEntityType = (typeof STREAM_ENTITY_TYPES)[number];

function LivestreamGlyph() {
  return (
    <span className="relative block h-[18px] w-[20px] text-[#eb5f65]" aria-hidden="true">
      <span className="absolute left-[4px] top-0 h-[18px] w-[2px] rounded-full bg-current opacity-90" />
      <span className="absolute left-[14px] top-[2px] h-[14px] w-[2px] rounded-full bg-current opacity-90" />
      <span className="absolute left-[1px] top-[4px] h-[4px] w-[4px] rounded-full bg-current" />
      <span className="absolute left-[11px] top-[8px] h-[4px] w-[4px] rounded-full bg-current" />
    </span>
  );
}

function normalizeLabel(value: string): string {
  const raw = String(value || "").trim();
  if (!raw) {
    return "";
  }
  return `${raw.charAt(0).toUpperCase()}${raw.slice(1).toLowerCase()}`;
}

function formatRelativeCompact(value: string): string {
  const parsed = Date.parse(String(value || ""));
  if (!Number.isFinite(parsed)) {
    return "now";
  }
  const diffSeconds = Math.max(1, Math.floor((Date.now() - parsed) / 1000));
  if (diffSeconds < 60) {
    return `${diffSeconds}s ago`;
  }
  const diffMinutes = Math.floor(diffSeconds / 60);
  if (diffMinutes < 60) {
    return `${diffMinutes}m ago`;
  }
  const diffHours = Math.floor(diffMinutes / 60);
  if (diffHours < 24) {
    return `${diffHours}h ago`;
  }
  return `${Math.floor(diffHours / 24)}d ago`;
}

function paletteForItem(item: any): StreamPalette {
  const severity = String(item?.severity || item?.status || item?.action || "")
    .trim()
    .toLowerCase();
  if (severity === "critical") {
    return { dot: "bg-[#eb5f65]/75", text: "text-[#eb5f65]" };
  }
  if (severity === "high") {
    return { dot: "bg-[#ffc700]", text: "text-[#ffc700]" };
  }
  if (severity === "resolved" || severity === "closed" || severity === "done" || severity === "low") {
    return { dot: "bg-[#66ff4c]", text: "text-[#66ff4c]" };
  }
  return { dot: "bg-[#3b82f6]", text: "text-[#3b82f6]" };
}

function buildTitle(item: any): string {
  const title = String(item?.title || "").trim();
  if (title) {
    const severityLabel = normalizeLabel(String(item?.severity || item?.status || "").trim());
    const hasPrefix = /^((critical|high|medium|low|resolved|closed|triaged)\s*:)/i.test(title);
    const shouldPrefix = !hasPrefix && !!severityLabel && String(item?.entity || "").trim().toLowerCase() === "alert";
    if (shouldPrefix) {
      return `${severityLabel}: ${title}`;
    }
    return title;
  }
  const entity = normalizeLabel(String(item?.entity || "").trim()) || "Event";
  const action = normalizeLabel(String(item?.action || "").trim()) || "Updated";
  return `${entity}: ${action}`;
}

function buildTypeLabel(item: any): string {
  const severity = normalizeLabel(String(item?.severity || "").trim());
  if (severity) {
    return severity;
  }
  const status = normalizeLabel(String(item?.status || "").trim());
  if (status) {
    return status;
  }
  return normalizeLabel(String(item?.action || "").trim()) || "Info";
}

function buildSourceLabel(item: any): string {
  const source = String(item?.source || "").trim();
  if (source) {
    return source.toUpperCase();
  }
  const entityId = String(item?.entityId || "").trim();
  if (entityId) {
    return entityId;
  }
  const caseId = String(item?.caseId || "").trim();
  if (caseId) {
    return caseId;
  }
  return "SOC";
}

export default function ActivityStreamPage() {
  const [location, setLocation] = useLocation();
  const queryHydratedRef = useRef(false);
  const { currentTenantId, currentTenantSlug } = useAppState();
  const [q, setQ] = useState("");
  const [paused, setPaused] = useState(false);
  const [limit, setLimit] = useState<number>(200);
  const [page, setPage] = useState<number>(1);
  const [assigneeID, setAssigneeID] = useState<string>("all");
  const [enabledTypes, setEnabledTypes] = useState<Record<StreamEntityType, boolean>>({
    case: true,
    alert: true,
    task: true,
  });
  const { data: users = [], isLoading: usersLoading } = useUsers(currentTenantId);

  const selectedTypes = useMemo(
    () => STREAM_ENTITY_TYPES.filter((type) => enabledTypes[type]),
    [enabledTypes],
  );
  const selectedTypesQuery = useMemo(() => selectedTypes.join(","), [selectedTypes]);

  useEffect(() => {
    const { params } = splitLocationPathAndSearch(location);
    const qParam = String(params.get("q") || "").trim();
    if (qParam) {
      setQ(qParam);
    }
    const assigneeParam = String(params.get("assignee") || "").trim();
    if (assigneeParam) {
      setAssigneeID(assigneeParam);
    }
    const rawLimit = Number.parseInt(String(params.get("limit") || "").trim(), 10);
    if (Number.isFinite(rawLimit) && STREAM_LIMIT_OPTIONS.includes(rawLimit as (typeof STREAM_LIMIT_OPTIONS)[number])) {
      setLimit(rawLimit);
    }
    const rawPage = Number.parseInt(String(params.get("page") || "").trim(), 10);
    if (Number.isFinite(rawPage) && rawPage > 0) {
      setPage(rawPage);
    }
    const typesParam = String(params.get("types") || "").trim().toLowerCase();
    if (typesParam) {
      const parsedTypes = Array.from(
        new Set(
          typesParam
            .split(",")
            .map((item) => item.trim())
            .filter((item): item is StreamEntityType => STREAM_ENTITY_TYPES.includes(item as StreamEntityType)),
        ),
      );
      if (parsedTypes.length > 0) {
        setEnabledTypes({
          case: parsedTypes.includes("case"),
          alert: parsedTypes.includes("alert"),
          task: parsedTypes.includes("task"),
        });
      }
    }
    const pausedParam = String(params.get("paused") || "").trim().toLowerCase();
    if (pausedParam === "1" || pausedParam === "true") {
      setPaused(true);
    }
    queryHydratedRef.current = true;
  }, []);

  useEffect(() => {
    if (!queryHydratedRef.current) return;
    const nextLocation = applySearchPatch(location, {
      q: q.trim() || undefined,
      assignee: assigneeID !== "all" ? assigneeID : undefined,
      limit: limit !== 200 ? String(limit) : undefined,
      page: page > 1 ? String(page) : undefined,
      types: selectedTypesQuery !== "case,alert,task" ? selectedTypesQuery : undefined,
      paused: paused ? "1" : undefined,
    });
    const currentLocation =
      typeof window !== "undefined" ? `${window.location.pathname}${window.location.search}` : location;
    if (nextLocation !== currentLocation) {
      setLocation(nextLocation, { replace: true });
    }
  }, [q, assigneeID, limit, page, selectedTypesQuery, paused, location, setLocation]);

  const streamOffset = useMemo(() => Math.max(0, (page - 1) * limit), [page, limit]);

  const { data, isLoading: streamLoading } = useActivityLivestream(currentTenantId, {
    q,
    types: selectedTypes,
    assigneeId: assigneeID === "all" ? "" : assigneeID,
    limit,
    offset: streamOffset,
    refetchInterval: paused ? 0 : 3000,
    refetchOnWindowFocus: !paused,
  });
  const showActivityPageSkeleton = useMinimumLoading(streamLoading || usersLoading);

  const normalizedItems = useMemo(() => {
    const items = data?.items || [];
    return [...items].sort((left: any, right: any) => {
      const leftTimestamp = Date.parse(String(left?.updatedAt || left?.createdAt || ""));
      const rightTimestamp = Date.parse(String(right?.updatedAt || right?.createdAt || ""));
      if (!Number.isFinite(leftTimestamp) || !Number.isFinite(rightTimestamp)) {
        return String(right?.id || "").localeCompare(String(left?.id || ""));
      }
      return rightTimestamp - leftTimestamp;
    });
  }, [data?.items]);
  const hasMore = Boolean(data?.hasMore);
  const visibleFrom = normalizedItems.length > 0 ? streamOffset + 1 : 0;
  const visibleTo = streamOffset + normalizedItems.length;
  const headerEventsLabel = hasMore ? `${visibleTo}+ events` : `${visibleTo} events`;

  useEffect(() => {
    if (page <= 1 || streamLoading) {
      return;
    }
    if (normalizedItems.length === 0) {
      setPage(1);
    }
  }, [page, streamLoading, normalizedItems.length]);

  const generatedAtLabel = useMemo(() => {
    const raw = String(data?.generatedAt || "");
    const parsed = Date.parse(raw);
    if (!Number.isFinite(parsed)) {
      return "";
    }
    return new Date(parsed).toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    });
  }, [data?.generatedAt]);

  const streamStats = useMemo(() => {
    let alerts = 0;
    let cases = 0;
    let tasks = 0;
    for (const item of normalizedItems) {
      const entity = String(item?.entity || "").trim().toLowerCase();
      if (entity === "alert") alerts += 1;
      else if (entity === "case") cases += 1;
      else if (entity === "task") tasks += 1;
    }
    return { alerts, cases, tasks };
  }, [normalizedItems]);

  const openItem = (item: any) => {
    if (item.entity === "alert") {
      setLocation(withTenantPath(currentTenantSlug, `/alerts/${item.entityId}`));
      return;
    }
    if (item.entity === "case") {
      setLocation(withTenantPath(currentTenantSlug, `/cases/${item.entityId}`));
      return;
    }
    if (item.entity === "task" && item.caseId) {
      setLocation(withTenantPath(currentTenantSlug, `/cases/${item.caseId}`));
      return;
    }
    setLocation(withTenantPath(currentTenantSlug, "/cases"));
  };

  const toggleType = (type: StreamEntityType) => {
    setEnabledTypes((current) => {
      const enabledCount = STREAM_ENTITY_TYPES.reduce((sum, key) => sum + (current[key] ? 1 : 0), 0);
      if (current[type] && enabledCount <= 1) {
        return current;
      }
      return { ...current, [type]: !current[type] };
    });
    setPage(1);
  };

  if (showActivityPageSkeleton) {
    return (
      <AppLayout>
        <section className={ACTIVITY_PAGE_SHELL_CLASS}>
          <div className={ACTIVITY_PANEL_CLASS}>
            <div className="space-y-2">
              <Skeleton className="h-8 w-56 rounded-lg" />
              <Skeleton className="h-4 w-56 rounded-lg" />
            </div>
            <div className="mt-4 grid grid-cols-1 gap-2 lg:grid-cols-[1fr_190px_142px_auto]">
              <Skeleton className="h-[38px] w-full rounded-lg" />
              <Skeleton className="h-[38px] w-full rounded-lg" />
              <Skeleton className="h-[38px] w-full rounded-lg lg:col-span-2" />
            </div>
            <div className="mt-4 grid grid-cols-1 gap-2 sm:grid-cols-3">
              <Skeleton className="h-14 w-full rounded-lg" />
              <Skeleton className="h-14 w-full rounded-lg" />
              <Skeleton className="h-14 w-full rounded-lg" />
            </div>
            <div className="mt-5 space-y-2">
              {Array.from({ length: 4 }).map((_, index) => (
                <Skeleton key={`activity-loading-item-${index}`} className="h-[98px] w-full rounded-lg" />
              ))}
            </div>
          </div>
        </section>
      </AppLayout>
    );
  }

  return (
    <AppLayout>
      <section className={ACTIVITY_PAGE_SHELL_CLASS}>
        <div className={ACTIVITY_PANEL_CLASS}>
          <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div className="flex min-w-0 items-start gap-3 md:items-center">
              <div className="mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-[rgba(235,95,101,0.2)] text-[#eb5f65]">
                <LivestreamGlyph />
              </div>
              <div>
                <h1 className="text-lg font-semibold leading-7 tracking-[-0.4px] text-white">Live Activity Stream</h1>
                <p className={`${ACTIVITY_MUTED_TEXT_CLASS} leading-4 tracking-[-0.4px]`}>
                  Real-time security events
                  {generatedAtLabel ? <span className="ml-2 text-[#8b91a3]">Updated {generatedAtLabel}</span> : null}
                </p>
              </div>
            </div>
            <div className="flex w-full items-center gap-2 md:w-[354px]">
              <div className="hidden h-[30px] shrink-0 items-center rounded-md border border-[#2a2c3c] bg-[#10131e] px-2 text-[11px] text-[#8b91a3] md:inline-flex">
                {headerEventsLabel}
              </div>
              <label className="relative block w-full md:w-64">
                <Search size={12} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[#6b7280]" />
                <input
                  value={q}
                  onChange={(event) => {
                    setQ(event.target.value);
                    setPage(1);
                  }}
                  placeholder="Filter events..."
                  className={`${FILTER_INPUT_CLASS} w-full pl-9`}
                  data-testid="activity-filter-search"
                />
              </label>
              <button
                type="button"
                onClick={() => setPaused((current) => !current)}
                className="inline-flex h-[38px] w-[34.75px] shrink-0 items-center justify-center rounded-lg border border-[#2a2c3c] bg-[#0f1118] text-[#9ca3af] transition-colors hover:border-[#4b5168] hover:bg-[#1a1f2d]"
                aria-label={paused ? "Resume livestream updates" : "Pause livestream updates"}
                title={paused ? "Resume updates" : "Pause updates"}
              >
                {paused ? <Play size={13} /> : <Pause size={13} />}
              </button>
            </div>
          </div>

          <div className="mt-3 grid grid-cols-1 gap-2 lg:grid-cols-[1fr_190px_142px_auto]">
            <Select
              value={assigneeID}
              onValueChange={(value) => {
                setAssigneeID(value);
                setPage(1);
              }}
            >
              <SelectTrigger className={FILTER_SELECT_TRIGGER_CLASS} data-testid="activity-filter-assignee">
                <SelectValue placeholder="All assignees" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All assignees</SelectItem>
                {users.map((user: any) => (
                  <SelectItem key={user.id} value={String(user.id)}>
                    {String(user.name || user.email || user.id)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select
              value={String(limit)}
              onValueChange={(value) => {
                setLimit(Number(value) || 200);
                setPage(1);
              }}
            >
              <SelectTrigger className={FILTER_SELECT_TRIGGER_CLASS} data-testid="activity-filter-limit">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {STREAM_LIMIT_OPTIONS.map((value) => (
                  <SelectItem key={value} value={String(value)}>
                    {value} events
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <div className="col-span-1 flex items-center gap-2 lg:col-span-2" data-testid="activity-filter-types">
              {STREAM_ENTITY_TYPES.map((type) => {
                const active = enabledTypes[type];
                const label = type === "case" ? "Cases" : type === "alert" ? "Alerts" : "Tasks";
                return (
                  <button
                    key={type}
                    type="button"
                    onClick={() => toggleType(type)}
                    className={`h-[38px] rounded-lg border px-3 text-xs font-medium transition-colors ${active ? TYPE_TOGGLE_ACTIVE_CLASS : TYPE_TOGGLE_INACTIVE_CLASS}`}
                    aria-pressed={active}
                    data-testid={`activity-filter-type-${type}`}
                  >
                    {label}
                  </button>
                );
              })}
            </div>
          </div>

          <div className="mt-4 grid grid-cols-1 gap-2 sm:grid-cols-3">
            <div className="rounded-lg border border-[#2a2c3c] bg-[#10131d] px-3 py-2">
              <div className="text-[11px] uppercase tracking-[0.12em] text-[#8b91a3]">Alerts</div>
              <div className="mt-1 text-sm font-semibold text-[#f3f4f6]">{streamStats.alerts}</div>
            </div>
            <div className="rounded-lg border border-[#2a2c3c] bg-[#10131d] px-3 py-2">
              <div className="text-[11px] uppercase tracking-[0.12em] text-[#8b91a3]">Cases</div>
              <div className="mt-1 text-sm font-semibold text-[#f3f4f6]">{streamStats.cases}</div>
            </div>
            <div className="rounded-lg border border-[#2a2c3c] bg-[#10131d] px-3 py-2">
              <div className="text-[11px] uppercase tracking-[0.12em] text-[#8b91a3]">Tasks</div>
              <div className="mt-1 text-sm font-semibold text-[#f3f4f6]">{streamStats.tasks}</div>
            </div>
          </div>

          <div className="mt-5 space-y-2">
            {normalizedItems.map((item: any) => {
              const palette = paletteForItem(item);
              const typeLabel = buildTypeLabel(item);
              const sourceLabel = buildSourceLabel(item);
              return (
                <div
                  key={item.id}
                  onClick={() => openItem(item)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter" || event.key === " ") {
                      event.preventDefault();
                      openItem(item);
                    }
                  }}
                  role="button"
                  tabIndex={0}
                  className="relative block h-[98px] w-full rounded-lg border border-[#2a2c3c] bg-[#10131d]/65 px-3 py-3 text-left transition-colors hover:border-[#4b5168] hover:bg-[#171b2a]"
                >
                  <span className={`absolute left-3 top-[19px] h-2 w-2 rounded-full ${palette.dot}`} />
                  <div className="ml-5 min-w-0">
                    <div className="flex items-start justify-between gap-3">
                      <p className="truncate pr-2 text-sm font-normal tracking-[-0.5px] text-white">{buildTitle(item)}</p>
                      <span className="shrink-0 pt-0.5 text-xs text-[#8b91a3]">
                        {formatRelativeCompact(item.updatedAt || item.createdAt)}
                      </span>
                    </div>
                    <p className="mt-1 truncate text-xs tracking-[-0.5px] text-[#9ca3af]">{item.description || "No details available"}</p>
                    <div className="mt-2 flex flex-wrap items-center gap-x-6 gap-y-1">
                      <span className={`rounded bg-[#151b2b] px-1.5 py-0.5 text-[10px] tracking-[-0.5px] ${palette.text}`}>{typeLabel}</span>
                      <span className="text-xs tracking-[-0.5px] text-[#8b91a3]">{sourceLabel}</span>
                      <button
                        type="button"
                        onClick={(event) => {
                          event.stopPropagation();
                          openItem(item);
                        }}
                        className="ml-auto inline-flex h-6 items-center rounded-md border border-[#2a2c3c] bg-[#0f1118] px-2 text-[10px] font-medium uppercase tracking-[0.08em] text-[#d1d5db] transition-colors hover:border-[#4b5168] hover:bg-[#1a1f2d]"
                        data-testid={`activity-open-${item.id}`}
                      >
                        Open
                      </button>
                    </div>
                  </div>
                </div>
              );
            })}
            {normalizedItems.length === 0 ? (
              <div className="flex min-h-[360px] items-center justify-center rounded-lg border border-dashed border-[#2a2c3c] bg-[#10121c]/70 px-4 text-sm text-[#8b91a3]">
                No activity for selected filters.
              </div>
            ) : null}
          </div>
          <div className="mt-3 flex flex-col gap-2 border-t border-[#2a2c3c] pt-3 sm:flex-row sm:items-center sm:justify-between">
            <p className={ACTIVITY_MUTED_TEXT_CLASS}>
              Showing {visibleFrom}-{visibleTo}
              {hasMore ? " (more available)" : ""}
            </p>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => setPage((current) => Math.max(1, current - 1))}
                disabled={page <= 1}
                className="inline-flex h-[32px] items-center rounded-lg border border-[#2a2c3c] bg-[#0f1118] px-3 text-xs font-medium text-[#d1d5db] transition-colors hover:border-[#4b5168] hover:bg-[#1a1f2d] disabled:cursor-not-allowed disabled:opacity-45"
              >
                <ChevronLeft size={13} className="mr-1" />
                Prev
              </button>
              <span className="min-w-[58px] text-center text-xs font-medium text-[#d1d5db]">Page {page}</span>
              <button
                type="button"
                onClick={() => setPage((current) => current + 1)}
                disabled={!hasMore}
                className="inline-flex h-[32px] items-center rounded-lg border border-[#2a2c3c] bg-[#0f1118] px-3 text-xs font-medium text-[#d1d5db] transition-colors hover:border-[#4b5168] hover:bg-[#1a1f2d] disabled:cursor-not-allowed disabled:opacity-45"
              >
                Next
                <ChevronRight size={13} className="ml-1" />
              </button>
            </div>
          </div>
        </div>
      </section>
    </AppLayout>
  );
}
