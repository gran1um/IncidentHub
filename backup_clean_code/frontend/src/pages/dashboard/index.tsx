import { useEffect, useMemo, useState } from "react";
import { Link } from "wouter";
import {
  Bell,
  BriefcaseBusiness,
  CalendarDays,
  ChevronLeft,
  ChevronRight,
  ChartNoAxesColumn,
  Clock3,
  Clock,
  Moon,
  Pause,
  Play,
  Plus,
  Search,
  ShieldCheck,
  TrendingDown,
  TrendingUp,
  X,
} from "lucide-react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip as ChartTooltip,
  XAxis,
  YAxis,
} from "recharts";
import { AppLayout } from "@/components/layout";
import { UserAvatar } from "@/components/user-avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { EllipsisText } from "@/components/ui/ellipsis-text";
import {
  useActivityLivestream,
  useAppState,
  useCreateDashboardCustomMetric,
  useCreateShift,
  useDashboardMetrics,
  useDashboardCustomMetrics,
  useDeleteDashboardCustomMetric,
  useDeleteShift,
  useDashboardStats,
  useDutyOverview,
  useShifts,
  useUpdateDashboardCustomMetric,
  useUsers,
  useCases,
} from "@/lib/api";
import { buildResolutionBySeverity, formatAvgResponseMinutes } from "@/lib/dashboard";
import { useI18n, useT } from "@/lib/i18n";
import { withTenantPath } from "@/lib/tenant-url";
import { useMinimumLoading } from "@/lib/use-minimum-loading";
import { toast } from "sonner";

type TrendDirection = "up" | "down";

type StatCard = {
  id: string;
  label: string;
  value: string;
  trendValue: string;
  trendLabel: string;
  trendDirection: TrendDirection;
  icon: any;
  iconClassName: string;
};

type StreamEvent = {
  id: string;
  title: string;
  description: string;
  severityLabel: string;
  sourceLabel: string;
  severityClassName: string;
  timeLabel: string;
};

type AnalystItem = {
  id: string;
  name: string;
  role: string;
  avatar: string;
  resolvedCases: number;
};

type ShiftCalendarEntry = {
  id: string;
  analystId: string;
  day: number;
  month: number;
  year: number;
  start: string;
  end: string;
};

type ShiftCalendarAssignment = ShiftCalendarEntry & {
  isOvernight: boolean;
  isActive: boolean;
};

type DutyAnalystCandidate = AnalystItem;

const CARD_CLASS =
  "rounded-xl border border-[rgba(255,255,255,0.05)] bg-[rgba(19,20,28,0.95)] shadow-[0_4px_20px_rgba(0,0,0,0.3)]";

function DashboardLoadingSkeleton() {
  return (
    <AppLayout>
      <div className="mx-auto w-full max-w-[1120px] space-y-6">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {Array.from({ length: 4 }).map((_, index) => (
            <Card key={`dashboard-kpi-${index}`} className={`${CARD_CLASS} h-[150px] overflow-hidden p-4`}>
              <div className="flex h-full min-h-0 flex-col gap-3.5">
                <div className="h-[48px] rounded-lg border border-[#2a2c3c] bg-[#0f131d] px-3 py-2.5">
                  <div className="flex items-center gap-3">
                    <Skeleton className="h-7 w-7 rounded-lg" />
                    <Skeleton className="h-3.5 w-24 rounded-md" />
                  </div>
                </div>
                <div className="flex h-[56px] min-h-0 flex-col justify-center rounded-lg border border-[#2a2c3c] bg-[#0f131d] px-3 py-2.5">
                  <div className="space-y-2">
                    <Skeleton className="h-4 w-20 rounded-md" />
                    <Skeleton className="h-2.5 w-28 rounded-md" />
                  </div>
                </div>
              </div>
            </Card>
          ))}
        </div>

        <div className="grid grid-cols-1 gap-4 xl:grid-cols-[357px_1fr]">
          <Card className={`${CARD_CLASS} h-[268px] overflow-hidden p-6`}>
            <div className="mb-5 flex items-center justify-between">
              <Skeleton className="h-8 w-44 rounded-md" />
              <Skeleton className="h-7 w-20 rounded-md" />
            </div>
            <div className="space-y-4">
              <div className="rounded-lg border border-[#2a2c3c] bg-[#0f131d] p-3">
                <div className="flex items-center justify-between">
                  <div className="space-y-2">
                    <Skeleton className="h-3 w-24 rounded-md" />
                    <div className="flex -space-x-2">
                      <Skeleton className="h-8 w-8 rounded-full border border-black/70 p-[2px] dark:border-white/80" />
                      <Skeleton className="h-8 w-8 rounded-full border border-black/70 p-[2px] dark:border-white/80" />
                      <Skeleton className="h-8 w-8 rounded-full border border-black/70 p-[2px] dark:border-white/80" />
                    </div>
                  </div>
                  <Skeleton className="h-4 w-20 rounded-md" />
                </div>
              </div>
              <div className="h-px bg-[#2a2c3c]" />
              <div className="rounded-lg border border-[#2a2c3c] bg-[#0f131d] p-3">
                <div className="flex items-center justify-between">
                  <div className="space-y-2">
                    <Skeleton className="h-3 w-20 rounded-md" />
                    <div className="flex -space-x-2">
                      <Skeleton className="h-8 w-8 rounded-full border border-black/70 p-[2px] dark:border-white/80" />
                      <Skeleton className="h-8 w-8 rounded-full border border-black/70 p-[2px] dark:border-white/80" />
                    </div>
                  </div>
                  <Skeleton className="h-4 w-20 rounded-md" />
                </div>
              </div>
            </div>
          </Card>

          <Card className={`${CARD_CLASS} h-[268px] overflow-hidden p-6`}>
            <div className="mb-5 flex items-center justify-between">
              <Skeleton className="h-8 w-52 rounded-md" />
              <Skeleton className="h-6 w-20 rounded-md" />
            </div>
            <div className="grid grid-cols-1 gap-3 lg:grid-cols-3">
              {Array.from({ length: 3 }).map((_, metricIndex) => (
                <div key={`dashboard-metric-loading-${metricIndex}`} className="rounded-lg border border-[#2a2c3c] p-4">
                  <Skeleton className="h-3 w-20 rounded-md" />
                  <Skeleton className="mt-3 h-8 w-16 rounded-md" />
                  <Skeleton className="mt-3 h-2 w-full rounded-full" />
                </div>
              ))}
            </div>
          </Card>
        </div>

        <div className="grid grid-cols-1 gap-4 xl:grid-cols-[1.2fr_0.8fr]">
          <Card className={`${CARD_CLASS} h-[312px] overflow-hidden p-6`}>
            <div className="mb-5 flex items-center justify-between">
              <div className="space-y-2">
                <Skeleton className="h-7 w-48 rounded-md" />
                <Skeleton className="h-3 w-40 rounded-md" />
              </div>
              <Skeleton className="h-8 w-24 rounded-md" />
            </div>
            <div className="space-y-3">
              {Array.from({ length: 4 }).map((_, eventIndex) => (
                <div key={`dashboard-stream-loading-${eventIndex}`} className="rounded-lg border border-[#2a2c3c] p-3">
                  <Skeleton className="h-4 w-44 rounded-md" />
                  <Skeleton className="mt-2 h-3 w-64 rounded-md" />
                </div>
              ))}
            </div>
          </Card>

          <Card className={`${CARD_CLASS} h-[312px] overflow-hidden p-6`}>
            <div className="mb-5 flex items-center justify-between">
              <Skeleton className="h-7 w-40 rounded-md" />
              <Skeleton className="h-8 w-24 rounded-md" />
            </div>
            <Skeleton className="h-[180px] w-full rounded-xl" />
            <div className="mt-4 space-y-2">
              <Skeleton className="h-3 w-full rounded-md" />
              <Skeleton className="h-3 w-4/5 rounded-md" />
            </div>
          </Card>
        </div>
      </div>
    </AppLayout>
  );
}

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

function asNumber(value: unknown): number {
  const n = Number(value ?? 0);
  if (!Number.isFinite(n)) {
    return 0;
  }
  return n;
}

function normalizeSeverity(value: string): "critical" | "high" | "medium" | "low" {
  const normalized = String(value || "").trim().toLowerCase();
  if (normalized === "critical") return "critical";
  if (normalized === "high") return "high";
  if (normalized === "low") return "low";
  return "medium";
}

function formatShiftRange(startsAt: string, endsAt: string, locale: string): string {
  const startMs = Date.parse(String(startsAt || ""));
  if (!Number.isFinite(startMs)) {
    return "";
  }
  const start = new Date(startMs);
  const startLabel = start.toLocaleTimeString(locale, {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });

  const endMs = Date.parse(String(endsAt || ""));
  if (!Number.isFinite(endMs)) {
    return "";
  }
  const end = new Date(endMs);
  const endLabel = end.toLocaleTimeString(locale, {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });

  return `${startLabel} - ${endLabel}`;
}

function mapRoleLabel(role: string, locale: string): string {
  const normalized = String(role || "").trim().toLowerCase();
  const isRu = locale.startsWith("ru");
  if (normalized === "platform_admin") return isRu ? "Платформенный администратор" : "Platform Admin";
  if (normalized === "tenant_admin") return isRu ? "Администратор тенанта" : "Tenant Admin";
  if (normalized === "viewer") return isRu ? "Наблюдатель" : "Viewer";
  if (normalized === "analyst") return isRu ? "Аналитик" : "Analyst";
  if (normalized === "security analyst") return isRu ? "Аналитик ИБ" : "Security Analyst";
  return role || (isRu ? "Аналитик" : "Analyst");
}

function parseShiftEntry(item: any): ShiftCalendarEntry | null {
  const data = item?.data && typeof item.data === "object" ? item.data : item || {};
  const day = Number(data?.day ?? data?.day_of_month ?? 0);
  const month = Number(data?.month ?? 0);
  const year = Number(data?.year ?? 0);
  if (!Number.isFinite(day) || day <= 0) {
    return null;
  }
  const start = String(data?.start || data?.startsAt || data?.starts_at || "").trim() || "08:00";
  const end = String(data?.end || data?.endsAt || data?.ends_at || "").trim() || "16:00";
  return {
    id: String(item?.id || "").trim(),
    analystId: String(data?.analystId || data?.analyst_id || data?.user_id || data?.owner_id || "").trim(),
    day: Math.floor(day),
    month: Number.isFinite(month) ? Math.floor(month) : 0,
    year: Number.isFinite(year) ? Math.floor(year) : 0,
    start,
    end,
  };
}

