import { type DragEvent, useEffect, useMemo, useRef, useState } from "react";
import { useLocation } from "wouter";
import { AppLayout } from "@/components/layout";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Switch } from "@/components/ui/switch";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { toast } from "sonner";
import { ArrowLeft, Blocks, Bot, Cable, Check, CornerDownRight, FlaskConical, GitBranch, History, Play, Plus, Save, Send, Settings2, Trash2, WandSparkles, Workflow } from "lucide-react";
import {
  useAppState,
  useAskAI,
  useConnectorMethods,
  useCreateWorkflowCatalogItem,
  useDeleteWorkflowCatalogItem,
  useOutboundConnectors,
  useWorkflowRunHistory,
  useWorkflowTestRun,
  useUpdateWorkflowCatalogItem,
  useWorkflowCatalog,
  useWorkflowCredentials,
  useCreateWorkflowCredential,
  useUpdateWorkflowCredential,
  useDeleteWorkflowCredential,
  type WorkflowCatalogKind,
} from "@/lib/api";
import { EllipsisText } from "@/components/ui/ellipsis-text";
import { Skeleton } from "@/components/ui/skeleton";
import {
  WORKFLOW_NODE_LIBRARY,
  groupWorkflowNodeLibraryByCategory,
  workflowNodeSummary,
  workflowNodeTemplateById,
  type WorkflowNodeField,
  type WorkflowNodeTemplate,
} from "@/features/workflow-studio/node-library";
import { workflowNodeVisual } from "@/features/workflow-studio/node-visuals";
import { applySearchPatch, parsePositiveIntParam, splitLocationPathAndSearch } from "@/lib/url-state";
import { useMinimumLoading } from "@/lib/use-minimum-loading";

import {
  EDGE_LABEL_OPTIONS,
  NODE_HEIGHT,
  NODE_WIDTH,
  WORKFLOW_ACTION_BUTTON_CLASS,
  WORKFLOW_CATALOG_KIND_SET,
  WORKFLOW_MUTED_TEXT_CLASS,
  WORKFLOW_PAGE_SHELL_CLASS,
  WORKFLOW_PAGE_SIZE_OPTIONS,
  WORKFLOW_PANEL_CLASS,
  WORKFLOW_TEMPLATE_DRAG_TYPE,
  WORKFLOW_VIEW_MODE_SET,
} from "./constants";
import { WorkflowCredentialManagerDialog } from "./components/workflow-credential-manager-dialog";
import { WorkflowNodeCredentialsPanel } from "./components/workflow-node-credentials-panel";
import type {
  DragState,
  EdgeDragState,
  PanState,
  WorkflowAIMessage,
  WorkflowAISuggestion,
  WorkflowDefinition,
  WorkflowDraft,
  WorkflowEdge,
  WorkflowNode,
} from "./types";
import {
  buildWorkflowAssistantPrompt,
  connectorNameForID,
  defaultDraft,
  edgePath,
  formatDuration,
  formatRunDate,
  normalizeDefinition,
  parseWorkflowSuggestion,
  resolveViewModeFromLocation,
  shouldRenderNodeField,
} from "./utils";

