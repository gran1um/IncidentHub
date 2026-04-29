import { useState, useEffect, useMemo, useRef, useLayoutEffect } from "react";
import { Link, useLocation } from "wouter";
import {
  Gauge,
  Bell,
  Bot,
  BriefcaseBusiness,
  Layers,
  ShieldCheck,
  Settings,
  Search,
  Plus,
  ChevronDown,
  Building2,
  Sparkles,
  WandSparkles,
  HelpCircle,
  ShieldAlert,
  Activity,
  ChevronLeft,
  ChevronRight,
  MessageCircle,
  Menu,
  CheckCircle2,
  X,
  Paperclip,
  Mic,
  Globe,
  Moon,
  Sun,
  Workflow,
  Send,
  Plug,
  Square,
  Trash2,
} from "lucide-react";
import { useI18n, useT } from "@/lib/i18n";
import { useTheme } from "@/lib/theme";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { Popover, PopoverAnchor, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Dialog, DialogContent, DialogTitle, DialogDescription, DialogFooter } from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import { toast } from "sonner";
import { useAppState, useAlertsTotal, useCasesTotal, useUser, useTenants, useConfig, useHealth, useSearch, useNotifications, useMarkNotificationRead, useMarkAllNotificationsRead, useDeleteNotification, useClearNotifications, useAskAI, useAISessions, useCreateAISession, useAIMessages, useCreateAlert, useCreateCase, useConnectorMethods, useExecuteConnectorHub, useOutboundConnectors, logout } from "@/lib/api";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Skeleton } from "@/components/ui/skeleton";
import { EllipsisText } from "@/components/ui/ellipsis-text";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { useIsMobile } from "@/hooks/use-mobile";
import { formatModuleResponseMs } from "@/lib/module-health";
import { withTenantLocation, withTenantPath } from "@/lib/tenant-url";

function getModuleStatusSeverity(mod: { status: string; responseMs: number }): 0 | 1 | 2 {
  const status = String(mod?.status || "").toLowerCase();
  const responseMs = Number(mod?.responseMs || 0);
  if (status === "error") return 2;
  if (status === "disabled") return 0;
  if (responseMs >= 1000) return 2;
  if (responseMs >= 100) return 1;
  return 0;
}

function getModuleStatusColor(mod: { status: string; responseMs: number }) {
  const status = String(mod?.status || "").toLowerCase();
  if (status === "disabled") return "bg-slate-400";
  const severity = getModuleStatusSeverity(mod);
  if (severity === 2) return "bg-red-500";
  if (severity === 1) {
    return Number(mod?.responseMs || 0) >= 500 ? "bg-orange-500" : "bg-yellow-500";
  }
  return "bg-green-500";
}

function formatAttachmentSize(size: number): string {
  if (!Number.isFinite(size) || size <= 0) {
    return "0 B";
  }
  if (size < 1024) {
    return `${size} B`;
  }
  const kb = size / 1024;
  if (kb < 1024) {
    return `${kb.toFixed(kb >= 100 ? 0 : 1)} KB`;
  }
  const mb = kb / 1024;
  return `${mb.toFixed(mb >= 100 ? 0 : 1)} MB`;
}

function composeAIQuestionWithAttachments(question: string, attachments: File[], language: string): string {
  const trimmedQuestion = question.trim();
  if (attachments.length === 0) {
    return trimmedQuestion;
  }
  const title = language === "ru" ? "Вложения:" : "Attachments:";
  const lines = attachments.map((file) => `- ${file.name} (${formatAttachmentSize(file.size)})`);
  return `${trimmedQuestion}\n\n${title}\n${lines.join("\n")}`;
}

type NavSection = "operations" | "intelligence" | "system";

type NavItem = {
  label: string;
  icon: any;
  href: string;
  count?: number;
  countLoading?: boolean;
  section: NavSection;
};

type QuickActionKind = "none" | "add_alert" | "add_case" | "run_connector";