function parseTimeToMinutes(value: string): number | null {
  const match = String(value || "").trim().match(/^(\d{1,2}):(\d{2})$/);
  if (!match) {
    return null;
  }
  const hours = Number(match[1]);
  const minutes = Number(match[2]);
  if (!Number.isFinite(hours) || !Number.isFinite(minutes)) {
    return null;
  }
  if (hours < 0 || hours > 23 || minutes < 0 || minutes > 59) {
    return null;
  }
  return hours * 60 + minutes;
}

function isOvernightShift(start: string, end: string): boolean {
  const startMinutes = parseTimeToMinutes(start);
  const endMinutes = parseTimeToMinutes(end);
  if (startMinutes === null || endMinutes === null) {
    return false;
  }
  return endMinutes < startMinutes;
}

function resolveShiftWindow(day: number, month: number, year: number, start: string, end: string): { startAt: number; endAt: number } | null {
  const startMinutes = parseTimeToMinutes(start);
  const endMinutes = parseTimeToMinutes(end);
  if (startMinutes === null || endMinutes === null) {
    return null;
  }
  const startAt = new Date(year, month - 1, day, Math.floor(startMinutes / 60), startMinutes % 60);
  const endDay = isOvernightShift(start, end) ? day + 1 : day;
  const endAt = new Date(year, month - 1, endDay, Math.floor(endMinutes / 60), endMinutes % 60);
  if (!Number.isFinite(startAt.getTime()) || !Number.isFinite(endAt.getTime())) {
    return null;
  }
  return {
    startAt: startAt.getTime(),
    endAt: endAt.getTime(),
  };
}

function resolveNeighborMonth(month: number, year: number, direction: -1 | 1): { month: number; year: number } {
  if (direction === -1) {
    if (month === 1) {
      return { month: 12, year: year - 1 };
    }
    return { month: month - 1, year };
  }
  if (month === 12) {
    return { month: 1, year: year + 1 };
  }
  return { month: month + 1, year };
}

function scheduleRowClassName(isActive: boolean): string {
  if (isActive) {
    return "flex items-center justify-between rounded-md border border-[rgba(102,255,76,0.3)] bg-[rgba(102,255,76,0.12)] p-2";
  }
  return "flex items-center justify-between rounded-md border border-[rgba(59,130,246,0.3)] bg-[rgba(59,130,246,0.12)] p-2";
}

function scheduleNameClassName(isActive: boolean): string {
  return isActive ? "max-w-[150px] text-xs font-medium text-[#d9ffd1]" : "max-w-[150px] text-xs font-medium text-[#dbeafe]";
}

function scheduleMetaClassName(isActive: boolean): string {
  return isActive ? "mt-1 flex items-center gap-1 text-[11px] text-[#86ff76]" : "mt-1 flex items-center gap-1 text-[11px] text-[#93c5fd]";
}

function scheduleMoonIconClassName(isActive: boolean): string {
  return isActive ? "text-[#66ff4c]" : "text-[#93c5fd]";
}

function schedulePreviewFillClassName(isActive: boolean): string {
  if (isActive) {
    return "border border-[rgba(102,255,76,0.38)] bg-[rgba(102,255,76,0.24)] text-[#86ff76]";
  }
  return "border border-[rgba(59,130,246,0.36)] bg-[rgba(59,130,246,0.22)] text-[#93c5fd]";
}

function dutyShiftTimeRangeLabel(shift: any, locale: string): string {
  const byTimestamp = formatShiftRange(String(shift?.startsAt || ""), String(shift?.endsAt || ""), locale);
  if (byTimestamp) {
    return byTimestamp;
  }
  const rawStart = String(shift?.start || shift?.starts_at_time || shift?.startsAtTime || shift?.data?.start || "").trim();
  const rawEnd = String(shift?.end || shift?.ends_at_time || shift?.endsAtTime || shift?.data?.end || "").trim();
  if (rawStart && rawEnd) {
    return `${rawStart} - ${rawEnd}`;
  }
  return "";
}

function buildDutyAnalystCandidates(source: any[], locale: string): DutyAnalystCandidate[] {
  return source
    .map((item: any) => ({
      id: String(item?.id || "").trim(),
      name: String(item?.name || item?.username || item?.email || item?.id || "Unknown"),
      avatar: String(item?.avatar || item?.avatar_url || "").trim(),
      role: mapRoleLabel(String(item?.role || "analyst"), locale),
      resolvedCases: 0,
    }))
    .filter((item) => item.id);
}

function resolveShiftPreviewSegment(shift: ShiftCalendarAssignment, isContinuation: boolean): { leftPercent: number; widthPercent: number } {
  const continuationEnd = parseTimeToMinutes(shift.end);
  const dayStart = isContinuation ? 0 : parseTimeToMinutes(shift.start);
  const dayEnd = isContinuation ? continuationEnd : (shift.isOvernight ? 24 * 60 : parseTimeToMinutes(shift.end));

  const startMinutes = Math.max(0, Math.min(24 * 60, dayStart ?? 0));
  const resolvedEnd = Math.max(startMinutes + 1, Math.min(24 * 60, dayEnd ?? 24 * 60));
  const widthRaw = ((resolvedEnd - startMinutes) / (24 * 60)) * 100;
  const leftPercent = (startMinutes / (24 * 60)) * 100;
  const widthPercent = Math.min(Math.max(widthRaw, 7), 100 - leftPercent);

  return { leftPercent, widthPercent };
}

function formatRelativeShort(value: string, locale: string): string {
  const parsed = Date.parse(String(value || ""));
  if (!Number.isFinite(parsed)) {
    return locale.startsWith("ru") ? "сейчас" : "now";
  }
  const diffSeconds = Math.max(1, Math.floor((Date.now() - parsed) / 1000));
  const isRu = locale.startsWith("ru");

  if (diffSeconds < 60) {
    return isRu ? `${diffSeconds}с назад` : `${diffSeconds}s ago`;
  }
  const diffMinutes = Math.floor(diffSeconds / 60);
  if (diffMinutes < 60) {
    return isRu ? `${diffMinutes}м назад` : `${diffMinutes}m ago`;
  }
  const diffHours = Math.floor(diffMinutes / 60);
  if (diffHours < 24) {
    return isRu ? `${diffHours}ч назад` : `${diffHours}h ago`;
  }
  const diffDays = Math.floor(diffHours / 24);
  return isRu ? `${diffDays}д назад` : `${diffDays}d ago`;
}

function severityClassName(severity: "critical" | "high" | "medium" | "low"): string {
  if (severity === "critical") return "bg-[#eb5f65]";
  if (severity === "high") return "bg-[#ffc700]";
  if (severity === "low") return "bg-[#3b82f6]";
  return "bg-[#66ff4c]";
}

function severityText(severity: "critical" | "high" | "medium" | "low", locale: string): string {
  if (locale.startsWith("ru")) {
    if (severity === "critical") return "Критический";
    if (severity === "high") return "Высокий";
    if (severity === "low") return "Низкий";
    return "Средний";
  }
  if (severity === "critical") return "Critical";
  if (severity === "high") return "High";
  if (severity === "low") return "Low";
  return "Medium";
}

const FALLBACK_STREAM_EVENTS_EN: StreamEvent[] = [
  {
    id: "fallback-1",
    title: "Critical: Brute Force Attack Detected",
    description: "Multiple failed login attempts from IP 192.168.1.105",
    severityLabel: "Critical",
    sourceLabel: "SIEM-2847",
    severityClassName: "bg-[#eb5f65]",
    timeLabel: "2s ago",
  },
  {
    id: "fallback-2",
    title: "High: Suspicious Outbound Traffic",
    description: "Unusual data transfer to external domain detected",
    severityLabel: "High",
    sourceLabel: "FW-1923",
    severityClassName: "bg-[#ffc700]",
    timeLabel: "45s ago",
  },
  {
    id: "fallback-3",
    title: "Case Resolved: Phishing Investigation",
    description: "Email threat neutralized, user credentials secured",
    severityLabel: "Resolved",
    sourceLabel: "CASE-5612",
    severityClassName: "bg-[#66ff4c]",
    timeLabel: "2m ago",
  },
  {
    id: "fallback-4",
    title: "Medium: Policy Violation Alert",
    description: "Unauthorized software installation attempt blocked",
    severityLabel: "Medium",
    sourceLabel: "EDR-8834",
    severityClassName: "bg-[#3b82f6]",
    timeLabel: "5m ago",
  },
];

const FALLBACK_STREAM_EVENTS_RU: StreamEvent[] = [
  {
    id: "fallback-1-ru",
    title: "Критический: Обнаружен перебор паролей",
    description: "Множественные неудачные входы с IP 192.168.1.105",
    severityLabel: "Критический",
    sourceLabel: "SIEM-2847",
    severityClassName: "bg-[#eb5f65]",
    timeLabel: "2с назад",
  },
  {
    id: "fallback-2-ru",
    title: "Высокий: Подозрительный исходящий трафик",
    description: "Зафиксирована нетипичная передача данных во внешний домен",
    severityLabel: "Высокий",
    sourceLabel: "FW-1923",
    severityClassName: "bg-[#ffc700]",
    timeLabel: "45с назад",
  },
  {
    id: "fallback-3-ru",
    title: "Кейс решен: Расследование фишинга",
    description: "Угроза по email нейтрализована, учетные данные защищены",
    severityLabel: "Решено",
    sourceLabel: "CASE-5612",
    severityClassName: "bg-[#66ff4c]",
    timeLabel: "2м назад",
  },
  {
    id: "fallback-4-ru",
    title: "Средний: Нарушение политики",
    description: "Заблокирована попытка установки несанкционированного ПО",
    severityLabel: "Средний",
    sourceLabel: "EDR-8834",
    severityClassName: "bg-[#3b82f6]",
    timeLabel: "5м назад",
  },
];

