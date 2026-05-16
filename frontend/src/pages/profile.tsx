import { AppLayout } from "@/components/layout";
import { Card } from "@/components/ui/card";
import {
  useAppState,
  useUser,
  useUpdateUser,
  useUserAchievements,
  useUploadUserMedia,
  useInfiniteUserExperienceEvents,
  useUserCasePerformance,
  useUserSpecializationBadges,
  useSaveUserSpecializationBadges,
  useUserProfileBio,
  useSaveUserProfileBio,
} from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useState, useEffect, useMemo, useRef, type ChangeEvent, type UIEvent } from "react";
import { toast } from "sonner";
import {
  Camera,
  Award,
  Edit2,
  X,
  Save,
  ArrowUpRight,
  ArrowDownRight,
  Minus,
  ShieldCheck,
  Sparkles,
  Activity,
  Trophy,
  Clock3,
  Shield,
  Star,
  ArrowRight,
  BriefcaseBusiness,
  Users,
  Bell,
  GraduationCap,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Skeleton } from "@/components/ui/skeleton";
import { useT } from "@/lib/i18n";
import { formatAvgResponseMinutes } from "@/lib/dashboard";
import { isAchievementImageIcon } from "@/lib/achievement-icons";
import { useMinimumLoading } from "@/lib/use-minimum-loading";

const RARITY_ORDER: Record<string, number> = { Diamond: 0, Legendary: 1, Epic: 2, Rare: 3, Common: 4 };
const XP_PER_LEVEL = 1000;
const MAX_LEVEL = 100;
const ACHIEVEMENTS_PAGE_SIZE = 4;
const RECENT_ACTIVITY_LIMIT = 4;
const EXPERIENCE_HISTORY_PAGE_SIZE = 20;
const EXPERIENCE_HISTORY_SCROLL_THRESHOLD = 0.7;
const MAX_SPECIALIZATION_BADGES = 16;
const DEFAULT_PROFILE_BIO = "Cybersecurity professional specializing in\nthreat hunting and incident response.\nPassionate about protecting digital assets.";
const PROFILE_PAGE_SHELL_CLASS = "mx-auto w-full max-w-[1184px] space-y-6 pb-6";
const PROFILE_CANVAS_CLASS =
  "overflow-hidden rounded-[12px] border border-[rgba(255,255,255,0.05)] bg-[#0b0c10] shadow-[0_12px_30px_rgba(0,0,0,0.35)]";
const PROFILE_PANEL_CLASS = "rounded-xl border border-[rgba(255,255,255,0.05)] bg-[rgba(19,20,28,0.95)] shadow-[0_4px_20px_rgba(0,0,0,0.3)]";
const PROFILE_SUBPANEL_CLASS = "rounded-lg border border-[#2a2c3c] bg-[rgba(11,12,16,0.5)]";
const PROFILE_MUTED_TEXT_CLASS = "text-xs text-[#6b7280]";
const PROFILE_OUTLINE_BUTTON_CLASS = "border-[#2a2c3c] bg-[#13141c] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]";
const PROFILE_INPUT_CLASS =
  "h-[38px] border-[#2a2c3c] bg-[#0b0c10] text-[#f3f4f6] placeholder:text-[#6b7280] focus-visible:ring-1 focus-visible:ring-[#3b4a79]";

function getRarityStyles(rarity: string) {
  switch (rarity) {
    case "Diamond":
      return "border-2 border-cyan-400 bg-[rgba(11,12,16,0.5)] shadow-[0_0_20px_rgba(59,130,246,0.3)]";
    case "Legendary":
      return "border-2 border-[#ffc700] bg-[rgba(11,12,16,0.5)] shadow-[0_0_20px_rgba(255,199,0,0.3)]";
    case "Epic":
      return "border-2 border-[#a855f7] bg-[rgba(11,12,16,0.5)] shadow-[0_0_20px_rgba(168,85,247,0.3)]";
    case "Rare":
      return "border-2 border-[#3b82f6] bg-[rgba(11,12,16,0.5)] shadow-[0_0_20px_rgba(59,130,246,0.3)]";
    default:
      return "border-2 border-[#6b7280] bg-[rgba(11,12,16,0.5)] opacity-50";
  }
}

function getAchievementIconShellClass(rarity: string) {
  switch (rarity) {
    case "Diamond":
      return "bg-gradient-to-br from-cyan-500 to-purple-500";
    case "Legendary":
      return "bg-gradient-to-br from-[#ffc700] to-orange-500";
    case "Epic":
      return "bg-gradient-to-br from-[#a855f7] to-pink-600";
    case "Rare":
      return "bg-gradient-to-br from-[#3b82f6] to-cyan-500";
    default:
      return "bg-gradient-to-br from-[#4b5563] to-[#374151]";
  }
}

function getRarityTextClass(rarity: string) {
  switch (rarity) {
    case "Diamond":
      return "text-cyan-400";
    case "Legendary":
      return "text-[#ffc700]";
    case "Epic":
      return "text-[#a855f7]";
    case "Rare":
      return "text-[#3b82f6]";
    default:
      return "text-[#9ca3af]";
  }
}

function normalizeSpecializationBadge(input: string): string {
  return input.trim().replace(/\s+/g, " ").slice(0, 40);
}

function sanitizeSpecializationBadges(raw: unknown): string[] {
  if (!Array.isArray(raw)) {
    return [];
  }
  return Array.from(
    new Set(
      raw
        .map((badge) => normalizeSpecializationBadge(String(badge ?? "")))
        .filter(Boolean),
    ),
  ).slice(0, MAX_SPECIALIZATION_BADGES);
}

function getQuickStatsValueClass(label: string) {
  if (label === "Current Streak") {
    return "text-[#ffc700]";
  }
  if (label === "Rank") {
    return "text-[#66ff4c]";
  }
  return "text-white";
}

type LevelProgress = {
  level: number;
  tier: number;
  totalXP: number;
  progressXP: number;
  progressMaxXP: number;
  progressPercent: number;
  toNextLevelXP: number;
  isMaxLevel: boolean;
};

function buildLevelProgress(rawXP: unknown): LevelProgress {
  const parsedXP = Number(rawXP ?? 0);
  const totalXP = Number.isFinite(parsedXP) && parsedXP > 0 ? Math.floor(parsedXP) : 0;
  const unclampedLevel = Math.floor(totalXP / XP_PER_LEVEL) + 1;
  const level = Math.min(Math.max(unclampedLevel, 1), MAX_LEVEL);
  const isMaxLevel = level >= MAX_LEVEL;
  const progressXP = isMaxLevel ? XP_PER_LEVEL : totalXP % XP_PER_LEVEL;
  const progressPercent = isMaxLevel ? 100 : Math.min(100, Math.max(0, (progressXP / XP_PER_LEVEL) * 100));
  const toNextLevelXP = isMaxLevel ? 0 : XP_PER_LEVEL - progressXP;
  const tier = isMaxLevel ? 10 : Math.min(10, Math.max(1, Math.ceil(level / 10)));
  return {
    level,
    tier,
    totalXP,
    progressXP,
    progressMaxXP: XP_PER_LEVEL,
    progressPercent,
    toNextLevelXP,
    isMaxLevel,
  };
}

