import { useEffect, useMemo, useState } from "react";
import { useLocation } from "wouter";
import { AppLayout } from "@/components/layout";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import { Checkbox } from "@/components/ui/checkbox";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { AIEntityTrace, type AIEntityTraceActionContext } from "@/components/ai-entity-trace";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
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
import {
  useAIAgentEntityTrace,
  useAIAgentOperationsOverview,
  useAIAgentRuns,
  useAIAgentRunsFeed,
  useAIAgents,
  useAppState,
  useCloseAIAgentWorkload,
  useCreateAIAgent,
  useDeleteAIAgent,
  useOutboundConnectors,
  useRestartAIAgentWorkload,
  useRunAIAgent,
  useUpdateAIAgent,
  useUsers,
} from "@/lib/api";
import { useMinimumLoading } from "@/lib/use-minimum-loading";
import { toast } from "sonner";
import { OperationsTraceDrawer } from "./components/operations-trace-drawer";
import {
  buildAgentPayload,
  buildEntityOpenLabel,
  buildEntityTraceLocation,
  buildEventTracePayload,
  buildOperationsDrawerEventLog,
  buildOperationsTraceSubtitle,
  buildOperationsTraceTitle,
  buildWorkloadAgentProgress,
  defaultAgentDraft,
  formatDateTime,
  formatDuration,
  formatExecutionPolicy,
  normalizeConnectorOption,
  normalizeEntityType,
  parseIdentifierList,
  positiveInt,
  readTriadCaseEntries,
  resolveOperationsTraceWorkload,
  toAgentDraft,
  workloadStatusBadgeClass,
  type AIAgentsTab,
  type AgentDraft,
  type ConnectorOption,
  type InvestigationStageDraft,
  type OperationsTraceDrawerState,
} from "./helpers";
import {
  Activity,
  AlertTriangle,
  BarChart3,
  Bot,
  Cable,
  ChevronDown,
  ChevronRight,
  CheckCircle2,
  Clock3,
  Loader2,
  Play,
  PlusCircle,
  Plus,
  Save,
  Sparkles,
  Target,
  Tag,
  Trash2,
  WandSparkles,
  ArrowUpRight,
  Eye,
  X,
} from "lucide-react";

const PAGE_SHELL_CLASS = "mx-auto w-full max-w-[1240px] space-y-6 pb-8";
const PANEL_CLASS = "rounded-2xl border border-[#2a2c3c] bg-[#13141c] shadow-none";
const SUBPANEL_CLASS = "rounded-xl border border-[#2a2c3c] bg-[#111622]";
const INPUT_CLASS =
  "h-[38px] border-[#2a2c3c] bg-[#0b0c10] text-[#f3f4f6] placeholder:text-[#6b7280] focus-visible:ring-1 focus-visible:ring-[#3b4a79]";
const SELECT_TRIGGER_CLASS =
  "h-[38px] rounded-lg border border-[#2a2c3c] bg-[#0b0c10] text-sm text-[#d1d5db] focus:ring-1 focus:ring-[#3b4a79]";
const SELECT_CONTENT_CLASS = "rounded-xl border border-[#2a2c3c] bg-[#13141c] text-white";
const TEXTAREA_CLASS =
  "border-[#2a2c3c] bg-[#0b0c10] text-[#f3f4f6] placeholder:text-[#6b7280] focus-visible:ring-1 focus-visible:ring-[#3b4a79]";
const OUTLINE_BUTTON_CLASS = "border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db] hover:bg-[#171b2a] hover:text-white";
const PRIMARY_BUTTON_CLASS = "bg-[#11141d] text-[#f3f4f6] hover:bg-[#1d2433]";
const MUTED_TEXT_CLASS = "text-xs text-[#8b91a3]";
const AI_AGENT_TAB_LIST_CLASS = "flex h-11 w-full flex-wrap items-stretch justify-start overflow-hidden rounded-full border border-[#2a2c3c] bg-[#111622] p-0";
const AI_AGENT_TAB_TRIGGER_CLASS =
  "relative flex flex-1 min-w-[150px] items-center justify-center gap-2 px-4 text-xs font-semibold text-[#9ca3af] transition-colors hover:bg-[#171b2a] hover:text-[#e5e7eb] data-[state=active]:bg-[#111827] data-[state=active]:text-white data-[state=active]:after:absolute data-[state=active]:after:bottom-0 data-[state=active]:after:left-3 data-[state=active]:after:right-3 data-[state=active]:after:h-[2px] data-[state=active]:rounded-full data-[state=active]:after:bg-[#66ff4c]";

const OPERATIONS_TRACE_DRAWER_REFRESH_MS = 2000;

const TRIAD_PAGE_SIZE = 10;
const RUN_HISTORY_PAGE_SIZE = 10;

type AgentBuilderPane = "list" | "editor";

const AI_TRACE_LABELS = {
  title: "AI Workloads",
  subtitle: "Per-workload trace with execution stages and connector calls.",
  empty: "No AI workloads were recorded for this entity yet.",
  queueEvent: "Queue Event",
  source: "Source",
  lastUpdated: "Last Updated",
  attempts: "Attempts",
  execution: "Execution",
  stage: "Stage",
  verdict: "Verdict",
  blockers: "Action Blockers",
  error: "Error",
  noWorkloads: "No workloads were recorded for this event.",
  stageTimeline: "Stage Timeline",
  connectorTimeline: "Connector Timeline",
  connector: "Connector",
  duration: "Duration",
  reply: "Reply",
  caseTags: "Case Tags",
  restart: "Restart",
  close: "Close",
  liveEventLog: "Live Event Log",
  liveEventLogSubtitle: "Auto-refresh keeps stage and connector updates visible in real time.",
  live: "Live",
  refreshing: "Refreshing",
  refresh: "Refresh",
  waitingForUpdate: "Waiting for workload events.",
};