export default function Dashboard() {
  const t = useT();
  const { language } = useI18n();
  const locale = language === "ru" ? "ru-RU" : "en-US";
  const isRu = language === "ru";
  const [chartPeriod, setChartPeriod] = useState<"week" | "month">("week");
  const [streamSearch, setStreamSearch] = useState("");
  const [streamPaused, setStreamPaused] = useState(false);
  const [scheduleOpen, setScheduleOpen] = useState(false);
  const [metricsBuilderOpen, setMetricsBuilderOpen] = useState(false);
  const [editingMetricID, setEditingMetricID] = useState("");
  const [metricName, setMetricName] = useState("");
  const [metricDescription, setMetricDescription] = useState("");
  const [metricSource, setMetricSource] = useState<"cases" | "alerts">("cases");
  const [metricMeasure, setMetricMeasure] = useState<"count" | "avg_resolution_minutes" | "overdue_count">("count");
  const [metricStatuses, setMetricStatuses] = useState("");
  const [metricSeverities, setMetricSeverities] = useState("");
  const [metricCategories, setMetricCategories] = useState("");
  const [metricCreatedWithinHours, setMetricCreatedWithinHours] = useState("0");
  const [metricOverdueMinutes, setMetricOverdueMinutes] = useState("1440");
  const [scheduleMonth, setScheduleMonth] = useState(() => new Date().getMonth() + 1);
  const [scheduleYear, setScheduleYear] = useState(() => new Date().getFullYear());
  const [newShift, setNewShift] = useState({ analystId: "", start: "08:00", end: "16:00" });

  const { currentTenantId, currentTenantSlug } = useAppState();
  const streamPausedStorageKey = useMemo(
    () => `incidenthub-dashboard-stream-paused:${String(currentTenantId || "").trim() || "none"}`,
    [currentTenantId],
  );
  const { data: stats, isLoading: statsLoading } = useDashboardStats(currentTenantId);
  const { data: dashboardMetrics, isLoading: dashboardMetricsLoading } = useDashboardMetrics(currentTenantId);
  const { data: customMetrics = [] } = useDashboardCustomMetrics(currentTenantId);
  const { data: users = [], isLoading: usersLoading } = useUsers(currentTenantId);
  const { data: dutyOverview, isLoading: dutyOverviewLoading } = useDutyOverview(currentTenantId);
  const { data: cases = [], isLoading: casesLoading } = useCases(currentTenantId);
  const { data: scheduleShifts = [] } = useShifts(currentTenantId, scheduleMonth, scheduleYear);
  const previousScheduleMonth = useMemo(() => resolveNeighborMonth(scheduleMonth, scheduleYear, -1), [scheduleMonth, scheduleYear]);
  const nextScheduleMonth = useMemo(() => resolveNeighborMonth(scheduleMonth, scheduleYear, 1), [scheduleMonth, scheduleYear]);
  const { data: previousScheduleShifts = [] } = useShifts(currentTenantId, previousScheduleMonth.month, previousScheduleMonth.year);
  useShifts(currentTenantId, nextScheduleMonth.month, nextScheduleMonth.year);
  const createShift = useCreateShift();
  const deleteShift = useDeleteShift();
  const createDashboardCustomMetric = useCreateDashboardCustomMetric();
  const updateDashboardCustomMetric = useUpdateDashboardCustomMetric();
  const deleteDashboardCustomMetric = useDeleteDashboardCustomMetric();
  const { data: livestreamData, isLoading: streamLoading } = useActivityLivestream(currentTenantId, {
    q: streamSearch,
    limit: 4,
    enabled: Boolean(currentTenantId) && !streamPaused,
    refetchOnWindowFocus: !streamPaused,
    refetchInterval: streamPaused ? 0 : 3000,
  });
  const isDashboardInitialLoading = Boolean(
    statsLoading ||
      dashboardMetricsLoading ||
      usersLoading ||
      dutyOverviewLoading ||
      casesLoading,
  );
  const showDashboardLoadingSkeleton = useMinimumLoading(isDashboardInitialLoading);

  const statCardData = useMemo<StatCard[]>(() => {
    const activeCases = asNumber(stats?.activeCases);
    const alerts24h = asNumber(stats?.alertsCount);
    const resolvedToday = asNumber(stats?.resolvedToday);
    const avgResponseMinutes = asNumber(stats?.avgResponseMin);

    const labels = {
      vsLastWeek: isRu ? "к прошлой неделе" : "vs last week",
      vsYesterday: isRu ? "ко вчера" : "vs yesterday",
      efficiencyGain: isRu ? "прирост эффективности" : "efficiency gain",
      faster: isRu ? "быстрее" : "faster",
    };

    return [
      {
        id: "active-cases",
        label: t("dashboard.activeCases"),
        value: String(activeCases),
        trendValue: "12%",
        trendLabel: labels.vsLastWeek,
        trendDirection: "down",
        icon: BriefcaseBusiness,
        iconClassName: "bg-[rgba(235,95,101,0.2)] text-[#eb5f65]",
      },
      {
        id: "alerts-24h",
        label: t("dashboard.alerts24h"),
        value: String(alerts24h),
        trendValue: "8%",
        trendLabel: labels.vsYesterday,
        trendDirection: "up",
        icon: Bell,
        iconClassName: "bg-[rgba(255,199,0,0.2)] text-[#ffc700]",
      },
      {
        id: "resolved-today",
        label: t("dashboard.resolvedToday"),
        value: String(resolvedToday),
        trendValue: "15%",
        trendLabel: labels.efficiencyGain,
        trendDirection: "up",
        icon: ShieldCheck,
        iconClassName: "bg-[rgba(102,255,76,0.2)] text-[#66ff4c]",
      },
      {
        id: "avg-response",
        label: t("dashboard.avgResponse"),
        value: formatAvgResponseMinutes(avgResponseMinutes),
        trendValue: "22%",
        trendLabel: labels.faster,
        trendDirection: "up",
        icon: Clock3,
        iconClassName: "bg-[rgba(59,130,246,0.2)] text-[#3b82f6]",
      },
    ];
  }, [isRu, stats?.activeCases, stats?.alertsCount, stats?.avgResponseMin, stats?.resolvedToday, t]);

  const analystsList = useMemo(
    () =>
      users.map((item: any) => ({
        id: String(item?.id || "").trim(),
        name: String(item?.name || item?.username || item?.email || "Unknown"),
      })),
    [users],
  );
  const analystNameById = useMemo(() => new Map(analystsList.map((item) => [item.id, item.name])), [analystsList]);

  const effectiveAnalystId = newShift.analystId || (analystsList[0]?.id || "");
  const normalizedScheduleShifts = useMemo<ShiftCalendarAssignment[]>(() => {
    const now = Date.now();
    return scheduleShifts
      .map((item: any) => parseShiftEntry(item))
      .filter((item: ShiftCalendarEntry | null): item is ShiftCalendarEntry => Boolean(item))
      .map((shift) => {
        const month = shift.month > 0 ? shift.month : scheduleMonth;
        const year = shift.year > 0 ? shift.year : scheduleYear;
        const isOvernight = isOvernightShift(shift.start, shift.end);
        const window = resolveShiftWindow(shift.day, month, year, shift.start, shift.end);
        const isActive = Boolean(window && now >= window.startAt && now < window.endAt);
        return {
          ...shift,
          month,
          year,
          isOvernight,
          isActive,
        };
      });
  }, [scheduleMonth, scheduleShifts, scheduleYear]);

  const normalizedPreviousScheduleShifts = useMemo<ShiftCalendarAssignment[]>(() => {
    const now = Date.now();
    return previousScheduleShifts
      .map((item: any) => parseShiftEntry(item))
      .filter((item: ShiftCalendarEntry | null): item is ShiftCalendarEntry => Boolean(item))
      .map((shift) => {
        const month = shift.month > 0 ? shift.month : previousScheduleMonth.month;
        const year = shift.year > 0 ? shift.year : previousScheduleMonth.year;
        const isOvernight = isOvernightShift(shift.start, shift.end);
        const window = resolveShiftWindow(shift.day, month, year, shift.start, shift.end);
        const isActive = Boolean(window && now >= window.startAt && now < window.endAt);
        return {
          ...shift,
          month,
          year,
          isOvernight,
          isActive,
        };
      });
  }, [previousScheduleMonth.month, previousScheduleMonth.year, previousScheduleShifts]);

  const currentAssignments = useMemo(() => {
    const grouped: Record<number, ShiftCalendarAssignment[]> = {};
    normalizedScheduleShifts.forEach((shift) => {
      if (!grouped[shift.day]) {
        grouped[shift.day] = [];
      }
      grouped[shift.day].push(shift);
    });
    return grouped;
  }, [normalizedScheduleShifts]);

  const previousAssignments = useMemo(() => {
    const grouped: Record<number, ShiftCalendarAssignment[]> = {};
    normalizedPreviousScheduleShifts.forEach((shift) => {
      if (!grouped[shift.day]) {
        grouped[shift.day] = [];
      }
      grouped[shift.day].push(shift);
    });
    return grouped;
  }, [normalizedPreviousScheduleShifts]);

  const weekdayLabels = useMemo(
    () => (isRu ? ["Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"] : ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]),
    [isRu],
  );

  const firstWeekdayIndex = useMemo(() => {
    const firstDay = new Date(scheduleYear, scheduleMonth - 1, 1).getDay(); // 0=Sun..6=Sat
    return (firstDay + 6) % 7; // 0=Mon..6=Sun for our Monday-first calendar
  }, [scheduleMonth, scheduleYear]);

  const days = useMemo(() => {
    const total = new Date(scheduleYear, scheduleMonth, 0).getDate();
    return Array.from({ length: total }, (_, index) => index + 1);
  }, [scheduleMonth, scheduleYear]);

  const previousMonthDaysCount = useMemo(
    () => new Date(previousScheduleMonth.year, previousScheduleMonth.month, 0).getDate(),
    [previousScheduleMonth.month, previousScheduleMonth.year],
  );

  const scheduleMonthLabel = useMemo(
    () =>
      new Date(scheduleYear, scheduleMonth - 1, 1).toLocaleDateString(locale, {
        month: "long",
        year: "numeric",
      }),
    [locale, scheduleMonth, scheduleYear],
  );

  const handlePrevMonth = () => {
    const previousMonth = resolveNeighborMonth(scheduleMonth, scheduleYear, -1);
    setScheduleMonth(previousMonth.month);
    setScheduleYear(previousMonth.year);
  };

  const handleNextMonth = () => {
    const followingMonth = resolveNeighborMonth(scheduleMonth, scheduleYear, 1);
    setScheduleMonth(followingMonth.month);
    setScheduleYear(followingMonth.year);
  };

  const resolveAnalystDisplayName = (shift: ShiftCalendarEntry) =>
    analystNameById.get(shift.analystId) || shift.analystId || (isRu ? "Не назначен" : "Unassigned");

  const addShiftHandler = (day: number) => {
    if (!currentTenantId) {
      toast.error(isRu ? "Тенант не выбран" : "Tenant is not selected");
      return;
    }
    const dayShifts = currentAssignments[day] || [];
    if (dayShifts.length >= 2) {
      toast.error(t("dashboard.maxShiftsPerDay"));
      return;
    }
    if (!effectiveAnalystId) {
      toast.error(isRu ? "Нет аналитиков для смены" : "No analysts available");
      return;
    }
    createShift.mutate(
      {
        analystId: effectiveAnalystId,
        day,
        month: scheduleMonth,
        year: scheduleYear,
        start: newShift.start,
        end: newShift.end,
        tenantId: currentTenantId,
      },
      {
        onSuccess: () => {
          toast.success(t("dashboard.shiftAdded"));
        },
        onError: (error: any) => {
          toast.error(error?.message || (isRu ? "Не удалось добавить смену" : "Failed to add shift"));
        },
      },
    );
  };

  const removeShift = (shiftId: string) => {
    deleteShift.mutate(shiftId, {
      onError: (error: any) => toast.error(error?.message || (isRu ? "Не удалось удалить смену" : "Failed to delete shift")),
    });
  };

  const parseMetricList = (raw: string): string[] =>
    raw
      .split(",")
      .map((entry) => entry.trim().toLowerCase())
      .filter((entry, index, source) => entry.length > 0 && source.indexOf(entry) === index);

  const resetMetricForm = () => {
    setEditingMetricID("");
    setMetricName("");
    setMetricDescription("");
    setMetricSource("cases");
    setMetricMeasure("count");
    setMetricStatuses("");
    setMetricSeverities("");
    setMetricCategories("");
    setMetricCreatedWithinHours("0");
    setMetricOverdueMinutes("1440");
  };

  const editMetric = (metric: any) => {
    setEditingMetricID(String(metric?.id || ""));
    setMetricName(String(metric?.name || ""));
    setMetricDescription(String(metric?.description || ""));
    setMetricSource(metric?.source === "alerts" ? "alerts" : "cases");
    const nextMeasure = String(metric?.measure || "count").toLowerCase();
    if (nextMeasure === "avg_resolution_minutes" || nextMeasure === "overdue_count") {
      setMetricMeasure(nextMeasure as "avg_resolution_minutes" | "overdue_count");
    } else {
      setMetricMeasure("count");
    }
    setMetricStatuses((metric?.filters?.statuses || []).join(", "));
    setMetricSeverities((metric?.filters?.severities || []).join(", "));
    setMetricCategories((metric?.filters?.categories || []).join(", "));
    setMetricCreatedWithinHours(String(Number(metric?.filters?.createdWithinHours ?? 0)));
    setMetricOverdueMinutes(String(Number(metric?.filters?.overdueMinutes ?? 1440)));
    setMetricsBuilderOpen(true);
  };

  const saveMetric = () => {
    const name = metricName.trim();
    if (!name) {
      toast.error(isRu ? "Введите название метрики" : "Metric name is required");
      return;
    }
    const source = metricSource;
    const measure = source === "alerts" && metricMeasure !== "count" ? "count" : metricMeasure;
    const payload = {
      name,
      description: metricDescription.trim(),
      source,
      measure,
      enabled: true,
      filters: {
        statuses: parseMetricList(metricStatuses),
        severities: parseMetricList(metricSeverities),
        categories: parseMetricList(metricCategories),
        createdWithinHours: Math.max(0, Number(metricCreatedWithinHours || 0)),
        overdueMinutes: Math.max(0, Number(metricOverdueMinutes || 0)),
      },
    };
    const onSuccess = () => {
      toast.success(isRu ? "Метрика сохранена" : "Metric saved");
      setMetricsBuilderOpen(false);
      resetMetricForm();
    };
    const onError = (error: any) => toast.error(error?.message || (isRu ? "Не удалось сохранить метрику" : "Failed to save metric"));
    if (editingMetricID) {
      updateDashboardCustomMetric.mutate({ id: editingMetricID, data: payload }, { onSuccess, onError });
      return;
    }
    createDashboardCustomMetric.mutate(payload, { onSuccess, onError });
  };

  const toggleMetricEnabled = (metric: any) => {
    updateDashboardCustomMetric.mutate(
      {
        id: metric.id,
        data: {
          name: metric.name,
          description: metric.description || "",
          source: metric.source || "cases",
          measure: metric.measure || "count",
          enabled: !metric.enabled,
          filters: metric.filters || {},
        },
      },
      {
        onError: (error: any) => toast.error(error?.message || (isRu ? "Не удалось обновить метрику" : "Failed to update metric")),
      },
    );
  };

  const removeMetric = (metric: any) => {
    deleteDashboardCustomMetric.mutate(metric.id, {
      onSuccess: () => toast.success(isRu ? "Метрика удалена" : "Metric deleted"),
      onError: (error: any) => toast.error(error?.message || (isRu ? "Не удалось удалить метрику" : "Failed to delete metric")),
    });
  };

  const toggleStreamPause = () => {
    setStreamPaused((prev) => {
      const next = !prev;
      if (typeof window !== "undefined") {
        window.localStorage.setItem(streamPausedStorageKey, next ? "1" : "0");
      }
      return next;
    });
  };

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    const value = window.localStorage.getItem(streamPausedStorageKey);
    setStreamPaused(value === "1");
  }, [streamPausedStorageKey]);

  const onDutyAnalysts = useMemo<AnalystItem[]>(() => {
    const usersByID = new Map<string, any>();
    users.forEach((user: any) => usersByID.set(String(user?.id || "").trim(), user));

    const currentAnalysts = buildDutyAnalystCandidates(
      Array.isArray(dutyOverview?.current?.analysts) ? dutyOverview.current.analysts : [],
      locale,
    );
    const fallbackOnDuty = buildDutyAnalystCandidates(Array.isArray(dutyOverview?.onDuty) ? dutyOverview.onDuty : [], locale);
    const currentShiftAnalysts = buildDutyAnalystCandidates(
      (Array.isArray(dutyOverview?.current?.shifts) ? dutyOverview.current.shifts : []).map((shift: any) => {
        if (shift?.analyst) {
          return shift.analyst;
        }
        const analystID = String(shift?.analystId || shift?.analyst_id || "").trim();
        if (!analystID) {
          return null;
        }
        const user = usersByID.get(analystID);
        return user
          ? { ...user, role: user?.role || "analyst", avatar: user?.avatar || user?.avatar_url || "" }
          : { id: analystID, name: analystID, role: "analyst", avatar: "" };
      }),
      locale,
    );

    const preferred = currentAnalysts.length > 0 ? currentAnalysts : (fallbackOnDuty.length > 0 ? fallbackOnDuty : currentShiftAnalysts);
    const deduped = new Map<string, DutyAnalystCandidate>();
    preferred.forEach((analyst) => {
      if (!deduped.has(analyst.id)) {
        deduped.set(analyst.id, analyst);
      }
    });
    return Array.from(deduped.values()).slice(0, 4);
  }, [dutyOverview?.current?.analysts, dutyOverview?.current?.shifts, dutyOverview?.onDuty, locale, users]);

  const nextShiftAnalysts = useMemo<AnalystItem[]>(() => {
    const usersByID = new Map<string, any>();
    users.forEach((user: any) => usersByID.set(String(user?.id || "").trim(), user));

    const primary = buildDutyAnalystCandidates(Array.isArray(dutyOverview?.next?.analysts) ? dutyOverview.next.analysts : [], locale);
    if (primary.length > 0) {
      return primary.slice(0, 4);
    }
    const fromNextShifts = buildDutyAnalystCandidates(
      (Array.isArray(dutyOverview?.next?.shifts) ? dutyOverview.next.shifts : []).map((shift: any) => {
        if (shift?.analyst) {
          return shift.analyst;
        }
        const analystID = String(shift?.analystId || shift?.analyst_id || "").trim();
        if (!analystID) {
          return null;
        }
        const user = usersByID.get(analystID);
        return user
          ? { ...user, role: user?.role || "analyst", avatar: user?.avatar || user?.avatar_url || "" }
          : { id: analystID, name: analystID, role: "analyst", avatar: "" };
      }),
      locale,
    );
    return fromNextShifts.slice(0, 4);
  }, [dutyOverview?.next?.analysts, dutyOverview?.next?.shifts, locale, users]);

  const currentShiftRange = useMemo(() => {
    const firstShift = Array.isArray(dutyOverview?.current?.shifts) ? dutyOverview.current.shifts[0] : null;
    if (!firstShift) {
      return "";
    }
    return dutyShiftTimeRangeLabel(firstShift, locale);
  }, [dutyOverview?.current?.shifts, locale]);

  const nextShiftRange = useMemo(() => {
    const firstShift = Array.isArray(dutyOverview?.next?.shifts) ? dutyOverview.next.shifts[0] : null;
    if (!firstShift) {
      return "";
    }
    return dutyShiftTimeRangeLabel(firstShift, locale);
  }, [dutyOverview?.next?.shifts, locale]);

  const nextShiftPoint = useMemo(() => {
    const raw = String(dutyOverview?.next?.startsAt || "").trim();
    if (!raw) {
      return "";
    }
    const parsed = Date.parse(raw);
    if (!Number.isFinite(parsed)) {
      return "";
    }
    return new Date(parsed).toLocaleString(locale, {
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  }, [dutyOverview?.next?.startsAt, locale]);

  const currentShiftCount = asNumber(dutyOverview?.current?.count);
  const nextShiftCount = asNumber(dutyOverview?.next?.count);

  const slaBySeverity = useMemo(() => {
    const source = Array.isArray(dashboardMetrics?.slaBySeverity) ? dashboardMetrics.slaBySeverity : [];
    const bySeverity: Record<"critical" | "high" | "medium" | "low", number> = {
      critical: 0,
      high: 0,
      medium: 0,
      low: 0,
    };

    let resolvedTotal = 0;
    let breachedTotal = 0;

    source.forEach((item: any) => {
      const severity = normalizeSeverity(String(item?.severity || "medium"));
      const total = asNumber(item?.openCases) + asNumber(item?.resolvedCases) + asNumber(item?.breachedCases);
      bySeverity[severity] += total;
      resolvedTotal += asNumber(item?.resolvedCases);
      breachedTotal += asNumber(item?.breachedCases);
    });

    const distributionTotal = bySeverity.critical + bySeverity.high + bySeverity.medium + bySeverity.low;
    const fallbackDistribution = { critical: 12, high: 28, medium: 45, low: 15 };

    const distribution = {
      critical: distributionTotal > 0 ? Math.round((bySeverity.critical / distributionTotal) * 100) : fallbackDistribution.critical,
      high: distributionTotal > 0 ? Math.round((bySeverity.high / distributionTotal) * 100) : fallbackDistribution.high,
      medium: distributionTotal > 0 ? Math.round((bySeverity.medium / distributionTotal) * 100) : fallbackDistribution.medium,
      low: distributionTotal > 0 ? Math.round((bySeverity.low / distributionTotal) * 100) : fallbackDistribution.low,
    };

    const complianceBase = resolvedTotal + breachedTotal;
    const compliance = complianceBase > 0 ? (resolvedTotal / complianceBase) * 100 : 94.2;

    return {
      compliance,
      distribution,
    };
  }, [dashboardMetrics?.slaBySeverity]);

  const mttrMinutes = Math.max(0, Math.round(asNumber(stats?.avgResponseMin)));
  const mttrDeltaLabel = isRu ? "-5м за неделю" : "-5m this week";

  const livestreamEvents = useMemo<StreamEvent[]>(() => {
    const items = Array.isArray(livestreamData?.items) ? livestreamData.items : [];
    if (!items.length) {
      return isRu ? FALLBACK_STREAM_EVENTS_RU : FALLBACK_STREAM_EVENTS_EN;
    }

    return items.slice(0, 4).map((item: any) => {
      const severity = normalizeSeverity(String(item?.severity || "medium"));
      const status = String(item?.status || "").trim();
      const source = String(item?.source || item?.entityId || item?.caseId || "").trim();
      const title =
        String(item?.title || "").trim() ||
        `${severityText(severity, locale)}: ${String(item?.action || "updated").replace(/_/g, " ")}`;

      const description = String(item?.description || "").trim() || (isRu ? "Событие безопасности" : "Security event");

      return {
        id: String(item?.id || `${item?.entity || "event"}-${item?.entityId || Math.random()}`),
        title,
        description,
        severityLabel: status || severityText(severity, locale),
        sourceLabel: source || "-",
        severityClassName: severityClassName(severity),
        timeLabel: formatRelativeShort(String(item?.createdAt || item?.updatedAt || ""), locale),
      };
    });
  }, [isRu, livestreamData?.items, locale]);

  const livestreamGeneratedAtLabel = useMemo(() => {
    const raw = String(livestreamData?.generatedAt || "");
    const parsed = Date.parse(raw);
    if (!Number.isFinite(parsed)) {
      return "";
    }
    return new Date(parsed).toLocaleTimeString(locale, {
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    });
  }, [livestreamData?.generatedAt, locale]);

  const resolutionChartData = useMemo(
    () =>
      buildResolutionBySeverity(cases, chartPeriod, new Date(), {
        locale,
        weekPrefix: t("dashboard.weekPrefix"),
      }),
    [cases, chartPeriod, locale, t],
  );

  const topAnalysts = useMemo<AnalystItem[]>(() => {
    const usersByID = new Map<string, any>();
    users.forEach((user: any) => {
      usersByID.set(String(user?.id || ""), user);
    });

    const source = Array.isArray(dashboardMetrics?.resolvedByAnalyst) ? dashboardMetrics.resolvedByAnalyst : [];
    const fromMetrics = source
      .map((item: any) => {
        const id = String(item?.userId || "").trim();
        const user = usersByID.get(id);
        return {
          id: id || String(item?.username || "").trim(),
          name: String(item?.displayName || user?.name || item?.username || "Unknown"),
          role: mapRoleLabel(String(user?.role || "analyst"), locale),
          avatar: String(user?.avatar || ""),
          resolvedCases: asNumber(item?.resolvedCases),
        };
      })
      .filter((item) => item.id)
      .sort((left, right) => right.resolvedCases - left.resolvedCases);

    if (fromMetrics.length >= 1) {
      return fromMetrics.slice(0, 4);
    }

    return users.slice(0, 4).map((user: any, index: number) => ({
      id: String(user?.id || `user-${index}`),
      name: String(user?.name || `Analyst ${index + 1}`),
      role: mapRoleLabel(String(user?.role || "analyst"), locale),
      avatar: String(user?.avatar || ""),
      resolvedCases: Math.max(0, 140 - index * 14),
    }));
  }, [dashboardMetrics?.resolvedByAnalyst, locale, users]);

  if (showDashboardLoadingSkeleton) {
    return <DashboardLoadingSkeleton />;
  }

  return (
    <AppLayout>
      <div className="mx-auto w-full max-w-[1120px] space-y-6">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {statCardData.map((item) => {
            const TrendIcon = item.trendDirection === "up" ? TrendingUp : TrendingDown;
            const trendColor = item.trendDirection === "up" ? "text-[#66ff4c]" : "text-[#eb5f65]";
            const Icon = item.icon;
            return (
              <Card key={item.id} className={`${CARD_CLASS} h-[150px] p-5`}>
                <div className="flex h-full flex-col justify-between">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <div className={`flex h-10 w-10 items-center justify-center rounded-lg ${item.iconClassName}`}>
                        <Icon size={18} />
                      </div>
                      <div className="text-xs text-[#6b7280]">{item.label}</div>
                    </div>
                  </div>

                  <div>
                    {statsLoading ? (
                      <Skeleton className="h-9 w-20" />
                    ) : (
                      <div className="text-[40px] leading-[36px] tracking-[-0.5px]">{item.value}</div>
                    )}
                    <div className="mt-2 flex items-center gap-1.5 text-xs">
                      <TrendIcon size={11} className={trendColor} />
                      <span className={trendColor}>{item.trendValue}</span>
                      <span className="text-[#6b7280]">{item.trendLabel}</span>
                    </div>
                  </div>
                </div>
              </Card>
            );
          })}
        </div>

        <div className="grid grid-cols-1 gap-4 xl:grid-cols-[357px_1fr]">
          <Card className={`${CARD_CLASS} p-6`}>
            <div className="mb-5 flex items-center justify-between">
              <h3 className="whitespace-nowrap text-[22px] leading-[24px] tracking-[-0.5px] md:text-[24px] md:leading-[26px]">{isRu ? "Дежурные аналитики" : "On-Duty Analysts"}</h3>
              <button
                type="button"
                className="group flex cursor-pointer items-center gap-1.5 rounded-md border border-transparent px-2 py-1 text-sm font-medium text-[#66ff4c] transition-colors hover:border-[rgba(102,255,76,0.28)] hover:bg-[rgba(102,255,76,0.12)] hover:text-[#7dff67]"
                onClick={() => setScheduleOpen(true)}
                data-testid="button-duty-schedule-open"
              >
                <CalendarDays size={14} />
                {isRu ? "График" : "Schedule"}
                <ChevronRight size={12} className="opacity-70 transition-transform group-hover:translate-x-0.5" />
              </button>
            </div>

            <div className="space-y-4">
              <div className="flex items-start justify-between">
                <div>
                  <div className="mb-2 text-xs text-[#6b7280]">{isRu ? "Текущая смена" : "Current Shift"}</div>
                  <div className="flex -space-x-3">
                    {onDutyAnalysts.length > 0 ? (
                      onDutyAnalysts.map((analyst) => (
                        <Link key={`on-duty-${analyst.id}`} href={withTenantPath(currentTenantSlug, `/users/${analyst.id}`)}>
                          <span className="relative inline-flex cursor-pointer rounded-full border border-black/70 p-[2px] dark:border-white/80">
                            <UserAvatar
                              name={analyst.name}
                              avatar={analyst.avatar}
                              className="h-10 w-10"
                              fallbackClassName="bg-[#1d1e29] text-[11px] text-white"
                            />
                          </span>
                        </Link>
                      ))
                    ) : (
                      <div className="text-xs text-[#9ca3af]">{t("dashboard.noOnDutyNow")}</div>
                    )}
                  </div>
                </div>
                <div className="text-sm text-[#66ff4c]">{currentShiftRange || "08:00 - 16:00"}</div>
              </div>

              <div className="h-px bg-[#2a2c3c]" />

              <div className="flex items-start justify-between">
                <div>
                  <div className="mb-2 text-xs text-[#6b7280]">{isRu ? "Следующая смена" : "Next Shift"}</div>
                  <div className="flex -space-x-3">
                    {nextShiftAnalysts.length > 0 ? (
                      nextShiftAnalysts.map((analyst) => (
                        <Link key={`next-duty-${analyst.id}`} href={withTenantPath(currentTenantSlug, `/users/${analyst.id}`)}>
                          <span className="relative inline-flex cursor-pointer rounded-full border border-black/70 p-[2px] dark:border-white/80">
                            <UserAvatar
                              name={analyst.name}
                              avatar={analyst.avatar}
                              className="h-10 w-10"
                              fallbackClassName="bg-[#1d1e29] text-[11px] text-white"
                            />
                          </span>
                        </Link>
                      ))
                    ) : (
                      <div className="text-xs text-[#6b7280]">{isRu ? "Нет данных" : "No data"}</div>
                    )}
                  </div>
                </div>
                <div className="text-sm text-[#6b7280]">{nextShiftRange || nextShiftPoint || "16:00 - 00:00"}</div>
              </div>
            </div>

            <div className="sr-only" data-testid="duty-overview-current">
              {t("dashboard.currentShiftCount", { count: String(currentShiftCount) })}
            </div>
            <div className="sr-only" data-testid="duty-overview-next">
              {(nextShiftRange || nextShiftPoint)
                ? t("dashboard.nextShiftAt", { time: nextShiftRange || nextShiftPoint })
                : t("dashboard.noUpcomingShift")}
            </div>
            {nextShiftCount > 0 ? (
              <div className="sr-only" data-testid="duty-overview-next-count">
                {t("dashboard.nextShiftAnalysts", { count: String(nextShiftCount) })}
              </div>
            ) : null}
          </Card>

          <Dialog open={scheduleOpen} onOpenChange={setScheduleOpen}>
            <DialogContent
              aria-describedby={undefined}
              className="max-h-[90vh] max-w-5xl overflow-y-auto border border-[#2a2c3c] bg-[#11131a] text-white"
            >
              <DialogHeader className="flex flex-row items-center justify-between border-b border-[#2a2c3c] pb-4">
                <div>
                  <DialogTitle className="text-xl">{t("dashboard.shiftManagement")}</DialogTitle>
                  <p className="text-xs text-[#9ca3af]">{t("dashboard.overnightHint")}</p>
                </div>
                <div className="flex items-center gap-2 pr-7">
                  <Button
                    type="button"
                    variant="outline"
                    size="icon"
                    className="h-8 w-8 rounded-md border-[#2a2c3c] bg-[#1d1e29] hover:bg-[#242636]"
                    onClick={handlePrevMonth}
                  >
                    <ChevronLeft size={16} />
                  </Button>
                  <span className="min-w-[180px] text-center text-sm font-semibold capitalize">{scheduleMonthLabel}</span>
                  <Button
                    type="button"
                    variant="outline"
                    size="icon"
                    className="h-8 w-8 rounded-md border-[#2a2c3c] bg-[#1d1e29] hover:bg-[#242636]"
                    onClick={handleNextMonth}
                  >
                    <ChevronRight size={16} />
                  </Button>
                </div>
              </DialogHeader>

              <div className="grid grid-cols-7 gap-3 pt-2">
                {weekdayLabels.map((dayLabel) => (
                  <div key={dayLabel} className="text-center text-[10px] font-semibold uppercase tracking-wide text-[#6b7280]">
                    {dayLabel}
                  </div>
                ))}
                {days.map((day) => {
                  const shifts = currentAssignments[day] || [];
                  const colStart = day === 1 ? firstWeekdayIndex + 1 : undefined;
                  const previousDayShifts =
                    day > 1 ? (currentAssignments[day - 1] || []) : (previousAssignments[previousMonthDaysCount] || []);
                  const continuingShifts = previousDayShifts.filter((shift) => shift.isOvernight);
                  const previewEntries = [
                    ...continuingShifts.map((shift) => ({ shift, isContinuation: true })),
                    ...shifts.map((shift) => ({ shift, isContinuation: false })),
                  ];
                  return (
                    <Popover key={`schedule-day-${day}`}>
                      <PopoverTrigger asChild>
                        <button
                          type="button"
                          className="aspect-square rounded-lg border border-[#2a2c3c] bg-[#161923] p-2 text-left transition-colors hover:border-[#3a3d52] hover:bg-[#1c2030]"
                          style={colStart ? { gridColumnStart: colStart } : undefined}
                        >
                          <div className="text-xs font-semibold text-[#9ca3af]">{day}</div>
                          <div className="mt-1 space-y-1">
                            {previewEntries.slice(0, 2).map(({ shift, isContinuation }) => (
                              <div
                                key={`${isContinuation ? "cont" : "start"}-${shift.id}`}
                                data-testid={isContinuation ? `schedule-day-continuation-${day}-${shift.id}` : undefined}
                                className="relative h-[18px] overflow-hidden rounded-md border border-[#2a2c3c] bg-[#0f131d]"
                              >
                                {(() => {
                                  const segment = resolveShiftPreviewSegment(shift, isContinuation);
                                  return (
                                    <>
                                      <div
                                        className={`absolute inset-y-[1px] rounded-[5px] ${schedulePreviewFillClassName(Boolean(shift.isActive))}`}
                                        style={{ left: `${segment.leftPercent}%`, width: `${segment.widthPercent}%` }}
                                      />
                                      <span className="absolute inset-0 z-[1] truncate px-1.5 text-[10px] leading-[16px] text-[#d1d5db]">
                                        {isContinuation
                                          ? `${resolveAnalystDisplayName(shift)} -> ${shift.end}`
                                          : resolveAnalystDisplayName(shift)}
                                      </span>
                                    </>
                                  );
                                })()}
                              </div>
                            ))}
                            {previewEntries.length > 2 ? (
                              <div className="text-[10px] text-[#6b7280]">+{previewEntries.length - 2}</div>
                            ) : null}
                          </div>
                        </button>
                      </PopoverTrigger>
                      <PopoverContent className="w-[340px] border border-[#2a2c3c] bg-[#11131a] p-4 text-white" side="right" align="start">
                        <div className="space-y-3">
                          <div className="flex items-center justify-between">
                            <div className="flex items-center gap-2 text-sm font-semibold text-[#e5e7eb]">
                              <Clock size={14} />
                              {t("dashboard.dayShifts", { day: String(day) })}
                            </div>
                            <Badge variant="outline" className="border-[#2a2c3c] bg-[#1d1e29] text-[10px] text-[#9ca3af]">
                              {scheduleMonthLabel}
                            </Badge>
                          </div>

                          <div className="space-y-2">
                            {continuingShifts.map((shift) => (
                              <div key={`cont-list-${shift.id}`} className={scheduleRowClassName(Boolean(shift.isActive))}>
                                <div className="min-w-0">
                                  <EllipsisText
                                    text={resolveAnalystDisplayName(shift)}
                                    className={scheduleNameClassName(Boolean(shift.isActive))}
                                  />
                                  <div className={scheduleMetaClassName(Boolean(shift.isActive))}>
                                    <span>{isRu ? "Продолжение смены" : "Continuing shift"}</span>
                                    <span>{shift.end}</span>
                                  </div>
                                </div>
                                <Moon size={11} className={scheduleMoonIconClassName(Boolean(shift.isActive))} />
                              </div>
                            ))}
                            {shifts.map((shift) => (
                              <div key={`shift-${shift.id}`} className={scheduleRowClassName(Boolean(shift.isActive))}>
                                <div className="min-w-0">
                                  <EllipsisText
                                    text={resolveAnalystDisplayName(shift)}
                                    className={scheduleNameClassName(Boolean(shift.isActive))}
                                  />
                                  <div className={scheduleMetaClassName(Boolean(shift.isActive))}>
                                    <span>{shift.start}</span>
                                    <span>-</span>
                                    <span>{shift.end}</span>
                                    {shift.isOvernight ? <Moon size={11} className={scheduleMoonIconClassName(Boolean(shift.isActive))} /> : null}
                                  </div>
                                </div>
                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="icon"
                                  className="h-7 w-7 text-[#eb5f65] hover:bg-[rgba(235,95,101,0.1)] hover:text-[#eb5f65]"
                                  onClick={() => removeShift(shift.id)}
                                >
                                  <X size={12} />
                                </Button>
                              </div>
                            ))}
                            {shifts.length === 0 && continuingShifts.length === 0 ? (
                              <div className="rounded-md border border-dashed border-[#2a2c3c] px-3 py-2 text-center text-xs text-[#6b7280]">
                                {t("dashboard.noShiftsToday")}
                              </div>
                            ) : null}
                          </div>

                          <Separator className="bg-[#2a2c3c]" />

                          <div className="space-y-2">
                            <div className="space-y-1">
                              <Label className="text-[10px] uppercase tracking-wide text-[#6b7280]">{t("dashboard.analyst")}</Label>
                              <Select
                                value={effectiveAnalystId || undefined}
                                onValueChange={(value) => setNewShift((prev) => ({ ...prev, analystId: value }))}
                              >
                                <SelectTrigger className="h-9 border-[#2a2c3c] bg-[#1a1d28] text-white">
                                  <SelectValue placeholder={isRu ? "Выберите аналитика" : "Select analyst"} />
                                </SelectTrigger>
                                <SelectContent>
                                  {analystsList.map((analyst) => (
                                    <SelectItem key={analyst.id} value={analyst.id}>
                                      {analyst.name}
                                    </SelectItem>
                                  ))}
                                </SelectContent>
                              </Select>
                            </div>
                            <div className="grid grid-cols-2 gap-2">
                              <div className="space-y-1">
                                <Label className="text-[10px] uppercase tracking-wide text-[#6b7280]">{t("dashboard.start")}</Label>
                                <Input
                                  type="time"
                                  value={newShift.start}
                                  onChange={(event) => setNewShift((prev) => ({ ...prev, start: event.target.value }))}
                                  className="h-9 border-[#2a2c3c] bg-[#1a1d28] text-white"
                                />
                              </div>
                              <div className="space-y-1">
                                <Label className="text-[10px] uppercase tracking-wide text-[#6b7280]">{t("dashboard.end")}</Label>
                                <Input
                                  type="time"
                                  value={newShift.end}
                                  onChange={(event) => setNewShift((prev) => ({ ...prev, end: event.target.value }))}
                                  className="h-9 border-[#2a2c3c] bg-[#1a1d28] text-white"
                                />
                              </div>
                            </div>
                            <Button
                              type="button"
                              className="h-9 w-full gap-2 bg-[#4ed938] text-[#0b0c10] hover:bg-[#66ff4c]"
                              onClick={() => addShiftHandler(day)}
                            >
                              <Plus size={14} />
                              {t("dashboard.addShift")}
                            </Button>
                          </div>
                        </div>
                      </PopoverContent>
                    </Popover>
                  );
                })}
              </div>

              <div className="mt-5 flex justify-end">
                <Button
                  type="button"
                  className="h-9 bg-[#4ed938] px-6 text-[#0b0c10] hover:bg-[#66ff4c]"
                  onClick={() => {
                    toast.success(t("dashboard.scheduleSynced"));
                    setScheduleOpen(false);
                  }}
                >
                  {t("dashboard.confirmAllChanges")}
                </Button>
              </div>
            </DialogContent>
          </Dialog>

          <Card className={`${CARD_CLASS} p-6`} data-testid="dashboard-metrics-widget">
            <div className="mb-5 flex items-center justify-between">
              <h3 className="text-[28px] leading-[28px] tracking-[-0.5px]">{isRu ? "Операционные метрики" : "Operational Metrics"}</h3>
              <button
                type="button"
                className="flex items-center gap-1 text-xs text-[#66ff4c] hover:text-[#7dff67]"
                onClick={() => {
                  resetMetricForm();
                  setMetricsBuilderOpen(true);
                }}
                data-testid="button-dashboard-customize-open"
              >
                <ChartNoAxesColumn size={12} />
                {isRu ? "Настроить" : "Customize"}
              </button>
            </div>

            <div className="grid grid-cols-1 gap-3 lg:grid-cols-3">
              <div className="rounded-lg border border-[#2a2c3c] p-4">
                <div className="mb-2 text-xs text-[#6b7280]">SLA Compliance</div>
                <div className="mb-3 text-[40px] leading-[32px] tracking-[-0.5px]">{slaBySeverity.compliance.toFixed(1)}%</div>
                <div className="h-2 rounded-full bg-[#0b0c10]">
                  <div
                    className="h-2 rounded-full bg-[#66ff4c]"
                    style={{ width: `${Math.max(0, Math.min(100, slaBySeverity.compliance))}%` }}
                  />
                </div>
              </div>

              <div className="rounded-lg border border-[#2a2c3c] p-4">
                <div className="mb-2 text-xs text-[#6b7280]">Alert Distribution</div>
                <div className="space-y-1.5 text-sm">
                  <div className="flex items-center justify-between"><span className="text-[#9ca3af]">Critical</span><span className="text-[#eb5f65]">{slaBySeverity.distribution.critical}%</span></div>
                  <div className="flex items-center justify-between"><span className="text-[#9ca3af]">High</span><span className="text-[#ffc700]">{slaBySeverity.distribution.high}%</span></div>
                  <div className="flex items-center justify-between"><span className="text-[#9ca3af]">Medium</span><span className="text-[#66ff4c]">{slaBySeverity.distribution.medium}%</span></div>
                  <div className="flex items-center justify-between"><span className="text-[#9ca3af]">Low</span><span className="text-[#3b82f6]">{slaBySeverity.distribution.low}%</span></div>
                </div>
              </div>

              <div className="rounded-lg border border-[#2a2c3c] p-4">
                <div className="mb-2 text-xs text-[#6b7280]">MTTR Trend</div>
                <div className="mb-2 text-[40px] leading-[32px] tracking-[-0.5px]">{mttrMinutes}m <span className="text-base text-[#6b7280]">avg</span></div>
                <div className="flex items-center gap-1 text-sm text-[#66ff4c]">
                  <TrendingDown size={12} />
                  {mttrDeltaLabel}
                </div>
              </div>
            </div>

            {customMetrics.length > 0 ? (
              <div className="mt-4 space-y-2 border-t border-[#2a2c3c] pt-4">
                {customMetrics.slice(0, 3).map((metric: any) => (
                  <div key={metric.id} className="flex items-center justify-between rounded-md border border-[#2a2c3c] bg-[#171923] px-3 py-2">
                    <div className="min-w-0">
                      <EllipsisText text={metric.name} className="max-w-[260px] text-xs font-medium text-white" />
                      <div className="text-[11px] text-[#6b7280]">
                        {metric.source} · {metric.measure} · {metric.enabled ? (isRu ? "вкл" : "on") : (isRu ? "выкл" : "off")}
                      </div>
                    </div>
                    <div className="text-sm font-semibold text-white">{Math.round(Number(metric.value || 0))}</div>
                  </div>
                ))}
              </div>
            ) : null}
          </Card>
        </div>

        <Card className={`${CARD_CLASS} h-[530px] p-[25px]`} data-testid="dashboard-livestream-widget">
          <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div className="flex min-w-0 items-start gap-3 md:items-center">
              <div className="mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-[rgba(235,95,101,0.2)] text-[#eb5f65]">
                <LivestreamGlyph />
              </div>
              <div>
                <h3 className="text-lg font-normal leading-7 tracking-[-0.5px] text-white">
                  {isRu ? "Лента активности" : "Live Activity Stream"}
                </h3>
                <p className="text-xs leading-4 tracking-[-0.4px] text-[#6b7280]">
                  {isRu ? "События безопасности в реальном времени" : "Real-time security events"}
                  {livestreamGeneratedAtLabel ? (
                    <span className="ml-2 text-[#8b91a3]">{isRu ? `Обновлено ${livestreamGeneratedAtLabel}` : `Updated ${livestreamGeneratedAtLabel}`}</span>
                  ) : null}
                </p>
              </div>
            </div>
            <div className="flex w-full items-center gap-2 md:w-[354px]">
              <div className="hidden h-[30px] shrink-0 items-center rounded-md border border-[#2a2c3c] bg-[#10131e] px-2 text-[11px] text-[#8b91a3] md:inline-flex">
                {livestreamEvents.length} {isRu ? "событий" : "events"}
              </div>
              <label className="relative block w-full md:w-64">
                <Search size={12} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[#6b7280]" />
                <Input
                  value={streamSearch}
                  onChange={(event) => setStreamSearch(event.target.value)}
                  placeholder={isRu ? "Фильтр событий..." : "Filter events..."}
                  className="h-[38px] w-full rounded-lg border border-[#2a2c3c] bg-[#0b0c10] pl-9 pr-3 text-sm text-[#d1d5db] placeholder:text-[rgba(209,213,219,0.5)] focus-visible:ring-1 focus-visible:ring-[#3b4a79]"
                />
              </label>
              <Button
                variant="outline"
                size="icon"
                className="h-[38px] w-[34.75px] rounded-lg border border-[#2a2c3c] bg-[#1d1e29] text-[#9ca3af] hover:bg-[#252638] hover:text-[#d1d5db]"
                onClick={toggleStreamPause}
                aria-label={streamPaused ? "Resume stream" : "Pause stream"}
                data-testid="button-dashboard-stream-pause"
              >
                {streamPaused ? <Play size={13} /> : <Pause size={13} />}
              </Button>
            </div>
          </div>

          <div className="mt-5 h-[416px] space-y-2 overflow-y-auto">
            {streamLoading && !livestreamEvents.length
              ? Array.from({ length: 4 }).map((_, index) => (
                <div key={`stream-skeleton-${index}`} className="h-[98px] rounded-lg px-3 py-3">
                  <Skeleton className="h-full w-full rounded-lg" />
                </div>
              ))
              : livestreamEvents.map((event) => {
                const dotClass = event.severityClassName.includes("eb5f65")
                  ? `${event.severityClassName} opacity-75`
                  : event.severityClassName;
                return (
                  <div key={event.id} className="relative h-[98px] w-full rounded-lg border border-[#1d2334] bg-[#10131d]/65 px-3 py-3 transition-colors hover:border-[#2d3650] hover:bg-[#13192a]">
                    <span className={`absolute left-3 top-[19px] h-2 w-2 rounded-full ${dotClass}`} />
                    <div className="ml-5 min-w-0">
                      <div className="flex items-start justify-between gap-3">
                        <p className="truncate pr-2 text-sm font-normal tracking-[-0.5px] text-white">{event.title}</p>
                        <span className="shrink-0 pt-0.5 text-xs text-[#6b7280]">{event.timeLabel}</span>
                      </div>
                      <p className="mt-1 truncate text-xs tracking-[-0.5px] text-[#9ca3af]">{event.description}</p>
                      <div className="mt-2 flex items-center gap-6">
                        <span className={`rounded bg-[#151b2b] px-1.5 py-0.5 text-[10px] tracking-[-0.5px] ${event.severityClassName.replace("bg-", "text-")}`}>
                          {event.severityLabel}
                        </span>
                        <span className="text-xs tracking-[-0.5px] text-[#6b7280]">{event.sourceLabel}</span>
                      </div>
                    </div>
                  </div>
                );
              })}
          </div>
        </Card>

        <div className="grid grid-cols-1 gap-4 xl:grid-cols-[548px_1fr]">
          <Card className={`${CARD_CLASS} p-6`}>
            <div className="mb-4 flex items-center justify-between">
              <h3 className="text-[28px] leading-[28px] tracking-[-0.5px]">{t("dashboard.caseResolutionBySeverity")}</h3>
              <div className="flex items-center gap-2 rounded-lg bg-[#1d1e29] p-1">
                <button
                  type="button"
                  data-testid="button-case-resolution-week"
                  onClick={() => setChartPeriod("week")}
                  className={`rounded-md px-3 py-1.5 text-xs ${
                    chartPeriod === "week" ? "bg-[#4ed938] text-[#0b0c10]" : "text-[#9ca3af]"
                  }`}
                >
                  {t("dashboard.week")}
                </button>
                <button
                  type="button"
                  data-testid="button-case-resolution-month"
                  onClick={() => setChartPeriod("month")}
                  className={`rounded-md px-3 py-1.5 text-xs ${
                    chartPeriod === "month" ? "bg-[#4ed938] text-[#0b0c10]" : "text-[#9ca3af]"
                  }`}
                >
                  {t("dashboard.month")}
                </button>
              </div>
            </div>
            <div className="h-[300px] w-full">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={resolutionChartData} margin={{ left: 8, right: 8, top: 8, bottom: 0 }}>
                  <CartesianGrid vertical={false} stroke="rgba(255,255,255,0.06)" />
                  <XAxis dataKey="name" axisLine={false} tickLine={false} tick={{ fill: "#6b7280", fontSize: 11 }} />
                  <YAxis axisLine={false} tickLine={false} tick={{ fill: "#6b7280", fontSize: 11 }} />
                  <ChartTooltip
                    cursor={{ fill: "rgba(78,217,56,0.08)" }}
                    contentStyle={{
                      border: "1px solid rgba(255,255,255,0.06)",
                      background: "rgba(19,20,28,0.98)",
                      borderRadius: "10px",
                      color: "#fff",
                    }}
                  />
                  <Legend wrapperStyle={{ color: "#9ca3af", fontSize: 11 }} />
                  <Bar dataKey="Critical" stackId="cases" fill="#eb5f65" radius={[0, 0, 0, 0]} barSize={30} />
                  <Bar dataKey="High" stackId="cases" fill="#ffc700" radius={[0, 0, 0, 0]} barSize={30} />
                  <Bar dataKey="Medium" stackId="cases" fill="#3b82f6" radius={[0, 0, 0, 0]} barSize={30} />
                  <Bar dataKey="Low" stackId="cases" fill="#66ff4c" radius={[6, 6, 0, 0]} barSize={30} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          </Card>

          <Card className={`${CARD_CLASS} p-6`}>
            <div className="mb-4 flex items-center justify-between">
              <h3 className="text-[28px] leading-[28px] tracking-[-0.5px]">{t("dashboard.topAnalysts")}</h3>
              <span className="text-xs text-[#6b7280]">{isRu ? "Этот месяц" : "This Month"}</span>
            </div>

            <div className="space-y-4">
              {topAnalysts.map((analyst, index) => (
                <Link key={analyst.id} href={withTenantPath(currentTenantSlug, `/users/${analyst.id}`)}>
                  <div className="flex cursor-pointer items-center justify-between rounded-lg px-2 py-1 hover:bg-[#1d1e29]">
                    <div className="flex min-w-0 items-center gap-3">
                      <div className="relative">
                        <UserAvatar
                          name={analyst.name}
                          avatar={analyst.avatar}
                          className="h-12 w-12"
                          fallbackClassName="bg-[#1d1e29] text-[12px] text-white"
                        />
                        {index < 3 ? (
                          <Badge className="absolute -right-1 -top-1 flex h-5 w-5 items-center justify-center rounded-full border border-[#13141c] bg-[#2a2c3c] p-0 text-[10px] font-semibold leading-none text-white">
                            {index + 1}
                          </Badge>
                        ) : null}
                      </div>
                      <div className="min-w-0">
                        <EllipsisText text={analyst.name} className="max-w-[180px] text-sm font-semibold text-white" />
                        <EllipsisText text={analyst.role} className="max-w-[180px] text-xs leading-4 text-[#6b7280]" />
                      </div>
                    </div>
                    <div className="text-right">
                      <div className={`${index === 0 ? "text-[#66ff4c]" : "text-white"} text-[28px] leading-[24px] tracking-[-0.5px]`}>
                        {analyst.resolvedCases}
                      </div>
                      <div className="text-[11px] text-[#6b7280]">{isRu ? "кейсов" : "cases"}</div>
                    </div>
                  </div>
                </Link>
              ))}
            </div>
          </Card>
        </div>

        <Dialog
          open={metricsBuilderOpen}
          onOpenChange={(open) => {
            setMetricsBuilderOpen(open);
            if (!open) {
              resetMetricForm();
            }
          }}
        >
          <DialogContent
            aria-describedby={undefined}
            className="max-h-[90vh] max-w-2xl overflow-y-auto border border-[#2a2c3c] bg-[#11131a] text-white"
          >
            <DialogHeader>
              <DialogTitle>{editingMetricID ? (isRu ? "Редактирование метрики" : "Edit metric") : (isRu ? "Новая метрика" : "New metric")}</DialogTitle>
            </DialogHeader>
            <div className="space-y-4">
              <div className="space-y-2">
                {customMetrics.map((metric: any) => (
                  <div key={metric.id} className="flex items-center gap-2 rounded-md border border-[#2a2c3c] bg-[#171923] px-3 py-2">
                    <div className="min-w-0 flex-1">
                      <EllipsisText text={metric.name} className="max-w-[280px] text-sm font-semibold text-white" />
                      <div className="text-[11px] text-[#6b7280]">
                        {metric.source} · {metric.measure}
                        {metric.error ? ` · ${metric.error}` : ""}
                      </div>
                    </div>
                    <div className="text-sm font-semibold">{Math.round(Number(metric.value || 0))}</div>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="h-7 border-[#2a2c3c] bg-[#1d1e29] text-xs hover:bg-[#242636]"
                      onClick={() => editMetric(metric)}
                    >
                      {isRu ? "Изм." : "Edit"}
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="h-7 border-[#2a2c3c] bg-[#1d1e29] text-xs hover:bg-[#242636]"
                      onClick={() => toggleMetricEnabled(metric)}
                    >
                      {metric.enabled ? (isRu ? "Выкл" : "Disable") : (isRu ? "Вкл" : "Enable")}
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7 text-[#eb5f65] hover:bg-[rgba(235,95,101,0.1)] hover:text-[#eb5f65]"
                      onClick={() => removeMetric(metric)}
                    >
                      <X size={12} />
                    </Button>
                  </div>
                ))}
              </div>

              <Separator className="bg-[#2a2c3c]" />

              <div className="space-y-1">
                <Label className="text-[11px] text-[#9ca3af]">{isRu ? "Название" : "Name"}</Label>
                <Input
                  value={metricName}
                  onChange={(event) => setMetricName(event.target.value)}
                  placeholder={isRu ? "Например, Open phishing incidents" : "e.g. Open phishing incidents"}
                  className="border-[#2a2c3c] bg-[#1a1d28] text-white"
                />
              </div>

              <div className="space-y-1">
                <Label className="text-[11px] text-[#9ca3af]">{isRu ? "Описание" : "Description"}</Label>
                <Input
                  value={metricDescription}
                  onChange={(event) => setMetricDescription(event.target.value)}
                  placeholder={isRu ? "Опционально" : "Optional context"}
                  className="border-[#2a2c3c] bg-[#1a1d28] text-white"
                />
              </div>

              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div className="space-y-1">
                  <Label className="text-[11px] text-[#9ca3af]">{isRu ? "Источник" : "Source"}</Label>
                  <Select
                    value={metricSource}
                    onValueChange={(value: "cases" | "alerts") => {
                      setMetricSource(value);
                      if (value === "alerts" && metricMeasure !== "count") {
                        setMetricMeasure("count");
                      }
                    }}
                  >
                    <SelectTrigger className="border-[#2a2c3c] bg-[#1a1d28] text-white">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="cases">Cases</SelectItem>
                      <SelectItem value="alerts">Alerts</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1">
                  <Label className="text-[11px] text-[#9ca3af]">{isRu ? "Агрегация" : "Measure"}</Label>
                  <Select value={metricMeasure} onValueChange={(value: any) => setMetricMeasure(value)}>
                    <SelectTrigger className="border-[#2a2c3c] bg-[#1a1d28] text-white">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="count">{isRu ? "Количество" : "Count"}</SelectItem>
                      {metricSource === "cases" ? <SelectItem value="avg_resolution_minutes">{isRu ? "Среднее время (мин)" : "Avg resolution (min)"}</SelectItem> : null}
                      {metricSource === "cases" ? <SelectItem value="overdue_count">{isRu ? "Просроченные" : "Overdue count"}</SelectItem> : null}
                    </SelectContent>
                  </Select>
                </div>
              </div>

              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div className="space-y-1">
                  <Label className="text-[11px] text-[#9ca3af]">{isRu ? "Статусы (через запятую)" : "Statuses (comma-separated)"}</Label>
                  <Input
                    value={metricStatuses}
                    onChange={(event) => setMetricStatuses(event.target.value)}
                    placeholder="open, resolved"
                    className="border-[#2a2c3c] bg-[#1a1d28] text-white"
                  />
                </div>
                <div className="space-y-1">
                  <Label className="text-[11px] text-[#9ca3af]">{isRu ? "Severity (через запятую)" : "Severities (comma-separated)"}</Label>
                  <Input
                    value={metricSeverities}
                    onChange={(event) => setMetricSeverities(event.target.value)}
                    placeholder="critical, high"
                    className="border-[#2a2c3c] bg-[#1a1d28] text-white"
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div className="space-y-1">
                  <Label className="text-[11px] text-[#9ca3af]">{isRu ? "Категории (через запятую)" : "Categories (comma-separated)"}</Label>
                  <Input
                    value={metricCategories}
                    onChange={(event) => setMetricCategories(event.target.value)}
                    placeholder="phishing, malware"
                    className="border-[#2a2c3c] bg-[#1a1d28] text-white"
                  />
                </div>
                <div className="space-y-1">
                  <Label className="text-[11px] text-[#9ca3af]">{isRu ? "Создано за (часы)" : "Created within hours"}</Label>
                  <Input
                    type="number"
                    min={0}
                    value={metricCreatedWithinHours}
                    onChange={(event) => setMetricCreatedWithinHours(event.target.value)}
                    className="border-[#2a2c3c] bg-[#1a1d28] text-white"
                  />
                </div>
              </div>

              {metricSource === "cases" && metricMeasure === "overdue_count" ? (
                <div className="space-y-1">
                  <Label className="text-[11px] text-[#9ca3af]">{isRu ? "Порог просрочки (минуты)" : "Overdue threshold (minutes)"}</Label>
                  <Input
                    type="number"
                    min={1}
                    value={metricOverdueMinutes}
                    onChange={(event) => setMetricOverdueMinutes(event.target.value)}
                    className="border-[#2a2c3c] bg-[#1a1d28] text-white"
                  />
                </div>
              ) : null}

              <div className="flex justify-end gap-2 pt-2">
                <Button
                  type="button"
                  variant="outline"
                  className="border-[#2a2c3c] bg-[#1d1e29] hover:bg-[#242636]"
                  onClick={() => {
                    setMetricsBuilderOpen(false);
                    resetMetricForm();
                  }}
                >
                  {isRu ? "Отмена" : "Cancel"}
                </Button>
                <Button type="button" className="bg-[#4ed938] text-[#0b0c10] hover:bg-[#66ff4c]" onClick={saveMetric}>
                  {editingMetricID ? (isRu ? "Сохранить" : "Save") : (isRu ? "Создать" : "Create")}
                </Button>
              </div>
            </div>
          </DialogContent>
        </Dialog>
      </div>
    </AppLayout>
  );
}
