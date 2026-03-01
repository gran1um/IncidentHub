import { AppLayout } from "@/components/layout";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Search, Plus, FolderKanban, Users, Clock, Calendar, Tag, X, MessageSquare, Layers, Trash2, ChevronLeft, ChevronRight, ListChecks, SlidersHorizontal } from "lucide-react";
import { Link, useLocation } from "wouter";
import { useState, useMemo, useEffect, useRef } from "react";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { format } from "date-fns";

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { toast } from "sonner";
import {
  type AssignedFilterMode,
  useAppState,
  useCaseStatuses,
  useCaseTemplates,
  useCases,
  useCasesPage,
  useCasesSummaries,
  useUser,
  useUsers,
  useUpdateCase,
  useCreateCase,
  useCreateCaseTask,
  useDeleteCase,
  useDeleteCasesBulk,
} from "@/lib/api";
import { Skeleton } from "@/components/ui/skeleton";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
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
import { Textarea } from "@/components/ui/textarea";
import { useT } from "@/lib/i18n";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { EllipsisText } from "@/components/ui/ellipsis-text";
import { applySearchPatch, formatDateParam, parseDateParam, parsePositiveIntParam, splitLocationPathAndSearch } from "@/lib/url-state";
import { UserAvatar } from "@/components/user-avatar";
import { withTenantPath } from "@/lib/tenant-url";
import { useMinimumLoading } from "@/lib/use-minimum-loading";
import { useNow } from "@/hooks/use-now";
import { isSOARCaseFieldKey, mergeMissingSOARCaseFields } from "@/lib/soar-case-fields";
import { CollapsibleFiltersPanel } from "@/components/collapsible-filters-panel";

const PAGE_SIZE_OPTIONS = [10, 30, 50, 100] as const;
const BASE_CASE_SORT_FIELD_OPTIONS = [
  { value: "updated_at", label: "Updated" },
  { value: "created_at", label: "Created" },
  { value: "severity", label: "Severity" },
  { value: "status", label: "Status" },
  { value: "title", label: "Title" },
  { value: "case_number", label: "Case Number" },
] as const;
const CASE_SORT_ORDER_OPTIONS = [
  { value: "desc", label: "Descending" },
  { value: "asc", label: "Ascending" },
] as const;
const CASE_GROUP_OPTIONS = [
  { value: "none", label: "No Grouping" },
  { value: "rule", label: "Rule" },
  { value: "status", label: "Status" },
  { value: "severity", label: "Severity" },
  { value: "owner", label: "Assignee" },
  { value: "work_time", label: "Time In Work" },
  { value: "source", label: "Source" },
  { value: "date", label: "Created Date" },
] as const;
const CASE_SORT_FIELD_SET = new Set(BASE_CASE_SORT_FIELD_OPTIONS.map((item) => item.value));
const CASE_SORT_ORDER_SET = new Set(CASE_SORT_ORDER_OPTIONS.map((item) => item.value));
const CASE_GROUP_SET = new Set(CASE_GROUP_OPTIONS.map((item) => item.value));
const DARK_SELECT_CONTENT_CLASS = "rounded-xl border border-[#2a2c3c] bg-[#13141c] text-white";
const DARK_INPUT_CLASS =
  "h-[38px] rounded-lg border border-[#2a2c3c] bg-[#0b0c10] text-sm text-[#d1d5db] placeholder:text-[#6b7280] focus-visible:ring-1 focus-visible:ring-[#3b4a79]";
const DARK_SELECT_TRIGGER_CLASS =
  "h-[38px] rounded-lg border border-[#2a2c3c] bg-[#0b0c10] text-sm text-[#d1d5db] focus:ring-1 focus:ring-[#3b4a79]";
const CASE_PAGE_SHELL_CLASS = "mx-auto w-full max-w-[1144px] space-y-6 pb-6";
const CASE_PANEL_CLASS =
  "rounded-2xl border border-[rgba(255,255,255,0.06)] bg-[linear-gradient(180deg,rgba(19,20,28,0.97),rgba(17,20,32,0.97))] shadow-[0_14px_34px_rgba(0,0,0,0.28)]";
const CASE_SUBPANEL_CLASS = "rounded-xl border border-[#2a2c3c] bg-[#111624]";
const CASE_MUTED_TEXT_CLASS = "text-xs text-[#8b91a3]";
const CASE_ACTION_BUTTON_CLASS =
  "h-11 gap-2 rounded-xl border border-[#2a2c3c] bg-[#0d0f17] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]";
const CASE_PAGE_SIZE_SELECT_CLASS =
  "h-9 w-[102px] rounded-lg border border-[#2a2c3c] bg-[#0b0c10] text-sm text-[#d1d5db] focus:ring-1 focus:ring-[#3b4a79]";