export default function AIAgentsPage() {
  const { currentTenantId, currentTenantSlug } = useAppState();
  const [, setLocation] = useLocation();
  const { data: agents = [], isLoading: agentsLoading } = useAIAgents(currentTenantId);
  const { data: outboundConnectors = [], isLoading: connectorsLoading } = useOutboundConnectors(currentTenantId);
  const { data: tenantUsers = [] } = useUsers(currentTenantId);
  const { data: allRunsFeed = [], isLoading: runsFeedLoading } = useAIAgentRunsFeed(currentTenantId, 260);
  const {
    data: operationsOverview,
    isLoading: operationsLoading,
    isFetching: operationsOverviewFetching,
    refetch: refetchOperationsOverview,
  } = useAIAgentOperationsOverview(currentTenantId, 30);
  const createAgent = useCreateAIAgent();
  const updateAgent = useUpdateAIAgent();
  const deleteAgent = useDeleteAIAgent();
  const runAgent = useRunAIAgent();
  const restartWorkload = useRestartAIAgentWorkload();
  const closeWorkload = useCloseAIAgentWorkload();

  const [activeTab, setActiveTab] = useState<AIAgentsTab>("builder");
  const [selectedAgentId, setSelectedAgentId] = useState("");
  const [triadAgentFilter, setTriadAgentFilter] = useState("all");
  const [triadPage, setTriadPage] = useState(1);
  const [runPage, setRunPage] = useState(1);
  const [draft, setDraft] = useState<AgentDraft>(defaultAgentDraft);
  const [createMode, setCreateMode] = useState(false);
  const [runDryMode, setRunDryMode] = useState(false);
  const [runLimitInput, setRunLimitInput] = useState("");
  const [runCaseIdsInput, setRunCaseIdsInput] = useState("");
  const [runAlertIdsInput, setRunAlertIdsInput] = useState("");
  const [lastRunResult, setLastRunResult] = useState<any | null>(null);
  const [expandedWorkloadId, setExpandedWorkloadId] = useState("");
  const [pendingWorkloadAction, setPendingWorkloadAction] = useState<{ workloadId: string; action: "restart" | "close" } | null>(null);
  const [operationsTraceDrawer, setOperationsTraceDrawer] = useState<OperationsTraceDrawerState | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [builderPane, setBuilderPane] = useState<AgentBuilderPane>("list");
  const [builderEditorOpen, setBuilderEditorOpen] = useState(false);
  const [pendingDeleteAgent, setPendingDeleteAgent] = useState<{ id: string; name: string } | null>(null);

  const {
    data: operationsDrawerEntityTrace,
    isLoading: operationsDrawerEntityTraceLoading,
    isFetching: operationsDrawerEntityTraceFetching,
    dataUpdatedAt: operationsDrawerEntityTraceUpdatedAt,
    refetch: refetchOperationsDrawerEntityTrace,
  } = useAIAgentEntityTrace(
    operationsTraceDrawer?.entityType || "",
    operationsTraceDrawer?.entityId || "",
    currentTenantId,
    12,
    {
      enabled: Boolean(operationsTraceDrawer?.entityType && operationsTraceDrawer?.entityId),
      refetchInterval: operationsTraceDrawer ? OPERATIONS_TRACE_DRAWER_REFRESH_MS : false,
    },
  );

  const connectorOptions = useMemo(
    () =>
      (Array.isArray(outboundConnectors) ? outboundConnectors : [])
        .map(normalizeConnectorOption)
        .filter((item): item is ConnectorOption => Boolean(item))
        .sort((left, right) => left.name.localeCompare(right.name)),
    [outboundConnectors],
  );

  useEffect(() => {
    setTriadPage(1);
  }, [triadAgentFilter]);

  const assigneeOptions = useMemo(
    () =>
      (Array.isArray(tenantUsers) ? tenantUsers : [])
        .map((user: any) => ({
          id: String(user?.id || "").trim(),
          label: String(user?.name || user?.username || user?.email || user?.id || "User").trim(),
          meta: [String(user?.role || "").trim(), String(user?.email || "").trim()].filter(Boolean).join(" • "),
        }))
        .filter((item) => item.id)
        .sort((left, right) => left.label.localeCompare(right.label)),
    [tenantUsers],
  );

  useEffect(() => {
    if (!Array.isArray(agents) || agents.length === 0) {
      setSelectedAgentId("");
      return;
    }
    if (createMode) {
      return;
    }
    if (!selectedAgentId) {
      setSelectedAgentId(String(agents[0]?.id || ""));
    }
  }, [agents, createMode, selectedAgentId]);

  const selectedAgent = useMemo(
    () => agents.find((item: any) => item.id === selectedAgentId) || null,
    [agents, selectedAgentId],
  );

  const { data: agentRuns = [], isLoading: runsLoading } = useAIAgentRuns(selectedAgentId, currentTenantId, 40);
  const runTotalPages = agentRuns.length > 0 ? Math.ceil(agentRuns.length / RUN_HISTORY_PAGE_SIZE) : 1;
  const runCurrentPage = Math.min(Math.max(runPage, 1), runTotalPages);
  const runPageStart = (runCurrentPage - 1) * RUN_HISTORY_PAGE_SIZE;
  const runPageEnd = runPageStart + RUN_HISTORY_PAGE_SIZE;
  const visibleAgentRuns = agentRuns.slice(runPageStart, runPageEnd);
  useEffect(() => {
    setRunPage(1);
  }, [selectedAgentId]);


  useEffect(() => {
    if (!selectedAgent) {
      return;
    }
    setCreateMode(false);
    setDraft(toAgentDraft(selectedAgent));
  }, [selectedAgentId, selectedAgent]);

  useEffect(() => {
    const processingItems = Array.isArray(operationsOverview?.processing_events) ? operationsOverview.processing_events : [];
    if (!expandedWorkloadId) {
      return;
    }
    const exists = processingItems.some((item: any) => String(item?.id || "").trim() === expandedWorkloadId);
    if (!exists) {
      setExpandedWorkloadId("");
    }
  }, [operationsOverview?.processing_events, expandedWorkloadId]);

  const triadEntries = useMemo(() => {
    const entries = readTriadCaseEntries(Array.isArray(allRunsFeed) ? allRunsFeed : []);
    if (triadAgentFilter !== "all") {
      return entries.filter((item) => item.agentId === triadAgentFilter);
    }
    return entries;
  }, [allRunsFeed, triadAgentFilter]);
  const triadTotalPages = triadEntries.length > 0 ? Math.ceil(triadEntries.length / TRIAD_PAGE_SIZE) : 1;
  const triadCurrentPage = Math.min(Math.max(triadPage, 1), triadTotalPages);
  const triadPageStart = (triadCurrentPage - 1) * TRIAD_PAGE_SIZE;
  const triadPageEnd = triadPageStart + TRIAD_PAGE_SIZE;
  const triadVisibleEntries = triadEntries.slice(triadPageStart, triadPageEnd);
  const triadBlockedCount = triadEntries.filter((item) => !item.autoActionsAllowed).length;
  const triadHumanReviewCount = triadEntries.filter((item) => item.requiresHumanReview).length;
  const triadConsensusCount = triadEntries.filter((item) => item.reviewerConsensus).length;
  const triadConsensusRate = triadEntries.length > 0 ? Math.round((triadConsensusCount / triadEntries.length) * 100) : 0;

  const operationsDrawerEvent = useMemo(() => {
    if (!operationsTraceDrawer) {
      return null;
    }
    const processingItems = Array.isArray(operationsOverview?.processing_events) ? operationsOverview.processing_events : [];
    const failedItems = Array.isArray(operationsOverview?.recent_failed_events) ? operationsOverview.recent_failed_events : [];
    const collections = operationsTraceDrawer.source === "failed"
      ? [failedItems, processingItems]
      : [processingItems, failedItems];
    for (const collection of collections) {
      const matchedById = collection.find((item: any) => String(item?.id || "").trim() === operationsTraceDrawer.eventId);
      if (matchedById) {
        return matchedById;
      }
      const matchedByEntity = collection.find(
        (item: any) =>
          normalizeEntityType(item?.entity_type) === operationsTraceDrawer.entityType &&
          String(item?.entity_id || "").trim() === operationsTraceDrawer.entityId,
      );
      if (matchedByEntity) {
        return matchedByEntity;
      }
    }
    return operationsTraceDrawer.snapshot || null;
  }, [operationsOverview?.processing_events, operationsOverview?.recent_failed_events, operationsTraceDrawer]);

  const operationsDrawerSnapshotTrace = useMemo(
    () => (operationsDrawerEvent ? buildEventTracePayload(operationsDrawerEvent) : buildEventTracePayload(operationsTraceDrawer?.snapshot)),
    [operationsDrawerEvent, operationsTraceDrawer],
  );
  const operationsDrawerResolvedTrace = useMemo(() => {
    const fetchedEvents = Array.isArray(operationsDrawerEntityTrace?.events) ? operationsDrawerEntityTrace.events : [];
    if (fetchedEvents.length > 0) {
      return operationsDrawerEntityTrace;
    }
    return operationsDrawerSnapshotTrace;
  }, [operationsDrawerEntityTrace, operationsDrawerSnapshotTrace]);
  const operationsDrawerFocusedWorkload = useMemo(
    () => resolveOperationsTraceWorkload(operationsDrawerResolvedTrace, operationsTraceDrawer?.focusWorkloadId || ""),
    [operationsDrawerResolvedTrace, operationsTraceDrawer?.focusWorkloadId],
  );
  const operationsDrawerEventLog = useMemo(
    () => buildOperationsDrawerEventLog(operationsDrawerResolvedTrace, operationsTraceDrawer?.focusWorkloadId || ""),
    [operationsDrawerResolvedTrace, operationsTraceDrawer?.focusWorkloadId],
  );
  const operationsDrawerLastUpdatedLabel = operationsDrawerEntityTraceUpdatedAt
    ? new Date(operationsDrawerEntityTraceUpdatedAt).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })
    : "";
  const operationsDrawerIsRefreshing = operationsOverviewFetching || operationsDrawerEntityTraceFetching;
  const operationsDrawerEntityLocation = operationsTraceDrawer
    ? buildEntityTraceLocation(
        currentTenantSlug,
        operationsTraceDrawer.entityType,
        operationsTraceDrawer.entityId,
        operationsTraceDrawer.focusWorkloadId,
        operationsTraceDrawer.focusStageKey,
      )
    : "";

  const isSaving = createAgent.isPending || updateAgent.isPending;
  const isRunning = runAgent.isPending;

  const handleToggleConnector = (field: "enrichmentConnectorIds" | "notificationConnectorIds", connectorID: string, checked: boolean) => {
    setDraft((prev) => {
      const source = field === "enrichmentConnectorIds" ? prev.enrichmentConnectorIds : prev.notificationConnectorIds;
      if (checked) {
        if (source.includes(connectorID)) {
          return prev;
        }
        const updated = [...source, connectorID];
        return field === "enrichmentConnectorIds"
          ? { ...prev, enrichmentConnectorIds: updated }
          : { ...prev, notificationConnectorIds: updated };
      }
      const updated = source.filter((id) => id !== connectorID);
      return field === "enrichmentConnectorIds"
        ? { ...prev, enrichmentConnectorIds: updated }
        : { ...prev, notificationConnectorIds: updated };
    });
  };

  const handleToggleTargetType = (targetType: "case" | "alert", checked: boolean) => {
    setDraft((prev) => {
      const source = prev.targetTypes.map((item) => String(item || "").trim().toLowerCase());
      if (checked) {
        if (source.includes(targetType)) {
          return prev;
        }
        return { ...prev, targetTypes: [...source, targetType] };
      }
      const next = source.filter((item) => item !== targetType);
      return { ...prev, targetTypes: next };
    });
  };

  const handleAddStage = () => {
    setDraft((prev) => ({
      ...prev,
      stages: [
        ...prev.stages,
        {
          id: `stage-${Date.now()}`,
          name: "",
          description: "",
          prompt: "",
          caseTagsInput: "",
          enrichmentConnectorIds: [],
        },
      ],
    }));
  };

  const handleUpdateStage = (index: number, patch: Partial<InvestigationStageDraft>) => {
    setDraft((prev) => ({
      ...prev,
      stages: prev.stages.map((stage, stageIndex) => (stageIndex === index ? { ...stage, ...patch } : stage)),
    }));
  };

  const handleRemoveStage = (index: number) => {
    setDraft((prev) => ({
      ...prev,
      stages: prev.stages.filter((_, stageIndex) => stageIndex !== index),
    }));
  };

  const handleToggleStageConnector = (stageIndex: number, connectorID: string, checked: boolean) => {
    setDraft((prev) => ({
      ...prev,
      stages: prev.stages.map((stage, index) => {
        if (index !== stageIndex) {
          return stage;
        }
        if (checked) {
          if (stage.enrichmentConnectorIds.includes(connectorID)) {
            return stage;
          }
          return { ...stage, enrichmentConnectorIds: [...stage.enrichmentConnectorIds, connectorID] };
        }
        return {
          ...stage,
          enrichmentConnectorIds: stage.enrichmentConnectorIds.filter((id) => id !== connectorID),
        };
      }),
    }));
  };

  const handleCreateNew = () => {
    setActiveTab("builder");
    setCreateMode(true);
    setSelectedAgentId("");
    setDraft(defaultAgentDraft());
    setBuilderEditorOpen(true);
    setBuilderPane("editor");
  };

  const openAgentEditor = (agentId: string) => {
    setActiveTab("builder");
    setSelectedAgentId(agentId);
    setCreateMode(false);
    setBuilderEditorOpen(true);
    setBuilderPane("editor");
  };

  const closeAgentEditor = () => {
    setBuilderEditorOpen(false);
    setBuilderPane("list");
    setCreateMode(false);
    if (!selectedAgentId) {
      setDraft(defaultAgentDraft());
    }
  };

  const handleSaveAgent = () => {
    const payload = buildAgentPayload(draft);
    if (!payload.name) {
      toast.error("Agent name is required");
      return;
    }
    if (!Array.isArray(payload.targetTypes) || payload.targetTypes.length === 0) {
      toast.error("Select at least one target type");
      return;
    }

    if (draft.id) {
      updateAgent.mutate(
        { id: draft.id, data: payload },
        {
          onSuccess: () => {
            setCreateMode(false);
            toast.success("AI agent updated");
          },
          onError: (error: any) => {
            toast.error(String(error?.message || "Failed to update AI agent"));
          },
        },
      );
      return;
    }

    createAgent.mutate(payload, {
      onSuccess: (created: any) => {
        setCreateMode(false);
        toast.success("AI agent created");
        const createdID = String(created?.id || "").trim();
        if (createdID) {
          setSelectedAgentId(createdID);
        }
      },
      onError: (error: any) => {
        toast.error(String(error?.message || "Failed to create AI agent"));
      },
    });
  };

  const handleDeleteAgent = (agent: any) => {
    const agentID = String(agent?.id || "").trim();
    if (!agentID) return;
    const agentName = String(agent?.name || "AI agent").trim();
    setPendingDeleteAgent({ id: agentID, name: agentName || "AI agent" });
    setDeleteDialogOpen(true);
  };

  const confirmDeleteAgent = () => {
    const agentID = pendingDeleteAgent?.id ? String(pendingDeleteAgent.id).trim() : "";
    if (!agentID) {
      setDeleteDialogOpen(false);
      setPendingDeleteAgent(null);
      return;
    }
    deleteAgent.mutate(agentID, {
      onSuccess: () => {
        toast.success("AI agent deleted");
        if (selectedAgentId === agentID) {
          setSelectedAgentId("");
          setCreateMode(false);
          setDraft(defaultAgentDraft());
        }
        setDeleteDialogOpen(false);
        setPendingDeleteAgent(null);
      },
      onError: (error: any) => {
        toast.error(String(error?.message || "Failed to delete AI agent"));
        setDeleteDialogOpen(false);
        setPendingDeleteAgent(null);
      },
    });
  };

  const handleRunAgent = (agentID: string, dryRun: boolean, customLimit?: number) => {
    const payload: Record<string, any> = { dry_run: dryRun };
    if (Number.isFinite(customLimit) && Number(customLimit) > 0) {
      payload.max_cases = Math.max(1, Math.min(100, Math.round(Number(customLimit))));
    }
    const caseIDs = parseIdentifierList(runCaseIdsInput);
    const alertIDs = parseIdentifierList(runAlertIdsInput);
    if (caseIDs.length > 0) {
      payload.case_ids = caseIDs;
    }
    if (alertIDs.length > 0) {
      payload.alert_ids = alertIDs;
    }
    runAgent.mutate(
      { agentId: agentID, payload },
      {
        onSuccess: (result: any) => {
          setLastRunResult(result);
          setCreateMode(false);
          setSelectedAgentId(agentID);
          const processed = Number(result?.processed_cases ?? result?.processedCases ?? 0);
          toast.success(`Run finished: ${processed} case(s) processed`);
        },
        onError: (error: any) => {
          toast.error(String(error?.message || "AI agent run failed"));
        },
      },
    );
  };

  const handleWorkloadAction = (workloadIdRaw: string, action: "restart" | "close") => {
    const workloadId = String(workloadIdRaw || "").trim();
    if (!workloadId) {
      return;
    }
    setPendingWorkloadAction({ workloadId, action });
    const mutation = action === "restart" ? restartWorkload : closeWorkload;
    mutation.mutate(
      { workloadId },
      {
        onSuccess: () => {
          toast.success(action === "restart" ? "Workload restarted" : "Workload closed");
        },
        onError: (error: any) => {
          toast.error(String(error?.message || (action === "restart" ? "Failed to restart workload" : "Failed to close workload")));
        },
        onSettled: () => {
          setPendingWorkloadAction((current) => {
            if (!current) return null;
            if (current.workloadId !== workloadId || current.action !== action) return current;
            return null;
          });
        },
      },
    );
  };

  const resolveWorkloadStageKey = (workload: any, runResult?: any): string => {
    const normalizedLastStage = String(workload?.last_stage || workload?.lastStage || "").trim();
    if (normalizedLastStage) {
      return normalizedLastStage;
    }
    const stageTimeline = Array.isArray(runResult?.stage_timeline)
      ? runResult.stage_timeline
      : Array.isArray(runResult?.stageTimeline)
        ? runResult.stageTimeline
        : [];
    const lastStage = stageTimeline.length > 0 ? stageTimeline[stageTimeline.length - 1] : null;
    return String(lastStage?.id || lastStage?.name || "").trim();
  };

  const openEntityTrace = (entityType: any, entityId: any, workloadId?: any, stageKey?: any) => {
    const nextLocation = buildEntityTraceLocation(currentTenantSlug, entityType, entityId, workloadId, stageKey);
    if (!nextLocation) {
      return;
    }
    setLocation(nextLocation);
  };

  const openOperationsTraceDrawer = (
    event: any,
    source: "processing" | "failed",
    options?: { focusWorkloadId?: any; focusStageKey?: any; title?: string; subtitle?: string },
  ) => {
    const eventId = String(event?.id || "").trim();
    const entityType = normalizeEntityType(event?.entity_type);
    const entityId = String(event?.entity_id || "").trim();
    if (!eventId && !entityId) {
      return;
    }
    setOperationsTraceDrawer({
      source,
      eventId,
      entityType,
      entityId,
      title: String(options?.title || buildOperationsTraceTitle(event)).trim() || "AI workload trace",
      subtitle: String(options?.subtitle || buildOperationsTraceSubtitle(event)).trim(),
      focusWorkloadId: String(options?.focusWorkloadId || "").trim(),
      focusStageKey: String(options?.focusStageKey || "").trim(),
      snapshot: event,
    });
  };

  const renderOperationsTraceActions = (context: AIEntityTraceActionContext) => {
    const traceWorkloadId = String(context.workload?.id || "").trim();
    const entityType = context.event?.entity_type;
    const entityId = context.event?.entity_id;
    const stageKey = resolveWorkloadStageKey(context.workload, context.runResult);
    const entityLocation = buildEntityTraceLocation(currentTenantSlug, entityType, entityId, traceWorkloadId);
    const stageLocation = buildEntityTraceLocation(currentTenantSlug, entityType, entityId, traceWorkloadId, stageKey);
    return (
      <>
        {entityLocation ? (
          <Button
            type="button"
            size="sm"
            className={OUTLINE_BUTTON_CLASS}
            data-testid={`ops-ai-trace-open-entity-${traceWorkloadId}`}
            onClick={() => openEntityTrace(entityType, entityId, traceWorkloadId)}
          >
            <ArrowUpRight size={14} className="mr-1.5" />
            {buildEntityOpenLabel(entityType)}
          </Button>
        ) : null}
        {stageLocation && stageKey ? (
          <Button
            type="button"
            size="sm"
            className={OUTLINE_BUTTON_CLASS}
            data-testid={`ops-ai-trace-open-stage-${traceWorkloadId}`}
            onClick={() => openEntityTrace(entityType, entityId, traceWorkloadId, stageKey)}
          >
            <ArrowUpRight size={14} className="mr-1.5" />
            Open stage
          </Button>
        ) : null}
      </>
    );
  };

  const selectedRunLimit = positiveInt(runLimitInput, positiveInt(draft.maxCasesPerRun, 10));
  const lastRunCases = Array.isArray(lastRunResult?.results) ? lastRunResult.results : [];
  const lastRunErrors = lastRunCases.filter((item: any) => String(item?.error || "").trim()).length;
  const isAIAgentsPageLoading = agentsLoading || connectorsLoading;
  const showAIAgentsPageSkeleton = useMinimumLoading(isAIAgentsPageLoading);
  const showAgentRunsSkeleton = useMinimumLoading(Boolean(selectedAgent) && runsLoading);
  const showAgentsListSkeleton = useMinimumLoading(agentsLoading);
  const showTriadSkeleton = useMinimumLoading(runsFeedLoading);
  const showOperationsSkeleton = useMinimumLoading(operationsLoading);

  if (showAIAgentsPageSkeleton) {
    return (
      <AppLayout>
        <div className={PAGE_SHELL_CLASS}>
          <div className="space-y-2">
            <Skeleton className="h-8 w-44 rounded-lg" />
            <Skeleton className="h-4 w-[540px] max-w-full rounded-lg" />
          </div>
          <div className="grid gap-5 xl:grid-cols-[minmax(0,1.15fr)_minmax(0,0.85fr)]">
            <Skeleton className="h-[720px] w-full rounded-2xl" />
            <div className="space-y-5">
              <Skeleton className="h-[340px] w-full rounded-2xl" />
              <Skeleton className="h-[220px] w-full rounded-2xl" />
            </div>
          </div>
          <Skeleton className="h-[240px] w-full rounded-2xl" />
          <Skeleton className="h-[300px] w-full rounded-2xl" />
        </div>
      </AppLayout>
    );
  }

  return (
    <AppLayout>
      <div className={PAGE_SHELL_CLASS}>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h1 className="text-[30px] font-semibold tracking-[-0.4px] text-white">AI Agents</h1>
            <p className={`mt-1 ${MUTED_TEXT_CLASS}`}>
              Create autonomous agents that triage tagged cases, enrich context through connectors, and lead response actions.
            </p>
          </div>
          <Button
            className={PRIMARY_BUTTON_CLASS}
            onClick={handleCreateNew}
            data-testid="button-new-ai-agent"
          >
            <Plus size={16} className="mr-2" />
            New Agent
          </Button>
        </div>

        <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as AIAgentsTab)} className="space-y-5">
          <TabsList className={AI_AGENT_TAB_LIST_CLASS}>
            <TabsTrigger value="builder" className={AI_AGENT_TAB_TRIGGER_CLASS} data-testid="tab-ai-builder">
              <Bot size={14} className="mr-1.5" />
              Builder
            </TabsTrigger>
            <TabsTrigger value="triad" className={AI_AGENT_TAB_TRIGGER_CLASS} data-testid="tab-ai-triad">
              <BarChart3 size={14} className="mr-1.5" />
              Triad Analytics
            </TabsTrigger>
            <TabsTrigger value="operations" className={AI_AGENT_TAB_TRIGGER_CLASS} data-testid="tab-ai-operations">
              <Activity size={14} className="mr-1.5" />
              Operations
            </TabsTrigger>
          </TabsList>

          <TabsContent value="builder" className="mt-0 space-y-6">
            <Tabs value={builderPane} onValueChange={(value) => setBuilderPane(value as AgentBuilderPane)} className="space-y-6">
              {builderEditorOpen ? (
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <TabsList className="h-auto rounded-2xl border border-[#2a2c3c] bg-[#0f131d] p-1.5">
                    <TabsTrigger value="list" className="rounded-xl px-4 py-2 text-sm data-[state=active]:bg-[#171b2a] data-[state=active]:text-white">
                      Saved agents
                    </TabsTrigger>
                    <TabsTrigger value="editor" className="max-w-[48vw] rounded-xl px-4 py-2 text-sm data-[state=active]:bg-[#171b2a] data-[state=active]:text-white">
                      <span className="truncate">{draft.id ? (draft.name.trim() || "Edit agent") : "New agent"}</span>
                    </TabsTrigger>
                  </TabsList>
                  <Button
                    type="button"
                    size="icon"
                    variant="outline"
                    className="h-10 w-10 rounded-2xl border-[#2a2c3c] bg-[#0f131d] text-[#9ca3af] hover:bg-[#171b2a] hover:text-white"
                    onClick={closeAgentEditor}
                    aria-label="Close"
                    title="Close"
                    data-testid="button-close-ai-agent-editor"
                  >
                    <X size={16} />
                  </Button>
                </div>
              ) : null}

              <TabsContent value="list" className="m-0 space-y-6">
                <div className="space-y-5">
                  <Card className={`${PANEL_CLASS} p-5`}>
                    <div className="mb-4 flex items-start justify-between gap-2">
                      <div>
                        <h2 className="text-sm font-semibold text-white">Configured Agents</h2>
                            <p className={MUTED_TEXT_CLASS}>Select, run, and monitor autonomous SOC workers.</p>
                          </div>
                          <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#e5e7eb]">{agents.length}</Badge>
                        </div>

                        <div className="space-y-2">
                          {showAgentsListSkeleton ? (
                            <div className="space-y-2">
                              <Skeleton className="h-16 w-full rounded-xl" />
                              <Skeleton className="h-16 w-full rounded-xl" />
                              <Skeleton className="h-16 w-full rounded-xl" />
                            </div>
                          ) : null}

                          {!showAgentsListSkeleton && agents.length === 0 ? (
                            <div className={`${SUBPANEL_CLASS} p-4 text-sm text-[#d1d5db]`}>
                              No agents configured yet. Create your first AI agent to automate tagged case response.
                            </div>
                          ) : null}

                          {agents.map((agent: any) => {
                            const isSelected = selectedAgentId === agent.id;
                            const caseTags = Array.isArray(agent.caseTags) ? agent.caseTags : [];
                            const targetTypes = Array.isArray(agent.targetTypes) ? agent.targetTypes : ["case"];
                            return (
                              <div
                                key={agent.id}
                                className={`rounded-xl border px-4 py-3 transition-colors ${
                                  isSelected ? "border-[rgba(102,255,76,0.45)] bg-[rgba(102,255,76,0.08)]" : "border-[#2a2c3c] bg-[#111622]"
                                }`}
                              >
                                <div className="mb-2 flex items-center justify-between gap-2">
                                  <button
                                    className="truncate text-left text-sm font-semibold text-white"
                                    onClick={() => openAgentEditor(String(agent.id))}
                                  >
                                    {agent.name || "AI Agent"}
                                  </button>
                                  <Badge
                                    className={
                                      agent.enabled
                                        ? "border border-[rgba(102,255,76,0.3)] bg-[rgba(102,255,76,0.16)] text-[#86efac]"
                                        : "border border-[rgba(156,163,175,0.3)] bg-[rgba(75,85,99,0.22)] text-[#d1d5db]"
                                    }
                                  >
                                    {agent.enabled ? "on" : "off"}
                                  </Badge>
                                </div>
                                {agent.description ? <p className="mb-2 text-xs text-[#9ca3af]">{agent.description}</p> : null}
                                <div className="mb-3 flex flex-wrap gap-1.5">
                                  {targetTypes.map((targetType: string) => (
                                    <Badge key={`${agent.id}-${targetType}`} className="border border-[rgba(59,130,246,0.35)] bg-[rgba(59,130,246,0.12)] text-[#93c5fd]">
                                      {targetType === "alert" ? "alert" : "case"}
                                    </Badge>
                                  ))}
                                  {agent?.triadEnabled ? (
                                    <Badge className="border border-[rgba(102,255,76,0.35)] bg-[rgba(102,255,76,0.14)] text-[#86efac]">
                                      triad{agent?.triadCriticalOnly ? " (critical)" : ""}
                                    </Badge>
                                  ) : null}
                                  {caseTags.length === 0 ? (
                                    <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#9ca3af]">no tags</Badge>
                                  ) : (
                                    caseTags.slice(0, 4).map((tag: string) => (
                                      <Badge key={`${agent.id}-${tag}`} className="border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db]">
                                        <Tag size={11} className="mr-1" />
                                        {tag}
                                      </Badge>
                                    ))
                                  )}
                                </div>
                                <div className="grid grid-cols-2 gap-2">
                                  <Button
                                    variant="outline"
                                    className={OUTLINE_BUTTON_CLASS}
                                    onClick={() => openAgentEditor(String(agent.id))}
                                  >
                                    Edit
                                  </Button>
                                  <Button
                                    className={PRIMARY_BUTTON_CLASS}
                                    disabled={isRunning || !agent.enabled}
                                    onClick={() => handleRunAgent(String(agent.id), false)}
                                    data-testid={`button-run-agent-${agent.id}`}
                                  >
                                    {isRunning && selectedAgentId === agent.id ? (
                                      <Loader2 size={14} className="mr-1 animate-spin" />
                                    ) : (
                                      <Play size={14} className="mr-1" />
                                    )}
                                    Run
                                  </Button>
                                </div>
                              </div>
                            );
                          })}
                        </div>
                      </Card>

                      <Card className={`${PANEL_CLASS} p-5`}>
                        <div className="mb-3 flex items-center gap-2">
                          <WandSparkles size={16} className="text-[#9ca3af]" />
                          <h2 className="text-sm font-semibold text-white">Manual Run</h2>
                        </div>
                        <div className="space-y-3">
                          <div className="space-y-2">
                            <Label htmlFor="ai-agent-run-limit">Case limit for this run</Label>
                            <Input
                              id="ai-agent-run-limit"
                              className={INPUT_CLASS}
                              value={runLimitInput}
                              onChange={(event) => setRunLimitInput(event.target.value)}
                              placeholder={draft.maxCasesPerRun || "10"}
                              disabled={!selectedAgentId}
                            />
                          </div>
                          <div className="grid gap-3 md:grid-cols-2">
                            <div className="space-y-2">
                              <Label htmlFor="ai-agent-run-case-ids">Case IDs (optional)</Label>
                              <Textarea
                                id="ai-agent-run-case-ids"
                                className={TEXTAREA_CLASS}
                                value={runCaseIdsInput}
                                onChange={(event) => setRunCaseIdsInput(event.target.value)}
                                placeholder="UUID, UUID"
                                rows={2}
                                disabled={!selectedAgentId}
                              />
                            </div>
                            <div className="space-y-2">
                              <Label htmlFor="ai-agent-run-alert-ids">Alert IDs (optional)</Label>
                              <Textarea
                                id="ai-agent-run-alert-ids"
                                className={TEXTAREA_CLASS}
                                value={runAlertIdsInput}
                                onChange={(event) => setRunAlertIdsInput(event.target.value)}
                                placeholder="UUID, UUID"
                                rows={2}
                                disabled={!selectedAgentId}
                              />
                            </div>
                          </div>
                          <div className="flex items-center justify-between rounded-lg border border-[#2a2c3c] bg-[#0f131d] px-3 py-2">
                            <div>
                              <p className="text-sm font-medium text-white">Dry run</p>
                              <p className={MUTED_TEXT_CLASS}>No tasks/comments persisted.</p>
                            </div>
                            <Switch checked={runDryMode} onCheckedChange={setRunDryMode} />
                          </div>
                          <Button
                            className={`w-full ${PRIMARY_BUTTON_CLASS}`}
                            disabled={!selectedAgentId || isRunning || !selectedAgent?.enabled}
                            onClick={() => handleRunAgent(selectedAgentId, runDryMode, selectedRunLimit)}
                            data-testid="button-run-selected-agent"
                          >
                            {isRunning ? <Loader2 size={16} className="mr-2 animate-spin" /> : <Sparkles size={16} className="mr-2" />}
                            Execute Agent
                          </Button>
                          {selectedAgent && !selectedAgent.enabled ? (
                            <p className={MUTED_TEXT_CLASS}>Enable the agent before running it.</p>
                          ) : null}
                        </div>
                      </Card>
                    </div>
                {lastRunResult ? (
                  <Card className={`${PANEL_CLASS} p-5`}>
                    <div className="mb-3 flex flex-wrap items-center gap-2">
                      <h2 className="text-sm font-semibold text-white">Last Execution Result</h2>
                      <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db]">
                        {Number(lastRunResult?.processed_cases ?? 0)} processed
                      </Badge>
                      <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db]">{lastRunErrors} errors</Badge>
                    </div>
                    <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-4">
                      {lastRunCases.slice(0, 8).map((item: any, index: number) => {
                        const hasError = Boolean(String(item?.error || "").trim());
                        const caseLabel = String(item?.case_number || item?.case_id || `case-${index + 1}`);
                        return (
                          <div key={`${caseLabel}-${index}`} className={`${SUBPANEL_CLASS} p-3`}>
                            <p className="text-sm font-semibold text-white">{caseLabel}</p>
                            <p className="mt-1 text-xs text-[#9ca3af]">{String(item?.title || "").trim() || "Untitled case"}</p>
                            <p className={`mt-2 text-xs ${hasError ? "text-[#fca5a5]" : "text-[#86efac]"}`}>
                              {hasError ? String(item?.error || "Execution failed") : "Completed"}
                            </p>
                          </div>
                        );
                      })}
                    </div>
                  </Card>
                ) : null}

                <Card className={`${PANEL_CLASS} p-5`}>
                  <div className="mb-4 flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <Clock3 size={16} className="text-[#9ca3af]" />
                      <h2 className="text-sm font-semibold text-white">
                        Run History{selectedAgent ? ` • ${selectedAgent.name}` : ""}
                      </h2>
                    </div>
                    {selectedAgent ? (
                      <Button
                        variant="outline"
                        className={OUTLINE_BUTTON_CLASS}
                        disabled={isRunning || !selectedAgent?.enabled}
                        onClick={() => handleRunAgent(String(selectedAgent.id), false)}
                      >
                        <Play size={14} className="mr-1" />
                        Run Again
                      </Button>
                    ) : null}
                  </div>

                  {!selectedAgent ? (
                    <div className={`${SUBPANEL_CLASS} p-4 text-sm text-[#d1d5db]`}>Select an agent to inspect execution history.</div>
                  ) : null}

                  {showAgentRunsSkeleton ? (
                    <div className="space-y-2">
                      <Skeleton className="h-16 w-full rounded-xl" />
                      <Skeleton className="h-16 w-full rounded-xl" />
                      <Skeleton className="h-16 w-full rounded-xl" />
                    </div>
                  ) : null}

                  {selectedAgent && !showAgentRunsSkeleton && agentRuns.length === 0 ? (
                    <div className={`${SUBPANEL_CLASS} p-4 text-sm text-[#d1d5db]`}>
                      No runs yet. Execute this agent to generate case triage history.
                    </div>
                  ) : null}

                  <div className="space-y-2">
                    {selectedAgent && !showAgentRunsSkeleton
                      ? visibleAgentRuns.map((run: any) => {
                          const results = Array.isArray(run?.results) ? run.results : [];
                          const failed = results.filter((item: any) => String(item?.error || "").trim()).length;
                          const processed = Number(run?.processedCases ?? run?.processed_cases ?? 0);
                          const successful = Number(run?.successfulCases ?? run?.successful_cases ?? 0);
                          return (
                            <details key={run.id || run.runId} className={`${SUBPANEL_CLASS} p-3`} open={false}>
                              <summary className="flex cursor-pointer list-none flex-wrap items-center justify-between gap-2">
                                <div className="flex flex-wrap items-center gap-2">
                                  <span className="text-sm font-semibold text-white">{formatDateTime(run.startedAt || run.started_at)}</span>
                                  <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db]">{processed} processed</Badge>
                                  <Badge className="border border-[rgba(34,197,94,0.3)] bg-[rgba(34,197,94,0.16)] text-[#86efac]">
                                    {successful} successful
                                  </Badge>
                                  <Badge className="border border-[rgba(239,68,68,0.3)] bg-[rgba(239,68,68,0.16)] text-[#fca5a5]">
                                    {failed} failed
                                  </Badge>
                                  {run.dryRun ? (
                                    <Badge className="border border-[rgba(251,191,36,0.3)] bg-[rgba(251,191,36,0.15)] text-[#fcd34d]">dry-run</Badge>
                                  ) : null}
                                </div>
                                <span className={`${MUTED_TEXT_CLASS}`}>{formatDateTime(run.finishedAt || run.finished_at)}</span>
                              </summary>
                              <div className="mt-3 space-y-2">
                                {results.length === 0 ? (
                                  <p className={MUTED_TEXT_CLASS}>No case-level details available.</p>
                                ) : (
                                  results.slice(0, 12).map((item: any, index: number) => {
                                    const hasError = Boolean(String(item?.error || "").trim());
                                    const caseLabel = String(item?.case_number || item?.case_id || `case-${index + 1}`);
                                    return (
                                      <div key={`${caseLabel}-${index}`} className="rounded-lg border border-[#2a2c3c] bg-[#0b0c10] px-3 py-2">
                                        <div className="flex items-center justify-between gap-2">
                                          <span className="text-sm font-medium text-white">{caseLabel}</span>
                                          {hasError ? (
                                            <span className="text-xs text-[#fca5a5]">{String(item?.error || "Error")}</span>
                                          ) : (
                                            <span className="inline-flex items-center text-xs text-[#86efac]">
                                              <CheckCircle2 size={12} className="mr-1" />
                                              completed
                                            </span>
                                          )}
                                        </div>
                                        {item?.summary ? <p className="mt-1 text-xs text-[#9ca3af]">{String(item.summary)}</p> : null}
                                      </div>
                                    );
                                  })
                                )}
                              </div>
                            </details>
                          );
                        })
                      : null}
                  </div>
                  {selectedAgent && !showAgentRunsSkeleton && agentRuns.length > RUN_HISTORY_PAGE_SIZE ? (
                    <div className="mt-3 flex items-center justify-between gap-3 text-xs text-[#9ca3af]">
                      <span>Showing {(runPageStart + 1)}–{Math.min(runPageEnd, agentRuns.length)} of {agentRuns.length} runs</span>
                      <div className="flex items-center gap-2">
                        <Button
                          type="button"
                          size="sm"
                          variant="outline"
                          className={`${OUTLINE_BUTTON_CLASS} h-7 px-2 text-[11px]`}
                          disabled={runCurrentPage <= 1}
                          onClick={() => setRunPage(Math.max(1, runCurrentPage - 1))}
                        >
                          Prev
                        </Button>
                        <span className="min-w-[80px] text-center">Page {runCurrentPage} of {runTotalPages}</span>
                        <Button
                          type="button"
                          size="sm"
                          variant="outline"
                          className={`${OUTLINE_BUTTON_CLASS} h-7 px-2 text-[11px]`}
                          disabled={runCurrentPage >= runTotalPages}
                          onClick={() => setRunPage(Math.min(runTotalPages, runCurrentPage + 1))}
                        >
                          Next
                        </Button>
                      </div>
                    </div>
                  ) : null}
                </Card>
              </TabsContent>

              <TabsContent value="editor" className="m-0">
                <div className="mx-auto w-full max-w-[980px] space-y-6 pb-12">
                  <div className="rounded-2xl border border-[#2a2c3c] bg-[#111622] p-6">
                    <p className="text-xs font-semibold uppercase tracking-[0.16em] text-[#66ff4c]">Agent editor</p>
                    <h2 className="mt-2 text-[26px] font-semibold tracking-[-0.04em] text-white">
                      {draft.id ? "Edit agent" : "New agent"}
                    </h2>
                    <p className="mt-2 text-sm text-[#8b91a3]">Define scope, enrichment, and execution behavior.</p>
                  </div>

                  <Card className={`${PANEL_CLASS} p-5`}>
                            <div className="mb-4 flex items-center justify-between">
                              <div className="flex items-center gap-2">
                                <div className="flex h-9 w-9 items-center justify-center rounded-lg border border-[rgba(102,255,76,0.28)] bg-[rgba(102,255,76,0.12)] text-[#66ff4c]">
                                  <Bot size={18} />
                                </div>
                                <div>
                                  <h2 className="text-sm font-semibold text-white">{draft.id ? "Edit Agent" : "Create Agent"}</h2>
                                  <p className={MUTED_TEXT_CLASS}>Define scope, enrichment, and execution behavior.</p>
                                </div>
                              </div>
                              <Badge className="border border-[rgba(102,255,76,0.26)] bg-[rgba(102,255,76,0.14)] text-[#86efac]">
                                {draft.enabled ? "Enabled" : "Disabled"}
                              </Badge>
                            </div>

                            <div className="space-y-4">
                              <div className="grid gap-4 md:grid-cols-2">
                                <div className="space-y-2">
                                  <Label htmlFor="ai-agent-name">Agent Name</Label>
                                  <Input
                                    id="ai-agent-name"
                                    className={INPUT_CLASS}
                                    value={draft.name}
                                    onChange={(event) => setDraft((prev) => ({ ...prev, name: event.target.value }))}
                                    placeholder="e.g. Ransomware Response Agent"
                                  />
                                </div>
                                <div className="space-y-2">
                                  <Label htmlFor="ai-agent-language">Response Language</Label>
                                  <Input
                                    id="ai-agent-language"
                                    className={INPUT_CLASS}
                                    value={draft.language}
                                    onChange={(event) => setDraft((prev) => ({ ...prev, language: event.target.value }))}
                                    placeholder="auto / en / ru"
                                  />
                                </div>
                              </div>

                              <div className="space-y-2">
                                <Label htmlFor="ai-agent-description">Description</Label>
                                <Input
                                  id="ai-agent-description"
                                  className={INPUT_CLASS}
                                  value={draft.description}
                                  onChange={(event) => setDraft((prev) => ({ ...prev, description: event.target.value }))}
                                  placeholder="What this agent is responsible for"
                                />
                              </div>

                              <div className={`${SUBPANEL_CLASS} space-y-3 p-4`}>
                                <div className="flex items-center gap-2">
                                  <Target size={15} className="text-[#9ca3af]" />
                                  <p className="text-sm font-semibold text-white">Target Scope</p>
                                </div>
                                <div className="grid gap-2 md:grid-cols-2">
                                  <label className="flex items-center gap-3 rounded-lg border border-[#2a2c3c] bg-[#0b0c10] px-3 py-2 text-sm text-[#e5e7eb]">
                                    <Checkbox
                                      checked={draft.targetTypes.includes("case")}
                                      onCheckedChange={(checked) => handleToggleTargetType("case", checked === true)}
                                    />
                                    <span>Cases</span>
                                  </label>
                                  <label className="flex items-center gap-3 rounded-lg border border-[#2a2c3c] bg-[#0b0c10] px-3 py-2 text-sm text-[#e5e7eb]">
                                    <Checkbox
                                      checked={draft.targetTypes.includes("alert")}
                                      onCheckedChange={(checked) => handleToggleTargetType("alert", checked === true)}
                                    />
                                    <span>Alerts</span>
                                  </label>
                                </div>
                                <p className={MUTED_TEXT_CLASS}>Choose where this agent auto-triggers from queue events.</p>
                              </div>

                              {draft.targetTypes.includes("case") ? (
                                <div className="space-y-2">
                                  <Label htmlFor="ai-agent-tags">Case Filter Tags (optional)</Label>
                                  <Input
                                    id="ai-agent-tags"
                                    className={INPUT_CLASS}
                                    value={draft.tagsInput}
                                    onChange={(event) => setDraft((prev) => ({ ...prev, tagsInput: event.target.value }))}
                                    placeholder="malware, phishing, exfiltration"
                                  />
                                  <p className={MUTED_TEXT_CLASS}>If empty, the agent can process all non-closed cases.</p>
                                </div>
                              ) : null}

                              {draft.targetTypes.includes("alert") ? (
                                <div className="space-y-3">
                                  <div className="space-y-2">
                                    <Label htmlFor="ai-agent-alert-sources">Alert Sources (optional)</Label>
                                    <Input
                                      id="ai-agent-alert-sources"
                                      className={INPUT_CLASS}
                                      value={draft.alertSourcesInput}
                                      onChange={(event) => setDraft((prev) => ({ ...prev, alertSourcesInput: event.target.value }))}
                                      placeholder="edr, siem, email_gateway"
                                    />
                                    <p className={MUTED_TEXT_CLASS}>If empty, agent accepts alerts from any source.</p>
                                  </div>
                                  <div className="flex items-center justify-between rounded-lg border border-[#2a2c3c] bg-[#0f131d] px-3 py-2">
                                    <div>
                                      <p className="text-sm font-medium text-white">Auto-create case from alert</p>
                                      <p className={MUTED_TEXT_CLASS}>Create and link a case when alert is not attached to one.</p>
                                    </div>
                                    <Switch
                                      checked={draft.autoCreateCaseFromAlert}
                                      onCheckedChange={(value) => setDraft((prev) => ({ ...prev, autoCreateCaseFromAlert: value }))}
                                    />
                                  </div>
                                </div>
                              ) : null}

                              <div className="space-y-2">
                                <Label htmlFor="ai-agent-auto-tags">Case Tags To Apply (optional)</Label>
                                <Input
                                  id="ai-agent-auto-tags"
                                  className={INPUT_CLASS}
                                  value={draft.autoCaseTagsInput}
                                  onChange={(event) => setDraft((prev) => ({ ...prev, autoCaseTagsInput: event.target.value }))}
                                  placeholder="ai-reviewed, auto-triaged"
                                />
                                <p className={MUTED_TEXT_CLASS}>Applied to case metadata after successful run.</p>
                              </div>

                              <div className="grid gap-4 md:grid-cols-2">
                                <div className="space-y-2">
                                  <Label htmlFor="ai-agent-max-cases">Max Cases per Run</Label>
                                  <Input
                                    id="ai-agent-max-cases"
                                    className={INPUT_CLASS}
                                    value={draft.maxCasesPerRun}
                                    onChange={(event) => setDraft((prev) => ({ ...prev, maxCasesPerRun: event.target.value }))}
                                    placeholder="10"
                                  />
                                </div>
                                <div className="space-y-2">
                                  <Label htmlFor="ai-agent-assignee">Task Assignee (optional)</Label>
                                  <Select
                                    value={draft.taskAssigneeId || "none"}
                                    onValueChange={(value) => setDraft((prev) => ({ ...prev, taskAssigneeId: value === "none" ? "" : value }))}
                                  >
                                    <SelectTrigger id="ai-agent-assignee" className={SELECT_TRIGGER_CLASS}>
                                      <SelectValue placeholder="Select analyst" />
                                    </SelectTrigger>
                                    <SelectContent className={SELECT_CONTENT_CLASS}>
                                      <SelectItem value="none">No default assignee</SelectItem>
                                      {assigneeOptions.map((user) => (
                                        <SelectItem key={user.id} value={user.id}>
                                          {user.label}{user.meta ? ` - ${user.meta}` : ""}
                                        </SelectItem>
                                      ))}
                                    </SelectContent>
                                  </Select>
                                  <p className={MUTED_TEXT_CLASS}>Used when agent auto-creates follow-up tasks.</p>
                                </div>
                                <div className="space-y-2">
                                  <Label htmlFor="ai-agent-execution-policy">Execution Policy</Label>
                                  <Select
                                    value={draft.executionPolicy || "all_matching"}
                                    onValueChange={(value) => setDraft((prev) => ({ ...prev, executionPolicy: value }))}
                                  >
                                    <SelectTrigger id="ai-agent-execution-policy" className={SELECT_TRIGGER_CLASS}>
                                      <SelectValue placeholder="Select policy" />
                                    </SelectTrigger>
                                    <SelectContent className={SELECT_CONTENT_CLASS}>
                                      <SelectItem value="all_matching">All matching</SelectItem>
                                      <SelectItem value="exclusive">Exclusive</SelectItem>
                                      <SelectItem value="first_match">First match</SelectItem>
                                      <SelectItem value="fallback_chain">Fallback chain</SelectItem>
                                    </SelectContent>
                                  </Select>
                                  <p className={MUTED_TEXT_CLASS}>Controls whether all agents run, only one wins, or fallbacks execute sequentially.</p>
                                </div>
                                <div className="space-y-2">
                                  <Label htmlFor="ai-agent-execution-priority">Execution Priority</Label>
                                  <Input
                                    id="ai-agent-execution-priority"
                                    className={INPUT_CLASS}
                                    value={draft.executionPriority}
                                    onChange={(event) => setDraft((prev) => ({ ...prev, executionPriority: event.target.value }))}
                                    placeholder="0"
                                  />
                                  <p className={MUTED_TEXT_CLASS}>Higher priority wins for exclusive / first-match and orders fallback chains.</p>
                                </div>
                              </div>

                              <div className="space-y-2">
                                <Label htmlFor="ai-agent-model">Model (optional)</Label>
                                <Input
                                  id="ai-agent-model"
                                  className={INPUT_CLASS}
                                  value={draft.model}
                                  onChange={(event) => setDraft((prev) => ({ ...prev, model: event.target.value }))}
                                  placeholder="gpt-5-mini / internal model alias"
                                />
                              </div>

                              <div className="grid gap-4 md:grid-cols-2">
                                <div className="space-y-2">
                                  <Label htmlFor="ai-agent-provider">Provider</Label>
                                  <Input
                                    id="ai-agent-provider"
                                    className={INPUT_CLASS}
                                    value={draft.provider}
                                    onChange={(event) => setDraft((prev) => ({ ...prev, provider: event.target.value }))}
                                    placeholder="openai / ollama"
                                  />
                                </div>
                                <div className="space-y-2">
                                  <Label htmlFor="ai-agent-endpoint">Endpoint (optional)</Label>
                                  <Input
                                    id="ai-agent-endpoint"
                                    className={INPUT_CLASS}
                                    value={draft.endpoint}
                                    onChange={(event) => setDraft((prev) => ({ ...prev, endpoint: event.target.value }))}
                                    placeholder="http://192.168.31.190:8000/v1"
                                  />
                                </div>
                              </div>

                              <div className="space-y-2">
                                <Label htmlFor="ai-agent-prompt">Agent Instructions</Label>
                                <Textarea
                                  id="ai-agent-prompt"
                                  className={`${TEXTAREA_CLASS} min-h-[132px]`}
                                  value={draft.prompt}
                                  onChange={(event) => setDraft((prev) => ({ ...prev, prompt: event.target.value }))}
                                  placeholder="Provide guidance for triage, evidence checks, and response sequencing."
                                />
                              </div>

                              <div className={`${SUBPANEL_CLASS} space-y-4 p-4`}>
                                <div className="flex items-center justify-between">
                                  <div>
                                    <p className="text-sm font-semibold text-white">Triad Investigation Mode</p>
                                    <p className={MUTED_TEXT_CLASS}>Investigator + Reviewer + Arbiter for higher-confidence critical-case decisions.</p>
                                  </div>
                                  <Switch
                                    checked={draft.triadEnabled}
                                    onCheckedChange={(value) => setDraft((prev) => ({ ...prev, triadEnabled: value }))}
                                    data-testid="switch-triad-enabled"
                                  />
                                </div>
                                <Separator className="bg-[#2a2c3c]" />
                                <div className="flex items-center justify-between">
                                  <div>
                                    <p className="text-sm font-semibold text-white">Critical cases only</p>
                                    <p className={MUTED_TEXT_CLASS}>Run triad only for critical severity/priority cases.</p>
                                  </div>
                                  <Switch
                                    checked={draft.triadCriticalOnly}
                                    onCheckedChange={(value) => setDraft((prev) => ({ ...prev, triadCriticalOnly: value }))}
                                    disabled={!draft.triadEnabled}
                                    data-testid="switch-triad-critical-only"
                                  />
                                </div>
                                <div className="flex items-center justify-between">
                                  <div>
                                    <p className="text-sm font-semibold text-white">Require reviewer consensus</p>
                                    <p className={MUTED_TEXT_CLASS}>Block auto-actions if investigator and reviewer disagree.</p>
                                  </div>
                                  <Switch
                                    checked={draft.requireReviewerConsensus}
                                    onCheckedChange={(value) => setDraft((prev) => ({ ...prev, requireReviewerConsensus: value }))}
                                    disabled={!draft.triadEnabled}
                                    data-testid="switch-triad-consensus"
                                  />
                                </div>
                                <div className="space-y-2">
                                  <Label htmlFor="ai-agent-auto-action-confidence">Auto-action minimum confidence</Label>
                                  <Input
                                    id="ai-agent-auto-action-confidence"
                                    className={INPUT_CLASS}
                                    value={draft.autoActionMinConfidence}
                                    onChange={(event) => setDraft((prev) => ({ ...prev, autoActionMinConfidence: event.target.value }))}
                                    placeholder="85"
                                    disabled={!draft.triadEnabled}
                                  />
                                </div>
                                <div className="grid gap-3">
                                  <div className="space-y-2">
                                    <Label htmlFor="ai-agent-investigator-prompt">Investigator prompt (optional)</Label>
                                    <Textarea
                                      id="ai-agent-investigator-prompt"
                                      className={`${TEXTAREA_CLASS} min-h-[72px]`}
                                      value={draft.investigatorPrompt}
                                      onChange={(event) => setDraft((prev) => ({ ...prev, investigatorPrompt: event.target.value }))}
                                      placeholder="How investigator should perform initial triage."
                                      disabled={!draft.triadEnabled}
                                    />
                                  </div>
                                  <div className="space-y-2">
                                    <Label htmlFor="ai-agent-reviewer-prompt">Reviewer prompt (optional)</Label>
                                    <Textarea
                                      id="ai-agent-reviewer-prompt"
                                      className={`${TEXTAREA_CLASS} min-h-[72px]`}
                                      value={draft.reviewerPrompt}
                                      onChange={(event) => setDraft((prev) => ({ ...prev, reviewerPrompt: event.target.value }))}
                                      placeholder="How reviewer should challenge and validate conclusions."
                                      disabled={!draft.triadEnabled}
                                    />
                                  </div>
                                  <div className="space-y-2">
                                    <Label htmlFor="ai-agent-arbiter-prompt">Arbiter prompt (optional)</Label>
                                    <Textarea
                                      id="ai-agent-arbiter-prompt"
                                      className={`${TEXTAREA_CLASS} min-h-[72px]`}
                                      value={draft.arbiterPrompt}
                                      onChange={(event) => setDraft((prev) => ({ ...prev, arbiterPrompt: event.target.value }))}
                                      placeholder="How arbiter should synthesize final verdict and risk."
                                      disabled={!draft.triadEnabled}
                                    />
                                  </div>
                                </div>
                              </div>

                              <div className={`${SUBPANEL_CLASS} space-y-4 p-4`}>
                                <div className="mb-3 flex items-center gap-2 text-sm font-semibold text-white">
                                  <Cable size={15} className="text-[#9ca3af]" />
                                  Connector Routing
                                </div>
                                <div className="grid gap-4 xl:grid-cols-2">
                                  <div className="space-y-2">
                                    <p className="text-sm font-medium text-white">Enrichment Connectors</p>
                                    {connectorOptions.length === 0 ? (
                                      <p className={MUTED_TEXT_CLASS}>No outbound connectors available.</p>
                                    ) : (
                                      <div className="space-y-2">
                                        {connectorOptions.map((connector) => (
                                          <label
                                            key={`enrichment-${connector.id}`}
                                            className="flex items-center gap-3 rounded-lg border border-[#2a2c3c] bg-[#0b0c10] px-3 py-2 text-sm text-[#e5e7eb]"
                                          >
                                            <Checkbox
                                              checked={draft.enrichmentConnectorIds.includes(connector.id)}
                                              onCheckedChange={(checked) => handleToggleConnector("enrichmentConnectorIds", connector.id, checked === true)}
                                            />
                                            <div className="min-w-0 flex-1">
                                              <p className="truncate text-sm text-white">{connector.name}</p>
                                              <p className={MUTED_TEXT_CLASS}>
                                                {connector.channel || "connector"}{connector.enabled ? "" : " • disabled"}
                                              </p>
                                            </div>
                                          </label>
                                        ))}
                                      </div>
                                    )}
                                  </div>
                                  <div className="space-y-2">
                                    <p className="text-sm font-medium text-white">Notification Connectors</p>
                                    {connectorOptions.length === 0 ? (
                                      <p className={MUTED_TEXT_CLASS}>No outbound connectors available.</p>
                                    ) : (
                                      <div className="space-y-2">
                                        {connectorOptions.map((connector) => (
                                          <label
                                            key={`notification-${connector.id}`}
                                            className="flex items-center gap-3 rounded-lg border border-[#2a2c3c] bg-[#0b0c10] px-3 py-2 text-sm text-[#e5e7eb]"
                                          >
                                            <Checkbox
                                              checked={draft.notificationConnectorIds.includes(connector.id)}
                                              onCheckedChange={(checked) => handleToggleConnector("notificationConnectorIds", connector.id, checked === true)}
                                            />
                                            <div className="min-w-0 flex-1">
                                              <p className="truncate text-sm text-white">{connector.name}</p>
                                              <p className={MUTED_TEXT_CLASS}>
                                                {connector.channel || "connector"}{connector.enabled ? "" : " • disabled"}
                                              </p>
                                            </div>
                                          </label>
                                        ))}
                                      </div>
                                    )}
                                  </div>
                                </div>
                              </div>

                              <div className={`${SUBPANEL_CLASS} space-y-3 p-4`}>
                                <div className="flex items-center justify-between">
                                  <div className="flex items-center gap-2">
                                    <PlusCircle size={15} className="text-[#9ca3af]" />
                                    <div>
                                      <p className="text-sm font-semibold text-white">Investigation Plan</p>
                                      <p className={MUTED_TEXT_CLASS}>Define stage prompts and connector calls per stage.</p>
                                    </div>
                                  </div>
                                  <Button variant="outline" className={OUTLINE_BUTTON_CLASS} onClick={handleAddStage}>
                                    <Plus size={14} className="mr-1" />
                                    Add Stage
                                  </Button>
                                </div>
                                {draft.stages.length === 0 ? (
                                  <p className={MUTED_TEXT_CLASS}>No explicit stages. Agent will run with default prompt/connectors.</p>
                                ) : (
                                  <div className="space-y-3">
                                    {draft.stages.map((stage, stageIndex) => (
                                      <div key={stage.id || `stage-${stageIndex}`} className="rounded-lg border border-[#2a2c3c] bg-[#0b0c10] p-3">
                                        <div className="mb-3 flex items-center justify-between gap-2">
                                          <p className="text-sm font-semibold text-white">Stage {stageIndex + 1}</p>
                                          <Button
                                            variant="ghost"
                                            className="h-7 w-7 p-0 text-[#9ca3af] hover:bg-[#171b2a] hover:text-white"
                                            onClick={() => handleRemoveStage(stageIndex)}
                                          >
                                            <X size={14} />
                                          </Button>
                                        </div>
                                        <div className="grid gap-3 md:grid-cols-2">
                                          <div className="space-y-2">
                                            <Label htmlFor={`ai-agent-stage-name-${stageIndex}`}>Name</Label>
                                            <Input
                                              id={`ai-agent-stage-name-${stageIndex}`}
                                              className={INPUT_CLASS}
                                              value={stage.name}
                                              onChange={(event) => handleUpdateStage(stageIndex, { name: event.target.value })}
                                              placeholder="Threat intelligence enrichment"
                                            />
                                          </div>
                                          <div className="space-y-2">
                                            <Label htmlFor={`ai-agent-stage-tags-${stageIndex}`}>Stage Tags (optional)</Label>
                                            <Input
                                              id={`ai-agent-stage-tags-${stageIndex}`}
                                              className={INPUT_CLASS}
                                              value={stage.caseTagsInput}
                                              onChange={(event) => handleUpdateStage(stageIndex, { caseTagsInput: event.target.value })}
                                              placeholder="ti-reviewed, external-contacted"
                                            />
                                          </div>
                                        </div>
                                        <div className="mt-3 space-y-2">
                                          <Label htmlFor={`ai-agent-stage-description-${stageIndex}`}>Description (optional)</Label>
                                          <Textarea
                                            id={`ai-agent-stage-description-${stageIndex}`}
                                            className={`${TEXTAREA_CLASS} min-h-[68px]`}
                                            value={stage.description}
                                            onChange={(event) => handleUpdateStage(stageIndex, { description: event.target.value })}
                                            placeholder="Explain what this stage should verify or enrich before the next step."
                                          />
                                        </div>
                                        <div className="mt-3 space-y-2">
                                          <Label htmlFor={`ai-agent-stage-prompt-${stageIndex}`}>Prompt</Label>
                                          <Textarea
                                            id={`ai-agent-stage-prompt-${stageIndex}`}
                                            className={`${TEXTAREA_CLASS} min-h-[88px]`}
                                            value={stage.prompt}
                                            onChange={(event) => handleUpdateStage(stageIndex, { prompt: event.target.value })}
                                            placeholder="Instructions for this stage"
                                          />
                                        </div>
                                        <div className="mt-3 space-y-2">
                                          <p className="text-xs font-medium uppercase tracking-wider text-[#9ca3af]">Stage Enrichment Connectors</p>
                                          {connectorOptions.length === 0 ? (
                                            <p className={MUTED_TEXT_CLASS}>No connectors available.</p>
                                          ) : (
                                            <div className="grid gap-2 md:grid-cols-2">
                                              {connectorOptions.map((connector) => (
                                                <label
                                                  key={`stage-${stageIndex}-connector-${connector.id}`}
                                                  className="flex items-center gap-3 rounded-lg border border-[#2a2c3c] bg-[#111622] px-3 py-2 text-sm text-[#e5e7eb]"
                                                >
                                                  <Checkbox
                                                    checked={stage.enrichmentConnectorIds.includes(connector.id)}
                                                    onCheckedChange={(checked) => handleToggleStageConnector(stageIndex, connector.id, checked === true)}
                                                  />
                                                  <div className="min-w-0 flex-1">
                                                    <p className="truncate text-sm text-white">{connector.name}</p>
                                                    <p className={MUTED_TEXT_CLASS}>{connector.channel || "connector"}</p>
                                                  </div>
                                                </label>
                                              ))}
                                            </div>
                                          )}
                                        </div>
                                      </div>
                                    ))}
                                  </div>
                                )}
                              </div>

                              <div className={`${SUBPANEL_CLASS} space-y-3 p-4`}>
                                <div className="flex items-center justify-between">
                                  <div>
                                    <p className="text-sm font-semibold text-white">Enabled</p>
                                    <p className={MUTED_TEXT_CLASS}>Participates in automated and manual runs.</p>
                                  </div>
                                  <Switch checked={draft.enabled} onCheckedChange={(value) => setDraft((prev) => ({ ...prev, enabled: value }))} />
                                </div>
                                <Separator className="bg-[#2a2c3c]" />
                                <div className="flex items-center justify-between">
                                  <div>
                                    <p className="text-sm font-semibold text-white">Auto-create tasks</p>
                                    <p className={MUTED_TEXT_CLASS}>Create operational tasks from recommendations.</p>
                                  </div>
                                  <Switch
                                    checked={draft.autoCreateTasks}
                                    onCheckedChange={(value) => setDraft((prev) => ({ ...prev, autoCreateTasks: value }))}
                                  />
                                </div>
                                <Separator className="bg-[#2a2c3c]" />
                                <div className="flex items-center justify-between">
                                  <div>
                                    <p className="text-sm font-semibold text-white">Post agent comment</p>
                                    <p className={MUTED_TEXT_CLASS}>Write execution summary back to case timeline.</p>
                                  </div>
                                  <Switch checked={draft.autoComment} onCheckedChange={(value) => setDraft((prev) => ({ ...prev, autoComment: value }))} />
                                </div>
                                <Separator className="bg-[#2a2c3c]" />
                                <div className="flex items-center justify-between">
                                  <div>
                                    <p className="text-sm font-semibold text-white">Auto-close case by verdict</p>
                                    <p className={MUTED_TEXT_CLASS}>Automatically close case if verdict is in allow-list.</p>
                                  </div>
                                  <Switch checked={draft.autoCloseCase} onCheckedChange={(value) => setDraft((prev) => ({ ...prev, autoCloseCase: value }))} />
                                </div>
                                {draft.autoCloseCase ? (
                                  <div className="space-y-2">
                                    <Label htmlFor="ai-agent-auto-close-verdicts">Auto-close verdicts</Label>
                                    <Input
                                      id="ai-agent-auto-close-verdicts"
                                      className={INPUT_CLASS}
                                      value={draft.autoCloseVerdictsInput}
                                      onChange={(event) => setDraft((prev) => ({ ...prev, autoCloseVerdictsInput: event.target.value }))}
                                      placeholder="benign, resolved, closed"
                                    />
                                  </div>
                                ) : null}
                              </div>

                              <div className="flex flex-wrap items-center justify-end gap-2">
                                {draft.id ? (
                                  <Button
                                    variant="outline"
                                    className={OUTLINE_BUTTON_CLASS}
                                    onClick={() => handleDeleteAgent({ id: draft.id, name: draft.name })}
                                    disabled={deleteAgent.isPending}
                                  >
                                    {deleteAgent.isPending ? <Loader2 size={15} className="mr-2 animate-spin" /> : <Trash2 size={15} className="mr-2" />}
                                    Delete
                                  </Button>
                                ) : null}
                                <Button
                                  className={PRIMARY_BUTTON_CLASS}
                                  onClick={handleSaveAgent}
                                  disabled={isSaving}
                                  data-testid="button-save-ai-agent"
                                >
                                  {isSaving ? <Loader2 size={15} className="mr-2 animate-spin" /> : <Save size={15} className="mr-2" />}
                                  {draft.id ? "Save Changes" : "Create Agent"}
                                </Button>
                              </div>
                            </div>
                          </Card>
                </div>
              </TabsContent>
            </Tabs>
          </TabsContent>

          <TabsContent value="triad" className="mt-0 space-y-5">
            <div className="grid gap-4 md:grid-cols-4">
              <Card className={`${PANEL_CLASS} p-4`}>
                <p className={MUTED_TEXT_CLASS}>Triad Cases</p>
                <p className="mt-1 text-2xl font-semibold text-white">{triadEntries.length}</p>
              </Card>
              <Card className={`${PANEL_CLASS} p-4`}>
                <p className={MUTED_TEXT_CLASS}>Blocked Auto-Actions</p>
                <p className="mt-1 text-2xl font-semibold text-[#fca5a5]">{triadBlockedCount}</p>
              </Card>
              <Card className={`${PANEL_CLASS} p-4`}>
                <p className={MUTED_TEXT_CLASS}>Requires Human Review</p>
                <p className="mt-1 text-2xl font-semibold text-[#fcd34d]">{triadHumanReviewCount}</p>
              </Card>
              <Card className={`${PANEL_CLASS} p-4`}>
                <p className={MUTED_TEXT_CLASS}>Reviewer Consensus</p>
                <p className="mt-1 text-2xl font-semibold text-[#86efac]">{triadConsensusRate}%</p>
              </Card>
            </div>

            <Card className={`${PANEL_CLASS} p-4`}>
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <h2 className="text-sm font-semibold text-white">Triad Runs by Case</h2>
                  <p className={MUTED_TEXT_CLASS}>Compare investigator/reviewer/arbiter outputs and policy blockers for every triad run.</p>
                </div>
                <div className="flex items-center gap-2">
                  <Label htmlFor="triad-agent-filter" className="text-xs text-[#9ca3af]">Agent</Label>
                  <Select value={triadAgentFilter} onValueChange={setTriadAgentFilter}>
                    <SelectTrigger id="triad-agent-filter" className={`${SELECT_TRIGGER_CLASS} w-[220px]`}>
                      <SelectValue placeholder="All agents" />
                    </SelectTrigger>
                    <SelectContent className={SELECT_CONTENT_CLASS}>
                      <SelectItem value="all">All agents</SelectItem>
                      {agents.map((item: any) => (
                        <SelectItem key={`triad-agent-${item.id}`} value={String(item.id)}>
                          {String(item.name || item.id)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>

              {showTriadSkeleton ? (
                <div className="mt-4 space-y-2">
                  <Skeleton className="h-20 w-full rounded-xl" />
                  <Skeleton className="h-20 w-full rounded-xl" />
                  <Skeleton className="h-20 w-full rounded-xl" />
                </div>
              ) : null}

              {!showTriadSkeleton && triadEntries.length === 0 ? (
                <div className={`${SUBPANEL_CLASS} mt-4 p-4 text-sm text-[#d1d5db]`}>
                  No triad runs yet. Enable triad mode and execute an agent to collect analytics.
                </div>
              ) : null}

              <div className="mt-4 space-y-3">
                {!showTriadSkeleton
                  ? triadVisibleEntries.map((entry, index) => {
                      const investigator = entry.triad?.investigator || {};
                      const reviewer = entry.triad?.reviewer || {};
                      const arbiter = entry.triad?.arbiter || {};
                      return (
                        <Card key={`${entry.runId}-${entry.caseId || index}`} className={`${SUBPANEL_CLASS} p-4`}>
                          <div className="mb-3 flex flex-wrap items-start justify-between gap-2">
                            <div>
                              <p className="text-sm font-semibold text-white">
                                {entry.caseNumber || entry.caseId || "Case"}
                              </p>
                              <p className="text-xs text-[#9ca3af]">{entry.title || "Untitled case"} • {entry.agentName || entry.agentId}</p>
                            </div>
                            <div className="flex flex-col items-end gap-2">
                              <div className="flex flex-wrap justify-end gap-1.5">
                                <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db]">
                                  verdict: {entry.verdict || "unknown"}
                                </Badge>
                                <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db]">
                                  conf: {Number.isFinite(entry.confidence) ? entry.confidence.toFixed(1) : "0.0"}
                                </Badge>
                                <Badge
                                  className={
                                    entry.autoActionsAllowed
                                      ? "border border-[rgba(34,197,94,0.3)] bg-[rgba(34,197,94,0.16)] text-[#86efac]"
                                      : "border border-[rgba(239,68,68,0.3)] bg-[rgba(239,68,68,0.16)] text-[#fca5a5]"
                                  }
                                >
                                  {entry.autoActionsAllowed ? "auto actions allowed" : "auto actions blocked"}
                                </Badge>
                              </div>
                              {entry.caseId ? (
                                <Button
                                  type="button"
                                  size="sm"
                                  className={OUTLINE_BUTTON_CLASS}
                                  data-testid={`button-triad-open-case-${entry.runId}`}
                                  onClick={() => openEntityTrace("case", entry.caseId)}
                                >
                                  <ArrowUpRight size={14} className="mr-1.5" />
                                  Open case
                                </Button>
                              ) : null}
                            </div>
                          </div>
                          <div className="grid gap-2 md:grid-cols-3">
                            {[{ key: "Investigator", node: investigator }, { key: "Reviewer", node: reviewer }, { key: "Arbiter", node: arbiter }].map((role) => (
                              <div key={role.key} className="rounded-lg border border-[#2a2c3c] bg-[#0b0c10] p-3">
                                <p className="text-xs font-semibold uppercase tracking-wide text-[#9ca3af]">{role.key}</p>
                                <p className="mt-1 text-sm text-white">{String(role.node?.verdict || "n/a")}</p>
                                <p className="text-xs text-[#9ca3af]">conf: {Number(role.node?.confidence ?? 0).toFixed(1)}</p>
                              </div>
                            ))}
                          </div>
                          <div className="mt-3 flex flex-wrap gap-1.5">
                            <Badge
                              className={
                                entry.reviewerConsensus
                                  ? "border border-[rgba(34,197,94,0.3)] bg-[rgba(34,197,94,0.16)] text-[#86efac]"
                                  : "border border-[rgba(234,179,8,0.3)] bg-[rgba(234,179,8,0.16)] text-[#fcd34d]"
                              }
                            >
                              reviewer consensus: {entry.reviewerConsensus ? "yes" : "no"}
                            </Badge>
                            <Badge
                              className={
                                entry.requiresHumanReview
                                  ? "border border-[rgba(234,179,8,0.3)] bg-[rgba(234,179,8,0.16)] text-[#fcd34d]"
                                  : "border border-[rgba(34,197,94,0.3)] bg-[rgba(34,197,94,0.16)] text-[#86efac]"
                              }
                            >
                              human review: {entry.requiresHumanReview ? "required" : "not required"}
                            </Badge>
                            {entry.actionBlockers.slice(0, 3).map((item) => (
                              <Badge key={`${entry.runId}-blocker-${item}`} className="border border-[rgba(239,68,68,0.3)] bg-[rgba(239,68,68,0.16)] text-[#fca5a5]">
                                {item}
                              </Badge>
                            ))}
                          </div>
                        </Card>
                      );
                    })
                  : null}
              </div>
              {!showTriadSkeleton && triadEntries.length > TRIAD_PAGE_SIZE ? (
                <div className="mt-3 flex items-center justify-between gap-3 text-xs text-[#9ca3af]">
                  <span>Showing {(triadPageStart + 1)}–{Math.min(triadPageEnd, triadEntries.length)} of {triadEntries.length} triad runs</span>
                  <div className="flex items-center gap-2">
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      className={`${OUTLINE_BUTTON_CLASS} h-7 px-2 text-[11px]`}
                      disabled={triadCurrentPage <= 1}
                      onClick={() => setTriadPage(Math.max(1, triadCurrentPage - 1))}
                    >
                      Prev
                    </Button>
                    <span className="min-w-[80px] text-center">Page {triadCurrentPage} of {triadTotalPages}</span>
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      className={`${OUTLINE_BUTTON_CLASS} h-7 px-2 text-[11px]`}
                      disabled={triadCurrentPage >= triadTotalPages}
                      onClick={() => setTriadPage(Math.min(triadTotalPages, triadCurrentPage + 1))}
                    >
                      Next
                    </Button>
                  </div>
                </div>
              ) : null}
            </Card>
          </TabsContent>

          <TabsContent value="operations" className="mt-0 space-y-5">
            {showOperationsSkeleton ? (
              <div className="space-y-4">
                <Skeleton className="h-24 w-full rounded-2xl" />
                <Skeleton className="h-64 w-full rounded-2xl" />
                <Skeleton className="h-64 w-full rounded-2xl" />
              </div>
            ) : (
              <>
                <div className="grid gap-4 md:grid-cols-5">
                  {[
                    { key: "queued", label: "Queued", color: "text-[#fcd34d]" },
                    { key: "processing", label: "Processing", color: "text-[#60a5fa]" },
                    { key: "done", label: "Done", color: "text-[#86efac]" },
                    { key: "failed", label: "Failed", color: "text-[#fca5a5]" },
                    { key: "total", label: "Total", color: "text-white" },
                  ].map((metric) => (
                    <Card key={metric.key} className={`${PANEL_CLASS} p-4`}>
                      <p className={MUTED_TEXT_CLASS}>{metric.label}</p>
                      <p className={`mt-1 text-2xl font-semibold ${metric.color}`}>
                        {Number((operationsOverview?.queue || {})[metric.key] ?? 0)}
                      </p>
                    </Card>
                  ))}
                </div>

                <Card className={`${PANEL_CLASS} p-4`}>
                  <div className="mb-3 flex items-center justify-between">
                    <h2 className="text-sm font-semibold text-white">Active Workloads</h2>
                    <p className={MUTED_TEXT_CLASS}>
                      Poll: {Number(operationsOverview?.workers?.poll_interval_ms ?? 0)} ms • Batch: {Number(operationsOverview?.workers?.batch_size ?? 0)} • Retries: {Number(operationsOverview?.workers?.max_retries ?? 0)} • LLM slots: {Number(operationsOverview?.workers?.llm_max_concurrent ?? 0)}
                    </p>
                  </div>
                  {Array.isArray(operationsOverview?.processing_events) && operationsOverview.processing_events.length > 0 ? (
                    <div className="space-y-2">
                      {operationsOverview.processing_events.map((item: any) => {
                        const workloadId = String(item?.id || `${item?.entity_type || "entity"}-${item?.entity_id || "unknown"}-${item?.status || "status"}`);
                        const isExpanded = workloadId === expandedWorkloadId;
                        const agentProgress = buildWorkloadAgentProgress(item, Array.isArray(allRunsFeed) ? allRunsFeed : []);
                        const workloadItems = Array.isArray(item?.workloads) ? item.workloads : [];
                        const actionableWorkloads = workloadItems.filter((entry: any) => {
                          const status = String(entry?.status || "").trim().toLowerCase();
                          return status !== "done" && status !== "cancelled" && status !== "canceled";
                        });
                        const primaryActionWorkloadId = actionableWorkloads.length === 1 ? String(actionableWorkloads[0]?.id || "").trim() : "";
                        const isRestartPending =
                          pendingWorkloadAction?.workloadId === primaryActionWorkloadId &&
                          pendingWorkloadAction?.action === "restart" &&
                          restartWorkload.isPending;
                        const isClosePending =
                          pendingWorkloadAction?.workloadId === primaryActionWorkloadId &&
                          pendingWorkloadAction?.action === "close" &&
                          closeWorkload.isPending;
                        const matchedAgents = Number(item?.workload_count ?? item?.matched_agents ?? item?.candidate_count ?? agentProgress.length ?? 0);
                        const processedAgents = Number(
                          item?.processed_agents ??
                            workloadItems.filter((entry: any) => {
                              const status = String(entry?.status || "").trim().toLowerCase();
                              return status === "done" || status === "failed" || status === "cancelled" || status === "canceled";
                            }).length,
                        );
                        const progressPercent =
                          matchedAgents > 0 ? Math.max(0, Math.min(100, Math.round((processedAgents / matchedAgents) * 100))) : 0;
                        const startedAt = String(item?.started_at || "").trim();
                        const finishedAt = String(item?.finished_at || "").trim();
                        const startedAtMs = startedAt ? new Date(startedAt).getTime() : Number.NaN;
                        const finishedAtMs = finishedAt ? new Date(finishedAt).getTime() : Number.NaN;
                        const workloadDurationMs = Number.isFinite(startedAtMs)
                          ? Math.max(0, (Number.isFinite(finishedAtMs) ? finishedAtMs : Date.now()) - startedAtMs)
                          : 0;
                        const workloadError = String(item?.last_error || "").trim();
                        const hasStartedRuns = agentProgress.some((progress) => progress.runId);
                        const summaryFocusWorkloadId = primaryActionWorkloadId || String(workloadItems[0]?.id || "").trim();
                        const summaryFocusWorkload = workloadItems.find((entry: any) => String(entry?.id || "").trim() === summaryFocusWorkloadId) || workloadItems[0] || null;
                        const summaryStageKey = resolveWorkloadStageKey(summaryFocusWorkload, summaryFocusWorkload?.run?.result);
                        const entityLocation = buildEntityTraceLocation(currentTenantSlug, item?.entity_type, item?.entity_id, summaryFocusWorkloadId, summaryStageKey);

                        return (
                          <div key={workloadId} className={`${SUBPANEL_CLASS} p-3`}>
                            <div className="flex flex-wrap items-start justify-between gap-2">
                              <div>
                                <p className="text-sm font-semibold text-white">{String(item?.entity_reference || item?.entity_id || "")}</p>
                                <p className="text-xs text-[#9ca3af]">{String(item?.entity_title || "")}</p>
                              </div>
                              <div className="flex items-center gap-1.5">
                                <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db]">{String(item?.entity_type || "entity")}</Badge>
                                <Badge className="border border-[rgba(59,130,246,0.35)] bg-[rgba(59,130,246,0.12)] text-[#93c5fd]">
                                  {matchedAgents} workloads
                                </Badge>
                                {summaryFocusWorkloadId ? (
                                  <Button
                                    type="button"
                                    size="icon"
                                    variant="ghost"
                                    data-testid={`button-workload-inspect-${workloadId}`}
                                    className="h-7 w-7 rounded-md border border-[#2a2c3c] bg-[#0f131d] text-[#c7cedf] hover:bg-[#171b2a] hover:text-white"
                                    onClick={() =>
                                      openOperationsTraceDrawer(item, "processing", {
                                        focusWorkloadId: summaryFocusWorkloadId,
                                        focusStageKey: summaryStageKey,
                                      })
                                    }
                                  >
                                    <Eye size={14} />
                                  </Button>
                                ) : null}
                                <Button
                                  type="button"
                                  size="icon"
                                  variant="ghost"
                                  data-testid={`button-workload-expand-${workloadId}`}
                                  className="h-7 w-7 rounded-md border border-[#2a2c3c] bg-[#0f131d] text-[#c7cedf] hover:bg-[#171b2a] hover:text-white"
                                  onClick={() => setExpandedWorkloadId((prev) => (prev === workloadId ? "" : workloadId))}
                                >
                                  {isExpanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                                </Button>
                              </div>
                            </div>
                            <div className="mt-2 flex flex-wrap items-center justify-between gap-2">
                              <div className="flex flex-wrap gap-1.5">
                                {Array.isArray(item?.candidate_agents)
                                  ? item.candidate_agents.slice(0, 5).map((agent: any) => (
                                      <Badge key={`${item.id}-${agent.id}`} className="border border-[#2a2c3c] bg-[#111622] text-[#d1d5db]">
                                        {String(agent?.name || agent?.id || "agent")}
                                      </Badge>
                                    ))
                                  : null}
                              </div>
                              <div className="flex flex-wrap gap-2">
                                {entityLocation ? (
                                  <Button
                                    type="button"
                                    size="sm"
                                    className={OUTLINE_BUTTON_CLASS}
                                    data-testid={`button-workload-open-entity-${workloadId}`}
                                    onClick={() => openEntityTrace(item?.entity_type, item?.entity_id, summaryFocusWorkloadId, summaryStageKey)}
                                  >
                                    <ArrowUpRight size={14} className="mr-1.5" />
                                    {buildEntityOpenLabel(item?.entity_type)}
                                  </Button>
                                ) : null}
                              </div>
                            </div>

                            {isExpanded ? (
                              <>
                              <div className="mt-3 grid gap-3 xl:grid-cols-[300px_1fr]">
                                <div className="rounded-lg border border-[#2a2c3c] bg-[#0b0c10] p-3">
                                  <p className="text-xs font-semibold uppercase tracking-wide text-[#9ca3af]">Workload Diagnostics</p>
                                  <div className="mt-2 space-y-1 text-xs text-[#cbd5e1]">
                                    <p>Queue event: <span className="font-mono text-[#e5e7eb]">{workloadId}</span></p>
                                    <p>Status: <span className="text-white">{String(item?.status || "processing")}</span></p>
                                    <p>Workflow: <span className="font-mono text-[#9ca3af]">{String(item?.workflow_id || "n/a")}</span></p>
                                    <p>Progress: <span className="text-white">{processedAgents}/{Math.max(matchedAgents, 0)} agents</span></p>
                                    <p>Attempts: <span className="text-white">{Number(item?.attempt_count ?? 0)}/{Math.max(1, Number(item?.max_attempts ?? 1))}</span></p>
                                    <p>Started: <span className="text-white">{formatDateTime(startedAt)}</span></p>
                                    <p>Updated: <span className="text-white">{formatDateTime(String(item?.updated_at || ""))}</span></p>
                                    <p>Runtime: <span className="text-white">{formatDuration(workloadDurationMs)}</span></p>
                                  </div>
                                  <div className="mt-3 h-1.5 w-full rounded-full bg-[#131a29]">
                                    <div
                                      className="h-1.5 rounded-full bg-[rgba(59,130,246,0.7)] transition-all duration-200"
                                      style={{ width: `${progressPercent}%` }}
                                    />
                                  </div>
                                  {workloadError ? (
                                    <div className="mt-3 rounded-lg border border-[rgba(239,68,68,0.35)] bg-[rgba(127,29,29,0.2)] p-2 text-xs text-[#fecaca]">
                                      <p className="mb-1 flex items-center gap-1 font-semibold text-[#fca5a5]">
                                        <AlertTriangle size={12} />
                                        Last queue error
                                      </p>
                                      <p className="break-words">{workloadError}</p>
                                    </div>
                                  ) : null}
                                  <div className="mt-3 flex flex-wrap gap-2">
                                    {primaryActionWorkloadId ? (
                                      <>
                                        <Button
                                          type="button"
                                          size="sm"
                                          className={OUTLINE_BUTTON_CLASS}
                                          data-testid={`button-workload-restart-${primaryActionWorkloadId}`}
                                          disabled={isRestartPending || isClosePending}
                                          onClick={() => handleWorkloadAction(primaryActionWorkloadId, "restart")}
                                        >
                                          {isRestartPending ? <Loader2 size={14} className="mr-1.5 animate-spin" /> : <Play size={14} className="mr-1.5" />}
                                          Restart
                                        </Button>
                                        <Button
                                          type="button"
                                          size="sm"
                                          className="border border-[rgba(239,68,68,0.35)] bg-[rgba(127,29,29,0.2)] text-[#fecaca] hover:bg-[rgba(153,27,27,0.32)]"
                                          data-testid={`button-workload-close-${primaryActionWorkloadId}`}
                                          disabled={isRestartPending || isClosePending}
                                          onClick={() => handleWorkloadAction(primaryActionWorkloadId, "close")}
                                        >
                                          {isClosePending ? <Loader2 size={14} className="mr-1.5 animate-spin" /> : <X size={14} className="mr-1.5" />}
                                          Close
                                        </Button>
                                      </>
                                    ) : actionableWorkloads.length > 1 ? (
                                      <p className="text-xs text-[#9ca3af]">Use per-agent controls below for multi-agent workloads.</p>
                                    ) : null}
                                  </div>
                                </div>

                                <div className="rounded-lg border border-[#2a2c3c] bg-[#0b0c10] p-3">
                                  <p className="text-xs font-semibold uppercase tracking-wide text-[#9ca3af]">Agent Execution</p>
                                  {agentProgress.length > 0 ? (
                                    <div className="mt-2 space-y-2">
                                      {agentProgress.map((progress) => (
                                        <div key={`${workloadId}-${progress.agentId || progress.agentName}`} className="rounded-md border border-[#23283a] bg-[#101623] p-2.5">
                                          <div className="flex flex-wrap items-center justify-between gap-2">
                                            <p className="text-sm font-medium text-white">{progress.agentName}</p>
                                            <Badge className={workloadStatusBadgeClass(progress.status)}>{progress.statusLabel}</Badge>
                                          </div>
                                          <div className="mt-1 space-y-0.5 text-xs text-[#9ca3af]">
                                            <p>Run: <span className="font-mono text-[#cbd5e1]">{progress.runId || "not started yet"}</span></p>
                                            {progress.workloadId ? <p>Workload: <span className="font-mono text-[#cbd5e1]">{progress.workloadId}</span></p> : null}
                                            {progress.workflowId ? <p>Workflow: <span className="font-mono text-[#cbd5e1]">{progress.workflowId}</span></p> : null}
                                            <p>Execution: <span className="text-[#e5e7eb]">{formatExecutionPolicy(progress.executionPolicy)}</span>{Number.isFinite(progress.executionPriority) && progress.executionPriority !== 0 ? ` · priority ${progress.executionPriority}` : ""}{progress.executionPolicy === "fallback_chain" ? ` · step ${progress.executionIndex + 1}` : ""}</p>
                                            <p>Started: <span className="text-[#e5e7eb]">{formatDateTime(progress.startedAt)}</span></p>
                                            {progress.finishedAt ? <p>Finished: <span className="text-[#e5e7eb]">{formatDateTime(progress.finishedAt)}</span></p> : null}
                                            {progress.lastStage ? <p>Stage: <span className="text-[#e5e7eb]">{progress.lastStage}</span></p> : null}
                                            {progress.maxAttempts > 0 ? (
                                              <p>Attempts: <span className="text-[#e5e7eb]">{progress.attemptCount}/{progress.maxAttempts}</span></p>
                                            ) : null}
                                            {progress.verdict ? (
                                              <p>
                                                Verdict: <span className="text-[#e5e7eb]">{progress.verdict}</span>
                                                {progress.confidence !== null ? ` (${progress.confidence.toFixed(1)})` : ""}
                                              </p>
                                            ) : null}
                                          </div>
                                          {progress.workloadId ? (
                                            <div className="mt-2 flex flex-wrap gap-2">
                                              <Button
                                                type="button"
                                                size="sm"
                                                className={OUTLINE_BUTTON_CLASS}
                                                data-testid={`button-agent-workload-restart-${progress.workloadId}`}
                                                disabled={progress.workloadStatus === "processing" || progress.workloadStatus === "running" || progress.workloadStatus === "in_progress" || (pendingWorkloadAction?.workloadId === progress.workloadId && closeWorkload.isPending)}
                                                onClick={() => handleWorkloadAction(progress.workloadId, "restart")}
                                              >
                                                {pendingWorkloadAction?.workloadId === progress.workloadId && pendingWorkloadAction?.action === "restart" && restartWorkload.isPending ? (
                                                  <Loader2 size={14} className="mr-1.5 animate-spin" />
                                                ) : (
                                                  <Play size={14} className="mr-1.5" />
                                                )}
                                                Restart
                                              </Button>
                                              <Button
                                                type="button"
                                                size="sm"
                                                className="border border-[rgba(239,68,68,0.35)] bg-[rgba(127,29,29,0.2)] text-[#fecaca] hover:bg-[rgba(153,27,27,0.32)]"
                                                data-testid={`button-agent-workload-close-${progress.workloadId}`}
                                                disabled={progress.workloadStatus === "processing" || progress.workloadStatus === "running" || progress.workloadStatus === "in_progress" || (pendingWorkloadAction?.workloadId === progress.workloadId && restartWorkload.isPending)}
                                                onClick={() => handleWorkloadAction(progress.workloadId, "close")}
                                              >
                                                {pendingWorkloadAction?.workloadId === progress.workloadId && pendingWorkloadAction?.action === "close" && closeWorkload.isPending ? (
                                                  <Loader2 size={14} className="mr-1.5 animate-spin" />
                                                ) : (
                                                  <X size={14} className="mr-1.5" />
                                                )}
                                                Close
                                              </Button>
                                            </div>
                                          ) : null}
                                          {progress.error ? (
                                            <p className="mt-1.5 text-xs text-[#fca5a5]">{progress.error}</p>
                                          ) : progress.summary ? (
                                            <p className="mt-1.5 text-xs text-[#cbd5e1]">
                                              {progress.summary.length > 220 ? `${progress.summary.slice(0, 220)}...` : progress.summary}
                                            </p>
                                          ) : null}
                                        </div>
                                      ))}
                                    </div>
                                  ) : (
                                    <p className="mt-2 text-sm text-[#9ca3af]">No candidate agents available for this workload.</p>
                                  )}
                                  {!hasStartedRuns ? (
                                    <p className="mt-3 text-xs text-[#fcd34d]">
                                      No agent runs started yet. If this state is long-lived, verify queue worker health and connector latency.
                                    </p>
                                  ) : null}
                                </div>
                              </div>
                              <AIEntityTrace
                                trace={buildEventTracePayload(item)}
                                labels={AI_TRACE_LABELS}
                                compact
                                testIdPrefix={`ops-ai-trace-${workloadId}`}
                                renderExtraWorkloadActions={renderOperationsTraceActions}
                              />
                              </>
                            ) : null}
                          </div>
                        );
                      })}
                    </div>
                  ) : (
                    <div className={`${SUBPANEL_CLASS} p-4 text-sm text-[#d1d5db]`}>No active processing workloads right now.</div>
                  )}
                </Card>

                <Card className={`${PANEL_CLASS} p-4`}>
                  <h2 className="mb-3 text-sm font-semibold text-white">Recent Failed Workloads</h2>
                  {Array.isArray(operationsOverview?.recent_failed_events) && operationsOverview.recent_failed_events.length > 0 ? (
                    <div className="space-y-2">
                      {operationsOverview.recent_failed_events.flatMap((item: any) => {
                        const failedWorkloads = Array.isArray(item?.workloads)
                          ? item.workloads.filter((entry: any) => String(entry?.status || "").trim().toLowerCase() === "failed")
                          : [];
                        const entries = failedWorkloads.length > 0 ? failedWorkloads : [item];
                        return entries.map((entry: any) => {
                          const workloadId = String(entry?.id || `${item?.id || item?.entity_id || "workload"}-failed`);
                          const isRestartPending =
                            pendingWorkloadAction?.workloadId === workloadId &&
                            pendingWorkloadAction?.action === "restart" &&
                            restartWorkload.isPending;
                          const isClosePending =
                            pendingWorkloadAction?.workloadId === workloadId &&
                            pendingWorkloadAction?.action === "close" &&
                            closeWorkload.isPending;
                          return (
                            <div key={workloadId} className="rounded-lg border border-[rgba(239,68,68,0.35)] bg-[rgba(127,29,29,0.16)] p-3">
                              <div className="flex flex-wrap items-start justify-between gap-2">
                                <div className="min-w-0">
                                  <p className="truncate text-sm font-medium text-white">{String(item?.entity_reference || item?.entity_id || "")}</p>
                                  {String(entry?.agent_name || entry?.agentName || "").trim() ? (
                                    <p className="truncate text-xs text-[#fca5a5]">{String(entry?.agent_name || entry?.agentName || "")}</p>
                                  ) : null}
                                  <p className="truncate text-xs text-[#fecaca]">{String(entry?.last_error || item?.last_error || "workflow failed")}</p>
                                </div>
                                <Badge className="border border-[rgba(239,68,68,0.35)] bg-[rgba(127,29,29,0.2)] text-[#fecaca]">
                                  attempts {Number(entry?.attempt_count ?? item?.attempt_count ?? 0)}/{Math.max(1, Number(entry?.max_attempts ?? item?.max_attempts ?? 1))}
                                </Badge>
                              </div>
                              <div className="mt-2 flex flex-wrap gap-2">
                                <Button
                                  type="button"
                                  size="sm"
                                  className={OUTLINE_BUTTON_CLASS}
                                  data-testid={`button-failed-workload-restart-${workloadId}`}
                                  disabled={!workloadId || isRestartPending || isClosePending}
                                  onClick={() => handleWorkloadAction(workloadId, "restart")}
                                >
                                  {isRestartPending ? <Loader2 size={14} className="mr-1.5 animate-spin" /> : <Play size={14} className="mr-1.5" />}
                                  Restart
                                </Button>
                                <Button
                                  type="button"
                                  size="sm"
                                  className="border border-[rgba(239,68,68,0.35)] bg-[rgba(127,29,29,0.2)] text-[#fecaca] hover:bg-[rgba(153,27,27,0.32)]"
                                  data-testid={`button-failed-workload-close-${workloadId}`}
                                  disabled={!workloadId || isRestartPending || isClosePending}
                                  onClick={() => handleWorkloadAction(workloadId, "close")}
                                >
                                  {isClosePending ? <Loader2 size={14} className="mr-1.5 animate-spin" /> : <X size={14} className="mr-1.5" />}
                                  Close
                                </Button>
                                <Button
                                  type="button"
                                  size="sm"
                                  className={OUTLINE_BUTTON_CLASS}
                                  data-testid={`button-failed-workload-inspect-${workloadId}`}
                                  onClick={() =>
                                    openOperationsTraceDrawer(item, "failed", {
                                      focusWorkloadId: workloadId,
                                      focusStageKey: resolveWorkloadStageKey(entry, entry?.run?.result),
                                    })
                                  }
                                >
                                  <Eye size={14} className="mr-1.5" />
                                  Inspect
                                </Button>
                                {buildEntityTraceLocation(currentTenantSlug, item?.entity_type, item?.entity_id, workloadId, resolveWorkloadStageKey(entry, entry?.run?.result)) ? (
                                  <Button
                                    type="button"
                                    size="sm"
                                    className={OUTLINE_BUTTON_CLASS}
                                    data-testid={`button-failed-workload-open-${workloadId}`}
                                    onClick={() => openEntityTrace(item?.entity_type, item?.entity_id, workloadId, resolveWorkloadStageKey(entry, entry?.run?.result))}
                                  >
                                    <ArrowUpRight size={14} className="mr-1.5" />
                                    {buildEntityOpenLabel(item?.entity_type)}
                                  </Button>
                                ) : null}
                              </div>
                            </div>
                          );
                        });
                      })}
                    </div>
                  ) : (
                    <div className={`${SUBPANEL_CLASS} p-4 text-sm text-[#d1d5db]`}>No failed workloads.</div>
                  )}
                </Card>

                <div className="grid gap-5 xl:grid-cols-2">
                  <Card className={`${PANEL_CLASS} p-4`}>
                    <h2 className="mb-3 text-sm font-semibold text-white">Queue Backlog</h2>
                    {Array.isArray(operationsOverview?.queued_events) && operationsOverview.queued_events.length > 0 ? (
                      <div className="space-y-2">
                        {operationsOverview.queued_events.map((item: any) => (
                          <div
                            key={String(item?.id || `${item?.entity_type || "entity"}-${item?.entity_id || "unknown"}-${item?.status || "status"}`)}
                            className={`${SUBPANEL_CLASS} flex items-center justify-between gap-3 p-3`}
                          >
                            <div className="min-w-0">
                              <p className="truncate text-sm font-medium text-white">{String(item?.entity_reference || item?.entity_id || "")}</p>
                              <p className="truncate text-xs text-[#9ca3af]">{String(item?.entity_title || "")}</p>
                            </div>
                            <Badge className="border border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db]">{Number(item?.candidate_count ?? 0)} agents</Badge>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <div className={`${SUBPANEL_CLASS} p-4 text-sm text-[#d1d5db]`}>Queue is empty.</div>
                    )}
                  </Card>

                  <Card className={`${PANEL_CLASS} p-4`}>
                    <h2 className="mb-3 text-sm font-semibold text-white">Agent Workforce</h2>
                    {Array.isArray(operationsOverview?.agent_runtime) && operationsOverview.agent_runtime.length > 0 ? (
                      <div className="space-y-2">
                        {operationsOverview.agent_runtime.map((item: any) => (
                          <div key={String(item?.agent_id || item?.name || "agent")} className={`${SUBPANEL_CLASS} p-3`}>
                            <div className="flex items-center justify-between gap-2">
                              <p className="text-sm font-medium text-white">{String(item?.name || item?.agent_id || "Agent")}</p>
                              <Badge
                                className={
                                  item?.enabled
                                    ? "border border-[rgba(34,197,94,0.3)] bg-[rgba(34,197,94,0.16)] text-[#86efac]"
                                    : "border border-[rgba(156,163,175,0.3)] bg-[rgba(75,85,99,0.22)] text-[#d1d5db]"
                                }
                              >
                                {item?.enabled ? "enabled" : "disabled"}
                              </Badge>
                            </div>
                            <div className="mt-2 grid grid-cols-3 gap-2 text-xs text-[#9ca3af]">
                              <span>processing: {Number(item?.processing_items ?? 0)}</span>
                              <span>queued: {Number(item?.queue_matches ?? 0)}</span>
                              <span>errors: {Number(item?.last_run_errors ?? 0)}</span>
                            </div>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <div className={`${SUBPANEL_CLASS} p-4 text-sm text-[#d1d5db]`}>No agents available for this tenant.</div>
                    )}
                  </Card>
                </div>
              </>
            )}
          </TabsContent>
        </Tabs>
      </div>
      <OperationsTraceDrawer
        drawer={operationsTraceDrawer}
        onClose={() => setOperationsTraceDrawer(null)}
        subtitleFallback="Inspect workload stages, connector calls, blockers, and recovery actions without leaving Operations."
        subpanelClass={SUBPANEL_CLASS}
        outlineButtonClass={OUTLINE_BUTTON_CLASS}
        mutedTextClass={MUTED_TEXT_CLASS}
        isRefreshing={operationsDrawerIsRefreshing}
        lastUpdatedLabel={operationsDrawerLastUpdatedLabel}
        entityLocation={operationsDrawerEntityLocation}
        onOpenEntity={() =>
          openEntityTrace(
            operationsTraceDrawer?.entityType,
            operationsTraceDrawer?.entityId,
            operationsTraceDrawer?.focusWorkloadId,
            operationsTraceDrawer?.focusStageKey,
          )
        }
        onRefresh={() => {
          void refetchOperationsOverview();
          void refetchOperationsDrawerEntityTrace();
        }}
        focusedWorkload={operationsDrawerFocusedWorkload}
        eventLog={operationsDrawerEventLog}
        resolvedTrace={operationsDrawerResolvedTrace}
        entityTraceLoading={operationsDrawerEntityTraceLoading}
        traceLabels={AI_TRACE_LABELS}
        pendingWorkloadAction={pendingWorkloadAction}
        onRestartWorkload={(workloadId) => handleWorkloadAction(workloadId, "restart")}
        onCloseWorkload={(workloadId) => handleWorkloadAction(workloadId, "close")}
        renderExtraWorkloadActions={renderOperationsTraceActions}
        focusWorkloadId={operationsTraceDrawer?.focusWorkloadId || ""}
        focusStageKey={operationsTraceDrawer?.focusStageKey || ""}
      />
      <AlertDialog
        open={deleteDialogOpen}
        onOpenChange={(open) => {
          setDeleteDialogOpen(open);
          if (!open) {
            setPendingDeleteAgent(null);
          }
        }}
      >
        <AlertDialogContent className="rounded-xl border border-[#2a2c3c] bg-[#13141c] text-white">
          <AlertDialogHeader>
            <AlertDialogTitle>Delete agent</AlertDialogTitle>
            <AlertDialogDescription className="text-[#8b91a3]">
              {`Delete "${pendingDeleteAgent?.name || "AI agent"}"? This action cannot be undone.`}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel className="border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]">
              Cancel
            </AlertDialogCancel>
            <AlertDialogAction
              className="bg-[#7f1d1d] text-[#fee2e2] hover:bg-[#991b1b]"
              onClick={(event) => {
                event.preventDefault();
                confirmDeleteAgent();
              }}
              disabled={deleteAgent.isPending}
            >
              {deleteAgent.isPending ? <Loader2 size={14} className="mr-1 animate-spin" /> : <Trash2 size={14} className="mr-1" />}
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </AppLayout>
  );
}
