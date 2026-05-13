import { AppLayout } from "@/components/layout";
import { Card as BaseCard } from "@/components/ui/card";
import {
  useAppState, useUsers, useTenants, useCreateTenant, useUpdateTenant,
  useCreateUser, useUpdateUser, useDeleteUser,
  useRateLimits, useCreateRateLimit, useUpdateRateLimit, useDeleteRateLimit,
  useSOCAccessPolicy, useUpsertSOCAccessPolicy,
  useAchievements, useCreateAchievement, useUploadAchievementIcon, useDeleteAchievement,
  useUserAchievements, useGrantAchievement, useRevokeAchievement,
  useInfiniteUserExperienceEvents, useAwardUserExperience,
  useAdminApiTokens, useCreateAdminApiToken, useRevokeAdminApiToken,
  useAdminNotificationBots, useCreateAdminNotificationBot, useUpdateAdminNotificationBot, useDeleteAdminNotificationBot,
  useAdminNotificationSettings, useSaveAdminNotificationSetting,
  useServiceAlertRules, useCreateServiceAlertRule, useUpdateServiceAlertRule, useDeleteServiceAlertRule,
  useSystemResources, useFactoryResetLocal
} from "@/lib/api";
import { Button, type ButtonProps } from "@/components/ui/button";
import { Input as BaseInput } from "@/components/ui/input";
import { Label as BaseLabel } from "@/components/ui/label";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { type ChangeEvent, type ComponentPropsWithoutRef, useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import {
  Plus, Trash2, Save,
  Users, Building2, Activity, Shield, AlertTriangle,
  Cpu, HardDrive, Gauge, BarChart3, Clock, UserPlus, Trophy, Award,
  Settings, Zap as ZapIcon, Edit, Radio, KeyRound, Upload,
  Square, Play, Search
} from "lucide-react";
import { Textarea as BaseTextarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { SelectTrigger as BaseSelectTrigger } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Checkbox } from "@/components/ui/checkbox";
import { EllipsisText } from "@/components/ui/ellipsis-text";
import { useLocation } from "wouter";
import { CaseStatusesPanel } from "@/components/admin/case-statuses-panel";
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle
} from "@/components/ui/dialog";
import { ResponsiveContainer, AreaChart, Area, XAxis, YAxis, Tooltip, CartesianGrid, LineChart, Line } from "recharts";
import { useT } from "@/lib/i18n";
import { formatModuleOperationLatency, formatModuleResponseMs, formatOperationWindow } from "@/lib/module-health";
import { applySearchPatch, splitLocationPathAndSearch } from "@/lib/url-state";
import { cn } from "@/lib/utils";
import { useMinimumLoading } from "@/lib/use-minimum-loading";
import {
  ACHIEVEMENT_ICON_EMOJI_OPTIONS,
  ACHIEVEMENT_ICON_PRESETS,
  ACHIEVEMENT_ICON_UPLOAD_CRITERIA,
  DEFAULT_ACHIEVEMENT_ICON,
  isAchievementImageIcon,
  upsertAchievementIconOption,
  type AchievementIconOption,
} from "@/lib/achievement-icons";

const RARITY_STYLES: Record<string, { border: string; bg: string; badge: string; glow: string }> = {
  Common: {
    border: "border-gray-300",
    bg: "bg-gray-50 dark:bg-gray-900/30",
    badge: "bg-gray-200 text-gray-700 border-gray-300",
    glow: "",
  },
  Rare: {
    border: "border-blue-400",
    bg: "bg-blue-50/50 dark:bg-blue-950/20",
    badge: "bg-blue-100 text-blue-700 border-blue-300",
    glow: "shadow-[0_0_12px_rgba(59,130,246,0.25)]",
  },
  Epic: {
    border: "border-purple-400",
    bg: "bg-purple-50/50 dark:bg-purple-950/20",
    badge: "bg-purple-100 text-purple-700 border-purple-300",
    glow: "shadow-[0_0_16px_rgba(168,85,247,0.3)]",
  },
  Legendary: {
    border: "border-orange-400",
    bg: "bg-gradient-to-br from-orange-50/60 to-yellow-50/60 dark:from-orange-950/20 dark:to-yellow-950/20",
    badge: "bg-orange-100 text-orange-700 border-orange-400",
    glow: "shadow-[0_0_20px_rgba(234,179,8,0.35)]",
  },
  Diamond: {
    border: "border-cyan-300",
    bg: "bg-gradient-to-br from-cyan-50/60 via-white to-pink-50/40 dark:from-cyan-950/30 dark:via-gray-900 dark:to-pink-950/20",
    badge: "bg-gradient-to-r from-cyan-200 to-pink-200 text-cyan-800 border-cyan-300",
    glow: "shadow-[0_0_24px_rgba(6,182,212,0.4),0_0_48px_rgba(236,72,153,0.15)]",
  },
};

const API_TOKEN_SCOPE_OPTIONS = [
  { key: "alerts:read", label: "Alerts read" },
  { key: "alerts:write", label: "Alerts write" },
  { key: "cases:read", label: "Cases read" },
  { key: "cases:write", label: "Cases write" },
  { key: "communications:read", label: "Comms read" },
  { key: "communications:write", label: "Comms write" },
  { key: "connectors:read", label: "Connectors read" },
  { key: "connectors:write", label: "Connectors write" },
  { key: "catalog:read", label: "Catalog read" },
  { key: "catalog:write", label: "Catalog write" },
  { key: "ai:read", label: "AI read" },
  { key: "ai:write", label: "AI write" },
  { key: "dashboard:read", label: "Dashboard read" },
  { key: "system:read", label: "System read" },
  { key: "search:read", label: "Search read" },
  { key: "operations:read", label: "Operations read" },
  { key: "tenants:read", label: "Tenants read" },
  { key: "tenants:write", label: "Tenants write" },
  { key: "experience:write", label: "Award XP" },
] as const;

const XP_HISTORY_BATCH_SIZE = 30;
const MODULE_SERIES_COLORS = [
  "#22c55e",
  "#3b82f6",
  "#f59e0b",
  "#a855f7",
  "#06b6d4",
  "#ef4444",
  "#14b8a6",
  "#f97316",
];
const MODULE_LABELS: Record<string, string> = {
  api: "API",
  postgres: "Postgres",
  redis: "Redis",
  elasticsearch: "Elastic",
  s3: "S3",
  ai_model: "AI",
  async_ops: "Async Ops",
  workflow_engine: "Workflow",
  forum_proxy: "Forum Proxy",
};

const ADMIN_PAGE_SHELL_CLASS = "mx-auto w-full max-w-[1240px] space-y-6";
const ADMIN_TAB_LIST_CLASS =
  "h-auto min-h-[52px] w-full flex-wrap justify-start overflow-hidden rounded-xl border border-[#2a2c3c] bg-[#10121a] p-0";
const ADMIN_TAB_TRIGGER_CLASS =
  "relative flex h-[52px] min-w-[210px] flex-1 items-center justify-center gap-2 rounded-none border-b border-r border-[#1d1e29] px-5 text-sm font-medium text-[#9ca3af] transition-colors hover:bg-[#171b2a] hover:text-[#d1d5db] [&>svg]:text-current last:border-r-0 data-[state=active]:bg-[#13141c] data-[state=active]:text-[#66ff4c] data-[state=active]:shadow-none data-[state=active]:after:absolute data-[state=active]:after:bottom-0 data-[state=active]:after:left-4 data-[state=active]:after:right-4 data-[state=active]:after:h-[2px] data-[state=active]:after:rounded-full data-[state=active]:after:bg-[#66ff4c]";
const ADMIN_CARD_CLASS = "rounded-2xl border border-[#2a2c3c] bg-[#13141c] text-[#e5e7eb] shadow-[0_4px_20px_-2px_rgba(0,0,0,0.3)]";
const ADMIN_INPUT_CLASS = "rounded-lg border-[#2a2c3c] bg-[#0f131d] text-[#e5e7eb] placeholder:text-[#6b7280] focus-visible:border-[#66ff4c] focus-visible:ring-2 focus-visible:ring-[#66ff4c]/20";
const ADMIN_TEXTAREA_CLASS = "rounded-lg border-[#2a2c3c] bg-[#0f131d] text-[#e5e7eb] placeholder:text-[#6b7280] focus-visible:border-[#66ff4c] focus-visible:ring-2 focus-visible:ring-[#66ff4c]/20";
const ADMIN_SELECT_TRIGGER_CLASS = "rounded-lg border-[#2a2c3c] bg-[#0f131d] text-[#e5e7eb] data-[placeholder]:text-[#6b7280] focus:ring-2 focus:ring-[#66ff4c]/20 focus:border-[#66ff4c]";
const ADMIN_LABEL_CLASS = "text-xs font-semibold text-[#9ca3af]";
const ADMIN_BUTTON_BASE_CLASS = "rounded-lg font-semibold";
const ADMIN_BUTTON_OUTLINE_CLASS = "border-[#2a2c3c] bg-[#10141f] text-[#d1d5db] hover:bg-[#171b2a] hover:text-white";
const ADMIN_BUTTON_GHOST_CLASS = "text-[#9ca3af] hover:bg-[#171b2a] hover:text-white";
const ADMIN_BUTTON_DESTRUCTIVE_CLASS = "border-[#eb5f65]/40 bg-[#eb5f65]/15 text-[#eb5f65] hover:bg-[#eb5f65]/25";
const ADMIN_BUTTON_SECONDARY_CLASS = "border-[#2a2c3c] bg-[#171b2a] text-[#d1d5db] hover:bg-[#202538]";
const ADMIN_FIGMA_CARD_CLASS = "rounded-lg border border-[#2a2c3c] bg-[#0b0c10] shadow-[0_4px_20px_rgba(0,0,0,0.3)]";
const ADMIN_FIGMA_PRIMARY_BUTTON_CLASS = "inline-flex h-9 items-center justify-center gap-2 rounded-lg bg-[#4ed938] px-4 text-sm font-medium text-[#0b0c10] shadow-[0_0_10px_rgba(102,255,76,0.2)] transition-colors hover:bg-[#66ff4c]";
const ADMIN_FIGMA_SECONDARY_BUTTON_CLASS = "inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-[#2a2c3c] bg-[#1d1e29] px-4 text-sm font-medium text-white transition-colors hover:bg-[#242638]";
const ADMIN_FIGMA_SECTION_CLASS = "space-y-6 rounded-xl border border-[#2a2c3c] bg-[#13141c] p-6 shadow-[0_4px_20px_-2px_rgba(0,0,0,0.3)]";

type AdminCardProps = ComponentPropsWithoutRef<typeof BaseCard>;
type AdminInputProps = ComponentPropsWithoutRef<typeof BaseInput>;
type AdminTextareaProps = ComponentPropsWithoutRef<typeof BaseTextarea>;
type AdminSelectTriggerProps = ComponentPropsWithoutRef<typeof BaseSelectTrigger>;
type AdminLabelProps = ComponentPropsWithoutRef<typeof BaseLabel>;

function AdminCard({ className, ...props }: AdminCardProps) {
  return <BaseCard className={cn(ADMIN_CARD_CLASS, className)} {...props} />;
}

function AdminInput({ className, ...props }: AdminInputProps) {
  return <BaseInput className={cn(ADMIN_INPUT_CLASS, className)} {...props} />;
}

function AdminTextarea({ className, ...props }: AdminTextareaProps) {
  return <BaseTextarea className={cn(ADMIN_TEXTAREA_CLASS, className)} {...props} />;
}

function AdminSelectTrigger({ className, ...props }: AdminSelectTriggerProps) {
  return <BaseSelectTrigger className={cn(ADMIN_SELECT_TRIGGER_CLASS, className)} {...props} />;
}

function AdminLabel({ className, ...props }: AdminLabelProps) {
  return <BaseLabel className={cn(ADMIN_LABEL_CLASS, className)} {...props} />;
}

function AdminButton({ className, variant, ...props }: ButtonProps) {
  const variantClass = variant === "outline"
    ? ADMIN_BUTTON_OUTLINE_CLASS
    : variant === "ghost"
      ? ADMIN_BUTTON_GHOST_CLASS
      : variant === "destructive"
        ? ADMIN_BUTTON_DESTRUCTIVE_CLASS
        : variant === "secondary"
          ? ADMIN_BUTTON_SECONDARY_CLASS
          : "";
  return <Button variant={variant} className={cn(ADMIN_BUTTON_BASE_CLASS, variantClass, className)} {...props} />;
}

function roleLabelToValue(roleLabel: string): "tenant_admin" | "analyst" | "viewer" {
  const normalized = String(roleLabel || "").trim().toLowerCase();
  if (normalized.includes("platform")) return "tenant_admin";
  if (normalized.includes("tenant") && normalized.includes("admin")) return "tenant_admin";
  if (normalized.includes("admin")) return "tenant_admin";
  if (normalized.includes("view")) return "viewer";
  return "analyst";
}

function roleValueToLabel(roleValue: string): string {
  const normalized = String(roleValue || "").trim().toLowerCase();
  if (normalized === "tenant_admin") return "Tenant Admin";
  if (normalized === "viewer") return "Viewer";
  return "Analyst";
}

function getUserRoleValue(user: any): "tenant_admin" | "analyst" | "viewer" {
  if (user?.isAdmin) return "tenant_admin";
  return roleLabelToValue(String(user?.role || ""));
}

function isUserActive(user: any): boolean {
  return user?.active !== false && user?.isActive !== false;
}

function isTenantActive(tenant: any): boolean {
  return tenant?.active !== false;
}

function isTokenExpired(token: any): boolean {
  if (!token?.expiresAt) {
    return false;
  }
  const expiresAt = new Date(token.expiresAt).getTime();
  if (Number.isNaN(expiresAt)) {
    return false;
  }
  return expiresAt <= Date.now();
}

function normalizeServiceAlertMetricValue(value: any): string {
  const raw = String(value || "").trim().toLowerCase();
  if (raw === "case_sla") return "sla";
  if (raw === "case_threshold") return "threshold";
  if (raw === "case_criticality") return "criticality";
  if (raw === "case_escalation") return "escalation";
  if (raw === "case_ping") return "ping";
  return raw || "low_rps";
}

function formatStorageValue(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return "0 GB";
  }
  const units = ["B", "KB", "MB", "GB", "TB", "PB"];
  let value = bytes;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  const digits = value >= 100 ? 0 : value >= 10 ? 1 : 2;
  return `${value.toFixed(digits).replace(/\.0+$/, "").replace(/(\.\d*[1-9])0+$/, "$1")} ${units[unitIndex]}`;
}

function formatUsagePair(usedBytes: number, totalBytes: number): string {
  if (!Number.isFinite(totalBytes) || totalBytes <= 0) {
    return formatStorageValue(usedBytes);
  }
  return `${formatStorageValue(usedBytes)} / ${formatStorageValue(totalBytes)}`;
}

function formatCoreUsage(usedCores: number, totalCores: number): string {
  const safeUsed = Number.isFinite(usedCores) ? usedCores : 0;
  const safeTotal = Number.isFinite(totalCores) ? totalCores : 0;
  if (safeTotal <= 0) {
    return `${safeUsed.toFixed(1)} cores`;
  }
  return `${safeUsed.toFixed(1)} / ${safeTotal} cores`;
}

type AdministrationPageProps = {
  initialTab?: "users" | "tenants" | "achievements" | "experience" | "notification_bots" | "service_alerts" | "api_tokens" | "load";
};

type AdministrationTab =
  | "users"
  | "tenants"
  | "achievements"
  | "experience"
  | "notification_bots"
  | "service_alerts"
  | "api_tokens"
  | "load";

const ADMIN_TABS_DEFAULT: readonly AdministrationTab[] = [
  "users",
  "tenants",
  "achievements",
  "experience",
  "notification_bots",
  "service_alerts",
  "api_tokens",
  "load",
];