export default function WorkflowStudioPage() {
  const { currentTenantId } = useAppState();
  const [location, setLocation] = useLocation();
  const queryHydratedRef = useRef(false);
  const [viewMode, setViewMode] = useState<"list" | "editor">(() => resolveViewModeFromLocation(location));
  const [workflowSearch, setWorkflowSearch] = useState("");
  const [workflowPage, setWorkflowPage] = useState(1);
  const [workflowPageSize, setWorkflowPageSize] = useState<number>(WORKFLOW_PAGE_SIZE_OPTIONS[0]);
  const { data: workflowItems = [], isLoading: workflowCatalogLoading } = useWorkflowCatalog(currentTenantId);
  const createWorkflow = useCreateWorkflowCatalogItem();
  const updateWorkflow = useUpdateWorkflowCatalogItem();
  const deleteWorkflow = useDeleteWorkflowCatalogItem();
  const workflowTestRun = useWorkflowTestRun();
  const askAI = useAskAI();
  const { data: connectors = [], isLoading: connectorsLoading } = useOutboundConnectors(currentTenantId);

  const [selectedWorkflowID, setSelectedWorkflowID] = useState<string | null | undefined>(undefined);
  const [selectedWorkflowKind, setSelectedWorkflowKind] = useState<WorkflowCatalogKind>("workflows");
  const [draft, setDraft] = useState<WorkflowDraft>(defaultDraft());
  const [dirty, setDirty] = useState(false);
  const [selectedNodeID, setSelectedNodeID] = useState("");
  const [dragging, setDragging] = useState<DragState | null>(null);
  const [panning, setPanning] = useState<PanState | null>(null);
  const [edgeDrag, setEdgeDrag] = useState<EdgeDragState | null>(null);
  const [edgeTarget, setEdgeTarget] = useState("");
  const [edgeLabel, setEdgeLabel] = useState("next");
  const [viewportOffset, setViewportOffset] = useState({ x: 120, y: 80 });
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [isLibraryOpen, setIsLibraryOpen] = useState(false);
  const [isCanvasDropActive, setIsCanvasDropActive] = useState(false);
  const [isAIAssistantOpen, setIsAIAssistantOpen] = useState(false);
  const [aiPrompt, setAIPrompt] = useState("");
  const [aiSessionID, setAISessionID] = useState("");
  const [aiMessages, setAIMessages] = useState<WorkflowAIMessage[]>([]);
  const canvasRef = useRef<HTMLDivElement>(null);

  const selectedNodeTypeForCredentials =
    draft.definition.nodes.find((node) => node.id === selectedNodeID)?.type || "";

  const { data: workflowCredentials = [] } = useWorkflowCredentials(currentTenantId, {
    nodeType: selectedNodeTypeForCredentials,
  });
  const createWorkflowCredential = useCreateWorkflowCredential();
  const updateWorkflowCredential = useUpdateWorkflowCredential();
  const deleteWorkflowCredential = useDeleteWorkflowCredential();
  const [credentialManagerOpen, setCredentialManagerOpen] = useState(false);
  const [editingCredentialId, setEditingCredentialId] = useState<string | null>(null);
  const [credentialDraftName, setCredentialDraftName] = useState("");
  const [credentialDraftDescription, setCredentialDraftDescription] = useState("");
  const [credentialDraftFields, setCredentialDraftFields] = useState<Record<string, string>>({});

  const aiAbortRef = useRef<AbortController | null>(null);
  const hasPersistedWorkflow = Boolean(selectedWorkflowID);

  const {
    data: workflowRunHistory,
    isFetching: isWorkflowRunsFetching,
    refetch: refetchWorkflowRuns,
  } = useWorkflowRunHistory(selectedWorkflowID || "", 100);
  const workflowRuns = workflowRunHistory?.runs || [];
  const workflowRunsRetentionLimit = workflowRunHistory?.retentionLimit || 1000;

  const selectedNode = useMemo(
    () => draft.definition.nodes.find((node) => node.id === selectedNodeID),
    [draft.definition.nodes, selectedNodeID],
  );
  const selectedNodeTemplate = useMemo(
    () => (selectedNode ? workflowNodeTemplateById(selectedNode.type, WORKFLOW_NODE_LIBRARY) : null),
    [selectedNode],
  );
  const selectedConnectorID = String(selectedNode?.config?.connectorId || "");
  const selectedNodeNeedsConnectorMethods = Boolean(
    selectedNodeTemplate?.fields?.some((field) => field.type === "connector_method"),
  );
  const { data: connectorMethods = [] } = useConnectorMethods(selectedNodeNeedsConnectorMethods ? selectedConnectorID : "");

  const selectedNodeHasSensitiveFields = Boolean(
    selectedNodeTemplate?.fields?.some((field) => field.sensitive),
  );
  const selectedNodeCredentialId = String(selectedNode?.config?.credential_id || "");

  const showWorkflowPageSkeleton = useMinimumLoading(workflowCatalogLoading || connectorsLoading);
  const showWorkflowRunsSkeleton = useMinimumLoading(Boolean(selectedWorkflowID) && isWorkflowRunsFetching);

  useEffect(() => {
    const { params } = splitLocationPathAndSearch(location);
    const modeParam = String(params.get("mode") || "").trim().toLowerCase();
    if (WORKFLOW_VIEW_MODE_SET.has(modeParam as "list" | "editor")) {
      setViewMode(modeParam as "list" | "editor");
    }
    const searchParam = params.get("q");
    if (searchParam !== null) {
      setWorkflowSearch(searchParam);
    }
    const parsedPageSize = parsePositiveIntParam(params.get("page_size"), WORKFLOW_PAGE_SIZE_OPTIONS[0]);
    if (WORKFLOW_PAGE_SIZE_OPTIONS.includes(parsedPageSize as typeof WORKFLOW_PAGE_SIZE_OPTIONS[number])) {
      setWorkflowPageSize(parsedPageSize);
    }
    const parsedPage = parsePositiveIntParam(params.get("page"), 1);
    if (parsedPage >= 1) {
      setWorkflowPage(parsedPage);
    }
    const workflowIDParam = String(params.get("wf") || "").trim();
    if (workflowIDParam) {
      setSelectedWorkflowID(workflowIDParam);
    }
    const workflowKindParam = String(params.get("wf_kind") || "").trim();
    if (WORKFLOW_CATALOG_KIND_SET.has(workflowKindParam as WorkflowCatalogKind)) {
      setSelectedWorkflowKind(workflowKindParam as WorkflowCatalogKind);
    }
    queryHydratedRef.current = true;
  }, []);

  useEffect(() => {
    if (!queryHydratedRef.current) return;
    const nextLocation = applySearchPatch(location, {
      mode: viewMode !== "list" ? viewMode : undefined,
      q: workflowSearch.trim() || undefined,
      page: workflowPage > 1 ? String(workflowPage) : undefined,
      page_size: workflowPageSize !== WORKFLOW_PAGE_SIZE_OPTIONS[0] ? String(workflowPageSize) : undefined,
      wf: selectedWorkflowID ? String(selectedWorkflowID) : undefined,
      wf_kind: selectedWorkflowID && selectedWorkflowKind !== "workflows" ? selectedWorkflowKind : undefined,
    });
    const currentLocation =
      typeof window !== "undefined" ? `${window.location.pathname}${window.location.search}` : location;
    if (nextLocation !== currentLocation) {
      setLocation(nextLocation, { replace: true });
    }
  }, [
    viewMode,
    workflowSearch,
    workflowPage,
    workflowPageSize,
    selectedWorkflowID,
    selectedWorkflowKind,
    location,
    setLocation,
  ]);

  useEffect(() => {
    setWorkflowPage(1);
  }, [workflowSearch, workflowPageSize]);

  const filteredWorkflowItems = useMemo(() => {
    const needle = workflowSearch.trim().toLowerCase();
    if (!needle) {
      return workflowItems;
    }
    return workflowItems.filter((item: any) => {
      const name = String(item?.name || "").toLowerCase();
      return name.includes(needle);
    });
  }, [workflowItems, workflowSearch]);

  const workflowPageCount = Math.max(1, Math.ceil(filteredWorkflowItems.length / workflowPageSize));
  useEffect(() => {
    setWorkflowPage((prev) => Math.min(prev, workflowPageCount));
  }, [workflowPageCount]);

  const paginatedWorkflowItems = useMemo(() => {
    const offset = (workflowPage - 1) * workflowPageSize;
    return filteredWorkflowItems.slice(offset, offset + workflowPageSize);
  }, [filteredWorkflowItems, workflowPage, workflowPageSize]);

  const applyWorkflowItem = (item: any, selectedID: string | null = item?.id || null) => {
    if (!item) {
      setDraft(defaultDraft());
      setSelectedWorkflowKind("workflows");
      setSelectedWorkflowID(selectedID);
      setSelectedNodeID("");
      setDirty(false);
      return;
    }
    const definition = normalizeDefinition(item.definition);
    setDraft({
      name: item.name || "",
      description: item.description || "",
      status: item.status === "published" ? "published" : "draft",
      enabled: Boolean(item.enabled ?? item.is_enabled ?? item.isEnabled ?? item.status === "published"),
      definition,
    });
    setSelectedWorkflowKind((item.kind as WorkflowCatalogKind) || "workflows");
    setSelectedWorkflowID(selectedID);
    setSelectedNodeID("");
    setDirty(false);
  };

  const openWorkflowEditor = (item: any) => {
    if (dirty && selectedWorkflowID !== item.id) {
      toast.warning("Unsaved changes were replaced");
    }
    applyWorkflowItem(item, item.id);
    setViewMode("editor");
    setIsSettingsOpen(false);
    setIsLibraryOpen(false);
  };

  const startNewWorkflow = () => {
    if (dirty) {
      toast.warning("Unsaved changes were replaced");
    }
    applyWorkflowItem(null, null);
    setViewMode("editor");
    setIsSettingsOpen(true);
    setIsLibraryOpen(false);
  };

  const goToWorkflowList = () => {
    if (dirty) {
      toast.warning("Unsaved changes are not saved");
    }
    setViewMode("list");
    setIsSettingsOpen(false);
    setIsLibraryOpen(false);
  };

  useEffect(() => {
    if (selectedWorkflowID === undefined) {
      applyWorkflowItem(null, null);
      return;
    }
    if (selectedWorkflowID === null) {
      return;
    }
    const current = workflowItems.find((item: any) => item.id === selectedWorkflowID);
    if (!current) {
      applyWorkflowItem(null, null);
    }
  }, [workflowItems, selectedWorkflowID]);

  const boardPointFromClient = (clientX: number, clientY: number) => {
    const rect = canvasRef.current?.getBoundingClientRect();
    if (!rect) {
      return null;
    }
    return {
      x: clientX - rect.left - viewportOffset.x,
      y: clientY - rect.top - viewportOffset.y,
    };
  };

  const connectNodes = (sourceNodeId: string, targetNodeId: string, label = "next") => {
    if (!sourceNodeId || !targetNodeId || sourceNodeId === targetNodeId) {
      return false;
    }
    const alreadyExists = draft.definition.edges.some(
      (edge) => edge.source === sourceNodeId && edge.target === targetNodeId && (edge.label || "") === label,
    );
    if (alreadyExists) {
      toast.error("Connection already exists");
      return false;
    }
    updateDefinition((definition) => ({
      ...definition,
      edges: [
        ...definition.edges,
        {
          id: `edge_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
          source: sourceNodeId,
          target: targetNodeId,
          label,
        },
      ],
    }));
    return true;
  };

  useEffect(() => {
    if (!dragging && !panning && !edgeDrag) return;
    const onMove = (event: MouseEvent) => {
      if (dragging) {
        const boardPoint = boardPointFromClient(event.clientX, event.clientY);
        if (!boardPoint) return;
        const x = boardPoint.x - dragging.pointerOffsetX;
        const y = boardPoint.y - dragging.pointerOffsetY;
        setDraft((prev) => ({
          ...prev,
          definition: {
            ...prev.definition,
            nodes: prev.definition.nodes.map((node) =>
              node.id === dragging.nodeId
                ? { ...node, position: { x, y } }
                : node,
            ),
          },
        }));
        setDirty(true);
        return;
      }

      if (panning) {
        setViewportOffset({
          x: panning.startOffsetX + (event.clientX - panning.startClientX),
          y: panning.startOffsetY + (event.clientY - panning.startClientY),
        });
        return;
      }

      if (edgeDrag) {
        const boardPoint = boardPointFromClient(event.clientX, event.clientY);
        if (!boardPoint) return;
        setEdgeDrag((prev) => (prev ? { ...prev, pointer: boardPoint } : prev));
      }
    };
    const onUp = (event: MouseEvent) => {
      if (edgeDrag) {
        const targetElement = document.elementFromPoint(event.clientX, event.clientY) as HTMLElement | null;
        const targetNodeId = targetElement?.closest<HTMLElement>("[data-workflow-node-id]")?.dataset.workflowNodeId || "";
        if (targetNodeId) {
          connectNodes(edgeDrag.sourceNodeId, targetNodeId, "next");
        }
      }
      setDragging(null);
      setPanning(null);
      setEdgeDrag(null);
    };
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
    return () => {
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    };
  }, [dragging, panning, edgeDrag, viewportOffset, draft.definition.edges]);

  useEffect(() => {
    return () => {
      aiAbortRef.current?.abort();
      aiAbortRef.current = null;
    };
  }, []);

  useEffect(() => {
    if (isAIAssistantOpen) {
      return;
    }
    if (askAI.isPending) {
      aiAbortRef.current?.abort();
      aiAbortRef.current = null;
    }
  }, [isAIAssistantOpen, askAI.isPending]);

  
  const nodesById = useMemo(() => {
    const map = new Map<string, WorkflowNode>();
    draft.definition.nodes.forEach((node) => map.set(node.id, node));
    return map;
  }, [draft.definition.nodes]);

  const edgesWithNodes = useMemo(() => {
    return draft.definition.edges
      .map((edge) => {
        const source = nodesById.get(edge.source);
        const target = nodesById.get(edge.target);
        if (!source || !target) return null;
        return { ...edge, sourceNode: source, targetNode: target };
      })
      .filter(Boolean) as Array<WorkflowEdge & { sourceNode: WorkflowNode; targetNode: WorkflowNode }>;
  }, [draft.definition.edges, nodesById]);

  const updateDefinition = (updater: (definition: WorkflowDefinition) => WorkflowDefinition) => {
    setDraft((prev) => ({ ...prev, definition: updater(prev.definition) }));
    setDirty(true);
  };

  const addNode = (template: WorkflowNodeTemplate, position?: { x: number; y: number }) => {
    const id = `${template.id}_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
    const fallbackPosition = {
      x: 60 + (draft.definition.nodes.length % 3) * 260,
      y: 60 + Math.floor(draft.definition.nodes.length / 3) * 150,
    };
    const nodePosition = {
      x: Number.isFinite(position?.x) ? Number(position?.x) : fallbackPosition.x,
      y: Number.isFinite(position?.y) ? Number(position?.y) : fallbackPosition.y,
    };
    const nextNode: WorkflowNode = {
      id,
      type: template.id,
      label: template.title,
      position: nodePosition,
      config: { ...template.defaultConfig },
    };
    updateDefinition((definition) => ({
      ...definition,
      entryNodeId: definition.entryNodeId || id,
      nodes: [...definition.nodes, nextNode],
    }));
    setSelectedNodeID(id);
  };

  const addNodeFromTemplateID = (templateID: string, position?: { x: number; y: number }) => {
    const template = workflowNodeTemplateById(templateID, WORKFLOW_NODE_LIBRARY);
    if (!template) {
      toast.error("Unknown node template");
      return;
    }
    addNode(template, position);
  };

  const startTemplateDrag = (event: DragEvent<HTMLElement>, templateID: string) => {
    event.dataTransfer.effectAllowed = "copy";
    event.dataTransfer.setData(WORKFLOW_TEMPLATE_DRAG_TYPE, templateID);
    event.dataTransfer.setData("text/plain", templateID);
  };

  const dropTemplateOnCanvas = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    setIsCanvasDropActive(false);
    const templateID = event.dataTransfer.getData(WORKFLOW_TEMPLATE_DRAG_TYPE) || event.dataTransfer.getData("text/plain");
    if (!templateID) {
      return;
    }
    const boardPoint = boardPointFromClient(event.clientX, event.clientY);
    if (!boardPoint) {
      return;
    }
    addNodeFromTemplateID(templateID, {
      x: Math.max(16, boardPoint.x - NODE_WIDTH / 2),
      y: Math.max(16, boardPoint.y - NODE_HEIGHT / 2),
    });
  };

  const removeNode = (nodeId: string) => {
    updateDefinition((definition) => {
      const nodes = definition.nodes.filter((node) => node.id !== nodeId);
      const edges = definition.edges.filter((edge) => edge.source !== nodeId && edge.target !== nodeId);
      const entryNodeId = definition.entryNodeId === nodeId ? nodes[0]?.id || "" : definition.entryNodeId;
      return { ...definition, nodes, edges, entryNodeId };
    });
    if (selectedNodeID === nodeId) {
      setSelectedNodeID("");
    }
  };

  const saveNodeConfig = (nodeId: string, patch: Record<string, any>) => {
    updateDefinition((definition) => ({
      ...definition,
      nodes: definition.nodes.map((node) =>
        node.id === nodeId ? { ...node, config: { ...node.config, ...patch } } : node,
      ),
    }));
  };

  const saveNodeFieldValue = (nodeId: string, field: WorkflowNodeField, rawValue: any) => {
    let nextValue: any = rawValue;
    if (field.type === "number") {
      const parsed = Number(rawValue);
      nextValue = Number.isFinite(parsed) ? parsed : field.min ?? 0;
    } else if (field.type === "switch") {
      nextValue = Boolean(rawValue);
    }
    const patch: Record<string, any> = { [field.id]: nextValue };
    if (field.type === "connector" && field.id === "connectorId") {
      patch.methodId = "";
    }
    saveNodeConfig(nodeId, patch);
  };

  const saveNodeLabel = (nodeId: string, label: string) => {
    updateDefinition((definition) => ({
      ...definition,
      nodes: definition.nodes.map((node) => (node.id === nodeId ? { ...node, label } : node)),
    }));
  };


  const buildCredentialDraftFields = (template: WorkflowNodeTemplate | null, seed?: Record<string, string>) => {
    const baseFields: Record<string, string> = {};
    for (const field of template?.fields || []) {
      baseFields[field.id] = "";
    }
    return { ...baseFields, ...(seed || {}) };
  };

  const resetCredentialDraft = () => {
    setCredentialDraftName("");
    setCredentialDraftDescription("");
    setCredentialDraftFields({});
    setEditingCredentialId(null);
  };

  const closeCredentialManager = () => {
    setCredentialManagerOpen(false);
    resetCredentialDraft();
  };

  const handleOpenCredentialManager = (mode: "new" | "edit", credentialId?: string) => {
    if (!selectedNode || !selectedNodeTemplate) {
      toast.error("Select node to manage credentials");
      return;
    }
    if (mode === "edit" && credentialId) {
      const existing = (workflowCredentials as any[]).find((cred: any) => String(cred.id) === credentialId);
      if (existing) {
        setEditingCredentialId(String(existing.id));
        setCredentialDraftName(String(existing.name || ""));
        setCredentialDraftDescription(String(existing.description || ""));
        const existingFields = (existing.fields || {}) as Record<string, string>;
        setCredentialDraftFields(buildCredentialDraftFields(selectedNodeTemplate, existingFields));
      } else {
        resetCredentialDraft();
      }
    } else {
      setEditingCredentialId(null);
      setCredentialDraftName(`${selectedNodeTemplate.title} credentials`);
      setCredentialDraftDescription("");
      setCredentialDraftFields(buildCredentialDraftFields(selectedNodeTemplate));
    }
    setCredentialManagerOpen(true);
  };

  const handleCredentialFieldChange = (fieldId: string, value: string) => {
    setCredentialDraftFields((prev) => ({ ...prev, [fieldId]: value }));
  };

  const handleSaveCredential = () => {
    if (!selectedNode || !selectedNodeTemplate) {
      toast.error("Select node to save credentials");
      return;
    }
    const nodeType = selectedNode.type;
    const name = credentialDraftName.trim() || `${selectedNodeTemplate.title} credentials`;
    const description = credentialDraftDescription.trim();
    const fields = Object.fromEntries(
      Object.entries(credentialDraftFields).map(([key, value]) => [key, String(value ?? "").trim()]),
    );
    if (!name) {
      toast.error("Credential name is required");
      return;
    }
    if (editingCredentialId) {
      updateWorkflowCredential.mutate(
        { id: editingCredentialId, name, description, node_type: nodeType, fields },
        {
          onSuccess: () => {
            if (selectedNode && editingCredentialId) {
              saveNodeConfig(selectedNode.id, { credential_id: editingCredentialId });
            }
            toast.success("Credentials updated");
            closeCredentialManager();
          },
          onError: (error: any) => {
            toast.error(error?.message || "Failed to update credentials");
          },
        },
      );
    } else {
      createWorkflowCredential.mutate(
        { name, description, node_type: nodeType, fields },
        {
          onSuccess: (created: any) => {
            const id = String(created?.id || "").trim();
            if (selectedNode && id) {
              saveNodeConfig(selectedNode.id, { credential_id: id });
            }
            toast.success("Credentials created");
            closeCredentialManager();
          },
          onError: (error: any) => {
            toast.error(error?.message || "Failed to create credentials");
          },
        },
      );
    }
  };

  const handleDeleteCredential = () => {
    if (!editingCredentialId) {
      closeCredentialManager();
      return;
    }
    deleteWorkflowCredential.mutate(editingCredentialId, {
      onSuccess: () => {
        if (selectedNode && selectedNodeCredentialId === editingCredentialId) {
          saveNodeConfig(selectedNode.id, { credential_id: "" });
        }
        toast.success("Credentials deleted");
        closeCredentialManager();
      },
      onError: (error: any) => {
        toast.error(error?.message || "Failed to delete credentials");
      },
    });
  };

  const applyWorkflowSwitch = (patch: { status?: "draft" | "published"; enabled?: boolean }) => {
    setDraft((prev) => {
      const next: WorkflowDraft = {
        ...prev,
        ...(patch.status !== undefined ? { status: patch.status } : {}),
        ...(patch.enabled !== undefined ? { enabled: patch.enabled } : {}),
      };
      return next;
    });

    if (!selectedWorkflowID) {
      setDirty(true);
      return;
    }

    const updatePatch: Record<string, any> = {};
    if (patch.status !== undefined) {
      updatePatch.status = patch.status;
    }
    if (patch.enabled !== undefined) {
      updatePatch.enabled = patch.enabled;
    }
    if (Object.keys(updatePatch).length === 0) {
      return;
    }

    updateWorkflow.mutate(
      { id: selectedWorkflowID, kind: selectedWorkflowKind, data: updatePatch },
      {
        onError: (error: any) => {
          setDirty(true);
          toast.error(error?.message || "Failed to update workflow switch");
        },
      },
    );
  };

  const connectorsForNode = (nodeType: string) => {
    if (nodeType === "telegram_send") {
      return connectors.filter((item: any) => String(item?.channel || "").toLowerCase() === "telegram");
    }
    if (nodeType === "webhook_trigger") {
      return connectors.filter((item: any) => String(item?.channel || "").toLowerCase() === "webhook");
    }
    if (nodeType === "kafka_trigger") {
      return connectors.filter((item: any) => String(item?.channel || "").toLowerCase() === "kafka");
    }
    return connectors;
  };

  const addEdge = () => {
    if (!selectedNode || !edgeTarget || selectedNode.id === edgeTarget) {
      return;
    }
    if (connectNodes(selectedNode.id, edgeTarget, edgeLabel)) {
      setEdgeTarget("");
    }
  };

  const removeEdge = (edgeId: string) => {
    updateDefinition((definition) => ({
      ...definition,
      edges: definition.edges.filter((edge) => edge.id !== edgeId),
    }));
  };

  const saveWorkflow = () => {
    const name = draft.name.trim();
    if (!name) {
      toast.error("Workflow name is required");
      return;
    }
    if (draft.definition.nodes.length === 0) {
      toast.error("Add at least one node");
      return;
    }
    const normalizedDefinition = normalizeDefinition(draft.definition);
    const payload: Record<string, any> = {
      name,
      description: draft.description.trim(),
      status: draft.status,
      enabled: draft.enabled,
      type: "workflow",
      definition: normalizedDefinition,
    };
    if (selectedWorkflowID) {
      updateWorkflow.mutate(
        { id: selectedWorkflowID, kind: selectedWorkflowKind, data: payload },
        {
          onSuccess: () => {
            toast.success("Workflow updated");
            setDirty(false);
          },
          onError: (error: any) => toast.error(error?.message || "Failed to update workflow"),
        },
      );
      return;
    }
    createWorkflow.mutate(payload, {
      onSuccess: (created: any) => {
        setSelectedWorkflowID(created?.id || undefined);
        setSelectedWorkflowKind((created?.kind as WorkflowCatalogKind) || "workflows");
        toast.success("Workflow created");
        setDirty(false);
      },
      onError: (error: any) => toast.error(error?.message || "Failed to create workflow"),
    });
  };

  const runTestWorkflow = () => {
    if (!selectedWorkflowID) {
      toast.error("Save workflow first to run a test");
      return;
    }
    workflowTestRun.mutate(
      {
        workflowID: selectedWorkflowID,
        data: {
          input: {
            source: "workflow_studio_manual",
            workflow_type: "workflow",
          },
          definition: normalizeDefinition(draft.definition),
        },
      },
      {
        onSuccess: (response: any) => {
          if (response?.ok) {
            toast.success("Workflow test run completed");
          } else {
            toast.warning("Workflow test run finished with errors");
          }
          refetchWorkflowRuns();
        },
        onError: (error: any) => toast.error(error?.message || "Failed to run workflow test"),
      },
    );
  };

  const deleteSelectedWorkflow = () => {
    if (!selectedWorkflowID) {
      applyWorkflowItem(null, null);
      return;
    }
    deleteWorkflow.mutate({ id: selectedWorkflowID, kind: selectedWorkflowKind }, {
      onSuccess: () => {
        toast.success("Workflow deleted");
        setSelectedWorkflowID(undefined);
        setSelectedWorkflowKind("workflows");
        setDraft(defaultDraft());
        setSelectedNodeID("");
        setDirty(false);
        setViewMode("list");
      },
      onError: (error: any) => toast.error(error?.message || "Failed to delete workflow"),
    });
  };

  const availableAINodeTypes = useMemo(
    () => WORKFLOW_NODE_LIBRARY.map((node) => node.id).sort((left, right) => left.localeCompare(right)),
    [],
  );
  const allowedAIWorkflowNodeTypes = useMemo(
    () => new Set(availableAINodeTypes),
    [availableAINodeTypes],
  );

  const applyAISuggestion = (suggestion: WorkflowAISuggestion) => {
    if (!suggestion) {
      return;
    }
    if (dirty) {
      toast.warning("Unsaved changes were replaced by AI draft");
    }
    setSelectedWorkflowID(null);
    setSelectedWorkflowKind("workflows");
    setDraft({
      name: suggestion.name || "AI workflow draft",
      description: suggestion.description || "",
      status: "draft",
      enabled: false,
      definition: normalizeDefinition(suggestion.definition),
    });
    setSelectedNodeID("");
    setViewMode("editor");
    setDirty(true);
    setIsAIAssistantOpen(false);
    toast.success("AI workflow draft applied");
  };

  const sendAIPrompt = async () => {
    const prompt = aiPrompt.trim();
    if (!prompt) {
      toast.error("Prompt is required");
      return;
    }
    if (askAI.isPending) {
      return;
    }
    const userMessage: WorkflowAIMessage = {
      id: `ai_user_${Date.now()}_${Math.random().toString(36).slice(2, 8)}` ,
      role: "user",
      content: prompt,
      createdAt: Date.now(),
    };
    setAIMessages((prev) => [...prev, userMessage]);
    setAIPrompt("");
    const controller = new AbortController();
    aiAbortRef.current = controller;
    try {
      const response: any = await askAI.mutateAsync({
        question: buildWorkflowAssistantPrompt(prompt, availableAINodeTypes),
        sessionId: aiSessionID || undefined,
        signal: controller.signal,
      });
      const answer = String(response?.answer || "").trim();
      if (!answer) {
        throw new Error("AI returned empty response");
      }
      const suggestion = parseWorkflowSuggestion(answer, allowedAIWorkflowNodeTypes);
      const assistantMessage: WorkflowAIMessage = {
        id: `ai_assistant_${Date.now()}_${Math.random().toString(36).slice(2, 8)}` ,
        role: "assistant",
        content: answer,
        createdAt: Date.now(),
        suggestion: suggestion || undefined,
      };
      setAIMessages((prev) => [...prev, assistantMessage]);
      const returnedSession = String(response?.session_id || "").trim();
      if (returnedSession) {
        setAISessionID(returnedSession);
      }
      if (!suggestion) {
        toast.warning("AI response did not contain valid workflow JSON");
      }
    } catch (error: any) {
      if (error?.name === "AbortError") {
        toast.message("AI request canceled");
      } else {
        toast.error(error?.message || "Failed to generate workflow from AI prompt");
      }
    } finally {
      if (aiAbortRef.current === controller) {
        aiAbortRef.current = null;
      }
    }
  };

  const cancelAIPrompt = () => {
    aiAbortRef.current?.abort();
    aiAbortRef.current = null;
  };

  const nodeCategoryGroups = useMemo(() => {
    return groupWorkflowNodeLibraryByCategory(WORKFLOW_NODE_LIBRARY);
  }, []);

  const selectedNodeOutgoingEdges = useMemo(
    () => draft.definition.edges.filter((edge) => edge.source === selectedNodeID),
    [draft.definition.edges, selectedNodeID],
  );

  if (showWorkflowPageSkeleton) {
    return (
      <AppLayout>
        <div className={WORKFLOW_PAGE_SHELL_CLASS}>
          <div className="space-y-2">
            <Skeleton className="h-8 w-64 rounded-lg" />
            <Skeleton className="h-4 w-80 rounded-lg" />
          </div>
          <div className={`${WORKFLOW_PANEL_CLASS} p-4`}>
            <div className="flex flex-wrap items-center gap-2">
              <Skeleton className="h-10 w-32 rounded-lg" />
              <Skeleton className="h-10 w-32 rounded-lg" />
              <Skeleton className="h-10 w-40 rounded-lg" />
            </div>
            <div className="mt-4 grid gap-4 lg:grid-cols-[340px_1fr]">
              <Skeleton className="h-[520px] w-full rounded-xl" />
              <Skeleton className="h-[520px] w-full rounded-xl" />
            </div>
          </div>
        </div>
      </AppLayout>
    );
  }

  return (
    <AppLayout>
      <div className={WORKFLOW_PAGE_SHELL_CLASS}>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h1 className="flex items-center gap-2 text-[32px] font-semibold leading-8 tracking-[-0.5px] text-white" data-testid="text-workflow-studio-title">
              <Workflow className="text-[#facc15]" size={30} />
              Workflow Studio
            </h1>
            <p className={`mt-1 text-sm ${WORKFLOW_MUTED_TEXT_CLASS}`}>
              Build and run workflows from blocks, inspired by n8n-style node graphs.
            </p>
          </div>
          {viewMode === "list" ? (
            <div className="flex w-full lg:w-auto flex-wrap items-center gap-2">
              <Button
                variant="outline"
                className={`w-full sm:w-auto ${WORKFLOW_ACTION_BUTTON_CLASS}`}
                onClick={() => setIsAIAssistantOpen(true)}
                data-testid="button-open-workflow-ai-assistant"
              >
                <Bot size={16} className="mr-2" />
                AI Agent
              </Button>
              <Button className="w-full sm:w-auto" onClick={startNewWorkflow} data-testid="button-create-workflow">
                <Plus size={16} className="mr-2" />
                Create workflow
              </Button>
            </div>
          ) : (
            <div className="flex w-full lg:w-auto flex-wrap items-center gap-2">
              <Button
                variant="outline"
                className={`w-full sm:w-auto ${WORKFLOW_ACTION_BUTTON_CLASS}`}
                onClick={() => setIsAIAssistantOpen(true)}
                data-testid="button-open-workflow-ai-assistant"
              >
                <Bot size={16} className="mr-2" />
                AI Agent
              </Button>
              <Button variant="outline" className={`w-full sm:w-auto ${WORKFLOW_ACTION_BUTTON_CLASS}`} onClick={goToWorkflowList} data-testid="button-back-to-workflows">
                <ArrowLeft size={16} className="mr-2" />
                To workflows
              </Button>
              <Badge variant={dirty ? "destructive" : "secondary"}>{dirty ? "Unsaved changes" : "Saved"}</Badge>
              <Button
                variant="outline"
                className={`w-full sm:w-auto ${WORKFLOW_ACTION_BUTTON_CLASS}`}
                onClick={startNewWorkflow}
                data-testid="button-new-workflow"
              >
                <Plus size={16} className="mr-2" />
                New workflow
              </Button>
              <Button
                className="w-full sm:w-auto"
                onClick={saveWorkflow}
                disabled={createWorkflow.isPending || updateWorkflow.isPending}
                data-testid="button-save-workflow"
              >
                <Save size={16} className="mr-2" />
                Save
              </Button>
              <Button
                variant="secondary"
                className="w-full sm:w-auto"
                onClick={runTestWorkflow}
                disabled={!hasPersistedWorkflow || workflowTestRun.isPending}
                data-testid="button-test-workflow"
              >
                <Play size={16} className="mr-2" />
                Test run
              </Button>
              <Button
                variant="destructive"
                className="w-full sm:w-auto"
                onClick={deleteSelectedWorkflow}
                disabled={!selectedWorkflowID || deleteWorkflow.isPending}
                data-testid="button-delete-workflow"
              >
                <Trash2 size={16} className="mr-2" />
                Delete workflow
              </Button>
            </div>
          )}
        </div>

        <Card className={`${WORKFLOW_PANEL_CLASS} p-2`}>
          <div className="flex flex-wrap items-center gap-2">
            <p className={`text-xs ${WORKFLOW_MUTED_TEXT_CLASS}`}>
              Native builder mode uses IncidentHub blocks and runtime.
            </p>
          </div>
        </Card>

        {viewMode === "list" ? (
          <Card className={`${WORKFLOW_PANEL_CLASS} p-4 space-y-4`} data-testid="card-workflow-list">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex w-full flex-col gap-2 sm:flex-row sm:items-center">
                <Input
                  value={workflowSearch}
                  onChange={(event) => setWorkflowSearch(event.target.value)}
                  placeholder="Search workflows by name"
                  className="w-full sm:max-w-md"
                  data-testid="input-workflow-filter"
                />
                <Select
                  value={String(workflowPageSize)}
                  onValueChange={(value) => setWorkflowPageSize(Number(value) || WORKFLOW_PAGE_SIZE_OPTIONS[0])}
                >
                  <SelectTrigger className="w-full sm:w-40" data-testid="select-workflow-page-size">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {WORKFLOW_PAGE_SIZE_OPTIONS.map((size) => (
                      <SelectItem key={size} value={String(size)}>
                        {size} per page
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <Badge variant="outline" className="border-[#2a2c3c] bg-[#0f131d] text-[#9ca3af]">{filteredWorkflowItems.length} workflows</Badge>
            </div>
            <div className="space-y-2">
              {filteredWorkflowItems.length === 0 ? (
                <div className="rounded-lg border border-dashed border-[#2a2c3c] bg-[#111622] p-6 text-center text-sm text-[#9ca3af]">
                  {workflowItems.length === 0 ? "No workflows yet" : "No workflows match current filter"}
                </div>
              ) : (
                paginatedWorkflowItems.map((item: any) => (
                  <button
                    key={item.id}
                    className="w-full rounded-lg border border-[#2a2c3c] bg-[#111622] p-3 text-left transition-colors hover:border-[#4b5168] hover:bg-[#171b2a]"
                    onClick={() => openWorkflowEditor(item)}
                    data-testid={`item-workflow-${item.id}`}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <EllipsisText text={item.name} className="font-medium max-w-[280px]" />
                      <div className="flex items-center gap-1">
                        <Badge variant={item.status === "published" ? "default" : "secondary"}>{item.status}</Badge>
                        <Badge variant={item.enabled ? "default" : "outline"}>{item.enabled ? "enabled" : "disabled"}</Badge>
                      </div>
                    </div>
                    <EllipsisText text={item.description || "No description"} className="text-xs text-[#9ca3af] mt-1 max-w-[360px]" />
                    <p className="text-[11px] text-[#9ca3af] mt-2">
                      Nodes: {item.definition?.nodes?.length || 0} • Edges: {item.definition?.edges?.length || 0}
                    </p>
                  </button>
                ))
              )}
            </div>
            {filteredWorkflowItems.length > 0 && (
              <div className="flex flex-col gap-3 border-t border-[#2a2c3c] pt-3 sm:flex-row sm:items-center sm:justify-between">
                <p className="text-xs text-[#9ca3af]">
                  Showing {(workflowPage - 1) * workflowPageSize + 1}
                  {" - "}
                  {Math.min(workflowPage * workflowPageSize, filteredWorkflowItems.length)}
                  {" of "}
                  {filteredWorkflowItems.length}
                </p>
                <div className="flex items-center gap-1">
                  <Button
                    variant="outline"
                    className={WORKFLOW_ACTION_BUTTON_CLASS}
                    size="sm"
                    onClick={() => setWorkflowPage((prev) => Math.max(1, prev - 1))}
                    disabled={workflowPage <= 1}
                    data-testid="button-workflow-prev-page"
                  >
                    Prev
                  </Button>
                  {Array.from({ length: workflowPageCount }, (_, index) => index + 1).map((page) => (
                    <Button
                      key={page}
                      variant={page === workflowPage ? "default" : "outline"}
                      className={page === workflowPage ? "" : WORKFLOW_ACTION_BUTTON_CLASS}
                      size="sm"
                      onClick={() => setWorkflowPage(page)}
                      data-testid={`button-workflow-page-${page}`}
                    >
                      {page}
                    </Button>
                  ))}
                  <Button
                    variant="outline"
                    className={WORKFLOW_ACTION_BUTTON_CLASS}
                    size="sm"
                    onClick={() => setWorkflowPage((prev) => Math.min(workflowPageCount, prev + 1))}
                    disabled={workflowPage >= workflowPageCount}
                    data-testid="button-workflow-next-page"
                  >
                    Next
                  </Button>
                </div>
              </div>
            )}
          </Card>
        ) : (
          <div className="grid grid-cols-1 gap-4 xl:grid-cols-12">
            <Card className={`${WORKFLOW_PANEL_CLASS} ${selectedNode ? "xl:col-span-9" : "xl:col-span-12"} p-0 overflow-hidden`}>
              <div className="border-b border-[#2a2c3c] px-4 py-3 flex flex-wrap items-center justify-between gap-3">
                <div className="flex min-w-0 flex-col items-start gap-1">
                  <EllipsisText text={draft.name.trim() || "Untitled workflow"} className="max-w-[460px] font-semibold text-sm" />
                  <Badge variant={dirty ? "destructive" : "secondary"}>{dirty ? "Unsaved changes" : "Saved"}</Badge>
                  <p className="text-xs text-[#9ca3af]">
                    Nodes: {draft.definition.nodes.length} • Edges: {draft.definition.edges.length}
                  </p>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <Popover open={isSettingsOpen} onOpenChange={setIsSettingsOpen}>
                    <PopoverTrigger asChild>
                      <Button variant="outline" className={WORKFLOW_ACTION_BUTTON_CLASS} data-testid="button-open-workflow-settings">
                        <Settings2 size={15} className="mr-2" />
                        Workflow settings
                      </Button>
                    </PopoverTrigger>
                    <PopoverContent
                      align="end"
                      className="w-[min(92vw,520px)] space-y-3 border-[#2a2c3c] bg-[#111622]"
                      data-testid="popover-workflow-settings"
                    >
                      <div className="space-y-1">
                        <h3 className="text-sm font-semibold">Workflow settings</h3>
                        <p className="text-xs text-[#9ca3af]">Configure workflow metadata.</p>
                      </div>
                      <div className="space-y-2">
                        <Label htmlFor="workflow-name">Workflow name</Label>
                        <Input
                          id="workflow-name"
                          value={draft.name}
                          onChange={(event) => {
                            setDraft((prev) => ({ ...prev, name: event.target.value }));
                            setDirty(true);
                          }}
                          placeholder="Incident triage and notify"
                          data-testid="input-workflow-name"
                        />
                      </div>
                      <div className="space-y-2">
                        <Label htmlFor="workflow-description">Description</Label>
                        <Textarea
                          id="workflow-description"
                          className="min-h-24"
                          value={draft.description}
                          onChange={(event) => {
                            setDraft((prev) => ({ ...prev, description: event.target.value }));
                            setDirty(true);
                          }}
                          placeholder="What this workflow does and when it should be used"
                          data-testid="textarea-workflow-description"
                        />
                      </div>
                      <div className="space-y-2">
                        <div className="flex items-center justify-between rounded-lg border border-[#2a2c3c] bg-[#0f131d] px-3 py-2">
                          <div className="text-sm">Published</div>
                          <Switch
                            checked={draft.status === "published"}
                            disabled={updateWorkflow.isPending}
                            onCheckedChange={(checked) => applyWorkflowSwitch({ status: checked ? "published" : "draft" })}
                            data-testid="switch-workflow-published"
                          />
                        </div>
                        <div className="flex items-center justify-between rounded-lg border border-[#2a2c3c] bg-[#0f131d] px-3 py-2">
                          <div className="text-sm">Enabled</div>
                          <Switch
                            checked={draft.enabled}
                            disabled={updateWorkflow.isPending}
                            onCheckedChange={(checked) => applyWorkflowSwitch({ enabled: checked })}
                            data-testid="switch-workflow-enabled"
                          />
                        </div>
                      </div>
                    </PopoverContent>
                  </Popover>

                  <Popover open={isLibraryOpen} onOpenChange={setIsLibraryOpen}>
                    <PopoverTrigger asChild>
                      <Button variant="outline" className={WORKFLOW_ACTION_BUTTON_CLASS} data-testid="button-open-block-library">
                        <Blocks size={15} className="mr-2" />
                        Block library
                      </Button>
                    </PopoverTrigger>
                    <PopoverContent
                      align="end"
                      className="w-[min(92vw,420px)] border-[#2a2c3c] bg-[#111622] p-3"
                      data-testid="popover-node-library"
                    >
                      <div className="space-y-3">
                        <div className="space-y-1">
                          <h3 className="text-sm font-semibold">Block Library</h3>
                          <p className="text-xs text-[#9ca3af]">
                            Click to add block or drag it onto canvas.
                          </p>
                        </div>
                        <div className="space-y-3 max-h-[60vh] overflow-y-auto pr-1">
                          {Object.entries(nodeCategoryGroups).map(([category, templates]) => (
                            <div key={category} className="space-y-2">
                              <p className="text-xs font-semibold uppercase text-[#9ca3af]">{category}</p>
                              {templates.map((template) => {
                                const nodeVisual = workflowNodeVisual(template.id, template.category);
                                const NodeIcon = nodeVisual.icon;
                                return (
                                  <button
                                    key={template.id}
                                    className="w-full rounded-lg border border-[#2a2c3c] bg-[#0f131d] p-2.5 text-left transition-colors hover:border-[#4b5168] hover:bg-[#171b2a]"
                                    onClick={() => addNodeFromTemplateID(template.id)}
                                    draggable
                                    onDragStart={(event) => startTemplateDrag(event, template.id)}
                                    data-testid={`button-add-node-${template.id}`}
                                  >
                                    <div className="flex items-start gap-2.5">
                                      <span className={`inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md border ${nodeVisual.badgeClass}`}>
                                        <NodeIcon size={14} />
                                      </span>
                                      <div className="min-w-0">
                                        <div className="font-medium text-sm">{template.title}</div>
                                        <p className="text-xs text-[#9ca3af]">{template.description}</p>
                                      </div>
                                    </div>
                                  </button>
                                );
                              })}
                            </div>
                          ))}
                        </div>
                      </div>
                    </PopoverContent>
                  </Popover>
                </div>
              </div>

              <div
                ref={canvasRef}
                className={`relative h-[700px] overflow-hidden select-none bg-[radial-gradient(circle_at_1px_1px,hsl(var(--muted-foreground)/0.2)_1px,transparent_0)] [background-size:22px_22px] ${
                  isCanvasDropActive ? "ring-2 ring-[#facc15]/45 ring-inset" : ""
                }`}
                style={{ backgroundPosition: `${viewportOffset.x}px ${viewportOffset.y}px` }}
                onMouseDown={(event) => {
                  if (event.button !== 0) return;
                  const target = event.target as HTMLElement;
                  if (target.closest("[data-workflow-node-id]")) {
                    return;
                  }
                  setSelectedNodeID("");
                  setPanning({
                    startClientX: event.clientX,
                    startClientY: event.clientY,
                    startOffsetX: viewportOffset.x,
                    startOffsetY: viewportOffset.y,
                  });
                }}
                onDragOver={(event) => {
                  event.preventDefault();
                  event.dataTransfer.dropEffect = "copy";
                  setIsCanvasDropActive(true);
                }}
                onDragLeave={() => setIsCanvasDropActive(false)}
                onDrop={dropTemplateOnCanvas}
                data-testid="workflow-canvas"
              >
              <svg className="absolute inset-0 w-full h-full pointer-events-none">
                {edgesWithNodes.map((edge) => (
                  <g key={edge.id}>
                    <path
                      d={edgePath(
                        edge.sourceNode.position.x + viewportOffset.x + NODE_WIDTH,
                        edge.sourceNode.position.y + viewportOffset.y + NODE_HEIGHT / 2,
                        edge.targetNode.position.x + viewportOffset.x,
                        edge.targetNode.position.y + viewportOffset.y + NODE_HEIGHT / 2,
                      )}
                      fill="none"
                      stroke="hsl(var(--primary) / 0.55)"
                      strokeWidth={2}
                    />
                    {edge.label && (
                      <text
                        x={(edge.sourceNode.position.x + edge.targetNode.position.x + NODE_WIDTH + viewportOffset.x * 2) / 2}
                        y={(edge.sourceNode.position.y + edge.targetNode.position.y + NODE_HEIGHT + viewportOffset.y * 2) / 2 - 6}
                        fontSize="11"
                        fill="hsl(var(--muted-foreground))"
                      >
                        {edge.label}
                      </text>
                    )}
                  </g>
                ))}
                {edgeDrag && nodesById.get(edgeDrag.sourceNodeId) && (
                  <path
                    d={edgePath(
                      nodesById.get(edgeDrag.sourceNodeId)!.position.x + viewportOffset.x + NODE_WIDTH,
                      nodesById.get(edgeDrag.sourceNodeId)!.position.y + viewportOffset.y + NODE_HEIGHT / 2,
                      edgeDrag.pointer.x + viewportOffset.x,
                      edgeDrag.pointer.y + viewportOffset.y,
                    )}
                    fill="none"
                    stroke="hsl(var(--primary))"
                    strokeWidth={2}
                    strokeDasharray="6 6"
                  />
                )}
              </svg>

              {draft.definition.nodes.map((node) => {
                const isEntry = draft.definition.entryNodeId === node.id;
                const nodeTemplate = workflowNodeTemplateById(node.type, WORKFLOW_NODE_LIBRARY);
                const nodeVisual = workflowNodeVisual(node.type, nodeTemplate?.category);
                const NodeIcon = nodeVisual.icon;
                const nodeSummary = workflowNodeSummary(
                  nodeTemplate,
                  node.config,
                  (connectorId) => connectorNameForID(connectors, connectorId),
                );
                return (
                  <div
                    key={node.id}
                    className={`absolute z-10 rounded-lg border border-[#2a2c3c] bg-[#111622] px-3 py-2 text-left shadow-sm transition-colors ${
                      selectedNodeID === node.id ? "border-[#facc15] ring-2 ring-[#facc15]/25" : "hover:border-[#4b5168]"
                    }`}
                    style={{
                      left: node.position.x + viewportOffset.x,
                      top: node.position.y + viewportOffset.y,
                      width: NODE_WIDTH,
                      minHeight: NODE_HEIGHT,
                    }}
                    onMouseDown={(event) => {
                      if (event.button !== 0) return;
                      const target = event.target as HTMLElement;
                      if (target.closest("[data-workflow-handle='output']")) {
                        return;
                      }
                      const boardPoint = boardPointFromClient(event.clientX, event.clientY);
                      if (!boardPoint) return;
                      setDragging({
                        nodeId: node.id,
                        pointerOffsetX: boardPoint.x - node.position.x,
                        pointerOffsetY: boardPoint.y - node.position.y,
                      });
                    }}
                    onClick={() => setSelectedNodeID(node.id)}
                    data-testid={`workflow-node-${node.id}`}
                    data-workflow-node-id={node.id}
                  >
                    <span className={`absolute -left-2 top-1/2 h-3 w-3 -translate-y-1/2 rounded-full border bg-[#0f131d] ${nodeVisual.handleClass}`} />
                    <span
                      className={`absolute -right-2 top-1/2 h-4 w-4 -translate-y-1/2 cursor-crosshair rounded-full border ${nodeVisual.handleClass}`}
                      onMouseDown={(event) => {
                        if (event.button !== 0) return;
                        event.preventDefault();
                        event.stopPropagation();
                        const boardPoint = boardPointFromClient(event.clientX, event.clientY);
                        if (!boardPoint) return;
                        setSelectedNodeID(node.id);
                        setEdgeDrag({ sourceNodeId: node.id, pointer: boardPoint });
                      }}
                      data-workflow-handle="output"
                      data-testid={`workflow-node-handle-${node.id}`}
                    />
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex min-w-0 items-center gap-2">
                        <span className={`inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md border ${nodeVisual.badgeClass}`}>
                          <NodeIcon size={13} />
                        </span>
                        <EllipsisText text={node.label} className="font-semibold text-sm max-w-[145px]" />
                      </div>
                      <Badge variant="outline" className="text-[10px]">{node.type}</Badge>
                    </div>
                    {isEntry && (
                      <p className="text-[10px] text-[#facc15] font-semibold mt-1 flex items-center gap-1">
                        <Play size={10} />
                        Entry node
                      </p>
                    )}
                    <EllipsisText text={nodeSummary} className="text-[11px] text-[#9ca3af] mt-1 max-w-[200px]" />
                  </div>
                );
              })}
              <div className="pointer-events-none absolute bottom-2 right-2 rounded-md border bg-[#0f131d]/85 px-2 py-1 text-[11px] text-[#9ca3af]">
                Drag canvas to pan • Drag block from library to place • Drag right handle to connect
              </div>
            </div>
          </Card>

          {selectedNode && (
          <Card className={`${WORKFLOW_PANEL_CLASS} xl:col-span-3 p-4 space-y-4`}>
            <div className="flex items-center justify-between">
              <h2 className="font-semibold text-sm uppercase text-[#9ca3af]">Inspector</h2>
            </div>
                <div className="space-y-2">
                  <Label>Node label</Label>
                  <Input
                    value={selectedNode.label}
                    onChange={(event) => saveNodeLabel(selectedNode.id, event.target.value)}
                    data-testid="input-node-label"
                  />
                </div>
                <WorkflowNodeCredentialsPanel
                  visible={selectedNodeHasSensitiveFields}
                  credentialId={selectedNodeCredentialId}
                  credentials={workflowCredentials}
                  actionButtonClass={WORKFLOW_ACTION_BUTTON_CLASS}
                  onSelectCredential={(credentialId) => saveNodeConfig(selectedNode.id, { credential_id: credentialId })}
                  onManageCredential={handleOpenCredentialManager}
                />

                <div className="space-y-3">
                  {(selectedNodeTemplate?.fields || [])
                    .filter((field) => shouldRenderNodeField(field, selectedNode.config || {}))
                    .map((field) => {
                      const currentValue = selectedNode.config?.[field.id];
                      if (field.type === "switch") {
                        return (
                          <div key={field.id} className="flex items-center justify-between rounded-lg border px-3 py-2">
                            <div className="min-w-0">
                              <div className="text-sm">{field.label}</div>
                              {field.description && <p className="text-xs text-[#9ca3af]">{field.description}</p>}
                            </div>
                            <Switch
                              checked={Boolean(currentValue)}
                              onCheckedChange={(checked) => saveNodeFieldValue(selectedNode.id, field, checked)}
                            />
                          </div>
                        );
                      }

                      if (field.type === "select") {
                        return (
                          <div key={field.id} className="space-y-2">
                            <Label>{field.label}</Label>
                            <Select
                              value={String(currentValue ?? field.options?.[0]?.value ?? "")}
                              onValueChange={(value) => saveNodeFieldValue(selectedNode.id, field, value)}
                            >
                              <SelectTrigger data-testid={`select-node-field-${field.id}`}>
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                {(field.options || []).map((option) => (
                                  <SelectItem key={option.value} value={option.value}>
                                    {option.label}
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                            {field.description && <p className="text-xs text-[#9ca3af]">{field.description}</p>}
                          </div>
                        );
                      }

                      if (field.type === "connector") {
                        const fieldConnectors = connectorsForNode(selectedNode.type);
                        return (
                          <div key={field.id} className="space-y-2">
                            <Label>{field.label}</Label>
                            <Select
                              value={String(currentValue || "")}
                              onValueChange={(value) => saveNodeFieldValue(selectedNode.id, field, value)}
                            >
                              <SelectTrigger data-testid="select-node-connector">
                                <SelectValue placeholder="Select connector" />
                              </SelectTrigger>
                              <SelectContent>
                                {fieldConnectors.map((connector: any) => (
                                  <SelectItem key={connector.id} value={connector.id}>
                                    <EllipsisText text={connector.name} className="max-w-[220px]" />
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                            {field.description && <p className="text-xs text-[#9ca3af]">{field.description}</p>}
                          </div>
                        );
                      }

                      if (field.type === "connector_method") {
                        return (
                          <div key={field.id} className="space-y-2">
                            <Label>{field.label}</Label>
                            <Select
                              value={String(currentValue || "")}
                              onValueChange={(value) => saveNodeFieldValue(selectedNode.id, field, value)}
                            >
                              <SelectTrigger data-testid="select-node-method">
                                <SelectValue placeholder="Select method" />
                              </SelectTrigger>
                              <SelectContent>
                                {connectorMethods.map((method: any) => (
                                  <SelectItem key={method.id} value={method.id}>
                                    <EllipsisText text={method.name || method.id} className="max-w-[220px]" />
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                            {field.description && <p className="text-xs text-[#9ca3af]">{field.description}</p>}
                          </div>
                        );
                      }

                      if (field.sensitive) {
                        return (
                          <div key={field.id} className="space-y-2">
                            <Label>{field.label}</Label>
                            <div className="rounded-md border border-[#2a2c3c] bg-[#0f131d] px-3 py-2 text-xs text-[#9ca3af]">
                              Sensitive value is managed by credentials for this node. Select or create credentials above.
                            </div>
                            {field.description && <p className="text-xs text-[#9ca3af]">{field.description}</p>}
                          </div>
                        );
                      }

                      if (field.type === "textarea" || field.type === "json") {
                        return (
                          <div key={field.id} className="space-y-2">
                            <Label>{field.label}</Label>
                            <Textarea
                              value={String(currentValue ?? "")}
                              onChange={(event) => saveNodeFieldValue(selectedNode.id, field, event.target.value)}
                              placeholder={field.placeholder}
                              className={field.type === "json" ? "font-mono text-xs min-h-28" : undefined}
                            />
                            {field.description && <p className="text-xs text-[#9ca3af]">{field.description}</p>}
                          </div>
                        );
                      }

                      if (field.type === "number") {
                        return (
                          <div key={field.id} className="space-y-2">
                            <Label>{field.label}</Label>
                            <Input
                              type="number"
                              min={field.min}
                              max={field.max}
                              step={field.step}
                              value={String(currentValue ?? field.min ?? 0)}
                              onChange={(event) => saveNodeFieldValue(selectedNode.id, field, event.target.value)}
                            />
                            {field.description && <p className="text-xs text-[#9ca3af]">{field.description}</p>}
                          </div>
                        );
                      }

                      return (
                        <div key={field.id} className="space-y-2">
                          <Label>{field.label}</Label>
                          <Input
                            value={String(currentValue ?? "")}
                            onChange={(event) => saveNodeFieldValue(selectedNode.id, field, event.target.value)}
                            placeholder={field.placeholder}
                          />
                          {field.description && <p className="text-xs text-[#9ca3af]">{field.description}</p>}
                        </div>
                      );
                    })}
                </div>

                <Separator />

                <div className="space-y-2">
                  <Label>Connect to node</Label>
                  <Select value={edgeTarget} onValueChange={setEdgeTarget}>
                    <SelectTrigger data-testid="select-edge-target">
                      <SelectValue placeholder="Select target node" />
                    </SelectTrigger>
                    <SelectContent>
                      {draft.definition.nodes
                        .filter((node) => node.id !== selectedNode.id)
                        .map((node) => (
                          <SelectItem key={node.id} value={node.id}>
                            {node.label}
                          </SelectItem>
                        ))}
                    </SelectContent>
                  </Select>
                  <Select value={edgeLabel} onValueChange={setEdgeLabel}>
                    <SelectTrigger data-testid="select-edge-label">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {EDGE_LABEL_OPTIONS.map((label) => (
                        <SelectItem key={label} value={label}>
                          {label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <Button className="w-full" variant="outline" onClick={addEdge} data-testid="button-add-edge">
                    <Cable size={14} className="mr-2" />
                    Add connection
                  </Button>
                </div>

                <div className="space-y-2">
                  <Label>Outgoing connections</Label>
                  {selectedNodeOutgoingEdges.length === 0 && (
                    <p className="text-xs text-[#9ca3af]">No outgoing edges</p>
                  )}
                  {selectedNodeOutgoingEdges.map((edge) => (
                    <div key={edge.id} className="flex items-center justify-between gap-2 rounded-lg border px-2 py-1.5">
                      <div className="min-w-0">
                        <div className="flex items-center gap-1">
                          <CornerDownRight size={12} />
                          <EllipsisText text={nodesById.get(edge.target)?.label || edge.target} className="text-xs font-medium max-w-[160px]" />
                        </div>
                        {edge.label && <p className="text-[11px] text-[#9ca3af]">{edge.label}</p>}
                      </div>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7 text-red-500 hover:text-red-700"
                        onClick={() => removeEdge(edge.id)}
                      >
                        <Trash2 size={13} />
                      </Button>
                    </div>
                  ))}
                </div>

                <Button
                  variant="destructive"
                  className="w-full"
                  onClick={() => removeNode(selectedNode.id)}
                  data-testid="button-delete-node"
                >
                  Remove node
                </Button>

            <Separator />

            <div className="space-y-2">
              <p className="text-xs font-semibold uppercase text-[#9ca3af]">Quick stats</p>
              <div className="grid grid-cols-2 gap-2 text-xs">
                <div className="rounded-lg border p-2">
                  <p className="text-[#9ca3af]">Nodes</p>
                  <p className="font-semibold text-base">{draft.definition.nodes.length}</p>
                </div>
                <div className="rounded-lg border p-2">
                  <p className="text-[#9ca3af]">Edges</p>
                  <p className="font-semibold text-base">{draft.definition.edges.length}</p>
                </div>
              </div>
            </div>

            <Separator />

            <div className="space-y-2" data-testid="workflow-run-history">
              <div className="flex items-center justify-between gap-2">
                <p className="text-xs font-semibold uppercase text-[#9ca3af] flex items-center gap-1">
                  <History size={12} />
                  Run history
                </p>
                <Badge variant="outline" className="text-[10px]">
                  last {workflowRunsRetentionLimit}
                </Badge>
              </div>
              {!selectedWorkflowID ? (
                <p className="text-xs text-[#9ca3af]">Select an existing workflow to view run history.</p>
              ) : showWorkflowRunsSkeleton ? (
                <div className="space-y-2">
                  <Skeleton className="h-12 w-full rounded-lg" />
                  <Skeleton className="h-12 w-full rounded-lg" />
                  <Skeleton className="h-12 w-full rounded-lg" />
                </div>
              ) : workflowRuns.length === 0 ? (
                <p className="text-xs text-[#9ca3af]">No runs yet. Use “Test run” to execute this workflow.</p>
              ) : (
                <div className="space-y-2 max-h-52 overflow-y-auto pr-1">
                  {workflowRuns.map((run: any) => (
                    <div key={run.id} className="rounded-lg border p-2">
                      <div className="flex items-center justify-between gap-2">
                        <Badge
                          variant={
                            run.status === "success"
                              ? "default"
                              : run.status === "failed"
                                ? "destructive"
                                : "secondary"
                          }
                          className="text-[10px]"
                        >
                          {run.status}
                        </Badge>
                        <span className="text-[11px] text-[#9ca3af]">{formatDuration(run.durationMs)}</span>
                      </div>
                      <p className="text-[11px] mt-1 text-[#9ca3af]">{formatRunDate(run.startedAt)}</p>
                      {run.error ? (
                        <p className="text-[11px] mt-1 text-red-600">{run.error}</p>
                      ) : (
                        <EllipsisText text={String(run.result?.summary || "Run completed")} className="text-[11px] mt-1 max-w-[220px]" />
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>

            <div className="rounded-lg border bg-[#171b2a] p-3 text-xs text-[#9ca3af] space-y-1">
              <p className="font-semibold text-foreground flex items-center gap-1">
                <Check size={12} className="text-green-600" />
                n8n-style principles used
              </p>
              <p className="flex items-center gap-1"><Play size={12} /> trigger-first workflow graph</p>
              <p className="flex items-center gap-1"><GitBranch size={12} /> branching via conditional connections</p>
              <p className="flex items-center gap-1"><FlaskConical size={12} /> per-node parameter inspector</p>
              <p className="flex items-center gap-1"><Workflow size={12} /> timeout and delay control nodes</p>
              <p className="flex items-center gap-1"><Send size={12} /> action nodes for communication connectors</p>
            </div>
          </Card>
          )}
          </div>
        )}
      </div>
      <WorkflowCredentialManagerDialog
        open={credentialManagerOpen}
        onOpenChange={(open) => {
          if (open) {
            setCredentialManagerOpen(true);
            return;
          }
          closeCredentialManager();
        }}
        credentials={workflowCredentials}
        editingCredentialId={editingCredentialId}
        draftName={credentialDraftName}
        draftDescription={credentialDraftDescription}
        draftFields={credentialDraftFields}
        selectedNodeTemplate={selectedNodeTemplate}
        actionButtonClass={WORKFLOW_ACTION_BUTTON_CLASS}
        onCreateNew={() => handleOpenCredentialManager("new")}
        onSelectCredential={(credentialId) => handleOpenCredentialManager("edit", credentialId)}
        onDraftNameChange={setCredentialDraftName}
        onDraftDescriptionChange={setCredentialDraftDescription}
        onDraftFieldChange={handleCredentialFieldChange}
        onDelete={handleDeleteCredential}
        onSave={handleSaveCredential}
      />

      <Sheet open={isAIAssistantOpen} onOpenChange={setIsAIAssistantOpen}>
        <SheetContent side="right" className="w-full border-l border-[#2a2c3c] bg-[#111622] p-0 sm:max-w-2xl">
          <div className="flex h-full min-h-0 flex-col">
            <div className="border-b border-[#2a2c3c] px-4 py-3">
              <SheetHeader className="pr-8 space-y-1">
                <SheetTitle className="flex items-center gap-2 text-base">
                  <WandSparkles size={18} className="text-[#facc15]" />
                  Workflow AI Agent
                </SheetTitle>
                <SheetDescription>
                  Describe the automation goal. Agent returns workflow JSON and can apply it to canvas.
                </SheetDescription>
              </SheetHeader>
            </div>

            <div className="min-h-0 flex-1 space-y-3 overflow-y-auto px-4 py-3" data-testid="workflow-ai-chat-messages">
              {aiMessages.length === 0 ? (
                <div className="rounded-lg border border-dashed border-[#2a2c3c] bg-[#0f131d] p-3 text-sm text-[#9ca3af]">
                  Example prompt: "Build workflow for phishing triage: extract IOC, match watchlist, map MITRE, score risk and send Telegram."
                </div>
              ) : (
                aiMessages.map((message) => (
                  <div
                    key={message.id}
                    className={`rounded-lg border border-[#2a2c3c] p-3 ${message.role === "assistant" ? "bg-[#171b2a]" : "bg-[#0f131d]"}`}
                  >
                    <p className="mb-1 text-[11px] font-semibold uppercase text-[#9ca3af]">
                      {message.role === "assistant" ? "AI Agent" : "You"}
                    </p>
                    <p className="whitespace-pre-wrap break-words text-sm">{message.content}</p>
                    {message.suggestion && (
                      <div className="mt-3 flex flex-wrap items-center gap-2">
                        <Badge variant="secondary">JSON parsed</Badge>
                        <Button
                          size="sm"
                          onClick={() => applyAISuggestion(message.suggestion as WorkflowAISuggestion)}
                          data-testid={`button-apply-ai-suggestion-${message.id}`}
                        >
                          Apply to canvas
                        </Button>
                      </div>
                    )}
                  </div>
                ))
              )}
            </div>

            <div className="space-y-2 border-t border-[#2a2c3c] px-4 py-3">
              <Textarea
                value={aiPrompt}
                onChange={(event) => setAIPrompt(event.target.value)}
                placeholder="Describe workflow you want to generate..."
                className="min-h-24 border-[#2a2c3c] bg-[#0f131d] text-[#f3f4f6] placeholder:text-[#6b7280]"
                data-testid="textarea-workflow-ai-prompt"
              />
              <div className="flex flex-wrap items-center justify-between gap-2">
                <Button
                  variant="ghost"
                  onClick={() => {
                    setAISessionID("");
                    setAIMessages([]);
                  }}
                  data-testid="button-workflow-ai-reset-chat"
                >
                  New chat
                </Button>
                <div className="flex flex-wrap items-center gap-2">
                  {askAI.isPending && (
                    <Button variant="outline" onClick={cancelAIPrompt} data-testid="button-workflow-ai-stop">
                      Stop
                    </Button>
                  )}
                  <Button
                    onClick={() => void sendAIPrompt()}
                    disabled={askAI.isPending || !aiPrompt.trim()}
                    data-testid="button-workflow-ai-generate"
                  >
                    <Send size={14} className="mr-2" />
                    Generate workflow
                  </Button>
                </div>
              </div>
            </div>
          </div>
        </SheetContent>
      </Sheet>
    </AppLayout>
  );
}