export function AppLayout({ children, backHref }: { children: React.ReactNode; backHref?: string }) {
  const [location, setLocation] = useLocation();
  const isMobile = useIsMobile();
  const [aiOpen, setAiOpen] = useState(false);
  const [collapsed, setCollapsed] = useState(() => {
    try { return localStorage.getItem("sidebar-collapsed") === "true"; } catch { return false; }
  });
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [statusPopoverOpen, setStatusPopoverOpen] = useState(false);
  const [aiHidden, setAiHidden] = useState(() => {
    try { return localStorage.getItem("ai-hidden") === "true"; } catch { return false; }
  });
  const [aiHasUnread, setAiHasUnread] = useState(false);
  const [bootstrapAISession, setBootstrapAISession] = useState<any | null>(null);
  const [aiBootstrapAttempted, setAiBootstrapAttempted] = useState(false);
  const [aiQuestion, setAiQuestion] = useState("");
  const [aiAttachments, setAIAttachments] = useState<File[]>([]);
  const [aiVoiceSupported, setAIVoiceSupported] = useState(false);
  const [aiVoiceListening, setAIVoiceListening] = useState(false);
  const [optimisticAIUserMessage, setOptimisticAIUserMessage] = useState<any | null>(null);
  const [aiThinking, setAIThinking] = useState(false);
  const [quickActionKind, setQuickActionKind] = useState<QuickActionKind>("none");
  const [quickAlertTitle, setQuickAlertTitle] = useState("");
  const [quickAlertDescription, setQuickAlertDescription] = useState("");
  const [quickAlertSeverity, setQuickAlertSeverity] = useState("High");
  const [quickCaseTitle, setQuickCaseTitle] = useState("");
  const [quickCaseDescription, setQuickCaseDescription] = useState("");
  const [quickCaseSeverity, setQuickCaseSeverity] = useState("High");
  const [quickConnectorId, setQuickConnectorId] = useState("");
  const [quickConnectorMethodId, setQuickConnectorMethodId] = useState("");
  const [quickConnectorMessage, setQuickConnectorMessage] = useState("");
  const [quickConnectorMetadata, setQuickConnectorMetadata] = useState("{}");
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const debouncedSearchQuery = useDebouncedValue(searchQuery, 300);
  const searchInputRef = useRef<HTMLInputElement>(null);
  const aiComposerRef = useRef<HTMLTextAreaElement | null>(null);
  const aiAttachmentInputRef = useRef<HTMLInputElement | null>(null);
  const aiMessagesContainerRef = useRef<HTMLDivElement | null>(null);
  const aiAbortControllerRef = useRef<AbortController | null>(null);
  const aiCancelRequestedRef = useRef(false);
  const aiVoiceRecognitionRef = useRef<any>(null);
  const aiVoiceBaseTextRef = useRef("");

  const t = useT();
  const { language, setLanguage } = useI18n();
  const { theme, toggleTheme } = useTheme();

  const { currentTenantId, currentTenantSlug, currentUserId, setCurrentTenant, session } = useAppState();
  const { data: alertsTotal = 0, isLoading: alertsTotalLoading } = useAlertsTotal(currentTenantId);
  const { data: casesTotal = 0, isLoading: casesTotalLoading } = useCasesTotal(currentTenantId);
  const { data: currentUser } = useUser(currentUserId);
  const { data: tenants = [] } = useTenants();
  const { data: config } = useConfig();
  const { data: health } = useHealth(15000);
  const { data: searchResults, isLoading: searchLoading } = useSearch(currentTenantId, debouncedSearchQuery);
  const normalizedSearchQuery = searchQuery.trim();
  const normalizedDebouncedSearchQuery = debouncedSearchQuery.trim();
  const isSearchQueryReady = normalizedSearchQuery.length >= 2;
  const isDebouncedSearchReady = normalizedDebouncedSearchQuery.length >= 2;
  const isSearchWaitingForDebounce = isSearchQueryReady && normalizedSearchQuery !== normalizedDebouncedSearchQuery;
  const showSearchLoadingState = isSearchWaitingForDebounce || searchLoading;
  const hasSearchResults = Boolean(
    searchResults?.alerts?.length ||
    searchResults?.cases?.length ||
    searchResults?.threads?.length,
  );
  const shouldShowSearchPopover = searchOpen && isSearchQueryReady;
  const askAI = useAskAI();
  const createAlert = useCreateAlert();
  const createCase = useCreateCase();
  const executeConnectorHub = useExecuteConnectorHub();
  const { data: outboundConnectors = [] } = useOutboundConnectors(currentTenantId);
  const { data: quickConnectorMethods = [] } = useConnectorMethods(quickConnectorId);
  const createAISession = useCreateAISession();
  const { data: aiSessions = [], refetch: refetchAISessions } = useAISessions(currentTenantId, 60);
  const activeAISession = useMemo(() => {
    const source = [...aiSessions];
    source.sort((left: any, right: any) => {
      const leftOrder = Number(left?.sortOrder ?? left?.sort_order ?? 0);
      const rightOrder = Number(right?.sortOrder ?? right?.sort_order ?? 0);
      if (leftOrder !== rightOrder) {
        return rightOrder - leftOrder;
      }
      const leftUpdated = String(left?.lastMessageAt || left?.updatedAt || "");
      const rightUpdated = String(right?.lastMessageAt || right?.updatedAt || "");
      return rightUpdated.localeCompare(leftUpdated);
    });
    return source[0] || bootstrapAISession || null;
  }, [aiSessions, bootstrapAISession]);
  const { data: aiMessages = [], refetch: refetchAIMessages } = useAIMessages(activeAISession?.id, 120, currentTenantId);
  const aiBusy = askAI.isPending || aiThinking;
  const aiAttachmentLimit = 5;
  const displayedAIMessages = useMemo(() => {
    const messages = [...aiMessages];
    if (optimisticAIUserMessage && optimisticAIUserMessage.sessionId === activeAISession?.id) {
      messages.push(optimisticAIUserMessage);
    }
    if (aiThinking && activeAISession?.id) {
      messages.push({
        id: `thinking-${activeAISession.id}`,
        role: "assistant",
        content: "",
        isThinking: true,
      });
    }
    return messages;
  }, [aiMessages, optimisticAIUserMessage, aiThinking, activeAISession?.id]);

  const { data: notifications = [] } = useNotifications(currentUserId, currentTenantId);
  const markRead = useMarkNotificationRead();
  const markAllRead = useMarkAllNotificationsRead();
  const deleteNotification = useDeleteNotification();
  const clearNotifications = useClearNotifications();
  const unreadCount = notifications.filter((n: any) => !n.read).length;

  const tenantLabels: Record<string, { slug: string; name: string }> = {};
  tenants.forEach((tenant: any) => {
    tenantLabels[tenant.id] = {
      slug: tenant.slug || tenant.id,
      name: tenant.name || "",
    };
  });
  const knownTenantSlugs = useMemo(
    () =>
      (tenants || [])
        .map((tenant: any) => String(tenant?.slug || "").trim().toLowerCase())
        .filter(Boolean),
    [tenants],
  );
  const currentTenantMembership = session?.memberships?.find((m) => m.tenant_id === currentTenantId && m.is_active);
  const currentUserName = useMemo(() => {
    const fromUser = String(currentUser?.name || "").trim();
    if (fromUser) {
      return fromUser;
    }
    const fromIdentity = String(session?.identity?.username || "").trim();
    if (fromIdentity) {
      return fromIdentity;
    }
    return "User";
  }, [currentUser?.name, session?.identity?.username]);
  const currentUserAvatar = String(currentUser?.avatar || "").trim();
  const currentUserInitials = useMemo(() => {
    const parts = currentUserName
      .split(/\s+/)
      .map((part) => part.trim())
      .filter(Boolean);
    if (parts.length === 0) {
      return "U";
    }
    return parts.slice(0, 2).map((part) => part[0]).join("").toUpperCase();
  }, [currentUserName]);
  const isAdminUser = Boolean(currentUser?.isAdmin || session?.identity?.is_platform_admin || currentTenantMembership?.role === "tenant_admin");
  const isPlatformAdmin = Boolean(session?.identity?.is_platform_admin);
  const canUseWorkflowStudio = Boolean(
    isPlatformAdmin || currentTenantMembership?.role === "tenant_admin" || currentTenantMembership?.role === "analyst",
  );
  const currentTenantLabel = tenantLabels[currentTenantId]?.slug || t("tenant.select");
  const enabledOutboundConnectors = useMemo(
    () => outboundConnectors.filter((item: any) => item?.enabled !== false),
    [outboundConnectors],
  );
  const enabledQuickConnectorMethods = useMemo(
    () => quickConnectorMethods.filter((item: any) => item?.enabled !== false),
    [quickConnectorMethods],
  );
  const tenantRootPath = withTenantPath(currentTenantSlug, "/dashboard");
  const handleTenantSwitch = (nextTenantID: string) => {
    const nextID = String(nextTenantID || "").trim();
    if (!nextID) {
      return;
    }
    const nextSlug = String(tenantLabels[nextID]?.slug || "").trim().toLowerCase();
    setCurrentTenant(nextID);
    if (!nextSlug) {
      return;
    }
    setLocation(withTenantLocation(nextSlug, location, knownTenantSlugs), { replace: true });
  };

  const canAccessInvestigations = canUseWorkflowStudio;
  const navItems: NavItem[] = [
    { label: t("nav.dashboard"), icon: Gauge, href: withTenantPath(currentTenantSlug, "/dashboard"), section: "operations" },
    ...(canAccessInvestigations ? [{ label: t("nav.activity"), icon: Activity, href: withTenantPath(currentTenantSlug, "/activity"), section: "operations" as const }] : []),
    ...(canAccessInvestigations ? [{ label: t("nav.alerts"), icon: Bell, href: withTenantPath(currentTenantSlug, "/alerts"), count: alertsTotal, countLoading: alertsTotalLoading, section: "operations" as const }] : []),
    ...(canAccessInvestigations ? [{ label: t("nav.cases"), icon: BriefcaseBusiness, href: withTenantPath(currentTenantSlug, "/cases"), count: casesTotal, countLoading: casesTotalLoading, section: "operations" as const }] : []),
    ...(isAdminUser ? [{ label: t("nav.aiAgents"), icon: Bot, href: withTenantPath(currentTenantSlug, "/ai-agents"), section: "intelligence" as const }] : []),
    ...(canAccessInvestigations ? [{ label: t("nav.forum"), icon: MessageCircle, href: withTenantPath(currentTenantSlug, "/forum"), section: "intelligence" as const }] : []),
    ...(canAccessInvestigations ? [{ label: t("nav.templates"), icon: Layers, href: withTenantPath(currentTenantSlug, "/templates"), section: "system" as const }] : []),
    ...(canUseWorkflowStudio ? [{ label: t("nav.workflowStudio"), icon: Workflow, href: withTenantPath(currentTenantSlug, "/workflow-studio"), section: "system" as const }] : []),
    ...(isAdminUser ? [
      { label: t("nav.administration"), icon: Settings, href: withTenantPath(currentTenantSlug, "/administration"), section: "system" as const },
      { label: t("nav.outboundConnectors"), icon: Plug, href: withTenantPath(currentTenantSlug, "/connectors"), section: "system" as const },
    ] : []),
  ];
  const navSections: Array<{ id: NavSection; label: string }> = [
    { id: "operations", label: "Operations" },
    { id: "intelligence", label: "Intelligence" },
    { id: "system", label: "System" },
  ];
  const navItemsBySection = useMemo(() => {
    const grouped: Record<NavSection, NavItem[]> = {
      operations: [],
      intelligence: [],
      system: [],
    };
    navItems.forEach((item) => {
      grouped[item.section].push(item);
    });
    return grouped;
  }, [navItems]);
  const sidebarCollapsed = !isMobile && collapsed;
  const mobileSidebarStyle = isMobile
    ? { transform: mobileMenuOpen ? "translateX(0)" : "translateX(-110%)" }
    : undefined;

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        searchInputRef.current?.focus();
      }
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, []);

  useEffect(() => {
    if (searchOpen && searchInputRef.current) {
      setTimeout(() => searchInputRef.current?.focus(), 100);
    }
  }, [searchOpen]);

  useEffect(() => {
    localStorage.setItem("ai-hidden", String(aiHidden));
  }, [aiHidden]);

  useEffect(() => {
    localStorage.setItem("sidebar-collapsed", String(collapsed));
  }, [collapsed]);

  useEffect(() => {
    if (!isMobile) {
      setMobileMenuOpen(false);
    }
  }, [isMobile]);

  useEffect(() => {
    setMobileMenuOpen(false);
  }, [location]);

  useEffect(() => {
    setAiBootstrapAttempted(false);
    setBootstrapAISession(null);
  }, [currentTenantId]);

  useEffect(() => {
    if (!bootstrapAISession?.id) {
      return;
    }
    if (aiSessions.some((session: any) => session.id === bootstrapAISession.id)) {
      setBootstrapAISession(null);
    }
  }, [aiSessions, bootstrapAISession]);

  useEffect(() => {
    if (!aiOpen) {
      setAiBootstrapAttempted(false);
      return;
    }
    if (!currentTenantId || activeAISession || createAISession.isPending || aiBootstrapAttempted) {
      return;
    }
    setAiBootstrapAttempted(true);
    createAISession.mutate(
      { tenantId: currentTenantId, title: t("layout.ai.newChatDefaultTitle") },
      {
        onSuccess: (session: any) => {
          setBootstrapAISession(session);
          refetchAISessions();
        },
        onError: (error: any) => {
          toast.error(error?.message || t("layout.ai.error"));
        },
      },
    );
  }, [activeAISession, aiBootstrapAttempted, aiOpen, createAISession, currentTenantId, refetchAISessions, t]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    const supported = Boolean((window as any).SpeechRecognition || (window as any).webkitSpeechRecognition);
    setAIVoiceSupported(supported);
  }, []);

  useEffect(() => {
    if (typeof document === "undefined") {
      return;
    }
    document.documentElement.classList.add("app-shell-scroll-lock");
    document.body.classList.add("app-shell-scroll-lock");
    return () => {
      document.documentElement.classList.remove("app-shell-scroll-lock");
      document.body.classList.remove("app-shell-scroll-lock");
    };
  }, []);

  useLayoutEffect(() => {
    if (!aiOpen) {
      return;
    }
    const container = aiMessagesContainerRef.current;
    if (!container) {
      return;
    }
    container.scrollTop = container.scrollHeight;
  }, [displayedAIMessages.length, activeAISession?.id, aiOpen]);

  useEffect(() => {
    const textarea = aiComposerRef.current;
    if (!textarea) {
      return;
    }
    textarea.style.height = "auto";
    const nextHeight = Math.min(textarea.scrollHeight, 120);
    textarea.style.height = `${Math.max(nextHeight, 44)}px`;
  }, [aiQuestion, aiOpen]);

  useEffect(() => {
    setOptimisticAIUserMessage(null);
    setAIThinking(false);
    aiCancelRequestedRef.current = false;
    if (aiAbortControllerRef.current) {
      aiAbortControllerRef.current.abort();
      aiAbortControllerRef.current = null;
    }
    if (aiVoiceRecognitionRef.current) {
      try {
        aiVoiceRecognitionRef.current.stop();
      } catch {
        // noop
      }
    }
    setAIVoiceListening(false);
  }, [activeAISession?.id]);

  useEffect(() => {
    if (aiOpen || !aiVoiceListening) {
      return;
    }
    if (aiVoiceRecognitionRef.current) {
      try {
        aiVoiceRecognitionRef.current.stop();
      } catch {
        // noop
      }
    }
    setAIVoiceListening(false);
  }, [aiOpen, aiVoiceListening]);

  useEffect(
    () => () => {
      if (!aiVoiceRecognitionRef.current) {
        return;
      }
      try {
        aiVoiceRecognitionRef.current.stop();
      } catch {
        // noop
      }
    },
    [],
  );

  useEffect(() => {
    if (quickActionKind !== "run_connector") {
      return;
    }
    if (enabledOutboundConnectors.length === 0) {
      setQuickConnectorId("");
      return;
    }
    if (!quickConnectorId || !enabledOutboundConnectors.some((item: any) => item.id === quickConnectorId)) {
      setQuickConnectorId(String(enabledOutboundConnectors[0].id || ""));
    }
  }, [quickActionKind, quickConnectorId, enabledOutboundConnectors]);

  useEffect(() => {
    if (!quickConnectorId) {
      setQuickConnectorMethodId("");
      return;
    }
    if (enabledQuickConnectorMethods.length === 0) {
      setQuickConnectorMethodId("");
      return;
    }
    if (!quickConnectorMethodId || !enabledQuickConnectorMethods.some((item: any) => item.id === quickConnectorMethodId)) {
      setQuickConnectorMethodId(String(enabledQuickConnectorMethods[0].id || ""));
    }
  }, [quickConnectorId, quickConnectorMethodId, enabledQuickConnectorMethods]);

  const resetQuickActionForms = () => {
    setQuickAlertTitle("");
    setQuickAlertDescription("");
    setQuickAlertSeverity("High");
    setQuickCaseTitle("");
    setQuickCaseDescription("");
    setQuickCaseSeverity("High");
    setQuickConnectorId("");
    setQuickConnectorMethodId("");
    setQuickConnectorMessage("");
    setQuickConnectorMetadata("{}");
  };

  const closeQuickActionDialog = () => {
    setQuickActionKind("none");
    resetQuickActionForms();
  };

  const openQuickAction = (kind: QuickActionKind) => {
    if (kind === "run_connector" && enabledOutboundConnectors.length === 0) {
      toast.error("Create a connector first");
      return;
    }
    setQuickActionKind(kind);
  };

  const submitQuickAlert = () => {
    const title = quickAlertTitle.trim();
    if (title.length < 3) {
      toast.error("Alert title is required");
      return;
    }
    createAlert.mutate(
      {
        title,
        description: quickAlertDescription.trim(),
        severity: quickAlertSeverity,
        source: "quick-action",
      },
      {
        onSuccess: (created: any) => {
          toast.success("Alert created");
          closeQuickActionDialog();
          if (created?.id) {
            setLocation(withTenantPath(currentTenantSlug, `/alerts/${created.id}`));
            return;
          }
          setLocation(withTenantPath(currentTenantSlug, "/alerts"));
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to create alert");
        },
      },
    );
  };

  const submitQuickCase = () => {
    const title = quickCaseTitle.trim();
    if (title.length < 3) {
      toast.error(t("layout.quick.error.caseTitleRequired"));
      return;
    }
    createCase.mutate(
      {
        title,
        description: quickCaseDescription.trim(),
        severity: quickCaseSeverity,
        source: "quick-action",
      },
      {
        onSuccess: (created: any) => {
          toast.success("Case created");
          closeQuickActionDialog();
          if (created?.id) {
            setLocation(withTenantPath(currentTenantSlug, `/cases/${created.id}`));
            return;
          }
          setLocation(withTenantPath(currentTenantSlug, "/cases"));
        },
        onError: (error: any) => {
          toast.error(error?.message || t("layout.quick.error.caseCreateFailed"));
        },
      },
    );
  };

  const submitQuickConnector = () => {
    if (!quickConnectorId || !quickConnectorMethodId) {
      toast.error("Choose a connector and method");
      return;
    }
    let metadata: Record<string, any> = {};
    const rawMetadata = quickConnectorMetadata.trim();
    if (rawMetadata) {
      try {
        const parsed = JSON.parse(rawMetadata);
        if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
          toast.error("Connector metadata must be a JSON object");
          return;
        }
        metadata = parsed as Record<string, any>;
      } catch {
        toast.error("Connector metadata must be valid JSON");
        return;
      }
    }
    executeConnectorHub.mutate(
      {
        connector_id: quickConnectorId,
        method_id: quickConnectorMethodId,
        message: quickConnectorMessage.trim(),
        metadata,
        dry_run: false,
      },
      {
        onSuccess: () => {
          toast.success("Connector execution started");
          closeQuickActionDialog();
          setLocation(withTenantPath(currentTenantSlug, "/connectors"));
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to run connector");
        },
      },
    );
  };

  const handleSelectAIAttachments = (event: React.ChangeEvent<HTMLInputElement>) => {
    const selected = Array.from(event.target.files || []);
    event.target.value = "";
    if (selected.length === 0) {
      return;
    }

    let skippedByLimit = 0;
    setAIAttachments((prev) => {
      const next: File[] = [...prev];
      const seen = new Set(prev.map((file) => `${file.name}:${file.size}:${file.lastModified}`));
      for (const file of selected) {
        const key = `${file.name}:${file.size}:${file.lastModified}`;
        if (seen.has(key)) {
          continue;
        }
        if (next.length >= aiAttachmentLimit) {
          skippedByLimit += 1;
          continue;
        }
        seen.add(key);
        next.push(file);
      }
      return next;
    });
    if (skippedByLimit > 0) {
      toast.warning(
        language === "ru"
          ? `Лимит вложений: ${aiAttachmentLimit}`
          : `Attachment limit reached: ${aiAttachmentLimit}`,
      );
    }
  };

  const removeAIAttachment = (index: number) => {
    setAIAttachments((prev) => prev.filter((_, currentIndex) => currentIndex !== index));
  };

  const toggleAIVoiceInput = () => {
    if (!aiVoiceSupported) {
      toast.error(language === "ru" ? "Голосовой ввод не поддерживается в этом браузере" : "Voice input is not supported in this browser");
      return;
    }
    if (aiVoiceListening) {
      if (aiVoiceRecognitionRef.current) {
        try {
          aiVoiceRecognitionRef.current.stop();
        } catch {
          // noop
        }
      }
      setAIVoiceListening(false);
      return;
    }
    if (typeof window === "undefined") {
      return;
    }
    const SpeechRecognitionCtor = (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;
    if (!SpeechRecognitionCtor) {
      toast.error(language === "ru" ? "Голосовой ввод недоступен" : "Voice input is unavailable");
      return;
    }

    const recognition = new SpeechRecognitionCtor();
    recognition.lang = language === "ru" ? "ru-RU" : "en-US";
    recognition.interimResults = true;
    recognition.continuous = false;
    recognition.maxAlternatives = 1;

    aiVoiceBaseTextRef.current = aiQuestion.trim();
    recognition.onresult = (resultEvent: any) => {
      let transcript = "";
      const results = resultEvent?.results || [];
      for (let index = 0; index < results.length; index += 1) {
        const chunk = String(results[index]?.[0]?.transcript || "").trim();
        if (chunk) {
          transcript = `${transcript} ${chunk}`.trim();
        }
      }
      const base = aiVoiceBaseTextRef.current;
      const nextValue = [base, transcript].filter(Boolean).join(" ").trim();
      setAiQuestion(nextValue);
    };
    recognition.onerror = (errorEvent: any) => {
      setAIVoiceListening(false);
      const errorCode = String(errorEvent?.error || "").trim().toLowerCase();
      if (errorCode === "aborted") {
        return;
      }
      if (errorCode === "not-allowed") {
        toast.error(language === "ru" ? "Нет доступа к микрофону" : "Microphone access denied");
        return;
      }
      toast.error(language === "ru" ? "Ошибка голосового ввода" : "Voice input failed");
    };
    recognition.onend = () => {
      setAIVoiceListening(false);
    };

    try {
      aiVoiceRecognitionRef.current = recognition;
      setAIVoiceListening(true);
      recognition.start();
    } catch {
      setAIVoiceListening(false);
      toast.error(language === "ru" ? "Не удалось запустить голосовой ввод" : "Failed to start voice input");
    }
  };

  const sendAIQuestion = (overrideQuestion?: string, forcedSessionID?: string, preparedQuestion = false) => {
    const isFromComposer = typeof overrideQuestion === "undefined";
    const rawQuestion = (overrideQuestion ?? aiQuestion).trim();
    if (rawQuestion.length < 2 || aiBusy) {
      return;
    }
    const payloadQuestion = preparedQuestion
      ? rawQuestion
      : composeAIQuestionWithAttachments(rawQuestion, isFromComposer ? aiAttachments : [], language);
    const sessionID = forcedSessionID || activeAISession?.id;
    if (!sessionID) {
      if (!currentTenantId || createAISession.isPending) {
        return;
      }
      if (isFromComposer) {
        setAiQuestion("");
        setAIAttachments([]);
      }
      createAISession.mutate(
        { tenantId: currentTenantId, title: t("layout.ai.newChatDefaultTitle") },
        {
        onSuccess: (session: any) => {
          setBootstrapAISession(session);
          refetchAISessions();
          sendAIQuestion(payloadQuestion, session?.id, true);
        },
          onError: (error: any) => {
            toast.error(error?.message || t("layout.ai.error"));
          },
        },
      );
      return;
    }

    setOptimisticAIUserMessage({
      id: `optimistic-user-${Date.now()}`,
      sessionId: sessionID,
      role: "user",
      content: payloadQuestion,
      createdAt: new Date().toISOString(),
    });
    setAIThinking(true);
    if (isFromComposer) {
      setAiQuestion("");
      setAIAttachments([]);
    }
    if (aiVoiceListening && aiVoiceRecognitionRef.current) {
      try {
        aiVoiceRecognitionRef.current.stop();
      } catch {
        // noop
      }
      setAIVoiceListening(false);
    }
    const abortController = new AbortController();
    aiAbortControllerRef.current = abortController;
    aiCancelRequestedRef.current = false;
    askAI.mutate(
      { question: payloadQuestion, sessionId: sessionID, language, signal: abortController.signal },
      {
        onSuccess: async () => {
          aiAbortControllerRef.current = null;
          aiCancelRequestedRef.current = false;
          setAIThinking(false);
          await Promise.all([refetchAIMessages(), refetchAISessions()]);
          setOptimisticAIUserMessage(null);
        },
        onError: (err: any) => {
          const message = String(err?.message || "");
          const aborted = aiCancelRequestedRef.current || err?.name === "AbortError" || /abort/i.test(message);
          aiAbortControllerRef.current = null;
          aiCancelRequestedRef.current = false;
          setAIThinking(false);
          if (aborted) {
            toast.success(t("layout.ai.requestCancelled"));
            return;
          }
          toast.error(message || t("layout.ai.error"));
        },
      },
    );
  };

  const stopAIRequest = () => {
    if (!askAI.isPending) {
      return;
    }
    aiCancelRequestedRef.current = true;
    aiAbortControllerRef.current?.abort();
  };

  const aiQuickActions = useMemo(
    () => [
      {
        id: "caseFromAlerts",
        title: t("layout.ai.quick.caseFromAlerts"),
        prompt: t("layout.ai.quick.caseFromAlertsPrompt"),
        icon: BriefcaseBusiness,
        colorClass: "text-[#3B82F6]",
        submit: true,
      },
      {
        id: "runAgent",
        title: t("layout.ai.quick.runAgent"),
        prompt: t("layout.ai.quick.runAgentPrompt"),
        icon: Bot,
        colorClass: "text-[#FFC700]",
        submit: true,
      },
      {
        id: "analyzeCase",
        title: t("layout.ai.quick.analyzeCase"),
        prompt: t("layout.ai.quick.analyzeCasePrompt"),
        icon: Activity,
        colorClass: "text-[#c4b5fd]",
        submit: true,
      },
      {
        id: "enrichCase",
        title: t("layout.ai.quick.enrichCase"),
        prompt: t("layout.ai.quick.enrichCasePrompt"),
        icon: Sparkles,
        colorClass: "text-[#66FF4C]",
        submit: true,
      },
    ],
    [t],
  );

  const formatAIMessageTimestamp = (value: unknown) => {
    const raw = String(value || "").trim();
    if (!raw) {
      return "";
    }
    const parsed = new Date(raw);
    if (Number.isNaN(parsed.getTime())) {
      return "";
    }
    return parsed.toLocaleTimeString(language === "ru" ? "ru-RU" : "en-US", {
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  const modules = health?.modules || {};
  const moduleEntries = Object.entries(modules);
  const overallStatusSeverity = useMemo(() => {
    let severity: 0 | 1 | 2 = 0;
    if (String(health?.status || "ok").toLowerCase() !== "ok") {
      severity = 1;
    }
    Object.values(modules).forEach((module: any) => {
      const moduleSeverity = getModuleStatusSeverity({
        status: String(module?.status || ""),
        responseMs: Number(module?.responseMs || 0),
      });
      if (moduleSeverity > severity) {
        severity = moduleSeverity;
      }
    });
    return severity;
  }, [health?.status, modules]);

  return (
    <div className="relative flex h-[100dvh] overflow-hidden bg-[#0b0c10] text-white" data-testid="app-layout-shell">
      {isMobile && mobileMenuOpen && (
        <button
          type="button"
          className="fixed inset-0 z-30 bg-black/70 backdrop-blur-[1px]"
          onClick={() => setMobileMenuOpen(false)}
          aria-label="Close menu overlay"
        />
      )}
      {/* Sidebar */}
      <aside
        className={`${isMobile
          ? "fixed inset-y-0 left-0 z-40 w-[82vw] max-w-[300px] border-r border-[#1d1e29] bg-[#13141c] transition-transform duration-300 will-change-transform"
          : `${sidebarCollapsed ? "w-20" : "w-64"} border-r border-[#1d1e29] bg-[#13141c]`
          } flex flex-col md:relative ${isMobile ? "overflow-hidden" : "overflow-visible"}`}
        style={mobileSidebarStyle}
      >
        <div className={`flex items-center border-b border-[#1d1e29] px-6 py-4 ${sidebarCollapsed ? "justify-center" : "justify-between"}`}>
          {!sidebarCollapsed && (
            <div onClick={() => setLocation(tenantRootPath)} className="flex cursor-pointer items-center gap-3">
              <svg width="32" height="32" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg" className="w-8 h-8 flex-shrink-0">
                <path d="M16 2L4 8v8c0 7.18 5.12 13.88 12 16 6.88-2.12 12-8.82 12-16V8L16 2z" fill="url(#shield-grad)" stroke="hsl(var(--primary))" strokeWidth="1.5" />
                <path d="M12 16l3 3 5-6" stroke="hsl(var(--primary-foreground))" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" fill="none" />
                <circle cx="16" cy="10" r="2" fill="hsl(var(--primary-foreground))" opacity="0.6" />
                <defs>
                  <linearGradient id="shield-grad" x1="4" y1="2" x2="28" y2="26">
                    <stop stopColor="hsl(45, 93%, 47%)" />
                    <stop offset="1" stopColor="hsl(40, 96%, 40%)" />
                  </linearGradient>
                </defs>
              </svg>
              <span className="text-xl font-semibold tracking-tight">{t("layout.brand")}</span>
            </div>
          )}
          {sidebarCollapsed && (
            <div onClick={() => setLocation(tenantRootPath)} className="cursor-pointer">
              <svg width="32" height="32" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg" className="w-8 h-8 flex-shrink-0">
                <path d="M16 2L4 8v8c0 7.18 5.12 13.88 12 16 6.88-2.12 12-8.82 12-16V8L16 2z" fill="url(#shield-grad)" stroke="hsl(var(--primary))" strokeWidth="1.5" />
                <path d="M12 16l3 3 5-6" stroke="hsl(var(--primary-foreground))" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" fill="none" />
                <circle cx="16" cy="10" r="2" fill="hsl(var(--primary-foreground))" opacity="0.6" />
                <defs>
                  <linearGradient id="shield-grad" x1="4" y1="2" x2="28" y2="26">
                    <stop stopColor="hsl(45, 93%, 47%)" />
                    <stop offset="1" stopColor="hsl(40, 96%, 40%)" />
                  </linearGradient>
                </defs>
              </svg>
            </div>
          )}
        </div>

        {!isMobile && (
          <div className="absolute right-0 top-6 z-30 translate-x-1/2">
            <Button
              variant="outline"
              size="icon"
              className="h-6 w-6 rounded-full border-[#2a2c3c] bg-[#13141c] text-[#9ca3af] shadow-sm hover:bg-[#1d1e29] hover:text-white"
              onClick={() => setCollapsed(!collapsed)}
            >
              {sidebarCollapsed ? <ChevronRight size={12} /> : <ChevronLeft size={12} />}
            </Button>
          </div>
        )}

        <div className="flex min-h-0 flex-1 flex-col">
          <nav className="mt-4 flex-1 space-y-1 overflow-y-auto px-0">
            {navSections.map((section) => {
              const items = navItemsBySection[section.id];
              if (items.length === 0) {
                return null;
              }
              return (
                <div key={section.id} className="space-y-1">
                  {!sidebarCollapsed && (
                    <div className="px-6 pb-1 pt-3 text-[12px] uppercase tracking-[0.1px] text-[#6b7280]">
                      {section.label}
                    </div>
                  )}
                  {items.map((item) => {
                    const isActive = location === item.href || (item.href !== "/" && location.startsWith(item.href));
                    const hasCountBadge = typeof item.count === "number" && item.count > 0;
                    const iconColorClass = isActive ? "text-[#66ff4c]" : "text-[#9ca3af] group-hover:text-white";
                    return (
                      <Link key={item.href} href={item.href}>
                        <button
                          className={`group relative flex w-full cursor-pointer items-center ${sidebarCollapsed ? "justify-center px-0" : "justify-start px-6"} py-3 text-sm transition-colors ${
                            isActive
                              ? "border-l-[3px] border-l-[#66ff4c] bg-gradient-to-r from-[rgba(102,255,76,0.1)] to-[rgba(102,255,76,0)] text-white"
                              : "border-l-[3px] border-l-transparent text-[#9ca3af] hover:bg-[#1d1e29] hover:text-white"
                          }`}
                          title={sidebarCollapsed ? item.label : undefined}
                          onClick={() => {
                            if (isMobile) {
                              setMobileMenuOpen(false);
                            }
                          }}
                        >
                          <item.icon size={20} className={iconColorClass} />
                          {!sidebarCollapsed && <span className="ml-3">{item.label}</span>}
                          {item.countLoading ? (
                            <div className={`pointer-events-none absolute ${sidebarCollapsed ? "right-2 top-1.5" : "right-5 top-1/2 -translate-y-1/2"}`}>
                              <Skeleton className="h-4 w-7 rounded-full bg-[#2a2c3c]/70" />
                            </div>
                          ) : null}
                          {!item.countLoading && hasCountBadge && (
                            <div className={`pointer-events-none absolute ${sidebarCollapsed ? "right-2 top-1.5" : "right-5 top-1/2 -translate-y-1/2"}`}>
                              <div className="inline-flex items-center justify-center rounded-full bg-[#EB5F65] px-1.5 py-0.5 text-[10px] font-bold leading-none text-white whitespace-nowrap">
                                {item.count}
                              </div>
                            </div>
                          )}
                        </button>
                      </Link>
                    );
                  })}
                </div>
              );
            })}
          </nav>

        <div className="space-y-5 border-t border-[#1d1e29] p-4 pb-5 pt-5">
          {!sidebarCollapsed ? (
            <a
              href={config?.supportUrl || "#"}
              target="_blank"
              rel="noopener noreferrer"
              className="flex h-11 w-full cursor-pointer items-center gap-3 rounded-[16px] px-4 text-[#9ca3af] transition-colors duration-150 hover:text-white"
            >
              <HelpCircle size={20} />
              {t("nav.support")}
            </a>
          ) : (
            <div className="flex justify-center">
              <a href={config?.supportUrl || "#"} target="_blank" rel="noopener noreferrer">
                <Button
                  variant="ghost"
                  size="icon"
                  className="rounded-xl text-[#9ca3af] hover:bg-transparent hover:text-white"
                >
                  <HelpCircle size={20} />
                </Button>
              </a>
            </div>
          )}

          {!sidebarCollapsed && (
            <Popover open={statusPopoverOpen} onOpenChange={setStatusPopoverOpen}>
              <PopoverTrigger asChild>
                <button
                  type="button"
                  className="w-full select-none rounded-[24px] border border-[#2a2c3c] bg-[linear-gradient(180deg,#1f2230_0%,#1a1d2a_100%)] px-5 py-4 text-left transition-colors duration-200 hover:border-[#3a3d52]"
                  aria-expanded={statusPopoverOpen}
                  data-testid="sidebar-status-trigger"
                >
                  <div className="mb-2 flex w-full items-center justify-between text-[10px] font-bold uppercase tracking-[0.8px] text-[#9ca3af]">
                    Status
                    <ChevronDown size={14} className={`transition-transform duration-200 ${statusPopoverOpen ? "rotate-180" : ""}`} />
                  </div>
                  <div className="flex items-center gap-2.5 text-sm font-medium text-white">
                    <div
                      className={`h-2.5 w-2.5 rounded-full ${
                        overallStatusSeverity === 0
                          ? "bg-green-500"
                          : overallStatusSeverity > 1
                            ? "bg-red-500"
                            : "bg-yellow-500"
                      } animate-pulse`}
                    />
                    {overallStatusSeverity === 0 ? t("status.operational") : t("status.degraded")}
                  </div>
                </button>
              </PopoverTrigger>
              <PopoverContent
                side="top"
                align="start"
                sideOffset={10}
                className="w-[var(--radix-popover-trigger-width)] rounded-[20px] border border-[#2a2c3c] bg-[linear-gradient(180deg,#1f2230_0%,#1a1d2a_100%)] p-3 text-white shadow-[0_16px_40px_rgba(0,0,0,0.45)]"
                data-testid="sidebar-status-content"
              >
                <div className="space-y-1 pr-1" data-testid="sidebar-status-list">
                  {moduleEntries.length === 0 ? (
                    <p className="px-2 py-1 text-xs text-[#9ca3af]">No module data</p>
                  ) : (
                    moduleEntries.map(([name, mod]: [string, any]) => (
                      <div key={name} className="flex items-center justify-between rounded-lg px-2 py-1.5 text-[12px]">
                        <div className="flex min-w-0 items-center gap-2">
                          <div className={`h-2 w-2 rounded-full ${getModuleStatusColor(mod)}`} />
                          <EllipsisText text={name} className="capitalize text-[12px] leading-4 text-[#d1d5db]" />
                        </div>
                        <span className="ml-3 shrink-0 text-[12px] text-[#9ca3af]">{formatModuleResponseMs(Number(mod?.responseMs || 0), String(mod?.status || ""))}</span>
                      </div>
                    ))
                  )}
                </div>
              </PopoverContent>
            </Popover>
          )}
        </div>
        </div>
      </aside>

      {/* Main Content */}
      <main className="relative flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        {/* Top Bar */}
        <header className="sticky top-0 z-10 flex h-16 items-center justify-between border-b border-[#1d1e29] bg-[#0b0c10] px-4 md:px-6">
          <div className="flex items-center gap-6 flex-1">
            {isMobile && (
              <Button
                variant="ghost"
                size="icon"
                className="shrink-0 rounded-xl text-[#9ca3af] hover:bg-[#1d1e29] hover:text-white"
                onClick={() => setMobileMenuOpen(true)}
                data-testid="button-open-mobile-menu"
              >
                <Menu size={20} />
              </Button>
            )}
            {backHref && (
              <Link href={withTenantPath(currentTenantSlug, backHref)}>
                <Button variant="ghost" size="icon" className="rounded-full text-[#9ca3af] hover:bg-[#1d1e29] hover:text-white">
                  <ChevronLeft size={20} />
                </Button>
              </Link>
            )}

            {/* Tenant Switcher */}
            {isPlatformAdmin ? (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" className="max-w-[180px] gap-2 rounded-xl border border-[#2a2c3c] bg-[#13141c] px-3 text-white hover:bg-[#1d1e29] md:max-w-none" data-testid="tenant-switcher">
                    <Building2 size={18} className="text-[#9ca3af]" />
                    <EllipsisText text={currentTenantLabel} className="max-w-[120px] font-semibold md:max-w-[220px]" />
                    <ChevronDown size={14} className="text-[#9ca3af]" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start" className="w-56 rounded-xl border border-[#2a2c3c] bg-[#13141c] text-white shadow-[0_4px_20px_rgba(0,0,0,0.3)] animate-in fade-in zoom-in-95">
                  <DropdownMenuLabel>{t("tenant.switch")}</DropdownMenuLabel>
                  <DropdownMenuSeparator />
                  {Object.entries(tenantLabels).map(([id, tenant]) => (
                    <DropdownMenuItem key={id} onClick={() => handleTenantSwitch(id)} className="rounded-lg cursor-pointer">
                      <div className="flex flex-col">
                        <span>{tenant.slug}</span>
                        {tenant.name && tenant.name !== tenant.slug && (
                          <span className="text-[11px] text-muted-foreground">{tenant.name}</span>
                        )}
                      </div>
                      {id === currentTenantId && <CheckCircle2 size={14} className="ml-auto text-primary" />}
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
            ) : (
              <Button
                variant="ghost"
                className="max-w-[180px] cursor-default gap-2 rounded-xl border border-[#2a2c3c] bg-[#13141c] px-3 text-white hover:bg-[#13141c] md:max-w-none"
                data-testid="tenant-badge"
                disabled
              >
                <Building2 size={18} className="text-[#9ca3af]" />
                <EllipsisText text={currentTenantLabel} className="max-w-[120px] font-semibold md:max-w-[220px]" />
              </Button>
            )}

            <Popover
              open={searchOpen}
              onOpenChange={(open) => {
                setSearchOpen(open);
              }}
            >
              <PopoverAnchor asChild>
                <div className="relative max-w-md w-full group hidden md:block" data-testid="global-search">
                  <div className="pointer-events-none absolute inset-y-0 left-3 flex items-center text-[#6b7280] group-focus-within:text-[#66ff4c] transition-colors">
                    <Search size={18} />
                  </div>
                  <Input
                    ref={searchInputRef}
                    placeholder={t("layout.globalSearch")}
                    className="h-[38px] rounded-xl border-[#1d1e29] bg-[#0b0c10] pl-10 pr-16 text-white placeholder:text-[#6b7280] focus-visible:ring-[#66ff4c]/50"
                    value={searchQuery}
                    onFocus={() => {
                      if (isSearchQueryReady) {
                        setSearchOpen(true);
                      }
                    }}
                    onChange={(e) => {
                      const nextQuery = e.target.value;
                      setSearchQuery(nextQuery);
                      setSearchOpen(nextQuery.trim().length >= 2);
                    }}
                    onKeyDown={(e) => {
                      if (e.key === "Escape") {
                        setSearchOpen(false);
                        e.currentTarget.blur();
                      }
                    }}
                    data-testid="search-dialog-input"
                  />
                  <div className="absolute inset-y-0 right-3 flex items-center gap-1 pointer-events-none">
                    <kbd className="hidden h-5 select-none items-center gap-1 rounded border border-[#2a2c3c] bg-[#13141c] px-1.5 font-mono text-[10px] font-medium text-[#6b7280] opacity-100 sm:inline-flex">
                      <span className="text-xs">⌘</span>K
                    </kbd>
                  </div>
                </div>
              </PopoverAnchor>
              {shouldShowSearchPopover && (
                <PopoverContent
                  align="start"
                  side="bottom"
                  className="w-[min(92vw,640px)] p-0 rounded-xl fintech-shadow"
                  onOpenAutoFocus={(e) => e.preventDefault()}
                >
                  <div className="p-4 max-h-[420px] overflow-auto">
                    {showSearchLoadingState && (
                      <div className="space-y-3">
                        <Skeleton className="h-10 w-full rounded-lg" />
                        <Skeleton className="h-10 w-full rounded-lg" />
                        <Skeleton className="h-10 w-full rounded-lg" />
                        <Skeleton className="h-10 w-3/4 rounded-lg" />
                      </div>
                    )}
                    {!showSearchLoadingState && isDebouncedSearchReady && searchResults && (
                      <div className="space-y-4">
                        {searchResults.alerts?.length > 0 && (
                          <div>
                            <h4 className="text-xs font-bold uppercase tracking-wider text-muted-foreground mb-2">{t("layout.search.alerts")}</h4>
                            <div className="space-y-1">
                              {searchResults.alerts.map((alert: any) => (
                                <button
                                  key={alert.id}
                                  className="w-full text-left px-3 py-2 rounded-lg hover:bg-secondary transition-colors flex items-center gap-3 cursor-pointer"
                                  onClick={() => {
                                    setLocation(withTenantPath(currentTenantSlug, `/alerts/${alert.id}`));
                                    setSearchOpen(false);
                                    setSearchQuery("");
                                  }}
                                  data-testid={`search-result-alert-${alert.id}`}
                                >
                                  <Bell size={14} className="text-muted-foreground shrink-0" />
                                  <div className="flex-1 min-w-0">
                                    <EllipsisText text={alert.title} className="text-sm font-medium max-w-[300px]" />
                                    <EllipsisText text={`${alert.severity} · ${alert.status}`} className="text-xs text-muted-foreground max-w-[300px]" />
                                  </div>
                                </button>
                              ))}
                            </div>
                          </div>
                        )}
                        {searchResults.cases?.length > 0 && (
                          <div>
                            <h4 className="text-xs font-bold uppercase tracking-wider text-muted-foreground mb-2">{t("layout.search.cases")}</h4>
                            <div className="space-y-1">
                              {searchResults.cases.map((c: any) => (
                                <button
                                  key={c.id}
                                  className="w-full text-left px-3 py-2 rounded-lg hover:bg-secondary transition-colors flex items-center gap-3 cursor-pointer"
                                  onClick={() => {
                                    setLocation(withTenantPath(currentTenantSlug, `/cases/${c.id}`));
                                    setSearchOpen(false);
                                    setSearchQuery("");
                                  }}
                                  data-testid={`search-result-case-${c.id}`}
                                >
                                  <BriefcaseBusiness size={14} className="text-muted-foreground shrink-0" />
                                  <div className="flex-1 min-w-0">
                                    <EllipsisText text={c.title} className="text-sm font-medium max-w-[300px]" />
                                    <EllipsisText text={`${c.severity} · ${c.status}`} className="text-xs text-muted-foreground max-w-[300px]" />
                                  </div>
                                </button>
                              ))}
                            </div>
                          </div>
                        )}
                        {searchResults.threads?.length > 0 && (
                          <div>
                            <h4 className="text-xs font-bold uppercase tracking-wider text-muted-foreground mb-2">{t("layout.search.threads")}</h4>
                            <div className="space-y-1">
                              {searchResults.threads.map((thread: any) => (
                                <button
                                  key={thread.id}
                                  className="w-full text-left px-3 py-2 rounded-lg hover:bg-secondary transition-colors flex items-center gap-3 cursor-pointer"
                                  onClick={() => {
                                    setLocation(withTenantPath(currentTenantSlug, `/forum/${thread.id}`));
                                    setSearchOpen(false);
                                    setSearchQuery("");
                                  }}
                                  data-testid={`search-result-thread-${thread.id}`}
                                >
                                  <MessageCircle size={14} className="text-muted-foreground shrink-0" />
                                  <div className="flex-1 min-w-0">
                                    <EllipsisText text={thread.title} className="text-sm font-medium max-w-[300px]" />
                                    <EllipsisText text={thread.category} className="text-xs text-muted-foreground max-w-[300px]" />
                                  </div>
                                </button>
                              ))}
                            </div>
                          </div>
                        )}
                        {!hasSearchResults && (
                          <p className="text-sm text-muted-foreground text-center py-8">{t("layout.search.noResults").replace("{query}", searchQuery)}</p>
                        )}
                      </div>
                    )}
                  </div>
                </PopoverContent>
              )}
            </Popover>
          </div>

          <div className="flex items-center gap-3">
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="default" className="hidden h-8 rounded-lg gap-2 px-3 text-[14px] sm:inline-flex" data-testid="quick-action">
                  <Plus size={18} />
                  {t("layout.quickAction")}
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-56 rounded-xl border border-[#2a2c3c] bg-[#13141c] text-white shadow-[0_4px_20px_rgba(0,0,0,0.3)]">
                <DropdownMenuLabel>{t("layout.instantOperations")}</DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  onClick={() => openQuickAction("add_alert")}
                  className="cursor-pointer gap-2 rounded-md focus:bg-[#1d1e29]"
                  data-testid="quick-action-item-add-alert"
                >
                  <ShieldAlert size={16} /> Add Alert
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() => openQuickAction("add_case")}
                  className="cursor-pointer gap-2 rounded-md focus:bg-[#1d1e29]"
                  data-testid="quick-action-item-add-case"
                >
                  <BriefcaseBusiness size={16} /> Add Case
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() => openQuickAction("run_connector")}
                  className="cursor-pointer gap-2 rounded-md focus:bg-[#1d1e29]"
                  data-testid="quick-action-item-run-connector"
                >
                  <Plug size={16} /> Run Connector
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>

            <Popover>
              <PopoverTrigger asChild>
                <Button variant="ghost" size="icon" className="relative rounded-lg text-[#9ca3af] hover:bg-[#1d1e29] hover:text-white" data-testid="notifications">
                  <Bell size={20} />
                  {unreadCount > 0 && (
                    <span className="absolute -top-1 -right-1 w-5 h-5 bg-red-500 text-white text-[10px] font-bold rounded-full flex items-center justify-center">
                      {unreadCount}
                    </span>
                  )}
                </Button>
              </PopoverTrigger>
              <PopoverContent align="end" className="w-96 max-h-96 overflow-hidden rounded-xl border border-[#2a2c3c] bg-[#13141c] p-0 text-white">
                <div className="flex items-center justify-between border-b border-[#2a2c3c] p-4">
                  <h3 className="font-semibold">{t("notifications.title")}</h3>
                  <div className="flex items-center gap-1">
                    {unreadCount > 0 && (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => markAllRead.mutate({ userId: currentUserId, tenantId: currentTenantId })}
                      >
                        {t("notifications.markAllRead")}
                      </Button>
                    )}
                    {notifications.length > 0 && (
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-[#eb5f65] hover:bg-[rgba(235,95,101,0.12)] hover:text-[#f87171]"
                        onClick={() => clearNotifications.mutate({ userId: currentUserId, tenantId: currentTenantId })}
                      >
                        {t("notifications.clearAll")}
                      </Button>
                    )}
                  </div>
                </div>
                <div className="max-h-72 overflow-y-auto">
                  {notifications.length === 0 ? (
                    <p className="p-4 text-center text-sm text-[#6b7280]">{t("notifications.empty")}</p>
                  ) : (
                    notifications.map((n: any) => (
                      <div
                        key={n.id}
                        className={`cursor-pointer border-b border-[#2a2c3c] p-3 transition-colors last:border-b-0 hover:bg-[#1d1e29] ${!n.read ? "bg-[#1d1e29]" : ""}`}
                        onClick={() => !n.read && markRead.mutate(n.id)}
                      >
                        <div className="flex items-start gap-3">
                          <div className={`w-2 h-2 mt-2 rounded-full flex-shrink-0 ${n.type === "error" ? "bg-red-500" : n.type === "warning" ? "bg-yellow-500" : n.type === "success" ? "bg-green-500" : "bg-blue-500"}`} />
                          <div className="flex-1 min-w-0">
                            <p className="text-sm font-medium">{n.title}</p>
                            <p className="mt-0.5 text-xs text-[#9ca3af]">{n.message}</p>
                            <p className="mt-1 text-xs text-[#6b7280]">{new Date(n.createdAt).toLocaleString()}</p>
                          </div>
                          <div className="flex items-start gap-1">
                            {n.global && <Badge variant="outline" className="text-[10px] flex-shrink-0">{t("layout.badge.global")}</Badge>}
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon"
                              className="h-7 w-7 rounded-lg text-[#6b7280] hover:bg-[rgba(235,95,101,0.12)] hover:text-[#f87171]"
                              onClick={(event) => {
                                event.stopPropagation();
                                deleteNotification.mutate(n.id);
                              }}
                              title={t("notifications.delete")}
                              aria-label={t("notifications.delete")}
                            >
                              <Trash2 size={14} />
                            </Button>
                          </div>
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </PopoverContent>
            </Popover>

            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon" className="rounded-xl p-0 text-[#9ca3af] hover:bg-[#1d1e29] hover:text-white" data-testid="user-profile">
                  <Avatar className="h-9 w-9 border border-[#2a2c3c] shadow-none">
                    <AvatarImage src={currentUserAvatar || undefined} />
                    <AvatarFallback className="bg-[#4ed938] font-bold text-[#0b0c10]">
                      {currentUserInitials}
                    </AvatarFallback>
                  </Avatar>
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-56 rounded-xl border border-[#2a2c3c] bg-[#13141c] text-white shadow-[0_4px_20px_rgba(0,0,0,0.3)] animate-in slide-in-from-top-2">
                <DropdownMenuLabel>{t("layout.account")}</DropdownMenuLabel>
                <DropdownMenuSeparator />
                <Link href={withTenantPath(currentTenantSlug, "/profile")}>
                  <DropdownMenuItem className="cursor-pointer">{t("nav.profile")}</DropdownMenuItem>
                </Link>
                <Link href={withTenantPath(currentTenantSlug, "/security")}>
                  <DropdownMenuItem className="cursor-pointer">{t("profile.security")}</DropdownMenuItem>
                </Link>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  className="cursor-pointer"
                  onClick={() => {
                    const next = language === "en" ? "ru" : "en";
                    setLanguage(next);
                  }}
                  data-testid="language-switcher"
                >
                  <Globe size={16} className="mr-2" />
                  {t("lang.switch")}: {language.toUpperCase()}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={toggleTheme} className="cursor-pointer">
                  {theme === "light" ? <Moon size={16} className="mr-2" /> : <Sun size={16} className="mr-2" />}
                  {theme === "light" ? t("theme.dark") : t("theme.light")}
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  className="text-destructive cursor-pointer"
                  onClick={async () => {
                    await logout();
                    setLocation("/login");
                  }}
                >
                  {t("nav.signOut")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </header>

        {/* Workspace */}
        <div
          className="relative min-h-0 flex-1 overflow-y-auto bg-[#0b0c10] p-4 md:p-8"
          data-testid="layout-scroll-region"
        >
          {children}

          {/* AI Launcher */}
          {!aiHidden ? (
            <div className="fixed bottom-8 right-8 z-20 flex flex-col items-end gap-4">
              <div className="group relative">
                <Button
                  size="icon"
                  className="ai-fab-button relative h-14 w-14 rounded-full shadow-[0_0_10px_rgba(102,255,76,0.2)] transition-transform active:scale-90"
                  data-testid="ai-toggle"
                  onClick={() => {
                    setAiOpen(true);
                    setAiHasUnread(false);
                  }}
                >
                  <span className="ai-fab-icon" aria-hidden="true">
                    <WandSparkles size={22} className="ai-fab-single-star" />
                  </span>
                  {aiHasUnread && (
                    <span className="absolute -top-1 -right-1 h-4 w-4 animate-bounce rounded-full border-2 border-white bg-red-500" />
                  )}
                </Button>
                <button
                  type="button"
                  className="absolute -left-2 -top-2 flex h-6 w-6 items-center justify-center rounded-full border border-[#2a2c3c] bg-[#1d1e29] opacity-0 shadow-sm transition-all hover:bg-destructive hover:text-white group-hover:opacity-100"
                  onClick={() => setAiHidden(true)}
                  data-testid="ai-hide"
                  aria-label={language === "ru" ? "Свернуть кнопку ИИ" : "Collapse AI launcher"}
                >
                  <X size={12} />
                </button>
              </div>
            </div>
          ) : (
            <button
              type="button"
              className="fixed bottom-8 right-0 z-20 flex h-14 w-8 cursor-pointer items-center justify-center rounded-l-full bg-primary shadow-lg"
              onClick={() => setAiHidden(false)}
              data-testid="ai-show"
              aria-label={language === "ru" ? "Показать кнопку ИИ" : "Show AI launcher"}
            >
              <ChevronLeft size={16} className="text-primary-foreground" />
            </button>
          )}
        </div>
      </main>

      <Dialog
        open={quickActionKind !== "none"}
        onOpenChange={(open) => {
          if (open) {
            return;
          }
          if (createAlert.isPending || createCase.isPending || executeConnectorHub.isPending) {
            return;
          }
          closeQuickActionDialog();
        }}
      >
        <DialogContent className="rounded-2xl">
          <DialogTitle>
            {quickActionKind === "add_alert" && "Quick Action: Add Alert"}
            {quickActionKind === "add_case" && "Quick Action: Add Case"}
            {quickActionKind === "run_connector" && "Quick Action: Run Connector"}
          </DialogTitle>
          <DialogDescription>
            {quickActionKind === "add_alert" && "Create a new alert without leaving the current screen."}
            {quickActionKind === "add_case" && "Create a new case without leaving the current screen."}
            {quickActionKind === "run_connector" && "Start a manual connector execution from anywhere in the app."}
          </DialogDescription>

          {quickActionKind === "add_alert" && (
            <div className="space-y-3">
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-muted-foreground">Alert title</label>
                <Input
                  value={quickAlertTitle}
                  onChange={(event) => setQuickAlertTitle(event.target.value)}
                  placeholder="Suspicious login from VPN gateway"
                  data-testid="quick-action-alert-title"
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-muted-foreground">Severity</label>
                <Select value={quickAlertSeverity} onValueChange={setQuickAlertSeverity}>
                  <SelectTrigger data-testid="quick-action-alert-severity">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="Critical">{t("severity.critical")}</SelectItem>
                    <SelectItem value="High">{t("severity.high")}</SelectItem>
                    <SelectItem value="Medium">{t("severity.medium")}</SelectItem>
                    <SelectItem value="Low">{t("severity.low")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-muted-foreground">Description</label>
                <Textarea
                  value={quickAlertDescription}
                  onChange={(event) => setQuickAlertDescription(event.target.value)}
                  placeholder="Describe what happened and why the alert matters"
                  rows={4}
                  data-testid="quick-action-alert-description"
                />
              </div>
            </div>
          )}

          {quickActionKind === "add_case" && (
            <div className="space-y-3">
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-muted-foreground">Case title</label>
                <Input
                  value={quickCaseTitle}
                  onChange={(event) => setQuickCaseTitle(event.target.value)}
                  placeholder={t("layout.quick.form.caseTitlePlaceholder")}
                  data-testid="quick-action-case-title"
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-muted-foreground">Severity</label>
                <Select value={quickCaseSeverity} onValueChange={setQuickCaseSeverity}>
                  <SelectTrigger data-testid="quick-action-case-severity">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="Critical">{t("severity.critical")}</SelectItem>
                    <SelectItem value="High">{t("severity.high")}</SelectItem>
                    <SelectItem value="Medium">{t("severity.medium")}</SelectItem>
                    <SelectItem value="Low">{t("severity.low")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-muted-foreground">Description</label>
                <Textarea
                  value={quickCaseDescription}
                  onChange={(event) => setQuickCaseDescription(event.target.value)}
                  placeholder={t("layout.quick.form.caseDescriptionPlaceholder")}
                  rows={4}
                  data-testid="quick-action-case-description"
                />
              </div>
            </div>
          )}

          {quickActionKind === "run_connector" && (
            <div className="space-y-3">
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-muted-foreground">Connector</label>
                <Select value={quickConnectorId || "none"} onValueChange={(value) => setQuickConnectorId(value === "none" ? "" : value)}>
                  <SelectTrigger data-testid="quick-action-connector-id">
                    <SelectValue placeholder="Choose connector" />
                  </SelectTrigger>
                  <SelectContent searchable searchPlaceholder="Search connectors...">
                    <SelectItem value="none">Choose connector</SelectItem>
                    {enabledOutboundConnectors.map((item: any) => (
                      <SelectItem key={item.id} value={String(item.id)}>{item.name || item.id}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-muted-foreground">Method</label>
                <Select value={quickConnectorMethodId || "none"} onValueChange={(value) => setQuickConnectorMethodId(value === "none" ? "" : value)}>
                  <SelectTrigger data-testid="quick-action-connector-method">
                    <SelectValue placeholder="Choose method" />
                  </SelectTrigger>
                  <SelectContent searchable searchPlaceholder="Search methods...">
                    <SelectItem value="none">Choose method</SelectItem>
                    {enabledQuickConnectorMethods.map((item: any) => (
                      <SelectItem key={item.id} value={String(item.id)}>{String(item?.data?.name || item?.name || item.id)}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-muted-foreground">Message</label>
                <Textarea
                  value={quickConnectorMessage}
                  onChange={(event) => setQuickConnectorMessage(event.target.value)}
                  placeholder="Optional plain-text message for the connector method"
                  rows={3}
                  data-testid="quick-action-connector-message"
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-muted-foreground">Metadata JSON</label>
                <Textarea
                  value={quickConnectorMetadata}
                  onChange={(event) => setQuickConnectorMetadata(event.target.value)}
                  placeholder='{"observable":{"type":"ip","value":"1.1.1.1"}}'
                  className="font-mono text-xs"
                  rows={5}
                  data-testid="quick-action-connector-metadata"
                />
              </div>
            </div>
          )}

          <DialogFooter>
            <Button
              variant="outline"
              onClick={closeQuickActionDialog}
              disabled={createAlert.isPending || createCase.isPending || executeConnectorHub.isPending}
            >
              {t("common.cancel")}
            </Button>
            {quickActionKind === "add_alert" && (
              <Button
                onClick={submitQuickAlert}
                disabled={createAlert.isPending}
                data-testid="quick-action-alert-submit"
              >
                {createAlert.isPending ? t("common.loading") : "Add Alert"}
              </Button>
            )}
            {quickActionKind === "add_case" && (
              <Button
                onClick={submitQuickCase}
                disabled={createCase.isPending}
                data-testid="quick-action-case-submit"
              >
                {createCase.isPending ? t("common.loading") : "Add Case"}
              </Button>
            )}
            {quickActionKind === "run_connector" && (
              <Button
                onClick={submitQuickConnector}
                disabled={executeConnectorHub.isPending || enabledQuickConnectorMethods.length === 0}
                data-testid="quick-action-connector-submit"
              >
                {executeConnectorHub.isPending ? t("common.loading") : "Run Connector"}
              </Button>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <section
        className={`fixed z-30 flex min-h-0 flex-col overflow-hidden rounded-2xl border border-[#2a2c3c] bg-[#13141c] text-white fintech-shadow transition-transform duration-300 ease-out ${
          aiOpen ? "translate-x-0" : "pointer-events-none translate-x-[110%]"
        } ${
          isMobile
            ? "inset-x-2 bottom-2 top-16"
            : "bottom-8 right-8 h-[min(78dvh,760px)] w-[min(92vw,520px)]"
        }`}
        role="dialog"
        aria-modal="true"
        aria-label={t("layout.ai.title")}
        aria-hidden={!aiOpen}
        data-testid="ai-chat-panel"
      >
            <div id="chat-header" className="flex items-center justify-between border-b border-[#1d1e29] px-5 py-4">
              <div className="flex min-w-0 items-center gap-4">
                <div className="relative">
                  <div className="flex h-11 w-11 items-center justify-center rounded-xl bg-gradient-to-br from-[#8B5CF6] via-[#EC4899] to-[#3B82F6] shadow-[0_0_16px_rgba(139,92,246,0.35)]">
                    <Sparkles size={18} className="text-white" />
                  </div>
                  <span className="absolute -bottom-0.5 -right-0.5 flex h-3.5 w-3.5">
                    <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-70" />
                    <span className="relative inline-flex h-3.5 w-3.5 rounded-full border border-[#13141c] bg-[#66FF4C]" />
                  </span>
                </div>
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <h2 className="truncate text-base font-bold text-white md:text-lg">{t("layout.ai.title")}</h2>
                    <span className="rounded bg-[#8B5CF6]/25 px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide text-[#c4b5fd]">
                      Pro
                    </span>
                  </div>
                  <p className="truncate text-xs text-[#9ca3af]">{t("layout.ai.subtitle")}</p>
                </div>
              </div>

              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8 rounded-lg text-[#9ca3af] hover:bg-[#1d1e29] hover:text-white"
                onClick={() => setAiOpen(false)}
                aria-label={t("common.close")}
                title={t("common.close")}
              >
                <X size={18} />
              </Button>
            </div>

            <div className="flex min-h-0 flex-1 flex-col" data-testid="ai-chat-body">
            <div
              id="chat-messages"
              ref={aiMessagesContainerRef}
              className="min-h-0 flex-1 overflow-y-auto overscroll-y-contain px-4 py-5 md:px-6 md:py-6"
              data-testid="ai-chat-messages"
            >
              <div className="flex min-h-full flex-col justify-end gap-4" data-testid="ai-chat-messages-stack">
                {displayedAIMessages.length === 0 ? (
                  <div className="rounded-2xl border border-[#2a2c3c] bg-[#1d1e29]/60 p-4 text-sm text-[#9ca3af]">
                    {activeAISession ? t("layout.ai.noMessages") : t("layout.ai.initializing")}
                  </div>
                ) : (
                  displayedAIMessages.map((message: any) => {
                    const isUserMessage = message.role === "user";
                    const timestamp = formatAIMessageTimestamp(
                      message.createdAt || message.created_at || message.updatedAt || message.updated_at,
                    );
                    return (
                      <div key={message.id} className={`flex gap-3 ${isUserMessage ? "justify-end" : ""}`}>
                        {!isUserMessage && (
                          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-gradient-to-br from-[#8B5CF6] via-[#EC4899] to-[#3B82F6]">
                            <Sparkles size={14} className="text-white" />
                          </div>
                        )}

                        <div className={`max-w-[min(100%,48rem)] ${isUserMessage ? "text-right" : ""}`}>
                          <div className={`mb-1 flex items-center gap-2 text-xs ${isUserMessage ? "justify-end" : ""}`}>
                            {isUserMessage ? (
                              <>
                                {timestamp && <span className="text-[#6b7280]">{timestamp}</span>}
                                <span className="font-semibold text-white">{t("layout.ai.you")}</span>
                              </>
                            ) : (
                              <>
                                <span className="font-semibold text-white">{t("layout.ai.assistant")}</span>
                                {timestamp && <span className="text-[#6b7280]">{timestamp}</span>}
                              </>
                            )}
                          </div>

                          <div
                            className={`rounded-2xl border p-4 text-sm leading-relaxed ${
                              isUserMessage
                                ? "rounded-tr-sm border-[#66FF4C]/40 bg-[#66FF4C]/10 text-[#e5e7eb]"
                                : "rounded-tl-sm border-[#2a2c3c] bg-[#1d1e29]/70 text-[#d1d5db]"
                            }`}
                          >
                            {message.isThinking ? (
                              <div className="flex items-center gap-1.5">
                                {[0, 150, 300].map((delay) => (
                                  <span
                                    key={delay}
                                    className="h-2 w-2 animate-bounce rounded-full bg-[#9ca3af]"
                                    style={{ animationDelay: `${delay}ms`, animationDuration: "1.1s" }}
                                  />
                                ))}
                                <span className="ml-1 text-xs text-[#9ca3af]">{t("layout.ai.thinking")}</span>
                              </div>
                            ) : (
                              <div className="whitespace-pre-wrap break-words">{String(message.content || "")}</div>
                            )}
                          </div>
                        </div>

                        {isUserMessage && (
                          <Avatar className="h-8 w-8 shrink-0 border border-[#2a2c3c]">
                            <AvatarImage src={currentUserAvatar || undefined} />
                            <AvatarFallback className="bg-[#4ed938] font-bold text-[#0b0c10]">
                              {currentUserInitials}
                            </AvatarFallback>
                          </Avatar>
                        )}
                      </div>
                    );
                  })
                )}
              </div>
            </div>

            <div id="quick-actions" className="shrink-0 border-t border-[#1d1e29] px-4 pb-4 pt-3 md:px-6">
              <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.08em] text-[#6b7280]">
                {t("layout.ai.quickActions")}
              </div>
              <div className="grid grid-cols-2 gap-2">
                {aiQuickActions.map((item: any) => {
                  const Icon = item.icon;
                  return (
                    <button
                      key={item.id}
                      type="button"
                      onClick={() => {
                        if (item.submit) {
                          sendAIQuestion(item.prompt);
                          return;
                        }
                        setAiQuestion(item.prompt);
                        window.requestAnimationFrame(() => {
                          aiComposerRef.current?.focus();
                        });
                      }}
                      disabled={aiBusy}
                      className="rounded-lg border border-[#2a2c3c] bg-[#1d1e29]/55 p-3 text-left transition-all hover:-translate-y-0.5 hover:border-[#66FF4C]/45 hover:bg-[#1d1e29] disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      <Icon size={14} className={`mb-1 ${item.colorClass}`} />
                      <p className="text-xs font-semibold text-white">{item.title}</p>
                    </button>
                  );
                })}
              </div>
            </div>

            <div id="chat-input" className="shrink-0 border-t border-[#1d1e29] px-4 pb-4 pt-3 md:px-6 md:pb-5">
              <div className="relative">
                <input
                  ref={aiAttachmentInputRef}
                  type="file"
                  className="hidden"
                  multiple
                  onChange={handleSelectAIAttachments}
                  data-testid="ai-chat-attachment-input"
                />
                {aiAttachments.length > 0 ? (
                  <div className="mb-2 flex flex-wrap gap-1.5">
                    {aiAttachments.map((file, index) => (
                      <button
                        key={`${file.name}:${file.size}:${file.lastModified}:${index}`}
                        type="button"
                        onClick={() => removeAIAttachment(index)}
                        className="inline-flex max-w-full items-center gap-1 rounded-md border border-[#2a2c3c] bg-[#1d1e29] px-2 py-1 text-[11px] text-[#d1d5db] transition-colors hover:bg-[#2a2c3c]"
                        title={language === "ru" ? "Удалить вложение" : "Remove attachment"}
                      >
                        <span className="truncate">{file.name}</span>
                        <X size={12} className="shrink-0 text-[#9ca3af]" />
                      </button>
                    ))}
                  </div>
                ) : null}
                <Textarea
                  ref={aiComposerRef}
                  rows={1}
                  value={aiQuestion}
                  onChange={(event) => setAiQuestion(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter" && !event.shiftKey) {
                      event.preventDefault();
                      if (aiBusy) {
                        return;
                      }
                      if (aiQuestion.trim().length >= 2) {
                        sendAIQuestion();
                      }
                    }
                  }}
                  placeholder={t("layout.ai.inputPlaceholder")}
                  className="min-h-[44px] max-h-[120px] w-full resize-none rounded-xl border border-[#2a2c3c] bg-[#1d1e29] px-4 py-3 pr-36 text-sm text-[#d1d5db] placeholder:text-[#6b7280] focus-visible:ring-2 focus-visible:ring-[#66FF4C]/25 focus-visible:border-[#66FF4C]"
                  data-testid="ai-chat-input"
                />
                <div className="absolute bottom-2 right-2 flex items-center gap-1.5">
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8 rounded-lg text-[#9ca3af] hover:bg-[#2a2c3c] hover:text-white"
                    onClick={() => aiAttachmentInputRef.current?.click()}
                    title={language === "ru" ? "Прикрепить файл" : "Attach file"}
                    aria-label={language === "ru" ? "Прикрепить файл" : "Attach file"}
                    data-testid="ai-chat-attach-button"
                  >
                    <Paperclip size={14} />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className={`h-8 w-8 rounded-lg ${aiVoiceListening ? "bg-[#66FF4C]/20 text-[#66FF4C]" : "text-[#9ca3af] hover:bg-[#2a2c3c] hover:text-white"}`}
                    onClick={toggleAIVoiceInput}
                    title={language === "ru" ? "Голосовой ввод" : "Voice input"}
                    aria-label={language === "ru" ? "Голосовой ввод" : "Voice input"}
                    data-testid="ai-chat-voice-button"
                  >
                    <Mic size={14} />
                  </Button>
                  <Button
                    type="button"
                    className={`h-8 min-w-20 rounded-lg px-3 text-xs font-semibold transition-colors ${
                      askAI.isPending
                        ? "bg-[#EB5F65] text-white hover:bg-[#d64a50]"
                        : "bg-gradient-to-r from-[#4ED938] to-[#2cae48] text-[#0b0c10] hover:from-[#66FF4C] hover:to-[#3bc95c]"
                    }`}
                    disabled={(!askAI.isPending && aiQuestion.trim().length < 2) || (aiBusy && !askAI.isPending)}
                    onClick={() => {
                      if (askAI.isPending) {
                        stopAIRequest();
                        return;
                      }
                      sendAIQuestion();
                    }}
                    aria-label={askAI.isPending ? t("layout.ai.stop") : t("layout.ai.send")}
                    title={askAI.isPending ? t("layout.ai.stop") : t("layout.ai.send")}
                    data-testid="ai-send-toggle"
                  >
                    {askAI.isPending ? (
                      <span className="inline-flex items-center gap-1.5">
                        <Square size={12} className="fill-current" />
                        {t("layout.ai.stop")}
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1.5">
                        {t("layout.ai.send")}
                        <Send size={12} />
                      </span>
                    )}
                  </Button>
                </div>
              </div>

              <div className="mt-2 flex items-center justify-between text-[11px] text-[#6b7280]">
                <span className="inline-flex items-center gap-1">
                  <ShieldCheck size={12} />
                  {language === "ru" ? "Защищенный канал" : "Secure channel"}
                </span>
                <span>{language === "ru" ? "Enter - отправка, Shift+Enter - новая строка" : "Press Enter to send, Shift+Enter for new line"}</span>
              </div>
            </div>
            </div>
      </section>

    </div>
  );
}