function CasesLoadingSkeleton() {
  return (
    <AppLayout>
      <div className={CASE_PAGE_SHELL_CLASS}>
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div className="space-y-2">
            <Skeleton className="h-8 w-40 rounded-md" />
            <Skeleton className="h-4 w-72 max-w-full rounded-md" />
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Skeleton className="h-12 w-12 rounded-xl" />
            <Skeleton className="h-12 w-[86px] rounded-xl" />
            <Skeleton className="h-5 w-12 rounded-md" />
            <Skeleton className="h-12 w-12 rounded-xl" />
            <Skeleton className="h-12 w-[152px] rounded-xl" />
            <Skeleton className="h-11 w-[168px] rounded-xl" />
          </div>
        </div>

        <Card className={`${CASE_PANEL_CLASS} p-5`}>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-12">
            <div className="space-y-2 md:col-span-6">
              <Skeleton className="h-4 w-20 rounded-md" />
              <Skeleton className="h-14 w-full rounded-xl" />
              <div className="flex flex-wrap items-center gap-2 pt-1">
                {Array.from({ length: 4 }).map((_, index) => (
                  <Skeleton key={`cases-loading-search-toggle-${index}`} className="h-10 w-20 rounded-lg" />
                ))}
              </div>
            </div>

            {Array.from({ length: 8 }).map((_, index) => (
              <div key={`cases-loading-filter-${index}`} className="space-y-2 md:col-span-3">
                <Skeleton className="h-4 w-20 rounded-md" />
                <Skeleton className="h-14 w-full rounded-xl" />
              </div>
            ))}

            <div className="space-y-2 md:col-span-3">
              <Skeleton className="h-4 w-16 rounded-md" />
              <Skeleton className="h-14 w-full rounded-xl" />
            </div>

            <div className="space-y-2 md:col-span-3">
              <Skeleton className="h-4 w-24 rounded-md" />
              <Skeleton className="h-14 w-full rounded-xl" />
            </div>
          </div>

          <div className="mt-5 flex items-center justify-between border-t border-[#2a2c3c] pt-4">
            <div className="flex items-center gap-2">
              <Skeleton className="h-4 w-4 rounded-sm" />
              <Skeleton className="h-4 w-44 rounded-md" />
            </div>
            <Skeleton className="h-4 w-32 rounded-md" />
          </div>
        </Card>

        <Card className={`${CASE_PANEL_CLASS} p-4`}>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
            <div className="flex items-center gap-3">
              <Skeleton className="h-4 w-4 rounded-sm" />
              <Skeleton className="h-5 w-36 rounded-md" />
              <Skeleton className="h-6 w-24 rounded-lg" />
            </div>
            <div className="flex-1" />
            <Skeleton className="h-10 w-44 rounded-xl" />
          </div>
          <div className="mt-3 grid gap-2 md:grid-cols-[1fr,1fr,auto,auto]">
            <Skeleton className="h-10 w-full rounded-xl" />
            <Skeleton className="h-10 w-full rounded-xl" />
            <Skeleton className="h-10 w-[168px] rounded-xl" />
            <div className="flex gap-2">
              <Skeleton className="h-10 w-[124px] rounded-xl" />
              <Skeleton className="h-10 w-[136px] rounded-xl" />
            </div>
          </div>
        </Card>

        <div className={`${CASE_SUBPANEL_CLASS} px-4 py-2.5`}>
          <Skeleton className="h-4 w-56 rounded-md" />
        </div>

        <div className="grid gap-4">
          {Array.from({ length: 3 }).map((_, index) => (
            <Card key={`cases-loading-card-${index}`} className={`${CASE_PANEL_CLASS} overflow-hidden rounded-xl p-5`}>
              <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
                <div className="flex min-w-0 flex-1 items-start gap-4">
                  <Skeleton className="mt-2 h-4 w-4 rounded-sm" />
                  <Skeleton className="h-11 w-11 rounded-lg" />
                  <div className="min-w-0 flex-1 space-y-3">
                    <div className="flex flex-wrap items-center gap-2">
                      <Skeleton className="h-4 w-28 rounded-md" />
                      <Skeleton className="h-5 w-16 rounded-md" />
                      <Skeleton className="h-4 w-12 rounded-md" />
                      <Skeleton className="h-4 w-12 rounded-md" />
                    </div>
                    <Skeleton className="h-8 w-3/4 rounded-md" />
                    <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
                      <Skeleton className="h-4 w-40 rounded-md" />
                      <Skeleton className="h-4 w-36 rounded-md" />
                      <Skeleton className="h-4 w-32 rounded-md" />
                    </div>
                    <div className="space-y-2">
                      <Skeleton className="h-2 w-full rounded-full" />
                      <Skeleton className="h-2 w-4/5 rounded-full" />
                    </div>
                  </div>
                </div>
                <div className="grid w-full grid-cols-2 gap-2 md:w-[228px]">
                  {Array.from({ length: 4 }).map((__, metaIndex) => (
                    <div
                      key={`cases-loading-card-${index}-meta-${metaIndex}`}
                      className="rounded-lg border border-[#2a2c3c] bg-[#0b0c10] p-2.5"
                    >
                      <Skeleton className="h-3 w-16 rounded-md" />
                      <Skeleton className="mt-2 h-4 w-12 rounded-md" />
                    </div>
                  ))}
                </div>
              </div>
            </Card>
          ))}
        </div>

        <Card className={`${CASE_PANEL_CLASS} p-4`}>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex items-center gap-2">
              <Skeleton className="h-9 w-32 rounded-lg" />
              <Skeleton className="h-4 w-44 rounded-md" />
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

function formatTimeDeltaCompact(deltaMs: number): string {
  const totalMinutes = Math.max(0, Math.round(Math.abs(deltaMs) / 60000));
  if (totalMinutes <= 0) return "<1m";
  if (totalMinutes < 60) return `${totalMinutes}m`;

  const totalHours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  if (totalHours < 24) {
    return minutes > 0 ? `${totalHours}h ${minutes}m` : `${totalHours}h`;
  }

  const days = Math.floor(totalHours / 24);
  const hours = totalHours % 24;
  return hours > 0 ? `${days}d ${hours}h` : `${days}d`;
}

function taskDueBadgeClass(deltaMs: number): string {
  if (deltaMs < 0) {
    return "border-[#6a2f39] bg-[#341b22] text-[#ff9fb3]";
  }
  if (deltaMs <= 4 * 60 * 60 * 1000) {
    return "border-[rgba(245,158,11,0.35)] bg-[rgba(245,158,11,0.14)] text-[#fcd34d]";
  }
  return "border-[rgba(34,197,94,0.28)] bg-[rgba(34,197,94,0.14)] text-[#86efac]";
}

type CaseGroupBy = (typeof CASE_GROUP_OPTIONS)[number]["value"];
type CaseGroupSection = {
  key: string;
  label: string;
  items: any[];
};

type CaseCustomFieldRow = {
  id: string;
  key: string;
  value: string;
};

type SearchMode = "plain" | "regex" | "fulltext";
type SearchLogic = "all" | "any";

const TAG_COLOR_STORAGE_KEY = "incidenthub-case-tag-colors";
const CASE_VISIBLE_FIELDS_STORAGE_KEY = "incidenthub-cases-visible-fields";
const CASE_FILTERS_COLLAPSED_STORAGE_KEY = "incidenthub-cases-filters-collapsed";
const DEFAULT_CASE_VISIBLE_FIELDS = [
  "id",
  "status",
  "tags",
  "assignee",
  "created",
  "work_time",
  "custom_field_values_count",
  "rule",
  "task_progress",
  "responder_statuses",
  "discussion",
];

type CaseVisibleFieldOption = {
  key: string;
  label: string;
};

function normalizeCaseText(value: unknown): string {
  return String(value || "").trim().toLowerCase();
}

function formatCaseDateInputValue(value: Date | undefined): string {
  if (!value) return "";
  const year = value.getFullYear();
  const month = String(value.getMonth() + 1).padStart(2, "0");
  const day = String(value.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function parseCaseDateInputValue(value: string): Date | undefined {
  const normalized = String(value || "").trim();
  if (!normalized) return undefined;
  const next = new Date(`${normalized}T00:00:00`);
  if (!Number.isFinite(next.getTime())) return undefined;
  return next;
}

function tokenizeCaseSearch(value: string): string[] {
  return value
    .split(/\s+/g)
    .map((token) => token.trim())
    .filter(Boolean);
}

function caseAssigneeID(item: any): string {
  return String(item?.assignee || item?.owner || "").trim();
}

function caseInWorkHours(item: any): number {
  const startedAt = new Date(String(item?.time || item?.createdAt || item?.updatedAt || "")).getTime();
  if (!Number.isFinite(startedAt) || startedAt <= 0) {
    return 0;
  }
  const closedAtRaw = String(item?.closedAt || "").trim();
  const endedAt = closedAtRaw ? new Date(closedAtRaw).getTime() : Date.now();
  if (!Number.isFinite(endedAt) || endedAt <= startedAt) {
    return 0;
  }
  return Math.max(0, (endedAt - startedAt) / 3_600_000);
}

function hasCaseObservables(item: any): boolean {
  const customFields = item?.customFields && typeof item.customFields === "object" ? item.customFields : {};
  const indicators = String(customFields?.indicators || customFields?.observable || customFields?.observables || "").trim();
  if (indicators) {
    return true;
  }
  const countRaw = Number(customFields?.observables_count ?? customFields?.observable_count ?? 0);
  return Number.isFinite(countRaw) && countRaw > 0;
}

function normalizeCaseCustomSortBy(value: string): string {
  const normalized = normalizeCaseText(value);
  if (!normalized.startsWith("cf:")) {
    return "";
  }
  const key = normalized.slice(3).trim();
  if (!/^[a-z0-9_][a-z0-9_:-]{0,63}$/.test(key)) {
    return "";
  }
  return `cf:${key}`;
}

function caseCustomFieldValueCount(item: any): number {
  const customFields = item?.customFields && typeof item.customFields === "object" ? item.customFields : {};
  return Object.values(customFields).filter((value) => String(value ?? "").trim() !== "").length;
}

function isClosedCaseItem(item: any, closedStatusCodes: Set<string>): boolean {
  const statusCode = normalizeCaseText(item?.statusCode || item?.status);
  if (!statusCode) {
    return false;
  }
  if (closedStatusCodes.has(statusCode)) {
    return true;
  }
  return statusCode.includes("closed") || statusCode.includes("resolved");
}

function normalizeSeverityWeight(value: unknown): number {
  switch (normalizeCaseText(value)) {
    case "critical":
      return 4;
    case "high":
      return 3;
    case "medium":
      return 2;
    case "low":
      return 1;
    default:
      return 0;
  }
}

function sortValueByField(item: any, sortBy: string, usersByID: Map<string, any>): string | number {
  const customSort = normalizeCaseCustomSortBy(sortBy);
  if (customSort) {
    const fieldKey = customSort.slice(3);
    return normalizeCaseText(item?.customFields?.[fieldKey]);
  }
  switch (sortBy) {
    case "created_at":
      return new Date(String(item?.createdAt || item?.time || "")).getTime() || 0;
    case "updated_at":
      return new Date(String(item?.updatedAt || item?.time || "")).getTime() || 0;
    case "severity":
      return normalizeSeverityWeight(item?.sev);
    case "status":
      return normalizeCaseText(item?.statusCode || item?.status);
    case "title":
      return normalizeCaseText(item?.title);
    case "case_number":
      return normalizeCaseText(item?.caseNumber || item?.id);
    case "owner": {
      const assigneeID = caseAssigneeID(item);
      if (!assigneeID) return "";
      return normalizeCaseText(usersByID.get(assigneeID)?.name || usersByID.get(assigneeID)?.email || assigneeID);
    }
    default:
      return new Date(String(item?.updatedAt || item?.time || "")).getTime() || 0;
  }
}

function loadTagColors(): Record<string, string> {
  if (typeof window === "undefined") return {};
  try {
    const raw = window.localStorage.getItem(TAG_COLOR_STORAGE_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw);
    if (!parsed || typeof parsed !== "object") return {};
    const out: Record<string, string> = {};
    Object.entries(parsed as Record<string, unknown>).forEach(([key, value]) => {
      const normalizedKey = String(key || "").trim().toLowerCase();
      const normalizedValue = String(value || "").trim();
      if (!normalizedKey || !/^#?[0-9a-fA-F]{6}$/.test(normalizedValue)) return;
      out[normalizedKey] = normalizedValue.startsWith("#") ? normalizedValue : `#${normalizedValue}`;
    });
    return out;
  } catch {
    return {};
  }
}

function saveTagColors(value: Record<string, string>) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(TAG_COLOR_STORAGE_KEY, JSON.stringify(value));
}

function formatTagColorValue(color: string): string {
  const normalized = String(color || "").trim();
  if (!/^#?[0-9a-fA-F]{6}$/.test(normalized)) return "#4f46e5";
  return normalized.startsWith("#") ? normalized : `#${normalized}`;
}

function loadCaseVisibleFields(): string[] {
  if (typeof window === "undefined") return [...DEFAULT_CASE_VISIBLE_FIELDS];
  try {
    const raw = window.localStorage.getItem(CASE_VISIBLE_FIELDS_STORAGE_KEY);
    if (!raw) return [...DEFAULT_CASE_VISIBLE_FIELDS];
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [...DEFAULT_CASE_VISIBLE_FIELDS];
    const out = parsed
      .map((item) => String(item || "").trim())
      .filter(Boolean);
    return out.length > 0 ? Array.from(new Set(out)) : [...DEFAULT_CASE_VISIBLE_FIELDS];
  } catch {
    return [...DEFAULT_CASE_VISIBLE_FIELDS];
  }
}

function saveCaseVisibleFields(value: string[]) {
  if (typeof window === "undefined") return;
  const normalized = Array.from(new Set(value.map((item) => String(item || "").trim()).filter(Boolean)));
  window.localStorage.setItem(CASE_VISIBLE_FIELDS_STORAGE_KEY, JSON.stringify(normalized));
}

function matchesCaseSearch(
  item: any,
  includeQuery: string,
  excludeQuery: string,
  mode: SearchMode,
  logic: SearchLogic,
  ownerName: string,
): boolean {
  if (mode === "fulltext") {
    return true;
  }
  const searchableChunks = [
    item?.id,
    item?.caseNumber,
    item?.title,
    item?.description,
    item?.source,
    item?.incidentType,
    item?.status,
    item?.statusCode,
    item?.sev,
    item?.stage,
    ownerName,
    ...(Array.isArray(item?.tags) ? item.tags : []),
    ...(item?.customFields && typeof item.customFields === "object"
      ? Object.entries(item.customFields).flatMap(([key, value]) => [key, value as any])
      : []),
  ]
    .map((value) => String(value || ""))
    .join(" ")
    .toLowerCase();

  const include = includeQuery.trim();
  if (include) {
    if (mode === "regex") {
      try {
        const includeRe = new RegExp(include, "i");
        if (!includeRe.test(searchableChunks)) {
          return false;
        }
      } catch {
        return false;
      }
    } else {
      const tokens = tokenizeCaseSearch(include.toLowerCase());
      if (tokens.length > 0) {
        const matched = logic === "all"
          ? tokens.every((token) => searchableChunks.includes(token))
          : tokens.some((token) => searchableChunks.includes(token));
        if (!matched) {
          return false;
        }
      }
    }
  }

  const exclude = excludeQuery.trim();
  if (exclude) {
    if (mode === "regex") {
      try {
        const excludeRe = new RegExp(exclude, "i");
        if (excludeRe.test(searchableChunks)) {
          return false;
        }
      } catch {
        return false;
      }
    } else {
      const tokens = tokenizeCaseSearch(exclude.toLowerCase());
      if (tokens.some((token) => token && searchableChunks.includes(token))) {
        return false;
      }
    }
  }

  return true;
}

const INTEGRATION_REQUIRED_CASE_FIELDS = [
  { key: "mdm", label: "MDM" },
  { key: "scenario_id", label: "Scenario ID" },
  { key: "incident_date", label: "Incident Date" },
] as const;

function createCaseCustomFieldRow(partial?: Partial<CaseCustomFieldRow>): CaseCustomFieldRow {
  return {
    id: `case_cf_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
    key: partial?.key || "",
    value: partial?.value || "",
  };
}

export default function CasesPage() {
  const t = useT();
  const now = useNow(60_000);
  const [search, setSearch] = useState("");
  const [searchExclude, setSearchExclude] = useState("");
  const [searchScope, setSearchScope] = useState<"include" | "exclude">("include");
  const [searchMode, setSearchMode] = useState<SearchMode>("plain");
  const [searchLogic, setSearchLogic] = useState<SearchLogic>("all");
  const [severity, setSeverity] = useState<string>("all");
  const [assignedFilter, setAssignedFilter] = useState<AssignedFilterMode>("all");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [stageFilter, setStageFilter] = useState<string>("all");
  const [assigneeFilter, setAssigneeFilter] = useState<string>("all");
  const [ruleFilter, setRuleFilter] = useState("");
  const [inboundEventFilter, setInboundEventFilter] = useState("");
  const [hasObservablesFilter, setHasObservablesFilter] = useState<"all" | "with" | "without">("all");
  const [customFieldKeyFilter, setCustomFieldKeyFilter] = useState("");
  const [customFieldValueFilter, setCustomFieldValueFilter] = useState("");
  const [timeInWorkMinHours, setTimeInWorkMinHours] = useState("");
  const [dateRange, setDateRange] = useState<{ from: Date | undefined; to: Date | undefined }>({ from: undefined, to: undefined });
  const [tagInput, setTagInput] = useState("");
  const [activeTags, setActiveTags] = useState<string[]>([]);
  const [teamFilter, setTeamFilter] = useState<string>("all");
  const [tagColors] = useState<Record<string, string>>(() => loadTagColors());
  const [selectedCaseIds, setSelectedCaseIds] = useState<string[]>([]);
  const [bulkCaseStatus, setBulkCaseStatus] = useState("");
  const [bulkCaseTag, setBulkCaseTag] = useState("");
  const [bulkCasePending, setBulkCasePending] = useState(false);
  const [deleteCaseDialogOpen, setDeleteCaseDialogOpen] = useState(false);
  const [deleteCaseIDs, setDeleteCaseIDs] = useState<string[]>([]);
  const [createCaseOpen, setCreateCaseOpen] = useState(false);
  const [createTaskOpen, setCreateTaskOpen] = useState(false);
  const [newCase, setNewCase] = useState({
    templateId: "",
    title: "",
    description: "",
    severity: "High",
    source: "manual",
    incidentType: "",
    status: "",
    priority: "medium",
    confidence: 0,
    tlp: "amber",
    pap: "amber",
    tags: "",
    detectedAt: "",
    occurredAt: "",
  });
  const [newCaseCustomFields, setNewCaseCustomFields] = useState<CaseCustomFieldRow[]>(
    INTEGRATION_REQUIRED_CASE_FIELDS.map((item) => createCaseCustomFieldRow({ key: item.key, value: "" })),
  );
  const [newTask, setNewTask] = useState({
    caseId: "",
    title: "",
    description: "",
    status: "new",
    assigneeId: "",
    dueAt: "",
  });
  const [visibleCaseFields, setVisibleCaseFields] = useState<string[]>(() => loadCaseVisibleFields());
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState<number>(10);
  const [sortBy, setSortBy] = useState<string>("updated_at");
  const [sortOrder, setSortOrder] = useState<"asc" | "desc">("desc");
  const [groupBy, setGroupBy] = useState<CaseGroupBy>("none");
  const [pageInput, setPageInput] = useState("1");
  const [includeClosedCases, setIncludeClosedCases] = useState(false);
  const [location, setLocation] = useLocation();
  const queryHydratedRef = useRef(false);

  const { currentTenantId, currentTenantSlug, currentUserId } = useAppState();
  const { data: allCases = [], isLoading: allCasesLoading } = useCases(currentTenantId);
  const normalizedCustomSortBy = normalizeCaseCustomSortBy(sortBy);
  const resolvedSortBy = CASE_SORT_FIELD_SET.has(sortBy as (typeof BASE_CASE_SORT_FIELD_OPTIONS)[number]["value"])
    ? sortBy
    : (normalizedCustomSortBy || "updated_at");
  const assignedFilterForQuery: AssignedFilterMode = assigneeFilter === "unassigned" ? "unassigned" : assignedFilter;
  const assigneeFilterForQuery =
    assigneeFilter !== "all" &&
    assigneeFilter !== "unassigned" &&
    assignedFilterForQuery !== "mine" &&
    assignedFilterForQuery !== "unassigned"
      ? assigneeFilter
      : "";
  const { data: casesPageData, isLoading: casesPageLoading, isFetching: casesPageFetching } = useCasesPage(
    currentTenantId,
    page,
    pageSize,
    assignedFilterForQuery,
    search,
    resolvedSortBy,
    sortOrder,
    searchExclude,
    searchMode,
    searchLogic,
    assigneeFilterForQuery,
  );
  const cases = casesPageData?.items || [];
  const caseIDsForSummary = useMemo(
    () => cases.map((item: any) => String(item?.id || "").trim()).filter(Boolean),
    [cases],
  );
  const { data: caseSummaries = [] } = useCasesSummaries(currentTenantId, caseIDsForSummary);
  const totalCases = Number(casesPageData?.total || 0);
  const totalPages = Math.max(1, Number(casesPageData?.totalPages || 1));
  const { data: caseStatuses = [], isLoading: caseStatusesLoading } = useCaseStatuses(currentTenantId);
  const { data: caseTemplates = [], isLoading: caseTemplatesLoading } = useCaseTemplates(currentTenantId);
  const { data: currentUser } = useUser(currentUserId);
  const { data: tenantUsers = [], isLoading: tenantUsersLoading } = useUsers(currentTenantId);
  const updateCase = useUpdateCase();
  const createCase = useCreateCase();
  const createTask = useCreateCaseTask();
  const deleteCase = useDeleteCase();
  const deleteCasesBulk = useDeleteCasesBulk();
  const caseStatusOptions = useMemo(() => {
    if (caseStatuses.length > 0) {
      return caseStatuses;
    }
    return [
      { code: "new", label: "New", isClosed: false },
      { code: "open", label: "Open", isClosed: false },
      { code: "resolved", label: "Resolved", isClosed: true },
      { code: "closed", label: "Closed", isClosed: true },
    ];
  }, [caseStatuses]);
  const openStatusCode = useMemo(() => {
    const primaryOpen = caseStatusOptions.find((item: any) => !item.isClosed)?.code;
    if (primaryOpen) return primaryOpen;
    return "open";
  }, [caseStatusOptions]);
  const closedStatusCodes = useMemo(() => {
    const out = new Set<string>();
    caseStatusOptions.forEach((item: any) => {
      if (!item?.isClosed) return;
      const code = normalizeCaseText(item?.code);
      if (!code) return;
      out.add(code);
    });
    return out;
  }, [caseStatusOptions]);
  const closedStatusCode = useMemo(() => {
    const firstClosed = caseStatusOptions.find((item: any) => item?.isClosed)?.code;
    return firstClosed || "closed";
  }, [caseStatusOptions]);
  const statusFilterIsClosed = useMemo(() => {
    if (statusFilter === "all") return false;
    return closedStatusCodes.has(normalizeCaseText(statusFilter));
  }, [closedStatusCodes, statusFilter]);

  const caseTemplateByID = useMemo(() => {
    const map = new Map<string, any>();
    (caseTemplates || []).forEach((template: any) => {
      if (!template?.id) return;
      map.set(String(template.id), template);
    });
    return map;
  }, [caseTemplates]);

  const usersByID = useMemo(() => {
    const map = new Map<string, any>();
    tenantUsers.forEach((user: any) => {
      if (!user?.id) return;
      map.set(String(user.id), user);
    });
    return map;
  }, [tenantUsers]);

  const taskCaseOptions = useMemo(() => {
    const sourceItems = allCases.length > 0 ? allCases : cases;
    return [...sourceItems].sort((left: any, right: any) => {
      const leftUpdated = String(left?.updatedAt || left?.time || "");
      const rightUpdated = String(right?.updatedAt || right?.time || "");
      return rightUpdated.localeCompare(leftUpdated);
    });
  }, [allCases, cases]);

  const caseSummaryByID = useMemo(() => {
    const map = new Map<string, any>();
    caseSummaries.forEach((item: any) => {
      const key = String(item?.caseId || item?.case_id || "").trim();
      if (!key) return;
      map.set(key, item);
    });
    return map;
  }, [caseSummaries]);
  const caseByID = useMemo(() => {
    const map = new Map<string, any>();
    cases.forEach((item: any) => {
      const key = String(item?.id || "").trim();
      if (!key) return;
      map.set(key, item);
    });
    return map;
  }, [cases]);

  const availableTags = useMemo(() => {
    const counts = new Map<string, { label: string; count: number }>();
    cases.forEach((item: any) => {
      const tags = Array.isArray(item?.tags) ? item.tags : [];
      tags.forEach((tag: string) => {
        const label = String(tag || "").trim();
        if (!label) return;
        const key = label.toLowerCase();
        const current = counts.get(key);
        if (current) {
          current.count += 1;
          return;
        }
        counts.set(key, { label, count: 1 });
      });
    });
    return Array.from(counts.values()).sort((left, right) => left.label.localeCompare(right.label));
  }, [cases]);

  const caseSortFieldOptions = useMemo(() => {
    const sourceItems = allCases.length > 0 ? allCases : cases;
    const customFieldKeys = new Set<string>();
    sourceItems.forEach((item: any) => {
      const customFields = item?.customFields && typeof item.customFields === "object" ? item.customFields : {};
      Object.keys(customFields).forEach((key) => {
        const normalized = normalizeCaseText(key);
        if (!normalized) return;
        customFieldKeys.add(normalized);
      });
    });
    const customOptions = Array.from(customFieldKeys)
      .sort((left, right) => left.localeCompare(right))
      .map((key) => ({ value: `cf:${key}`, label: `CF: ${key}` }));
    const out = [...BASE_CASE_SORT_FIELD_OPTIONS, ...customOptions];
    const activeCustom = normalizeCaseCustomSortBy(sortBy);
    if (activeCustom && !out.some((item) => item.value === activeCustom)) {
      out.push({ value: activeCustom, label: `CF: ${activeCustom.slice(3)}` });
    }
    return out;
  }, [allCases, cases, sortBy]);

  const caseVisibleFieldOptions = useMemo<CaseVisibleFieldOption[]>(() => {
    const baseOptions: CaseVisibleFieldOption[] = [
      { key: "id", label: "ID" },
      { key: "status", label: "Status" },
      { key: "tags", label: "Tags" },
      { key: "assignee", label: "Assignee" },
      { key: "created", label: "Created" },
      { key: "work_time", label: "Time In Work" },
      { key: "custom_field_values_count", label: "Custom Field Values" },
      { key: "rule", label: "Rule" },
      { key: "source", label: "Source" },
      { key: "discussion", label: "Discussion" },
      { key: "task_progress", label: "Task Progress" },
      { key: "responder_statuses", label: "Responder Statuses" },
    ];
    const customFieldKeys = new Set<string>();
    const sourceItems = allCases.length > 0 ? allCases : cases;
    sourceItems.forEach((item: any) => {
      const customFields = item?.customFields && typeof item.customFields === "object" ? item.customFields : {};
      Object.keys(customFields).forEach((key) => {
        const normalized = normalizeCaseText(key);
        if (!normalized) return;
        customFieldKeys.add(normalized);
      });
    });
    const customOptions = Array.from(customFieldKeys)
      .sort((left, right) => left.localeCompare(right))
      .map((key) => ({ key: `cf:${key}`, label: `CF: ${key}` }));
    return [...baseOptions, ...customOptions];
  }, [allCases, cases]);

  const visibleCaseFieldSet = useMemo(() => {
    const availableKeys = new Set(caseVisibleFieldOptions.map((item) => item.key));
    const filtered = visibleCaseFields.filter((key) => availableKeys.has(key));
    const resolved = filtered.length > 0 ? filtered : DEFAULT_CASE_VISIBLE_FIELDS.filter((key) => availableKeys.has(key));
    return new Set(resolved);
  }, [caseVisibleFieldOptions, visibleCaseFields]);

  const selectedCustomCaseFieldKeys = useMemo(() => {
    return Array.from(visibleCaseFieldSet)
      .filter((key) => key.startsWith("cf:"))
      .map((key) => key.slice(3))
      .filter(Boolean);
  }, [visibleCaseFieldSet]);

  const filteredCases = useMemo(() => {
    const minHours = Number.parseFloat(String(timeInWorkMinHours || "").trim());
    const hasTimeFilter = Number.isFinite(minHours) && minHours > 0;
    const normalizedRule = normalizeCaseText(ruleFilter);
    const normalizedInboundEvent = normalizeCaseText(inboundEventFilter);
    const normalizedCustomFieldKey = normalizeCaseText(customFieldKeyFilter);
    const normalizedCustomFieldValue = normalizeCaseText(customFieldValueFilter);

    return cases.filter((c: any) => {
      const assigneeID = caseAssigneeID(c);
      const ownerName = assigneeID ? String(usersByID.get(assigneeID)?.name || usersByID.get(assigneeID)?.email || "").toLowerCase() : "";
      if (!matchesCaseSearch(c, search, searchExclude, searchMode, searchLogic, ownerName)) return false;

      if (severity !== "all" && c.sev !== severity) return false;
      if (statusFilter !== "all" && normalizeCaseText(c.statusCode || c.status) !== normalizeCaseText(statusFilter)) return false;
      if (!includeClosedCases && !statusFilterIsClosed && isClosedCaseItem(c, closedStatusCodes)) return false;
      if (stageFilter !== "all" && normalizeCaseText(c.stage) !== normalizeCaseText(stageFilter)) return false;
      if (assignedFilter === "assigned" && !assigneeID) return false;
      if (assignedFilter === "unassigned" && !!assigneeID) return false;
      if (assignedFilter === "mine" && assigneeID !== currentUserId) return false;
      if (assigneeFilter === "unassigned" && !!assigneeID) return false;
      if (assigneeFilter !== "all" && assigneeFilter !== "unassigned" && assigneeID !== assigneeFilter) return false;
      if (teamFilter !== "all") {
        const assigneeTeam = normalizeCaseText(usersByID.get(assigneeID || "")?.team || "");
        if (!assigneeTeam || assigneeTeam !== normalizeCaseText(teamFilter)) return false;
      }

      if (dateRange.from && new Date(c.time) < dateRange.from) return false;
      if (dateRange.to && new Date(c.time) > dateRange.to) return false;

      if (activeTags.length > 0 && !activeTags.every(t => c.tags.map((at: string) => at.toLowerCase()).includes(t.toLowerCase()))) return false;
      if (normalizedRule && !normalizeCaseText(c.incidentType).includes(normalizedRule)) return false;
      if (normalizedInboundEvent) {
        const inboundValue = normalizeCaseText(c?.customFields?.inbound_event || "");
        if (!inboundValue.includes(normalizedInboundEvent)) return false;
      }
      if (hasObservablesFilter === "with" && !hasCaseObservables(c)) return false;
      if (hasObservablesFilter === "without" && hasCaseObservables(c)) return false;
      if (hasTimeFilter && caseInWorkHours(c) < minHours) return false;
      if (normalizedCustomFieldKey) {
        const customFields = c?.customFields && typeof c.customFields === "object" ? c.customFields : {};
        const entries = Object.entries(customFields).map(([key, value]) => ({
          key: normalizeCaseText(key),
          value: normalizeCaseText(value),
        }));
        const customFieldMatched = entries.some((entry) => {
          if (!entry.key.includes(normalizedCustomFieldKey)) return false;
          if (!normalizedCustomFieldValue) return true;
          return entry.value.includes(normalizedCustomFieldValue);
        });
        if (!customFieldMatched) return false;
      } else if (normalizedCustomFieldValue) {
        const customFields = c?.customFields && typeof c.customFields === "object" ? c.customFields : {};
        const values = Object.values(customFields).map((value) => normalizeCaseText(value));
        if (!values.some((value) => value.includes(normalizedCustomFieldValue))) return false;
      }

      return true;
    });
  }, [
    cases,
    search,
    searchExclude,
    searchMode,
    searchLogic,
    severity,
    statusFilter,
    includeClosedCases,
    statusFilterIsClosed,
    closedStatusCodes,
    stageFilter,
    assignedFilter,
    assigneeFilter,
    teamFilter,
    ruleFilter,
    inboundEventFilter,
    hasObservablesFilter,
    customFieldKeyFilter,
    customFieldValueFilter,
    timeInWorkMinHours,
    dateRange,
    activeTags,
    currentUserId,
    usersByID,
  ]);

  const sortedCases = useMemo(() => {
    const direction = sortOrder === "asc" ? 1 : -1;
    const source = [...filteredCases];
    source.sort((left, right) => {
      const leftValue = sortValueByField(left, sortBy, usersByID);
      const rightValue = sortValueByField(right, sortBy, usersByID);

      if (typeof leftValue === "number" || typeof rightValue === "number") {
        const leftNum = Number(leftValue || 0);
        const rightNum = Number(rightValue || 0);
        if (leftNum < rightNum) return -1 * direction;
        if (leftNum > rightNum) return 1 * direction;
      } else {
        const leftText = String(leftValue || "").trim();
        const rightText = String(rightValue || "").trim();
        if (!leftText && rightText) return 1;
        if (leftText && !rightText) return -1;
        const textCmp = leftText.localeCompare(rightText);
        if (textCmp !== 0) return textCmp * direction;
      }

      const leftUpdated = new Date(String(left?.updatedAt || left?.time || "")).getTime();
      const rightUpdated = new Date(String(right?.updatedAt || right?.time || "")).getTime();
      if (leftUpdated > rightUpdated) return -1;
      if (leftUpdated < rightUpdated) return 1;
      return String(left?.id || "").localeCompare(String(right?.id || ""));
    });
    return source;
  }, [filteredCases, sortBy, sortOrder, usersByID]);

  const groupedCases = useMemo<CaseGroupSection[]>(() => {
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
        case "rule":
          return String(item.incidentType || "No rule");
        case "owner": {
          const assigneeID = caseAssigneeID(item);
          if (!assigneeID) return "Unassigned";
          const ownerUser = usersByID.get(assigneeID);
          return String(ownerUser?.name || ownerUser?.email || assigneeID || "Unknown assignee");
        }
        case "work_time": {
          const hours = caseInWorkHours(item);
          if (hours < 4) return "< 4h";
          if (hours < 24) return "4h - 24h";
          if (hours < 72) return "1d - 3d";
          return ">= 3d";
        }
        case "source":
          return String(item.source || "Unknown source");
        case "date":
          return normalizeDate(String(item.time || ""));
        default:
          return "All cases";
      }
    };

    if (groupBy === "none") {
      return [{ key: "all", label: "All cases", items: sortedCases }];
    }
    const buckets = new Map<string, CaseGroupSection>();
    sortedCases.forEach((item) => {
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
  }, [groupBy, sortedCases, usersByID]);

  const activeFiltersCount = useMemo(() => {
    let count = 0;
    if (search.trim()) count += 1;
    if (searchExclude.trim()) count += 1;
    if (searchMode !== "plain") count += 1;
    if (searchLogic !== "all") count += 1;
    if (severity !== "all") count += 1;
    if (statusFilter !== "all") count += 1;
    if (assignedFilter !== "all") count += 1;
    if (assigneeFilter !== "all") count += 1;
    if (teamFilter !== "all") count += 1;
    if (stageFilter !== "all") count += 1;
    if (ruleFilter.trim()) count += 1;
    if (inboundEventFilter.trim()) count += 1;
    if (hasObservablesFilter !== "all") count += 1;
    if (customFieldKeyFilter.trim()) count += 1;
    if (customFieldValueFilter.trim()) count += 1;
    if (String(timeInWorkMinHours || "").trim()) count += 1;
    if (dateRange.from || dateRange.to) count += 1;
    if (activeTags.length > 0) count += 1;
    if (sortBy !== "updated_at" || sortOrder !== "desc") count += 1;
    if (groupBy !== "none") count += 1;
    if (includeClosedCases) count += 1;
    return count;
  }, [
    search,
    searchExclude,
    searchMode,
    searchLogic,
    severity,
    statusFilter,
    assignedFilter,
    assigneeFilter,
    teamFilter,
    stageFilter,
    ruleFilter,
    inboundEventFilter,
    hasObservablesFilter,
    customFieldKeyFilter,
    customFieldValueFilter,
    timeInWorkMinHours,
    dateRange.from,
    dateRange.to,
    activeTags.length,
    sortBy,
    sortOrder,
    groupBy,
    includeClosedCases,
  ]);

  const caseFilterSummaryItems = useMemo(() => {
    const items: Array<{ key: string; label: string }> = [];
    const normalizedSearch = search.trim();
    const normalizedExclude = searchExclude.trim();
    if (normalizedSearch) {
      items.push({ key: "search", label: `Search: ${normalizedSearch}` });
    }
    if (normalizedExclude) {
      items.push({ key: "exclude", label: `Exclude: ${normalizedExclude}` });
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
    if (assigneeFilter !== "all") {
      if (assigneeFilter === "unassigned") {
        items.push({ key: "assignee", label: "Assignee: Unassigned" });
      } else {
        const assignee = usersByID.get(String(assigneeFilter));
        const label = String(assignee?.name || assignee?.email || assigneeFilter);
        items.push({ key: "assignee", label: `Assignee: ${label}` });
      }
    } else if (assignedFilter !== "all") {
      items.push({ key: "owner", label: `Owner: ${assignedFilter}` });
    }
    if (dateRange.from || dateRange.to) {
      const fromLabel = dateRange.from ? format(dateRange.from, "dd.MM.yyyy") : "…";
      const toLabel = dateRange.to ? format(dateRange.to, "dd.MM.yyyy") : "…";
      items.push({ key: "date-range", label: `Date: ${fromLabel} - ${toLabel}` });
    }
    if (activeTags.length > 0) {
      items.push({ key: "tags", label: `Tags: ${activeTags.join(", ")}` });
    }
    if (groupBy !== "none") {
      const groupLabel = CASE_GROUP_OPTIONS.find((item) => item.value === groupBy)?.label || groupBy;
      items.push({ key: "group-by", label: `Group: ${groupLabel}` });
    }
    if (sortBy !== "updated_at" || sortOrder !== "desc") {
      const sortLabel = caseSortFieldOptions.find((item: { value: string; label: string }) => item.value === sortBy)?.label || sortBy;
      items.push({ key: "sort", label: `Sort: ${sortLabel} (${sortOrder})` });
    }
    if (includeClosedCases) {
      items.push({ key: "include-closed", label: "Include closed" });
    }
    return items;
  }, [
    activeTags,
    assignedFilter,
    assigneeFilter,
    dateRange.from,
    dateRange.to,
    groupBy,
    includeClosedCases,
    search,
    searchExclude,
    searchLogic,
    searchMode,
    severity,
    sortBy,
    sortOrder,
    statusFilter,
    caseSortFieldOptions,
    usersByID,
  ]);

  const activeSearchValue = searchScope === "include" ? search : searchExclude;

  useEffect(() => {
    setSelectedCaseIds((prev) => prev.filter((id) => cases.some((item: any) => item.id === id)));
  }, [cases]);

  useEffect(() => {
    if (!casesPageData) return;
    setPage((prev) => Math.min(Math.max(1, prev), totalPages));
  }, [casesPageData, totalPages]);

  useEffect(() => {
    setPageInput(String(page));
  }, [page]);

  useEffect(() => {
    const { params } = splitLocationPathAndSearch(location);
    const includeQuery = String(params.get("q") || "");
    const excludeQuery = String(params.get("q_not") || "");
    setSearch(includeQuery);
    setSearchExclude(excludeQuery);
    if (excludeQuery.trim() && !includeQuery.trim()) {
      setSearchScope("exclude");
    } else {
      setSearchScope("include");
    }
    const searchModeParam = String(params.get("search_mode") || "").trim().toLowerCase();
    if (searchModeParam === "plain" || searchModeParam === "regex" || searchModeParam === "fulltext") {
      setSearchMode(searchModeParam as SearchMode);
    }
    const searchLogicParam = String(params.get("search_logic") || "").trim().toLowerCase();
    if (searchLogicParam === "all" || searchLogicParam === "any") {
      setSearchLogic(searchLogicParam as SearchLogic);
    }
    const severityParam = params.get("severity");
    setSeverity(severityParam || "all");
    const statusParam = params.get("status");
    setStatusFilter(statusParam || "all");
    const stageParam = params.get("stage");
    setStageFilter(stageParam || "all");
    const assigneeParam = params.get("assignee");
    setAssigneeFilter(assigneeParam || "all");
    const teamParam = params.get("team");
    setTeamFilter(teamParam || "all");
    setRuleFilter(params.get("rule") || "");
    setInboundEventFilter(params.get("inbound_event") || "");
    const hasObservablesParam = String(params.get("has_observables") || "").trim().toLowerCase();
    if (hasObservablesParam === "with" || hasObservablesParam === "without" || hasObservablesParam === "all") {
      setHasObservablesFilter(hasObservablesParam as "all" | "with" | "without");
    }
    setCustomFieldKeyFilter(params.get("cf_key") || "");
    setCustomFieldValueFilter(params.get("cf_value") || "");
    setTimeInWorkMinHours(params.get("time_work_min_h") || "");
    const assignedParam = params.get("assigned");
    if (assignedParam === "all" || assignedParam === "assigned" || assignedParam === "unassigned" || assignedParam === "mine") {
      setAssignedFilter(assignedParam as AssignedFilterMode);
    } else {
      setAssignedFilter("all");
    }
    const tagsParam = params.get("tags");
    if (tagsParam) {
      const parsedTags = tagsParam.split(",").map((item) => item.trim()).filter(Boolean);
      setActiveTags(Array.from(new Set(parsedTags)));
    } else {
      setActiveTags([]);
    }
    const fieldsParam = params.get("fields");
    if (fieldsParam) {
      const parsedFields = fieldsParam.split(",").map((item) => item.trim()).filter(Boolean);
      if (parsedFields.length > 0) {
        setVisibleCaseFields(Array.from(new Set(parsedFields)));
      }
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
    if (CASE_SORT_FIELD_SET.has(sortByParam as (typeof BASE_CASE_SORT_FIELD_OPTIONS)[number]["value"])) {
      setSortBy(sortByParam);
    } else {
      const customSortBy = normalizeCaseCustomSortBy(sortByParam);
      if (customSortBy) {
        setSortBy(customSortBy);
      }
    }
    const sortOrderParam = String(params.get("sort_order") || "").trim().toLowerCase();
    if (CASE_SORT_ORDER_SET.has(sortOrderParam as (typeof CASE_SORT_ORDER_OPTIONS)[number]["value"])) {
      setSortOrder(sortOrderParam as "asc" | "desc");
    }
    const groupByParam = String(params.get("group_by") || "").trim().toLowerCase();
    if (CASE_GROUP_SET.has(groupByParam as CaseGroupBy)) {
      setGroupBy(groupByParam as CaseGroupBy);
    }
    const parsedPage = parsePositiveIntParam(params.get("page"), page);
    if (parsedPage >= 1) {
      setPage(parsedPage);
      setPageInput(String(parsedPage));
    }
    const closedParam = String(params.get("closed") || "").trim().toLowerCase();
    setIncludeClosedCases(closedParam === "include");
    queryHydratedRef.current = true;
  }, []);

  useEffect(() => {
    if (!queryHydratedRef.current) return;
    const nextLocation = applySearchPatch(location, {
      q: search.trim() || undefined,
      q_not: searchExclude.trim() || undefined,
      search_mode: searchMode !== "plain" ? searchMode : undefined,
      search_logic: searchLogic !== "all" ? searchLogic : undefined,
      severity: severity !== "all" ? severity : undefined,
      status: statusFilter !== "all" ? statusFilter : undefined,
      stage: stageFilter !== "all" ? stageFilter : undefined,
      assigned: assignedFilter !== "all" ? assignedFilter : undefined,
      assignee: assigneeFilter !== "all" ? assigneeFilter : undefined,
      team: teamFilter !== "all" ? teamFilter : undefined,
      rule: ruleFilter.trim() || undefined,
      inbound_event: inboundEventFilter.trim() || undefined,
      has_observables: hasObservablesFilter !== "all" ? hasObservablesFilter : undefined,
      cf_key: customFieldKeyFilter.trim() || undefined,
      cf_value: customFieldValueFilter.trim() || undefined,
      time_work_min_h: timeInWorkMinHours.trim() || undefined,
      from: formatDateParam(dateRange.from),
      to: formatDateParam(dateRange.to),
      tags: activeTags.length > 0 ? activeTags.join(",") : undefined,
      fields: visibleCaseFields.join(",") !== DEFAULT_CASE_VISIBLE_FIELDS.join(",") ? visibleCaseFields.join(",") : undefined,
      sort_by: sortBy !== "updated_at" ? sortBy : undefined,
      sort_order: sortOrder !== "desc" ? sortOrder : undefined,
      group_by: groupBy !== "none" ? groupBy : undefined,
      closed: includeClosedCases ? "include" : undefined,
      page: page > 1 ? String(page) : undefined,
      page_size: pageSize !== 10 ? String(pageSize) : undefined,
    });
    const currentLocation =
      typeof window !== "undefined" ? `${window.location.pathname}${window.location.search}` : location;
    if (nextLocation !== currentLocation) {
      setLocation(nextLocation, { replace: true });
    }
  }, [
    search,
    searchExclude,
    searchMode,
    searchLogic,
    severity,
    statusFilter,
    stageFilter,
    assignedFilter,
    assigneeFilter,
    teamFilter,
    ruleFilter,
    inboundEventFilter,
    hasObservablesFilter,
    customFieldKeyFilter,
    customFieldValueFilter,
    timeInWorkMinHours,
    dateRange.from,
    dateRange.to,
    activeTags,
    visibleCaseFields,
    sortBy,
    sortOrder,
    groupBy,
    includeClosedCases,
    page,
    pageSize,
    location,
    setLocation,
  ]);

  useEffect(() => {
    saveTagColors(tagColors);
  }, [tagColors]);

  useEffect(() => {
    const availableKeys = new Set(caseVisibleFieldOptions.map((item) => item.key));
    setVisibleCaseFields((prev) => {
      const normalized = prev.filter((key) => availableKeys.has(key));
      if (normalized.length > 0) {
        return normalized;
      }
      return DEFAULT_CASE_VISIBLE_FIELDS.filter((key) => availableKeys.has(key));
    });
  }, [caseVisibleFieldOptions]);

  useEffect(() => {
    saveCaseVisibleFields(visibleCaseFields);
  }, [visibleCaseFields]);

  useEffect(() => {
    if (!createCaseOpen) {
      return;
    }
    setNewCase((prev) => ({
      ...prev,
      status: prev.status || openStatusCode,
    }));
    setNewCaseCustomFields((prev) => ensureRequiredCustomFields(prev));
  }, [createCaseOpen, openStatusCode]);

  const allFilteredSelected = filteredCases.length > 0 && filteredCases.every((item: any) => selectedCaseIds.includes(item.id));
  const selectedCount = selectedCaseIds.length;

  const addTag = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && tagInput.trim()) {
      if (!activeTags.includes(tagInput.trim())) {
        setActiveTags([...activeTags, tagInput.trim()]);
        setPage(1);
      }
      setTagInput("");
    }
  };

  const getTagColor = (tag: string): string => {
    const key = String(tag || "").trim().toLowerCase();
    const configured = tagColors[key];
    if (configured) return formatTagColorValue(configured);
    return "#2563eb";
  };

  const isCaseFieldVisible = (key: string): boolean => visibleCaseFieldSet.has(key);

  const toggleTagFilter = (tag: string) => {
    const normalized = String(tag || "").trim();
    if (!normalized) return;
    setActiveTags((prev) => {
      const exists = prev.some((item) => item.toLowerCase() === normalized.toLowerCase());
      if (exists) {
        return prev.filter((item) => item.toLowerCase() !== normalized.toLowerCase());
      }
      return [...prev, normalized];
    });
    setPage(1);
  };

  const resetCaseFilters = () => {
    setSearch("");
    setSearchExclude("");
    setSearchScope("include");
    setSearchMode("plain");
    setSearchLogic("all");
    setSeverity("all");
    setAssignedFilter("all");
    setStatusFilter("all");
    setStageFilter("all");
    setAssigneeFilter("all");
    setRuleFilter("");
    setInboundEventFilter("");
    setHasObservablesFilter("all");
    setCustomFieldKeyFilter("");
    setCustomFieldValueFilter("");
    setTimeInWorkMinHours("");
    setDateRange({ from: undefined, to: undefined });
    setTagInput("");
    setActiveTags([]);
    setTeamFilter("all");
    setSortBy("updated_at");
    setSortOrder("desc");
    setGroupBy("none");
    setIncludeClosedCases(false);
    setPage(1);
  };

  const toggleCaseSelection = (caseId: string) => {
    setSelectedCaseIds((prev) => (prev.includes(caseId) ? prev.filter((id) => id !== caseId) : [...prev, caseId]));
  };

  const toggleSelectAllFiltered = (checked: boolean) => {
    if (!checked) {
      setSelectedCaseIds((prev) => prev.filter((id) => !filteredCases.some((item: any) => item.id === id)));
      return;
    }
    setSelectedCaseIds((prev) => {
      const merged = new Set(prev);
      filteredCases.forEach((item: any) => merged.add(item.id));
      return Array.from(merged);
    });
  };

  const handleAssign = (e: React.MouseEvent, id: string) => {
    e.preventDefault();
    e.stopPropagation();
    const actorId = String(currentUser?.id || currentUserId || "").trim();
    if (!actorId) return;
    updateCase.mutate({ id, data: { assignee: actorId, status: openStatusCode } });
    toast.success(t("cases.toast.assigned"));
  };

  const mutateCaseAsync = (id: string, data: Record<string, any>) =>
    new Promise<void>((resolve, reject) => {
      updateCase.mutate(
        { id, data },
        {
          onSuccess: () => resolve(),
          onError: (error: any) => reject(error),
        },
      );
    });

  const applyBulkCaseUpdate = async (
    buildPayload: (item: any) => Record<string, any>,
    successKey: string,
    partialKey: string,
  ) => {
    if (selectedCaseIds.length === 0) {
      toast.error(t("cases.bulk.selectCasesRequired"));
      return;
    }
    if (bulkCasePending) {
      return;
    }
    const ids = [...selectedCaseIds];
    setBulkCasePending(true);
    try {
      const settled = await Promise.allSettled(
        ids.map((id) => mutateCaseAsync(id, buildPayload(caseByID.get(id) || {}))),
      );
      const failedIDs: string[] = [];
      settled.forEach((result, idx) => {
        if (result.status === "rejected") {
          failedIDs.push(ids[idx]);
        }
      });
      const failed = failedIDs.length;
      const updated = settled.length - failed;
      setSelectedCaseIds(failedIDs);
      if (failed > 0) {
        toast.error(t(partialKey, { updated: updated.toString(), failed: failed.toString() }));
        return;
      }
      toast.success(t(successKey, { count: updated.toString() }));
    } finally {
      setBulkCasePending(false);
    }
  };

  const handleBulkCaseStatusApply = () => {
    const statusCode = String(bulkCaseStatus || "").trim();
    if (!statusCode) {
      toast.error(t("cases.bulk.selectStatusRequired"));
      return;
    }
    void applyBulkCaseUpdate(
      () => ({ status: statusCode }),
      "cases.bulk.statusSuccess",
      "cases.bulk.statusPartial",
    );
  };

  const handleBulkCaseClose = () => {
    void applyBulkCaseUpdate(
      () => ({ status: closedStatusCode }),
      "cases.bulk.closeSuccess",
      "cases.bulk.closePartial",
    );
  };

  const handleBulkCaseTagAdd = () => {
    const nextTag = String(bulkCaseTag || "").trim();
    if (!nextTag) {
      toast.error(t("cases.bulk.tagRequired"));
      return;
    }
    void applyBulkCaseUpdate(
      (item) => {
        const existing = Array.isArray(item?.tags) ? item.tags : [];
        const normalized = existing
          .map((tag: any) => String(tag || "").trim())
          .filter(Boolean);
        return { tags: Array.from(new Set([...normalized, nextTag])) };
      },
      "cases.bulk.tagSuccess",
      "cases.bulk.tagPartial",
    );
    setBulkCaseTag("");
  };

  const openDeleteCaseConfirm = (e: React.MouseEvent, caseId: string) => {
    e.preventDefault();
    e.stopPropagation();
    setDeleteCaseIDs([caseId]);
    setDeleteCaseDialogOpen(true);
  };

  const openDeleteSelectedCasesConfirm = () => {
    if (selectedCaseIds.length === 0) {
      toast.error(t("cases.bulk.selectCasesRequired"));
      return;
    }
    setDeleteCaseIDs([...selectedCaseIds]);
    setDeleteCaseDialogOpen(true);
  };

  const handleConfirmDeleteCases = () => {
    const idsToDelete = [...deleteCaseIDs];
    if (idsToDelete.length === 0) {
      setDeleteCaseDialogOpen(false);
      return;
    }
    setDeleteCaseDialogOpen(false);
    setDeleteCaseIDs([]);

    if (idsToDelete.length === 1) {
      const caseID = idsToDelete[0];
      deleteCase.mutate(caseID, {
        onSuccess: () => {
          setSelectedCaseIds((prev) => prev.filter((item) => item !== caseID));
          toast.success(t("cases.toast.deleted"));
        },
        onError: (error: any) => {
          toast.error(error?.message || t("cases.toast.deleteFailed"));
        },
      });
      return;
    }

    deleteCasesBulk.mutate(idsToDelete, {
      onSuccess: (result: any) => {
        const deleted = Number(result?.deleted ?? 0);
        const failed = Number(result?.failed ?? 0);
        setSelectedCaseIds([]);
        if (failed > 0) {
          toast.error(t("cases.bulk.deletePartial", { deleted: deleted.toString(), failed: failed.toString() }));
          return;
        }
        toast.success(t("cases.bulk.deleteSuccess", { count: deleted.toString() }));
      },
      onError: (error: any) => {
        toast.error(error?.message || t("cases.toast.deleteFailed"));
      },
    });
  };

  const normalizeCustomFieldKey = (value: string): string =>
    value.trim().toLowerCase().replace(/\s+/g, "_");

  const ensureRequiredCustomFields = (rows: CaseCustomFieldRow[]): CaseCustomFieldRow[] => {
    const nextRows = [...rows];
    const seen = new Set(nextRows.map((row) => normalizeCustomFieldKey(row.key)).filter(Boolean));
    for (const field of INTEGRATION_REQUIRED_CASE_FIELDS) {
      if (seen.has(field.key)) {
        continue;
      }
      nextRows.push(createCaseCustomFieldRow({ key: field.key, value: "" }));
      seen.add(field.key);
    }
    return nextRows;
  };

  const resetCreateCaseForm = () => {
    setNewCase({
      templateId: "",
      title: "",
      description: "",
      severity: "High",
      source: "manual",
      incidentType: "",
      status: openStatusCode,
      priority: "medium",
      confidence: 0,
      tlp: "amber",
      pap: "amber",
      tags: "",
      detectedAt: "",
      occurredAt: "",
    });
    setNewCaseCustomFields(INTEGRATION_REQUIRED_CASE_FIELDS.map((item) => createCaseCustomFieldRow({ key: item.key, value: "" })));
  };

  const resetCreateTaskForm = () => {
    setNewTask({
      caseId: "",
      title: "",
      description: "",
      status: "new",
      assigneeId: "",
      dueAt: "",
    });
  };

  const toggleVisibleCaseField = (key: string, checked: boolean) => {
    setVisibleCaseFields((prev) => {
      if (checked) {
        if (prev.includes(key)) return prev;
        return [...prev, key];
      }
      const next = prev.filter((item) => item !== key);
      return next.length > 0 ? next : prev;
    });
  };

  const applyCaseTemplate = (templateID: string) => {
    const normalizedTemplateID = String(templateID || "").trim();
    const template = caseTemplateByID.get(normalizedTemplateID);
    if (!template) {
      setNewCase((prev) => ({ ...prev, templateId: "" }));
      setNewCaseCustomFields((prev) => ensureRequiredCustomFields(prev));
      return;
    }
    setNewCase((prev) => ({
      ...prev,
      templateId: normalizedTemplateID,
      title: String(template?.name || "").trim() || prev.title,
      description: String(template?.description || "").trim() || prev.description,
      severity: String(template?.severity || "").trim() || prev.severity,
      status: String(template?.status || "").trim() || prev.status,
      source: String(template?.source || "").trim() || prev.source,
      incidentType: String(template?.incidentType || template?.incident_type || "").trim() || prev.incidentType,
      priority: String(template?.priority || "").trim() || prev.priority,
      confidence: Number.isFinite(Number(template?.confidence)) ? Number(template?.confidence) : prev.confidence,
      tlp: String(template?.tlp || "").trim() || prev.tlp,
      pap: String(template?.pap || "").trim() || prev.pap,
      detectedAt: String(template?.detectedAt || template?.detected_at || "").trim() || prev.detectedAt,
      occurredAt: String(template?.occurredAt || template?.occurred_at || "").trim() || prev.occurredAt,
      tags: Array.isArray(template?.tags) ? template.tags.join(", ") : prev.tags,
    }));
    const templateFields = template?.customFields && typeof template.customFields === "object" ? template.customFields : {};
    const rows = Object.entries(templateFields).map(([key, value]) =>
      createCaseCustomFieldRow({
        key: String(key || ""),
        value: value === null || value === undefined ? "" : String(value),
      }),
    );
    setNewCaseCustomFields(ensureRequiredCustomFields(rows));
  };

  const handleAddSOARPresetFields = () => {
    const withRequired = ensureRequiredCustomFields(newCaseCustomFields);
    const merged = mergeMissingSOARCaseFields(
      withRequired,
      (preset) => createCaseCustomFieldRow({ key: preset.key, value: "" }),
      normalizeCustomFieldKey,
    );
    setNewCaseCustomFields(merged);
    if (merged.length > withRequired.length) {
      toast.success(t("cases.toast.soarFieldsAdded"));
      return;
    }
    toast.success(t("cases.toast.soarFieldsAlreadyPresent"));
  };

  const handleCreateCase = () => {
    if (!newCase.title.trim()) {
      toast.error(t("cases.toast.titleRequired"));
      return;
    }
    const toISO = (value: string) => {
      const trimmed = value.trim();
      if (!trimmed) return "";
      const parsed = new Date(trimmed);
      if (Number.isNaN(parsed.getTime())) return "";
      return parsed.toISOString();
    };
    const tags = newCase.tags
      .split(",")
      .map((item) => item.trim())
      .filter(Boolean)
      .filter((value, index, all) => all.findIndex((item) => item.toLowerCase() === value.toLowerCase()) === index);

    const customFieldsPayload: Record<string, string> = {};
    for (const row of newCaseCustomFields) {
      const normalizedKey = normalizeCustomFieldKey(row.key);
      const value = String(row.value || "");
      if (!normalizedKey && !value.trim()) {
        continue;
      }
      if (!normalizedKey && value.trim()) {
        toast.error(t("cases.toast.customFieldKeyRequired"));
        return;
      }
      if (Object.prototype.hasOwnProperty.call(customFieldsPayload, normalizedKey)) {
        toast.error(t("cases.toast.customFieldDuplicate"));
        return;
      }
      customFieldsPayload[normalizedKey] = value;
    }

    const fallbackIncidentDate = toISO(newCase.occurredAt) || toISO(newCase.detectedAt);
    if (!String(customFieldsPayload.incident_date || "").trim() && fallbackIncidentDate) {
      customFieldsPayload.incident_date = fallbackIncidentDate;
    }
    const missingRequiredFields = INTEGRATION_REQUIRED_CASE_FIELDS
      .map((item) => item.key)
      .filter((key) => !String(customFieldsPayload[key] || "").trim());
    if (missingRequiredFields.length > 0) {
      toast.error(t("cases.toast.integrationFieldsRequired"));
      return;
    }

    createCase.mutate(
      {
        title: newCase.title.trim(),
        description: newCase.description.trim(),
        severity: newCase.severity,
        source: newCase.source.trim() || "manual",
        incidentType: newCase.incidentType.trim(),
        priority: newCase.priority,
        confidence: Number(newCase.confidence || 0),
        tlp: newCase.tlp,
        pap: newCase.pap,
        tags,
        customFields: customFieldsPayload,
        detectedAt: toISO(newCase.detectedAt),
        occurredAt: toISO(newCase.occurredAt),
        status: newCase.status || openStatusCode,
      },
      {
        onSuccess: (created: any) => {
          toast.success(t("cases.toast.created"));
          setCreateCaseOpen(false);
          resetCreateCaseForm();
          if (created?.id) {
            setLocation(withTenantPath(currentTenantSlug, `/cases/${created.id}`));
          }
        },
      },
    );
  };

  const handleCreateTask = () => {
    const caseId = String(newTask.caseId || "").trim();
    const title = String(newTask.title || "").trim();
    const dueAt = String(newTask.dueAt || "").trim();
    if (!caseId) {
      toast.error("Select case");
      return;
    }
    if (!title) {
      toast.error("Task title is required");
      return;
    }
    if (!dueAt) {
      toast.error("Task due date is required");
      return;
    }
    const dueDateISO = new Date(dueAt).toISOString();
    createTask.mutate(
      {
        caseId,
        title,
        description: String(newTask.description || "").trim(),
        status: String(newTask.status || "new").trim().toLowerCase(),
        assigneeId: String(newTask.assigneeId || "").trim() || undefined,
        dueDate: dueDateISO,
      },
      {
        onSuccess: () => {
          toast.success("Task created");
          setCreateTaskOpen(false);
          resetCreateTaskForm();
        },
      },
    );
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
      className={`${CASE_PANEL_CLASS} p-4`}
      data-testid={`cases-pagination-${position}`}
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
              className={`${CASE_PAGE_SIZE_SELECT_CLASS} w-full sm:w-40`}
              data-testid={position === "bottom" ? "select-cases-page-size" : `select-cases-page-size-${position}`}
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
          <p className={CASE_MUTED_TEXT_CLASS}>
            {totalCases > 0
              ? t("pagination.showing", {
                from: String((page - 1) * pageSize + 1),
                to: String(Math.min(page * pageSize, totalCases)),
                total: String(totalCases),
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
            className="h-9 w-9 rounded-lg border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]"
            data-testid={position === "bottom" ? "button-cases-prev-page" : `button-cases-prev-page-${position}`}
          >
            <ChevronLeft size={14} />
          </Button>
          <div
            className="inline-flex h-8 min-w-[92px] items-center justify-center rounded-md border border-[#2a2c3c] bg-[#0b0c10] px-2 text-xs font-medium text-[#d1d5db]"
            data-testid={position === "bottom" ? "cases-page-indicator" : `cases-page-indicator-${position}`}
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
              data-testid={position === "bottom" ? "input-cases-page" : `input-cases-page-${position}`}
            />
            <span className="mx-0.5 text-[#6b7280]">/</span>
            <span className="min-w-[22px] text-left">{totalPages}</span>
          </div>
          <Button
            variant="outline"
            size="icon"
            onClick={() => setPage((prev) => Math.min(totalPages, prev + 1))}
            disabled={page >= totalPages}
            className="h-9 w-9 rounded-lg border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]"
            data-testid={position === "bottom" ? "button-cases-next-page" : `button-cases-next-page-${position}`}
          >
            <ChevronRight size={14} />
          </Button>
        </div>
      </div>
    </Card>
  );

  const hasCasesPayload = Boolean(casesPageData);
  const isCasesBootstrapLoading = Boolean(
    !hasCasesPayload &&
      (casesPageLoading || caseStatusesLoading || caseTemplatesLoading || tenantUsersLoading || allCasesLoading),
  );
  const showCasesPageSkeleton = useMinimumLoading(isCasesBootstrapLoading);
  const showCasesEntitiesSkeleton = useMinimumLoading(Boolean(hasCasesPayload && casesPageFetching));

  if (showCasesPageSkeleton) {
    return <CasesLoadingSkeleton />;
  }

  return (
    <AppLayout>
      <div className={CASE_PAGE_SHELL_CLASS}>
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h1 className="text-[32px] font-semibold leading-8 tracking-[-0.5px] text-white">{t("cases.title")}</h1>
            <p className={`mt-1 ${CASE_MUTED_TEXT_CLASS}`}>Manage and track security investigation cases</p>
          </div>
          <div className="flex items-center gap-2" data-testid="cases-pagination-top">
            <Button
              variant="outline"
              size="icon"
              onClick={() => setPage((prev) => Math.max(1, prev - 1))}
              disabled={page <= 1}
              className="h-12 w-12 rounded-xl border-[#2a2c3c] bg-[#0f1118] text-[#9ca3af] hover:border-[#4b5168] hover:bg-[#171b2a] hover:text-[#d1d5db]"
              data-testid="button-cases-prev-page-top"
            >
              <ChevronLeft size={18} />
            </Button>
            <span className="text-sm font-medium text-[#6b7280]">Page</span>
            <div className="inline-flex h-12 min-w-[86px] items-center justify-center rounded-xl border border-[#2a2c3c] bg-[#0b0c10] px-2">
              <Input
                value={pageInput}
                onChange={(event) => setPageInput(event.target.value.replace(/[^\d]/g, ""))}
                onBlur={() => goToPage(pageInput)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    goToPage(pageInput);
                  }
                }}
                className="h-8 w-10 border-none bg-transparent px-0 text-center text-sm text-[#f3f4f6] focus-visible:ring-0"
                inputMode="numeric"
                aria-label={t("pagination.goToPage")}
                data-testid="input-cases-page-top"
              />
            </div>
            <span className="text-sm text-[#6b7280]">of {totalPages}</span>
            <Button
              variant="outline"
              size="icon"
              onClick={() => setPage((prev) => Math.min(totalPages, prev + 1))}
              disabled={page >= totalPages}
              className="h-12 w-12 rounded-xl border-[#2a2c3c] bg-[#0f1118] text-[#9ca3af] hover:border-[#4b5168] hover:bg-[#171b2a] hover:text-[#d1d5db]"
              data-testid="button-cases-next-page-top"
            >
              <ChevronRight size={18} />
            </Button>
            <Select
              value={String(pageSize)}
              onValueChange={(value) => {
                const parsed = Number(value) || PAGE_SIZE_OPTIONS[0];
                setPageSize(parsed);
                setPage(1);
              }}
            >
              <SelectTrigger
                className="h-12 w-[152px] rounded-xl border border-[#2a2c3c] bg-[#0f1118] text-sm text-[#d1d5db] focus:ring-1 focus:ring-[#3b4a79]"
                data-testid="select-cases-page-size-top"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                {PAGE_SIZE_OPTIONS.map((size) => (
                  <SelectItem key={`cases-page-size-top-${size}`} value={String(size)}>
                    {size} / page
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="sr-only">
            <div className="flex flex-wrap items-center gap-2">
            <Popover>
              <PopoverTrigger asChild>
                <Button variant="outline" className={CASE_ACTION_BUTTON_CLASS} data-testid="button-case-visible-fields-hidden">
                  <SlidersHorizontal size={14} />
                  Fields
                </Button>
              </PopoverTrigger>
              <PopoverContent className={`w-[320px] ${CASE_PANEL_CLASS} border-[#2a2c3c] p-4 text-white`} align="end">
                <div className="space-y-3">
                  <div className="flex items-center justify-between">
                    <p className="text-sm font-semibold">Visible fields</p>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="text-[#9ca3af] hover:bg-[#171b2a] hover:text-[#d1d5db]"
                      onClick={() => setVisibleCaseFields([...DEFAULT_CASE_VISIBLE_FIELDS])}
                    >
                      Reset
                    </Button>
                  </div>
                  <div className="max-h-64 space-y-2 overflow-y-auto pr-1">
                    {caseVisibleFieldOptions.map((option) => (
                      <label key={option.key} className="flex cursor-pointer items-center gap-2 text-sm text-[#d1d5db]">
                        <Checkbox
                          checked={visibleCaseFieldSet.has(option.key)}
                          onCheckedChange={(checked) => toggleVisibleCaseField(option.key, Boolean(checked))}
                        />
                        <span>{option.label}</span>
                      </label>
                    ))}
                  </div>
                </div>
              </PopoverContent>
            </Popover>

            <Dialog
              open={createTaskOpen}
              onOpenChange={(open) => {
                setCreateTaskOpen(open);
                if (!open) {
                  resetCreateTaskForm();
                }
              }}
            >
              <DialogTrigger asChild>
                <Button variant="outline" className={CASE_ACTION_BUTTON_CLASS} data-testid="button-new-task">
                  <ListChecks size={14} />
                  New Task
                </Button>
              </DialogTrigger>
              <DialogContent className={`${CASE_PANEL_CLASS} rounded-2xl border-[#2a2c3c] text-white`}>
                <DialogHeader>
                  <DialogTitle>Create Task</DialogTitle>
                  <DialogDescription className={CASE_MUTED_TEXT_CLASS}>Manual task creation for any case.</DialogDescription>
                </DialogHeader>
                <div className="space-y-3">
                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold text-[#9ca3af]">Case</label>
                    <Select value={newTask.caseId || "none"} onValueChange={(value) => setNewTask((prev) => ({ ...prev, caseId: value === "none" ? "" : value }))}>
                      <SelectTrigger className={DARK_SELECT_TRIGGER_CLASS} data-testid="select-new-task-case">
                        <SelectValue placeholder="Select case" />
                      </SelectTrigger>
                      <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                        <SelectItem value="none">Select case</SelectItem>
                        {taskCaseOptions.map((item: any) => (
                          <SelectItem key={item.id} value={String(item.id)}>
                            {item.caseNumber || item.id} · {item.title}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold text-[#9ca3af]">{t("cases.field.title")}</label>
                    <Input
                      value={newTask.title}
                      onChange={(event) => setNewTask((prev) => ({ ...prev, title: event.target.value }))}
                      placeholder="Task title"
                      className={DARK_INPUT_CLASS}
                      data-testid="input-new-task-title"
                    />
                  </div>
                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold text-[#9ca3af]">{t("cases.field.description")}</label>
                    <Textarea
                      value={newTask.description}
                      onChange={(event) => setNewTask((prev) => ({ ...prev, description: event.target.value }))}
                      placeholder="Task description"
                      rows={3}
                      className="rounded-xl border border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] placeholder:text-[#6b7280] focus-visible:ring-[#3b4a79]"
                    />
                  </div>
                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold text-[#9ca3af]">Due date</label>
                    <Input
                      type="datetime-local"
                      value={newTask.dueAt}
                      onChange={(event) => setNewTask((prev) => ({ ...prev, dueAt: event.target.value }))}
                      className={DARK_INPUT_CLASS}
                      data-testid="input-new-task-due-date"
                    />
                    <div className="mt-2 flex flex-wrap items-center gap-2" data-testid="new-task-due-presets">
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        className="h-8 rounded-lg text-xs"
                        onClick={() =>
                          setNewTask((prev) => ({
                            ...prev,
                            dueAt: format(new Date(Date.now() + 60 * 60 * 1000), "yyyy-MM-dd'T'HH:mm"),
                          }))
                        }
                      >
                        +1h
                      </Button>
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        className="h-8 rounded-lg text-xs"
                        onClick={() =>
                          setNewTask((prev) => ({
                            ...prev,
                            dueAt: format(new Date(Date.now() + 4 * 60 * 60 * 1000), "yyyy-MM-dd'T'HH:mm"),
                          }))
                        }
                      >
                        +4h
                      </Button>
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        className="h-8 rounded-lg text-xs"
                        onClick={() =>
                          setNewTask((prev) => ({
                            ...prev,
                            dueAt: format(new Date(Date.now() + 24 * 60 * 60 * 1000), "yyyy-MM-dd'T'HH:mm"),
                          }))
                        }
                      >
                        +1d
                      </Button>
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        className="h-8 rounded-lg text-xs"
                        onClick={() =>
                          setNewTask((prev) => ({
                            ...prev,
                            dueAt: format(new Date(Date.now() + 3 * 24 * 60 * 60 * 1000), "yyyy-MM-dd'T'HH:mm"),
                          }))
                        }
                      >
                        +3d
                      </Button>
                    </div>
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div className="space-y-1.5">
                      <label className="text-xs font-semibold text-[#9ca3af]">{t("cases.field.status")}</label>
                      <Select value={newTask.status} onValueChange={(value) => setNewTask((prev) => ({ ...prev, status: value }))}>
                        <SelectTrigger className={DARK_SELECT_TRIGGER_CLASS}>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                          <SelectItem value="new">New</SelectItem>
                          <SelectItem value="in_progress">In progress</SelectItem>
                          <SelectItem value="done">Done</SelectItem>
                          <SelectItem value="blocked">Blocked</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-1.5">
                      <label className="text-xs font-semibold text-[#9ca3af]">Assignee</label>
                      <Select value={newTask.assigneeId || "none"} onValueChange={(value) => setNewTask((prev) => ({ ...prev, assigneeId: value === "none" ? "" : value }))}>
                        <SelectTrigger className={DARK_SELECT_TRIGGER_CLASS}>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                          <SelectItem value="none">Unassigned</SelectItem>
                          {tenantUsers.map((user: any) => (
                            <SelectItem key={user.id} value={String(user.id)}>
                              {user.name || user.email || user.id}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  </div>
                </div>
                <DialogFooter>
                  <Button variant="outline" className={CASE_ACTION_BUTTON_CLASS} onClick={() => setCreateTaskOpen(false)}>
                    {t("common.cancel")}
                  </Button>
                  <Button className="h-11 rounded-xl border border-[#4adf37] bg-[#4adf37] px-5 font-semibold text-[#0b0c10] hover:bg-[#61f44f]" onClick={handleCreateTask} disabled={createTask.isPending || !String(newTask.caseId || '').trim() || !String(newTask.title || '').trim() || !String(newTask.dueAt || '').trim()} data-testid="button-create-task-confirm">
                    {createTask.isPending ? "Creating..." : "Create Task"}
                  </Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>

          <Dialog
            open={createCaseOpen}
            onOpenChange={(open) => {
              setCreateCaseOpen(open);
              if (!open) {
                resetCreateCaseForm();
              }
            }}
          >
            <DialogTrigger asChild>
              <Button className="h-11 rounded-xl gap-2 border border-[#4adf37] bg-[#4adf37] px-6 font-semibold text-[#0b0c10] shadow-[0_6px_18px_rgba(74,223,55,0.35)] hover:bg-[#61f44f]" data-testid="button-new-case">
                <Plus size={20} /> {t("cases.button.newCase")}
              </Button>
            </DialogTrigger>
            <DialogContent className={`${CASE_PANEL_CLASS} rounded-2xl border-[#2a2c3c] text-white`}>
              <DialogHeader>
                <DialogTitle>{t("cases.dialog.createTitle")}</DialogTitle>
                <DialogDescription className={CASE_MUTED_TEXT_CLASS}>{t("cases.dialog.createDescription")}</DialogDescription>
              </DialogHeader>
              <div className="space-y-4">
                <div className="space-y-2">
                  <label className="text-sm font-medium text-[#9ca3af]">{t("cases.field.template")}</label>
                  <Select
                    value={newCase.templateId || "none"}
                    onValueChange={(value) => {
                      if (value === "none") {
                        setNewCase((prev) => ({ ...prev, templateId: "" }));
                        setNewCaseCustomFields((prev) => ensureRequiredCustomFields(prev));
                        return;
                      }
                      applyCaseTemplate(value);
                    }}
                  >
                    <SelectTrigger className={DARK_SELECT_TRIGGER_CLASS} data-testid="select-new-case-template">
                      <SelectValue placeholder={t("cases.placeholder.template")} />
                    </SelectTrigger>
                    <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                      <SelectItem value="none">{t("cases.template.none")}</SelectItem>
                      {caseTemplates.map((template: any) => (
                        <SelectItem key={template.id} value={String(template.id)}>
                          {template.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium text-[#9ca3af]">{t("cases.field.title")}</label>
                  <Input
                    value={newCase.title}
                    onChange={(e) => setNewCase((prev) => ({ ...prev, title: e.target.value }))}
                    placeholder={t("cases.placeholder.title")}
                    className={DARK_INPUT_CLASS}
                    data-testid="input-new-case-title"
                  />
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium text-[#9ca3af]">{t("cases.field.severity")}</label>
                  <Select value={newCase.severity} onValueChange={(v) => setNewCase((prev) => ({ ...prev, severity: v }))}>
                    <SelectTrigger className={DARK_SELECT_TRIGGER_CLASS} data-testid="select-new-case-severity">
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
                <div className="space-y-2">
                  <label className="text-sm font-medium text-[#9ca3af]">{t("cases.field.status")}</label>
                  <Select value={newCase.status || openStatusCode} onValueChange={(v) => setNewCase((prev) => ({ ...prev, status: v }))}>
                    <SelectTrigger className={DARK_SELECT_TRIGGER_CLASS} data-testid="select-new-case-status">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                      {caseStatuses.map((item: any) => (
                        <SelectItem key={item.code} value={item.code}>
                          {item.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div className="space-y-2">
                    <label className="text-sm font-medium text-[#9ca3af]">{t("cases.field.source")}</label>
                    <Input
                      value={newCase.source}
                      onChange={(e) => setNewCase((prev) => ({ ...prev, source: e.target.value }))}
                      placeholder={t("cases.placeholder.source")}
                      className={DARK_INPUT_CLASS}
                      data-testid="input-new-case-source"
                    />
                  </div>
                  <div className="space-y-2">
                    <label className="text-sm font-medium text-[#9ca3af]">{t("cases.field.incidentType")}</label>
                    <Input
                      value={newCase.incidentType}
                      onChange={(e) => setNewCase((prev) => ({ ...prev, incidentType: e.target.value }))}
                      placeholder={t("cases.placeholder.incidentType")}
                      className={DARK_INPUT_CLASS}
                      data-testid="input-new-case-incident-type"
                    />
                  </div>
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium text-[#9ca3af]">{t("cases.field.tags")}</label>
                  <Input
                    value={newCase.tags}
                    onChange={(e) => setNewCase((prev) => ({ ...prev, tags: e.target.value }))}
                    placeholder={t("cases.placeholder.tags")}
                    className={DARK_INPUT_CLASS}
                    data-testid="input-new-case-tags"
                  />
                </div>
                <div className="grid grid-cols-3 gap-3">
                  <div className="space-y-2">
                    <label className="text-sm font-medium text-[#9ca3af]">{t("cases.field.priority")}</label>
                    <Select value={newCase.priority} onValueChange={(v) => setNewCase((prev) => ({ ...prev, priority: v }))}>
                      <SelectTrigger className={DARK_SELECT_TRIGGER_CLASS} data-testid="select-new-case-priority">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                        <SelectItem value="critical">{t("severity.critical")}</SelectItem>
                        <SelectItem value="high">{t("severity.high")}</SelectItem>
                        <SelectItem value="medium">{t("severity.medium")}</SelectItem>
                        <SelectItem value="low">{t("severity.low")}</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <label className="text-sm font-medium text-[#9ca3af]">{t("cases.field.tlp")}</label>
                    <Select value={newCase.tlp} onValueChange={(v) => setNewCase((prev) => ({ ...prev, tlp: v }))}>
                      <SelectTrigger className={DARK_SELECT_TRIGGER_CLASS} data-testid="select-new-case-tlp"><SelectValue /></SelectTrigger>
                      <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                        <SelectItem value="red">RED</SelectItem>
                        <SelectItem value="amber">AMBER</SelectItem>
                        <SelectItem value="green">GREEN</SelectItem>
                        <SelectItem value="clear">CLEAR</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <label className="text-sm font-medium text-[#9ca3af]">{t("cases.field.pap")}</label>
                    <Select value={newCase.pap} onValueChange={(v) => setNewCase((prev) => ({ ...prev, pap: v }))}>
                      <SelectTrigger className={DARK_SELECT_TRIGGER_CLASS} data-testid="select-new-case-pap"><SelectValue /></SelectTrigger>
                      <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                        <SelectItem value="red">RED</SelectItem>
                        <SelectItem value="amber">AMBER</SelectItem>
                        <SelectItem value="green">GREEN</SelectItem>
                        <SelectItem value="clear">CLEAR</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                </div>
                <div className="grid grid-cols-3 gap-3">
                  <div className="space-y-2">
                    <label className="text-sm font-medium text-[#9ca3af]">{t("cases.field.confidence")}</label>
                    <Input
                      type="number"
                      min={0}
                      max={100}
                      value={newCase.confidence}
                      onChange={(e) => setNewCase((prev) => ({ ...prev, confidence: Math.max(0, Math.min(100, Number(e.target.value || 0))) }))}
                      className={DARK_INPUT_CLASS}
                      data-testid="input-new-case-confidence"
                    />
                  </div>
                  <div className="space-y-2">
                    <label className="text-sm font-medium text-[#9ca3af]">{t("cases.field.detectedAt")}</label>
                    <Input
                      type="datetime-local"
                      value={newCase.detectedAt}
                      onChange={(e) => setNewCase((prev) => ({ ...prev, detectedAt: e.target.value }))}
                      className={`${DARK_INPUT_CLASS} font-mono`}
                      data-testid="input-new-case-detected-at"
                    />
                  </div>
                  <div className="space-y-2">
                    <label className="text-sm font-medium text-[#9ca3af]">{t("cases.field.occurredAt")}</label>
                    <Input
                      type="datetime-local"
                      value={newCase.occurredAt}
                      onChange={(e) => setNewCase((prev) => ({ ...prev, occurredAt: e.target.value }))}
                      className={`${DARK_INPUT_CLASS} font-mono`}
                      data-testid="input-new-case-occurred-at"
                    />
                  </div>
                </div>
                <div className={`${CASE_SUBPANEL_CLASS} space-y-2 p-3`}>
                  <div className="flex items-center justify-between gap-3">
                    <Label className="text-sm font-medium text-[#d1d5db]">{t("cases.field.customFields")}</Label>
                    <div className="flex items-center gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        className="border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]"
                        onClick={handleAddSOARPresetFields}
                        data-testid="button-add-new-case-soar-fields"
                      >
                        <Layers size={12} className="mr-1" />
                        {t("cases.button.addSoarFields")}
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        className="border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]"
                        onClick={() => setNewCaseCustomFields((prev) => [...prev, createCaseCustomFieldRow()])}
                        data-testid="button-add-new-case-custom-field"
                      >
                        <Plus size={12} className="mr-1" />
                        {t("cases.button.addCustomField")}
                      </Button>
                    </div>
                  </div>
                  <p className={CASE_MUTED_TEXT_CLASS}>{t("cases.field.integrationHint")}</p>
                  <div className="space-y-2">
                    {newCaseCustomFields.map((row, index) => {
                      const normalizedKey = normalizeCustomFieldKey(row.key);
                      const requiredField = INTEGRATION_REQUIRED_CASE_FIELDS.find((item) => item.key === normalizedKey);
                      const soarField = isSOARCaseFieldKey(normalizedKey);
                      const canRemove = !requiredField;
                      return (
                        <div key={row.id} className="grid grid-cols-[1fr_1fr_auto] gap-2" data-testid={`new-case-custom-field-row-${index}`}>
                          <Input
                            value={row.key}
                            onChange={(event) =>
                              setNewCaseCustomFields((prev) =>
                                prev.map((item) => (item.id === row.id ? { ...item, key: event.target.value } : item)),
                              )
                            }
                            placeholder={t("cases.placeholder.customFieldKey")}
                            className={DARK_INPUT_CLASS}
                            data-testid={`input-new-case-custom-field-key-${index}`}
                          />
                          <Input
                            value={row.value}
                            onChange={(event) =>
                              setNewCaseCustomFields((prev) =>
                                prev.map((item) => (item.id === row.id ? { ...item, value: event.target.value } : item)),
                              )
                            }
                            placeholder={requiredField ? requiredField.label : soarField ? t(`soarCaseField.${normalizedKey}.placeholder`) : t("cases.placeholder.customFieldValue")}
                            className={DARK_INPUT_CLASS}
                            data-testid={`input-new-case-custom-field-value-${index}`}
                          />
                          <Button
                            variant="ghost"
                            size="icon"
                            className="text-[#9ca3af] hover:bg-[#171b2a] hover:text-[#d1d5db]"
                            disabled={!canRemove}
                            onClick={() =>
                              setNewCaseCustomFields((prev) => prev.filter((item) => item.id !== row.id))
                            }
                            data-testid={`button-remove-new-case-custom-field-${index}`}
                          >
                            <X size={14} />
                          </Button>
                        </div>
                      );
                    })}
                  </div>
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium text-[#9ca3af]">{t("cases.field.description")}</label>
                  <Textarea
                    value={newCase.description}
                    onChange={(e) => setNewCase((prev) => ({ ...prev, description: e.target.value }))}
                    placeholder={t("cases.placeholder.description")}
                    rows={4}
                    className="rounded-xl border border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] placeholder:text-[#6b7280] focus-visible:ring-[#3b4a79]"
                    data-testid="input-new-case-description"
                  />
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" className={CASE_ACTION_BUTTON_CLASS} onClick={() => setCreateCaseOpen(false)}>{t("common.cancel")}</Button>
                <Button className="h-11 rounded-xl border border-[#4adf37] bg-[#4adf37] px-6 font-semibold text-[#0b0c10] hover:bg-[#61f44f]" onClick={handleCreateCase} disabled={createCase.isPending} data-testid="button-create-case-confirm">
                  {createCase.isPending ? t("cases.button.creating") : t("cases.button.createCase")}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
          </div>
        </div>
        </div>

        <Card className={`${CASE_PANEL_CLASS} p-5`}>
          <CollapsibleFiltersPanel
            storageKey={CASE_FILTERS_COLLAPSED_STORAGE_KEY}
            activeFiltersCount={activeFiltersCount}
            summaryItems={caseFilterSummaryItems}
            onReset={resetCaseFilters}
          >
          <div className="grid grid-cols-1 gap-4 md:grid-cols-12">
            <div className="space-y-2 md:col-span-6">
              <label className="text-sm font-medium text-[#9ca3af]">Search</label>
              <div className="relative group">
                <Search className="absolute left-4 top-1/2 -translate-y-1/2 text-[#6b7280] transition-colors group-focus-within:text-[#9ca3af]" size={22} />
                <Input
                  placeholder="Search cases by title, ID, description..."
                  value={activeSearchValue}
                  onChange={(e) => {
                    const nextValue = e.target.value;
                    if (searchScope === "include") {
                      setSearch(nextValue);
                    } else {
                      setSearchExclude(nextValue);
                    }
                    setPage(1);
                  }}
                  className="h-14 rounded-xl border border-[#2a2c3c] bg-[#0b0c10] pl-12 text-sm text-[#d1d5db] placeholder:text-[#6b7280] focus-visible:ring-1 focus-visible:ring-[#3b4a79]"
                  data-testid="input-cases-search-include"
                />
              </div>
              <div className="flex items-center gap-3 pt-1">
                <Button
                  type="button"
                  variant="outline"
                  className={`h-10 rounded-lg border px-4 text-sm font-medium ${searchScope === "include" ? "border-[#4adf37] bg-[#4adf37] text-[#0b0c10] hover:bg-[#61f44f]" : "border-[#2a2c3c] bg-[#1d1e29] text-[#9ca3af] hover:bg-[#232537] hover:text-[#d1d5db]"}`}
                  onClick={() => setSearchScope("include")}
                >
                  Include
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  className={`h-10 rounded-lg border px-4 text-sm font-medium ${searchScope === "exclude" ? "border-[#4adf37] bg-[#4adf37] text-[#0b0c10] hover:bg-[#61f44f]" : "border-[#2a2c3c] bg-[#1d1e29] text-[#9ca3af] hover:bg-[#232537] hover:text-[#d1d5db]"}`}
                  onClick={() => setSearchScope("exclude")}
                >
                  Exclude
                </Button>
                <span className="mx-1 h-5 w-px bg-[#2a2c3c]" />
                <Button
                  type="button"
                  variant="outline"
                  className={`h-10 rounded-lg border px-4 text-sm font-medium ${searchLogic === "all" ? "border-[#4adf37] bg-[#4adf37] text-[#0b0c10] hover:bg-[#61f44f]" : "border-[#2a2c3c] bg-[#1d1e29] text-[#9ca3af] hover:bg-[#232537] hover:text-[#d1d5db]"}`}
                  onClick={() => {
                    setSearchLogic("all");
                    setPage(1);
                  }}
                >
                  All
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  className={`h-10 rounded-lg border px-4 text-sm font-medium ${searchLogic === "any" ? "border-[#4adf37] bg-[#4adf37] text-[#0b0c10] hover:bg-[#61f44f]" : "border-[#2a2c3c] bg-[#1d1e29] text-[#9ca3af] hover:bg-[#232537] hover:text-[#d1d5db]"}`}
                  onClick={() => {
                    setSearchLogic("any");
                    setPage(1);
                  }}
                >
                  Any
                </Button>
              </div>
            </div>

            <div className="space-y-2 md:col-span-3">
              <label className="text-sm font-medium text-[#9ca3af]">Severity</label>
              <Select
                value={severity}
                onValueChange={(value) => {
                  setSeverity(value);
                  setPage(1);
                }}
              >
                <SelectTrigger className="h-14 rounded-xl border border-[#2a2c3c] bg-[#0b0c10] text-sm text-[#d1d5db] focus:ring-1 focus:ring-[#3b4a79]" data-testid="select-cases-severity">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                  <SelectItem value="all">All Severities</SelectItem>
                  <SelectItem value="Critical">{t("severity.critical")}</SelectItem>
                  <SelectItem value="High">{t("severity.high")}</SelectItem>
                  <SelectItem value="Medium">{t("severity.medium")}</SelectItem>
                  <SelectItem value="Low">{t("severity.low")}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2 md:col-span-3">
              <label className="text-sm font-medium text-[#9ca3af]">Status</label>
              <Select
                value={statusFilter}
                onValueChange={(value) => {
                  setStatusFilter(value);
                  setPage(1);
                }}
              >
                <SelectTrigger className="h-14 rounded-xl border border-[#2a2c3c] bg-[#0b0c10] text-sm text-[#d1d5db] focus:ring-1 focus:ring-[#3b4a79]" data-testid="select-cases-status">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                  <SelectItem value="all">All Statuses</SelectItem>
                  {caseStatusOptions.map((item: any) => (
                    <SelectItem key={item.code} value={String(item.code)}>
                      {item.label || item.code}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2 md:col-span-3">
              <label className="text-sm font-medium text-[#9ca3af]">Assignee</label>
              <Select
                value={assigneeFilter}
                onValueChange={(value) => {
                  setAssigneeFilter(value);
                  if (value === "unassigned") {
                    setAssignedFilter("unassigned");
                  } else if (value !== "all" && (assignedFilter === "mine" || assignedFilter === "unassigned")) {
                    setAssignedFilter("all");
                  }
                  setPage(1);
                }}
              >
                <SelectTrigger className="h-14 rounded-xl border border-[#2a2c3c] bg-[#0b0c10] text-sm text-[#d1d5db] focus:ring-1 focus:ring-[#3b4a79]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                  <SelectItem value="all">All Users</SelectItem>
                  <SelectItem value="unassigned">Unassigned</SelectItem>
                  {tenantUsers.map((user: any) => (
                    <SelectItem key={`assignee-filter-${user.id}`} value={String(user.id)}>
                      {user.name || user.email || user.id}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2 md:col-span-3">
              <label className="text-sm font-medium text-[#9ca3af]">Date From</label>
              <div className="relative">
                <Input
                  type="date"
                  value={formatCaseDateInputValue(dateRange.from)}
                  onChange={(event) => {
                    setDateRange((prev) => ({ ...prev, from: parseCaseDateInputValue(event.target.value) }));
                    setPage(1);
                  }}
                  className="h-14 rounded-xl border border-[#2a2c3c] bg-[#0b0c10] pr-12 text-sm text-[#d1d5db] focus-visible:ring-1 focus-visible:ring-[#3b4a79]"
                />
                <Calendar className="pointer-events-none absolute right-4 top-1/2 -translate-y-1/2 text-[#9ca3af]" size={20} />
              </div>
            </div>

            <div className="space-y-2 md:col-span-3">
              <label className="text-sm font-medium text-[#9ca3af]">Date To</label>
              <div className="relative">
                <Input
                  type="date"
                  value={formatCaseDateInputValue(dateRange.to)}
                  onChange={(event) => {
                    setDateRange((prev) => ({ ...prev, to: parseCaseDateInputValue(event.target.value) }));
                    setPage(1);
                  }}
                  className="h-14 rounded-xl border border-[#2a2c3c] bg-[#0b0c10] pr-12 text-sm text-[#d1d5db] focus-visible:ring-1 focus-visible:ring-[#3b4a79]"
                />
                <Calendar className="pointer-events-none absolute right-4 top-1/2 -translate-y-1/2 text-[#9ca3af]" size={20} />
              </div>
            </div>

            <div className="space-y-2 md:col-span-3">
              <label className="text-sm font-medium text-[#9ca3af]">Tags</label>
              <Input
                placeholder="Filter by tags..."
                value={tagInput}
                onChange={(e) => setTagInput(e.target.value)}
                onKeyDown={addTag}
                className="h-14 rounded-xl border border-[#2a2c3c] bg-[#0b0c10] text-sm text-[#d1d5db] placeholder:text-[#6b7280] focus-visible:ring-1 focus-visible:ring-[#3b4a79]"
              />
            </div>

            <div className="space-y-2 md:col-span-3">
              <label className="text-sm font-medium text-[#9ca3af]">Group By</label>
              <Select
                value={groupBy}
                onValueChange={(value) => {
                  setGroupBy(value as CaseGroupBy);
                  setPage(1);
                }}
              >
                <SelectTrigger className="h-14 rounded-xl border border-[#2a2c3c] bg-[#0b0c10] text-sm text-[#d1d5db] focus:ring-1 focus:ring-[#3b4a79]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                  {CASE_GROUP_OPTIONS.map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2 md:col-span-3">
              <label className="text-sm font-medium text-[#9ca3af]">Sort By</label>
              <Select
                value={sortBy}
                onValueChange={(value) => {
                  setSortBy(value);
                  setSortOrder("desc");
                  setPage(1);
                }}
              >
                <SelectTrigger className="h-14 rounded-xl border border-[#2a2c3c] bg-[#0b0c10] text-sm text-[#d1d5db] focus:ring-1 focus:ring-[#3b4a79]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                  <SelectItem value="created_at">Created (Newest)</SelectItem>
                  <SelectItem value="updated_at">Updated (Newest)</SelectItem>
                  <SelectItem value="severity">Severity (High to Low)</SelectItem>
                  <SelectItem value="status">Status</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2 md:col-span-6">
              <label className="text-sm font-medium text-[#9ca3af]">Options</label>
              <div className="flex h-14 items-center gap-3 rounded-xl border border-[#2a2c3c] bg-[#0b0c10] px-4">
                <Checkbox
                  checked={includeClosedCases}
                  onCheckedChange={(checked) => {
                    setIncludeClosedCases(Boolean(checked));
                    setPage(1);
                  }}
                />
                <span className="text-sm text-[#d1d5db]">Include Closed</span>
              </div>
            </div>

            <div className="space-y-2 md:col-span-6">
              <label className="text-sm font-medium text-[#9ca3af]">Card Fields</label>
              <Popover>
                <PopoverTrigger asChild>
                  <Button variant="outline" className="h-14 w-full justify-between rounded-xl border border-[#2a2c3c] bg-[#0b0c10] px-4 text-sm text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]" data-testid="button-case-visible-fields">
                    <span>Customize Fields</span>
                    <SlidersHorizontal size={20} className="text-[#9ca3af]" />
                  </Button>
                </PopoverTrigger>
                <PopoverContent className={`w-[320px] ${CASE_PANEL_CLASS} border-[#2a2c3c] p-4 text-white`} align="start">
                  <div className="space-y-3">
                    <div className="flex items-center justify-between">
                      <p className="text-sm font-semibold">Visible fields</p>
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-[#9ca3af] hover:bg-[#171b2a] hover:text-[#d1d5db]"
                        onClick={() => setVisibleCaseFields([...DEFAULT_CASE_VISIBLE_FIELDS])}
                      >
                        Reset
                      </Button>
                    </div>
                    <div className="max-h-64 space-y-2 overflow-y-auto pr-1">
                      {caseVisibleFieldOptions.map((option) => (
                        <label key={`panel-${option.key}`} className="flex cursor-pointer items-center gap-2 text-sm text-[#d1d5db]">
                          <Checkbox
                            checked={visibleCaseFieldSet.has(option.key)}
                            onCheckedChange={(checked) => toggleVisibleCaseField(option.key, Boolean(checked))}
                          />
                          <span>{option.label}</span>
                        </label>
                      ))}
                    </div>
                  </div>
                </PopoverContent>
              </Popover>
            </div>
          </div>

          <div className="hidden" aria-hidden="true">
            <Input
              value={searchExclude}
              onChange={(e) => {
                setSearchExclude(e.target.value);
                setPage(1);
              }}
              data-testid="input-cases-search-exclude"
            />
            <Input
              value={ruleFilter}
              onChange={(e) => {
                setRuleFilter(e.target.value);
                setPage(1);
              }}
              data-testid="input-cases-rule"
            />
          </div>
          </CollapsibleFiltersPanel>

          <div className="sr-only">
            <button
              type="button"
              onClick={() => {
                setIncludeClosedCases((prev) => !prev);
                setPage(1);
              }}
              data-testid="button-cases-toggle-closed"
            >
              {includeClosedCases ? t("cases.closed.hide") : t("cases.closed.load")}
            </button>
            {availableTags.map((tagItem) => (
              <button key={`sr-tag-${tagItem.label}`} type="button" onClick={() => toggleTagFilter(tagItem.label)}>
                {tagItem.label}
              </button>
            ))}
          </div>
        </Card>

        <Card className={`${CASE_PANEL_CLASS} p-4`}>
          <div className="flex flex-col sm:flex-row sm:items-center gap-3">
            <div className="flex items-center gap-3">
              <Checkbox checked={allFilteredSelected} onCheckedChange={(checked: boolean) => toggleSelectAllFiltered(Boolean(checked))} data-testid="checkbox-select-all-cases" />
              <span className="text-sm font-semibold text-white">{t("cases.bulk.selected", { count: selectedCount.toString() })}</span>
              <Badge variant="outline" className="border-[#2a2c3c] bg-[#0f121b] text-xs text-[#9ca3af]">
                <Layers size={12} className="mr-1" />
                {t("cases.bulk.filtered", { count: filteredCases.length.toString() })}
              </Badge>
            </div>
            <div className="flex-1" />
            <Button
              variant="destructive"
              className="rounded-xl gap-2 border border-[#6a2f39] bg-[#341b22] text-[#ff9fb3] hover:bg-[#44232c]"
              onClick={openDeleteSelectedCasesConfirm}
              disabled={selectedCount === 0 || deleteCasesBulk.isPending || bulkCasePending}
              data-testid="button-delete-selected-cases"
            >
              <Trash2 size={14} />
              {deleteCasesBulk.isPending ? t("cases.bulk.deleting") : t("cases.bulk.deleteSelected")}
            </Button>
          </div>
          <div className="mt-3 grid gap-2 md:grid-cols-[1fr,1fr,auto,auto]">
            <Select value={bulkCaseStatus} onValueChange={setBulkCaseStatus}>
              <SelectTrigger className="w-full rounded-xl border border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db]" data-testid="select-cases-bulk-status">
                <SelectValue placeholder={t("cases.bulk.selectStatus")} />
              </SelectTrigger>
              <SelectContent className={DARK_SELECT_CONTENT_CLASS}>
                {caseStatusOptions.map((item: any) => {
                  const code = String(item?.code || "").trim();
                  if (!code) return null;
                  const label = String(item?.label || code).trim() || code;
                  return (
                    <SelectItem key={`bulk-case-status-${code}`} value={code}>
                      {label}
                    </SelectItem>
                  );
                })}
              </SelectContent>
            </Select>
            <Input
              value={bulkCaseTag}
              onChange={(event) => setBulkCaseTag(event.target.value)}
              placeholder={t("cases.bulk.tagPlaceholder")}
              className={DARK_INPUT_CLASS}
              data-testid="input-cases-bulk-tag"
            />
            <Button
              variant="outline"
              className="rounded-xl border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]"
              onClick={handleBulkCaseStatusApply}
              disabled={selectedCount === 0 || bulkCasePending}
              data-testid="button-cases-bulk-apply-status"
            >
              {bulkCasePending ? t("cases.bulk.updating") : t("cases.bulk.applyStatus")}
            </Button>
            <div className="flex gap-2">
              <Button
                variant="outline"
                className="rounded-xl border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]"
                onClick={handleBulkCaseTagAdd}
                disabled={selectedCount === 0 || bulkCasePending}
                data-testid="button-cases-bulk-add-tag"
              >
                <Tag size={14} className="mr-1" />
                {bulkCasePending ? t("cases.bulk.updating") : t("cases.bulk.addTag")}
              </Button>
              <Button
                variant="outline"
                className="rounded-xl border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]"
                onClick={handleBulkCaseClose}
                disabled={selectedCount === 0 || bulkCasePending}
                data-testid="button-cases-bulk-close"
              >
                {bulkCasePending ? t("cases.bulk.updating") : t("cases.bulk.closeSelected")}
              </Button>
            </div>
          </div>
        </Card>

        <div className={`${CASE_SUBPANEL_CLASS} px-4 py-2.5`}>
          {showCasesEntitiesSkeleton ? (
            <Skeleton className="h-4 w-48 rounded-md" />
          ) : (
            <p className={CASE_MUTED_TEXT_CLASS}>
              Showing {filteredCases.length} of {totalCases} cases
            </p>
          )}
        </div>

        <div className="grid gap-6">
          {showCasesEntitiesSkeleton ? (
            Array.from({ length: 3 }).map((_, index) => (
              <Card key={`cases-entities-skeleton-${index}`} className={`${CASE_PANEL_CLASS} overflow-hidden rounded-xl p-5`}>
                <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
                  <div className="flex min-w-0 flex-1 items-start gap-4">
                    <Skeleton className="mt-2 h-4 w-4 rounded-sm" />
                    <Skeleton className="h-11 w-11 rounded-lg" />
                    <div className="min-w-0 flex-1 space-y-3">
                      <div className="flex flex-wrap items-center gap-2">
                        <Skeleton className="h-4 w-24 rounded-md" />
                        <Skeleton className="h-5 w-20 rounded-md" />
                        <Skeleton className="h-4 w-14 rounded-md" />
                      </div>
                      <Skeleton className="h-7 w-2/3 rounded-md" />
                      <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
                        <Skeleton className="h-4 w-40 rounded-md" />
                        <Skeleton className="h-4 w-32 rounded-md" />
                        <Skeleton className="h-4 w-28 rounded-md" />
                      </div>
                      <div className={`${CASE_SUBPANEL_CLASS} space-y-2 p-2.5`}>
                        <div className="flex flex-wrap items-center gap-2">
                          <Skeleton className="h-3 w-20 rounded-md" />
                          <Skeleton className="h-3 w-20 rounded-md" />
                          <Skeleton className="h-3 w-24 rounded-md" />
                        </div>
                        <Skeleton className="h-2 w-full rounded-full" />
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-3 md:pt-1">
                    <Skeleton className="h-9 w-32 rounded-xl" />
                    <Skeleton className="h-7 w-24 rounded-full" />
                  </div>
                </div>
              </Card>
            ))
          ) : (
            groupedCases.map((group) => (
              <section key={group.key} className="space-y-3">
                {groupBy !== "none" ? (
                  <div className={`${CASE_SUBPANEL_CLASS} flex items-center justify-between px-3 py-2`}>
                    <p className="text-sm font-semibold text-white">{group.label}</p>
                    <Badge variant="outline" className="border-[#2a2c3c] text-[10px] font-semibold uppercase text-[#9ca3af]">
                      {group.items.length}
                    </Badge>
                  </div>
                ) : null}
                <div className="grid gap-4">
                  {group.items.map((c: any) => {
                    const assigneeID = caseAssigneeID(c);
                    const assigneeUser = assigneeID ? usersByID.get(assigneeID) : null;
                    const ownerLabel = assigneeUser?.name || assigneeUser?.email || (assigneeID || "Unknown user");
                    const caseSummary = caseSummaryByID.get(String(c.id)) || {
                      openTasks: 0,
                      closedTasks: 0,
                      totalTasks: 0,
                      taskProgressPercent: 0,
                      nextOpenTaskDueAt: "",
                      overdueOpenTasks: 0,
                      responderStatuses: {
                        queued: 0,
                        running: 0,
                        completed: 0,
                        failed: 0,
                      },
                    };
                    const createdAt = new Date(String(c.createdAt || c.time || ""));
                    const workHours = caseInWorkHours(c);
                    const nextTaskDueAt = String(caseSummary.nextOpenTaskDueAt || "").trim();
                    const nextTaskDueTs = nextTaskDueAt ? Date.parse(nextTaskDueAt) : 0;
                    const nextTaskDeltaMs = nextTaskDueTs ? nextTaskDueTs - now : 0;
                    const showTaskCountdown = nextTaskDueTs > 0 && Number(caseSummary.openTasks || 0) > 0;
                    return (
                      <div key={c.id}>
                        <Link href={withTenantPath(currentTenantSlug, `/cases/${c.id}`)}>
                          <Card className={`${CASE_PANEL_CLASS} group relative min-h-[182px] cursor-pointer rounded-xl p-5 transition-colors hover:border-[#4b5168] hover:bg-[linear-gradient(180deg,rgba(23,27,42,0.98),rgba(20,24,36,0.98))]`} data-testid={`case-card-${c.id}`}>
                            <Button
                              variant="destructive"
                              size="icon"
                              className="absolute -right-2 -top-2 z-20 h-7 w-7 rounded-full opacity-0 shadow-sm transition-opacity group-hover:opacity-100 group-focus-within:opacity-100"
                              onClick={(e) => openDeleteCaseConfirm(e, c.id)}
                              disabled={deleteCase.isPending || deleteCasesBulk.isPending}
                              data-testid={`button-delete-case-${c.id}`}
                            >
                              <Trash2 size={12} />
                            </Button>
                            <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
                              <div className="flex items-start gap-4 flex-1">
                                <Checkbox
                                  checked={selectedCaseIds.includes(c.id)}
                                  onClick={(e) => {
                                    e.preventDefault();
                                    e.stopPropagation();
                                    toggleCaseSelection(c.id);
                                  }}
                                  data-testid={`checkbox-case-${c.id}`}
                                  className="mt-2"
                                />
                                <div className="relative">
                                  <div className="mt-[2px] flex h-11 w-11 items-center justify-center rounded-lg border border-[#2a2c3c] bg-[#0f121b] text-[#8fb6ff]">
                                    <FolderKanban size={18} />
                                  </div>
                                  {!assigneeID ? (
                                    <span className="absolute -top-1 -right-1 h-3 w-3 rounded-full border-2 border-[#13141c] bg-[#eb5f65]" />
                                  ) : null}
                                </div>
                                <div className="min-w-0 flex-1">
                                  <div className="mb-2 flex flex-wrap items-center gap-2">
                                    {isCaseFieldVisible("id") ? (
                                      <EllipsisText text={c.id} className="max-w-[220px] font-mono text-[11px] font-semibold text-[#8b91a3]" />
                                    ) : null}
                                    {isCaseFieldVisible("status") ? (
                                      <Badge variant="outline" className="h-5 rounded-lg border-[#2a2c3c] bg-[#101827] text-[10px] font-semibold uppercase text-[#8fb6ff]">{c.status}</Badge>
                                    ) : null}
                                    {isCaseFieldVisible("tags")
                                      ? c.tags.map((tag: string) => (
                                        <Badge
                                          key={tag}
                                          variant="secondary"
                                          className="h-4 border-none px-1.5 text-[9px] font-medium"
                                          style={{
                                            backgroundColor: `${getTagColor(tag)}1f`,
                                            color: getTagColor(tag),
                                          }}
                                        >
                                          {tag}
                                        </Badge>
                                      ))
                                      : null}
                                  </div>
                                  <EllipsisText text={c.title} className="text-sm font-normal leading-8 tracking-[-0.5px] text-white" />
                                  <div className="mt-2 flex flex-wrap items-center gap-4 text-sm font-medium text-[#8b91a3]">
                                    {isCaseFieldVisible("assignee")
                                      ? assigneeID ? (
                                        <button
                                          type="button"
                                          className="inline-flex min-w-0 items-center gap-1.5 text-inherit hover:text-[#d1d5db]"
                                          onClick={(e) => {
                                            e.preventDefault();
                                            e.stopPropagation();
                                            setLocation(withTenantPath(currentTenantSlug, `/users/${assigneeID}`));
                                          }}
                                          data-testid={`button-case-owner-${c.id}`}
                                        >
                                          <UserAvatar
                                            name={ownerLabel}
                                            avatar={assigneeUser?.avatar}
                                            className="h-5 w-5 border border-[#2a2c3c]"
                                            fallbackClassName="bg-[#243459] text-[#d7e3ff] text-[9px] font-bold"
                                          />
                                          <EllipsisText text={ownerLabel} className="max-w-[200px]" />
                                        </button>
                                      ) : (
                                        <span className="flex items-center gap-1.5 text-[#ffc700]"><Users size={14} /> {t("cases.unassigned")}</span>
                                      )
                                      : null}
                                    {isCaseFieldVisible("created") ? (
                                      <span className="flex items-center gap-1.5"><Clock size={14} /> {Number.isFinite(createdAt.getTime()) ? format(createdAt, "MMM dd HH:mm") : "n/a"}</span>
                                    ) : null}
                                    {isCaseFieldVisible("work_time") ? (
                                      <span className="text-xs font-semibold text-[#8fb6ff]">{`Work: ${workHours.toFixed(1)}h`}</span>
                                    ) : null}
                                    {isCaseFieldVisible("discussion") && c.forumId ? (
                                      <button
                                        type="button"
                                        className="flex items-center gap-1.5 text-[#74d7a2] hover:text-[#9be6be]"
                                        onClick={(e) => {
                                          e.preventDefault();
                                          e.stopPropagation();
                                          setLocation(withTenantPath(currentTenantSlug, `/forum/${c.forumId}`));
                                        }}
                                        data-testid={`button-open-case-discussion-${c.id}`}
                                      >
                                        <MessageSquare size={14} /> {t("cases.activeDiscussion")}
                                      </button>
                                    ) : null}
                                  </div>
                                  {(isCaseFieldVisible("task_progress")
                                    || isCaseFieldVisible("responder_statuses")
                                    || isCaseFieldVisible("rule")
                                    || isCaseFieldVisible("source")
                                    || selectedCustomCaseFieldKeys.length > 0) ? (
                                      <div className={`${CASE_SUBPANEL_CLASS} mt-3 space-y-2 p-2.5`}>
                                        <div className="flex flex-wrap items-center gap-2 text-[11px] font-semibold text-[#8b91a3]">
                                          {isCaseFieldVisible("task_progress") ? <span>{`Open tasks: ${caseSummary.openTasks}`}</span> : null}
                                          {isCaseFieldVisible("task_progress") ? <span>{`Closed tasks: ${caseSummary.closedTasks}`}</span> : null}
                                          {isCaseFieldVisible("task_progress") && showTaskCountdown ? (
                                            <Badge
                                              variant="outline"
                                              className={`h-5 text-[10px] font-semibold ${taskDueBadgeClass(nextTaskDeltaMs)}`}
                                              data-testid={`case-next-task-due-${c.id}`}
                                            >
                                              {nextTaskDeltaMs < 0
                                                ? `Overdue ${formatTimeDeltaCompact(nextTaskDeltaMs)}`
                                                : `Due in ${formatTimeDeltaCompact(nextTaskDeltaMs)}`}
                                            </Badge>
                                          ) : null}
                                          {isCaseFieldVisible("custom_field_values_count") ? <span>{`Custom values: ${caseCustomFieldValueCount(c)}`}</span> : null}
                                          {isCaseFieldVisible("rule") ? <span>{`Rule: ${c.incidentType || "n/a"}`}</span> : null}
                                          {isCaseFieldVisible("source") ? <span>{`Source: ${c.source || "n/a"}`}</span> : null}
                                          {selectedCustomCaseFieldKeys.map((fieldKey) => {
                                            const value = c?.customFields?.[fieldKey];
                                            if (value === undefined || value === null || String(value).trim() === "") return null;
                                            return <span key={`${c.id}-${fieldKey}`}>{`${fieldKey}: ${String(value)}`}</span>;
                                          })}
                                        </div>
                                        {isCaseFieldVisible("task_progress") ? (
                                          <div className="h-2 w-full overflow-hidden rounded-full bg-[#1f2535]">
                                            <div
                                              className="h-full rounded-full bg-[#4adf37] transition-all"
                                              style={{ width: `${Math.max(0, Math.min(100, Number(caseSummary.taskProgressPercent || 0)))}%` }}
                                              data-testid={`case-task-progress-${c.id}`}
                                            />
                                          </div>
                                        ) : null}
                                        {isCaseFieldVisible("responder_statuses") ? (
                                          <div className="flex flex-wrap items-center gap-1.5">
                                            <Badge variant="outline" className="h-5 border-[#2a2c3c] bg-[#0f121b] text-[10px] text-[#9ca3af]">
                                              {`Q ${caseSummary.responderStatuses.queued || 0}`}
                                            </Badge>
                                            <Badge variant="outline" className="h-5 border-[#2a2c3c] bg-[#0f121b] text-[10px] text-[#9ca3af]">
                                              {`R ${caseSummary.responderStatuses.running || 0}`}
                                            </Badge>
                                            <Badge variant="outline" className="h-5 border-[#2a2c3c] bg-[#0f121b] text-[10px] text-[#9ca3af]">
                                              {`OK ${caseSummary.responderStatuses.completed || 0}`}
                                            </Badge>
                                            <Badge variant="outline" className="h-5 border-[#6a2f39] bg-[#341b22] text-[10px] text-[#ff9fb3]">
                                              {`ERR ${caseSummary.responderStatuses.failed || 0}`}
                                            </Badge>
                                          </div>
                                        ) : null}
                                      </div>
                                    ) : null}
                                </div>
                              </div>
                              <div className="flex items-center gap-3 pr-1 md:pt-1">
                                {!assigneeID ? (
                                  <Button
                                    variant="secondary"
                                    className="h-9 rounded-xl bg-[#4adf37] px-4 font-semibold text-[#0b0c10] hover:bg-[#61f44f]"
                                    onClick={(e) => handleAssign(e, c.id)}
                                  >
                                    {t("cases.assignToMe")}
                                  </Button>
                                ) : null}

                                <Badge className={`${c.sev === "Critical" ? "bg-[#eb5f65]" : c.sev === "High" ? "bg-[#ffc700] text-[#0b0c10]" : c.sev === "Medium" ? "bg-[#3b82f6]" : "bg-[#66ff4c] text-[#0b0c10]"} min-w-[94px] justify-center font-semibold`}>{c.sev}</Badge>
                              </div>
                            </div>
                          </Card>
                        </Link>
                      </div>
                    );
                  })}
                </div>
              </section>
            ))
          )}
        </div>

        {renderPaginationControls("bottom")}
      </div>

      <AlertDialog
        open={deleteCaseDialogOpen}
        onOpenChange={(open) => {
          setDeleteCaseDialogOpen(open);
          if (!open) {
            setDeleteCaseIDs([]);
          }
        }}
      >
      <AlertDialogContent className="rounded-xl border border-[#2a2c3c] bg-[#13141c] text-white">
          <AlertDialogHeader>
            <AlertDialogTitle>{t("common.delete")}</AlertDialogTitle>
            <AlertDialogDescription className="text-[#6b7280]">
              {deleteCaseIDs.length > 1
                ? t("cases.bulk.deleteConfirm", { count: deleteCaseIDs.length.toString() })
                : t("cases.deleteConfirmOne")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel className="border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]">{t("common.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              className="border border-[#6a2f39] bg-[#341b22] text-[#ff9fb3] hover:bg-[#44232c]"
              onClick={handleConfirmDeleteCases}
              disabled={deleteCase.isPending || deleteCasesBulk.isPending}
            >
              {t("common.delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </AppLayout>
  );
}