function LevelBadge({ level, tier: _tier, isMaxLevel }: { level: number; tier: number; isMaxLevel: boolean }) {
  if (isMaxLevel) {
    return (
      <div className="flex h-10 w-[46px] shrink-0 items-center justify-center rounded-full border-4 border-[#0b0c10] bg-[#ffc700] shadow-[0_0_10px_rgba(255,199,0,0.2)]" data-testid="badge-level-diamond">
        <span className="text-[13px] font-semibold text-[#0b0c10]">100</span>
      </div>
    );
  }

  return (
    <div
      className="flex h-10 w-[46px] shrink-0 items-center justify-center rounded-full border-4 border-[#0b0c10] bg-[#ffc700] shadow-[0_0_10px_rgba(255,199,0,0.2)]"
      data-testid="badge-level"
    >
      <span className="text-[14px] font-semibold leading-none text-[#0b0c10]">{level}</span>
    </div>
  );
}

type MetricTrend = {
  state: "up" | "down" | "flat";
  tone: "positive" | "negative" | "neutral";
  label: string;
};

function closedCasesTrend(current: number, previous: number): MetricTrend {
  const delta = current - previous;
  if (delta > 0) {
    return { state: "up", tone: "positive", label: `+${delta}` };
  }
  if (delta < 0) {
    return { state: "down", tone: "negative", label: `${delta}` };
  }
  return { state: "flat", tone: "neutral", label: "0" };
}

function avgInvestigationTrend(current: number | null, previous: number | null): MetricTrend {
  if (current === null || previous === null) {
    return { state: "flat", tone: "neutral", label: "—" };
  }
  const delta = current - previous;
  if (delta < 0) {
    return { state: "down", tone: "positive", label: `-${Math.abs(delta).toFixed(2)}m` };
  }
  if (delta > 0) {
    return { state: "up", tone: "negative", label: `+${delta.toFixed(2)}m` };
  }
  return { state: "flat", tone: "neutral", label: "0m" };
}

function TrendPill({ trend }: { trend: MetricTrend }) {
  const toneClass = trend.tone === "positive"
    ? "text-[#66ff4c]"
    : trend.tone === "negative"
      ? "text-[#eb5f65]"
      : "text-[#6b7280]";
  const Icon = trend.state === "up" ? ArrowUpRight : trend.state === "down" ? ArrowDownRight : Minus;
  return (
    <span className={`inline-flex items-center gap-1 text-[12px] ${toneClass}`}>
      <Icon size={11} />
      {trend.label}
    </span>
  );
}

type ActivityVisualTone = {
  text: string;
  bg: string;
  icon: any;
};

function getActivityTone(index: number): ActivityVisualTone {
  const tones: ActivityVisualTone[] = [
    { text: "#66ff4c", bg: "rgba(102,255,76,0.2)", icon: Trophy },
    { text: "#eb5f65", bg: "rgba(235,95,101,0.2)", icon: BriefcaseBusiness },
    { text: "#3b82f6", bg: "rgba(59,130,246,0.2)", icon: Users },
    { text: "#ffc700", bg: "rgba(255,199,0,0.2)", icon: Bell },
    { text: "#a855f7", bg: "rgba(168,85,247,0.2)", icon: Star },
    { text: "#66ff4c", bg: "rgba(102,255,76,0.2)", icon: GraduationCap },
  ];
  return tones[index % tones.length];
}

export default function ProfilePage() {
  const { currentUserId } = useAppState();
  return <UserProfileView userId={currentUserId} editable />;
}

type UserProfileViewProps = {
  userId: string;
  editable?: boolean;
  backHref?: string;
};

type EditableProfileField = "name" | "email" | "team" | "personalLink";
type ProfileFieldDrafts = {
  name: string;
  email: string;
  team: string;
  personalLink: string;
  bio: string;
};

