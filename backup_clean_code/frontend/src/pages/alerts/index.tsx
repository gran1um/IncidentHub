import { AppLayout } from "@/components/layout";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Search, Clock, CheckCircle2, Calendar, Tag, X, Link2, Plus, Layers, Trash2, ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Calendar as CalendarComponent } from "@/components/ui/calendar";
import { format } from "date-fns";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useLocation } from "wouter";
import {
  type AssignedFilterMode,
  useAppState,
  useAlertsPage,
  useUser,
  useUsers,
  useUpdateAlert,
  useCreateAlert,
  useCases,
  useBindAlertsToCase,
  useCreateCaseFromAlerts,
  useCaseStatuses,
  useDeleteAlert,
  useDeleteAlertsBulk,
} from "@/lib/api";
import { Skeleton } from "@/components/ui/skeleton";
import { useT } from "@/lib/i18n";
import { Checkbox } from "@/components/ui/checkbox";
import { EllipsisText } from "@/components/ui/ellipsis-text";
import { applySearchPatch, formatDateParam, parseDateParam, parsePositiveIntParam, splitLocationPathAndSearch } from "@/lib/url-state";
import { UserAvatar } from "@/components/user-avatar";
import { withTenantPath } from "@/lib/tenant-url";
import { useMinimumLoading } from "@/lib/use-minimum-loading";
import { CollapsibleFiltersPanel } from "@/components/collapsible-filters-panel";

type DateRange = { from: Date | undefined; to: Date | undefined };
type AlertSearchMode = "plain" | "regex" | "fulltext";
type AlertSearchLogic = "all" | "any";

const PAGE_SIZE_OPTIONS = [10, 30, 50, 100] as const;
const ALERT_SORT_FIELD_OPTIONS = [
  { value: "updated_at", label: "Updated" },
  { value: "created_at", label: "Created" },
  { value: "severity", label: "Severity" },
  { value: "status", label: "Status" },
  { value: "source", label: "Source" },
  { value: "title", label: "Title" },
] as const;
const ALERT_SORT_ORDER_OPTIONS = [
  { value: "desc", label: "Descending" },
  { value: "asc", label: "Ascending" },
] as const;
const ALERT_GROUP_OPTIONS = [
  { value: "none", label: "No Grouping" },
  { value: "status", label: "Status" },
  { value: "severity", label: "Severity" },
  { value: "owner", label: "Assignee" },
  { value: "source", label: "Source" },
  { value: "linked", label: "Case Link" },
  { value: "date", label: "Created Date" },
] as const;
const ALERT_SORT_FIELD_SET = new Set(ALERT_SORT_FIELD_OPTIONS.map((item) => item.value));
const ALERT_SORT_ORDER_SET = new Set(ALERT_SORT_ORDER_OPTIONS.map((item) => item.value));
const ALERT_GROUP_SET = new Set(ALERT_GROUP_OPTIONS.map((item) => item.value));
const DARK_SELECT_CONTENT_CLASS = "rounded-xl border border-[#2a2c3c] bg-[#13141c] text-white";
const DARK_DIALOG_INPUT_CLASS = "border border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] placeholder:text-[#6b7280]";
const DARK_DIALOG_TRIGGER_CLASS = "border border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db]";
const ALERT_PAGE_SHELL_CLASS = "mx-auto w-full max-w-[1144px] space-y-6 pb-6";
const ALERT_PANEL_CLASS =
  "rounded-2xl border border-[rgba(255,255,255,0.06)] bg-[linear-gradient(180deg,rgba(19,20,28,0.97),rgba(17,20,32,0.97))] shadow-[0_14px_34px_rgba(0,0,0,0.28)]";
const ALERT_SUBPANEL_CLASS = "rounded-xl border border-[#2a2c3c] bg-[#111624]";
const ALERT_MUTED_TEXT_CLASS = "text-xs text-[#8b91a3]";
const ALERT_ACTION_BUTTON_CLASS =
  "h-9 rounded-lg border border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]";
const ALERT_INPUT_CLASS =
  "h-[38px] rounded-lg border border-[#2a2c3c] bg-[#0b0c10] text-sm text-[#d1d5db] placeholder:text-[#6b7280] focus-visible:ring-1 focus-visible:ring-[#3b4a79]";
const ALERT_SELECT_TRIGGER_CLASS =
  "h-[38px] rounded-lg border border-[#2a2c3c] bg-[#0b0c10] text-sm text-[#d1d5db] focus:ring-1 focus:ring-[#3b4a79]";
const ALERT_FILTERS_COLLAPSED_STORAGE_KEY = "incidenthub-alerts-filters-collapsed";

type AlertGroupBy = (typeof ALERT_GROUP_OPTIONS)[number]["value"];
type AlertGroupSection = {
  key: string;
  label: string;
  items: any[];
};

function AlertsLoadingSkeleton() {
  return (
    <AppLayout>
      <div className={ALERT_PAGE_SHELL_CLASS}>
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div className="space-y-2">
            <Skeleton className="h-8 w-40 rounded-md" />
            <Skeleton className="h-4 w-80 max-w-full rounded-md" />
          </div>
          <Skeleton className="h-11 w-36 rounded-xl" />
        </div>

        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          {Array.from({ length: 4 }).map((_, index) => (
            <Card key={`alerts-loading-kpi-${index}`} className={`${ALERT_PANEL_CLASS} rounded-xl px-4 py-3`}>
              <Skeleton className="h-3 w-16 rounded-md" />
              <Skeleton className="mt-2 h-6 w-12 rounded-md" />
            </Card>
          ))}
        </div>

        <Card className={`${ALERT_PANEL_CLASS} p-5`}>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
            <div className="space-y-2">
              <Skeleton className="h-4 w-20 rounded-md" />
              <Skeleton className="h-[38px] w-full rounded-lg" />
            </div>
            {Array.from({ length: 3 }).map((_, index) => (
              <div key={`alerts-loading-main-filter-${index}`} className="space-y-2">
                <Skeleton className="h-4 w-16 rounded-md" />
                <Skeleton className="h-[38px] w-full rounded-lg" />
              </div>
            ))}

            {Array.from({ length: 2 }).map((_, index) => (
              <div key={`alerts-loading-toggle-${index}`} className="space-y-2">
                <Skeleton className="h-4 w-20 rounded-md" />
                <div className="flex h-[38px] items-center gap-1 rounded-lg border border-[#2a2c3c] bg-[#0b0c10] p-1">
                  <Skeleton className="h-7 flex-1 rounded-md" />
                  <Skeleton className="h-7 flex-1 rounded-md" />
                  <Skeleton className="h-7 flex-1 rounded-md" />
                </div>
              </div>
            ))}

            {Array.from({ length: 8 }).map((_, index) => (
              <div key={`alerts-loading-secondary-filter-${index}`} className="space-y-2">
                <Skeleton className="h-4 w-16 rounded-md" />
                <Skeleton className="h-[38px] w-full rounded-lg" />
              </div>
            ))}
          </div>

          <div className="mt-4 flex flex-wrap items-center justify-between gap-3 border-t border-[#2a2c3c] pt-3">
            <Skeleton className="h-4 w-28 rounded-md" />
            <Skeleton className="h-7 w-32 rounded-md" />
          </div>
          <div className="mt-3 flex flex-wrap gap-2">
            <Skeleton className="h-6 w-20 rounded-md" />
            <Skeleton className="h-6 w-24 rounded-md" />
            <Skeleton className="h-6 w-16 rounded-md" />
          </div>
        </Card>

        <Card className={`${ALERT_PANEL_CLASS} p-4`}>
          <div className="flex flex-col gap-3 xl:flex-row xl:items-center">
            <div className="flex items-center gap-3">
              <Skeleton className="h-4 w-4 rounded-sm" />
              <Skeleton className="h-5 w-36 rounded-md" />
              <Skeleton className="h-6 w-24 rounded-md" />
            </div>
            <div className="flex-1" />
            <div className="flex flex-col gap-2 md:flex-row md:items-center">
              <Skeleton className="h-9 w-full rounded-lg md:w-64" />
              <Skeleton className="h-9 w-full rounded-xl md:w-[156px]" />
              <Skeleton className="h-9 w-full rounded-xl md:w-[174px]" />
              <Skeleton className="h-9 w-full rounded-xl md:w-[172px]" />
            </div>
          </div>
          <div className="mt-3 grid gap-2 md:grid-cols-[1fr,1fr,auto,auto]">
            <Skeleton className="h-10 w-full rounded-xl" />
            <Skeleton className="h-10 w-full rounded-xl" />
            <Skeleton className="h-10 w-[164px] rounded-xl" />
            <div className="flex gap-2">
              <Skeleton className="h-10 w-[124px] rounded-xl" />
              <Skeleton className="h-10 w-[132px] rounded-xl" />
            </div>
          </div>
        </Card>

        <Card className={`${ALERT_PANEL_CLASS} p-4`}>
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
              <Skeleton className="h-9 w-[102px] rounded-lg" />
              <Skeleton className="h-4 w-40 rounded-md" />
            </div>
            <div className="flex items-center gap-2">
              <Skeleton className="h-9 w-9 rounded-lg" />
              <Skeleton className="h-8 w-24 rounded-md" />
              <Skeleton className="h-9 w-9 rounded-lg" />
            </div>
          </div>
        </Card>

        <div className={`${ALERT_SUBPANEL_CLASS} px-4 py-2.5`}>
          <Skeleton className="h-4 w-52 rounded-md" />
        </div>

        <div className="grid gap-3">
          {Array.from({ length: 4 }).map((_, index) => (
            <Card key={`alerts-loading-card-${index}`} className={`${ALERT_PANEL_CLASS} overflow-hidden rounded-xl p-5`}>
              <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
                <div className="flex min-w-0 flex-1 items-start gap-3">
                  <Skeleton className="mt-1.5 h-4 w-4 rounded-sm" />
                  <Skeleton className="h-11 w-11 rounded-lg" />
                  <div className="min-w-0 flex-1 space-y-3">
                    <div className="flex flex-wrap items-center gap-2">
                      <Skeleton className="h-5 w-20 rounded-md" />
                      <Skeleton className="h-5 w-16 rounded-md" />
                      <Skeleton className="h-4 w-28 rounded-md" />
                    </div>
                    <Skeleton className="h-8 w-[72%] rounded-md" />
                    <div className="space-y-2">
                      <Skeleton className="h-4 w-full rounded-md" />
                      <Skeleton className="h-4 w-3/4 rounded-md" />
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                      <Skeleton className="h-4 w-28 rounded-md" />
                      <Skeleton className="h-4 w-24 rounded-md" />
                      <Skeleton className="h-4 w-36 rounded-md" />
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                      <Skeleton className="h-5 w-16 rounded-md" />
                      <Skeleton className="h-5 w-20 rounded-md" />
                      <Skeleton className="h-5 w-14 rounded-md" />
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-3 pr-1 md:pt-1">
                  <Skeleton className="h-9 w-[118px] rounded-xl" />
                  <Skeleton className="h-7 w-[94px] rounded-full" />
                </div>
              </div>
            </Card>
          ))}
        </div>

        <Card className={`${ALERT_PANEL_CLASS} p-4`}>
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
              <Skeleton className="h-9 w-[102px] rounded-lg" />
              <Skeleton className="h-4 w-40 rounded-md" />
            </div>
            <div className="flex items-center gap-2">
              <Skeleton className="h-9 w-9 rounded-lg" />
              <Skeleton className="h-8 w-24 rounded-md" />
              <Skeleton className="h-9 w-9 rounded-lg" />
            </div>
          </div>
        </Card>
      </div>
    </AppLayout>
  );
}