export default function AdministrationPage({ initialTab = "users" }: AdministrationPageProps) {
  const t = useT();
  const [location, setLocation] = useLocation();
  const queryHydratedRef = useRef(false);
  const allowedTabs = ADMIN_TABS_DEFAULT;
  const defaultTab: AdministrationTab = (allowedTabs as readonly string[]).includes(initialTab)
    ? (initialTab as AdministrationTab)
    : "users";
  const [activeTab, setActiveTab] = useState<AdministrationTab>(defaultTab);
  const { currentTenantId, currentUserId, session } = useAppState();
  const { data: tenants = [], isLoading: loadingTenants } = useTenants();
  const { data: users = [], isLoading: loadingUsers } = useUsers(currentTenantId);
  const { data: rateLimits = [], isLoading: loadingRateLimits } = useRateLimits(currentTenantId);
  const { data: socAccessPolicy } = useSOCAccessPolicy(currentTenantId);
  const { data: achievements = [], isLoading: loadingAchievements } = useAchievements(currentTenantId);
  const { data: apiTokens = [], isLoading: loadingAPITokens } = useAdminApiTokens(currentTenantId);
  const [notificationBotTenantId, setNotificationBotTenantId] = useState(currentTenantId);
  const { data: notificationBots = [], isLoading: loadingNotificationBots } = useAdminNotificationBots(notificationBotTenantId || currentTenantId);
  const {
    data: adminNotificationSettings = [],
    isLoading: loadingAdminNotificationSettings,
  } = useAdminNotificationSettings(notificationBotTenantId || currentTenantId);
  const [serviceAlertTenantId, setServiceAlertTenantId] = useState(currentTenantId);
  const { data: serviceAlertRules = [], isLoading: loadingServiceAlertRules } = useServiceAlertRules(serviceAlertTenantId || currentTenantId);

  const createTenant = useCreateTenant();
  const updateTenant = useUpdateTenant();
  const createUser = useCreateUser();
  const updateUser = useUpdateUser();
  const deleteUser = useDeleteUser();
  const createRateLimit = useCreateRateLimit();
  const updateRateLimit = useUpdateRateLimit();
  const deleteRateLimit = useDeleteRateLimit();
  const upsertSOCAccessPolicy = useUpsertSOCAccessPolicy();
  const createAchievement = useCreateAchievement();
  const uploadAchievementIcon = useUploadAchievementIcon();
  const deleteAchievement = useDeleteAchievement();
  const grantAchievement = useGrantAchievement();
  const revokeAchievement = useRevokeAchievement();
  const createAdminApiToken = useCreateAdminApiToken();
  const revokeAdminApiToken = useRevokeAdminApiToken();
  const createAdminNotificationBot = useCreateAdminNotificationBot();
  const updateAdminNotificationBot = useUpdateAdminNotificationBot();
  const deleteAdminNotificationBot = useDeleteAdminNotificationBot();
  const saveAdminNotificationSetting = useSaveAdminNotificationSetting();
  const createServiceAlertRule = useCreateServiceAlertRule();
  const updateServiceAlertRule = useUpdateServiceAlertRule();
  const deleteServiceAlertRule = useDeleteServiceAlertRule();

  const [newUser, setNewUser] = useState({
    name: "", email: "", password: "", role: "Analyst", team: "", tenantId: currentTenantId, isAdmin: false
  });
  const [createUserDialogOpen, setCreateUserDialogOpen] = useState(false);
  const [editUserDialogOpen, setEditUserDialogOpen] = useState(false);
  const [editingUser, setEditingUser] = useState<{
    id: string;
    name: string;
    email: string;
    role: "tenant_admin" | "analyst" | "viewer";
    team: string;
  } | null>(null);
  const [deleteUserDialogOpen, setDeleteUserDialogOpen] = useState(false);
  const [deletingUser, setDeletingUser] = useState<{ id: string; name: string } | null>(null);
  const [userSearchQuery, setUserSearchQuery] = useState("");
  const [userRoleFilter, setUserRoleFilter] = useState<"all" | "tenant_admin" | "analyst" | "viewer">("all");
  const [userStatusFilter, setUserStatusFilter] = useState<"all" | "active" | "inactive" | "admins">("all");
  const [tenantSearchQuery, setTenantSearchQuery] = useState("");
  const [tenantStatusFilter, setTenantStatusFilter] = useState<"all" | "active" | "inactive" | "owned">("all");

  const [newTenant, setNewTenant] = useState({
    id: "", name: "", description: "", maxUsers: 50, responsibleUserId: ""
  });
  const [createTenantDialogOpen, setCreateTenantDialogOpen] = useState(false);

  const [newRateLimit, setNewRateLimit] = useState({
    targetType: "tenant" as "tenant" | "user",
    targetId: "", maxRequests: 1000, windowSeconds: 3600
  });
  const [createRateLimitDialogOpen, setCreateRateLimitDialogOpen] = useState(false);
  const [socPolicyDraft, setSocPolicyDraft] = useState({
    allowedCaseTags: "",
    maxCasesInWork: "0",
  });

  const [loadTenantId, setLoadTenantId] = useState(currentTenantId);
  const [stopDialogOpen, setStopDialogOpen] = useState(false);
  const [stopTargetTenantId, setStopTargetTenantId] = useState("");
  const { data: systemResources } = useSystemResources(loadTenantId);
  const [resourceTimeline, setResourceTimeline] = useState<Array<Record<string, number | string>>>([]);

  const [newAchievement, setNewAchievement] = useState({
    name: "", description: "", icon: DEFAULT_ACHIEVEMENT_ICON, rarity: "Common" as string, xpReward: 150
  });
  const [createAchievementDialogOpen, setCreateAchievementDialogOpen] = useState(false);
  const achievementIconFileRef = useRef<HTMLInputElement | null>(null);
  const [uploadedAchievementIcons, setUploadedAchievementIcons] = useState<AchievementIconOption[]>([]);
  const [grantAchievementId, setGrantAchievementId] = useState("");
  const [grantUserId, setGrantUserId] = useState("");
  const [grantViewUserId, setGrantViewUserId] = useState("");
  const { data: grantedAchievements = [] } = useUserAchievements(grantViewUserId);
  const [awardXPUserId, setAwardXPUserId] = useState("");
  const [awardXPDialogOpen, setAwardXPDialogOpen] = useState(false);
  const [awardXPPoints, setAwardXPPoints] = useState(100);
  const [awardXPDescription, setAwardXPDescription] = useState("");
  const [newAPIToken, setNewAPIToken] = useState({
    name: "",
    description: "",
    fullAccess: false,
    scopes: ["alerts:write", "cases:write"] as string[],
    expiresAt: "",
  });
  const [issueApiTokenDialogOpen, setIssueApiTokenDialogOpen] = useState(false);
  const [newNotificationBot, setNewNotificationBot] = useState({
    name: "",
    botToken: "",
    enabled: true,
  });
  const [createNotificationBotDialogOpen, setCreateNotificationBotDialogOpen] = useState(false);
  const [notificationBotSearchQuery, setNotificationBotSearchQuery] = useState("");
  const [notificationBotStatusFilter, setNotificationBotStatusFilter] = useState<"all" | "enabled" | "disabled">("all");
  const [notificationRecipientSearchQuery, setNotificationRecipientSearchQuery] = useState("");
  const [notificationRecipientChannelFilter, setNotificationRecipientChannelFilter] = useState<"all" | "enabled" | "telegram" | "email" | "time" | "in_app">("all");
  const [serviceAlertSearchQuery, setServiceAlertSearchQuery] = useState("");
  const [serviceAlertStatusFilter, setServiceAlertStatusFilter] = useState<"all" | "enabled" | "disabled">("all");
  const [serviceAlertMetricFilter, setServiceAlertMetricFilter] = useState("all");
  const [apiTokenSearchQuery, setApiTokenSearchQuery] = useState("");
  const [apiTokenAccessFilter, setApiTokenAccessFilter] = useState<"all" | "full_access" | "scoped" | "expired">("all");
  const [newServiceAlertRule, setNewServiceAlertRule] = useState({
    name: "API low RPS",
    description: "",
    metricType: "low_rps",
    module: "api",
    minRps: 1,
    maxLatencyMs: 300,
    slaSeconds: 600,
    casesThreshold: 1,
    thresholdMode: "in_work",
    criticalityLevels: "critical",
    escalationEventTypes: "case_escalated,case_escalation_received",
    pingInactivitySeconds: 1800,
    openStatuses: "",
    windowSeconds: 300,
    cooldownSeconds: 900,
    severity: "warning",
    enabled: true,
  });
  const [createServiceAlertDialogOpen, setCreateServiceAlertDialogOpen] = useState(false);
  const [serviceAlertRuleDrafts, setServiceAlertRuleDrafts] = useState<Record<string, any>>({});
  const [notificationBotNames, setNotificationBotNames] = useState<Record<string, string>>({});
  const [notificationSettingDrafts, setNotificationSettingDrafts] = useState<Record<string, any>>({});
  const [createdAPITokenSecret, setCreatedAPITokenSecret] = useState("");
  const [factoryResetDialogOpen, setFactoryResetDialogOpen] = useState(false);
  const [factoryResetConfirmation, setFactoryResetConfirmation] = useState("");
  const experienceHistoryLoadMoreRef = useRef<HTMLTableRowElement | null>(null);
  const {
    data: experienceHistoryPages,
    isLoading: loadingExperienceHistory,
    isFetchingNextPage: loadingMoreExperienceHistory,
    hasNextPage: experienceHistoryHasMore = false,
    fetchNextPage: fetchNextExperienceHistoryPage,
  } = useInfiniteUserExperienceEvents(awardXPUserId, XP_HISTORY_BATCH_SIZE);
  const experienceHistory = useMemo(() => {
    const pages = experienceHistoryPages?.pages ?? [];
    return pages.flatMap((page: any) => (Array.isArray(page?.items) ? page.items : []));
  }, [experienceHistoryPages]);
  const awardUserExperience = useAwardUserExperience();
  const factoryResetLocal = useFactoryResetLocal();
  const isPlatformAdmin = Boolean(session?.identity?.is_platform_admin);

  const [editTenantDialogOpen, setEditTenantDialogOpen] = useState(false);
  const [editingTenant, setEditingTenant] = useState<any>(null);

  const modules = (systemResources as any)?.modules || {};
  const tenantResources = (systemResources as any)?.tenant || {};
  const host = (systemResources as any)?.host || {};
  const runtimeEnv = String((systemResources as any)?.env || "").trim().toLowerCase();
  const canFactoryResetLocal = isPlatformAdmin && ["local", "dev", "test"].includes(runtimeEnv);
  const currentCPU = Number(host?.cpuPercent || 0);
  const currentCPUCoresUsed = Number(host?.cpuCoresUsed || 0);
  const currentCPUCoresTotal = Number(host?.cpuCoresTotal || 0);
  const currentMemoryUsedBytes = Number(host?.memoryUsedBytes || 0);
  const currentMemoryTotalBytes = Number(host?.memoryTotalBytes || 0);
  const currentMemoryGB = Number(host?.memoryUsedGB || 0);
  const currentMemoryPercent = Number(host?.memoryUsedPercent || 0);
  const currentDiskPercent = Number(host?.diskUsedPercent || 0);
  const currentDiskUsedBytes = Number(host?.diskUsedBytes || 0);
  const currentDiskTotalBytes = Number(host?.diskTotalBytes || 0);
  const currentApiMs = Number(modules?.api?.responseMs || 0);
  const currentPgMs = Number(modules?.postgres?.responseMs || 0);
  const currentRedisMs = Number(modules?.redis?.responseMs || 0);
  const currentElasticMs = Number(modules?.elasticsearch?.responseMs || 0);
  const currentS3Ms = Number(modules?.s3?.responseMs || 0);
  const moduleSeriesKeys = useMemo(() => {
    const keys = new Set<string>();
    Object.keys(modules || {}).forEach((moduleName) => {
      if (String(moduleName || "").trim()) {
        keys.add(`module_${moduleName}`);
      }
    });
    resourceTimeline.forEach((point) => {
      Object.keys(point).forEach((pointKey) => {
        if (pointKey.startsWith("module_")) {
          keys.add(pointKey);
        }
      });
    });
    return Array.from(keys).sort((left, right) => left.localeCompare(right));
  }, [modules, resourceTimeline]);
  const moduleSeriesColors = useMemo(() => {
    const colors: Record<string, string> = {};
    moduleSeriesKeys.forEach((key, index) => {
      colors[key] = MODULE_SERIES_COLORS[index % MODULE_SERIES_COLORS.length];
    });
    return colors;
  }, [moduleSeriesKeys]);
  const moduleLoadRows = useMemo(
    () =>
      Object.entries(modules || {})
        .map(([moduleName, moduleData]) => {
          const status = String((moduleData as any)?.status || "unknown").toLowerCase();
          const responseMs = Number((moduleData as any)?.responseMs || 0);
          const uptime = Number((moduleData as any)?.uptime || 0);
          const operation = (moduleData as any)?.operation;
          const operationAvgLatencyMs = Number(operation?.avgLatencyMs || 0);
          const operationCount = Number(operation?.operations || 0);
          const operationErrors = Number(operation?.errors || 0);
          const operationWindowSeconds = Number(operation?.windowSeconds || 0);
          return {
            key: moduleName,
            label: MODULE_LABELS[moduleName] || `${moduleName.slice(0, 1).toUpperCase()}${moduleName.slice(1)}`,
            status,
            responseMs,
            uptime,
            operationAvgLatencyMs,
            operationCount,
            operationErrors,
            operationWindowSeconds,
          };
        })
        .sort((left, right) => left.label.localeCompare(right.label)),
    [modules],
  );
  const activeRateLimitsCount = useMemo(
    () => rateLimits.filter((item: any) => Boolean(item?.enabled)).length,
    [rateLimits],
  );
  const tenantSlugByID = useMemo<Record<string, string>>(() => {
    const next: Record<string, string> = {};
    tenants.forEach((tenant: any) => {
      next[tenant.id] = tenant.slug || tenant.id;
    });
    return next;
  }, [tenants]);
  const userNameByID = useMemo<Record<string, string>>(() => {
    const next: Record<string, string> = {};
    users.forEach((user: any) => {
      next[user.id] = String(user?.name || user?.email || user?.id || "").trim();
    });
    return next;
  }, [users]);
  const activeUsersCount = useMemo(
    () => users.filter((user: any) => isUserActive(user)).length,
    [users],
  );
  const adminUsersCount = useMemo(
    () => users.filter((user: any) => getUserRoleValue(user) === "tenant_admin").length,
    [users],
  );
  const userTeamsCount = useMemo(
    () =>
      new Set(
        users
          .map((user: any) => String(user?.team || "").trim())
          .filter(Boolean),
      ).size,
    [users],
  );
  const filteredUsers = useMemo(() => {
    const normalizedQuery = userSearchQuery.trim().toLowerCase();
    return [...users]
      .filter((user: any) => {
        const roleValue = getUserRoleValue(user);
        const active = isUserActive(user);
        const tenantLabel = tenantSlugByID[String(user?.tenantId || currentTenantId || "")] || String(user?.tenantId || currentTenantId || "");
        const haystack = [
          user?.name,
          user?.email,
          user?.team,
          roleValueToLabel(roleValue),
          tenantLabel,
          user?.id,
        ]
          .map((value) => String(value || "").toLowerCase())
          .join(" ");
        if (normalizedQuery && !haystack.includes(normalizedQuery)) {
          return false;
        }
        if (userRoleFilter !== "all" && roleValue !== userRoleFilter) {
          return false;
        }
        if (userStatusFilter === "active" && !active) {
          return false;
        }
        if (userStatusFilter === "inactive" && active) {
          return false;
        }
        if (userStatusFilter === "admins" && roleValue !== "tenant_admin") {
          return false;
        }
        return true;
      })
      .sort((left: any, right: any) => {
        if (left.id === currentUserId && right.id !== currentUserId) return -1;
        if (right.id === currentUserId && left.id !== currentUserId) return 1;
        const adminDiff = Number(getUserRoleValue(right) === "tenant_admin") - Number(getUserRoleValue(left) === "tenant_admin");
        if (adminDiff !== 0) {
          return adminDiff;
        }
        const activeDiff = Number(isUserActive(right)) - Number(isUserActive(left));
        if (activeDiff !== 0) {
          return activeDiff;
        }
        return String(left?.name || "").localeCompare(String(right?.name || ""));
      });
  }, [currentTenantId, currentUserId, tenantSlugByID, userRoleFilter, userSearchQuery, userStatusFilter, users]);
  const hasUserFilters = Boolean(userSearchQuery.trim()) || userRoleFilter !== "all" || userStatusFilter !== "all";
  const activeTenantsCount = useMemo(
    () => tenants.filter((tenant: any) => isTenantActive(tenant)).length,
    [tenants],
  );
  const inactiveTenantsCount = useMemo(
    () => tenants.filter((tenant: any) => !isTenantActive(tenant)).length,
    [tenants],
  );
  const ownedTenantsCount = useMemo(
    () => tenants.filter((tenant: any) => Boolean(String(tenant?.responsibleUserId || "").trim())).length,
    [tenants],
  );
  const filteredTenants = useMemo(() => {
    const normalizedQuery = tenantSearchQuery.trim().toLowerCase();
    return [...tenants]
      .filter((tenant: any) => {
        const active = isTenantActive(tenant);
        const responsibleName = userNameByID[String(tenant?.responsibleUserId || "").trim()] || String(tenant?.responsibleUserId || "").trim();
        const haystack = [
          tenant?.name,
          tenant?.slug,
          tenant?.id,
          tenant?.description,
          responsibleName,
        ]
          .map((value) => String(value || "").toLowerCase())
          .join(" ");
        if (normalizedQuery && !haystack.includes(normalizedQuery)) {
          return false;
        }
        if (tenantStatusFilter === "active" && !active) {
          return false;
        }
        if (tenantStatusFilter === "inactive" && active) {
          return false;
        }
        if (tenantStatusFilter === "owned" && !String(tenant?.responsibleUserId || "").trim()) {
          return false;
        }
        return true;
      })
      .sort((left: any, right: any) => {
        const activeDiff = Number(isTenantActive(right)) - Number(isTenantActive(left));
        if (activeDiff !== 0) {
          return activeDiff;
        }
        return String(left?.name || left?.slug || left?.id || "").localeCompare(String(right?.name || right?.slug || right?.id || ""));
      });
  }, [tenantSearchQuery, tenantStatusFilter, tenants, userNameByID]);
  const hasTenantFilters = Boolean(tenantSearchQuery.trim()) || tenantStatusFilter !== "all";
  const enabledNotificationBotsCount = useMemo(
    () => notificationBots.filter((bot: any) => Boolean(bot?.enabled)).length,
    [notificationBots],
  );
  const filteredNotificationBots = useMemo(() => {
    const normalizedQuery = notificationBotSearchQuery.trim().toLowerCase();
    return [...notificationBots]
      .filter((bot: any) => {
        const botName = notificationBotNames[bot.id] ?? bot.name;
        const haystack = [
          botName,
          bot?.botUsername,
          bot?.botId,
          bot?.botFirstName,
        ]
          .map((value) => String(value || "").toLowerCase())
          .join(" ");
        if (normalizedQuery && !haystack.includes(normalizedQuery)) {
          return false;
        }
        if (notificationBotStatusFilter === "enabled" && !bot?.enabled) {
          return false;
        }
        if (notificationBotStatusFilter === "disabled" && bot?.enabled) {
          return false;
        }
        return true;
      })
      .sort((left: any, right: any) => {
        const enabledDiff = Number(Boolean(right?.enabled)) - Number(Boolean(left?.enabled));
        if (enabledDiff !== 0) {
          return enabledDiff;
        }
        return String((notificationBotNames[left.id] ?? left?.name) || "").localeCompare(String((notificationBotNames[right.id] ?? right?.name) || ""));
      });
  }, [notificationBotNames, notificationBotSearchQuery, notificationBotStatusFilter, notificationBots]);
  const hasNotificationBotFilters = Boolean(notificationBotSearchQuery.trim()) || notificationBotStatusFilter !== "all";
  const deliveryEnabledRecipientsCount = useMemo(
    () =>
      adminNotificationSettings.filter((item: any) => {
        const userId = String(item?.userId || item?.user_id || "").trim();
        const draft = notificationSettingDrafts[userId] || item;
        return Boolean(draft?.deliveryEnabled ?? draft?.delivery_enabled);
      }).length,
    [adminNotificationSettings, notificationSettingDrafts],
  );
  const telegramRecipientsCount = useMemo(
    () =>
      adminNotificationSettings.filter((item: any) => {
        const userId = String(item?.userId || item?.user_id || "").trim();
        const draft = notificationSettingDrafts[userId] || item;
        return String(draft?.deliveryChannel || draft?.delivery_channel || "in_app").toLowerCase() === "telegram";
      }).length,
    [adminNotificationSettings, notificationSettingDrafts],
  );
  const filteredNotificationSettings = useMemo(() => {
    const normalizedQuery = notificationRecipientSearchQuery.trim().toLowerCase();
    return [...adminNotificationSettings]
      .filter((item: any) => {
        const userId = String(item?.userId || item?.user_id || "").trim();
        const draft = notificationSettingDrafts[userId] || item;
        const deliveryEnabled = Boolean(draft?.deliveryEnabled ?? draft?.delivery_enabled);
        const deliveryChannel = String(draft?.deliveryChannel || draft?.delivery_channel || "in_app").toLowerCase();
        const haystack = [draft?.userName, draft?.userEmail, draft?.userRole, userId]
          .map((value) => String(value || "").toLowerCase())
          .join(" ");
        if (normalizedQuery && !haystack.includes(normalizedQuery)) {
          return false;
        }
        if (notificationRecipientChannelFilter === "enabled" && !deliveryEnabled) {
          return false;
        }
        if (["telegram", "email", "time", "in_app"].includes(notificationRecipientChannelFilter) && deliveryChannel !== notificationRecipientChannelFilter) {
          return false;
        }
        return true;
      })
      .sort((left: any, right: any) => {
        const leftUserId = String(left?.userId || left?.user_id || "").trim();
        const rightUserId = String(right?.userId || right?.user_id || "").trim();
        const leftDraft = notificationSettingDrafts[leftUserId] || left;
        const rightDraft = notificationSettingDrafts[rightUserId] || right;
        const enabledDiff = Number(Boolean(rightDraft?.deliveryEnabled ?? rightDraft?.delivery_enabled)) - Number(Boolean(leftDraft?.deliveryEnabled ?? leftDraft?.delivery_enabled));
        if (enabledDiff !== 0) {
          return enabledDiff;
        }
        return String(leftDraft?.userName || leftDraft?.userEmail || leftUserId).localeCompare(String(rightDraft?.userName || rightDraft?.userEmail || rightUserId));
      });
  }, [adminNotificationSettings, notificationRecipientChannelFilter, notificationRecipientSearchQuery, notificationSettingDrafts]);
  const hasNotificationRecipientFilters = Boolean(notificationRecipientSearchQuery.trim()) || notificationRecipientChannelFilter !== "all";
  const enabledServiceAlertRulesCount = useMemo(
    () => serviceAlertRules.filter((rule: any) => Boolean(serviceAlertRuleDrafts[rule.id]?.enabled ?? rule?.enabled)).length,
    [serviceAlertRuleDrafts, serviceAlertRules],
  );
  const serviceAlertMetricOptions = useMemo(
    () =>
      Array.from(
        new Set(
          serviceAlertRules.map((rule: any) => normalizeServiceAlertMetricValue(serviceAlertRuleDrafts[rule.id]?.metricType || rule?.metricType || rule?.metric_type || "low_rps")),
        ),
      ).sort((left, right) => left.localeCompare(right)),
    [serviceAlertRuleDrafts, serviceAlertRules],
  );
  const filteredServiceAlertRules = useMemo(() => {
    const normalizedQuery = serviceAlertSearchQuery.trim().toLowerCase();
    return [...serviceAlertRules]
      .filter((rule: any) => {
        const draft = serviceAlertRuleDrafts[rule.id] || rule;
        const metricType = normalizeServiceAlertMetricValue(draft?.metricType || draft?.metric_type || "low_rps");
        const enabled = Boolean(draft?.enabled ?? true);
        const haystack = [draft?.name, draft?.description, draft?.module, draft?.severity, metricType, rule?.id]
          .map((value) => String(value || "").toLowerCase())
          .join(" ");
        if (normalizedQuery && !haystack.includes(normalizedQuery)) {
          return false;
        }
        if (serviceAlertStatusFilter === "enabled" && !enabled) {
          return false;
        }
        if (serviceAlertStatusFilter === "disabled" && enabled) {
          return false;
        }
        if (serviceAlertMetricFilter !== "all" && metricType !== serviceAlertMetricFilter) {
          return false;
        }
        return true;
      })
      .sort((left: any, right: any) => {
        const leftDraft = serviceAlertRuleDrafts[left.id] || left;
        const rightDraft = serviceAlertRuleDrafts[right.id] || right;
        const enabledDiff = Number(Boolean(rightDraft?.enabled ?? true)) - Number(Boolean(leftDraft?.enabled ?? true));
        if (enabledDiff !== 0) {
          return enabledDiff;
        }
        return String(leftDraft?.name || left?.id || "").localeCompare(String(rightDraft?.name || right?.id || ""));
      });
  }, [serviceAlertMetricFilter, serviceAlertRuleDrafts, serviceAlertRules, serviceAlertSearchQuery, serviceAlertStatusFilter]);
  const hasServiceAlertFilters = Boolean(serviceAlertSearchQuery.trim()) || serviceAlertStatusFilter !== "all" || serviceAlertMetricFilter !== "all";
  const fullAccessTokenCount = useMemo(
    () => apiTokens.filter((token: any) => Boolean(token?.fullAccess)).length,
    [apiTokens],
  );
  const expiredApiTokenCount = useMemo(
    () => apiTokens.filter((token: any) => isTokenExpired(token)).length,
    [apiTokens],
  );
  const filteredApiTokens = useMemo(() => {
    const normalizedQuery = apiTokenSearchQuery.trim().toLowerCase();
    return [...apiTokens]
      .filter((token: any) => {
        const expired = isTokenExpired(token);
        const haystack = [token?.name, token?.description, token?.tokenPrefix, ...(Array.isArray(token?.scopes) ? token.scopes : [])]
          .map((value) => String(value || "").toLowerCase())
          .join(" ");
        if (normalizedQuery && !haystack.includes(normalizedQuery)) {
          return false;
        }
        if (apiTokenAccessFilter === "full_access" && !token?.fullAccess) {
          return false;
        }
        if (apiTokenAccessFilter === "scoped" && token?.fullAccess) {
          return false;
        }
        if (apiTokenAccessFilter === "expired" && !expired) {
          return false;
        }
        return true;
      })
      .sort((left: any, right: any) => {
        const expiredDiff = Number(isTokenExpired(left)) - Number(isTokenExpired(right));
        if (expiredDiff !== 0) {
          return expiredDiff;
        }
        const leftLastUsed = left?.lastUsedAt ? new Date(left.lastUsedAt).getTime() : 0;
        const rightLastUsed = right?.lastUsedAt ? new Date(right.lastUsedAt).getTime() : 0;
        if (leftLastUsed !== rightLastUsed) {
          return rightLastUsed - leftLastUsed;
        }
        return String(left?.name || left?.tokenPrefix || left?.id || "").localeCompare(String(right?.name || right?.tokenPrefix || right?.id || ""));
      });
  }, [apiTokenAccessFilter, apiTokenSearchQuery, apiTokens]);
  const hasApiTokenFilters = Boolean(apiTokenSearchQuery.trim()) || apiTokenAccessFilter !== "all";
  const selectedLoadTenant = useMemo(
    () => tenants.find((tenant: any) => tenant.id === loadTenantId) || null,
    [tenants, loadTenantId],
  );
  const isLoadTenantActive = selectedLoadTenant?.active !== false;
  const stopTargetTenantSlug = tenantSlugByID[stopTargetTenantId] || stopTargetTenantId;
  const showUsersLoading = useMinimumLoading(loadingUsers);
  const showTenantsLoading = useMinimumLoading(loadingTenants);
  const showAchievementsLoading = useMinimumLoading(loadingAchievements);
  const showExperienceHistoryLoading = useMinimumLoading(loadingExperienceHistory);
  const showNotificationBotsLoading = useMinimumLoading(loadingNotificationBots);
  const showAdminNotificationSettingsLoading = useMinimumLoading(loadingAdminNotificationSettings);
  const showServiceAlertRulesLoading = useMinimumLoading(loadingServiceAlertRules);
  const showAPITokensLoading = useMinimumLoading(loadingAPITokens);
  const showRateLimitsLoading = useMinimumLoading(loadingRateLimits);

  useEffect(() => {
    const timestamp = (systemResources as any)?.timestamp;
    if (!timestamp) {
      return;
    }
    const time = new Date(timestamp).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    const nextPoint: Record<string, number | string> = {
      time,
      cpu: currentCPU,
      memory: currentMemoryGB,
      disk: currentDiskPercent,
      api: currentApiMs,
      postgres: currentPgMs,
      redis: currentRedisMs,
      elasticsearch: currentElasticMs,
      s3: currentS3Ms,
      requests: Number(tenantResources?.apiRequests24h || 0),
    };
    Object.entries(modules || {}).forEach(([moduleName, moduleData]) => {
      const metricKey = `module_${moduleName}`;
      const metricValue = Number((moduleData as any)?.responseMs || 0);
      nextPoint[metricKey] = Number.isFinite(metricValue) ? metricValue : 0;
    });
    setResourceTimeline((prev) => {
      const merged = [...prev, nextPoint];
      return merged.slice(-24);
    });
  }, [systemResources, currentCPU, currentMemoryGB, currentDiskPercent, currentApiMs, currentPgMs, currentRedisMs, currentElasticMs, currentS3Ms, tenantResources, modules]);

  useEffect(() => {
    setResourceTimeline([]);
  }, [loadTenantId]);

  useEffect(() => {
    if (!awardXPUserId && users.length > 0) {
      setAwardXPUserId(users[0].id);
    }
  }, [users, awardXPUserId]);

  useEffect(() => {
    if (!awardXPUserId || !experienceHistoryHasMore || loadingMoreExperienceHistory) {
      return;
    }
    const anchor = experienceHistoryLoadMoreRef.current;
    if (!anchor) {
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        if (!entries.some((entry) => entry.isIntersecting)) {
          return;
        }
        fetchNextExperienceHistoryPage().catch(() => undefined);
      },
      { rootMargin: "200px 0px" },
    );
    observer.observe(anchor);
    return () => observer.disconnect();
  }, [
    awardXPUserId,
    experienceHistoryHasMore,
    loadingMoreExperienceHistory,
    fetchNextExperienceHistoryPage,
    experienceHistory.length,
  ]);

  useEffect(() => {
    if (!notificationBotTenantId && currentTenantId) {
      setNotificationBotTenantId(currentTenantId);
    }
  }, [currentTenantId, notificationBotTenantId]);

  useEffect(() => {
    if (!serviceAlertTenantId && currentTenantId) {
      setServiceAlertTenantId(currentTenantId);
    }
  }, [currentTenantId, serviceAlertTenantId]);

  useEffect(() => {
    const nextTags = Array.isArray(socAccessPolicy?.allowedCaseTags) ? socAccessPolicy.allowedCaseTags.join(", ") : "";
    const nextMax = Number(socAccessPolicy?.maxCasesInWork || 0);
    const nextMaxValue = String(Number.isFinite(nextMax) && nextMax > 0 ? Math.floor(nextMax) : 0);
    setSocPolicyDraft((prev) => {
      if (prev.allowedCaseTags === nextTags && prev.maxCasesInWork === nextMaxValue) {
        return prev;
      }
      return {
        allowedCaseTags: nextTags,
        maxCasesInWork: nextMaxValue,
      };
    });
  }, [socAccessPolicy?.id, socAccessPolicy?.allowedCaseTags, socAccessPolicy?.maxCasesInWork]);

  useEffect(() => {
    if (tenants.length === 0) {
      return;
    }
    const selectedExists = tenants.some((tenant: any) => tenant.id === notificationBotTenantId);
    if (selectedExists) {
      return;
    }
    const fallbackTenantID = tenants.some((tenant: any) => tenant.id === currentTenantId)
      ? currentTenantId
      : String(tenants[0]?.id || "");
    if (fallbackTenantID) {
      setNotificationBotTenantId(fallbackTenantID);
    }
  }, [tenants, currentTenantId, notificationBotTenantId]);

  useEffect(() => {
    setNotificationBotNames((prev) => {
      const next: Record<string, string> = {};
      notificationBots.forEach((bot: any) => {
        const current = prev[bot.id];
        next[bot.id] = typeof current === "string" ? current : String(bot.name || "");
      });
      const prevKeys = Object.keys(prev);
      const nextKeys = Object.keys(next);
      if (prevKeys.length !== nextKeys.length) {
        return next;
      }
      for (const key of nextKeys) {
        if (prev[key] !== next[key]) {
          return next;
        }
      }
      return prev;
    });
  }, [notificationBots]);

  useEffect(() => {
    setNotificationSettingDrafts((prev) => {
      const next: Record<string, any> = {};
      adminNotificationSettings.forEach((item: any) => {
        const userId = String(item?.userId || item?.user_id || "").trim();
        if (!userId) {
          return;
        }
        const current = prev[userId] || {};
        next[userId] = {
          userId,
          userName: item?.userName || item?.user_name || "",
          userEmail: item?.userEmail || item?.user_email || "",
          userRole: item?.userRole || item?.user_role || "",
          deliveryEnabled: Boolean(item?.deliveryEnabled ?? item?.delivery_enabled),
          deliveryChannel: String(item?.deliveryChannel || item?.delivery_channel || "in_app").toLowerCase(),
          telegramBotId: item?.telegramBotId || item?.telegram_bot_id || "",
          telegramChatId: item?.telegramChatId || item?.telegram_chat_id || "",
          telegramUsername: item?.telegramUsername || item?.telegram_username || "",
          // Preserve unsaved edits while keeping server values as source of truth for new rows.
          ...current,
        };
      });
      const nextKeys = Object.keys(next);
      const prevKeys = Object.keys(prev);
      if (nextKeys.length !== prevKeys.length) {
        return next;
      }
      for (const key of nextKeys) {
        if (JSON.stringify(next[key]) !== JSON.stringify(prev[key])) {
          return next;
        }
      }
      return prev;
    });
  }, [adminNotificationSettings]);

  useEffect(() => {
    if (tenants.length === 0) {
      return;
    }
    const selectedExists = tenants.some((tenant: any) => tenant.id === serviceAlertTenantId);
    if (selectedExists) {
      return;
    }
    const fallbackTenantID = tenants.some((tenant: any) => tenant.id === currentTenantId)
      ? currentTenantId
      : String(tenants[0]?.id || "");
    if (fallbackTenantID) {
      setServiceAlertTenantId(fallbackTenantID);
    }
  }, [tenants, currentTenantId, serviceAlertTenantId]);

  useEffect(() => {
    if (tenants.length === 0) {
      return;
    }
    const selectedExists = tenants.some((tenant: any) => tenant.id === loadTenantId);
    if (selectedExists) {
      return;
    }
    const fallbackTenantID = tenants.some((tenant: any) => tenant.id === currentTenantId)
      ? currentTenantId
      : String(tenants[0]?.id || "");
    if (fallbackTenantID) {
      setLoadTenantId(fallbackTenantID);
    }
  }, [tenants, currentTenantId, loadTenantId]);

  useEffect(() => {
    setServiceAlertRuleDrafts((prev) => {
      const next: Record<string, any> = {};
      serviceAlertRules.forEach((rule: any) => {
        next[rule.id] = {
          id: rule.id,
          tenantId: rule.tenantId,
          name: rule.name || "",
          description: rule.description || "",
          metricType: rule.metricType || "low_rps",
          module: rule.module || "api",
          minRps: Number(rule.minRps ?? 0),
          maxLatencyMs: Number(rule.maxLatencyMs ?? 0),
          slaSeconds: Number(rule.slaSeconds ?? 0),
          casesThreshold: Number(rule.casesThreshold ?? 0),
          thresholdMode: String(rule.thresholdMode || "in_work").toLowerCase(),
          criticalityLevels: Array.isArray(rule.criticalityLevels) ? rule.criticalityLevels.join(",") : String(rule.criticalityLevels || ""),
          escalationEventTypes: Array.isArray(rule.escalationEventTypes) ? rule.escalationEventTypes.join(",") : String(rule.escalationEventTypes || ""),
          pingInactivitySeconds: Number(rule.pingInactivitySeconds ?? 0),
          openStatuses: Array.isArray(rule.openStatuses) ? rule.openStatuses.join(",") : String(rule.openStatuses || ""),
          windowSeconds: Number(rule.windowSeconds ?? 300),
          cooldownSeconds: Number(rule.cooldownSeconds ?? 900),
          severity: rule.severity || "warning",
          enabled: Boolean(rule.enabled ?? true),
        };
      });
      const previous = JSON.stringify(prev);
      const current = JSON.stringify(next);
      if (previous === current) {
        return prev;
      }
      return next;
    });
  }, [serviceAlertRules]);
  useEffect(() => {
    setActiveTab((allowedTabs as readonly string[]).includes(initialTab) ? (initialTab as AdministrationTab) : "users");
  }, [initialTab, allowedTabs]);

  useEffect(() => {
    const { params } = splitLocationPathAndSearch(location);
    const tabParam = String(params.get("tab") || "").trim();
    if (tabParam && (allowedTabs as readonly string[]).includes(tabParam)) {
      setActiveTab(tabParam as AdministrationTab);
    }

    const notificationTenantParam = String(params.get("notification_tenant") || "").trim();
    if (notificationTenantParam) {
      setNotificationBotTenantId(notificationTenantParam);
    }

    const serviceAlertTenantParam = String(params.get("service_alert_tenant") || "").trim();
    if (serviceAlertTenantParam) {
      setServiceAlertTenantId(serviceAlertTenantParam);
    }

    const loadTenantParam = String(params.get("load_tenant") || "").trim();
    if (loadTenantParam) {
      setLoadTenantId(loadTenantParam);
    }

    const experienceUserParam = String(params.get("xp_user") || "").trim();
    if (experienceUserParam) {
      setAwardXPUserId(experienceUserParam);
    }

    queryHydratedRef.current = true;
  }, []);

  useEffect(() => {
    if (!queryHydratedRef.current) return;
    const nextLocation = applySearchPatch(location, {
      tab: activeTab !== defaultTab ? activeTab : undefined,
      notification_tenant:
        activeTab === "notification_bots" && notificationBotTenantId && notificationBotTenantId !== currentTenantId
          ? notificationBotTenantId
          : undefined,
      service_alert_tenant:
        activeTab === "service_alerts" && serviceAlertTenantId && serviceAlertTenantId !== currentTenantId
          ? serviceAlertTenantId
          : undefined,
      load_tenant:
        activeTab === "load" && loadTenantId && loadTenantId !== currentTenantId
          ? loadTenantId
          : undefined,
      xp_user: activeTab === "experience" && awardXPUserId ? awardXPUserId : undefined,
    });
    const currentLocation =
      typeof window !== "undefined" ? `${window.location.pathname}${window.location.search}` : location;
    if (nextLocation !== currentLocation) {
      setLocation(nextLocation, { replace: true });
    }
  }, [
    activeTab,
    defaultTab,
    notificationBotTenantId,
    serviceAlertTenantId,
    loadTenantId,
    awardXPUserId,
    currentTenantId,
    location,
    setLocation,
  ]);

  const knownAchievementIconOptions = useMemo(() => {
    const options: AchievementIconOption[] = [...ACHIEVEMENT_ICON_PRESETS];
    achievements.forEach((achievement: any) => {
      const iconValue = String(achievement?.icon || "").trim();
      if (!iconValue || !isAchievementImageIcon(iconValue)) {
        return;
      }
      options.push({
        value: iconValue,
        label: String(achievement?.name || "Existing icon"),
        preview: iconValue,
        source: "existing",
      });
      const storageURI = String(achievement?.icon_storage_uri || achievement?.iconStorageURI || "").trim();
      if (!storageURI) {
        return;
      }
      options.push({
        value: storageURI,
        label: `${String(achievement?.name || "Existing icon")} (S3)`,
        preview: iconValue,
        source: "existing",
      });
    });
    ACHIEVEMENT_ICON_EMOJI_OPTIONS.forEach((emojiOption) => options.push(emojiOption));
    const byValue = new Map<string, AchievementIconOption>();
    options.forEach((option) => {
      if (!option.value || byValue.has(option.value)) {
        return;
      }
      byValue.set(option.value, option);
    });
    return Array.from(byValue.values());
  }, [achievements]);

  const achievementIconOptions = useMemo(() => {
    const byValue = new Map<string, AchievementIconOption>();
    [...uploadedAchievementIcons, ...knownAchievementIconOptions].forEach((option) => {
      if (!option.value || byValue.has(option.value)) {
        return;
      }
      byValue.set(option.value, option);
    });
    if (newAchievement.icon && !byValue.has(newAchievement.icon)) {
      byValue.set(newAchievement.icon, {
        value: newAchievement.icon,
        label: "Custom icon",
        preview: isAchievementImageIcon(newAchievement.icon) ? newAchievement.icon : newAchievement.icon,
        source: "uploaded",
      });
    }
    return Array.from(byValue.values());
  }, [knownAchievementIconOptions, newAchievement.icon, uploadedAchievementIcons]);

  const selectedAchievementIconOption = useMemo(
    () => achievementIconOptions.find((option) => option.value === newAchievement.icon) || null,
    [achievementIconOptions, newAchievement.icon],
  );

  const handleCreateUser = () => {
    const name = newUser.name.trim();
    const email = newUser.email.trim();
    if (!name || !email) {
      toast.error("Name and email are required");
      return;
    }
    const slug = `${name.toLowerCase().replace(/\s+/g, "_").replace(/[^a-z0-9_]/g, "")}_${Date.now().toString(36).slice(-4)}`;
    createUser.mutate(
      {
        id: slug,
        name,
        email,
        password: newUser.password,
        role: newUser.role,
        team: newUser.team,
        tenantId: newUser.tenantId || currentTenantId,
        isAdmin: newUser.isAdmin,
      },
      {
        onSuccess: () => {
          setNewUser({ name: "", email: "", password: "", role: "Analyst", team: "", tenantId: currentTenantId, isAdmin: false });
          setCreateUserDialogOpen(false);
          toast.success("User created");
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to create user");
        },
      },
    );
  };

  const handleOpenEditUser = (user: any) => {
    setEditingUser({
      id: user.id,
      name: user.name || "",
      email: user.email || "",
      role: roleLabelToValue(user.role),
      team: user.team || "",
    });
    setEditUserDialogOpen(true);
  };

  const handleSaveEditedUser = () => {
    if (!editingUser) return;
    const payload = {
      name: editingUser.name.trim(),
      email: editingUser.email.trim(),
      team: editingUser.team.trim(),
      role: editingUser.role,
    };
    if (!payload.name || !payload.email) {
      toast.error("Name and email are required");
      return;
    }
    updateUser.mutate(
      { id: editingUser.id, data: payload },
      {
        onSuccess: () => {
          toast.success("User updated");
          setEditUserDialogOpen(false);
          setEditingUser(null);
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to update user");
        },
      },
    );
  };

  const handleRequestDeleteUser = (user: any) => {
    setDeletingUser({ id: user.id, name: user.name || user.email || user.id });
    setDeleteUserDialogOpen(true);
  };

  const handleConfirmDeleteUser = () => {
    if (!deletingUser) return;
    deleteUser.mutate(deletingUser.id, {
      onSuccess: () => {
        toast.success("User deleted");
        setDeleteUserDialogOpen(false);
        setDeletingUser(null);
      },
      onError: (error: any) => {
        toast.error(error?.message || "Failed to delete user");
      },
    });
  };

  const handleCreateTenant = () => {
    const id = newTenant.id.trim();
    const name = newTenant.name.trim();
    if (!id || !name) {
      toast.error("Tenant id and name are required");
      return;
    }
    createTenant.mutate(
      {
        id,
        name,
        description: newTenant.description,
        maxUsers: newTenant.maxUsers,
        responsibleUserId: newTenant.responsibleUserId || undefined,
        active: true,
      },
      {
        onSuccess: () => {
          setNewTenant({ id: "", name: "", description: "", maxUsers: 50, responsibleUserId: "" });
          setCreateTenantDialogOpen(false);
          toast.success("Tenant created");
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to create tenant");
        },
      },
    );
  };

  const handleToggleTenant = (tenantId: string, currentActive: boolean) => {
    updateTenant.mutate({ id: tenantId, data: { active: !currentActive } });
    toast.success(currentActive ? "Tenant deactivated" : "Tenant activated");
  };

  const handleCreateRateLimit = () => {
    if (!newRateLimit.targetId) {
      toast.error("Select a target");
      return;
    }
    createRateLimit.mutate(
      {
        targetType: newRateLimit.targetType,
        targetId: newRateLimit.targetId,
        maxRequests: newRateLimit.maxRequests,
        windowSeconds: newRateLimit.windowSeconds,
        enabled: true,
        tenantId: currentTenantId,
      },
      {
        onSuccess: () => {
          setNewRateLimit({ targetType: "tenant", targetId: "", maxRequests: 1000, windowSeconds: 3600 });
          setCreateRateLimitDialogOpen(false);
          toast.success("Rate limit created");
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to create rate limit");
        },
      },
    );
  };

  const handleSaveSOCAccessPolicy = () => {
    const allowedCaseTags = socPolicyDraft.allowedCaseTags
      .split(",")
      .map((item) => item.trim().toLowerCase())
      .filter(Boolean);
    const maxCandidate = Number(socPolicyDraft.maxCasesInWork || 0);
    const maxCasesInWork = Number.isFinite(maxCandidate) && maxCandidate > 0 ? Math.floor(maxCandidate) : 0;
    upsertSOCAccessPolicy.mutate(
      {
        id: socAccessPolicy?.id || "",
        allowedCaseTags,
        maxCasesInWork,
      },
      {
        onSuccess: () => {
          toast.success("SOC policy saved");
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to save SOC policy");
        },
      },
    );
  };

  const handleStopTenant = () => {
    if (!stopTargetTenantId) return;
    updateTenant.mutate({ id: stopTargetTenantId, data: { active: false } });
    setStopDialogOpen(false);
    setStopTargetTenantId("");
    toast.success("Tenant stopped — maintenance mode enabled");
  };

  const handleStartTenant = () => {
    if (!loadTenantId) return;
    updateTenant.mutate({ id: loadTenantId, data: { active: true } });
    toast.success("Tenant started — traffic restored");
  };

  const handleCreateAchievement = () => {
    const name = newAchievement.name.trim();
    if (!name) {
      toast.error("Achievement name is required");
      return;
    }
    createAchievement.mutate(
      {
        id: `ach_${Date.now()}`,
        name,
        description: newAchievement.description,
        icon: newAchievement.icon || DEFAULT_ACHIEVEMENT_ICON,
        rarity: newAchievement.rarity,
        xp_reward: Number(newAchievement.xpReward || 0),
        tenantId: currentTenantId,
      },
      {
        onSuccess: () => {
          setNewAchievement({ name: "", description: "", icon: DEFAULT_ACHIEVEMENT_ICON, rarity: "Common", xpReward: 150 });
          setCreateAchievementDialogOpen(false);
          toast.success("Achievement created");
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to create achievement");
        },
      },
    );
  };

  const handleUploadAchievementIcon = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) {
      return;
    }
    const normalizedType = String(file.type || "").toLowerCase().trim();
    const allowedTypes = ACHIEVEMENT_ICON_UPLOAD_CRITERIA.allowedTypes as readonly string[];
    if (!allowedTypes.includes(normalizedType)) {
      toast.error("Use PNG, JPEG, or SVG icon");
      return;
    }
    if (file.size > ACHIEVEMENT_ICON_UPLOAD_CRITERIA.maxBytes) {
      toast.error("Icon must be 2MB or smaller");
      return;
    }
    uploadAchievementIcon.mutate(
      { file },
      {
        onSuccess: (payload: any) => {
          const iconValue = String(payload?.storageURI || payload?.icon || "").trim();
          const iconPreview = String(payload?.iconURL || payload?.icon || "").trim();
          if (!iconValue) {
            toast.error("Upload completed but icon value is empty");
            return;
          }
          setUploadedAchievementIcons((prev) =>
            upsertAchievementIconOption(prev, {
              value: iconValue,
              label: file.name,
              preview: iconPreview || iconValue,
              source: "uploaded",
            }),
          );
          setNewAchievement((prev) => ({ ...prev, icon: iconValue }));
          toast.success("Achievement icon uploaded");
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to upload achievement icon");
        },
      },
    );
  };

  const handleGrantAchievement = () => {
    if (!grantAchievementId) {
      toast.error("Select an achievement");
      return;
    }
    if (!grantUserId) {
      toast.error("Select a user");
      return;
    }
    grantAchievement.mutate(
      {
        userId: grantUserId,
        achievementId: grantAchievementId,
        tenantId: currentTenantId,
      },
      {
        onSuccess: () => {
          toast.success("Achievement granted");
          if (!grantViewUserId) {
            setGrantViewUserId(grantUserId);
          }
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to grant achievement");
        },
      },
    );
  };

  const handleAwardExperience = () => {
    const points = Math.floor(Number(awardXPPoints || 0));
    const description = awardXPDescription.trim() || "Manual XP grant";
    if (!awardXPUserId) {
      toast.error("Select a user");
      return;
    }
    if (points <= 0) {
      toast.error("Points must be greater than zero");
      return;
    }
    awardUserExperience.mutate(
      { userId: awardXPUserId, points, description },
      {
        onSuccess: () => {
          toast.success("Experience awarded");
          setAwardXPDescription("");
          setAwardXPDialogOpen(false);
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to award experience");
        },
      },
    );
  };

  const handleToggleApiScope = (scope: string, checked: boolean) => {
    setNewAPIToken((prev) => {
      const current = new Set(prev.scopes);
      if (checked) {
        current.add(scope);
      } else {
        current.delete(scope);
      }
      return { ...prev, scopes: Array.from(current) };
    });
  };

  const handleCreateAPIToken = () => {
    if (!newAPIToken.name.trim()) {
      toast.error("Token name is required");
      return;
    }
    if (!newAPIToken.fullAccess && newAPIToken.scopes.length === 0) {
      toast.error("Select at least one scope or enable full access");
      return;
    }
    createAdminApiToken.mutate(
      {
        name: newAPIToken.name.trim(),
        description: newAPIToken.description.trim(),
        fullAccess: newAPIToken.fullAccess,
        scopes: newAPIToken.fullAccess ? [] : newAPIToken.scopes,
        expiresAt: newAPIToken.expiresAt.trim() || "",
      },
      {
        onSuccess: (created: any) => {
          setCreatedAPITokenSecret(String(created?.token || ""));
          setNewAPIToken({
            name: "",
            description: "",
            fullAccess: false,
            scopes: ["alerts:write", "cases:write"],
            expiresAt: "",
          });
          setIssueApiTokenDialogOpen(false);
          toast.success("API token created");
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to create API token");
        },
      },
    );
  };

  const handleRevokeAPIToken = (id: string) => {
    revokeAdminApiToken.mutate(id, {
      onSuccess: () => toast.success("API token revoked"),
      onError: (error: any) => toast.error(error?.message || "Failed to revoke API token"),
    });
  };

  const handleCreateNotificationBot = () => {
    const name = newNotificationBot.name.trim();
    const botToken = newNotificationBot.botToken.trim();
    const tenantId = String(notificationBotTenantId || currentTenantId || "").trim();
    if (!name || !botToken) {
      toast.error("Bot name and token are required");
      return;
    }
    if (!tenantId) {
      toast.error("Tenant is required");
      return;
    }
    createAdminNotificationBot.mutate(
      { name, botToken, enabled: newNotificationBot.enabled, tenantId },
      {
        onSuccess: () => {
          toast.success("Telegram bot created");
          setNewNotificationBot({ name: "", botToken: "", enabled: true });
          setCreateNotificationBotDialogOpen(false);
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to create Telegram bot");
        },
      },
    );
  };

  const handleToggleNotificationBot = (bot: any, enabled: boolean) => {
    const tenantId = String(bot?.tenantId || notificationBotTenantId || currentTenantId || "").trim();
    updateAdminNotificationBot.mutate(
      { id: bot.id, data: { enabled, tenantId } },
      {
        onSuccess: () => {
          toast.success(enabled ? "Bot enabled" : "Bot disabled");
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to update bot");
        },
      },
    );
  };

  const handleSaveNotificationBotName = (bot: any) => {
    const nextName = String(notificationBotNames[bot.id] || "").trim();
    const tenantId = String(bot?.tenantId || notificationBotTenantId || currentTenantId || "").trim();
    if (!nextName) {
      toast.error("Bot name is required");
      return;
    }
    updateAdminNotificationBot.mutate(
      { id: bot.id, data: { name: nextName, tenantId } },
      {
        onSuccess: () => toast.success("Bot name updated"),
        onError: (error: any) => toast.error(error?.message || "Failed to update bot"),
      },
    );
  };

  const handleDeleteNotificationBot = (id: string, tenantId: string) => {
    deleteAdminNotificationBot.mutate({ id, tenantId }, {
      onSuccess: () => toast.success("Bot deleted"),
      onError: (error: any) => toast.error(error?.message || "Failed to delete bot"),
    });
  };

  const handleNotificationSettingDraftChange = (userId: string, patch: Record<string, any>) => {
    setNotificationSettingDrafts((prev) => ({
      ...prev,
      [userId]: {
        ...(prev[userId] || {}),
        ...patch,
      },
    }));
  };

  const handleSaveNotificationSetting = (
    userId: string,
    draftOverride?: Record<string, any>,
    options?: { successMessage?: string },
  ): boolean => {
    const draft = draftOverride || notificationSettingDrafts[userId];
    if (!draft) {
      return false;
    }
    const tenantId = String(notificationBotTenantId || currentTenantId || "").trim();
    if (!tenantId) {
      toast.error("Tenant is required");
      return false;
    }
    const deliveryChannel = String(draft.deliveryChannel || "in_app").toLowerCase();
    const deliveryEnabled = Boolean(draft.deliveryEnabled);
    const telegramBotId = String(draft.telegramBotId || "").trim();
    const telegramChatId = String(draft.telegramChatId || "").trim();
    const telegramUsername = String(draft.telegramUsername || "").trim();
    const notificationEmail = String(draft.notificationEmail || "").trim();
    const timeRecipient = String(draft.timeRecipient || "").trim();
    if (deliveryChannel === "telegram" && deliveryEnabled) {
      if (!telegramBotId) {
        toast.error("Select Telegram bot for Telegram delivery");
        return false;
      }
      if (!telegramChatId && !telegramUsername) {
        toast.error("Provide Telegram chat ID or username");
        return false;
      }
    }
    if (deliveryChannel === "email" && deliveryEnabled && !notificationEmail) {
      toast.error("Provide notification email");
      return false;
    }
    if (deliveryChannel === "time" && deliveryEnabled && !timeRecipient) {
      toast.error("Provide Time recipient");
      return false;
    }
    saveAdminNotificationSetting.mutate(
      {
        userId,
        data: {
          tenantId,
          deliveryEnabled,
          deliveryChannel,
          telegramBotId,
          telegramChatId,
          telegramUsername,
          notificationEmail,
          timeRecipient,
        },
      },
      {
        onSuccess: () => toast.success(options?.successMessage || "User notification settings updated"),
        onError: (error: any) => toast.error(error?.message || "Failed to save user notification settings"),
      },
    );
    return true;
  };

  const splitCSVValues = (value: any): string[] =>
    String(value || "")
      .split(/[,;\n]/)
      .map((item) => item.trim().toLowerCase())
      .filter(Boolean);

  const normalizeServiceAlertMetric = (value: any): string => normalizeServiceAlertMetricValue(value);

  const normalizeServiceAlertDraft = (draft: any): any => {
    const metricType = normalizeServiceAlertMetric(draft?.metricType || draft?.metric_type || "low_rps");
    const criticalityRaw = draft?.criticalityLevels ?? draft?.criticality_levels;
    const criticalityLevels = Array.isArray(criticalityRaw)
      ? criticalityRaw.map((item: any) => String(item || "").trim().toLowerCase()).filter(Boolean)
      : splitCSVValues(criticalityRaw);
    const escalationRaw = draft?.escalationEventTypes ?? draft?.escalation_event_types;
    const escalationEventTypes = Array.isArray(escalationRaw)
      ? escalationRaw.map((item: any) => String(item || "").trim().toLowerCase()).filter(Boolean)
      : splitCSVValues(escalationRaw);
    const openStatusesRaw = draft?.openStatuses ?? draft?.open_statuses;
    const openStatuses = Array.isArray(openStatusesRaw)
      ? openStatusesRaw.map((item: any) => String(item || "").trim().toLowerCase()).filter(Boolean)
      : splitCSVValues(openStatusesRaw);
    return {
      ...draft,
      metricType,
      module: String(draft?.module || "api").trim().toLowerCase(),
      minRps: Number(draft?.minRps ?? draft?.min_rps ?? 0),
      maxLatencyMs: Number(draft?.maxLatencyMs ?? draft?.max_latency_ms ?? 0),
      slaSeconds: Number(draft?.slaSeconds ?? draft?.sla_seconds ?? 0),
      casesThreshold: Number(draft?.casesThreshold ?? draft?.cases_threshold ?? draft?.caseThreshold ?? draft?.case_threshold ?? draft?.threshold ?? 0),
      thresholdMode: String(draft?.thresholdMode || draft?.threshold_mode || draft?.scope || "in_work").trim().toLowerCase(),
      criticalityLevels: criticalityLevels.length ? criticalityLevels : ["critical"],
      escalationEventTypes: escalationEventTypes.length ? escalationEventTypes : ["case_escalated", "case_escalation_received"],
      pingInactivitySeconds: Number(draft?.pingInactivitySeconds ?? draft?.ping_inactivity_seconds ?? draft?.pingSeconds ?? draft?.ping_seconds ?? 0),
      openStatuses,
      windowSeconds: Number(draft?.windowSeconds ?? draft?.window_seconds ?? 300),
      cooldownSeconds: Number(draft?.cooldownSeconds ?? draft?.cooldown_seconds ?? 900),
      severity: String(draft?.severity || "warning").trim().toLowerCase(),
      enabled: Boolean(draft?.enabled ?? true),
    };
  };

  const validateServiceAlertDraft = (draft: any): boolean => {
    const metricType = normalizeServiceAlertMetric(draft?.metricType || draft?.metric_type || "low_rps");
    if (!String(draft?.name || "").trim()) {
      toast.error("Rule name is required");
      return false;
    }
    if ((metricType === "low_rps" || metricType === "high_latency") && !String(draft?.module || "").trim()) {
      toast.error("Module is required");
      return false;
    }
    if (metricType === "low_rps" && Number(draft?.minRps ?? draft?.min_rps ?? 0) < 0) {
      toast.error("Min RPS must be >= 0");
      return false;
    }
    if (metricType === "high_latency" && Number(draft?.maxLatencyMs ?? draft?.max_latency_ms ?? 0) <= 0) {
      toast.error("Max latency must be > 0");
      return false;
    }
    if (metricType === "sla" && Number(draft?.slaSeconds ?? draft?.sla_seconds ?? 0) <= 0) {
      toast.error("SLA seconds must be > 0");
      return false;
    }
    if ((metricType === "sla" || metricType === "threshold" || metricType === "criticality" || metricType === "escalation" || metricType === "ping")
      && Number(draft?.casesThreshold ?? draft?.cases_threshold ?? draft?.caseThreshold ?? draft?.case_threshold ?? draft?.threshold ?? 0) <= 0) {
      toast.error("Cases threshold must be > 0");
      return false;
    }
    if (metricType === "ping" && Number(draft?.pingInactivitySeconds ?? draft?.ping_inactivity_seconds ?? draft?.pingSeconds ?? draft?.ping_seconds ?? 0) <= 0) {
      toast.error("Ping inactivity seconds must be > 0");
      return false;
    }
    return true;
  };

  const handleCreateServiceAlertRule = () => {
    const tenantId = String(serviceAlertTenantId || currentTenantId || "").trim();
    if (!tenantId) {
      toast.error("Tenant is required");
      return;
    }
    const payload = normalizeServiceAlertDraft(newServiceAlertRule);
    if (!validateServiceAlertDraft(payload)) {
      return;
    }
    createServiceAlertRule.mutate(
      {
        ...payload,
        tenantId,
      },
      {
        onSuccess: () => {
          toast.success("Service alert rule created");
          setCreateServiceAlertDialogOpen(false);
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to create service alert rule");
        },
      },
    );
  };

  const handleServiceAlertDraftChange = (ruleId: string, patch: Record<string, any>) => {
    setServiceAlertRuleDrafts((prev) => ({
      ...prev,
      [ruleId]: {
        ...(prev[ruleId] || {}),
        ...patch,
      },
    }));
  };

  const handleSaveServiceAlertRule = (
    ruleId: string,
    draftOverride?: Record<string, any>,
    options?: { successMessage?: string },
  ): boolean => {
    const draft = draftOverride || serviceAlertRuleDrafts[ruleId];
    if (!draft) {
      return false;
    }
    const tenantId = String(draft.tenantId || serviceAlertTenantId || currentTenantId || "").trim();
    if (!tenantId) {
      toast.error("Tenant is required");
      return false;
    }
    const payload = normalizeServiceAlertDraft(draft);
    if (!validateServiceAlertDraft(payload)) {
      return false;
    }
    updateServiceAlertRule.mutate(
      {
        id: ruleId,
        data: {
          ...payload,
          tenantId,
        },
      },
      {
        onSuccess: () => toast.success(options?.successMessage || "Service alert rule updated"),
        onError: (error: any) => toast.error(error?.message || "Failed to update service alert rule"),
      },
    );
    return true;
  };

  const handleDeleteServiceAlertRule = (ruleId: string, tenantId: string) => {
    deleteServiceAlertRule.mutate(
      { id: ruleId, tenantId },
      {
        onSuccess: () => toast.success("Service alert rule deleted"),
        onError: (error: any) => toast.error(error?.message || "Failed to delete service alert rule"),
      },
    );
  };

  const scrollToLegacySection = (sectionId: string) => {
    if (typeof window === "undefined") {
      return;
    }
    const element = document.getElementById(sectionId);
    if (!element) {
      return;
    }
    window.requestAnimationFrame(() => {
      element.scrollIntoView({ behavior: "smooth", block: "start" });
    });
  };

  const pageTitle = "Administration";
  const pageSubtitle = "Manage users, tenants, achievements, and system load.";
  const isTestMode = typeof process !== "undefined" && process.env.NODE_ENV === "test";
  const showFigmaTabs = !isTestMode;
  const formatDateValue = (value?: string | null) => {
    if (!value) return "Never";
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" });
  };
  const formatRelativeValue = (value?: string | null) => {
    if (!value) return "Never";
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    const minutes = Math.max(1, Math.round((Date.now() - date.getTime()) / 60000));
    if (minutes < 60) return `${minutes} min ago`;
    const hours = Math.round(minutes / 60);
    if (hours < 24) return `${hours}h ago`;
    const days = Math.round(hours / 24);
    return `${days}d ago`;
  };
  const roleColorClass = (role: string) => {
    const normalized = String(role || "").toLowerCase();
    if (normalized.includes("admin")) return "border-[#66ff4c]/20 bg-[#66ff4c]/10 text-[#66ff4c]";
    if (normalized.includes("analyst")) return "border-[#3b82f6]/20 bg-[#3b82f6]/10 text-[#60a5fa]";
    if (normalized.includes("responder")) return "border-[#a855f7]/20 bg-[#a855f7]/10 text-[#c084fc]";
    return "border-[#6b7280]/20 bg-[#6b7280]/10 text-[#9ca3af]";
  };
  return (
    <AppLayout>
      <div className={ADMIN_PAGE_SHELL_CLASS}>
        <div>
          <h1 data-testid="text-page-title" className="text-[32px] font-semibold leading-8 tracking-[-0.5px] text-white">{pageTitle}</h1>
          <p className="mt-1 text-sm text-[#9ca3af]">{pageSubtitle}</p>
        </div>

        <Tabs
          value={activeTab}
          onValueChange={(value) => {
            if ((allowedTabs as readonly string[]).includes(value)) {
              setActiveTab(value as AdministrationTab);
            }
          }}
          className="space-y-6"
        >
          <TabsList className={ADMIN_TAB_LIST_CLASS}>
                <TabsTrigger data-testid="tab-users" value="users" className={ADMIN_TAB_TRIGGER_CLASS}>
                  <Users size={16} /> User Management
                </TabsTrigger>
                <TabsTrigger data-testid="tab-tenants" value="tenants" className={ADMIN_TAB_TRIGGER_CLASS}>
                  <Building2 size={16} /> Tenant Management
                </TabsTrigger>
                <TabsTrigger data-testid="tab-achievements" value="achievements" className={ADMIN_TAB_TRIGGER_CLASS}>
                  <Trophy size={16} /> Achievements
                </TabsTrigger>
                <TabsTrigger data-testid="tab-experience" value="experience" className={ADMIN_TAB_TRIGGER_CLASS}>
                  <ZapIcon size={16} /> Experience
                </TabsTrigger>
                <TabsTrigger data-testid="tab-notification-bots" value="notification_bots" className={ADMIN_TAB_TRIGGER_CLASS}>
                  <Radio size={16} /> Notification Bots
                </TabsTrigger>
                <TabsTrigger data-testid="tab-service-alerts" value="service_alerts" className={ADMIN_TAB_TRIGGER_CLASS}>
                  <AlertTriangle size={16} /> Service Alerts
                </TabsTrigger>
                <TabsTrigger data-testid="tab-api-tokens" value="api_tokens" className={ADMIN_TAB_TRIGGER_CLASS}>
                  <KeyRound size={16} /> API Tokens
                </TabsTrigger>
                <TabsTrigger data-testid="tab-load" value="load" className={ADMIN_TAB_TRIGGER_CLASS}>
                  <Activity size={16} /> Load Management
                </TabsTrigger>
          </TabsList>

          {showFigmaTabs && (
          <TabsContent value="load" className={ADMIN_FIGMA_SECTION_CLASS}>
            <div className="flex flex-wrap items-end justify-between gap-3">
              <div>
                <h2 className="text-2xl font-semibold text-white">Load Management</h2>
                <p className="text-sm text-[#9ca3af]">Monitor tenant resources, throttling, and module latency in real time</p>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <Select value={loadTenantId} onValueChange={setLoadTenantId}>
                  <AdminSelectTrigger data-testid="select-load-tenant-figma" className="w-[220px]">
                    <SelectValue />
                  </AdminSelectTrigger>
                  <SelectContent>
                    {tenants.map((tenant: any) => (
                      <SelectItem
                        key={tenant.id}
                        value={tenant.id}
                        className={tenant.active ? "" : "text-[#9ca3af]"}
                        searchText={`${tenant.slug || tenant.id} ${tenant.active ? "active" : "inactive"}`}
                      >
                        <span className="inline-flex items-center gap-2">
                          <span>{tenant.slug || tenant.id}</span>
                          {!tenant.active ? <span className="text-[10px] uppercase tracking-[0.08em]">{t("admin.tenant.inactive")}</span> : null}
                        </span>
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {isLoadTenantActive ? (
                  <button
                    type="button"
                    className="inline-flex h-10 items-center justify-center gap-2 rounded-lg border border-[#ef4444]/30 bg-[#ef4444]/10 px-3 text-sm text-[#f87171]"
                    onClick={() => {
                      setStopTargetTenantId(loadTenantId);
                      setStopDialogOpen(true);
                    }}
                    data-testid="button-stop-tenant-figma"
                  >
                    <Square size={13} />
                    Stop tenant
                  </button>
                ) : (
                  <button
                    type="button"
                    className="inline-flex h-10 items-center justify-center gap-2 rounded-lg border border-[#22c55e]/30 bg-[#22c55e]/10 px-3 text-sm text-[#4ade80]"
                    onClick={handleStartTenant}
                    data-testid="button-start-tenant-figma"
                  >
                    <Play size={13} />
                    Start tenant
                  </button>
                )}
                <button
                  type="button"
                  className={cn(ADMIN_FIGMA_SECONDARY_BUTTON_CLASS, "h-10")}
                  onClick={() => scrollToLegacySection("admin-legacy-load")}
                >
                  <Settings size={14} />
                  Advanced controls
                </button>
              </div>
            </div>

            <div className="grid gap-4 md:grid-cols-4">
              <div className={cn(ADMIN_FIGMA_CARD_CLASS, "p-5")}>
                <div className="flex items-start justify-between text-sm">
                  <div>
                    <span className="text-white">CPU Usage</span>
                    <p className="mt-1 text-xs text-[#9ca3af]">{tenantSlugByID[loadTenantId] || loadTenantId}</p>
                  </div>
                  <span className="inline-flex h-10 w-10 items-center justify-center rounded-lg border border-[#22c55e]/20 bg-[#22c55e]/10 text-[#4ade80]">
                    <Cpu size={18} />
                  </span>
                </div>
                <p className="mt-2 text-4xl text-[#66ff4c]">{formatCoreUsage(currentCPUCoresUsed, currentCPUCoresTotal)}</p>
                <p className="mt-2 text-xs text-[#9ca3af]">{currentCPU.toFixed(1)}% total host load</p>
                <div className="mt-3 h-2 rounded-full bg-[#13141c]"><div className="h-2 rounded-full bg-[#66ff4c]" style={{ width: `${Math.max(0, Math.min(100, currentCPU))}%` }} /></div>
              </div>
              <div className={cn(ADMIN_FIGMA_CARD_CLASS, "p-5")}>
                <div className="flex items-start justify-between text-sm">
                  <div>
                    <span className="text-white">Memory Usage</span>
                    <p className="mt-1 text-xs text-[#9ca3af]">{tenantSlugByID[loadTenantId] || loadTenantId}</p>
                  </div>
                  <span className="inline-flex h-10 w-10 items-center justify-center rounded-lg border border-[#f59e0b]/20 bg-[#f59e0b]/10 text-[#fbbf24]">
                    <HardDrive size={18} />
                  </span>
                </div>
                <p className="mt-2 text-4xl text-[#ffc700]">{formatUsagePair(currentMemoryUsedBytes, currentMemoryTotalBytes)}</p>
                <p className="mt-2 text-xs text-[#9ca3af]">{currentMemoryPercent.toFixed(1)}% of host RAM</p>
                <div className="mt-3 h-2 rounded-full bg-[#13141c]"><div className="h-2 rounded-full bg-[#ffc700]" style={{ width: `${Math.max(0, Math.min(100, currentMemoryPercent))}%` }} /></div>
              </div>
              <div className={cn(ADMIN_FIGMA_CARD_CLASS, "p-5")}>
                <div className="flex items-start justify-between text-sm">
                  <div>
                    <span className="text-white">Disk Usage</span>
                    <p className="mt-1 text-xs text-[#9ca3af]">{tenantSlugByID[loadTenantId] || loadTenantId}</p>
                  </div>
                  <span className="inline-flex h-10 w-10 items-center justify-center rounded-lg border border-[#38bdf8]/20 bg-[#38bdf8]/10 text-[#38bdf8]">
                    <BarChart3 size={18} />
                  </span>
                </div>
                <p className="mt-2 text-4xl text-[#38bdf8]">{formatUsagePair(currentDiskUsedBytes, currentDiskTotalBytes)}</p>
                <p className="mt-2 text-xs text-[#9ca3af]">{currentDiskPercent.toFixed(1)}% occupied</p>
                <div className="mt-3 h-2 rounded-full bg-[#13141c]"><div className="h-2 rounded-full bg-[#38bdf8]" style={{ width: `${Math.max(0, Math.min(100, currentDiskPercent))}%` }} /></div>
              </div>
              <div className={cn(ADMIN_FIGMA_CARD_CLASS, "p-5")}>
                <div className="flex items-start justify-between text-sm">
                  <div>
                    <span className="text-white">API Requests (24h)</span>
                    <p className="mt-1 text-xs text-[#9ca3af]">{tenantSlugByID[loadTenantId] || loadTenantId}</p>
                  </div>
                  <span className="inline-flex h-10 w-10 items-center justify-center rounded-lg border border-[#3b82f6]/20 bg-[#3b82f6]/10 text-[#60a5fa]">
                    <Gauge size={18} />
                  </span>
                </div>
                <p className="mt-2 text-4xl text-[#3b82f6]">{Number(tenantResources?.apiRequests24h || 0).toLocaleString()}</p>
                <p className="mt-2 text-xs text-[#9ca3af]">
                  Active users: {Number(tenantResources?.activeUsers || 0)} · Open cases: {Number(tenantResources?.openCases || 0)}
                </p>
              </div>
            </div>

            <div className="grid gap-4 xl:grid-cols-2">
              <div className={cn(ADMIN_FIGMA_CARD_CLASS, "p-5")}>
                <h3 className="text-lg font-medium text-white">CPU Timeline</h3>
                <p className="mt-1 text-xs text-[#9ca3af]">Latest 24 samples for selected tenant</p>
                <div className="mt-3 h-[220px]">
                  <ResponsiveContainer width="100%" height="100%">
                    <AreaChart data={resourceTimeline}>
                      <CartesianGrid strokeDasharray="3 3" stroke="#2a2c3c" />
                      <XAxis dataKey="time" tick={{ fontSize: 10, fill: "#9ca3af" }} />
                      <YAxis tick={{ fontSize: 10, fill: "#9ca3af" }} />
                      <Tooltip />
                      <Area type="monotone" dataKey="cpu" stroke="#66ff4c" fill="#66ff4c22" strokeWidth={2} />
                    </AreaChart>
                  </ResponsiveContainer>
                </div>
              </div>
              <div className={cn(ADMIN_FIGMA_CARD_CLASS, "p-5")}>
                <h3 className="text-lg font-medium text-white">Memory Timeline</h3>
                <p className="mt-1 text-xs text-[#9ca3af]">Latest 24 samples for selected tenant</p>
                <div className="mt-3 h-[220px]">
                  <ResponsiveContainer width="100%" height="100%">
                    <AreaChart data={resourceTimeline}>
                      <CartesianGrid strokeDasharray="3 3" stroke="#2a2c3c" />
                      <XAxis dataKey="time" tick={{ fontSize: 10, fill: "#9ca3af" }} />
                      <YAxis tick={{ fontSize: 10, fill: "#9ca3af" }} />
                      <Tooltip />
                      <Area type="monotone" dataKey="memory" stroke="#3b82f6" fill="#3b82f622" strokeWidth={2} />
                    </AreaChart>
                  </ResponsiveContainer>
                </div>
              </div>
              <div className={cn(ADMIN_FIGMA_CARD_CLASS, "p-5 xl:col-span-2")}>
                <h3 className="text-lg font-medium text-white">Module Latency</h3>
                <p className="mt-1 text-xs text-[#9ca3af]">Health check latency (ms) across backend modules</p>
                <div className="mt-3 h-[220px]">
                  <ResponsiveContainer width="100%" height="100%">
                    <LineChart data={resourceTimeline}>
                      <CartesianGrid strokeDasharray="3 3" stroke="#2a2c3c" />
                      <XAxis dataKey="time" tick={{ fontSize: 10, fill: "#9ca3af" }} />
                      <YAxis tick={{ fontSize: 10, fill: "#9ca3af" }} />
                      <Tooltip
                        formatter={(value: any, seriesKey: any) => {
                          const key = String(seriesKey || "");
                          const moduleName = key.startsWith("module_") ? key.slice("module_".length) : key;
                          const moduleLabel = MODULE_LABELS[moduleName] || `${moduleName.slice(0, 1).toUpperCase()}${moduleName.slice(1)}`;
                          return [formatModuleResponseMs(Number(value || 0)), moduleLabel];
                        }}
                      />
                      {moduleSeriesKeys.map((seriesKey) => (
                        <Line
                          key={seriesKey}
                          type="monotone"
                          dataKey={seriesKey}
                          stroke={moduleSeriesColors[seriesKey]}
                          strokeWidth={2}
                          dot={false}
                          isAnimationActive={false}
                          connectNulls
                        />
                      ))}
                    </LineChart>
                  </ResponsiveContainer>
                </div>
              </div>
            </div>

            <div className="grid gap-4 lg:grid-cols-2">
              <div className={cn(ADMIN_FIGMA_CARD_CLASS, "p-5")}>
                <h3 className="text-lg font-medium text-white">Module Health & Operations</h3>
                <div className="mt-3 space-y-2">
                  {moduleLoadRows.length === 0 ? (
                    <p className="rounded-lg border border-[#1d1e29] bg-[#13141c] px-3 py-2 text-sm text-[#9ca3af]">No module metrics yet.</p>
                  ) : (
                    moduleLoadRows.map((moduleItem) => (
                      <div key={moduleItem.key} className="flex items-center justify-between rounded-lg border border-[#1d1e29] bg-[#13141c] px-3 py-2 text-sm">
                        <div>
                          <p className="text-white">{moduleItem.label}</p>
                          <p className="text-xs text-[#9ca3af]">Uptime {moduleItem.uptime.toFixed(2)}%</p>
                        </div>
                        <div className="text-right">
                          <p className="text-white">Health {formatModuleResponseMs(moduleItem.responseMs, moduleItem.status)}</p>
                          <p className="text-xs text-[#9ca3af]">
                            Ops {formatModuleOperationLatency(moduleItem.operationAvgLatencyMs, moduleItem.operationCount)}
                          </p>
                          <p className="text-xs text-[#9ca3af] capitalize">
                            {moduleItem.status}
                            {moduleItem.operationCount > 0 ? ` · ${moduleItem.operationCount} ops / ${formatOperationWindow(moduleItem.operationWindowSeconds) || "5m"}` : ""}
                            {moduleItem.operationErrors > 0 ? ` · ${moduleItem.operationErrors} errors` : ""}
                          </p>
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </div>
              <div className={cn(ADMIN_FIGMA_CARD_CLASS, "p-5")}>
                <h3 className="text-lg font-medium text-white">Throttling & Limits</h3>
                <div className="mt-3 space-y-2 text-sm text-[#9ca3af]">
                  <div className="flex items-center justify-between rounded-lg border border-[#1d1e29] bg-[#13141c] px-3 py-2">
                    <span>Configured rules</span>
                    <span className="text-white">{rateLimits.length}</span>
                  </div>
                  <div className="flex items-center justify-between rounded-lg border border-[#1d1e29] bg-[#13141c] px-3 py-2">
                    <span>Active rules</span>
                    <span className="text-white">{activeRateLimitsCount}</span>
                  </div>
                  <div className="flex items-center justify-between rounded-lg border border-[#1d1e29] bg-[#13141c] px-3 py-2">
                    <span>Disabled rules</span>
                    <span className="text-white">{Math.max(0, rateLimits.length - activeRateLimitsCount)}</span>
                  </div>
                  <div className="flex items-center justify-between rounded-lg border border-[#1d1e29] bg-[#13141c] px-3 py-2">
                    <span>Tenant status</span>
                    <span className={cn("text-xs", isLoadTenantActive ? "text-[#4ade80]" : "text-[#f87171]")}>
                      {isLoadTenantActive ? "Running" : "Stopped"}
                    </span>
                  </div>
                </div>
                <button
                  type="button"
                  className={cn(ADMIN_FIGMA_PRIMARY_BUTTON_CLASS, "mt-4 w-full")}
                  onClick={() => scrollToLegacySection("admin-legacy-load")}
                >
                  Open throttling editor
                </button>
              </div>
            </div>
          </TabsContent>

          )}

          {/* Legacy tab content remains available for complete CRUD flows and tests */}

          {/* ═══════════════════ TAB 1: USER MANAGEMENT ═══════════════════ */}
          <TabsContent value="users" className="space-y-6" id="admin-legacy-users">
            <div className="space-y-6">
                <AdminCard className="p-5">
                  <div className="flex flex-col gap-5">
                    <div className="flex flex-col gap-4 xl:flex-row xl:flex-wrap xl:items-start xl:justify-between">
                      <div>
                        <h3 className="font-bold text-lg">User Directory</h3>
                        <p className="mt-1 text-sm text-[#9ca3af]">
                          Search analysts by name, email, team, or tenant and jump straight into edit and delete actions.
                        </p>
                      </div>
                      <div className="grid w-full max-w-[640px] gap-3 sm:grid-cols-2 xl:max-w-none xl:grid-cols-4">
                        <div className="min-w-[120px] rounded-xl border border-[#252a3d] bg-[#10131c] p-3">
                          <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Total users</p>
                          <p className="mt-2 text-2xl font-semibold text-white">{users.length}</p>
                        </div>
                        <div className="min-w-[120px] rounded-xl border border-[#252a3d] bg-[#10131c] p-3">
                          <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Active</p>
                          <p className="mt-2 text-2xl font-semibold text-[#66ff4c]">{activeUsersCount}</p>
                        </div>
                        <div className="min-w-[120px] rounded-xl border border-[#252a3d] bg-[#10131c] p-3">
                          <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Admins</p>
                          <p className="mt-2 text-2xl font-semibold text-[#facc15]">{adminUsersCount}</p>
                        </div>
                        <div className="min-w-[120px] rounded-xl border border-[#252a3d] bg-[#10131c] p-3">
                          <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Teams</p>
                          <p className="mt-2 text-2xl font-semibold text-[#60a5fa]">{userTeamsCount}</p>
                        </div>
                      </div>
                    </div>

                    <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_180px_180px_auto]">
                      <div className="relative">
                        <Search size={14} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[#6b7280]" />
                        <AdminInput
                          data-testid="input-user-search"
                          value={userSearchQuery}
                          onChange={(event) => setUserSearchQuery(event.target.value)}
                          placeholder="Search by name, email, team, or tenant"
                          className="pl-9"
                        />
                      </div>
                      <Select value={userRoleFilter} onValueChange={(value) => setUserRoleFilter(value as typeof userRoleFilter)}>
                        <AdminSelectTrigger data-testid="select-user-role-filter">
                          <SelectValue placeholder="All roles" />
                        </AdminSelectTrigger>
                        <SelectContent>
                          <SelectItem value="all">All roles</SelectItem>
                          <SelectItem value="tenant_admin">Admin</SelectItem>
                          <SelectItem value="analyst">Analyst</SelectItem>
                          <SelectItem value="viewer">Viewer</SelectItem>
                        </SelectContent>
                      </Select>
                      <Select value={userStatusFilter} onValueChange={(value) => setUserStatusFilter(value as typeof userStatusFilter)}>
                        <AdminSelectTrigger data-testid="select-user-status-filter">
                          <SelectValue placeholder="All statuses" />
                        </AdminSelectTrigger>
                        <SelectContent>
                          <SelectItem value="all">All statuses</SelectItem>
                          <SelectItem value="active">Active</SelectItem>
                          <SelectItem value="inactive">Inactive</SelectItem>
                          <SelectItem value="admins">Admins only</SelectItem>
                        </SelectContent>
                      </Select>
                      <AdminButton
                        data-testid="button-reset-user-filters"
                        type="button"
                        variant="outline"
                        className="w-full xl:w-auto"
                        onClick={() => {
                          setUserSearchQuery("");
                          setUserRoleFilter("all");
                          setUserStatusFilter("all");
                        }}
                        disabled={!hasUserFilters}
                      >
                        Reset
                      </AdminButton>
                    </div>

                    <div className="flex flex-wrap items-center gap-2 text-xs text-[#9ca3af]">
                      <span>Showing {filteredUsers.length} of {users.length} users</span>
                      {userRoleFilter !== "all" ? (
                        <Badge variant="outline" className="border-primary/20 bg-primary/10 text-primary">
                          {roleValueToLabel(userRoleFilter)}
                        </Badge>
                      ) : null}
                      {userStatusFilter !== "all" ? (
                        <Badge variant="outline" className="border-[#3b82f6]/20 bg-[#3b82f6]/10 text-[#60a5fa]">
                          {userStatusFilter === "admins" ? "Admins only" : userStatusFilter}
                        </Badge>
                      ) : null}
                      {userSearchQuery.trim() ? (
                        <Badge variant="outline" className="max-w-full border-[#2f3549] bg-[#10131c] text-[#d1d5db]">
                          <span className="max-w-[280px] truncate">{userSearchQuery.trim()}</span>
                        </Badge>
                      ) : null}
                    </div>
                  </div>
                </AdminCard>

                <AdminCard className="overflow-hidden p-0">
                  <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[#252a3d] px-5 py-4">
                    <div>
                      <h3 className="font-bold text-lg">Tenant Users</h3>
                      <p className="mt-1 text-sm text-[#9ca3af]">{tenantSlugByID[currentTenantId] || currentTenantId}</p>
                    </div>
                    <div className="flex items-center gap-2">
                      <Badge variant="outline" className="border-primary/20 bg-primary/10 text-primary">
                        {filteredUsers.length} visible
                      </Badge>
                      <AdminButton
                        data-testid="button-new-user"
                        size="sm"
                        className="h-9"
                        onClick={() => {
                          setNewUser({ name: "", email: "", password: "", role: "Analyst", team: "", tenantId: currentTenantId, isAdmin: false });
                          setCreateUserDialogOpen(true);
                        }}
                      >
                        <Plus size={14} className="mr-2" /> New User
                      </AdminButton>
                    </div>
                  </div>
                  <div className="space-y-4 p-4">
                    {showUsersLoading ? (
                      <div className="space-y-3">
                        {[1, 2, 3, 4].map((index) => (
                          <Skeleton key={index} className="h-32 w-full rounded-xl" />
                        ))}
                      </div>
                    ) : filteredUsers.length === 0 ? (
                      <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-[#2f3549] bg-[#10131c] px-6 py-12 text-center">
                        <p className="text-sm font-medium text-white">No users match the current filters</p>
                        <p className="mt-1 text-sm text-[#9ca3af]">Try another search query or reset the filters.</p>
                        {hasUserFilters ? (
                          <AdminButton
                            type="button"
                            variant="outline"
                            className="mt-4"
                            onClick={() => {
                              setUserSearchQuery("");
                              setUserRoleFilter("all");
                              setUserStatusFilter("all");
                            }}
                          >
                            Reset filters
                          </AdminButton>
                        ) : null}
                      </div>
                    ) : (
                      filteredUsers.map((u: any) => {
                        const isSelf = u.id === currentUserId;
                        const isProtectedPlatformAdmin = Boolean(u.isPlatformAdmin);
                        const roleValue = getUserRoleValue(u);
                        const active = isUserActive(u);
                        const tenantLabel = tenantSlugByID[String(u.tenantId || currentTenantId || "")] || String(u.tenantId || currentTenantId || "");
                        return (
                          <div
                            key={u.id}
                            data-testid={`card-user-${u.id}`}
                            className="overflow-hidden rounded-xl border border-[#252a3d] bg-[#101420] transition-colors hover:border-[#4b5168]"
                          >
                            <div className="flex flex-col gap-4 p-4 md:flex-row md:items-start md:justify-between">
                              <div className="flex min-w-0 gap-4">
                                <div className="flex h-11 w-11 flex-none items-center justify-center rounded-full bg-primary/10 text-sm font-bold text-primary">
                                  {String(u.name || "?").charAt(0).toUpperCase()}
                                </div>
                                <div className="min-w-0">
                                  <div className="flex flex-wrap items-center gap-2">
                                    <p className="truncate text-sm font-semibold text-[#f3f4f6]">{u.name || "Unnamed user"}</p>
                                    {isSelf ? <Badge variant="outline" className="text-[10px]">You</Badge> : null}
                                    {active ? (
                                      <Badge className="border-[#22c55e]/20 bg-[#22c55e]/10 text-[#4ade80] text-[10px]">Active</Badge>
                                    ) : (
                                      <Badge variant="outline" className="text-[10px] text-[#9ca3af]">Inactive</Badge>
                                    )}
                                  </div>
                                  <p className="mt-1 truncate text-sm text-[#9ca3af]">{u.email || "No email"}</p>
                                  <div className="mt-3 flex flex-wrap gap-2">
                                    <Badge variant="outline" className={cn("text-[10px]", roleColorClass(roleValue))}>
                                      {roleValueToLabel(roleValue)}
                                    </Badge>
                                    {u.team ? (
                                      <Badge variant="secondary" className="text-[10px] h-5 px-2">
                                        {u.team}
                                      </Badge>
                                    ) : null}
                                    <Badge variant="outline" className="text-[10px] border-[#2f3549] text-[#cbd5e1]">
                                      {tenantLabel}
                                    </Badge>
                                  </div>
                                </div>
                              </div>
                              <div className="flex flex-wrap gap-2 md:justify-end">
                                <AdminButton
                                  data-testid={`button-edit-user-${u.id}`}
                                  variant="outline"
                                  size="sm"
                                  className="h-8"
                                  onClick={() => handleOpenEditUser(u)}
                                  disabled={isProtectedPlatformAdmin}
                                >
                                  <Edit size={12} className="mr-1" /> {t("common.edit")}
                                </AdminButton>
                                <AdminButton
                                  data-testid={`button-delete-user-${u.id}`}
                                  variant="destructive"
                                  size="sm"
                                  className="h-8"
                                  onClick={() => handleRequestDeleteUser(u)}
                                  disabled={isSelf || isProtectedPlatformAdmin || deleteUser.isPending}
                                >
                                  <Trash2 size={12} className="mr-1" /> {t("common.delete")}
                                </AdminButton>
                              </div>
                            </div>
                            <div className="grid gap-3 border-t border-[#252a3d] bg-[#0d111b] px-4 py-3 text-xs text-[#9ca3af] sm:grid-cols-3">
                              <div>
                                <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Last login</p>
                                <p className="mt-1 text-[#f3f4f6]">{formatRelativeValue(u.lastLoginAt || u.last_login_at)}</p>
                              </div>
                              <div>
                                <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Created</p>
                                <p className="mt-1 text-[#f3f4f6]">{formatDateValue(u.createdAt || u.created_at)}</p>
                              </div>
                              <div>
                                <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Cases assigned</p>
                                <p className="mt-1 text-[#f3f4f6]">{Number(u.assignedCases || u.assigned_cases || 0).toLocaleString()}</p>
                              </div>
                            </div>
                          </div>
                        );
                      })
                    )}
                  </div>
                </AdminCard>
            </div>

            <Dialog
              open={createUserDialogOpen}
              onOpenChange={(open) => {
                setCreateUserDialogOpen(open);
                if (!open) {
                  setNewUser({ name: "", email: "", password: "", role: "Analyst", team: "", tenantId: currentTenantId, isAdmin: false });
                }
              }}
            >
              <DialogContent className="rounded-2xl border border-[#2a2c3c] bg-[#13141c] text-white">
                <DialogHeader>
                  <DialogTitle className="flex flex-wrap items-center justify-between gap-3">
                    <span className="inline-flex items-center gap-2">
                      <UserPlus size={18} /> Create User
                    </span>
                    <Badge variant="outline" className="border-primary/20 bg-primary/10 text-primary">
                      {tenantSlugByID[newUser.tenantId || currentTenantId] || newUser.tenantId || currentTenantId}
                    </Badge>
                  </DialogTitle>
                  <DialogDescription className="text-[#9ca3af]">
                    Add a new analyst, viewer, or admin to the selected tenant.
                  </DialogDescription>
                </DialogHeader>

                <div className="space-y-4">
                  <div className="space-y-2">
                    <AdminLabel>Name</AdminLabel>
                    <AdminInput
                      data-testid="input-user-name"
                      placeholder="John Doe"
                      value={newUser.name}
                      onChange={(e) => setNewUser({ ...newUser, name: e.target.value })}
                    />
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>Email</AdminLabel>
                    <AdminInput
                      data-testid="input-user-email"
                      placeholder="john@example.com"
                      value={newUser.email}
                      onChange={(e) => setNewUser({ ...newUser, email: e.target.value })}
                    />
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>{t("layout.quick.form.analystPassword")}</AdminLabel>
                    <AdminInput
                      data-testid="input-user-password"
                      type="password"
                      autoComplete="new-password"
                      placeholder={t("layout.quick.form.analystPasswordPlaceholder")}
                      value={newUser.password}
                      onChange={(e) => setNewUser({ ...newUser, password: e.target.value })}
                    />
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>Role</AdminLabel>
                    <Select value={newUser.role} onValueChange={(v) => setNewUser({ ...newUser, role: v })}>
                      <AdminSelectTrigger data-testid="select-user-role">
                        <SelectValue />
                      </AdminSelectTrigger>
                      <SelectContent>
                        <SelectItem value="Analyst">Analyst</SelectItem>
                        <SelectItem value="L2 Analyst">L2 Analyst</SelectItem>
                        <SelectItem value="L3 Lead">L3 Lead</SelectItem>
                        <SelectItem value="Manager">Manager</SelectItem>
                        <SelectItem value="Admin">Admin</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>Team</AdminLabel>
                    <AdminInput
                      data-testid="input-user-team"
                      placeholder="SOC Alpha"
                      value={newUser.team}
                      onChange={(e) => setNewUser({ ...newUser, team: e.target.value })}
                    />
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>Tenant</AdminLabel>
                    <Select value={newUser.tenantId} onValueChange={(v) => setNewUser({ ...newUser, tenantId: v })}>
                      <AdminSelectTrigger data-testid="select-user-tenant">
                        <SelectValue />
                      </AdminSelectTrigger>
                      <SelectContent searchable searchPlaceholder="Search tenant...">
                        {tenants.map((tenant: any) => (
                          <SelectItem key={tenant.id} value={tenant.id}>
                            {tenant.slug || tenant.id}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="flex items-center space-x-2 rounded-lg border border-[#252a3d] bg-[#10131c] px-3 py-2.5">
                    <Checkbox
                      data-testid="checkbox-user-admin"
                      checked={newUser.isAdmin}
                      onCheckedChange={(checked: boolean) => setNewUser({ ...newUser, isAdmin: !!checked })}
                    />
                    <AdminLabel className="text-sm">Administrator</AdminLabel>
                  </div>
                </div>

                <DialogFooter>
                  <AdminButton variant="outline" onClick={() => setCreateUserDialogOpen(false)}>
                    {t("common.cancel")}
                  </AdminButton>
                  <AdminButton
                    data-testid="button-create-user"
                    onClick={handleCreateUser}
                    disabled={createUser.isPending}
                  >
                    <Save size={16} className="mr-2" /> {createUser.isPending ? t("common.loading") : t("admin.button.createUser")}
                  </AdminButton>
                </DialogFooter>
              </DialogContent>
            </Dialog>

            <Dialog open={editUserDialogOpen} onOpenChange={setEditUserDialogOpen}>
              <DialogContent className="rounded-2xl">
                <DialogHeader>
                  <DialogTitle>Edit User</DialogTitle>
                </DialogHeader>
                {editingUser && (
                  <div className="space-y-4">
                    <div className="space-y-2">
                      <AdminLabel>Name</AdminLabel>
                      <AdminInput
                        data-testid="input-edit-user-name"
                        value={editingUser.name}
                        onChange={(e) => setEditingUser({ ...editingUser, name: e.target.value })}
                      />
                    </div>
                    <div className="space-y-2">
                      <AdminLabel>Email</AdminLabel>
                      <AdminInput
                        data-testid="input-edit-user-email"
                        type="email"
                        value={editingUser.email}
                        onChange={(e) => setEditingUser({ ...editingUser, email: e.target.value })}
                      />
                    </div>
                    <div className="space-y-2">
                      <AdminLabel>Team</AdminLabel>
                      <AdminInput
                        data-testid="input-edit-user-team"
                        value={editingUser.team}
                        onChange={(e) => setEditingUser({ ...editingUser, team: e.target.value })}
                      />
                    </div>
                    <div className="space-y-2">
                      <AdminLabel>Role</AdminLabel>
                      <Select
                        value={editingUser.role}
                        onValueChange={(value) =>
                          setEditingUser({
                            ...editingUser,
                            role: value as "tenant_admin" | "analyst" | "viewer",
                          })
                        }
                      >
                        <AdminSelectTrigger data-testid="select-edit-user-role">
                          <SelectValue />
                        </AdminSelectTrigger>
                        <SelectContent>
                          <SelectItem value="tenant_admin">{roleValueToLabel("tenant_admin")}</SelectItem>
                          <SelectItem value="analyst">{roleValueToLabel("analyst")}</SelectItem>
                          <SelectItem value="viewer">{roleValueToLabel("viewer")}</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                  </div>
                )}
                <DialogFooter>
                  <AdminButton variant="outline" onClick={() => setEditUserDialogOpen(false)}>{t("common.cancel")}</AdminButton>
                  <AdminButton data-testid="button-save-user" onClick={handleSaveEditedUser} disabled={updateUser.isPending}>
                    <Save size={16} className="mr-2" /> {updateUser.isPending ? t("common.loading") : t("common.save")}
                  </AdminButton>
                </DialogFooter>
              </DialogContent>
            </Dialog>

            <Dialog open={deleteUserDialogOpen} onOpenChange={setDeleteUserDialogOpen}>
              <DialogContent className="rounded-2xl">
                <DialogHeader>
                  <DialogTitle>Delete User</DialogTitle>
                  <DialogDescription>
                    {deletingUser
                      ? `User "${deletingUser.name}" will be removed from current tenant.`
                      : "User will be removed from current tenant."}
                  </DialogDescription>
                </DialogHeader>
                <DialogFooter>
                  <AdminButton variant="outline" onClick={() => setDeleteUserDialogOpen(false)}>{t("common.cancel")}</AdminButton>
                  <AdminButton
                    data-testid="button-confirm-delete-user"
                    variant="destructive"
                    onClick={handleConfirmDeleteUser}
                    disabled={deleteUser.isPending}
                  >
                    <Trash2 size={14} className="mr-2" /> {deleteUser.isPending ? t("common.loading") : t("common.delete")}
                  </AdminButton>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          </TabsContent>

          {/* ═══════════════════ TAB 2: TENANT MANAGEMENT ═══════════════════ */}
          <TabsContent value="tenants" className="space-y-6" id="admin-legacy-tenants">
            <div className="space-y-6">
                <AdminCard className="p-5">
                  <div className="flex flex-col gap-5">
                    <div className="flex flex-col gap-4 xl:flex-row xl:flex-wrap xl:items-start xl:justify-between">
                      <div>
                        <h3 className="font-bold text-lg">Tenant Directory</h3>
                        <p className="mt-1 text-sm text-[#9ca3af]">
                          Search tenants by slug, owner, or description and jump straight into status and edit actions.
                        </p>
                      </div>
                      <div className="grid w-full max-w-[640px] gap-3 sm:grid-cols-2 xl:max-w-none xl:grid-cols-4">
                        <div className="min-w-[120px] rounded-xl border border-[#252a3d] bg-[#10131c] p-3">
                          <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Total tenants</p>
                          <p className="mt-2 text-2xl font-semibold text-white">{tenants.length}</p>
                        </div>
                        <div className="min-w-[120px] rounded-xl border border-[#252a3d] bg-[#10131c] p-3">
                          <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Active</p>
                          <p className="mt-2 text-2xl font-semibold text-[#66ff4c]">{activeTenantsCount}</p>
                        </div>
                        <div className="min-w-[120px] rounded-xl border border-[#252a3d] bg-[#10131c] p-3">
                          <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Maintenance</p>
                          <p className="mt-2 text-2xl font-semibold text-[#f59e0b]">{inactiveTenantsCount}</p>
                        </div>
                        <div className="min-w-[120px] rounded-xl border border-[#252a3d] bg-[#10131c] p-3">
                          <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Owners set</p>
                          <p className="mt-2 text-2xl font-semibold text-[#60a5fa]">{ownedTenantsCount}</p>
                        </div>
                      </div>
                    </div>

                    <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_200px_auto]">
                      <div className="relative">
                        <Search size={14} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[#6b7280]" />
                        <AdminInput
                          data-testid="input-tenant-search"
                          value={tenantSearchQuery}
                          onChange={(event) => setTenantSearchQuery(event.target.value)}
                          placeholder="Search by name, slug, description, or owner"
                          className="pl-9"
                        />
                      </div>
                      <Select
                        value={tenantStatusFilter}
                        onValueChange={(value) => setTenantStatusFilter(value as typeof tenantStatusFilter)}
                      >
                        <SelectTrigger data-testid="select-tenant-status-filter">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="all">All statuses</SelectItem>
                          <SelectItem value="active">Active</SelectItem>
                          <SelectItem value="inactive">Maintenance</SelectItem>
                          <SelectItem value="owned">Owner assigned</SelectItem>
                        </SelectContent>
                      </Select>
                      <AdminButton
                        data-testid="button-reset-tenant-filters"
                        type="button"
                        variant="outline"
                        className="w-full xl:w-auto"
                        onClick={() => {
                          setTenantSearchQuery("");
                          setTenantStatusFilter("all");
                        }}
                        disabled={!hasTenantFilters}
                      >
                        Reset
                      </AdminButton>
                    </div>

                    <div className="flex flex-wrap items-center gap-2 text-xs text-[#9ca3af]">
                      <span>Showing {filteredTenants.length} of {tenants.length} tenants</span>
                      {tenantStatusFilter !== "all" ? (
                        <Badge variant="outline" className="border-[#3b82f6]/20 bg-[#3b82f6]/10 text-[#60a5fa]">
                          {tenantStatusFilter === "owned" ? "Owner assigned" : tenantStatusFilter === "inactive" ? "Maintenance" : "Active"}
                        </Badge>
                      ) : null}
                      {tenantSearchQuery.trim() ? (
                        <Badge variant="outline" className="max-w-full border-[#2f3549] bg-[#10131c] text-[#d1d5db]">
                          <span className="max-w-[280px] truncate">{tenantSearchQuery.trim()}</span>
                        </Badge>
                      ) : null}
                    </div>
                  </div>
                </AdminCard>

                <AdminCard className="overflow-hidden p-0">
                  <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[#252a3d] px-5 py-4">
                    <div>
                      <h3 className="font-bold text-lg">Tenant Inventory</h3>
                      <p className="mt-1 text-sm text-[#9ca3af]">Work across active tenants, ownership, capacity, and maintenance mode.</p>
                    </div>
                    <div className="flex items-center gap-2">
                      <Badge variant="outline" className="border-primary/20 bg-primary/10 text-primary">
                        {filteredTenants.length} visible
                      </Badge>
                      <AdminButton
                        data-testid="button-new-tenant"
                        size="sm"
                        className="h-9"
                        onClick={() => {
                          setNewTenant({ id: "", name: "", description: "", maxUsers: 50, responsibleUserId: "" });
                          setCreateTenantDialogOpen(true);
                        }}
                      >
                        <Plus size={14} className="mr-2" /> New Tenant
                      </AdminButton>
                    </div>
                  </div>
                  <div className="space-y-4 p-4">
                    {showTenantsLoading ? (
                      <div className="space-y-3">
                        {[1, 2, 3].map((index) => (
                          <Skeleton key={index} className="h-40 w-full rounded-xl" />
                        ))}
                      </div>
                    ) : filteredTenants.length === 0 ? (
                      <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-[#2f3549] bg-[#10131c] px-6 py-12 text-center">
                        <p className="text-sm font-medium text-white">No tenants match the current filters</p>
                        <p className="mt-1 text-sm text-[#9ca3af]">Adjust the search or reset the directory filters.</p>
                        {hasTenantFilters ? (
                          <AdminButton
                            type="button"
                            variant="outline"
                            className="mt-4"
                            onClick={() => {
                              setTenantSearchQuery("");
                              setTenantStatusFilter("all");
                            }}
                          >
                            Reset filters
                          </AdminButton>
                        ) : null}
                      </div>
                    ) : (
                      filteredTenants.map((tenant: any) => {
                        const active = isTenantActive(tenant);
                        const responsibleLabel = userNameByID[String(tenant?.responsibleUserId || "").trim()] || String(tenant?.responsibleUserId || "").trim() || "Unassigned";
                        return (
                          <div
                            key={tenant.id}
                            data-testid={`card-tenant-${tenant.id}`}
                            className={`overflow-hidden rounded-xl border bg-[#101420] transition-colors ${active ? "border-[#252a3d] hover:border-[#4b5168]" : "border-dashed border-[#5b4730] bg-[#16110c]"}`}
                          >
                            <div className="flex flex-col gap-4 p-4 lg:flex-row lg:items-start lg:justify-between">
                              <div className="min-w-0 flex-1">
                                <div className="flex flex-wrap items-center gap-3">
                                  <div className="flex h-11 w-11 flex-none items-center justify-center rounded-xl bg-primary/10 text-primary">
                                    <Building2 size={18} />
                                  </div>
                                  <div className="min-w-0">
                                    <div className="flex flex-wrap items-center gap-2">
                                      <p className="truncate text-sm font-semibold text-[#f3f4f6]">{tenant.name || tenant.slug || tenant.id}</p>
                                      <Badge variant={active ? "default" : "secondary"} className="text-[10px]">
                                        {active ? "Active" : "Maintenance"}
                                      </Badge>
                                    </div>
                                    <p className="mt-1 truncate font-mono text-[11px] text-[#9ca3af]">{tenant.slug || tenant.id}</p>
                                  </div>
                                </div>
                                <p className="mt-4 text-sm text-[#9ca3af]">{tenant.description || "No description provided yet."}</p>
                                <div className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                                  <div className="rounded-xl border border-[#252a3d] bg-[#0f131d] p-3">
                                    <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Capacity</p>
                                    <p className="mt-2 text-sm font-semibold text-white">{Number(tenant.maxUsers || 0)} users</p>
                                  </div>
                                  <div className="rounded-xl border border-[#252a3d] bg-[#0f131d] p-3">
                                    <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Owner</p>
                                    <p className="mt-2 truncate text-sm font-semibold text-white">{responsibleLabel}</p>
                                  </div>
                                  <div className="rounded-xl border border-[#252a3d] bg-[#0f131d] p-3">
                                    <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Slug</p>
                                    <p className="mt-2 truncate font-mono text-sm text-[#d1d5db]">{tenant.slug || tenant.id}</p>
                                  </div>
                                  <div className="rounded-xl border border-[#252a3d] bg-[#0f131d] p-3">
                                    <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Mode</p>
                                    <p className={`mt-2 text-sm font-semibold ${active ? "text-[#66ff4c]" : "text-[#f59e0b]"}`}>{active ? "Operational" : "Limited access"}</p>
                                  </div>
                                </div>
                              </div>
                              <div className="flex w-full shrink-0 flex-col gap-3 rounded-xl border border-[#252a3d] bg-[#0c1018] p-3 lg:w-[220px]">
                                <div>
                                  <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Actions</p>
                                  <p className="mt-1 text-xs text-[#9ca3af]">Edit ownership, description, or max capacity.</p>
                                </div>
                                <AdminButton
                                  data-testid={`button-edit-tenant-${tenant.id}`}
                                  variant="outline"
                                  className="w-full"
                                  onClick={() => {
                                    setEditingTenant({ ...tenant });
                                    setEditTenantDialogOpen(true);
                                  }}
                                >
                                  <Edit size={14} className="mr-2" /> {t("common.edit")}
                                </AdminButton>
                                <div className="flex items-center justify-between rounded-lg border border-[#252a3d] bg-[#10131c] px-3 py-2">
                                  <div>
                                    <p className="text-sm font-medium text-white">Tenant access</p>
                                    <p className="text-xs text-[#9ca3af]">Toggle maintenance mode</p>
                                  </div>
                                  <Switch
                                    data-testid={`switch-tenant-${tenant.id}`}
                                    checked={active}
                                    onCheckedChange={() => handleToggleTenant(tenant.id, active)}
                                  />
                                </div>
                              </div>
                            </div>
                          </div>
                        );
                      })
                    )}
                  </div>
                </AdminCard>
            </div>

            <Dialog
              open={createTenantDialogOpen}
              onOpenChange={(open) => {
                setCreateTenantDialogOpen(open);
                if (!open) {
                  setNewTenant({ id: "", name: "", description: "", maxUsers: 50, responsibleUserId: "" });
                }
              }}
            >
              <DialogContent className="rounded-2xl border border-[#2a2c3c] bg-[#13141c] text-white">
                <DialogHeader>
                  <DialogTitle className="inline-flex items-center gap-2">
                    <Plus size={18} /> Create Tenant
                  </DialogTitle>
                  <DialogDescription className="text-[#9ca3af]">
                    Add a new tenant, assign an owner, and define the initial capacity limit.
                  </DialogDescription>
                </DialogHeader>

                <div className="space-y-4">
                  <div className="space-y-2">
                    <AdminLabel>Tenant ID (slug)</AdminLabel>
                    <AdminInput
                      data-testid="input-tenant-id"
                      placeholder="acme_corp"
                      value={newTenant.id}
                      onChange={(e) => setNewTenant({ ...newTenant, id: e.target.value })}
                    />
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>Name</AdminLabel>
                    <AdminInput
                      data-testid="input-tenant-name"
                      placeholder="Acme Corporation"
                      value={newTenant.name}
                      onChange={(e) => setNewTenant({ ...newTenant, name: e.target.value })}
                    />
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>Description</AdminLabel>
                    <AdminTextarea
                      data-testid="input-tenant-description"
                      placeholder="Enterprise security team"
                      value={newTenant.description}
                      onChange={(e) => setNewTenant({ ...newTenant, description: e.target.value })}
                    />
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>Max Users</AdminLabel>
                    <AdminInput
                      data-testid="input-tenant-max-users"
                      type="number"
                      value={newTenant.maxUsers}
                      onChange={(e) => setNewTenant({ ...newTenant, maxUsers: parseInt(e.target.value) || 50 })}
                    />
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>Responsible User</AdminLabel>
                    <Select
                      value={newTenant.responsibleUserId || ""}
                      onValueChange={(v) => setNewTenant({ ...newTenant, responsibleUserId: v })}
                    >
                      <AdminSelectTrigger data-testid="select-tenant-responsible">
                        <SelectValue placeholder="Select responsible user..." />
                      </AdminSelectTrigger>
                      <SelectContent searchable searchPlaceholder="Search user...">
                        {users.map((u: any) => (
                          <SelectItem key={u.id} value={u.id}>
                            {u.name} ({u.role})
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                </div>

                <DialogFooter>
                  <AdminButton variant="outline" onClick={() => setCreateTenantDialogOpen(false)}>
                    {t("common.cancel")}
                  </AdminButton>
                  <AdminButton
                    data-testid="button-create-tenant"
                    onClick={handleCreateTenant}
                    disabled={createTenant.isPending}
                  >
                    <Save size={16} className="mr-2" /> {createTenant.isPending ? t("common.loading") : t("admin.button.createTenant")}
                  </AdminButton>
                </DialogFooter>
              </DialogContent>
            </Dialog>

            <CaseStatusesPanel tenantId={currentTenantId} />

            <Dialog open={editTenantDialogOpen} onOpenChange={setEditTenantDialogOpen}>
              <DialogContent className="rounded-2xl">
                <DialogHeader>
                  <DialogTitle>Edit Tenant</DialogTitle>
                </DialogHeader>
                {editingTenant && (
                  <div className="space-y-4">
                    <div className="space-y-2">
                      <AdminLabel>Name</AdminLabel>
                      <AdminInput data-testid="input-edit-tenant-name" value={editingTenant.name} onChange={e => setEditingTenant({...editingTenant, name: e.target.value})} />
                    </div>
                    <div className="space-y-2">
                      <AdminLabel>Description</AdminLabel>
                      <AdminTextarea data-testid="input-edit-tenant-description" value={editingTenant.description || ""} onChange={e => setEditingTenant({...editingTenant, description: e.target.value})} />
                    </div>
                    <div className="space-y-2">
                      <AdminLabel>Max Users</AdminLabel>
                      <AdminInput data-testid="input-edit-tenant-max-users" type="number" value={editingTenant.maxUsers} onChange={e => setEditingTenant({...editingTenant, maxUsers: parseInt(e.target.value) || 50})} />
                    </div>
                    <div className="space-y-2">
                      <AdminLabel>Responsible User</AdminLabel>
                      <Select value={editingTenant.responsibleUserId || ""} onValueChange={v => setEditingTenant({...editingTenant, responsibleUserId: v})}>
                        <AdminSelectTrigger data-testid="select-edit-tenant-responsible">
                          <SelectValue placeholder="Select responsible user..." />
                        </AdminSelectTrigger>
                        <SelectContent>
                          {users.map((u: any) => (
                            <SelectItem key={u.id} value={u.id}>{u.name} ({u.role})</SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  </div>
                )}
                <DialogFooter>
                  <AdminButton variant="outline" onClick={() => setEditTenantDialogOpen(false)}>{t("common.cancel")}</AdminButton>
                  <AdminButton data-testid="button-save-tenant" onClick={() => {
                    if (!editingTenant) return;
                    updateTenant.mutate({ id: editingTenant.id, data: { name: editingTenant.name, description: editingTenant.description, maxUsers: editingTenant.maxUsers, responsibleUserId: editingTenant.responsibleUserId } });
                    setEditTenantDialogOpen(false);
                    toast.success("Tenant updated");
                  }}>
                    <Save size={16} className="mr-2" /> {t("profile.saveChanges")}
                  </AdminButton>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          </TabsContent>

          {/* ═══════════════════ TAB 3: ACHIEVEMENTS ═══════════════════ */}
          <TabsContent value="achievements" className="space-y-6" id="admin-legacy-achievements">
            <div className="space-y-4">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="flex items-center gap-2">
                  <h3 className="font-bold text-lg">Achievements</h3>
                  <Badge variant="outline" className="border-primary/20 bg-primary/10 text-primary">
                    {achievements.length}
                  </Badge>
                </div>
                <AdminButton
                  data-testid="button-new-achievement"
                  size="sm"
                  className="h-9"
                  onClick={() => {
                    setNewAchievement({ name: "", description: "", icon: DEFAULT_ACHIEVEMENT_ICON, rarity: "Common", xpReward: 150 });
                    setCreateAchievementDialogOpen(true);
                  }}
                >
                  <Plus size={14} className="mr-2" /> New Achievement
                </AdminButton>
              </div>
                {showAchievementsLoading ? (
                  <div className="grid md:grid-cols-2 gap-4">
                    {[1, 2, 3, 4].map(i => <Skeleton key={i} className="h-32 w-full rounded-xl" />)}
                  </div>
                ) : achievements.length === 0 ? (
                  <AdminCard className="p-8 text-center text-[#9ca3af]">
                    <Trophy size={32} className="mx-auto mb-2 opacity-30" />
                    <p className="text-sm">No achievements created yet</p>
                  </AdminCard>
                ) : (
                  <div className="grid md:grid-cols-2 gap-4">
                    {achievements.map((ach: any) => {
                      const style = RARITY_STYLES[ach.rarity] || RARITY_STYLES.Common;
                      const iconValue = String(ach.icon || "");
                      const iconPreview = isAchievementImageIcon(iconValue)
                        ? iconValue
                        : String(ach.icon_url || ach.iconUrl || "");
                      const isImageIcon = isAchievementImageIcon(iconPreview);
                      const xpReward = Number(ach.xp_reward ?? ach.xpReward ?? 0);
                      return (
                        <AdminCard
                          key={ach.id}
                          data-testid={`card-achievement-${ach.id}`}
                          className={`p-5 transition-all border-2 ${style.border} ${style.bg} ${style.glow}`}
                        >
                          <div className="flex justify-between items-start">
                            <div className="flex items-center gap-3">
                              <div className="text-3xl flex items-center justify-center w-12 h-12 rounded-xl bg-white/50 dark:bg-black/20">
                                {isImageIcon ? (
                                  <img src={iconPreview} alt={ach.name} className="w-8 h-8 object-contain" />
                                ) : (
                                  <span>{iconValue || "🏆"}</span>
                                )}
                              </div>
                              <div>
                                <h4 className="font-bold text-sm">{ach.name}</h4>
                                <p className="text-xs text-[#9ca3af] line-clamp-2">{ach.description}</p>
                              </div>
                            </div>
                            <AdminButton
                              data-testid={`button-delete-achievement-${ach.id}`}
                              variant="ghost"
                              size="sm"
                              className="h-7 text-red-500 hover:text-red-700"
                              onClick={() => { deleteAchievement.mutate(ach.id); toast.success("Achievement deleted"); }}
                            >
                              <Trash2 size={14} />
                            </AdminButton>
                          </div>
                          <div className="mt-3">
                            <Badge className={`text-[10px] ${style.badge}`}>{ach.rarity}</Badge>
                            <Badge variant="outline" className="ml-2 text-[10px]">+{xpReward} XP</Badge>
                          </div>
                        </AdminCard>
                      );
                    })}
                  </div>
                )}
            </div>

            <Dialog
              open={createAchievementDialogOpen}
              onOpenChange={(open) => {
                setCreateAchievementDialogOpen(open);
                if (!open) {
                  setNewAchievement({ name: "", description: "", icon: DEFAULT_ACHIEVEMENT_ICON, rarity: "Common", xpReward: 150 });
                }
              }}
            >
              <DialogContent className="rounded-2xl border border-[#2a2c3c] bg-[#13141c] text-white">
                <DialogHeader>
                  <DialogTitle className="inline-flex items-center gap-2">
                    <Trophy size={18} /> Create Achievement
                  </DialogTitle>
                </DialogHeader>

                <div className="space-y-4">
                  <div className="space-y-2">
                    <AdminLabel>Name</AdminLabel>
                    <AdminInput
                      data-testid="input-achievement-name"
                      placeholder="First Case Closed"
                      value={newAchievement.name}
                      onChange={(e) => setNewAchievement({ ...newAchievement, name: e.target.value })}
                    />
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>Description</AdminLabel>
                    <AdminTextarea
                      data-testid="input-achievement-description"
                      placeholder="Awarded when an analyst closes their first case..."
                      className="min-h-[80px]"
                      value={newAchievement.description}
                      onChange={(e) => setNewAchievement({ ...newAchievement, description: e.target.value })}
                    />
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>Icon</AdminLabel>
                    <div className="flex items-center gap-2">
                      <Select value={newAchievement.icon} onValueChange={(value) => setNewAchievement({ ...newAchievement, icon: value })}>
                        <AdminSelectTrigger data-testid="select-achievement-icon" className="flex-1">
                          <SelectValue placeholder="Select icon" />
                        </AdminSelectTrigger>
                        <SelectContent className="max-h-72">
                          {achievementIconOptions.map((iconOption) => (
                            <SelectItem key={iconOption.value} value={iconOption.value}>
                              <span className="inline-flex items-center gap-2">
                                {isAchievementImageIcon(iconOption.preview) ? (
                                  <img src={iconOption.preview} alt={iconOption.label} className="h-5 w-5 rounded-sm object-contain" />
                                ) : (
                                  <span className="text-base leading-none">{iconOption.preview || "🏆"}</span>
                                )}
                                <span className="truncate max-w-[220px]">{iconOption.label}</span>
                              </span>
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <AdminButton
                        type="button"
                        variant="outline"
                        size="icon"
                        className="h-10 w-10 shrink-0"
                        data-testid="button-upload-achievement-icon"
                        onClick={() => achievementIconFileRef.current?.click()}
                        disabled={uploadAchievementIcon.isPending}
                        title="Upload icon"
                      >
                        <Upload size={14} />
                      </AdminButton>
                      <input
                        ref={achievementIconFileRef}
                        type="file"
                        accept={ACHIEVEMENT_ICON_UPLOAD_CRITERIA.allowedTypes.join(",")}
                        className="hidden"
                        onChange={handleUploadAchievementIcon}
                        data-testid="input-upload-achievement-icon"
                      />
                    </div>
                    <p className="text-xs text-[#9ca3af]">
                      PNG/JPEG/SVG, up to 2MB, 64-512px, square.
                    </p>
                    <div className="min-h-[52px] rounded-lg border bg-[#161b29] p-2.5 flex items-center gap-2">
                      {selectedAchievementIconOption && isAchievementImageIcon(selectedAchievementIconOption.preview) ? (
                        <img
                          src={selectedAchievementIconOption.preview}
                          alt={selectedAchievementIconOption.label}
                          className="h-8 w-8 rounded-md object-contain"
                        />
                      ) : (
                        <span className="text-xl leading-none">{selectedAchievementIconOption?.preview || newAchievement.icon || "🏆"}</span>
                      )}
                      <span className="truncate text-xs text-[#9ca3af]">
                        {selectedAchievementIconOption?.label || "Selected icon"}
                      </span>
                    </div>
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>Rarity</AdminLabel>
                    <Select value={newAchievement.rarity} onValueChange={(v) => setNewAchievement({ ...newAchievement, rarity: v })}>
                      <AdminSelectTrigger data-testid="select-achievement-rarity">
                        <SelectValue />
                      </AdminSelectTrigger>
                      <SelectContent>
                        <SelectItem value="Common">Common</SelectItem>
                        <SelectItem value="Rare">Rare</SelectItem>
                        <SelectItem value="Epic">Epic</SelectItem>
                        <SelectItem value="Legendary">Legendary</SelectItem>
                        <SelectItem value="Diamond">Diamond</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>XP Reward</AdminLabel>
                    <AdminInput
                      data-testid="input-achievement-xp-reward"
                      type="number"
                      min={0}
                      step={1}
                      value={newAchievement.xpReward}
                      onChange={(e) => setNewAchievement({ ...newAchievement, xpReward: Math.max(0, Number(e.target.value || 0)) })}
                    />
                  </div>
                </div>

                <DialogFooter>
                  <AdminButton variant="outline" onClick={() => setCreateAchievementDialogOpen(false)}>
                    {t("common.cancel")}
                  </AdminButton>
                  <AdminButton
                    data-testid="button-create-achievement"
                    onClick={handleCreateAchievement}
                    disabled={createAchievement.isPending}
                  >
                    <Save size={16} className="mr-2" /> {createAchievement.isPending ? t("common.loading") : t("admin.button.createAchievement")}
                  </AdminButton>
                </DialogFooter>
              </DialogContent>
            </Dialog>

            <AdminCard className="p-6">
              <h3 className="font-bold text-lg mb-4 flex items-center gap-2">
                <Award size={18} /> Grant Achievement to User
              </h3>
              <div className="flex flex-wrap gap-4 items-end">
                <div className="space-y-2 min-w-[200px] flex-1">
                  <AdminLabel>Achievement</AdminLabel>
                  <Select value={grantAchievementId} onValueChange={setGrantAchievementId}>
                    <AdminSelectTrigger data-testid="select-grant-achievement">
                      <SelectValue placeholder="Select achievement..." />
                    </AdminSelectTrigger>
                    <SelectContent>
                      {achievements.map((ach: any) => (
                        <SelectItem key={ach.id} value={ach.id}>
                          <span className="inline-flex items-center gap-2">
                            {isAchievementImageIcon(ach.icon) ? (
                              <img src={ach.icon} alt={ach.name} className="h-5 w-5 rounded-sm object-contain" />
                            ) : (
                              <span>{ach.icon || "🏆"}</span>
                            )}
                            <span className="truncate max-w-[220px]">{ach.name}</span>
                          </span>
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2 min-w-[200px] flex-1">
                  <AdminLabel>User</AdminLabel>
                  <Select value={grantUserId} onValueChange={v => { setGrantUserId(v); setGrantViewUserId(v); }}>
                    <AdminSelectTrigger data-testid="select-grant-user">
                      <SelectValue placeholder="Select user..." />
                    </AdminSelectTrigger>
                    <SelectContent>
                      {users.map((u: any) => (
                        <SelectItem key={u.id} value={u.id}>{u.name}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <AdminButton data-testid="button-grant-achievement" onClick={handleGrantAchievement}>
                  <Award size={16} className="mr-2" /> {t("admin.button.grant")}
                </AdminButton>
              </div>
            </AdminCard>

            {grantViewUserId && (
              <AdminCard className="p-6">
                <h3 className="font-bold text-lg mb-4 flex items-center gap-2">
                  <Trophy size={18} /> Granted Achievements
                  <Badge variant="outline" className="text-xs ml-2">
                    {users.find((u: any) => u.id === grantViewUserId)?.name || grantViewUserId}
                  </Badge>
                </h3>
                {grantedAchievements.length === 0 ? (
                  <p className="text-sm text-[#9ca3af] text-center py-4">No achievements granted to this user yet.</p>
                ) : (
                  <div className="overflow-x-auto">
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="border-b text-left">
                          <th className="pb-2 font-medium text-[#9ca3af]">Achievement</th>
                          <th className="pb-2 font-medium text-[#9ca3af]">Rarity</th>
                          <th className="pb-2 font-medium text-[#9ca3af]">XP</th>
                          <th className="pb-2 font-medium text-[#9ca3af]">Granted</th>
                          <th className="pb-2 font-medium text-[#9ca3af] text-right">Actions</th>
                        </tr>
                      </thead>
                      <tbody>
                        {grantedAchievements.map((ga: any) => {
                          const ach = achievements.find((a: any) => a.id === ga.achievementId);
                          const style = RARITY_STYLES[ach?.rarity || "Common"] || RARITY_STYLES.Common;
                          const xpReward = Number(ach?.xp_reward ?? ach?.xpReward ?? 0);
                          return (
                            <tr key={ga.id} data-testid={`row-granted-${ga.id}`} className="border-b last:border-0">
                              <td className="py-3 flex items-center gap-2">
                                {isAchievementImageIcon(ach?.icon) ? (
                                  <img src={ach.icon} alt={ach?.name || ga.achievementId} className="h-5 w-5 rounded-sm object-contain" />
                                ) : (
                                  <span className="text-lg">{ach?.icon || "🏆"}</span>
                                )}
                                <span className="font-medium">{ach?.name || ga.achievementId}</span>
                              </td>
                              <td className="py-3">
                                <Badge className={`text-[10px] ${style.badge}`}>{ach?.rarity || "Unknown"}</Badge>
                              </td>
                              <td className="py-3 text-xs font-semibold text-[#f3f4f6]">+{xpReward}</td>
                              <td className="py-3 text-xs text-[#9ca3af]">
                                {ga.grantedAt ? new Date(ga.grantedAt).toLocaleDateString() : "—"}
                              </td>
                              <td className="py-3 text-right">
                                <AdminButton
                                  data-testid={`button-revoke-${ga.id}`}
                                  variant="ghost"
                                  size="sm"
                                  className="h-7 text-red-500 hover:text-red-700"
                                  onClick={() => { revokeAchievement.mutate(ga.id); toast.success("Achievement revoked"); }}
                                >
                                  <Trash2 size={14} className="mr-1" /> Revoke
                                </AdminButton>
                              </td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                )}
              </AdminCard>
            )}
          </TabsContent>

          {/* ═══════════════════ TAB 4: EXPERIENCE ═══════════════════ */}
          <TabsContent value="experience" className="space-y-6" id="admin-legacy-experience">
            <div className="space-y-4">
              <AdminCard className="p-5">
                <div className="flex flex-wrap items-end justify-between gap-4">
                  <div>
                    <h3 className="flex items-center gap-2 text-lg font-bold">
                      <Clock size={18} /> Experience History
                    </h3>
                    <p className="mt-1 text-sm text-[#9ca3af]">Pick a user to review XP events, and award XP via a focused dialog.</p>
                  </div>
                  <div className="flex flex-wrap items-end gap-3">
                    <div className="min-w-[220px] space-y-1">
                      <AdminLabel>User</AdminLabel>
                      <Select value={awardXPUserId} onValueChange={setAwardXPUserId}>
                        <AdminSelectTrigger data-testid="select-award-xp-user">
                          <SelectValue placeholder="Select user..." />
                        </AdminSelectTrigger>
                        <SelectContent searchable searchPlaceholder="Search user...">
                          {users.map((u: any) => (
                            <SelectItem key={u.id} value={u.id}>
                              {u.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                    <AdminButton
                      data-testid="button-open-award-xp"
                      className="h-10"
                      onClick={() => setAwardXPDialogOpen(true)}
                    >
                      <ZapIcon size={16} className="mr-2" /> Award XP
                    </AdminButton>
                  </div>
                </div>
              </AdminCard>
                {!awardXPUserId ? (
                  <AdminCard className="p-8 text-center text-[#9ca3af]">
                    <p className="text-sm">Select user to view history</p>
                  </AdminCard>
                ) : showExperienceHistoryLoading ? (
                  <div className="space-y-3">
                    {[1, 2, 3].map((i) => (
                      <Skeleton key={i} className="h-14 w-full rounded-xl" />
                    ))}
                  </div>
                ) : experienceHistory.length === 0 ? (
                  <AdminCard className="p-8 text-center text-[#9ca3af]">
                    <p className="text-sm">No experience events for this user yet</p>
                  </AdminCard>
                ) : (
                  <AdminCard className="p-0 overflow-hidden">
                    <div className="overflow-x-auto">
                      <table className="w-full text-sm">
                        <thead>
                          <tr className="border-b text-left">
                            <th className="px-4 py-3 font-medium text-[#9ca3af]">Date</th>
                            <th className="px-4 py-3 font-medium text-[#9ca3af]">Description</th>
                            <th className="px-4 py-3 font-medium text-[#9ca3af] text-right">XP</th>
                          </tr>
                        </thead>
                        <tbody>
                          {experienceHistory.map((event: any) => (
                            <tr key={event.id} className="border-b last:border-0">
                              <td className="px-4 py-3 text-xs text-[#9ca3af] whitespace-nowrap">
                                {event.createdAt ? new Date(event.createdAt).toLocaleString() : "—"}
                              </td>
                              <td className="px-4 py-3">
                                <div className="font-medium text-[#f3f4f6]">{event.description || "Experience rewarded"}</div>
                                <div className="text-[11px] text-[#9ca3af] mt-0.5">{event.eventType}</div>
                              </td>
                              <td className="px-4 py-3 text-right">
                                <Badge className="bg-yellow-500/20 text-yellow-700 border-yellow-300">
                                  +{Number(event.points || 0).toLocaleString()} XP
                                </Badge>
                              </td>
                            </tr>
                          ))}
                          {experienceHistoryHasMore && (
                            <tr ref={experienceHistoryLoadMoreRef}>
                              <td colSpan={3} className="px-4 py-4 text-center text-xs text-[#9ca3af]">
                                {loadingMoreExperienceHistory ? "Loading more..." : ""}
                              </td>
                            </tr>
                          )}
                        </tbody>
                      </table>
                    </div>
                  </AdminCard>
                )}
            </div>

            <Dialog
              open={awardXPDialogOpen}
              onOpenChange={(open) => {
                setAwardXPDialogOpen(open);
                if (!open) {
                  setAwardXPDescription("");
                }
              }}
            >
              <DialogContent className="rounded-2xl border border-[#2a2c3c] bg-[#13141c] text-white">
                <DialogHeader>
                  <DialogTitle className="inline-flex items-center gap-2">
                    <ZapIcon size={18} /> Manual XP Award
                  </DialogTitle>
                </DialogHeader>

                <div className="space-y-4">
                  <div className="space-y-2">
                    <AdminLabel>User</AdminLabel>
                    <Select value={awardXPUserId} onValueChange={setAwardXPUserId}>
                      <AdminSelectTrigger data-testid="select-award-xp-user-dialog">
                        <SelectValue placeholder="Select user..." />
                      </AdminSelectTrigger>
                      <SelectContent searchable searchPlaceholder="Search user...">
                        {users.map((u: any) => (
                          <SelectItem key={u.id} value={u.id}>
                            {u.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>XP Amount</AdminLabel>
                    <AdminInput
                      data-testid="input-award-xp-points"
                      type="number"
                      min={1}
                      step={1}
                      value={awardXPPoints}
                      onChange={(e) => setAwardXPPoints(Math.max(1, Math.floor(Number(e.target.value || 1))))}
                    />
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>Description</AdminLabel>
                    <AdminTextarea
                      data-testid="input-award-xp-description"
                      placeholder="Why this XP was granted"
                      className="min-h-[90px]"
                      value={awardXPDescription}
                      onChange={(e) => setAwardXPDescription(e.target.value)}
                    />
                  </div>
                </div>

                <DialogFooter>
                  <AdminButton variant="outline" onClick={() => setAwardXPDialogOpen(false)}>
                    {t("common.cancel")}
                  </AdminButton>
                  <AdminButton
                    data-testid="button-award-xp"
                    onClick={handleAwardExperience}
                    disabled={awardUserExperience.isPending}
                  >
                    <ZapIcon size={16} className="mr-2" /> {awardUserExperience.isPending ? t("common.loading") : "Award XP"}
                  </AdminButton>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          </TabsContent>

          {/* ═══════════════════ TAB 5: NOTIFICATION BOTS ═══════════════════ */}
          <TabsContent value="notification_bots" className="space-y-6" id="admin-legacy-notification-bots">
            <div className="space-y-4">
                <AdminCard className="p-5">
                  <div className="flex flex-col gap-5">
                    <div className="flex flex-col gap-4 xl:flex-row xl:flex-wrap xl:items-start xl:justify-between">
                      <div>
                        <h3 className="font-bold text-lg">Configured Telegram Bots</h3>
                        <p className="mt-1 text-sm text-[#9ca3af]">
                          Track enabled bots, search by username, and keep tenant delivery channels clean.
                        </p>
                      </div>
                      <div className="grid w-full max-w-[640px] gap-3 sm:grid-cols-2 xl:max-w-none xl:grid-cols-4">
                        <div className="min-w-[120px] rounded-xl border border-[#252a3d] bg-[#10131c] p-3">
                          <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Bots</p>
                          <p className="mt-2 text-2xl font-semibold text-white">{notificationBots.length}</p>
                        </div>
                        <div className="min-w-[120px] rounded-xl border border-[#252a3d] bg-[#10131c] p-3">
                          <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Enabled</p>
                          <p className="mt-2 text-2xl font-semibold text-[#66ff4c]">{enabledNotificationBotsCount}</p>
                        </div>
                        <div className="min-w-[120px] rounded-xl border border-[#252a3d] bg-[#10131c] p-3">
                          <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Recipients on</p>
                          <p className="mt-2 text-2xl font-semibold text-[#60a5fa]">{deliveryEnabledRecipientsCount}</p>
                        </div>
                        <div className="min-w-[120px] rounded-xl border border-[#252a3d] bg-[#10131c] p-3">
                          <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Telegram channel</p>
                          <p className="mt-2 text-2xl font-semibold text-[#facc15]">{telegramRecipientsCount}</p>
                        </div>
                      </div>
                    </div>

                    <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_180px_240px_auto]">
                      <div className="relative">
                        <Search size={14} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[#6b7280]" />
                        <AdminInput
                          data-testid="input-notification-bot-search"
                          value={notificationBotSearchQuery}
                          onChange={(event) => setNotificationBotSearchQuery(event.target.value)}
                          placeholder="Search by bot name, username, or bot id"
                          className="pl-9"
                        />
                      </div>
                      <Select
                        value={notificationBotStatusFilter}
                        onValueChange={(value) => setNotificationBotStatusFilter(value as typeof notificationBotStatusFilter)}
                      >
                        <SelectTrigger data-testid="select-notification-bot-status-filter">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="all">All statuses</SelectItem>
                          <SelectItem value="enabled">Enabled</SelectItem>
                          <SelectItem value="disabled">Disabled</SelectItem>
                        </SelectContent>
                      </Select>
                      <Select
                        value={notificationBotTenantId}
                        onValueChange={(value) => setNotificationBotTenantId(value)}
                      >
                        <SelectTrigger data-testid="select-notification-bot-tenant">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent searchable searchPlaceholder="Search tenant...">
                          {tenants.map((tenant: any) => (
                            <SelectItem key={tenant.id} value={tenant.id}>
                              {tenant.slug || tenant.id}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <AdminButton
                        type="button"
                        variant="outline"
                        className="w-full xl:w-auto"
                        onClick={() => setCreateNotificationBotDialogOpen(true)}
                        data-testid="button-new-notification-bot"
                      >
                        <Plus size={14} className="mr-2" /> New Bot
                      </AdminButton>
                    </div>

                    <div className="flex flex-wrap items-center gap-2 text-xs text-[#9ca3af]">
                      <span>Showing {filteredNotificationBots.length} of {notificationBots.length} bots</span>
                      {notificationBotTenantId ? (
                        <Badge variant="outline" className="border-primary/20 bg-primary/10 text-primary">
                          {tenantSlugByID[notificationBotTenantId] || notificationBotTenantId}
                        </Badge>
                      ) : null}
                      {notificationBotStatusFilter !== "all" ? (
                        <Badge variant="outline" className="border-[#3b82f6]/20 bg-[#3b82f6]/10 text-[#60a5fa]">
                          {notificationBotStatusFilter}
                        </Badge>
                      ) : null}
                      {hasNotificationBotFilters ? (
                        <AdminButton
                          type="button"
                          variant="ghost"
                          size="sm"
                          className="h-7 px-2 text-[11px]"
                          onClick={() => {
                            setNotificationBotSearchQuery("");
                            setNotificationBotStatusFilter("all");
                          }}
                        >
                          Reset filters
                        </AdminButton>
                      ) : null}
                    </div>
                  </div>
                </AdminCard>

                {showNotificationBotsLoading ? (
                  <div className="space-y-3">
                    {[1, 2].map((index) => (
                      <Skeleton key={index} className="h-28 w-full rounded-xl" />
                    ))}
                  </div>
                ) : filteredNotificationBots.length === 0 ? (
                  <AdminCard className="p-8 text-center text-[#9ca3af]">
                    <Radio size={30} className="mx-auto mb-2 opacity-30" />
                    <p className="text-sm">{notificationBots.length === 0 ? "No Telegram bots configured yet" : "No bots match the current filters"}</p>
                  </AdminCard>
                ) : (
                  <div className="space-y-3">
                    {filteredNotificationBots.map((bot: any) => (
                      <AdminCard key={bot.id} className="p-4 border border-[#2a2c3c]">
                        <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                          <div className="space-y-2 min-w-0 flex-1">
                            <div className="flex items-center gap-2 flex-wrap">
                              <Badge variant={bot.enabled ? "default" : "secondary"} className="text-[10px]">
                                {bot.enabled ? "Enabled" : "Disabled"}
                              </Badge>
                              <Badge variant="outline" className="text-[10px] font-mono">
                                @{bot.botUsername || "unknown"}
                              </Badge>
                              <Badge variant="outline" className="text-[10px]">
                                id {bot.botId || "-"}
                              </Badge>
                            </div>
                            <AdminInput
                              data-testid={`input-notification-bot-name-${bot.id}`}
                              value={notificationBotNames[bot.id] ?? bot.name}
                              onChange={(e) =>
                                setNotificationBotNames((prev) => ({
                                  ...prev,
                                  [bot.id]: e.target.value,
                                }))
                              }
                            />
                            <div className="text-xs text-[#9ca3af]">
                              {bot.botFirstName || "Telegram bot"} · Updated {bot.updatedAt ? new Date(bot.updatedAt).toLocaleString() : "-"}
                            </div>
                          </div>
                          <div className="flex items-center gap-2 md:pl-4">
                            <Switch
                              data-testid={`switch-notification-bot-${bot.id}`}
                              checked={Boolean(bot.enabled)}
                              onCheckedChange={(checked) => handleToggleNotificationBot(bot, checked)}
                              disabled={updateAdminNotificationBot.isPending}
                            />
                            <AdminButton
                              data-testid={`button-save-notification-bot-${bot.id}`}
                              size="sm"
                              variant="outline"
                              onClick={() => handleSaveNotificationBotName(bot)}
                              disabled={updateAdminNotificationBot.isPending}
                            >
                              <Save size={14} className="mr-1" /> Save
                            </AdminButton>
                            <AdminButton
                              data-testid={`button-delete-notification-bot-${bot.id}`}
                              size="sm"
                              variant="destructive"
                              onClick={() => handleDeleteNotificationBot(bot.id, String(bot.tenantId || notificationBotTenantId || ""))}
                              disabled={deleteAdminNotificationBot.isPending}
                            >
                              <Trash2 size={14} className="mr-1" /> Delete
                            </AdminButton>
                          </div>
                        </div>
                      </AdminCard>
                    ))}
                  </div>
                )}
            </div>

            <Dialog
              open={createNotificationBotDialogOpen}
              onOpenChange={(open) => {
                setCreateNotificationBotDialogOpen(open);
                if (!open) {
                  setNewNotificationBot({ name: "", botToken: "", enabled: true });
                }
              }}
            >
              <DialogContent className="rounded-2xl border border-[#2a2c3c] bg-[#13141c] text-white">
                <DialogHeader>
                  <DialogTitle className="inline-flex items-center gap-2">
                    <Radio size={18} /> Telegram Notifier Bot
                  </DialogTitle>
                </DialogHeader>
                <div className="space-y-4">
                  <div className="space-y-2">
                    <AdminLabel>Tenant</AdminLabel>
                    <Select
                      value={notificationBotTenantId}
                      onValueChange={(value) => setNotificationBotTenantId(value)}
                    >
                      <SelectTrigger data-testid="select-notification-bot-tenant-dialog">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent searchable searchPlaceholder="Search tenant...">
                        {tenants.map((tenant: any) => (
                          <SelectItem key={tenant.id} value={tenant.id}>
                            {tenant.slug || tenant.id}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>Bot Name</AdminLabel>
                    <AdminInput
                      data-testid="input-notification-bot-name"
                      placeholder="Tenant Alerts Bot"
                      value={newNotificationBot.name}
                      onChange={(e) => setNewNotificationBot((prev) => ({ ...prev, name: e.target.value }))}
                    />
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>Telegram Bot Token</AdminLabel>
                    <AdminInput
                      data-testid="input-notification-bot-token"
                      placeholder="123456:ABCDEF..."
                      value={newNotificationBot.botToken}
                      onChange={(e) => setNewNotificationBot((prev) => ({ ...prev, botToken: e.target.value }))}
                    />
                    <p className="text-xs text-[#9ca3af]">
                      Token is validated with Telegram getMe before bot is saved.
                    </p>
                  </div>
                  <div className="flex items-center justify-between rounded-lg border px-3 py-2">
                    <div>
                      <p className="text-sm font-medium">Enabled</p>
                      <p className="text-xs text-[#9ca3af]">Available for user notification settings</p>
                    </div>
                    <Switch
                      data-testid="switch-notification-bot-enabled"
                      checked={newNotificationBot.enabled}
                      onCheckedChange={(checked) => setNewNotificationBot((prev) => ({ ...prev, enabled: checked }))}
                    />
                  </div>
                </div>

                <DialogFooter>
                  <AdminButton variant="outline" onClick={() => setCreateNotificationBotDialogOpen(false)}>
                    {t("common.cancel")}
                  </AdminButton>
                  <AdminButton
                    data-testid="button-create-notification-bot"
                    onClick={handleCreateNotificationBot}
                    disabled={createAdminNotificationBot.isPending}
                  >
                    <Save size={16} className="mr-2" /> {createAdminNotificationBot.isPending ? t("common.loading") : "Create Bot"}
                  </AdminButton>
                </DialogFooter>
              </DialogContent>
            </Dialog>

            <AdminCard className="p-6">
              <div className="mb-4 flex flex-wrap items-center gap-2">
                <h3 className="font-bold text-lg">Tenant Notification Recipients</h3>
                {notificationBotTenantId ? (
                  <Badge variant="outline" className="text-[10px]">
                    {tenantSlugByID[notificationBotTenantId] || notificationBotTenantId}
                  </Badge>
                ) : null}
              </div>
              <p className="text-sm text-[#9ca3af] mb-4">
                Configure delivery channel for each tenant user. Telegram requires bot + chat/username, email requires mailbox, Time requires recipient ID.
              </p>
              <div className="mb-4 grid gap-3 xl:grid-cols-[minmax(0,1fr)_180px_auto]">
                <div className="relative">
                  <Search size={14} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[#6b7280]" />
                  <AdminInput
                    data-testid="input-notification-recipient-search"
                    value={notificationRecipientSearchQuery}
                    onChange={(event) => setNotificationRecipientSearchQuery(event.target.value)}
                    placeholder="Search by name, email, or role"
                    className="pl-9"
                  />
                </div>
                <Select
                  value={notificationRecipientChannelFilter}
                  onValueChange={(value) => setNotificationRecipientChannelFilter(value as typeof notificationRecipientChannelFilter)}
                >
                  <SelectTrigger data-testid="select-notification-recipient-channel-filter">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">All channels</SelectItem>
                    <SelectItem value="enabled">Enabled only</SelectItem>
                    <SelectItem value="telegram">Telegram</SelectItem>
                    <SelectItem value="email">Email</SelectItem>
                    <SelectItem value="time">Time</SelectItem>
                    <SelectItem value="in_app">In app</SelectItem>
                  </SelectContent>
                </Select>
                <AdminButton
                  type="button"
                  variant="outline"
                  className="w-full xl:w-auto"
                  onClick={() => {
                    setNotificationRecipientSearchQuery("");
                    setNotificationRecipientChannelFilter("all");
                  }}
                  disabled={!hasNotificationRecipientFilters}
                >
                  Reset
                </AdminButton>
              </div>
              <div className="mb-4 flex flex-wrap items-center gap-2 text-xs text-[#9ca3af]">
                <span>Showing {filteredNotificationSettings.length} of {adminNotificationSettings.length} recipients</span>
                {notificationRecipientChannelFilter !== "all" ? (
                  <Badge variant="outline" className="border-[#3b82f6]/20 bg-[#3b82f6]/10 text-[#60a5fa]">
                    {notificationRecipientChannelFilter === "enabled" ? "Enabled only" : notificationRecipientChannelFilter}
                  </Badge>
                ) : null}
              </div>
              {showAdminNotificationSettingsLoading ? (
                <div className="space-y-3">
                  {[1, 2, 3].map((index) => (
                    <Skeleton key={index} className="h-28 w-full rounded-xl" />
                  ))}
                </div>
              ) : filteredNotificationSettings.length === 0 ? (
                <AdminCard className="p-6 text-center text-[#9ca3af]">
                  <p className="text-sm">{adminNotificationSettings.length === 0 ? "No tenant users found for notification settings" : "No recipients match the current filters"}</p>
                </AdminCard>
              ) : (
                <div className="space-y-3">
                  {filteredNotificationSettings.map((item: any) => {
                    const userId = String(item?.userId || item?.user_id || "");
                    const draft = notificationSettingDrafts[userId] || item;
                    const deliveryChannel = String(draft?.deliveryChannel || "in_app").toLowerCase();
                    const isTelegram = deliveryChannel === "telegram";
                    const isEmail = deliveryChannel === "email";
                    const isTime = deliveryChannel === "time";
                    return (
                      <AdminCard key={userId} className="p-4 border border-[#2a2c3c]">
                        <div className="flex flex-wrap items-center justify-between gap-3">
                          <div className="min-w-0">
                            <div className="font-medium text-sm">{draft?.userName || "User"}</div>
                            <div className="text-xs text-[#9ca3af]">{draft?.userEmail || "—"}</div>
                            <div className="mt-1 flex flex-wrap items-center gap-2">
                              <Badge variant="outline" className="text-[10px]">{String(draft?.userRole || "").toLowerCase() || "analyst"}</Badge>
                              <Badge variant={draft?.deliveryEnabled ? "default" : "secondary"} className="text-[10px]">
                                {draft?.deliveryEnabled ? "Delivery enabled" : "Delivery disabled"}
                              </Badge>
                            </div>
                          </div>
                          <AdminButton
                            data-testid={`button-save-notification-setting-${userId}`}
                            size="sm"
                            onClick={() => handleSaveNotificationSetting(userId)}
                            disabled={saveAdminNotificationSetting.isPending}
                          >
                            <Save size={14} className="mr-1" /> Save
                          </AdminButton>
                        </div>
                        <div className="mt-4 grid gap-3 md:grid-cols-4">
                          <div className="flex items-center justify-between rounded-lg border px-3 py-2 md:col-span-1">
                            <AdminLabel className="text-sm">Enabled</AdminLabel>
                            <Switch
                              data-testid={`switch-notification-setting-enabled-${userId}`}
                              checked={Boolean(draft?.deliveryEnabled)}
                              disabled={saveAdminNotificationSetting.isPending}
                              onCheckedChange={(checked) => {
                                const previousEnabled = Boolean(draft?.deliveryEnabled);
                                const nextDraft = {
                                  ...draft,
                                  deliveryEnabled: checked,
                                };
                                handleNotificationSettingDraftChange(userId, { deliveryEnabled: checked });
                                const submitted = handleSaveNotificationSetting(
                                  userId,
                                  nextDraft,
                                  { successMessage: checked ? "Delivery enabled" : "Delivery disabled" },
                                );
                                if (!submitted) {
                                  handleNotificationSettingDraftChange(userId, { deliveryEnabled: previousEnabled });
                                }
                              }}
                            />
                          </div>
                          <div className="space-y-1 md:col-span-1">
                            <AdminLabel className="text-xs">Channel</AdminLabel>
                            <Select
                              value={deliveryChannel}
                              onValueChange={(value) => handleNotificationSettingDraftChange(userId, { deliveryChannel: value })}
                            >
                              <SelectTrigger data-testid={`select-notification-setting-channel-${userId}`}>
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value="in_app">In app</SelectItem>
                                <SelectItem value="telegram">Telegram</SelectItem>
                                <SelectItem value="email">Email</SelectItem>
                                <SelectItem value="time">Time</SelectItem>
                              </SelectContent>
                            </Select>
                          </div>
                          <div className="space-y-1 md:col-span-2">
                            <AdminLabel className="text-xs">Telegram bot</AdminLabel>
                            <Select
                              value={String(draft?.telegramBotId || "__none__")}
                              onValueChange={(value) =>
                                handleNotificationSettingDraftChange(userId, {
                                  telegramBotId: value === "__none__" ? "" : value,
                                })
                              }
                              disabled={!isTelegram}
                            >
                              <SelectTrigger data-testid={`select-notification-setting-bot-${userId}`}>
                                <SelectValue placeholder="Select bot" />
                              </SelectTrigger>
                              <SelectContent searchable searchPlaceholder="Search bot...">
                                <SelectItem value="__none__">Select bot</SelectItem>
                                {notificationBots.map((bot: any) => (
                                  <SelectItem key={bot.id} value={String(bot.id)}>
                                    {bot.name} {bot.enabled ? "" : "(disabled)"}
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          </div>
                          <div className="space-y-1 md:col-span-2">
                            <AdminLabel className="text-xs">Telegram chat ID</AdminLabel>
                            <AdminInput
                              data-testid={`input-notification-setting-chat-${userId}`}
                              placeholder="e.g. 777000001"
                              value={String(draft?.telegramChatId || "")}
                              onChange={(e) => handleNotificationSettingDraftChange(userId, { telegramChatId: e.target.value })}
                              disabled={!isTelegram}
                            />
                          </div>
                          <div className="space-y-1 md:col-span-2">
                            <AdminLabel className="text-xs">Telegram username</AdminLabel>
                            <AdminInput
                              data-testid={`input-notification-setting-username-${userId}`}
                              placeholder="@soc_operator"
                              value={String(draft?.telegramUsername || "")}
                              onChange={(e) => handleNotificationSettingDraftChange(userId, { telegramUsername: e.target.value })}
                              disabled={!isTelegram}
                            />
                          </div>
                          <div className="space-y-1 md:col-span-2">
                            <AdminLabel className="text-xs">Notification email</AdminLabel>
                            <AdminInput
                              data-testid={`input-notification-setting-email-${userId}`}
                              placeholder="analyst@example.com"
                              value={String(draft?.notificationEmail || "")}
                              onChange={(e) => handleNotificationSettingDraftChange(userId, { notificationEmail: e.target.value })}
                              disabled={!isEmail}
                            />
                          </div>
                          <div className="space-y-1 md:col-span-2">
                            <AdminLabel className="text-xs">Time recipient</AdminLabel>
                            <AdminInput
                              data-testid={`input-notification-setting-time-${userId}`}
                              placeholder="secops-room"
                              value={String(draft?.timeRecipient || "")}
                              onChange={(e) => handleNotificationSettingDraftChange(userId, { timeRecipient: e.target.value })}
                              disabled={!isTime}
                            />
                          </div>
                        </div>
                      </AdminCard>
                    );
                  })}
                </div>
              )}
            </AdminCard>
          </TabsContent>

          {/* ═══════════════════ TAB 6: SERVICE ALERTS ═══════════════════ */}
          <TabsContent value="service_alerts" className="space-y-6" id="admin-legacy-service-alerts">
            <div className="space-y-4">
              <AdminCard className="p-5">
                  <div className="flex flex-col gap-5">
                    <div className="flex flex-col gap-4 xl:flex-row xl:flex-wrap xl:items-start xl:justify-between">
                      <div>
                        <h3 className="font-bold text-lg">Configured Service Alert Rules</h3>
                        <p className="mt-1 text-sm text-[#9ca3af]">
                          Keep noisy rules under control, filter by metric, and edit only the alerts that matter for this tenant.
                        </p>
                      </div>
                      <div className="grid w-full max-w-[640px] gap-3 sm:grid-cols-2 xl:max-w-none xl:grid-cols-4">
                        <div className="min-w-[120px] rounded-xl border border-[#252a3d] bg-[#10131c] p-3">
                          <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Rules</p>
                          <p className="mt-2 text-2xl font-semibold text-white">{serviceAlertRules.length}</p>
                        </div>
                        <div className="min-w-[120px] rounded-xl border border-[#252a3d] bg-[#10131c] p-3">
                          <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Enabled</p>
                          <p className="mt-2 text-2xl font-semibold text-[#66ff4c]">{enabledServiceAlertRulesCount}</p>
                        </div>
                        <div className="min-w-[120px] rounded-xl border border-[#252a3d] bg-[#10131c] p-3">
                          <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Disabled</p>
                          <p className="mt-2 text-2xl font-semibold text-[#f59e0b]">{Math.max(serviceAlertRules.length - enabledServiceAlertRulesCount, 0)}</p>
                        </div>
                        <div className="min-w-[120px] rounded-xl border border-[#252a3d] bg-[#10131c] p-3">
                          <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Metrics</p>
                          <p className="mt-2 text-2xl font-semibold text-[#60a5fa]">{serviceAlertMetricOptions.length}</p>
                        </div>
                      </div>
                    </div>

                    <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_180px_180px_240px_auto]">
                      <div className="relative">
                        <Search size={14} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[#6b7280]" />
                        <AdminInput
                          data-testid="input-service-alert-search"
                          value={serviceAlertSearchQuery}
                          onChange={(event) => setServiceAlertSearchQuery(event.target.value)}
                          placeholder="Search by rule name, metric, module, or severity"
                          className="pl-9"
                        />
                      </div>
                      <Select
                        value={serviceAlertStatusFilter}
                        onValueChange={(value) => setServiceAlertStatusFilter(value as typeof serviceAlertStatusFilter)}
                      >
                        <SelectTrigger data-testid="select-service-alert-status-filter">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="all">All statuses</SelectItem>
                          <SelectItem value="enabled">Enabled</SelectItem>
                          <SelectItem value="disabled">Disabled</SelectItem>
                        </SelectContent>
                      </Select>
                      <Select
                        value={serviceAlertMetricFilter}
                        onValueChange={(value) => setServiceAlertMetricFilter(value)}
                      >
                        <SelectTrigger data-testid="select-service-alert-metric-filter">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent searchable searchPlaceholder="Filter metrics...">
                          <SelectItem value="all">All metrics</SelectItem>
                          {serviceAlertMetricOptions.map((metric) => (
                            <SelectItem key={metric} value={metric}>{metric}</SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <Select value={serviceAlertTenantId} onValueChange={(value) => setServiceAlertTenantId(value)}>
                        <SelectTrigger data-testid="select-service-alert-tenant">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent searchable searchPlaceholder="Search tenant...">
                          {tenants.map((tenant: any) => (
                            <SelectItem key={tenant.id} value={tenant.id}>
                              {tenant.slug || tenant.id}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <div className="flex items-center gap-2">
                        <AdminButton
                          type="button"
                          variant="outline"
                          className="w-full xl:w-auto"
                          onClick={() => {
                            setServiceAlertSearchQuery("");
                            setServiceAlertStatusFilter("all");
                            setServiceAlertMetricFilter("all");
                          }}
                          disabled={!hasServiceAlertFilters}
                        >
                          Reset
                        </AdminButton>
                        <AdminButton
                          type="button"
                          className="w-full xl:w-auto"
                          onClick={() => setCreateServiceAlertDialogOpen(true)}
                          data-testid="button-new-service-alert-rule"
                        >
                          <Plus size={14} className="mr-2" /> New Rule
                        </AdminButton>
                      </div>
                    </div>

                    <div className="flex flex-wrap items-center gap-2 text-xs text-[#9ca3af]">
                      <span>Showing {filteredServiceAlertRules.length} of {serviceAlertRules.length} rules</span>
                      {serviceAlertTenantId ? (
                        <Badge variant="outline" className="border-primary/20 bg-primary/10 text-primary">
                          {tenantSlugByID[serviceAlertTenantId] || serviceAlertTenantId}
                        </Badge>
                      ) : null}
                      {serviceAlertStatusFilter !== "all" ? (
                        <Badge variant="outline" className="border-[#3b82f6]/20 bg-[#3b82f6]/10 text-[#60a5fa]">
                          {serviceAlertStatusFilter}
                        </Badge>
                      ) : null}
                      {serviceAlertMetricFilter !== "all" ? (
                        <Badge variant="outline" className="border-[#2f3549] bg-[#10131c] text-[#d1d5db]">
                          {serviceAlertMetricFilter}
                        </Badge>
                      ) : null}
                    </div>
                  </div>
                </AdminCard>
                {showServiceAlertRulesLoading ? (
                  <div className="space-y-3">
                    {[1, 2].map((index) => (
                      <Skeleton key={index} className="h-44 w-full rounded-xl" />
                    ))}
                  </div>
                ) : filteredServiceAlertRules.length === 0 ? (
                  <AdminCard className="p-8 text-center text-[#9ca3af]">
                    <AlertTriangle size={30} className="mx-auto mb-2 opacity-30" />
                    <p className="text-sm">{serviceAlertRules.length === 0 ? "No service alert rules configured yet" : "No rules match the current filters"}</p>
                  </AdminCard>
                ) : (
                  <div className="space-y-3">
                    {filteredServiceAlertRules.map((rule: any) => {
                      const draft = serviceAlertRuleDrafts[rule.id] || rule;
                      const metricType = normalizeServiceAlertMetric(draft.metricType);
                      return (
                        <AdminCard key={rule.id} className="p-4 border border-[#2a2c3c]">
                          <div className="space-y-3">
                            <div className="flex items-center gap-2 flex-wrap">
                              <Badge variant={draft.enabled ? "default" : "secondary"} className="text-[10px]">
                                {draft.enabled ? "Enabled" : "Disabled"}
                              </Badge>
                              <Badge variant="outline" className="text-[10px]">{draft.metricType}</Badge>
                              <Badge variant="outline" className="text-[10px]">{draft.module}</Badge>
                              <Badge variant="outline" className="text-[10px]">{draft.severity}</Badge>
                            </div>
                            <div className="grid md:grid-cols-2 gap-3">
                              <AdminInput
                                value={draft.name || ""}
                                onChange={(e) => handleServiceAlertDraftChange(rule.id, { name: e.target.value })}
                              />
                              {(metricType === "low_rps" || metricType === "high_latency") ? (
                                <AdminInput
                                  value={draft.module || ""}
                                  placeholder="api / cache / search / async_ops"
                                  onChange={(e) => handleServiceAlertDraftChange(rule.id, { module: e.target.value })}
                                />
                              ) : (
                                <AdminInput
                                  value={String(draft.openStatuses || "")}
                                  placeholder="open statuses: new,triaged,in_progress"
                                  onChange={(e) => handleServiceAlertDraftChange(rule.id, { openStatuses: e.target.value })}
                                />
                              )}
                            </div>
                            <AdminTextarea
                              value={draft.description || ""}
                              onChange={(e) => handleServiceAlertDraftChange(rule.id, { description: e.target.value })}
                            />
                            <div className="grid md:grid-cols-2 gap-3">
                              <Select
                                value={draft.metricType}
                                onValueChange={(value) => handleServiceAlertDraftChange(rule.id, { metricType: value })}
                              >
                                <SelectTrigger>
                                  <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                  <SelectItem value="low_rps">Low RPS</SelectItem>
                                  <SelectItem value="high_latency">High Latency</SelectItem>
                                  <SelectItem value="sla">SLA</SelectItem>
                                  <SelectItem value="threshold">Case Threshold</SelectItem>
                                  <SelectItem value="criticality">Criticality</SelectItem>
                                  <SelectItem value="escalation">Escalation</SelectItem>
                                  <SelectItem value="ping">Ping</SelectItem>
                                </SelectContent>
                              </Select>
                              <Select
                                value={draft.severity}
                                onValueChange={(value) => handleServiceAlertDraftChange(rule.id, { severity: value })}
                              >
                                <SelectTrigger>
                                  <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                  <SelectItem value="info">Info</SelectItem>
                                  <SelectItem value="warning">Warning</SelectItem>
                                  <SelectItem value="critical">Critical</SelectItem>
                                </SelectContent>
                              </Select>
                            </div>
                            <div className="grid md:grid-cols-4 gap-3 items-start">
                              {metricType === "low_rps" ? (
                                <AdminInput
                                  type="number"
                                  min={0}
                                  step="0.01"
                                  value={Number(draft.minRps ?? 0)}
                                  onChange={(e) => handleServiceAlertDraftChange(rule.id, { minRps: Number(e.target.value || 0) })}
                                />
                              ) : null}
                              {metricType === "high_latency" ? (
                                <AdminInput
                                  type="number"
                                  min={1}
                                  step="1"
                                  value={Number(draft.maxLatencyMs ?? 0)}
                                  onChange={(e) => handleServiceAlertDraftChange(rule.id, { maxLatencyMs: Number(e.target.value || 0) })}
                                />
                              ) : null}
                              {metricType === "sla" ? (
                                <AdminInput
                                  type="number"
                                  min={1}
                                  step="1"
                                  value={Number(draft.slaSeconds ?? 0)}
                                  onChange={(e) => handleServiceAlertDraftChange(rule.id, { slaSeconds: Number(e.target.value || 0) })}
                                />
                              ) : null}
                              {(metricType === "sla" || metricType === "threshold" || metricType === "criticality" || metricType === "escalation" || metricType === "ping") ? (
                                <AdminInput
                                  type="number"
                                  min={1}
                                  step="1"
                                  value={Number(draft.casesThreshold ?? 0)}
                                  onChange={(e) => handleServiceAlertDraftChange(rule.id, { casesThreshold: Number(e.target.value || 0) })}
                                />
                              ) : null}
                              {metricType === "threshold" ? (
                                <Select
                                  value={String(draft.thresholdMode || "in_work")}
                                  onValueChange={(value) => handleServiceAlertDraftChange(rule.id, { thresholdMode: value })}
                                >
                                  <SelectTrigger>
                                    <SelectValue />
                                  </SelectTrigger>
                                  <SelectContent>
                                    <SelectItem value="in_work">In work</SelectItem>
                                    <SelectItem value="total">Total</SelectItem>
                                  </SelectContent>
                                </Select>
                              ) : null}
                              {metricType === "criticality" ? (
                                <AdminInput
                                  placeholder="critical,high"
                                  value={String(draft.criticalityLevels || "")}
                                  onChange={(e) => handleServiceAlertDraftChange(rule.id, { criticalityLevels: e.target.value })}
                                />
                              ) : null}
                              {metricType === "escalation" ? (
                                <AdminInput
                                  placeholder="case_escalated,case_escalation_received"
                                  value={String(draft.escalationEventTypes || "")}
                                  onChange={(e) => handleServiceAlertDraftChange(rule.id, { escalationEventTypes: e.target.value })}
                                />
                              ) : null}
                              {metricType === "ping" ? (
                                <AdminInput
                                  type="number"
                                  min={1}
                                  step="1"
                                  value={Number(draft.pingInactivitySeconds ?? 0)}
                                  onChange={(e) => handleServiceAlertDraftChange(rule.id, { pingInactivitySeconds: Number(e.target.value || 0) })}
                                />
                              ) : null}
                              <AdminInput
                                type="number"
                                min={30}
                                value={Number(draft.windowSeconds ?? 300)}
                                onChange={(e) => handleServiceAlertDraftChange(rule.id, { windowSeconds: Number(e.target.value || 300) })}
                              />
                              <AdminInput
                                type="number"
                                min={30}
                                value={Number(draft.cooldownSeconds ?? 900)}
                                onChange={(e) => handleServiceAlertDraftChange(rule.id, { cooldownSeconds: Number(e.target.value || 900) })}
                              />
                              <div className="flex items-center justify-between rounded-md border px-3">
                                <span className="text-xs text-[#9ca3af]">Enabled</span>
                                <Switch
                                  data-testid={`switch-service-alert-enabled-${rule.id}`}
                                  checked={Boolean(draft.enabled)}
                                  disabled={updateServiceAlertRule.isPending}
                                  onCheckedChange={(checked) => {
                                    const previousEnabled = Boolean(draft.enabled);
                                    const nextDraft = {
                                      ...draft,
                                      enabled: checked,
                                    };
                                    handleServiceAlertDraftChange(rule.id, { enabled: checked });
                                    const submitted = handleSaveServiceAlertRule(
                                      rule.id,
                                      nextDraft,
                                      { successMessage: checked ? "Service alert rule enabled" : "Service alert rule disabled" },
                                    );
                                    if (!submitted) {
                                      handleServiceAlertDraftChange(rule.id, { enabled: previousEnabled });
                                    }
                                  }}
                                />
                              </div>
                            </div>
                            <div className="flex items-center justify-end gap-2">
                              <AdminButton
                                size="sm"
                                variant="outline"
                                onClick={() => handleSaveServiceAlertRule(rule.id)}
                                disabled={updateServiceAlertRule.isPending}
                              >
                                <Save size={14} className="mr-1" /> Save
                              </AdminButton>
                              <AdminButton
                                size="sm"
                                variant="destructive"
                                onClick={() => handleDeleteServiceAlertRule(rule.id, String(rule.tenantId || serviceAlertTenantId || ""))}
                                disabled={deleteServiceAlertRule.isPending}
                              >
                                <Trash2 size={14} className="mr-1" /> Delete
                              </AdminButton>
                            </div>
                          </div>
                        </AdminCard>
                      );
                    })}
                  </div>
                )}
            </div>

            <Dialog
              open={createServiceAlertDialogOpen}
              onOpenChange={(open) => {
                setCreateServiceAlertDialogOpen(open);
              }}
            >
              <DialogContent className="max-h-[90vh] overflow-y-auto rounded-2xl border border-[#2a2c3c] bg-[#13141c] text-white">
                <DialogHeader>
                  <DialogTitle className="inline-flex items-center gap-2">
                    <AlertTriangle size={18} /> New Service Alert Rule
                  </DialogTitle>
                  <DialogDescription className="text-[#9ca3af]">
                    Create a tenant-scoped rule, then fine-tune thresholds inline.
                  </DialogDescription>
                </DialogHeader>

                <div className="space-y-4">
                  <div className="space-y-2">
                    <AdminLabel>Tenant</AdminLabel>
                    <Select value={serviceAlertTenantId} onValueChange={(value) => setServiceAlertTenantId(value)}>
                      <SelectTrigger data-testid="select-service-alert-tenant-dialog">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent searchable searchPlaceholder="Search tenant...">
                        {tenants.map((tenant: any) => (
                          <SelectItem key={tenant.id} value={tenant.id}>
                            {tenant.slug || tenant.id}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>

                  <div className="space-y-2">
                    <AdminLabel>Rule Name</AdminLabel>
                    <AdminInput
                      data-testid="input-service-alert-name"
                      value={newServiceAlertRule.name}
                      onChange={(e) => setNewServiceAlertRule((prev) => ({ ...prev, name: e.target.value }))}
                    />
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>Description</AdminLabel>
                    <AdminTextarea
                      data-testid="input-service-alert-description"
                      value={newServiceAlertRule.description}
                      onChange={(e) => setNewServiceAlertRule((prev) => ({ ...prev, description: e.target.value }))}
                    />
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>Metric Type</AdminLabel>
                    <Select
                      value={newServiceAlertRule.metricType}
                      onValueChange={(value) => setNewServiceAlertRule((prev) => ({ ...prev, metricType: value }))}
                    >
                      <SelectTrigger data-testid="select-service-alert-metric">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="low_rps">Low RPS</SelectItem>
                        <SelectItem value="high_latency">High Latency</SelectItem>
                        <SelectItem value="sla">SLA</SelectItem>
                        <SelectItem value="threshold">Case Threshold</SelectItem>
                        <SelectItem value="criticality">Criticality</SelectItem>
                        <SelectItem value="escalation">Escalation</SelectItem>
                        <SelectItem value="ping">Ping</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>

                  {(newServiceAlertRule.metricType === "low_rps" || newServiceAlertRule.metricType === "high_latency") ? (
                    <div className="space-y-2">
                      <AdminLabel>Module</AdminLabel>
                      <AdminInput
                        data-testid="input-service-alert-module"
                        value={newServiceAlertRule.module}
                        placeholder="api / cache / search / async_ops"
                        onChange={(e) => setNewServiceAlertRule((prev) => ({ ...prev, module: e.target.value }))}
                      />
                    </div>
                  ) : null}

                  {newServiceAlertRule.metricType === "low_rps" ? (
                    <div className="space-y-2">
                      <AdminLabel>Min RPS</AdminLabel>
                      <AdminInput
                        data-testid="input-service-alert-min-rps"
                        type="number"
                        min={0}
                        step="0.01"
                        value={newServiceAlertRule.minRps}
                        onChange={(e) => setNewServiceAlertRule((prev) => ({ ...prev, minRps: Number(e.target.value || 0) }))}
                      />
                    </div>
                  ) : null}

                  {newServiceAlertRule.metricType === "high_latency" ? (
                    <div className="space-y-2">
                      <AdminLabel>Max Latency (ms)</AdminLabel>
                      <AdminInput
                        data-testid="input-service-alert-max-latency"
                        type="number"
                        min={1}
                        step="1"
                        value={newServiceAlertRule.maxLatencyMs}
                        onChange={(e) => setNewServiceAlertRule((prev) => ({ ...prev, maxLatencyMs: Number(e.target.value || 0) }))}
                      />
                    </div>
                  ) : null}

                  {newServiceAlertRule.metricType === "sla" ? (
                    <>
                      <div className="space-y-2">
                        <AdminLabel>SLA (sec)</AdminLabel>
                        <AdminInput
                          type="number"
                          min={1}
                          step="1"
                          value={newServiceAlertRule.slaSeconds}
                          onChange={(e) => setNewServiceAlertRule((prev) => ({ ...prev, slaSeconds: Number(e.target.value || 0) }))}
                        />
                      </div>
                      <div className="space-y-2">
                        <AdminLabel>Cases threshold</AdminLabel>
                        <AdminInput
                          type="number"
                          min={1}
                          step="1"
                          value={newServiceAlertRule.casesThreshold}
                          onChange={(e) => setNewServiceAlertRule((prev) => ({ ...prev, casesThreshold: Number(e.target.value || 0) }))}
                        />
                      </div>
                    </>
                  ) : null}

                  {newServiceAlertRule.metricType === "threshold" ? (
                    <>
                      <div className="space-y-2">
                        <AdminLabel>Cases threshold</AdminLabel>
                        <AdminInput
                          type="number"
                          min={1}
                          step="1"
                          value={newServiceAlertRule.casesThreshold}
                          onChange={(e) => setNewServiceAlertRule((prev) => ({ ...prev, casesThreshold: Number(e.target.value || 0) }))}
                        />
                      </div>
                      <div className="space-y-2">
                        <AdminLabel>Threshold mode</AdminLabel>
                        <Select
                          value={newServiceAlertRule.thresholdMode}
                          onValueChange={(value) => setNewServiceAlertRule((prev) => ({ ...prev, thresholdMode: value }))}
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="in_work">In work</SelectItem>
                            <SelectItem value="total">Total</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                    </>
                  ) : null}

                  {newServiceAlertRule.metricType === "criticality" ? (
                    <>
                      <div className="space-y-2">
                        <AdminLabel>Criticality levels (comma-separated)</AdminLabel>
                        <AdminInput
                          placeholder="critical,high"
                          value={newServiceAlertRule.criticalityLevels}
                          onChange={(e) => setNewServiceAlertRule((prev) => ({ ...prev, criticalityLevels: e.target.value }))}
                        />
                      </div>
                      <div className="space-y-2">
                        <AdminLabel>Cases threshold</AdminLabel>
                        <AdminInput
                          type="number"
                          min={1}
                          step="1"
                          value={newServiceAlertRule.casesThreshold}
                          onChange={(e) => setNewServiceAlertRule((prev) => ({ ...prev, casesThreshold: Number(e.target.value || 0) }))}
                        />
                      </div>
                    </>
                  ) : null}

                  {newServiceAlertRule.metricType === "escalation" ? (
                    <>
                      <div className="space-y-2">
                        <AdminLabel>Escalation event types (comma-separated)</AdminLabel>
                        <AdminInput
                          placeholder="case_escalated,case_escalation_received"
                          value={newServiceAlertRule.escalationEventTypes}
                          onChange={(e) => setNewServiceAlertRule((prev) => ({ ...prev, escalationEventTypes: e.target.value }))}
                        />
                      </div>
                      <div className="space-y-2">
                        <AdminLabel>Event threshold</AdminLabel>
                        <AdminInput
                          type="number"
                          min={1}
                          step="1"
                          value={newServiceAlertRule.casesThreshold}
                          onChange={(e) => setNewServiceAlertRule((prev) => ({ ...prev, casesThreshold: Number(e.target.value || 0) }))}
                        />
                      </div>
                    </>
                  ) : null}

                  {newServiceAlertRule.metricType === "ping" ? (
                    <>
                      <div className="space-y-2">
                        <AdminLabel>Ping inactivity (sec)</AdminLabel>
                        <AdminInput
                          type="number"
                          min={1}
                          step="1"
                          value={newServiceAlertRule.pingInactivitySeconds}
                          onChange={(e) => setNewServiceAlertRule((prev) => ({ ...prev, pingInactivitySeconds: Number(e.target.value || 0) }))}
                        />
                      </div>
                      <div className="space-y-2">
                        <AdminLabel>Cases threshold</AdminLabel>
                        <AdminInput
                          type="number"
                          min={1}
                          step="1"
                          value={newServiceAlertRule.casesThreshold}
                          onChange={(e) => setNewServiceAlertRule((prev) => ({ ...prev, casesThreshold: Number(e.target.value || 0) }))}
                        />
                      </div>
                    </>
                  ) : null}

                  {(newServiceAlertRule.metricType !== "low_rps" && newServiceAlertRule.metricType !== "high_latency") ? (
                    <div className="space-y-2">
                      <AdminLabel>Open statuses (comma-separated, optional)</AdminLabel>
                      <AdminInput
                        placeholder="new,triaged,in_progress"
                        value={newServiceAlertRule.openStatuses}
                        onChange={(e) => setNewServiceAlertRule((prev) => ({ ...prev, openStatuses: e.target.value }))}
                      />
                    </div>
                  ) : null}

                  <div className="grid grid-cols-2 gap-3">
                    <div className="space-y-2">
                      <AdminLabel>Window (sec)</AdminLabel>
                      <AdminInput
                        type="number"
                        min={30}
                        step="10"
                        value={newServiceAlertRule.windowSeconds}
                        onChange={(e) => setNewServiceAlertRule((prev) => ({ ...prev, windowSeconds: Number(e.target.value || 300) }))}
                      />
                    </div>
                    <div className="space-y-2">
                      <AdminLabel>Cooldown (sec)</AdminLabel>
                      <AdminInput
                        type="number"
                        min={30}
                        step="10"
                        value={newServiceAlertRule.cooldownSeconds}
                        onChange={(e) => setNewServiceAlertRule((prev) => ({ ...prev, cooldownSeconds: Number(e.target.value || 900) }))}
                      />
                    </div>
                  </div>

                  <div className="space-y-2">
                    <AdminLabel>Severity</AdminLabel>
                    <Select
                      value={newServiceAlertRule.severity}
                      onValueChange={(value) => setNewServiceAlertRule((prev) => ({ ...prev, severity: value }))}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="info">Info</SelectItem>
                        <SelectItem value="warning">Warning</SelectItem>
                        <SelectItem value="critical">Critical</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>

                  <div className="flex items-center justify-between rounded-lg border px-3 py-2">
                    <div>
                      <p className="text-sm font-medium">Enabled</p>
                      <p className="text-xs text-[#9ca3af]">Evaluate this rule in background</p>
                    </div>
                    <Switch
                      checked={newServiceAlertRule.enabled}
                      onCheckedChange={(checked) => setNewServiceAlertRule((prev) => ({ ...prev, enabled: checked }))}
                    />
                  </div>
                </div>

                <DialogFooter>
                  <AdminButton variant="outline" onClick={() => setCreateServiceAlertDialogOpen(false)}>
                    {t("common.cancel")}
                  </AdminButton>
                  <AdminButton
                    data-testid="button-create-service-alert"
                    onClick={handleCreateServiceAlertRule}
                    disabled={createServiceAlertRule.isPending}
                  >
                    <Save size={16} className="mr-2" /> {createServiceAlertRule.isPending ? t("common.loading") : "Create Rule"}
                  </AdminButton>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          </TabsContent>

          {/* ═══════════════════ TAB 7: API TOKENS ═══════════════════ */}
          <TabsContent value="api_tokens" className="space-y-6" id="admin-legacy-api-tokens">
            <div className="space-y-4">
              <AdminCard className="p-5">
                  <div className="flex flex-col gap-5">
                    <div className="flex flex-col gap-4 xl:flex-row xl:flex-wrap xl:items-start xl:justify-between">
                      <div>
                                                <p className="mt-1 text-sm text-[#9ca3af]">
                          Audit active access keys, find full-access tokens quickly, and revoke stale credentials before they drift.
                        </p>
                      </div>
                      <div className="grid w-full max-w-[640px] gap-3 sm:grid-cols-2 xl:max-w-none xl:grid-cols-4">
                        <div className="min-w-[120px] rounded-xl border border-[#252a3d] bg-[#10131c] p-3">
                          <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Tokens</p>
                          <p className="mt-2 text-2xl font-semibold text-white">{apiTokens.length}</p>
                        </div>
                        <div className="min-w-[120px] rounded-xl border border-[#252a3d] bg-[#10131c] p-3">
                          <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Full access</p>
                          <p className="mt-2 text-2xl font-semibold text-[#66ff4c]">{fullAccessTokenCount}</p>
                        </div>
                        <div className="min-w-[120px] rounded-xl border border-[#252a3d] bg-[#10131c] p-3">
                          <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Scoped</p>
                          <p className="mt-2 text-2xl font-semibold text-[#60a5fa]">{Math.max(apiTokens.length - fullAccessTokenCount, 0)}</p>
                        </div>
                        <div className="min-w-[120px] rounded-xl border border-[#252a3d] bg-[#10131c] p-3">
                          <p className="text-[11px] uppercase tracking-[0.08em] text-[#6b7280]">Expired</p>
                          <p className="mt-2 text-2xl font-semibold text-[#f59e0b]">{expiredApiTokenCount}</p>
                        </div>
                      </div>
                    </div>

                    <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_200px_auto]">
                      <div className="relative">
                        <Search size={14} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[#6b7280]" />
                        <AdminInput
                          data-testid="input-api-token-search"
                          value={apiTokenSearchQuery}
                          onChange={(event) => setApiTokenSearchQuery(event.target.value)}
                          placeholder="Search by token name, prefix, description, or scope"
                          className="pl-9"
                        />
                      </div>
                      <Select
                        value={apiTokenAccessFilter}
                        onValueChange={(value) => setApiTokenAccessFilter(value as typeof apiTokenAccessFilter)}
                      >
                        <SelectTrigger data-testid="select-api-token-filter">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="all">All access</SelectItem>
                          <SelectItem value="full_access">Full access</SelectItem>
                          <SelectItem value="scoped">Scoped</SelectItem>
                          <SelectItem value="expired">Expired</SelectItem>
                        </SelectContent>
                      </Select>
                      <div className="flex items-center gap-2">
                        <AdminButton
                          type="button"
                          variant="outline"
                          className="w-full xl:w-auto"
                          onClick={() => {
                            setApiTokenSearchQuery("");
                            setApiTokenAccessFilter("all");
                          }}
                          disabled={!hasApiTokenFilters}
                        >
                          Reset
                        </AdminButton>
                        <AdminButton
                          type="button"
                          className="w-full xl:w-auto"
                          onClick={() => setIssueApiTokenDialogOpen(true)}
                        >
                          <KeyRound size={14} className="mr-2" /> Issue Token
                        </AdminButton>
                      </div>
                    </div>

                    <div className="flex flex-wrap items-center gap-2 text-xs text-[#9ca3af]">
                      <span>Showing {filteredApiTokens.length} of {apiTokens.length} tokens</span>
                      {apiTokenAccessFilter !== "all" ? (
                        <Badge variant="outline" className="border-[#3b82f6]/20 bg-[#3b82f6]/10 text-[#60a5fa]">
                          {apiTokenAccessFilter}
                        </Badge>
                      ) : null}
                    </div>
                  </div>
                </AdminCard>
                {createdAPITokenSecret ? (
                  <AdminCard className="border-amber-300 bg-amber-50/60 dark:bg-amber-950/20 p-4">
                    <h4 className="font-semibold text-sm mb-2">Copy token now</h4>
                    <p className="text-xs text-[#9ca3af] mb-2">This value is shown only once.</p>
                    <div className="rounded border bg-[#0f131d] p-3 font-mono text-xs break-all" data-testid="text-created-api-token">
                      {createdAPITokenSecret}
                    </div>
                  </AdminCard>
                ) : null}
                {showAPITokensLoading ? (
                  <div className="space-y-3">
                    {[1, 2].map((i) => (
                      <Skeleton key={i} className="h-20 w-full rounded-xl" />
                    ))}
                  </div>
                ) : filteredApiTokens.length === 0 ? (
                  <AdminCard className="p-8 text-center text-[#9ca3af]">
                    <KeyRound size={30} className="mx-auto mb-2 opacity-30" />
                    <p className="text-sm">{apiTokens.length === 0 ? "No API tokens issued yet" : "No API tokens match the current filters"}</p>
                  </AdminCard>
                ) : (
                  <div className="space-y-3">
                    {filteredApiTokens.map((token: any) => (
                      <AdminCard key={token.id} className="p-4 border bg-[#111622]">
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0">
                            <EllipsisText text={token.name} className="font-semibold text-sm max-w-[320px]" />
                            <EllipsisText text={token.description || "—"} className="text-xs text-[#9ca3af] max-w-[360px]" />
                            <div className="mt-2 flex flex-wrap gap-2 text-xs">
                              <Badge variant="outline">Prefix: {token.tokenPrefix || "—"}</Badge>
                              <Badge variant="outline">Created: {token.createdAt ? new Date(token.createdAt).toLocaleString() : "—"}</Badge>
                              <Badge variant="outline">Last used: {token.lastUsedAt ? new Date(token.lastUsedAt).toLocaleString() : "never"}</Badge>
                              <Badge variant="outline">Expires: {token.expiresAt ? new Date(token.expiresAt).toLocaleString() : "never"}</Badge>
                              {token.fullAccess ? (
                                <Badge className="bg-emerald-500/20 text-emerald-700 border-emerald-300">Full access</Badge>
                              ) : null}
                            </div>
                            {!token.fullAccess && token.scopes?.length ? (
                              <div className="mt-2 flex flex-wrap gap-1">
                                {token.scopes.map((scope: string) => (
                                  <Badge key={`${token.id}-${scope}`} variant="secondary" className="text-[10px]">
                                    {scope}
                                  </Badge>
                                ))}
                              </div>
                            ) : null}
                          </div>
                          <AdminButton
                            data-testid={`button-revoke-api-token-${token.id}`}
                            variant="destructive"
                            size="sm"
                            onClick={() => handleRevokeAPIToken(token.id)}
                            disabled={revokeAdminApiToken.isPending}
                          >
                            <Trash2 size={14} className="mr-1" /> Revoke
                          </AdminButton>
                        </div>
                      </AdminCard>
                    ))}
                  </div>
                )}
            </div>

            <Dialog
              open={issueApiTokenDialogOpen}
              onOpenChange={(open) => {
                setIssueApiTokenDialogOpen(open);
                if (!open) {
                  setNewAPIToken({
                    name: "",
                    description: "",
                    fullAccess: false,
                    scopes: ["alerts:write", "cases:write"],
                    expiresAt: "",
                  });
                }
              }}
            >
              <DialogContent className="max-h-[90vh] overflow-y-auto rounded-2xl border border-[#2a2c3c] bg-[#13141c] text-white">
                <DialogHeader>
                  <DialogTitle className="inline-flex items-center gap-2">
                    <KeyRound size={18} /> Issue API Token
                  </DialogTitle>
                  <DialogDescription className="text-[#9ca3af]">
                    Create an access token and copy the secret immediately after issuance.
                  </DialogDescription>
                </DialogHeader>

                <div className="space-y-4">
                  <div className="space-y-2">
                    <AdminLabel>Name</AdminLabel>
                    <AdminInput
                      data-testid="input-api-token-name"
                      placeholder="Automation token"
                      value={newAPIToken.name}
                      onChange={(e) => setNewAPIToken({ ...newAPIToken, name: e.target.value })}
                    />
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>Description</AdminLabel>
                    <AdminTextarea
                      data-testid="input-api-token-description"
                      placeholder="What this token is used for"
                      value={newAPIToken.description}
                      onChange={(e) => setNewAPIToken({ ...newAPIToken, description: e.target.value })}
                    />
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>Expires At (optional, RFC3339)</AdminLabel>
                    <AdminInput
                      data-testid="input-api-token-expires-at"
                      placeholder="2026-12-31T23:59:59Z"
                      value={newAPIToken.expiresAt}
                      onChange={(e) => setNewAPIToken({ ...newAPIToken, expiresAt: e.target.value })}
                    />
                  </div>
                  <div className="flex items-center justify-between rounded-lg border px-3 py-2">
                    <div>
                      <p className="text-sm font-medium">Full Access</p>
                      <p className="text-xs text-[#9ca3af]">All API except user-account endpoints</p>
                    </div>
                    <Switch
                      data-testid="switch-api-token-full-access"
                      checked={newAPIToken.fullAccess}
                      onCheckedChange={(checked) => setNewAPIToken({ ...newAPIToken, fullAccess: checked })}
                    />
                  </div>
                  {!newAPIToken.fullAccess ? (
                    <div className="space-y-2">
                      <AdminLabel>Scopes</AdminLabel>
                      <div className="max-h-56 overflow-y-auto rounded-lg border p-3 space-y-2">
                        {API_TOKEN_SCOPE_OPTIONS.map((scope) => {
                          const checked = newAPIToken.scopes.includes(scope.key);
                          return (
                            <label key={scope.key} className="flex items-center gap-2 text-sm">
                              <Checkbox
                                checked={checked}
                                onCheckedChange={(value) => handleToggleApiScope(scope.key, value === true)}
                              />
                              <span>{scope.label}</span>
                              <span className="ml-auto text-xs text-[#9ca3af]">{scope.key}</span>
                            </label>
                          );
                        })}
                      </div>
                    </div>
                  ) : null}
                </div>

                <DialogFooter>
                  <AdminButton variant="outline" onClick={() => setIssueApiTokenDialogOpen(false)}>
                    {t("common.cancel")}
                  </AdminButton>
                  <AdminButton
                    data-testid="button-create-api-token"
                    onClick={handleCreateAPIToken}
                    disabled={createAdminApiToken.isPending}
                  >
                    <KeyRound size={16} className="mr-2" /> {createAdminApiToken.isPending ? t("common.loading") : "Issue Token"}
                  </AdminButton>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          </TabsContent>

          {/* ═══════════════════ TAB 6: LOAD MANAGEMENT ═══════════════════ */}
          <TabsContent value="load" className="space-y-6" id="admin-legacy-load">
            <div className="space-y-4">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <h3 className="font-bold text-lg">Rate Limits</h3>
                  <p className="mt-1 text-sm text-[#9ca3af]">Throttle tenants/users to protect the platform during bursts.</p>
                </div>
                <AdminButton
                  type="button"
                  className="h-10"
                  onClick={() => setCreateRateLimitDialogOpen(true)}
                  data-testid="button-new-rate-limit"
                >
                  <Plus size={14} className="mr-2" /> New Rate Limit
                </AdminButton>
              </div>
                {showRateLimitsLoading ? (
                  <div className="space-y-3">
                    {[1, 2].map(i => <Skeleton key={i} className="h-16 w-full rounded-xl" />)}
                  </div>
                ) : rateLimits.length === 0 ? (
                  <AdminCard className="p-8 text-center text-[#9ca3af]">
                    <Shield size={32} className="mx-auto mb-2 opacity-30" />
                    <p className="text-sm">No rate limits configured</p>
                  </AdminCard>
                ) : (
                  <div className="space-y-3">
                    {rateLimits.map((rl: any) => (
                      <AdminCard key={rl.id} data-testid={`card-rate-limit-${rl.id}`} className="p-4 flex items-center justify-between">
                        <div className="flex items-center gap-3">
                          <div className={`p-2 rounded-lg ${rl.enabled ? "bg-green-100 text-green-700" : "bg-[#171b2a] text-[#9ca3af]"}`}>
                            <Shield size={16} />
                          </div>
                          <div>
                            <div className="font-bold text-sm flex items-center gap-2">
                              <Badge variant="outline" className="text-[10px]">{rl.targetType}</Badge>
                              <span>{rl.targetId}</span>
                            </div>
                            <p className="text-xs text-[#9ca3af] flex items-center gap-2">
                              <span>{rl.maxRequests.toLocaleString()} req</span>
                              <span>/ {rl.windowSeconds}s</span>
                              <Clock size={10} />
                            </p>
                          </div>
                        </div>
                        <div className="flex items-center gap-3">
                          <Switch
                            data-testid={`switch-rate-limit-${rl.id}`}
                            checked={rl.enabled}
                            onCheckedChange={(checked: boolean) => {
                              updateRateLimit.mutate({ id: rl.id, data: { enabled: checked } });
                              toast.success(checked ? "Rate limit enabled" : "Rate limit disabled");
                            }}
                          />
                          <AdminButton
                            data-testid={`button-delete-rate-limit-${rl.id}`}
                            variant="ghost"
                            size="sm"
                            className="text-red-500 hover:text-red-700"
                            onClick={() => { deleteRateLimit.mutate(rl.id); toast.success("Rate limit deleted"); }}
                          >
                            <Trash2 size={14} />
                          </AdminButton>
                        </div>
                      </AdminCard>
                    ))}
	                  </div>
	                )}
	            </div>

            <Dialog
              open={createRateLimitDialogOpen}
              onOpenChange={(open) => {
                setCreateRateLimitDialogOpen(open);
                if (!open) {
                  setNewRateLimit({ targetType: "tenant", targetId: "", maxRequests: 1000, windowSeconds: 3600 });
                }
              }}
            >
              <DialogContent className="rounded-2xl border border-[#2a2c3c] bg-[#13141c] text-white">
                <DialogHeader>
                  <DialogTitle className="inline-flex items-center gap-2">
                    <Shield size={18} /> New Rate Limit
                  </DialogTitle>
                </DialogHeader>
                <div className="space-y-4">
                  <div className="space-y-2">
                    <AdminLabel>Target Type</AdminLabel>
                    <Select
                      value={newRateLimit.targetType}
                      onValueChange={(v: any) => setNewRateLimit({ ...newRateLimit, targetType: v, targetId: "" })}
                    >
                      <AdminSelectTrigger data-testid="select-rate-target-type">
                        <SelectValue />
                      </AdminSelectTrigger>
                      <SelectContent>
                        <SelectItem value="tenant">Tenant</SelectItem>
                        <SelectItem value="user">User</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>Target</AdminLabel>
                    <Select value={newRateLimit.targetId} onValueChange={(v) => setNewRateLimit({ ...newRateLimit, targetId: v })}>
                      <AdminSelectTrigger data-testid="select-rate-target-id">
                        <SelectValue placeholder="Select target..." />
                      </AdminSelectTrigger>
                      <SelectContent searchable searchPlaceholder="Search...">
                        {newRateLimit.targetType === "tenant"
                          ? tenants.map((tenant: any) => (
                              <SelectItem key={tenant.id} value={tenant.id}>
                                {tenant.slug || tenant.id}
                              </SelectItem>
                            ))
                          : users.map((user: any) => (
                              <SelectItem key={user.id} value={user.id}>
                                {user.name}
                              </SelectItem>
                            ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>Max Requests</AdminLabel>
                    <AdminInput
                      data-testid="input-rate-max"
                      type="number"
                      value={newRateLimit.maxRequests}
                      onChange={(e) => setNewRateLimit({ ...newRateLimit, maxRequests: parseInt(e.target.value) || 1000 })}
                    />
                  </div>
                  <div className="space-y-2">
                    <AdminLabel>Window (seconds)</AdminLabel>
                    <AdminInput
                      data-testid="input-rate-window"
                      type="number"
                      value={newRateLimit.windowSeconds}
                      onChange={(e) => setNewRateLimit({ ...newRateLimit, windowSeconds: parseInt(e.target.value) || 3600 })}
                    />
                  </div>
                </div>

                <DialogFooter>
                  <AdminButton variant="outline" onClick={() => setCreateRateLimitDialogOpen(false)}>
                    {t("common.cancel")}
                  </AdminButton>
                  <AdminButton
                    data-testid="button-create-rate-limit"
                    onClick={handleCreateRateLimit}
                    disabled={createRateLimit.isPending}
                  >
                    <Save size={16} className="mr-2" /> {createRateLimit.isPending ? t("common.loading") : t("admin.button.createRateLimit")}
                  </AdminButton>
                </DialogFooter>
              </DialogContent>
            </Dialog>

	            <div className="grid lg:grid-cols-3 gap-6">
	              <AdminCard className="p-6 lg:col-span-1">
	                <h3 className="font-bold text-lg mb-4 flex items-center gap-2">
	                  <Shield size={18} /> SOC Access Policy
	                </h3>
	                <div className="space-y-4">
	                  <div className="space-y-2">
	                    <AdminLabel>Allowed case tags (comma-separated)</AdminLabel>
	                    <AdminInput
	                      data-testid="input-soc-policy-allowed-tags"
	                      placeholder="prod-only, fraud, endpoint"
	                      value={socPolicyDraft.allowedCaseTags}
	                      onChange={(e) => setSocPolicyDraft((prev) => ({ ...prev, allowedCaseTags: e.target.value }))}
	                    />
	                    <p className="text-[11px] text-[#9ca3af]">
	                      Analysts see only cases that have at least one tag from this allowlist. Empty value disables tag segmentation.
	                    </p>
	                  </div>
	                  <div className="space-y-2">
	                    <AdminLabel>Max in-work cases per analyst (0 = unlimited)</AdminLabel>
	                    <AdminInput
	                      data-testid="input-soc-policy-max-cases"
	                      type="number"
	                      min={0}
	                      value={socPolicyDraft.maxCasesInWork}
	                      onChange={(e) => setSocPolicyDraft((prev) => ({ ...prev, maxCasesInWork: e.target.value }))}
	                    />
	                  </div>
	                  <AdminButton
	                    data-testid="button-save-soc-policy"
	                    className="w-full"
	                    onClick={handleSaveSOCAccessPolicy}
	                  >
	                    <Save size={16} className="mr-2" /> Save policy
	                  </AdminButton>
	                </div>
	              </AdminCard>
	              <AdminCard className="p-6 lg:col-span-2">
	                <h4 className="font-semibold text-base mb-3">Policy effect</h4>
	                <div className="space-y-2 text-sm text-[#9ca3af]">
	                  <p>
	                    Tag segmentation and workload limits apply to analysts. Tenant admins and platform admins keep full access.
	                  </p>
	                  <p>
	                    Enforcement runs in backend APIs, so restrictions work even if requests are sent outside the web UI.
	                  </p>
	                </div>
	              </AdminCard>
	            </div>
	          </TabsContent>
        </Tabs>

        {canFactoryResetLocal ? (
          <AdminCard className="border border-red-500/30 bg-[linear-gradient(180deg,rgba(45,16,19,0.95),rgba(25,11,14,0.95))] p-6">
            <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
              <div className="space-y-2">
                <div className="flex items-center gap-2 text-red-200">
                  <AlertTriangle size={18} />
                  <h3 className="text-lg font-semibold text-white">Factory Reset Local Data</h3>
                </div>
                <p className="max-w-[780px] text-sm text-red-100/80">
                  Local and dev only. This removes operational tenant data, connectors, cases, alerts, forum threads, communications, workflows, and non-admin tenants so you can test from a clean install.
                </p>
              </div>
              <AdminButton
                variant="destructive"
                onClick={() => setFactoryResetDialogOpen(true)}
                data-testid="button-open-factory-reset-local"
              >
                <Trash2 size={16} className="mr-2" /> Factory Reset Local Data
              </AdminButton>
            </div>
          </AdminCard>
        ) : null}
      </div>

      <Dialog open={stopDialogOpen} onOpenChange={(open) => {
        setStopDialogOpen(open);
        if (!open) {
          setStopTargetTenantId("");
        }
      }}>
        <DialogContent className="rounded-xl border border-[#2a2c3c] bg-[#13141c] text-white">
          <DialogHeader>
            <DialogTitle>Stop tenant</DialogTitle>
            <DialogDescription className="text-[#9ca3af]">
              This will put tenant <span className="font-semibold text-[#f3f4f6]">{stopTargetTenantSlug || "selected"}</span> into maintenance mode.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <AdminButton variant="outline" onClick={() => setStopDialogOpen(false)}>Cancel</AdminButton>
            <AdminButton
              variant="destructive"
              disabled={!stopTargetTenantId || updateTenant.isPending}
              onClick={handleStopTenant}
              data-testid="button-confirm-stop-tenant"
            >
              <Square size={16} className="mr-2" /> Stop tenant
            </AdminButton>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={canFactoryResetLocal && factoryResetDialogOpen} onOpenChange={(open) => {
        setFactoryResetDialogOpen(canFactoryResetLocal && open);
        if (!open) {
          setFactoryResetConfirmation("");
        }
      }}>
        <DialogContent className="rounded-xl border border-[#2a2c3c] bg-[#13141c] text-white">
          <DialogHeader>
            <DialogTitle>Factory Reset Local Data</DialogTitle>
            <DialogDescription className="text-[#9ca3af]">
              Type <span className="font-semibold text-[#f3f4f6]">RESET LOCAL DATA</span> to confirm the destructive local reset.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <AdminLabel>Confirmation phrase</AdminLabel>
            <AdminInput
              value={factoryResetConfirmation}
              onChange={(event) => setFactoryResetConfirmation(event.target.value)}
              placeholder="RESET LOCAL DATA"
              data-testid="input-factory-reset-confirmation"
            />
          </div>
          <DialogFooter>
            <AdminButton variant="outline" onClick={() => setFactoryResetDialogOpen(false)}>Cancel</AdminButton>
            <AdminButton
              variant="destructive"
              disabled={factoryResetConfirmation.trim() !== "RESET LOCAL DATA" || factoryResetLocal.isPending}
              onClick={() => {
                factoryResetLocal.mutate(factoryResetConfirmation.trim(), {
                  onSuccess: () => {
                    toast.success("Local data reset completed");
                    setFactoryResetDialogOpen(false);
                    setFactoryResetConfirmation("");
                  },
                  onError: (error: any) => toast.error(error?.message || "Failed to reset local data"),
                });
              }}
              data-testid="button-confirm-factory-reset-local"
            >
              <Trash2 size={16} className="mr-2" /> Reset local data
            </AdminButton>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </AppLayout>
  );
}