export function UserProfileView({ userId, editable = false, backHref }: UserProfileViewProps) {
  const t = useT();
  const { data: currentUser, isLoading } = useUser(userId);
  const { data: achievements = [] } = useUserAchievements(userId);
  const { data: storedProfileBio = "" } = useUserProfileBio(userId);
  const { data: specializationBadges = [], isLoading: specializationBadgesLoading } = useUserSpecializationBadges(userId);
  const updateUser = useUpdateUser();
  const uploadUserMedia = useUploadUserMedia();
  const saveSpecializationBadges = useSaveUserSpecializationBadges(userId);
  const saveUserProfileBio = useSaveUserProfileBio(userId);

  const [activeProfileField, setActiveProfileField] = useState<EditableProfileField | null>(null);
  const [profileFieldDrafts, setProfileFieldDrafts] = useState<ProfileFieldDrafts>({
    name: "",
    email: "",
    team: "",
    personalLink: "",
    bio: DEFAULT_PROFILE_BIO,
  });
  const [achievementsPage, setAchievementsPage] = useState(1);
  const [experienceHistoryOpen, setExperienceHistoryOpen] = useState(false);
  const [isProfileInfoEditing, setIsProfileInfoEditing] = useState(false);
  const [isSpecializationsEditing, setIsSpecializationsEditing] = useState(false);
  const [specializationInput, setSpecializationInput] = useState("");
  const [specializationDrafts, setSpecializationDrafts] = useState<string[]>([]);
  const avatarFileRef = useRef<HTMLInputElement | null>(null);
  const coverFileRef = useRef<HTMLInputElement | null>(null);
  const {
    data: experienceHistoryPages,
    isLoading: experienceHistoryLoading,
    isFetchingNextPage: experienceHistoryLoadingMore,
    hasNextPage: experienceHistoryHasMore = false,
    fetchNextPage: fetchNextExperienceHistoryPage,
  } = useInfiniteUserExperienceEvents(userId, EXPERIENCE_HISTORY_PAGE_SIZE);
  const experienceHistory = useMemo(() => {
    const pages = experienceHistoryPages?.pages ?? [];
    return pages.flatMap((page: any) => (Array.isArray(page?.items) ? page.items : []));
  }, [experienceHistoryPages]);
  const { data: casePerformance } = useUserCasePerformance(userId);
  const showProfileLoadingSkeleton = useMinimumLoading(isLoading);

  useEffect(() => {
    if (currentUser) {
      setProfileFieldDrafts({
        name: String(currentUser.name || "").trim(),
        email: String(currentUser.email || "").trim(),
        team: String(currentUser.team || "SOC").trim(),
        personalLink: String(currentUser.personalLink || "").trim(),
        bio: String(storedProfileBio || DEFAULT_PROFILE_BIO).trim() || DEFAULT_PROFILE_BIO,
      });
    }
  }, [currentUser, storedProfileBio]);

  useEffect(() => {
    if (isSpecializationsEditing) {
      return;
    }
    const normalized = sanitizeSpecializationBadges(specializationBadges);
    setSpecializationDrafts((prev) => {
      if (prev.length === normalized.length && prev.every((item, index) => item === normalized[index])) {
        return prev;
      }
      return normalized;
    });
  }, [isSpecializationsEditing, specializationBadges]);

  const sortedAchievements = useMemo(() => {
    return [...achievements].sort((a: any, b: any) => {
      const ra = RARITY_ORDER[a.achievement?.rarity] ?? 5;
      const rb = RARITY_ORDER[b.achievement?.rarity] ?? 5;
      return ra - rb;
    });
  }, [achievements]);

  const totalAchievementsPages = useMemo(
    () => Math.max(1, Math.ceil(sortedAchievements.length / ACHIEVEMENTS_PAGE_SIZE)),
    [sortedAchievements.length],
  );
  const displayedAchievements = useMemo(
    () =>
      sortedAchievements.slice(
        (achievementsPage - 1) * ACHIEVEMENTS_PAGE_SIZE,
        achievementsPage * ACHIEVEMENTS_PAGE_SIZE,
      ),
    [sortedAchievements, achievementsPage],
  );
  const levelProgress = useMemo(() => buildLevelProgress(currentUser?.experiencePoints), [currentUser?.experiencePoints]);
  const closedCaseTrend = useMemo(
    () => closedCasesTrend(
      Number(casePerformance?.currentMonthClosedCases ?? 0),
      Number(casePerformance?.previousMonthClosedCases ?? 0),
    ),
    [casePerformance?.currentMonthClosedCases, casePerformance?.previousMonthClosedCases],
  );
  const avgInvestigationTrendValue = useMemo(
    () => avgInvestigationTrend(
      casePerformance?.currentMonthAvgInvestigationMinutes ?? null,
      casePerformance?.previousMonthAvgInvestigationMinutes ?? null,
    ),
    [casePerformance?.currentMonthAvgInvestigationMinutes, casePerformance?.previousMonthAvgInvestigationMinutes],
  );
  const profileRoleLabel = useMemo(() => {
    const raw = String(currentUser?.role || "Threat Hunter").trim();
    if (!raw) {
      return "Threat Hunter";
    }
    return raw
      .split(/\s+/)
      .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
      .join(" ");
  }, [currentUser?.role]);
  const levelTitle = useMemo(() => `Level ${levelProgress.level}`, [levelProgress.level]);
  const levelSummaryLine = useMemo(() => `${profileRoleLabel} • ${levelTitle} Elite Analyst`, [profileRoleLabel, levelTitle]);
  const achievementCards = useMemo(() => displayedAchievements, [displayedAchievements]);
  const unlockedCount = useMemo(() => sortedAchievements.length, [sortedAchievements.length]);
  const totalAchievements = 150;
  const accuracyScore = useMemo(() => {
    const closedTotal = Number(casePerformance?.closedCasesTotal ?? 0);
    if (closedTotal <= 0) return "98.2%";
    return `${Math.min(99.9, 94 + Math.log10(closedTotal + 1) * 2).toFixed(1)}%`;
  }, [casePerformance?.closedCasesTotal]);
  const quickStats = useMemo(
    () => [
      { label: "Total XP Earned", value: levelProgress.totalXP.toLocaleString() },
      { label: "Current Streak", value: `${Math.max(3, Math.min(60, Math.round(levelProgress.level * 0.8)))} days` },
      { label: "Longest Streak", value: `${Math.max(7, Math.min(120, Math.round(levelProgress.level * 2.1)))} days` },
      { label: "Rank", value: "#3 Global" },
    ],
    [levelProgress.level, levelProgress.totalXP],
  );
  const specializations = useMemo(
    () => (isSpecializationsEditing ? specializationDrafts : sanitizeSpecializationBadges(specializationBadges)),
    [isSpecializationsEditing, specializationBadges, specializationDrafts],
  );
  const recentActivity = useMemo(
    () => experienceHistory.slice(0, RECENT_ACTIVITY_LIMIT),
    [experienceHistory],
  );
  const hasAchievementPages = totalAchievementsPages > 1;
  const profileDepartment = useMemo(() => {
    if (String(currentUser?.team || "").trim()) {
      return currentUser.team;
    }
    if (String(currentUser?.role || "").toLowerCase().includes("admin")) {
      return "Security Operations Center";
    }
    return "Security Operations Center";
  }, [currentUser?.role, currentUser?.team]);
  const profileLocation = useMemo(() => {
    if (String(currentUser?.personalLink || "").trim()) {
      return String(currentUser.personalLink).replace(/^https?:\/\//i, "");
    }
    return "New York, USA";
  }, [currentUser?.personalLink]);
  const profileBio = useMemo(
    () => String(profileFieldDrafts.bio || "").trim() || DEFAULT_PROFILE_BIO,
    [profileFieldDrafts.bio],
  );
  const joinedDate = useMemo(() => {
    if (!currentUser?.createdAt) return "—";
    const date = new Date(currentUser.createdAt);
    if (!Number.isFinite(date.getTime())) return "—";
    return date.toLocaleDateString("en-US", { month: "long", day: "numeric", year: "numeric" });
  }, [currentUser?.createdAt]);

  const handleExperienceHistoryScroll = (event: UIEvent<HTMLDivElement>) => {
    if (!experienceHistoryHasMore || experienceHistoryLoadingMore) {
      return;
    }
    const element = event.currentTarget;
    const maxHeight = Math.max(element.scrollHeight, 1);
    const progress = (element.scrollTop + element.clientHeight) / maxHeight;
    if (progress < EXPERIENCE_HISTORY_SCROLL_THRESHOLD) {
      return;
    }
    fetchNextExperienceHistoryPage().catch(() => undefined);
  };

  useEffect(() => {
    setAchievementsPage((current) => Math.min(current, totalAchievementsPages));
  }, [totalAchievementsPages]);

  const startEditProfileField = (field: EditableProfileField) => {
    if (!editable || !currentUser) {
      return;
    }
    setActiveProfileField(field);
  };

  const cancelEditProfileField = () => {
    if (!currentUser) {
      setActiveProfileField(null);
      return;
    }
    setProfileFieldDrafts({
      name: String(currentUser.name || "").trim(),
      email: String(currentUser.email || "").trim(),
      team: String(currentUser.team || "SOC").trim(),
      personalLink: String(currentUser.personalLink || "").trim(),
      bio: String(storedProfileBio || DEFAULT_PROFILE_BIO).trim() || DEFAULT_PROFILE_BIO,
    });
    setActiveProfileField(null);
  };

  const saveProfileField = (field: EditableProfileField) => {
    if (!currentUser) {
      return;
    }
    const value = String(profileFieldDrafts[field] || "").trim();
    if (field === "name" && !value) {
      toast.error(t("profile.field.name"));
      return;
    }
    updateUser.mutate(
      {
        id: currentUser.id,
        data: field === "name"
          ? { name: value }
          : field === "email"
            ? { email: value }
            : field === "team"
              ? { team: value || "SOC" }
              : { personalLink: value },
      },
      {
        onSuccess: () => {
          toast.success(t("profile.toast.updated"));
          setActiveProfileField(null);
        },
        onError: (err: any) => {
          toast.error(err?.message || "Failed to update profile");
        },
      },
    );
  };

  const startProfileInfoEdit = () => {
    if (!editable || !currentUser) {
      return;
    }
    setProfileFieldDrafts({
      name: String(currentUser.name || "").trim(),
      email: String(currentUser.email || "").trim(),
      team: String(currentUser.team || "SOC").trim(),
      personalLink: String(currentUser.personalLink || "").trim(),
      bio: String(storedProfileBio || DEFAULT_PROFILE_BIO).trim() || DEFAULT_PROFILE_BIO,
    });
    setIsProfileInfoEditing(true);
  };

  const cancelProfileInfoEdit = () => {
    if (!currentUser) {
      setIsProfileInfoEditing(false);
      return;
    }
    setProfileFieldDrafts({
      name: String(currentUser.name || "").trim(),
      email: String(currentUser.email || "").trim(),
      team: String(currentUser.team || "SOC").trim(),
      personalLink: String(currentUser.personalLink || "").trim(),
      bio: String(storedProfileBio || DEFAULT_PROFILE_BIO).trim() || DEFAULT_PROFILE_BIO,
    });
    setIsProfileInfoEditing(false);
  };

  const saveProfileInfo = () => {
    if (!currentUser) {
      return;
    }
    updateUser.mutate(
      {
        id: currentUser.id,
        data: {
          email: String(profileFieldDrafts.email || "").trim(),
          team: String(profileFieldDrafts.team || "").trim() || "SOC",
          personalLink: String(profileFieldDrafts.personalLink || "").trim(),
        },
      },
      {
        onSuccess: () => {
          saveUserProfileBio.mutate(String(profileFieldDrafts.bio || "").trim(), {
            onSuccess: () => {
              toast.success(t("profile.toast.updated"));
              setIsProfileInfoEditing(false);
            },
            onError: (err: any) => {
              toast.error(err?.message || "Failed to update profile bio");
            },
          });
        },
        onError: (err: any) => {
          toast.error(err?.message || "Failed to update profile");
        },
      },
    );
  };

  const handleMediaUpload = (kind: "avatar" | "cover") => (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file || !currentUser) {
      return;
    }
    uploadUserMedia.mutate(
      { id: currentUser.id, kind, file },
      {
        onSuccess: () => {
          toast.success(kind === "avatar" ? t("profile.toast.avatarUploaded") : t("profile.toast.coverUploaded"));
        },
        onError: (err: any) => {
          toast.error(err?.message || t("profile.toast.mediaUploadFailed"));
        },
      },
    );
    event.target.value = "";
  };

  const startSpecializationsEdit = () => {
    if (!editable) {
      return;
    }
    setSpecializationDrafts(sanitizeSpecializationBadges(specializationBadges));
    setSpecializationInput("");
    setIsSpecializationsEditing(true);
  };

  const cancelSpecializationsEdit = () => {
    setSpecializationDrafts(sanitizeSpecializationBadges(specializationBadges));
    setSpecializationInput("");
    setIsSpecializationsEditing(false);
  };

  const saveSpecializations = () => {
    const normalized = sanitizeSpecializationBadges(specializationDrafts);
    saveSpecializationBadges.mutate(normalized, {
      onSuccess: () => {
        toast.success("Specializations updated");
        setIsSpecializationsEditing(false);
      },
      onError: (err: any) => {
        toast.error(err?.message || "Failed to save specializations");
      },
    });
  };

  const handleAddSpecializationBadge = () => {
    const normalized = normalizeSpecializationBadge(specializationInput);
    if (!normalized) {
      return;
    }
    if (specializationDrafts.includes(normalized)) {
      setSpecializationInput("");
      return;
    }
    if (specializationDrafts.length >= MAX_SPECIALIZATION_BADGES) {
      toast.error(`You can add up to ${MAX_SPECIALIZATION_BADGES} specialization badges`);
      return;
    }
    setSpecializationDrafts((prev) => sanitizeSpecializationBadges([...prev, normalized]));
    setSpecializationInput("");
  };

  const handleRemoveSpecializationBadge = (badge: string) => {
    setSpecializationDrafts((prev) => prev.filter((item) => item !== badge));
  };


  if (showProfileLoadingSkeleton || !currentUser) {
    return (
      <AppLayout backHref={backHref}>
        <div className={PROFILE_PAGE_SHELL_CLASS}>
          <Skeleton className="h-48 w-full rounded-xl" />
          <div className="flex items-end gap-6 px-6 -mt-16">
            <Skeleton className="h-32 w-32 rounded-full" />
            <div className="space-y-2 pb-2">
              <Skeleton className="h-8 w-48" />
              <Skeleton className="h-5 w-32" />
            </div>
          </div>
          <div className="grid md:grid-cols-2 gap-6 px-6">
            <Skeleton className="h-64 w-full rounded-xl" />
            <Skeleton className="h-64 w-full rounded-xl" />
          </div>
        </div>
      </AppLayout>
    );
  }

  return (
    <AppLayout backHref={backHref}>
      <div className={PROFILE_PAGE_SHELL_CLASS}>
        {editable && (
          <>
            <input
              ref={avatarFileRef}
              type="file"
              accept="image/*"
              className="hidden"
              onChange={handleMediaUpload("avatar")}
              data-testid="input-upload-avatar"
            />
            <input
              ref={coverFileRef}
              type="file"
              accept="image/*"
              className="hidden"
              onChange={handleMediaUpload("cover")}
              data-testid="input-upload-cover"
            />
          </>
        )}
        <div className={PROFILE_CANVAS_CLASS}>
          <div
            className="relative h-[256px] w-full overflow-hidden"
            style={
              currentUser.coverImage
                ? { backgroundImage: `url(${currentUser.coverImage})`, backgroundSize: "cover", backgroundPosition: "center" }
                : { backgroundImage: "linear-gradient(12deg,#1d1e29 50%,#2a2c3c 120%)" }
            }
            data-testid="cover-image"
          >
            <div className="absolute inset-0 opacity-30" />
            <div className="absolute inset-0 bg-[linear-gradient(168deg,rgba(102,255,76,0.1)_0%,rgba(0,0,0,0)_35%,rgba(168,85,247,0.1)_70%)]" />
            {editable && (
              <button
                className="absolute right-4 top-4 flex h-[38px] w-[146px] items-center gap-2 rounded-lg border border-[#2a2c3c] bg-[rgba(19,20,28,0.8)] px-4 text-[14px] text-white"
                onClick={() => coverFileRef.current?.click()}
                disabled={uploadUserMedia.isPending}
                data-testid="button-change-cover"
              >
                <Camera size={14} />
                Change Cover
              </button>
            )}
          </div>

          <div className="relative -mt-20 px-8 pb-8">
            <div className="flex flex-col gap-3">
              <div className="flex flex-wrap items-start justify-between gap-4 lg:pl-[152px]">
                <div className="group rounded-xl border border-[rgba(75,85,99,0.45)] bg-[rgba(55,65,81,0.42)] px-4 py-3 shadow-[0_4px_18px_rgba(0,0,0,0.28)] backdrop-blur-[2px]">
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0 flex-1">
                      {editable && activeProfileField === "name" ? (
                        <div className="flex items-center gap-2">
                          <Input
                            className="h-10 border-[#2a2c3c] bg-[#0b0c10] text-[20px] text-white focus-visible:ring-1 focus-visible:ring-[#3b4a79]"
                            value={profileFieldDrafts.name}
                            onChange={(event) =>
                              setProfileFieldDrafts((prev) => ({ ...prev, name: event.target.value }))
                            }
                            data-testid="input-inline-name"
                          />
                          <Button
                            variant="outline"
                            size="icon"
                            className={PROFILE_OUTLINE_BUTTON_CLASS}
                            onClick={() => saveProfileField("name")}
                            disabled={updateUser.isPending}
                            data-testid="button-inline-save-name"
                          >
                            <Save size={14} />
                          </Button>
                          <Button
                            variant="outline"
                            size="icon"
                            className={PROFILE_OUTLINE_BUTTON_CLASS}
                            onClick={cancelEditProfileField}
                            disabled={updateUser.isPending}
                            data-testid="button-inline-cancel-name"
                          >
                            <X size={14} />
                          </Button>
                        </div>
                      ) : (
                        <h1 className="truncate text-[30px] font-normal leading-[36px] tracking-[-0.5px] text-white" data-testid="text-username">
                          {currentUser.name}
                        </h1>
                      )}
                    </div>
                    {editable && activeProfileField !== "name" ? (
                      <button
                        type="button"
                        className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-[#2a2c3c] bg-[rgba(19,20,28,0.85)] text-[#9ca3af] opacity-0 transition-all hover:border-[#4b5168] hover:bg-[#171b2a] hover:text-white group-hover:opacity-100"
                        onClick={() => startEditProfileField("name")}
                        data-testid="button-inline-edit-name"
                      >
                        <Edit2 size={14} />
                      </button>
                    ) : null}
                  </div>
                  <p className="mt-1 text-[14px] leading-[20px] tracking-[-0.5px] text-[#c7ceda]">{levelSummaryLine}</p>
                  <div className="mt-2 flex flex-wrap items-center gap-2">
                    <span className="inline-flex h-[26px] items-center gap-1 rounded-full border border-[rgba(75,85,99,0.4)] bg-[rgba(55,65,81,0.5)] px-3 text-[12px] text-[#66ff4c]" data-testid="badge-role">
                      <ShieldCheck size={12} />
                      SOC Team Lead
                    </span>
                    <span className="inline-flex h-[26px] items-center gap-1 rounded-full border border-[rgba(75,85,99,0.4)] bg-[rgba(55,65,81,0.5)] px-3 text-[12px] text-[#a855f7]">
                      <Sparkles size={12} />
                      Top Performer
                    </span>
                    <span className="inline-flex h-[26px] items-center rounded-full border border-[rgba(75,85,99,0.55)] bg-[rgba(31,41,55,0.6)] px-3 text-[12px] text-[#d1d5db]">
                      {String(currentUser.team || "SOC").trim() || "SOC"}
                    </span>
                  </div>
                </div>
              </div>

              <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:gap-6">
                <div className="relative group">
                  <Avatar className="h-32 w-32 rounded-2xl border-4 border-[#0b0c10] shadow-[0_4px_20px_rgba(0,0,0,0.3)]" data-testid="avatar-display">
                    <AvatarImage src={currentUser.avatar || undefined} className="object-cover" />
                    <AvatarFallback className="rounded-2xl bg-[#13141c] text-4xl font-bold text-white">
                      {currentUser.name.split(" ").map((n: string) => n[0]).join("")}
                    </AvatarFallback>
                  </Avatar>
                  <div className="absolute -bottom-[12px] left-[96px]">
                    <LevelBadge level={levelProgress.level} tier={levelProgress.tier} isMaxLevel={levelProgress.isMaxLevel} />
                  </div>
                  {editable && (
                    <button
                      className="absolute inset-0 flex items-center justify-center rounded-2xl bg-black/40 opacity-0 transition-opacity group-hover:opacity-100"
                      onClick={() => avatarFileRef.current?.click()}
                      disabled={uploadUserMedia.isPending}
                      data-testid="button-change-avatar"
                    >
                      <Camera size={20} className="text-white" />
                    </button>
                  )}
                </div>

                <div className="relative w-full lg:flex-1">
                  <button
                    type="button"
                    className="w-full rounded-xl border border-[rgba(255,255,255,0.05)] bg-[rgba(19,20,28,0.95)] p-4 text-left"
                    data-testid="card-experience"
                    aria-expanded={experienceHistoryOpen}
                    onClick={() => setExperienceHistoryOpen((prev) => !prev)}
                  >
                    <div className="mb-3 flex items-center justify-between text-[14px] tracking-[-0.5px]">
                      <div className="inline-flex items-center gap-2 text-[#9ca3af]">
                        <span>{levelTitle}</span>
                        <ArrowRight size={11} className="text-[#9ca3af]" />
                        <span className="text-white">Level {Math.min(MAX_LEVEL, levelProgress.level + 1)}</span>
                      </div>
                      <div className="text-[#66ff4c]">
                        {levelProgress.totalXP.toLocaleString()} / {(levelProgress.level * XP_PER_LEVEL + XP_PER_LEVEL).toLocaleString()} XP
                      </div>
                    </div>
                    <div className="h-3 overflow-hidden rounded-full bg-[#1d1e29]">
                      <div
                        className="h-full rounded-full bg-gradient-to-r from-[#4ed938] to-[#10b981] shadow-[0_0_10px_rgba(102,255,76,0.2)]"
                        style={{ width: `${levelProgress.progressPercent}%` }}
                        data-testid="progress-experience"
                      />
                    </div>
                    <div className={`mt-2 ${PROFILE_MUTED_TEXT_CLASS}`}>
                      {levelProgress.isMaxLevel ? t("profile.maxLevel") : t("profile.nextLevelXP").replace("{xp}", levelProgress.toNextLevelXP.toLocaleString())}
                    </div>
                  </button>

                  {experienceHistoryOpen && (
                    <div
                      className={`absolute left-0 top-[calc(100%+12px)] z-30 w-full p-3 ${PROFILE_PANEL_CLASS}`}
                      data-testid="panel-experience-history"
                    >
                      <div className="flex flex-wrap items-start justify-between gap-2 border-b border-[#2a2c3c] pb-2">
                        <div>
                          <div className="text-sm font-semibold text-white">{t("profile.experienceHistoryTitle")}</div>
                          <div className={PROFILE_MUTED_TEXT_CLASS}>{t("profile.experienceHistoryDescription")}</div>
                        </div>
                        <Badge variant="outline" className={PROFILE_OUTLINE_BUTTON_CLASS}>
                          {t("profile.totalXP")}: {levelProgress.totalXP.toLocaleString()}
                        </Badge>
                      </div>
                      <div className="mt-3 max-h-[280px] overflow-auto pr-1" onScroll={handleExperienceHistoryScroll}>
                        {experienceHistoryLoading ? (
                          <div className="space-y-2 py-2">
                            {[1, 2, 3].map((i) => (
                              <Skeleton key={i} className="h-12 w-full rounded-lg" />
                            ))}
                          </div>
                        ) : experienceHistory.length === 0 ? (
                          <p className={`py-6 text-center text-sm ${PROFILE_MUTED_TEXT_CLASS}`}>{t("profile.experienceHistoryEmpty")}</p>
                        ) : (
                          <div className="space-y-2">
                            {experienceHistory.map((event: any) => (
                              <div key={event.id} className={`${PROFILE_SUBPANEL_CLASS} p-3`}>
                                <div className="flex items-start justify-between gap-3">
                                  <div className="min-w-0">
                                    <div className="text-sm font-medium text-white">{event.description || t("profile.experienceEventFallback")}</div>
                                    <div className={`mt-1 text-[11px] ${PROFILE_MUTED_TEXT_CLASS}`}>
                                      {event.createdAt ? new Date(event.createdAt).toLocaleString() : "—"}
                                      <span className="ml-2 font-mono">{event.eventType}</span>
                                    </div>
                                  </div>
                                  <Badge className="whitespace-nowrap border border-[#3f6b31] bg-[rgba(78,217,56,0.16)] text-[#66ff4c]">
                                    +{Number(event.points || 0).toLocaleString()} XP
                                  </Badge>
                                </div>
                              </div>
                            ))}
                            {experienceHistoryLoadingMore && (
                              <div className={`py-2 text-center text-xs ${PROFILE_MUTED_TEXT_CLASS}`}>Loading more...</div>
                            )}
                          </div>
                        )}
                      </div>
                    </div>
                  )}
                </div>
              </div>
            </div>
          </div>

          <div className="px-8 pb-8">
            <div className="grid gap-6 lg:grid-cols-[2fr_1fr] xl:grid-cols-[738px_357px]">
              <div className="space-y-6">
                <Card className={`${PROFILE_PANEL_CLASS} px-6 py-6`}>
                  <div className="mb-5 flex items-center gap-2 text-[18px] text-white">
                    <Activity size={18} className="text-[#66ff4c]" />
                    <span>Performance Overview</span>
                  </div>
                  <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
                    <div className={`${PROFILE_SUBPANEL_CLASS} h-[114px] p-4`} data-testid="metric-closed-cases">
                      <div className="inline-flex items-center gap-1 text-[12px] text-[#6b7280]">
                        <Trophy size={13} className="text-[#eb5f65]" />
                        Cases Closed
                      </div>
                      <p className="mt-2 text-[24px] leading-[32px] text-white">{Number(casePerformance?.closedCasesTotal ?? 0).toLocaleString()}</p>
                      <div className="mt-1">
                        <TrendPill trend={closedCaseTrend} />
                      </div>
                    </div>
                    <div className={`${PROFILE_SUBPANEL_CLASS} h-[114px] p-4`} data-testid="metric-avg-investigation">
                      <div className="inline-flex items-center gap-1 text-[12px] text-[#6b7280]">
                        <Clock3 size={13} className="text-[#3b82f6]" />
                        Avg MTTR
                      </div>
                      <p className="mt-2 text-[24px] leading-[32px] text-white">{formatAvgResponseMinutes(casePerformance?.avgInvestigationMinutes)}</p>
                      <div className="mt-1">
                        <TrendPill trend={avgInvestigationTrendValue} />
                      </div>
                    </div>
                    <div className={`${PROFILE_SUBPANEL_CLASS} h-[114px] p-4`}>
                      <div className="inline-flex items-center gap-1 text-[12px] text-[#6b7280]">
                        <Shield size={13} className="text-[#ffc700]" />
                        Accuracy
                      </div>
                      <p className="mt-2 text-[24px] leading-[32px] text-white">{accuracyScore}</p>
                      <p className="mt-1 text-[12px] text-[#66ff4c]">+2.1%</p>
                    </div>
                  </div>
                </Card>

                <Card className={`${PROFILE_PANEL_CLASS} px-6 py-6`} data-testid="card-achievements">
                  <div className="mb-5 flex items-center justify-between">
                    <div className="flex items-center gap-2 text-[18px] text-white">
                      <Award size={18} className="text-[#ffc700]" />
                      <span>{t("profile.achievements")}</span>
                    </div>
                    <p className="text-[14px] text-[#6b7280]">{unlockedCount} / {totalAchievements} unlocked</p>
                  </div>

                  {achievementCards.length === 0 ? (
                    <div className="flex min-h-[120px] items-center justify-center">
                      <p className={`text-center text-sm ${PROFILE_MUTED_TEXT_CLASS}`}>No achievements earned yet.</p>
                    </div>
                  ) : (
                    <div className="overflow-x-auto md:overflow-visible">
                      <div className="flex min-w-[676px] justify-between gap-3 pb-1 pt-1 md:min-w-0">
                        {achievementCards.map((ua: any) => {
                          const ach = ua.achievement;
                          if (!ach) return null;
                          const iconValue = String(ach.icon || "🏆");
                          const iconPreview = isAchievementImageIcon(iconValue) ? iconValue : String(ach.icon_url || ach.iconUrl || "");
                          return (
                            <div key={ua.id} className={`relative h-[184px] w-[160px] shrink-0 overflow-hidden rounded-lg p-4 ${getRarityStyles(ach.rarity)}`} data-testid={`card-achievement-${ua.id}`}>
                              <div className="absolute -left-[160px] -top-[148px] h-[475px] w-[475px] bg-[linear-gradient(135deg,rgba(0,0,0,0),rgba(255,255,255,0.05)_35%,rgba(0,0,0,0)_70%)]" />
                              <div className="relative flex h-full flex-col items-center text-center">
                                <div className={`flex h-16 w-16 items-center justify-center rounded-full ${getAchievementIconShellClass(ach.rarity)}`}>
                                  {isAchievementImageIcon(iconPreview) ? (
                                    <img src={iconPreview} alt={ach.name || "Achievement icon"} className="h-8 w-8 object-contain" />
                                  ) : (
                                    <span className="text-2xl">{iconValue}</span>
                                  )}
                                </div>
                                <p className="mt-3 text-[14px] leading-[20px] text-white">{ach.name}</p>
                                <p className="mt-1 text-[12px] leading-[16px] text-[#9ca3af]">{ach.description}</p>
                                <p className={`mt-auto text-[10px] leading-[20px] ${getRarityTextClass(ach.rarity)}`}>{ach.rarity}</p>
                              </div>
                            </div>
                          );
                        })}
                      </div>
                    </div>
                  )}

                  {achievementCards.length > 0 ? (
                    <div className="mt-4 flex flex-wrap items-center justify-between gap-3 border-t border-[#2a2c3c] pt-3">
                      <p className={`text-xs ${PROFILE_MUTED_TEXT_CLASS}`}>
                        Sorted by rarity. {hasAchievementPages ? "Use arrows to browse achievement pages." : ""}
                      </p>
                      {hasAchievementPages ? (
                        <div className="flex items-center gap-2">
                          <Button
                            variant="outline"
                            size="icon"
                            className={`h-8 w-8 ${PROFILE_OUTLINE_BUTTON_CLASS}`}
                            onClick={() => setAchievementsPage((current) => Math.max(1, current - 1))}
                            disabled={achievementsPage <= 1}
                            data-testid="button-achievements-prev-page"
                          >
                            <ChevronLeft size={14} />
                          </Button>
                          <span className="min-w-[72px] text-center text-xs text-[#9ca3af]">
                            {achievementsPage} / {totalAchievementsPages}
                          </span>
                          <Button
                            variant="outline"
                            size="icon"
                            className={`h-8 w-8 ${PROFILE_OUTLINE_BUTTON_CLASS}`}
                            onClick={() => setAchievementsPage((current) => Math.min(totalAchievementsPages, current + 1))}
                            disabled={achievementsPage >= totalAchievementsPages}
                            data-testid="button-achievements-next-page"
                          >
                            <ChevronRight size={14} />
                          </Button>
                        </div>
                      ) : null}
                    </div>
                  ) : null}
                </Card>

                <Card className={`${PROFILE_PANEL_CLASS} flex h-[592px] flex-col px-6 py-6`}>
                  <div className="mb-5 flex items-center gap-2 text-[18px] text-white">
                    <Activity size={18} className="text-[#66ff4c]" />
                    <span>Recent Activity</span>
                  </div>
                  <div className="flex-1 space-y-3 overflow-y-auto pr-1">
                    {experienceHistoryLoading && recentActivity.length === 0 ? (
                      <div className="space-y-3">
                        {Array.from({ length: 4 }).map((_, index) => (
                          <Skeleton key={`recent-activity-skeleton-${index}`} className="h-[98px] w-full rounded-lg" />
                        ))}
                      </div>
                    ) : recentActivity.length === 0 ? (
                      <p className={`py-8 text-center text-sm ${PROFILE_MUTED_TEXT_CLASS}`}>No recent events.</p>
                    ) : (
                      recentActivity.map((event: any, index: number) => {
                        const tone = getActivityTone(index);
                        const ActivityIcon = tone.icon;
                        return (
                          <div key={event.id} className={`${PROFILE_SUBPANEL_CLASS} flex h-[98px] items-start gap-4 px-4 py-4`}>
                            <span className="inline-flex h-10 w-10 items-center justify-center rounded-full" style={{ background: tone.bg, color: tone.text }}>
                              <ActivityIcon size={14} />
                            </span>
                            <div className="min-w-0 flex-1">
                              <p className="text-[14px] leading-[20px] text-white">{event.description || t("profile.experienceEventFallback")}</p>
                              <p className="mt-1 text-[12px] leading-[16px] text-[#9ca3af]">{event.eventType || "Incident activity update"}</p>
                              <div className="mt-1 flex items-center gap-3 text-[12px] leading-[16px]">
                                <span className="text-[#66ff4c]">+{Number(event.points || 0).toLocaleString()} XP</span>
                                <span>{event.createdAt ? new Date(event.createdAt).toLocaleString() : "—"}</span>
                              </div>
                            </div>
                          </div>
                        );
                      })
                    )}
                  </div>
                  <div className="mt-4 border-t border-[#2a2c3c] pt-3 text-right text-xs text-[#6b7280]">
                    Showing latest {RECENT_ACTIVITY_LIMIT} events
                  </div>
                </Card>
              </div>

              <div className="space-y-6">
                <Card className={`${PROFILE_PANEL_CLASS} min-h-[528px] px-6 py-7`} data-testid="card-account-details">
                  <div className="mb-6 flex items-center justify-between gap-3">
                    <div className="flex items-center gap-2 text-[18px] text-white">
                      <ShieldCheck size={18} className="text-[#66ff4c]" />
                      <span>Profile Information</span>
                    </div>
                    {editable ? (
                      isProfileInfoEditing ? (
                        <div className="flex items-center gap-2">
                          <Button
                            variant="outline"
                            className={`h-9 px-3 ${PROFILE_OUTLINE_BUTTON_CLASS}`}
                            onClick={cancelProfileInfoEdit}
                            disabled={updateUser.isPending || saveUserProfileBio.isPending}
                            data-testid="button-profile-info-cancel"
                          >
                            <X size={14} className="mr-1" />
                            Cancel
                          </Button>
                          <Button
                            variant="outline"
                            className={`h-9 px-3 ${PROFILE_OUTLINE_BUTTON_CLASS}`}
                            onClick={saveProfileInfo}
                            disabled={updateUser.isPending || saveUserProfileBio.isPending}
                            data-testid="button-profile-info-save"
                          >
                            <Save size={14} className="mr-1" />
                            Save
                          </Button>
                        </div>
                      ) : (
                        <Button
                          variant="outline"
                          className={`h-9 px-3 ${PROFILE_OUTLINE_BUTTON_CLASS}`}
                          onClick={startProfileInfoEdit}
                          data-testid="button-profile-info-edit"
                        >
                          <Edit2 size={14} className="mr-1" />
                          Edit
                        </Button>
                      )
                    ) : null}
                  </div>
                  <div className="space-y-5">
                    <div className="space-y-2">
                      <p className="text-[12px] leading-[15px] text-[#6b7280]">Email</p>
                      {isProfileInfoEditing ? (
                        <Input
                          className={`${PROFILE_INPUT_CLASS} h-[42px] px-4`}
                          value={profileFieldDrafts.email}
                          onChange={(event) =>
                            setProfileFieldDrafts((prev) => ({ ...prev, email: event.target.value }))
                          }
                          data-testid="input-profile-info-email"
                        />
                      ) : (
                        <div className="flex min-h-[44px] items-center rounded-lg border border-[#2a2c3c] bg-[#0b0c10] px-4 py-2">
                          <p className="text-[14px] leading-[20px] text-white">{currentUser.email || "—"}</p>
                        </div>
                      )}
                    </div>

                    <div className="space-y-2">
                      <p className="text-[12px] leading-[15px] text-[#6b7280]">Department</p>
                      {isProfileInfoEditing ? (
                        <Input
                          className={`${PROFILE_INPUT_CLASS} h-[42px] px-4`}
                          value={profileFieldDrafts.team}
                          onChange={(event) =>
                            setProfileFieldDrafts((prev) => ({ ...prev, team: event.target.value }))
                          }
                          data-testid="input-profile-info-team"
                        />
                      ) : (
                        <div className="flex min-h-[44px] items-center rounded-lg border border-[#2a2c3c] bg-[#0b0c10] px-4 py-2">
                          <p className="text-[14px] leading-[20px] text-white">{profileDepartment}</p>
                        </div>
                      )}
                    </div>

                    <div className="space-y-2">
                      <p className="text-[12px] leading-[15px] text-[#6b7280]">Location</p>
                      {isProfileInfoEditing ? (
                        <Input
                          className={`${PROFILE_INPUT_CLASS} h-[42px] px-4`}
                          value={profileFieldDrafts.personalLink}
                          onChange={(event) =>
                            setProfileFieldDrafts((prev) => ({ ...prev, personalLink: event.target.value }))
                          }
                          placeholder={t("profile.placeholder.url")}
                          data-testid="input-profile-info-personal-link"
                        />
                      ) : (
                        <div className="flex min-h-[44px] items-center rounded-lg border border-[#2a2c3c] bg-[#0b0c10] px-4 py-2">
                          <p className="text-[14px] leading-[20px] text-white">{profileLocation}</p>
                        </div>
                      )}
                    </div>

                    <div className="space-y-2">
                      <p className="text-[12px] leading-[15px] text-[#6b7280]">Bio</p>
                      {isProfileInfoEditing ? (
                        <textarea
                          className="min-h-[106px] w-full resize-y rounded-lg border border-[#2a2c3c] bg-[#0b0c10] px-4 py-3 text-[14px] leading-[22px] text-white outline-none transition focus:border-[#3b4a79]"
                          value={profileFieldDrafts.bio}
                          onChange={(event) =>
                            setProfileFieldDrafts((prev) => ({ ...prev, bio: event.target.value.slice(0, 1200) }))
                          }
                          data-testid="input-profile-info-bio"
                        />
                      ) : (
                        <div className="min-h-[98px] rounded-lg border border-[#2a2c3c] bg-[#0b0c10] px-4 py-3">
                          <p className="whitespace-pre-line text-[14px] leading-[22px] text-white">{profileBio}</p>
                        </div>
                      )}
                    </div>

                    <div className="space-y-2">
                      <p className="text-[12px] leading-[15px] text-[#6b7280]">Joined</p>
                      <p className="text-[16px] leading-[24px] text-white">{joinedDate}</p>
                    </div>
                  </div>
                </Card>

                <Card className={`${PROFILE_PANEL_CLASS} h-[306px] px-6 py-6`}>
                  <div className="mb-4 flex items-center gap-2 text-[18px] text-white">
                    <Sparkles size={18} className="text-[#66ff4c]" />
                    <span>Quick Stats</span>
                  </div>
                  <div className="space-y-3">
                    {quickStats.map((item) => (
                      <div key={item.label} className={`${PROFILE_SUBPANEL_CLASS} flex h-[44px] items-center justify-between px-3`}>
                        <span className="text-[14px] leading-[20px] text-[#9ca3af]">{item.label}</span>
                        <span className={`text-[14px] leading-[20px] ${getQuickStatsValueClass(item.label)}`}>{item.value}</span>
                      </div>
                    ))}
                  </div>
                </Card>

                <Card className={`${PROFILE_PANEL_CLASS} h-[262px] px-6 py-6`}>
                  <div className="mb-4 flex items-center justify-between gap-3">
                    <div className="flex items-center gap-2 text-[18px] text-white">
                      <Star size={18} className="text-[#66ff4c]" />
                      <span>Specializations</span>
                    </div>
                    {editable ? (
                      isSpecializationsEditing ? (
                        <div className="flex items-center gap-2">
                          <Button
                            variant="outline"
                            size="sm"
                            className={PROFILE_OUTLINE_BUTTON_CLASS}
                            onClick={cancelSpecializationsEdit}
                            disabled={saveSpecializationBadges.isPending}
                            data-testid="button-specializations-cancel"
                          >
                            Cancel
                          </Button>
                          <Button
                            variant="outline"
                            size="sm"
                            className={PROFILE_OUTLINE_BUTTON_CLASS}
                            onClick={saveSpecializations}
                            disabled={saveSpecializationBadges.isPending}
                            data-testid="button-specializations-save"
                          >
                            Save
                          </Button>
                        </div>
                      ) : (
                        <Button
                          variant="outline"
                          size="sm"
                          className={PROFILE_OUTLINE_BUTTON_CLASS}
                          onClick={startSpecializationsEdit}
                          data-testid="button-specializations-edit"
                        >
                          <Edit2 size={14} className="mr-1" />
                          Edit
                        </Button>
                      )
                    ) : null}
                  </div>
                  <div className="flex h-[176px] flex-col">
                    {isSpecializationsEditing ? (
                      <>
                        <div className="mb-2 flex items-center gap-2">
                          <Input
                            className={PROFILE_INPUT_CLASS}
                            placeholder="Add specialization..."
                            value={specializationInput}
                            onChange={(event) => setSpecializationInput(event.target.value)}
                            onKeyDown={(event) => {
                              if (event.key === "Enter") {
                                event.preventDefault();
                                handleAddSpecializationBadge();
                              }
                            }}
                            data-testid="input-specialization-badge"
                          />
                          <Button
                            variant="outline"
                            size="sm"
                            className={PROFILE_OUTLINE_BUTTON_CLASS}
                            onClick={handleAddSpecializationBadge}
                            disabled={!String(specializationInput).trim()}
                            data-testid="button-specialization-add"
                          >
                            Add
                          </Button>
                        </div>
                        <div className="space-y-2 overflow-y-auto pr-1">
                          {specializations.length === 0 ? (
                            <p className={`pt-6 text-center text-sm ${PROFILE_MUTED_TEXT_CLASS}`}>
                              No specializations yet.
                            </p>
                          ) : (
                            specializations.map((item) => (
                              <div key={item} className={`${PROFILE_SUBPANEL_CLASS} flex h-[36px] items-center justify-between gap-2 px-3 text-[14px] text-white`}>
                                <div className="flex min-w-0 items-center gap-2">
                                  <CheckCircle2 size={14} className="text-[#66ff4c]" />
                                  <span className="truncate leading-[20px]">{item}</span>
                                </div>
                                <button
                                  type="button"
                                  className="inline-flex h-6 w-6 items-center justify-center rounded-md text-[#9ca3af] transition hover:bg-[#1d1e29] hover:text-white"
                                  onClick={() => handleRemoveSpecializationBadge(item)}
                                  aria-label={`Remove ${item}`}
                                >
                                  <X size={13} />
                                </button>
                              </div>
                            ))
                          )}
                        </div>
                      </>
                    ) : specializationBadgesLoading ? (
                      <div className="space-y-2">
                        {Array.from({ length: 4 }).map((_, index) => (
                          <Skeleton key={`specialization-skeleton-${index}`} className="h-[36px] w-full rounded-lg" />
                        ))}
                      </div>
                    ) : specializations.length === 0 ? (
                      <p className={`pt-6 text-center text-sm ${PROFILE_MUTED_TEXT_CLASS}`}>
                        No specializations set yet.
                      </p>
                    ) : (
                      <div className="space-y-2 overflow-y-auto pr-1">
                        {specializations.map((item) => (
                          <div key={item} className={`${PROFILE_SUBPANEL_CLASS} flex h-[36px] items-center gap-2 px-3 text-[14px] text-white`}>
                            <CheckCircle2 size={14} className="text-[#66ff4c]" />
                            <span className="truncate leading-[20px]">{item}</span>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                </Card>
              </div>
            </div>
          </div>
        </div>

      </div>
    </AppLayout>
  );
}