export default function AlertsPage() {
  const t = useT();
  const [location, setLocation] = useLocation();
  const [search, setSearch] = useState("");
  const [searchMode, setSearchMode] = useState<AlertSearchMode>("plain");
  const [searchLogic, setSearchLogic] = useState<AlertSearchLogic>("all");
  const [severity, setSeverity] = useState<string>("all");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [sourceFilter, setSourceFilter] = useState<string>("all");
  const [assignedFilter, setAssignedFilter] = useState<AssignedFilterMode>("all");
  const [assigneeFilter, setAssigneeFilter] = useState<string>("all");
  const [linkedFilter, setLinkedFilter] = useState<string>("all");
  const [dateRange, setDateRange] = useState<DateRange>({ from: undefined, to: undefined });
  const [tagInput, setTagInput] = useState("");
  const [activeTags, setActiveTags] = useState<string[]>([]);
  const [selectedAlertIds, setSelectedAlertIds] = useState<string[]>([]);
  const [bulkAlertStatus, setBulkAlertStatus] = useState("");
  const [bulkAlertTag, setBulkAlertTag] = useState("");
  const [bulkAlertPending, setBulkAlertPending] = useState(false);
  const [deleteAlertDialogOpen, setDeleteAlertDialogOpen] = useState(false);
  const [deleteAlertIDs, setDeleteAlertIDs] = useState<string[]>([]);
  const [targetCaseId, setTargetCaseId] = useState("");
  const [createAlertDialogOpen, setCreateAlertDialogOpen] = useState(false);
  const [createCaseDialogOpen, setCreateCaseDialogOpen] = useState(false);
  const [newAlert, setNewAlert] = useState({
    title: "",
    description: "",
    source: "manual",
    severity: "High",
    status: "new",
    tlp: "amber",
    pap: "amber",
  });
  const [newCase, setNewCase] = useState({
    title: "",
    description: "",
    source: "alerts-bulk",
    severity: "High",
    status: "",
    priority: "medium",
  });
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState<number>(30);
  const [sortBy, setSortBy] = useState<string>("updated_at");
  const [sortOrder, setSortOrder] = useState<"asc" | "desc">("desc");
  const [groupBy, setGroupBy] = useState<AlertGroupBy>("none");
  const [pageInput, setPageInput] = useState("1");
  const queryHydratedRef = useRef(false);

  const { currentTenantId, currentTenantSlug, currentUserId } = useAppState();
  const { data: alertsPageData, isLoading: alertsPageLoading } = useAlertsPage(
    currentTenantId,
    page,
    pageSize,
    assignedFilter,
    search,
    sortBy,
    sortOrder,
    searchMode,
    searchLogic,
    assigneeFilter !== "all" && assignedFilter !== "mine" && assignedFilter !== "unassigned" ? assigneeFilter : "",
  );
  const alerts = alertsPageData?.items || [];
  const totalAlerts = Number(alertsPageData?.total || 0);
  const totalPages = Math.max(1, Number(alertsPageData?.totalPages || 1));
  const { data: currentUser } = useUser(currentUserId);
  const { data: tenantUsers = [], isLoading: tenantUsersLoading } = useUsers(currentTenantId);
  const { data: cases = [], isLoading: casesLoading } = useCases(currentTenantId);
  const { data: caseStatuses = [], isLoading: caseStatusesLoading } = useCaseStatuses(currentTenantId);
  const updateAlert = useUpdateAlert();
  const createAlert = useCreateAlert();
  const bindAlertsToCase = useBindAlertsToCase();
  const createCaseFromAlerts = useCreateCaseFromAlerts();
  const deleteAlert = useDeleteAlert();
  const deleteAlertsBulk = useDeleteAlertsBulk();

  const usersByID = useMemo(() => {
    const map = new Map<string, any>();
    tenantUsers.forEach((user: any) => {
      if (!user?.id) return;
      map.set(String(user.id), user);
    });
    return map;
  }, [tenantUsers]);
  const alertsByID = useMemo(() => {
    const map = new Map<string, any>();
    alerts.forEach((item: any) => {
      const key = String(item?.id || "").trim();
      if (!key) return;
      map.set(key, item);
    });
    return map;
  }, [alerts]);

  const statusOptions = useMemo(() => {
    if (caseStatuses.length > 0) return caseStatuses;
    return [
      { code: "new", label: "New", isClosed: false },
      { code: "open", label: "Open", isClosed: false },
      { code: "resolved", label: "Resolved", isClosed: true },
      { code: "closed", label: "Closed", isClosed: true },
    ];
  }, [caseStatuses]);
  const alertStatusOptions = useMemo(
    () => [
      { code: "new", label: "New" },
      { code: "triaged", label: "Triaged" },
      { code: "closed", label: "Closed" },
    ],
    [],
  );

  useEffect(() => {
    if (newCase.status) return;
    const openStatus = statusOptions.find((item: any) => !item.isClosed)?.code || "new";
    setNewCase((prev) => ({ ...prev, status: openStatus }));
  }, [newCase.status, statusOptions]);

  const sourceOptions = useMemo(() => {
    const unique = new Set<string>();
    alerts.forEach((item: any) => {
      const source = (item.source || "").trim();
      if (source) unique.add(source);
    });
    return Array.from(unique).sort((a, b) => a.localeCompare(b));
  }, [alerts]);

  const statusFilterOptions = useMemo(() => {
    const unique = new Set<string>();
    alerts.forEach((item: any) => {
      const status = (item.status || "").trim();
      if (status) unique.add(status);
    });
    return Array.from(unique).sort((a, b) => a.localeCompare(b));
  }, [alerts]);

  const filteredAlerts = useMemo(() => {
    return alerts.filter((item: any) => {
      const searchTerms = search.trim().toLowerCase().split(/\s+/).filter(Boolean);
      if (search.trim() && searchMode !== "fulltext") {
        const ownerName = item.owner ? String(usersByID.get(String(item.owner))?.name || "") : "";
        const haystack = [
          item.id,
          item.title,
          item.description,
          item.source,
          item.status,
          item.sev,
          item.caseId,
          ownerName,
        ]
          .filter(Boolean)
          .join(" ")
          .toLowerCase();
        if (searchMode === "regex") {
          try {
            const searchRegex = new RegExp(search.trim(), "i");
            if (!searchRegex.test(haystack)) {
              return false;
            }
          } catch {
            return false;
          }
        } else {
          const matched = searchLogic === "all"
            ? searchTerms.every((term) => haystack.includes(term))
            : searchTerms.some((term) => haystack.includes(term));
          if (!matched) return false;
        }
      }

      if (severity !== "all" && item.sev !== severity) return false;
      if (statusFilter !== "all" && item.status !== statusFilter) return false;
      if (sourceFilter !== "all" && item.source !== sourceFilter) return false;

      if (assignedFilter === "assigned" && !item.owner) return false;
      if (assignedFilter === "unassigned" && !!item.owner) return false;
      if (assignedFilter === "mine" && item.owner !== currentUserId) return false;
      if (assigneeFilter !== "all" && item.owner !== assigneeFilter) return false;

      if (linkedFilter === "linked" && !item.caseId) return false;
      if (linkedFilter === "unlinked" && !!item.caseId) return false;

      const itemDate = new Date(item.time);
      if (dateRange.from && itemDate < dateRange.from) return false;
      if (dateRange.to && itemDate > dateRange.to) return false;

      if (
        activeTags.length > 0 &&
        !activeTags.every((tag) => item.tags.map((itemTag: string) => itemTag.toLowerCase()).includes(tag.toLowerCase()))
      ) {
        return false;
      }
      return true;
    });
  }, [alerts, search, searchMode, searchLogic, severity, statusFilter, sourceFilter, assignedFilter, assigneeFilter, linkedFilter, dateRange, activeTags, currentUserId, usersByID]);

  const groupedAlerts = useMemo<AlertGroupSection[]>(() => {
    const normalizeDate = (value: string): string => {
      const parsed = new Date(value);
      if (Number.isNaN(parsed.getTime())) return "Unknown date";
      return format(parsed, "yyyy-MM-dd");
    };
    const resolveGroupLabel = (item: any): string => {
      switch (groupBy) {
        case "status":
          return String(item.status || "Unknown status");
        case "severity":
          return String(item.sev || "Unknown severity");
        case "owner": {
          if (!item.owner) return "Unassigned";
          const ownerUser = usersByID.get(String(item.owner));
          return String(ownerUser?.name || ownerUser?.email || item.owner || "Unknown assignee");
        }
        case "source":
          return String(item.source || "Unknown source");
        case "linked":
          return item.caseId ? "Linked to case" : "Unlinked";
        case "date":
          return normalizeDate(String(item.time || ""));
        default:
          return "All alerts";
      }
    };

    if (groupBy === "none") {
      return [{ key: "all", label: "All alerts", items: filteredAlerts }];
    }
    const buckets = new Map<string, AlertGroupSection>();
    filteredAlerts.forEach((item) => {
      const label = resolveGroupLabel(item);
      const key = label.toLowerCase();
      const existing = buckets.get(key);
      if (existing) {
        existing.items.push(item);
        return;
      }
      buckets.set(key, { key, label, items: [item] });
    });
    return Array.from(buckets.values());
  }, [filteredAlerts, groupBy, usersByID]);

  const resetAlertFilters = () => {
    setSearch("");
    setSearchMode("plain");
    setSearchLogic("all");
    setSeverity("all");
    setStatusFilter("all");
    setSourceFilter("all");
    setAssignedFilter("all");
    setAssigneeFilter("all");
    setLinkedFilter("all");
    setDateRange({ from: undefined, to: undefined });
    setTagInput("");
    setActiveTags([]);
    setSortBy("updated_at");
    setSortOrder("desc");
    setGroupBy("none");
    setPage(1);
  };

  const activeFiltersCount = useMemo(() => {
    let count = 0;
    if (search.trim()) count += 1;
    if (searchMode !== "plain") count += 1;
    if (searchLogic !== "all") count += 1;
    if (severity !== "all") count += 1;
    if (statusFilter !== "all") count += 1;
    if (sourceFilter !== "all") count += 1;
    if (assignedFilter !== "all") count += 1;
    if (assigneeFilter !== "all") count += 1;
    if (linkedFilter !== "all") count += 1;
    if (dateRange.from || dateRange.to) count += 1;
    if (activeTags.length > 0) count += 1;
    if (sortBy !== "updated_at" || sortOrder !== "desc") count += 1;
    if (groupBy !== "none") count += 1;
    return count;
  }, [
    search,
    searchMode,
    searchLogic,
    severity,
    statusFilter,
    sourceFilter,
    assignedFilter,
    assigneeFilter,
    linkedFilter,
    dateRange.from,
    dateRange.to,
    activeTags.length,
    sortBy,
    sortOrder,
    groupBy,
  ]);

  const alertFilterSummaryItems = useMemo(() => {
    const items: Array<{ key: string; label: string }> = [];
    const normalizedSearch = search.trim();
    if (normalizedSearch) {
      items.push({ key: "search", label: `Search: ${normalizedSearch}` });
    }
    if (searchMode !== "plain") {
      items.push({ key: "search-mode", label: `Mode: ${searchMode}` });
    }
    if (searchLogic !== "all") {
      items.push({ key: "search-logic", label: `Match: ${searchLogic}` });
    }
    if (severity !== "all") {
      items.push({ key: "severity", label: `Severity: ${severity}` });
    }
    if (statusFilter !== "all") {
      items.push({ key: "status", label: `Status: ${statusFilter}` });
    }
    if (sourceFilter !== "all") {
      items.push({ key: "source", label: `Source: ${sourceFilter}` });
    }
    if (assigneeFilter !== "all") {
      const assignee = usersByID.get(String(assigneeFilter));
      const label = String(assignee?.name || assignee?.email || assigneeFilter);
      items.push({ key: "assignee", label: `Assignee: ${label}` });
    } else if (assignedFilter !== "all") {
      items.push({ key: "owner", label: `Owner: ${assignedFilter}` });
    }
    if (linkedFilter !== "all") {
      items.push({ key: "linked", label: `Case link: ${linkedFilter}` });
    }
    if (dateRange.from || dateRange.to) {
      const fromLabel = dateRange.from ? format(dateRange.from, "dd.MM.yyyy") : "…";
      const toLabel = dateRange.to ? format(dateRange.to, "dd.MM.yyyy") : "…";
      items.push({ key: "date-range", label: `Date: ${fromLabel} - ${toLabel}` });
    }
    if (activeTags.length > 0) {
      items.push({ key: "tags", label: `Tags: ${activeTags.join(", ")}` });
    }
    if (sortBy !== "updated_at" || sortOrder !== "desc") {
      const sortLabel = ALERT_SORT_FIELD_OPTIONS.find((item) => item.value === sortBy)?.label || sortBy;
      items.push({ key: "sort", label: `Sort: ${sortLabel} (${sortOrder})` });
    }
    if (groupBy !== "none") {
      const groupLabel = ALERT_GROUP_OPTIONS.find((item) => item.value === groupBy)?.label || groupBy;
      items.push({ key: "group", label: `Group: ${groupLabel}` });
    }
    return items;
  }, [
    activeTags,
    assignedFilter,
    assigneeFilter,
    dateRange.from,
    dateRange.to,
    groupBy,
    linkedFilter,
    search,
    searchLogic,
    searchMode,
    severity,
    sortBy,
    sortOrder,
    sourceFilter,
    statusFilter,
    usersByID,
  ]);

  const severitySnapshot = useMemo(() => {
    return filteredAlerts.reduce(
      (acc, item: any) => {
        const sev = String(item?.sev || "").trim().toLowerCase();
        if (sev === "critical") acc.critical += 1;
        else if (sev === "high") acc.high += 1;
        else if (sev === "medium") acc.medium += 1;
        else if (sev === "low") acc.low += 1;
        return acc;
      },
      { critical: 0, high: 0, medium: 0, low: 0 },
    );
  }, [filteredAlerts]);

  useEffect(() => {
    setSelectedAlertIds((prev) => prev.filter((id) => alerts.some((item: any) => item.id === id)));
  }, [alerts]);

  useEffect(() => {
    if (!alertsPageData) return;
    setPage((prev) => Math.min(Math.max(1, prev), totalPages));
  }, [alertsPageData, totalPages]);

  useEffect(() => {
    setPageInput(String(page));
  }, [page]);

  useEffect(() => {
    const { params } = splitLocationPathAndSearch(location);
    const query = params.get("q");
    if (query) {
      setSearch(query);
    }
    const searchModeParam = String(params.get("search_mode") || "").trim().toLowerCase();
    if (searchModeParam === "plain" || searchModeParam === "regex" || searchModeParam === "fulltext") {
      setSearchMode(searchModeParam as AlertSearchMode);
    }
    const searchLogicParam = String(params.get("search_logic") || "").trim().toLowerCase();
    if (searchLogicParam === "all" || searchLogicParam === "any") {
      setSearchLogic(searchLogicParam as AlertSearchLogic);
    }
    const severityParam = params.get("severity");
    if (severityParam) {
      setSeverity(severityParam);
    }
    const statusParam = params.get("status");
    if (statusParam) {
      setStatusFilter(statusParam);
    }
    const sourceParam = params.get("source");
    if (sourceParam) {
      setSourceFilter(sourceParam);
    }
    const assignedParam = params.get("assigned");
    if (assignedParam === "all" || assignedParam === "assigned" || assignedParam === "unassigned" || assignedParam === "mine") {
      setAssignedFilter(assignedParam as AssignedFilterMode);
    }
    const assigneeParam = String(params.get("assignee") || "").trim();
    if (assigneeParam) {
      setAssigneeFilter(assigneeParam);
    }
    const linkedParam = params.get("linked");
    if (linkedParam === "all" || linkedParam === "linked" || linkedParam === "unlinked") {
      setLinkedFilter(linkedParam);
    }
    const tagsParam = params.get("tags");
    if (tagsParam) {
      const parsedTags = tagsParam.split(",").map((item) => item.trim()).filter(Boolean);
      setActiveTags(Array.from(new Set(parsedTags)));
    }
    const from = parseDateParam(params.get("from"));
    const to = parseDateParam(params.get("to"));
    if (from || to) {
      setDateRange({ from, to });
    }
    const parsedPageSize = parsePositiveIntParam(params.get("page_size"), pageSize);
    if (PAGE_SIZE_OPTIONS.includes(parsedPageSize as typeof PAGE_SIZE_OPTIONS[number])) {
      setPageSize(parsedPageSize);
    }
    const sortByParam = String(params.get("sort_by") || "").trim().toLowerCase();
    if (ALERT_SORT_FIELD_SET.has(sortByParam as (typeof ALERT_SORT_FIELD_OPTIONS)[number]["value"])) {
      setSortBy(sortByParam);
    }
    const sortOrderParam = String(params.get("sort_order") || "").trim().toLowerCase();
    if (ALERT_SORT_ORDER_SET.has(sortOrderParam as (typeof ALERT_SORT_ORDER_OPTIONS)[number]["value"])) {
      setSortOrder(sortOrderParam as "asc" | "desc");
    }
    const groupByParam = String(params.get("group_by") || "").trim().toLowerCase();
    if (ALERT_GROUP_SET.has(groupByParam as AlertGroupBy)) {
      setGroupBy(groupByParam as AlertGroupBy);
    }
    const parsedPage = parsePositiveIntParam(params.get("page"), page);
    if (parsedPage >= 1) {
      setPage(parsedPage);
      setPageInput(String(parsedPage));
    }
    queryHydratedRef.current = true;
  }, []);

  useEffect(() => {
    if (!queryHydratedRef.current) return;
    const nextLocation = applySearchPatch(location, {
      q: search.trim() || undefined,
      search_mode: searchMode !== "plain" ? searchMode : undefined,
      search_logic: searchLogic !== "all" ? searchLogic : undefined,
      severity: severity !== "all" ? severity : undefined,
      status: statusFilter !== "all" ? statusFilter : undefined,
      source: sourceFilter !== "all" ? sourceFilter : undefined,
      assigned: assignedFilter !== "all" ? assignedFilter : undefined,
      assignee: assigneeFilter !== "all" ? assigneeFilter : undefined,
      linked: linkedFilter !== "all" ? linkedFilter : undefined,
      from: formatDateParam(dateRange.from),
      to: formatDateParam(dateRange.to),
      tags: activeTags.length > 0 ? activeTags.join(",") : undefined,
      sort_by: sortBy !== "updated_at" ? sortBy : undefined,
      sort_order: sortOrder !== "desc" ? sortOrder : undefined,
      group_by: groupBy !== "none" ? groupBy : undefined,
      page: page > 1 ? String(page) : undefined,
      page_size: pageSize !== 30 ? String(pageSize) : undefined,
    });
    const currentLocation =
      typeof window !== "undefined" ? `${window.location.pathname}${window.location.search}` : location;
    if (nextLocation !== currentLocation) {
      setLocation(nextLocation, { replace: true });
    }
  }, [search, searchMode, searchLogic, severity, statusFilter, sourceFilter, assignedFilter, assigneeFilter, linkedFilter, dateRange.from, dateRange.to, activeTags, sortBy, sortOrder, groupBy, page, pageSize, location, setLocation]);

  const allFilteredSelected = filteredAlerts.length > 0 && filteredAlerts.every((item: any) => selectedAlertIds.includes(item.id));
  const selectedCount = selectedAlertIds.length;

  const addTag = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && tagInput.trim()) {
      const trimmed = tagInput.trim();
      if (!activeTags.includes(trimmed)) {
        setActiveTags((prev) => [...prev, trimmed]);
      }
      setTagInput("");
    }
  };

  const toggleAlertSelection = (alertId: string) => {
    setSelectedAlertIds((prev) => (prev.includes(alertId) ? prev.filter((id) => id !== alertId) : [...prev, alertId]));
  };

  const toggleSelectAllFiltered = (checked: boolean) => {
    if (!checked) {
      setSelectedAlertIds((prev) => prev.filter((id) => !filteredAlerts.some((item: any) => item.id === id)));
      return;
    }
    setSelectedAlertIds((prev) => {
      const merged = new Set(prev);
      filteredAlerts.forEach((item: any) => merged.add(item.id));
      return Array.from(merged);
    });
  };

  const handleAssign = (id: string) => {
    const actorId = String(currentUser?.id || currentUserId || "").trim();
    if (!actorId) return;
    updateAlert.mutate(
      { id, data: { owner: actorId, status: "Triaged" } },
      { onSuccess: () => toast.success(t("alerts.toast.assigned")) },
    );
  };

  const mutateAlertAsync = (id: string, data: Record<string, any>) =>
    new Promise<void>((resolve, reject) => {
      updateAlert.mutate(
        { id, data },
        {
          onSuccess: () => resolve(),
          onError: (error: any) => reject(error),
        },
      );
    });

  const applyBulkAlertUpdate = async (
    buildPayload: (item: any) => Record<string, any>,
    successKey: string,
    partialKey: string,
  ) => {
    if (selectedAlertIds.length === 0) {
      toast.error(t("alerts.bulk.selectAlertsRequired"));
      return;
    }
    if (bulkAlertPending) {
      return;
    }
    const ids = [...selectedAlertIds];
    setBulkAlertPending(true);
    try {
      const settled = await Promise.allSettled(
        ids.map((id) => mutateAlertAsync(id, buildPayload(alertsByID.get(id) || {}))),
      );
      const failedIDs: string[] = [];
      settled.forEach((result, idx) => {
        if (result.status === "rejected") {
          failedIDs.push(ids[idx]);
        }
      });
      const failed = failedIDs.length;
      const updated = settled.length - failed;
      setSelectedAlertIds(failedIDs);
      if (failed > 0) {
        toast.error(t(partialKey, { updated: updated.toString(), failed: failed.toString() }));
        return;
      }
      toast.success(t(successKey, { count: updated.toString() }));
    } finally {
      setBulkAlertPending(false);
    }
  };

  const handleBulkAlertStatusApply = () => {
    const statusCode = String(bulkAlertStatus || "").trim();
    if (!statusCode) {
      toast.error(t("alerts.bulk.selectStatusRequired"));
      return;
    }
    void applyBulkAlertUpdate(
      () => ({ status: statusCode }),
      "alerts.bulk.statusSuccess",
      "alerts.bulk.statusPartial",
    );
  };

  const handleBulkAlertClose = () => {
    void applyBulkAlertUpdate(
      () => ({ status: "closed" }),
      "alerts.bulk.closeSuccess",
      "alerts.bulk.closePartial",
    );
  };

  const handleBulkAlertTagAdd = () => {
    const nextTag = String(bulkAlertTag || "").trim();
    if (!nextTag) {
      toast.error(t("alerts.bulk.tagRequired"));
      return;
    }
    void applyBulkAlertUpdate(
      (item) => {
        const existing = Array.isArray(item?.tags) ? item.tags : [];
        const normalized = existing
          .map((tag: any) => String(tag || "").trim())
          .filter(Boolean);
        return { tags: Array.from(new Set([...normalized, nextTag])) };
      },
      "alerts.bulk.tagSuccess",
      "alerts.bulk.tagPartial",
    );
    setBulkAlertTag("");
  };

  const handleCreateAlert = () => {
    const title = newAlert.title.trim();
    if (!title) {
      toast.error("Alert title is required");
      return;
    }
    createAlert.mutate(
      {
        title,
        description: newAlert.description.trim(),
        source: newAlert.source.trim() || "manual",
        severity: newAlert.severity,
        status: newAlert.status || "new",
        tlp: newAlert.tlp || "amber",
        pap: newAlert.pap || "amber",
      },
      {
        onSuccess: (created: any) => {
          toast.success("Alert created");
          setCreateAlertDialogOpen(false);
          setNewAlert({
            title: "",
            description: "",
            source: "manual",
            severity: "High",
            status: "new",
            tlp: "amber",
            pap: "amber",
          });
          if (created?.id) {
            setLocation(withTenantPath(currentTenantSlug, `/alerts/${created.id}`));
          }
        },
      },
    );
  };

  const handleBindSelectedToCase = () => {
    if (!targetCaseId) {
      toast.error(t("alerts.bulk.selectCaseRequired"));
      return;
    }
    if (selectedAlertIds.length === 0) {
      toast.error(t("alerts.bulk.selectAlertsRequired"));
      return;
    }
    bindAlertsToCase.mutate(
      { alertIds: selectedAlertIds, caseId: targetCaseId },
      {
        onSuccess: () => {
          toast.success(t("alerts.bulk.bindSuccess"));
          setSelectedAlertIds([]);
          setTargetCaseId("");
        },
      },
    );
  };

  const handleCreateCaseFromSelected = () => {
    if (selectedAlertIds.length === 0) {
      toast.error(t("alerts.bulk.selectAlertsRequired"));
      return;
    }
    createCaseFromAlerts.mutate(
      {
        alertIds: selectedAlertIds,
        case: {
          title: newCase.title.trim() || undefined,
          description: newCase.description.trim() || undefined,
          source: newCase.source.trim() || undefined,
          severity: newCase.severity,
          status: newCase.status || undefined,
          priority: newCase.priority || "medium",
        },
      },
      {
        onSuccess: (payload: any) => {
          toast.success(t("alerts.bulk.createCaseSuccess"));
          setCreateCaseDialogOpen(false);
          setSelectedAlertIds([]);
          setTargetCaseId("");
          if (payload?.case?.id) {
            setLocation(withTenantPath(currentTenantSlug, `/cases/${payload.case.id}`));
          }
        },
      },
    );
  };

  const openDeleteAlertConfirm = (alertId: string) => {
    setDeleteAlertIDs([alertId]);
    setDeleteAlertDialogOpen(true);
  };

  const openDeleteSelectedAlertsConfirm = () => {
    if (selectedAlertIds.length === 0) {
      toast.error(t("alerts.bulk.selectAlertsRequired"));
      return;
    }
    setDeleteAlertIDs([...selectedAlertIds]);
    setDeleteAlertDialogOpen(true);
  };

  const handleConfirmDeleteAlerts = () => {
    const idsToDelete = [...deleteAlertIDs];
    if (idsToDelete.length === 0) {
      setDeleteAlertDialogOpen(false);
      return;
    }
    setDeleteAlertDialogOpen(false);
    setDeleteAlertIDs([]);

    if (idsToDelete.length === 1) {
      const alertID = idsToDelete[0];
      deleteAlert.mutate(alertID, {
        onSuccess: () => {
          setSelectedAlertIds((prev) => prev.filter((item) => item !== alertID));
          toast.success(t("alerts.toast.deleted"));
        },
        onError: (error: any) => {
          toast.error(error?.message || t("alerts.toast.deleteFailed"));
        },
      });
      return;
    }

    deleteAlertsBulk.mutate(idsToDelete, {
      onSuccess: (result: any) => {
        const deleted = Number(result?.deleted ?? 0);
        const failed = Number(result?.failed ?? 0);
        setSelectedAlertIds([]);
        if (failed > 0) {
          toast.error(t("alerts.bulk.deletePartial", { deleted: deleted.toString(), failed: failed.toString() }));
          return;
        }
        toast.success(t("alerts.bulk.deleteSuccess", { count: deleted.toString() }));
      },
      onError: (error: any) => {
        toast.error(error?.message || t("alerts.toast.deleteFailed"));
      },
    });
  };

  const goToPage = (input: string) => {
    const parsed = Number.parseInt(String(input).trim(), 10);
    if (!Number.isFinite(parsed)) {
      setPageInput(String(page));
      return;
    }
    const nextPage = Math.min(totalPages, Math.max(1, parsed));
    setPage(nextPage);
    setPageInput(String(nextPage));
  };

  const renderPaginationControls = (position: "top" | "bottom") => (
    <Card
      className={`${ALERT_PANEL_CLASS} p-4`}
      data-testid={`alerts-pagination-${position}`}
    >
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
          <Select
            value={String(pageSize)}
            onValueChange={(value) => {
              const parsed = Number(value) || PAGE_SIZE_OPTIONS[1];
              setPageSize(parsed);
              setPage(1);
            }}
          >
            <SelectTrigger
              className={`${ALERT_ACTION_BUTTON_CLASS} w-full sm:w-40`}
              data-testid={position === "bottom" ? "select-alerts-page-size" : `select-alerts-page-size-${position}`}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
              {PAGE_SIZE_OPTIONS.map((size) => (
                <SelectItem key={size} value={String(size)}>
                  {t("pagination.perPage", { size: String(size) })}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <p className={ALERT_MUTED_TEXT_CLASS}>
            {totalAlerts > 0
              ? t("pagination.showing", {
                from: String((page - 1) * pageSize + 1),
                to: String(Math.min(page * pageSize, totalAlerts)),
                total: String(totalAlerts),
              })
              : t("pagination.noItems")}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="icon"
            onClick={() => setPage((prev) => Math.max(1, prev - 1))}
            disabled={page <= 1}
            className={`${ALERT_ACTION_BUTTON_CLASS} w-9 p-0`}
            data-testid={position === "bottom" ? "button-alerts-prev-page" : `button-alerts-prev-page-${position}`}
          >
            <ChevronLeft size={16} />
          </Button>
          <div
            className="inline-flex h-8 min-w-[92px] items-center justify-center rounded-md border border-[#2a2c3c] bg-[#0b0c10] px-2 text-xs font-medium text-[#d1d5db]"
            data-testid={position === "bottom" ? "alerts-page-indicator" : `alerts-page-indicator-${position}`}
          >
            <Input
              value={pageInput}
              onChange={(event) => setPageInput(event.target.value.replace(/[^\d]/g, ""))}
              onBlur={() => goToPage(pageInput)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  goToPage(pageInput);
                }
              }}
              className="h-6 w-9 border-none bg-transparent px-0 text-center text-xs text-[#d1d5db] focus-visible:ring-0"
              inputMode="numeric"
              aria-label={t("pagination.goToPage")}
              data-testid={position === "bottom" ? "input-alerts-page" : `input-alerts-page-${position}`}
            />
            <span className="mx-0.5 text-[#6b7280]">/</span>
            <span className="min-w-[22px] text-left">{totalPages}</span>
          </div>
          <Button
            variant="outline"
            size="icon"
            onClick={() => setPage((prev) => Math.min(totalPages, prev + 1))}
            disabled={page >= totalPages}
            className={`${ALERT_ACTION_BUTTON_CLASS} w-9 p-0`}
            data-testid={position === "bottom" ? "button-alerts-next-page" : `button-alerts-next-page-${position}`}
          >
            <ChevronRight size={16} />
          </Button>
        </div>
      </div>
    </Card>
  );

  const hasAlertsPayload = Boolean(alertsPageData);
  const isAlertsBootstrapLoading = Boolean(
    !hasAlertsPayload && (alertsPageLoading || tenantUsersLoading || casesLoading || caseStatusesLoading),
  );
  const showAlertsPageSkeleton = useMinimumLoading(isAlertsBootstrapLoading);

  if (showAlertsPageSkeleton) {
    return <AlertsLoadingSkeleton />;
  }

  return (
    <AppLayout>
      <div className={ALERT_PAGE_SHELL_CLASS}>
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h1 className="text-[32px] font-semibold leading-8 tracking-[-0.5px] text-white">{t("alerts.title")}</h1>
            <p className={`mt-1 ${ALERT_MUTED_TEXT_CLASS}`}>Monitor, triage and process incoming security alerts.</p>
          </div>
          <Button
            className="h-11 rounded-xl gap-2 border border-[#4adf37] bg-[#4adf37] px-5 font-semibold text-[#0b0c10] shadow-[0_6px_18px_rgba(74,223,55,0.35)] hover:bg-[#61f44f]"
            onClick={() => setCreateAlertDialogOpen(true)}
            data-testid="button-new-alert"
          >
            <Plus size={16} />
            New Alert
          </Button>
        </div>

        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          <Card className={`${ALERT_PANEL_CLASS} rounded-xl px-4 py-3`}>
            <div className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Critical</div>
            <div className="mt-1 text-[22px] font-semibold leading-6 text-[#eb5f65]">{severitySnapshot.critical}</div>
          </Card>
          <Card className={`${ALERT_PANEL_CLASS} rounded-xl px-4 py-3`}>
            <div className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">High</div>
            <div className="mt-1 text-[22px] font-semibold leading-6 text-[#ffc700]">{severitySnapshot.high}</div>
          </Card>
          <Card className={`${ALERT_PANEL_CLASS} rounded-xl px-4 py-3`}>
            <div className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Medium</div>
            <div className="mt-1 text-[22px] font-semibold leading-6 text-[#3b82f6]">{severitySnapshot.medium}</div>
          </Card>
          <Card className={`${ALERT_PANEL_CLASS} rounded-xl px-4 py-3`}>
            <div className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Low</div>
            <div className="mt-1 text-[22px] font-semibold leading-6 text-[#66ff4c]">{severitySnapshot.low}</div>
          </Card>
        </div>

        <Card className={`${ALERT_PANEL_CLASS} p-5`}>
          <CollapsibleFiltersPanel
            storageKey={ALERT_FILTERS_COLLAPSED_STORAGE_KEY}
            activeFiltersCount={activeFiltersCount}
            summaryItems={alertFilterSummaryItems}
            onReset={resetAlertFilters}
          >
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
            <div className="space-y-2">
              <label className={`text-xs font-medium ${ALERT_MUTED_TEXT_CLASS}`}>{t("alerts.filter.search")}</label>
              <div className="relative group">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-[#6b7280] group-focus-within:text-[#9ca3af] transition-colors" size={14} />
                <Input
                  placeholder={t("alerts.filter.searchPlaceholder")}
                  value={search}
                  onChange={(e) => {
                    setSearch(e.target.value);
                    setPage(1);
                  }}
                  className={`${ALERT_INPUT_CLASS} pl-9`}
                />
              </div>
            </div>

            <div className="space-y-2">
              <label className={`text-xs font-medium ${ALERT_MUTED_TEXT_CLASS}`}>{t("alerts.filter.severity")}</label>
              <Select value={severity} onValueChange={setSeverity}>
                <SelectTrigger className={ALERT_SELECT_TRIGGER_CLASS}>
                  <SelectValue placeholder={t("alerts.filter.selectSeverity")} />
                </SelectTrigger>
                <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                  <SelectItem value="all">{t("alerts.filter.allSeverities")}</SelectItem>
                  <SelectItem value="Critical">{t("severity.critical")}</SelectItem>
                  <SelectItem value="High">{t("severity.high")}</SelectItem>
                  <SelectItem value="Medium">{t("severity.medium")}</SelectItem>
                  <SelectItem value="Low">{t("severity.low")}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <label className={`text-xs font-medium ${ALERT_MUTED_TEXT_CLASS}`}>{t("alerts.filter.status")}</label>
              <Select value={statusFilter} onValueChange={setStatusFilter}>
                <SelectTrigger className={ALERT_SELECT_TRIGGER_CLASS}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                  <SelectItem value="all">{t("alerts.filter.allStatuses")}</SelectItem>
                  {statusFilterOptions.map((item) => (
                    <SelectItem key={item} value={item}>
                      {item}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <label className={`text-xs font-medium ${ALERT_MUTED_TEXT_CLASS}`}>{t("alerts.filter.source")}</label>
              <Select value={sourceFilter} onValueChange={setSourceFilter}>
                <SelectTrigger className={ALERT_SELECT_TRIGGER_CLASS}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                  <SelectItem value="all">{t("alerts.filter.allSources")}</SelectItem>
                  {sourceOptions.map((item) => (
                    <SelectItem key={item} value={item}>
                      {item}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <label className={`text-xs font-medium ${ALERT_MUTED_TEXT_CLASS}`}>Search Mode</label>
              <div className="flex h-[38px] items-center rounded-lg border border-[#2a2c3c] bg-[#0b0c10] p-1">
                {([
                  { value: "plain", label: "Plain" },
                  { value: "regex", label: "Regex" },
                  { value: "fulltext", label: "Fulltext" },
                ] as const).map((item) => (
                  <button
                    key={`search-mode-${item.value}`}
                    type="button"
                    onClick={() => {
                      setSearchMode(item.value);
                      setPage(1);
                    }}
                    className={`h-7 rounded-md px-2.5 text-xs transition-colors ${
                      searchMode === item.value ? "bg-[#1d1e29] text-white" : "text-[#6b7280] hover:text-[#d1d5db]"
                    }`}
                  >
                    {item.label}
                  </button>
                ))}
              </div>
            </div>

            <div className="space-y-2">
              <label className={`text-xs font-medium ${ALERT_MUTED_TEXT_CLASS}`}>Match Mode</label>
              <div className="flex h-[38px] items-center rounded-lg border border-[#2a2c3c] bg-[#0b0c10] p-1">
                {([
                  { value: "all", label: "All" },
                  { value: "any", label: "Any" },
                ] as const).map((item) => (
                  <button
                    key={`search-logic-${item.value}`}
                    type="button"
                    onClick={() => {
                      setSearchLogic(item.value);
                      setPage(1);
                    }}
                    className={`h-7 rounded-md px-3 text-xs transition-colors ${
                      searchLogic === item.value ? "bg-[#1d1e29] text-white" : "text-[#6b7280] hover:text-[#d1d5db]"
                    }`}
                  >
                    {item.label}
                  </button>
                ))}
              </div>
            </div>

            <div className="space-y-2">
              <label className={`text-xs font-medium ${ALERT_MUTED_TEXT_CLASS}`}>{t("alerts.filter.owner")}</label>
              <Select
                value={assignedFilter}
                onValueChange={(value) => {
                  const nextValue = value as AssignedFilterMode;
                  setAssignedFilter(nextValue);
                  if ((nextValue === "mine" || nextValue === "unassigned") && assigneeFilter !== "all") {
                    setAssigneeFilter("all");
                  }
                  setPage(1);
                }}
              >
                <SelectTrigger className={ALERT_SELECT_TRIGGER_CLASS}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                  <SelectItem value="all">{t("alerts.filter.ownerAll")}</SelectItem>
                  <SelectItem value="assigned">{t("alerts.filter.ownerAssigned")}</SelectItem>
                  <SelectItem value="unassigned">{t("alerts.filter.ownerUnassigned")}</SelectItem>
                  <SelectItem value="mine">{t("alerts.filter.ownerMine")}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <label className={`text-xs font-medium ${ALERT_MUTED_TEXT_CLASS}`}>{t("alerts.filter.assignee")}</label>
              <Select
                value={assigneeFilter}
                onValueChange={(value) => {
                  setAssigneeFilter(value);
                  if (value !== "all" && (assignedFilter === "mine" || assignedFilter === "unassigned")) {
                    setAssignedFilter("all");
                  }
                  setPage(1);
                }}
              >
                <SelectTrigger className={ALERT_SELECT_TRIGGER_CLASS} data-testid="select-alert-assignee-filter">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                  <SelectItem value="all">{t("alerts.filter.assigneeAll")}</SelectItem>
                  {tenantUsers.map((user: any) => (
                    <SelectItem key={user.id} value={String(user.id)}>
                      {String(user.name || user.email || user.id)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <label className={`text-xs font-medium ${ALERT_MUTED_TEXT_CLASS}`}>{t("alerts.filter.caseLink")}</label>
              <Select value={linkedFilter} onValueChange={setLinkedFilter}>
                <SelectTrigger className={ALERT_SELECT_TRIGGER_CLASS}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                  <SelectItem value="all">{t("alerts.filter.caseAll")}</SelectItem>
                  <SelectItem value="linked">{t("alerts.filter.caseLinked")}</SelectItem>
                  <SelectItem value="unlinked">{t("alerts.filter.caseUnlinked")}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <label className={`text-xs font-medium ${ALERT_MUTED_TEXT_CLASS}`}>{t("alerts.filter.dateRange")}</label>
              <Popover>
                <PopoverTrigger asChild>
                  <Button variant="outline" className="h-[38px] w-full justify-start rounded-lg border border-[#2a2c3c] bg-[#0b0c10] px-3 text-xs font-medium tracking-[-0.3px] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]">
                    <Calendar className="mr-2 h-4 w-4 text-[#9ca3af]" />
                    {dateRange.from ? (
                      dateRange.to ? (
                        <>
                          {format(dateRange.from, "LLL dd")} - {format(dateRange.to, "LLL dd")}
                        </>
                      ) : (
                        format(dateRange.from, "LLL dd")
                      )
                    ) : (
                      <span>{t("alerts.filter.pickRange")}</span>
                    )}
                  </Button>
                </PopoverTrigger>
                <PopoverContent className={`w-auto ${ALERT_PANEL_CLASS} border-[#2a2c3c] p-0`} align="end">
                  <CalendarComponent
                    initialFocus
                    mode="range"
                    defaultMonth={dateRange.from}
                    selected={{ from: dateRange.from, to: dateRange.to }}
                    onSelect={(range: any) => setDateRange({ from: range?.from, to: range?.to })}
                    numberOfMonths={2}
                  />
                  <div className="p-3 border-t flex justify-end">
                    <Button variant="ghost" size="sm" onClick={() => setDateRange({ from: undefined, to: undefined })}>
                      {t("alerts.filter.reset")}
                    </Button>
                  </div>
                </PopoverContent>
              </Popover>
            </div>

            <div className="space-y-2">
              <label className={`text-xs font-medium ${ALERT_MUTED_TEXT_CLASS}`}>{t("alerts.filter.tags")}</label>
              <div className="relative group">
                <Tag className="absolute left-3 top-1/2 -translate-y-1/2 text-[#6b7280] group-focus-within:text-[#9ca3af] transition-colors" size={14} />
                <Input
                  placeholder={t("alerts.filter.tagPlaceholder")}
                  value={tagInput}
                  onChange={(e) => setTagInput(e.target.value)}
                  onKeyDown={addTag}
                  className={`${ALERT_INPUT_CLASS} pl-9`}
                />
              </div>
            </div>

            <div className="space-y-2">
              <label className={`text-xs font-medium ${ALERT_MUTED_TEXT_CLASS}`}>Sort By</label>
              <Select
                value={sortBy}
                onValueChange={(value) => {
                  setSortBy(value);
                  setPage(1);
                }}
              >
                <SelectTrigger className={ALERT_SELECT_TRIGGER_CLASS}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                  {ALERT_SORT_FIELD_OPTIONS.map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <label className={`text-xs font-medium ${ALERT_MUTED_TEXT_CLASS}`}>Order</label>
              <Select
                value={sortOrder}
                onValueChange={(value) => {
                  setSortOrder(value as "asc" | "desc");
                  setPage(1);
                }}
              >
                <SelectTrigger className={ALERT_SELECT_TRIGGER_CLASS}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                  {ALERT_SORT_ORDER_OPTIONS.map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <label className={`text-xs font-medium ${ALERT_MUTED_TEXT_CLASS}`}>Group By</label>
              <Select
                value={groupBy}
                onValueChange={(value) => {
                  setGroupBy(value as AlertGroupBy);
                  setPage(1);
                }}
              >
                <SelectTrigger className={ALERT_SELECT_TRIGGER_CLASS}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                  {ALERT_GROUP_OPTIONS.map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          {activeTags.length > 0 && (
            <div className="mt-3 flex flex-wrap gap-2">
              {activeTags.map((tag) => (
                <Badge key={tag} className="flex items-center gap-1.5 rounded-md border-none bg-[#1d2030] px-2 py-1 text-[11px] text-[#9cc8ff]">
                  {tag}
                  <X size={12} className="cursor-pointer hover:text-red-500" onClick={() => setActiveTags((prev) => prev.filter((item) => item !== tag))} />
                </Badge>
              ))}
              <Button variant="ghost" size="sm" className="h-6 text-[11px] font-medium text-[#8b91a3] hover:bg-[#171b2a] hover:text-[#d1d5db]" onClick={() => setActiveTags([])}>
                {t("alerts.filter.clearAll")}
              </Button>
            </div>
          )}
          </CollapsibleFiltersPanel>
        </Card>

        <Card className={`${ALERT_PANEL_CLASS} p-4`}>
          <div className="flex flex-col xl:flex-row xl:items-center gap-3">
            <div className="flex items-center gap-3">
              <Checkbox checked={allFilteredSelected} onCheckedChange={(checked: boolean) => toggleSelectAllFiltered(Boolean(checked))} data-testid="checkbox-select-all-alerts" />
              <span className="text-sm font-semibold text-white">{t("alerts.bulk.selected", { count: selectedCount.toString() })}</span>
              <Badge variant="outline" className="border-[#2a2c3c] bg-[#0f121b] text-xs text-[#9ca3af]">
                <Layers size={12} className="mr-1" />
                {t("alerts.bulk.filtered", { count: filteredAlerts.length.toString() })}
              </Badge>
            </div>
            <div className="flex-1" />
            <div className="flex flex-col md:flex-row items-stretch md:items-center gap-2">
              <Select value={targetCaseId} onValueChange={setTargetCaseId}>
                <SelectTrigger className="h-9 w-full rounded-lg border border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] md:w-64" data-testid="select-target-case">
                  <SelectValue placeholder={t("alerts.bulk.selectCase")} />
                </SelectTrigger>
                <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                  {cases.map((item: any) => (
                    <SelectItem key={item.id} value={item.id}>
                      {(item.caseNumber || item.id).toString()} · {item.title}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button
                className="rounded-xl gap-2 border border-[#4adf37] bg-[#4adf37] text-[#0b0c10] hover:bg-[#61f44f]"
                onClick={handleBindSelectedToCase}
                disabled={selectedCount === 0 || bindAlertsToCase.isPending || bulkAlertPending}
                data-testid="button-bind-alerts-to-case"
              >
                <Link2 size={14} />
                {bindAlertsToCase.isPending ? t("alerts.bulk.binding") : t("alerts.bulk.bindToCase")}
              </Button>
              <Button
                variant="outline"
                className="rounded-xl gap-2 border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]"
                onClick={() => setCreateCaseDialogOpen(true)}
                disabled={selectedCount === 0 || deleteAlertsBulk.isPending || bulkAlertPending}
                data-testid="button-create-case-from-alerts"
              >
                <Plus size={14} />
                {t("alerts.bulk.createCase")}
              </Button>
              <Button
                variant="destructive"
                className="rounded-xl gap-2"
                onClick={openDeleteSelectedAlertsConfirm}
                disabled={selectedCount === 0 || deleteAlertsBulk.isPending || bulkAlertPending}
                data-testid="button-delete-selected-alerts"
              >
                <Trash2 size={14} />
                {deleteAlertsBulk.isPending ? t("alerts.bulk.deleting") : t("alerts.bulk.deleteSelected")}
              </Button>
            </div>
          </div>
          <div className="mt-3 grid gap-2 md:grid-cols-[1fr,1fr,auto,auto]">
            <Select value={bulkAlertStatus} onValueChange={setBulkAlertStatus}>
              <SelectTrigger className="w-full rounded-xl border border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db]" data-testid="select-alerts-bulk-status">
                <SelectValue placeholder={t("alerts.bulk.selectStatus")} />
              </SelectTrigger>
              <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                {alertStatusOptions.map((item) => (
                  <SelectItem key={`bulk-alert-status-${item.code}`} value={item.code}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Input
              value={bulkAlertTag}
              onChange={(event) => setBulkAlertTag(event.target.value)}
              placeholder={t("alerts.bulk.tagPlaceholder")}
              className={ALERT_INPUT_CLASS}
              data-testid="input-alerts-bulk-tag"
            />
            <Button
              variant="outline"
              className="rounded-xl border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]"
              onClick={handleBulkAlertStatusApply}
              disabled={selectedCount === 0 || bulkAlertPending}
              data-testid="button-alerts-bulk-apply-status"
            >
              {bulkAlertPending ? t("alerts.bulk.updating") : t("alerts.bulk.applyStatus")}
            </Button>
            <div className="flex gap-2">
              <Button
                variant="outline"
                className="rounded-xl border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]"
                onClick={handleBulkAlertTagAdd}
                disabled={selectedCount === 0 || bulkAlertPending}
                data-testid="button-alerts-bulk-add-tag"
              >
                <Tag size={14} className="mr-1" />
                {bulkAlertPending ? t("alerts.bulk.updating") : t("alerts.bulk.addTag")}
              </Button>
              <Button
                variant="outline"
                className="rounded-xl border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]"
                onClick={handleBulkAlertClose}
                disabled={selectedCount === 0 || bulkAlertPending}
                data-testid="button-alerts-bulk-close"
              >
                {bulkAlertPending ? t("alerts.bulk.updating") : t("alerts.bulk.closeSelected")}
              </Button>
            </div>
          </div>
        </Card>

        {renderPaginationControls("top")}

        <div className={`${ALERT_SUBPANEL_CLASS} px-4 py-2.5`}>
          <p className={ALERT_MUTED_TEXT_CLASS}>
            Showing {filteredAlerts.length} of {totalAlerts} alerts
          </p>
        </div>

        <div className="grid gap-6">
          {groupedAlerts.map((group) => (
            <section key={group.key} className="space-y-3">
              {groupBy !== "none" ? (
                <div className={`${ALERT_SUBPANEL_CLASS} flex items-center justify-between px-3 py-2`}>
                  <p className="text-sm font-semibold text-white">{group.label}</p>
                  <Badge variant="outline" className="border-[#2a2c3c] text-[10px] font-semibold uppercase text-[#9ca3af]">
                    {group.items.length}
                  </Badge>
                </div>
              ) : null}
              <div className="grid gap-3">
                {group.items.map((alert: any) => {
                  const ownerUser = alert.owner ? usersByID.get(String(alert.owner)) : null;
                  const ownerLabel = ownerUser?.name || ownerUser?.email || "Unknown user";
                  return (
                    <div key={alert.id}>
                      <Card
                        className={`${ALERT_PANEL_CLASS} group relative min-h-[182px] cursor-pointer rounded-xl p-5 transition-colors hover:border-[#4b5168] hover:bg-[linear-gradient(180deg,rgba(23,27,42,0.98),rgba(20,24,36,0.98))]`}
                        onClick={() => setLocation(withTenantPath(currentTenantSlug, `/alerts/${alert.id}`))}
                      >
                        <Button
                          variant="destructive"
                          size="icon"
                          className="absolute -right-2 -top-2 z-20 h-7 w-7 rounded-full opacity-0 shadow-sm transition-opacity group-hover:opacity-100 group-focus-within:opacity-100"
                          onClick={(event) => {
                            event.preventDefault();
                            event.stopPropagation();
                            openDeleteAlertConfirm(alert.id);
                          }}
                          disabled={deleteAlert.isPending || deleteAlertsBulk.isPending}
                          data-testid={`button-delete-alert-${alert.id}`}
                        >
                          <Trash2 size={12} />
                        </Button>
                        <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
                          <div className="flex min-w-0 flex-1 items-start gap-3">
                            <Checkbox
                              checked={selectedAlertIds.includes(alert.id)}
                              onCheckedChange={() => toggleAlertSelection(alert.id)}
                              onClick={(event) => event.stopPropagation()}
                              data-testid={`checkbox-alert-${alert.id}`}
                              className="mt-1.5"
                            />
                            <div className="mt-[2px] flex h-11 w-11 shrink-0 items-center justify-center rounded-lg border border-[#2a2c3c] bg-[#0f121b]">
                              <span
                                className={`block h-3 w-3 rounded-full ${alert.sev === "Critical" ? "bg-[#eb5f65]/75" : alert.sev === "High" ? "bg-[#ffc700]" : alert.sev === "Medium" ? "bg-[#3b82f6]" : "bg-[#66ff4c]"}`}
                              />
                            </div>
                            <div className="min-w-0 flex-1">
                              <div className="mb-2 flex flex-wrap items-center gap-2">
                                <Badge variant="outline" className="h-5 border-[#2a2c3c] bg-[#0f121b] text-[10px] font-semibold uppercase text-[#9ca3af]">
                                  <EllipsisText text={alert.source} className="max-w-[120px]" />
                                </Badge>
                                <Badge variant="outline" className="h-5 border-[#2a2c3c] bg-[#101827] text-[10px] font-semibold text-[#8fb6ff]">
                                  {String(alert.status || "new")}
                                </Badge>
                                {alert.owner ? (
                                  <button
                                    type="button"
                                    className="inline-flex items-center gap-1"
                                    onClick={(event) => {
                                      event.preventDefault();
                                      event.stopPropagation();
                                      setLocation(withTenantPath(currentTenantSlug, `/users/${alert.owner}`));
                                    }}
                                    data-testid={`button-alert-owner-${alert.id}`}
                                  >
                                    <Badge variant="outline" className="h-4 gap-1 border-[#2a2c3c] bg-[#121a2c] px-1.5 text-[9px] font-medium text-[#8fb6ff]">
                                      <UserAvatar
                                        name={ownerLabel}
                                        avatar={ownerUser?.avatar}
                                        className="h-3.5 w-3.5"
                                        fallbackClassName="bg-[#243459] text-[#d7e3ff] text-[8px] font-bold"
                                      />
                                      <EllipsisText text={ownerLabel} className="max-w-[160px]" />
                                    </Badge>
                                  </button>
                                ) : null}
                              </div>
                              <EllipsisText text={alert.title} className="text-[20px] font-semibold leading-6 tracking-[-0.02em] text-white" />
                              <p className="mt-2 line-clamp-2 text-sm leading-5 text-[#8b91a3]">
                                {String(alert.description || "").trim() || "No additional details provided."}
                              </p>
                              <div className="mt-3 flex flex-wrap items-center gap-3 text-xs font-medium text-[#8b91a3]">
                                <span className="flex items-center gap-1">
                                  <Clock size={12} /> {format(new Date(alert.time), "MMM dd HH:mm")}
                                </span>
                                <span>•</span>
                                <span className="flex items-center gap-1">
                                  <CheckCircle2 size={12} className="text-[#66ff4c]" /> {t("alerts.confidenceHigh")}
                                </span>
                                {alert.caseId ? (
                                  <>
                                    <span>•</span>
                                    <span className="flex items-center gap-1 text-[#74d7a2]">
                                      <Link2 size={12} /> {t("alerts.caseLinkedShort")} {String(alert.caseId).slice(0, 8)}
                                    </span>
                                  </>
                                ) : null}
                              </div>
                              {alert.tags.length > 0 ? (
                                <div className="mt-3 flex flex-wrap items-center gap-2">
                                  {alert.tags.map((item: string) => (
                                    <Badge key={item} variant="secondary" className="h-5 rounded-md border-none bg-[#1d2030] px-2 text-[11px] font-medium text-[#9cc8ff]">
                                      {item}
                                    </Badge>
                                  ))}
                                </div>
                              ) : null}
                            </div>
                          </div>
                          <div className="flex shrink-0 items-center gap-3 pr-1 md:pt-1" onClick={(event) => event.stopPropagation()}>
                            {!alert.owner ? (
                              <Button
                                variant="secondary"
                                className="h-9 rounded-xl bg-[#4adf37] px-4 font-semibold text-[#0b0c10] hover:bg-[#61f44f]"
                                onClick={() => handleAssign(alert.id)}
                                data-testid={`button-assign-alert-${alert.id}`}
                              >
                                {t("alerts.assignToMe")}
                              </Button>
                            ) : null}
                            <Badge className={`${alert.sev === "Critical" ? "bg-[#eb5f65]" : alert.sev === "High" ? "bg-[#ffc700] text-[#0b0c10]" : alert.sev === "Medium" ? "bg-[#3b82f6]" : "bg-[#66ff4c] text-[#0b0c10]"} min-w-[94px] justify-center font-semibold`}>
                              {alert.sev}
                            </Badge>
                          </div>
                        </div>
                      </Card>
                    </div>
                  );
                })}
              </div>
            </section>
          ))}
          {filteredAlerts.length === 0 ? (
            <div className={`${ALERT_SUBPANEL_CLASS} border-dashed py-20 text-center`}>
              <div className="text-sm font-semibold text-[#8b91a3]">{t("alerts.emptyFiltered")}</div>
            </div>
          ) : null}
        </div>

        {renderPaginationControls("bottom")}
      </div>

      <AlertDialog
        open={deleteAlertDialogOpen}
        onOpenChange={(open) => {
          setDeleteAlertDialogOpen(open);
          if (!open) {
            setDeleteAlertIDs([]);
          }
        }}
      >
        <AlertDialogContent className={`${ALERT_PANEL_CLASS} rounded-xl border-[#2a2c3c] text-white`}>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("common.delete")}</AlertDialogTitle>
            <AlertDialogDescription className={ALERT_MUTED_TEXT_CLASS}>
              {deleteAlertIDs.length > 1
                ? t("alerts.bulk.deleteConfirm", { count: deleteAlertIDs.length.toString() })
                : t("alerts.deleteConfirmOne")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel className="border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]">{t("common.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              className="border border-[#6a2f39] bg-[#341b22] text-[#ff9fb3] hover:bg-[#44232c]"
              onClick={handleConfirmDeleteAlerts}
              disabled={deleteAlert.isPending || deleteAlertsBulk.isPending}
            >
              {t("common.delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <Dialog open={createAlertDialogOpen} onOpenChange={setCreateAlertDialogOpen}>
        <DialogContent className={`${ALERT_PANEL_CLASS} rounded-xl border-[#2a2c3c] text-white`}>
          <DialogHeader>
            <DialogTitle>Create Alert</DialogTitle>
            <DialogDescription className={ALERT_MUTED_TEXT_CLASS}>Manual alert creation for SOC intake.</DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-[#9ca3af]">{t("cases.field.title")}</label>
              <Input
                value={newAlert.title}
                onChange={(e) => setNewAlert((prev) => ({ ...prev, title: e.target.value }))}
                placeholder="Alert title"
                className={DARK_DIALOG_INPUT_CLASS}
                data-testid="input-new-alert-title"
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-[#9ca3af]">{t("cases.field.description")}</label>
              <Input
                value={newAlert.description}
                onChange={(e) => setNewAlert((prev) => ({ ...prev, description: e.target.value }))}
                placeholder="Alert description"
                className={DARK_DIALOG_INPUT_CLASS}
                data-testid="input-new-alert-description"
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-[#9ca3af]">{t("cases.field.severity")}</label>
                <Select value={newAlert.severity} onValueChange={(value) => setNewAlert((prev) => ({ ...prev, severity: value }))}>
                  <SelectTrigger className={DARK_DIALOG_TRIGGER_CLASS}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                    <SelectItem value="Critical">{t("severity.critical")}</SelectItem>
                    <SelectItem value="High">{t("severity.high")}</SelectItem>
                    <SelectItem value="Medium">{t("severity.medium")}</SelectItem>
                    <SelectItem value="Low">{t("severity.low")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-[#9ca3af]">{t("cases.field.status")}</label>
                <Select value={newAlert.status} onValueChange={(value) => setNewAlert((prev) => ({ ...prev, status: value }))}>
                  <SelectTrigger className={DARK_DIALOG_TRIGGER_CLASS}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                    <SelectItem value="new">New</SelectItem>
                    <SelectItem value="triaged">Triaged</SelectItem>
                    <SelectItem value="open">Open</SelectItem>
                    <SelectItem value="resolved">Resolved</SelectItem>
                    <SelectItem value="closed">Closed</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div className="grid grid-cols-3 gap-3">
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-[#9ca3af]">{t("cases.field.source")}</label>
                <Input className={DARK_DIALOG_INPUT_CLASS} value={newAlert.source} onChange={(e) => setNewAlert((prev) => ({ ...prev, source: e.target.value }))} />
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-[#9ca3af]">{t("cases.field.tlp")}</label>
                <Select value={newAlert.tlp} onValueChange={(value) => setNewAlert((prev) => ({ ...prev, tlp: value }))}>
                  <SelectTrigger className={DARK_DIALOG_TRIGGER_CLASS}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                    <SelectItem value="red">RED</SelectItem>
                    <SelectItem value="amber">AMBER</SelectItem>
                    <SelectItem value="green">GREEN</SelectItem>
                    <SelectItem value="clear">CLEAR</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-[#9ca3af]">{t("cases.field.pap")}</label>
                <Select value={newAlert.pap} onValueChange={(value) => setNewAlert((prev) => ({ ...prev, pap: value }))}>
                  <SelectTrigger className={DARK_DIALOG_TRIGGER_CLASS}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                    <SelectItem value="red">RED</SelectItem>
                    <SelectItem value="amber">AMBER</SelectItem>
                    <SelectItem value="green">GREEN</SelectItem>
                    <SelectItem value="clear">CLEAR</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" className="border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]" onClick={() => setCreateAlertDialogOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button className="border border-[#4adf37] bg-[#4adf37] text-[#0b0c10] hover:bg-[#61f44f]" onClick={handleCreateAlert} disabled={createAlert.isPending} data-testid="button-create-alert-confirm">
              {createAlert.isPending ? "Creating..." : "Create Alert"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={createCaseDialogOpen} onOpenChange={setCreateCaseDialogOpen}>
        <DialogContent className={`${ALERT_PANEL_CLASS} rounded-xl border-[#2a2c3c] text-white`}>
          <DialogHeader>
            <DialogTitle>{t("alerts.bulk.createCaseTitle")}</DialogTitle>
            <DialogDescription className={ALERT_MUTED_TEXT_CLASS}>{t("alerts.bulk.createCaseDescription")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-[#9ca3af]">{t("cases.field.title")}</label>
              <Input className={DARK_DIALOG_INPUT_CLASS} value={newCase.title} onChange={(e) => setNewCase((prev) => ({ ...prev, title: e.target.value }))} placeholder={t("alerts.bulk.caseTitlePlaceholder")} />
            </div>
            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-[#9ca3af]">{t("cases.field.description")}</label>
              <Input className={DARK_DIALOG_INPUT_CLASS} value={newCase.description} onChange={(e) => setNewCase((prev) => ({ ...prev, description: e.target.value }))} placeholder={t("alerts.bulk.caseDescriptionPlaceholder")} />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-[#9ca3af]">{t("cases.field.severity")}</label>
                <Select value={newCase.severity} onValueChange={(value) => setNewCase((prev) => ({ ...prev, severity: value }))}>
                  <SelectTrigger className={DARK_DIALOG_TRIGGER_CLASS}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                    <SelectItem value="Critical">{t("severity.critical")}</SelectItem>
                    <SelectItem value="High">{t("severity.high")}</SelectItem>
                    <SelectItem value="Medium">{t("severity.medium")}</SelectItem>
                    <SelectItem value="Low">{t("severity.low")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-[#9ca3af]">{t("cases.field.status")}</label>
                <Select value={newCase.status} onValueChange={(value) => setNewCase((prev) => ({ ...prev, status: value }))}>
                  <SelectTrigger className={DARK_DIALOG_TRIGGER_CLASS}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                    {statusOptions.map((item: any) => (
                      <SelectItem key={item.code} value={item.code}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-[#9ca3af]">{t("cases.field.source")}</label>
                <Input className={DARK_DIALOG_INPUT_CLASS} value={newCase.source} onChange={(e) => setNewCase((prev) => ({ ...prev, source: e.target.value }))} />
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-[#9ca3af]">{t("cases.field.priority")}</label>
                <Select value={newCase.priority} onValueChange={(value) => setNewCase((prev) => ({ ...prev, priority: value }))}>
                  <SelectTrigger className={DARK_DIALOG_TRIGGER_CLASS}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                    <SelectItem value="critical">{t("priority.p1")}</SelectItem>
                    <SelectItem value="high">{t("priority.p2")}</SelectItem>
                    <SelectItem value="medium">{t("priority.p3")}</SelectItem>
                    <SelectItem value="low">{t("priority.p4")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" className="border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]" onClick={() => setCreateCaseDialogOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button className="border border-[#4adf37] bg-[#4adf37] text-[#0b0c10] hover:bg-[#61f44f]" onClick={handleCreateCaseFromSelected} disabled={createCaseFromAlerts.isPending}>
              {createCaseFromAlerts.isPending ? t("cases.button.creating") : t("alerts.bulk.createCase")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </AppLayout >
  );
}
