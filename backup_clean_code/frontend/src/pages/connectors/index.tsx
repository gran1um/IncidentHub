import { useEffect, useMemo, useState } from "react";
import {
  Cloud,
  Database,
  Loader2,
  Mail,
  MessageSquare,
  Plus,
  Plug,
  RefreshCcw,
  Save,
  Search,
  Shield,
  Trash2,
  Webhook,
  X,
} from "lucide-react";
import { toast } from "sonner";

import { AppLayout } from "@/components/layout";
import { ConnectorExecutionDrawer } from "@/components/connector-execution-drawer";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import {
  useAppState,
  useConnectorHubExecutions,
  useConnectorMethods,
  useCreateConnector,
  useCreateConnectorMethod,
  useDeleteConnector,
  useDeleteConnectorMethod,
  useExecuteConnectorHub,
  useOutboundConnectors,
  useTenantConnectorMethods,
  useUpdateConnector,
  useUpdateConnectorMethod,
} from "@/lib/api";

type ConnectorPane = "catalog" | "editor";

type ConnectorForm = {
  name: string;
  description: string;
  type: string;
  category: string;
  enabled: boolean;
  capabilities: string[];
  communicationMode: string;
  config: Record<string, any>;
};

type MethodForm = {
  name: string;
  action: string;
  observableTypes: string;
  messageTemplate: string;
  metadataTemplate: string;
};

type ManualRunForm = {
  methodId: string;
  message: string;
  input: string;
  metadata: string;
  dryRun: boolean;
};

type TransportOption = {
  value: string;
  label: string;
  channel: string;
  segment: string;
  category?: string;
  communicationMode?: string;
  capabilities?: string[];
};

const CAPABILITY_OPTIONS = [
  { value: "hub_execute", label: "Hub execute" },
  { value: "case_communications", label: "Case communications" },
  { value: "forum_threads", label: "Forum threads" },
  { value: "sync_messages", label: "Sync messages" },
] as const;

const TRANSPORT_OPTIONS: TransportOption[] = [
  { value: "HTTP", label: "HTTP API", channel: "webhook", segment: "http" },
  { value: "Webhook", label: "Webhook", channel: "webhook", segment: "http" },
  { value: "SQL", label: "Database", channel: "sql", segment: "databases" },
  { value: "S3", label: "Object Storage", channel: "object_storage", segment: "storage" },
  { value: "Kafka", label: "Kafka", channel: "kafka", segment: "messaging" },
  { value: "Redis", label: "Redis", channel: "redis", segment: "messaging" },
  {
    value: "SMTP",
    label: "SMTP / Email",
    channel: "email",
    segment: "communications",
    communicationMode: "email",
    capabilities: ["hub_execute", "case_communications", "forum_threads", "sync_messages"],
  },
  {
    value: "Telegram",
    label: "Telegram",
    channel: "telegram",
    segment: "communications",
    communicationMode: "chat",
    capabilities: ["hub_execute", "case_communications", "forum_threads", "sync_messages"],
  },
  {
    value: "Slack",
    label: "Slack",
    channel: "slack",
    segment: "communications",
    communicationMode: "chat",
    capabilities: ["hub_execute", "case_communications", "forum_threads", "sync_messages"],
  },
  {
    value: "Outlook",
    label: "Outlook Mail",
    channel: "outlook",
    segment: "communications",
    communicationMode: "email",
    capabilities: ["hub_execute", "case_communications", "forum_threads", "sync_messages"],
  },
  {
    value: "Time",
    label: "Time",
    channel: "time",
    segment: "communications",
    communicationMode: "chat",
    capabilities: ["hub_execute", "case_communications", "sync_messages"],
  },
  { value: "Custom", label: "Custom", channel: "custom", segment: "custom", category: "custom" },
];

const SEGMENTS = [
  { value: "all", label: "All" },
  { value: "http", label: "HTTP & Webhooks" },
  { value: "databases", label: "Databases" },
  { value: "storage", label: "Storage" },
  { value: "messaging", label: "Messaging" },
  { value: "communications", label: "Email & Chat" },
  { value: "security", label: "Security" },
] as const;

function transportOptionByType(type: string): TransportOption | undefined {
  return TRANSPORT_OPTIONS.find((item) => item.value.toLowerCase() === String(type || "").trim().toLowerCase());
}

function defaultCapabilities(type: string): string[] {
  const option = transportOptionByType(type);
  return option?.capabilities ? [...option.capabilities] : ["hub_execute"];
}

function defaultCommunicationMode(type: string): string {
  return transportOptionByType(type)?.communicationMode || "none";
}

function defaultConfig(type: string): Record<string, any> {
  switch (String(type || "").trim()) {
    case "HTTP":
    case "Webhook":
      return { baseUrl: "", defaultHeaders: "", authType: "none" };
    case "SQL":
      return { engine: "postgres", host: "", port: "5432", database: "", username: "", password: "", sslMode: "disable", dsn: "" };
    case "S3":
      return { endpoint: "", region: "", bucket: "", accessKeyId: "", secretAccessKey: "", pathStyle: false };
    case "Kafka":
      return { brokers: "", topic: "", username: "", password: "", authType: "none", useTls: false };
    case "Redis":
      return { address: "", password: "", db: "0", list: "", useTls: false };
    case "SMTP":
      return { endpoint: "", username: "", password: "", from: "" };
    case "Telegram":
      return { botToken: "", chatId: "", username: "" };
    case "Slack":
      return { botToken: "", channelId: "", threadTs: "" };
    case "Outlook":
      return { tenantId: "", clientId: "", clientSecret: "", mailbox: "", authBaseURL: "", apiBaseURL: "" };
    case "Time":
      return { endpoint: "", token: "", recipient: "" };
    default:
      return {};
  }
}

function createEmptyConnectorForm(type = "HTTP"): ConnectorForm {
  return {
    name: "",
    description: "",
    type,
    category: transportOptionByType(type)?.category || "standard",
    enabled: true,
    capabilities: defaultCapabilities(type),
    communicationMode: defaultCommunicationMode(type),
    config: defaultConfig(type),
  };
}

function createEmptyMethodForm(): MethodForm {
  return {
    name: "",
    action: "",
    observableTypes: "",
    messageTemplate: "",
    metadataTemplate: "{}",
  };
}

function createEmptyManualRunForm(): ManualRunForm {
  return {
    methodId: "",
    message: "",
    input: "{}",
    metadata: "{}",
    dryRun: false,
  };
}

function parseJsonInput(raw: string, fallback: any) {
  const trimmed = String(raw || "").trim();
  if (!trimmed) {
    return fallback;
  }
  return JSON.parse(trimmed);
}

function prettifyJson(value: any): string {
  try {
    return JSON.stringify(value ?? {}, null, 2);
  } catch {
    return "{}";
  }
}

function connectorSummary(connector: any): string {
  const cfg = connector?.config || {};
  const type = String(connector?.type || "").trim().toLowerCase();
  if (type === "sql") {
    return [cfg.engine, cfg.host, cfg.database].filter(Boolean).join(" • ");
  }
  if (type === "s3") {
    return [cfg.bucket, cfg.endpoint].filter(Boolean).join(" • ");
  }
  if (type === "kafka") {
    return [cfg.topic, cfg.brokers].filter(Boolean).join(" • ");
  }
  if (type === "redis") {
    return [cfg.list, cfg.address].filter(Boolean).join(" • ");
  }
  if (type === "telegram") {
    return [cfg.chatId || cfg.chat_id, cfg.username].filter(Boolean).join(" • ");
  }
  if (type === "slack") {
    return [cfg.channelId || cfg.channel_id, cfg.threadTs || cfg.thread_ts].filter(Boolean).join(" • ");
  }
  if (type === "outlook") {
    return [cfg.mailbox, cfg.clientId || cfg.client_id].filter(Boolean).join(" • ");
  }
  return [cfg.baseUrl || cfg.endpoint || cfg.url, connector?.channel].filter(Boolean).join(" • ");
}

function connectorSegment(connector: any): string {
  const category = String(connector?.category || "").trim().toLowerCase();
  if (category === "security") {
    return "security";
  }
  return transportOptionByType(String(connector?.type || "").trim())?.segment || "all";
}

function connectorIcon(type: string) {
  switch (String(type || "").trim().toLowerCase()) {
    case "sql":
      return Database;
    case "s3":
      return Cloud;
    case "smtp":
    case "outlook":
      return Mail;
    case "telegram":
    case "slack":
    case "time":
      return MessageSquare;
    case "http":
    case "webhook":
      return Webhook;
    case "custom":
      return Shield;
    default:
      return Plug;
  }
}

function executionStatusClass(status: string): string {
  switch (String(status || "").trim().toLowerCase()) {
    case "completed":
      return "border-[rgba(34,197,94,0.35)] bg-[rgba(34,197,94,0.14)] text-[#86efac]";
    case "failed":
    case "dead_letter":
    case "cancelled":
      return "border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.14)] text-[#fda4af]";
    default:
      return "border-[rgba(245,158,11,0.35)] bg-[rgba(245,158,11,0.14)] text-[#fbbf24]";
  }
}

function formatExecutionTime(value?: string): string {
  if (!value) return "-";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
}

export default function ConnectorsPage() {
  const { currentTenantId } = useAppState();
  const { data: connectors = [], isLoading: connectorsLoading } = useOutboundConnectors(currentTenantId);
  const { data: tenantMethods = [] } = useTenantConnectorMethods(currentTenantId);
  const createConnector = useCreateConnector();
  const updateConnector = useUpdateConnector();
  const deleteConnector = useDeleteConnector();
  const createConnectorMethod = useCreateConnectorMethod();
  const updateConnectorMethod = useUpdateConnectorMethod();
  const deleteConnectorMethod = useDeleteConnectorMethod();
  const executeConnectorHub = useExecuteConnectorHub();

  const [searchQuery, setSearchQuery] = useState("");
  const [segment, setSegment] = useState("all");
  const [statusFilter, setStatusFilter] = useState("all");
  const [selectedConnectorId, setSelectedConnectorId] = useState("");
  const [selectedMethodId, setSelectedMethodId] = useState("");
  const [connectorForm, setConnectorForm] = useState<ConnectorForm>(() => createEmptyConnectorForm());
  const [methodForm, setMethodForm] = useState<MethodForm>(() => createEmptyMethodForm());
  const [manualRunForm, setManualRunForm] = useState<ManualRunForm>(() => createEmptyManualRunForm());
  const [drawerExecutionId, setDrawerExecutionId] = useState("");
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [connectorPane, setConnectorPane] = useState<ConnectorPane>("catalog");
  const [connectorEditorOpen, setConnectorEditorOpen] = useState(false);

  const filteredConnectors = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    return [...connectors]
      .filter((connector: any) => {
        if (segment !== "all" && connectorSegment(connector) !== segment) {
          return false;
        }
        if (statusFilter === "enabled" && !connector?.enabled) {
          return false;
        }
        if (statusFilter === "disabled" && connector?.enabled) {
          return false;
        }
        if (!query) {
          return true;
        }
        const haystack = [
          connector?.name,
          connector?.description,
          connector?.type,
          connector?.category,
          connectorSummary(connector),
        ]
          .map((item) => String(item || "").toLowerCase())
          .join(" ");
        return haystack.includes(query);
      })
      .sort((left: any, right: any) => String(left?.name || "").localeCompare(String(right?.name || "")));
  }, [connectors, searchQuery, segment, statusFilter]);


  const selectedConnector = useMemo(
    () => connectors.find((item: any) => String(item.id) === selectedConnectorId) || null,
    [connectors, selectedConnectorId],
  );

  const { data: connectorMethods = [] } = useConnectorMethods(selectedConnectorId);
  const selectedConnectorMethods = useMemo(
    () => (selectedConnectorId ? connectorMethods : tenantMethods.filter((item: any) => String(item?.refId || item?.ref_id || "") === selectedConnectorId)),
    [connectorMethods, selectedConnectorId, tenantMethods],
  );
  const selectedMethod = useMemo(
    () => selectedConnectorMethods.find((item: any) => String(item.id) === selectedMethodId) || null,
    [selectedConnectorMethods, selectedMethodId],
  );

  const executionsQuery = useConnectorHubExecutions(selectedConnectorId, { limit: 20, refetchInterval: connectorPane === "editor" ? 4000 : false });
  const executions = Array.isArray(executionsQuery.data) ? executionsQuery.data : [];

  useEffect(() => {
    if (!selectedConnector) {
      setConnectorForm(createEmptyConnectorForm());
      setSelectedMethodId("");
      setMethodForm(createEmptyMethodForm());
      setManualRunForm(createEmptyManualRunForm());
      return;
    }
    setConnectorForm({
      name: String(selectedConnector?.name || "").trim(),
      description: String(selectedConnector?.description || "").trim(),
      type: String(selectedConnector?.type || "HTTP").trim(),
      category: String(selectedConnector?.category || "standard").trim().toLowerCase() || "standard",
      enabled: Boolean(selectedConnector?.enabled ?? true),
      capabilities: Array.isArray(selectedConnector?.capabilities) ? selectedConnector.capabilities : defaultCapabilities(selectedConnector?.type),
      communicationMode: String(selectedConnector?.communicationMode || selectedConnector?.communication_mode || defaultCommunicationMode(selectedConnector?.type)).trim().toLowerCase() || "none",
      config: { ...defaultConfig(String(selectedConnector?.type || "HTTP")), ...(selectedConnector?.config || {}) },
    });
    setSelectedMethodId("");
    setMethodForm(createEmptyMethodForm());
    setManualRunForm(createEmptyManualRunForm());
  }, [selectedConnector?.id]);

  useEffect(() => {
    if (!selectedMethod) {
      setMethodForm(createEmptyMethodForm());
      return;
    }
    const data = selectedMethod?.data && typeof selectedMethod.data === "object" ? selectedMethod.data : selectedMethod;
    const observableTypesRaw = data?.observable_types ?? data?.observableTypes;
    setMethodForm({
      name: String(data?.name || "").trim(),
      action: String(data?.action || "").trim(),
      observableTypes: Array.isArray(observableTypesRaw)
        ? observableTypesRaw.join(", ")
        : String(observableTypesRaw ?? "").trim(),
      messageTemplate: String(data?.message_template || data?.messageTemplate || data?.body_template || data?.bodyTemplate || "").trim(),
      metadataTemplate: prettifyJson(data?.metadata_template || data?.metadataTemplate || {}),
    });
    setManualRunForm((prev) => ({ ...prev, methodId: String(selectedMethod.id) }));
  }, [selectedMethod?.id]);

  const connectorTypeOption = transportOptionByType(connectorForm.type);

  const enabledCount = useMemo(() => connectors.filter((item: any) => item?.enabled).length, [connectors]);
  const communicationCount = useMemo(
    () => connectors.filter((item: any) => Array.isArray(item?.capabilities) && item.capabilities.some((cap: string) => cap === "case_communications" || cap === "forum_threads")).length,
    [connectors],
  );

  const updateConfigField = (key: string, value: any) => {
    setConnectorForm((prev) => ({ ...prev, config: { ...prev.config, [key]: value } }));
  };

  const toggleCapability = (capability: string, checked: boolean) => {
    setConnectorForm((prev) => ({
      ...prev,
      capabilities: checked
        ? Array.from(new Set([...prev.capabilities, capability]))
        : prev.capabilities.filter((item) => item !== capability),
    }));
  };

  const handleTransportChange = (type: string) => {
    setConnectorForm((prev) => ({
      ...prev,
      type,
      category: transportOptionByType(type)?.category || (prev.category || "standard"),
      communicationMode: defaultCommunicationMode(type),
      capabilities: defaultCapabilities(type),
      config: { ...defaultConfig(type) },
    }));
  };

  const resetConnectorEditor = () => {
    setSelectedConnectorId("");
    setConnectorForm(createEmptyConnectorForm());
    setSelectedMethodId("");
    setMethodForm(createEmptyMethodForm());
    setManualRunForm(createEmptyManualRunForm());
  };

  const closeConnectorEditor = () => {
    setConnectorEditorOpen(false);
    setConnectorPane("catalog");
    resetConnectorEditor();
  };

  const openNewConnector = () => {
    resetConnectorEditor();
    setConnectorEditorOpen(true);
    setConnectorPane("editor");
  };

  const openConnectorEditor = (connectorId: string) => {
    setSelectedConnectorId(connectorId);
    setConnectorEditorOpen(true);
    setConnectorPane("editor");
  };

  const handleSaveConnector = () => {
    if (!currentTenantId) {
      toast.error("Tenant is required");
      return;
    }
    if (!connectorForm.name.trim()) {
      toast.error("Connector name is required");
      return;
    }
    const payload = {
      tenantId: currentTenantId,
      name: connectorForm.name.trim(),
      description: connectorForm.description.trim(),
      type: connectorForm.type,
      channel: connectorTypeOption?.channel,
      category: connectorForm.category,
      enabled: connectorForm.enabled,
      capabilities: connectorForm.capabilities,
      communicationMode: connectorForm.communicationMode === "none" ? "" : connectorForm.communicationMode,
      config: connectorForm.config,
    };
    const mutation = selectedConnectorId
      ? updateConnector.mutateAsync({ id: selectedConnectorId, data: payload, kind: "outbound_connectors" })
      : createConnector.mutateAsync(payload);
    mutation
      .then((saved: any) => {
        toast.success(selectedConnectorId ? "Connector updated" : "Connector created");
        if (saved?.id) {
          setSelectedConnectorId(String(saved.id));
        }
      })
      .catch((error: any) => {
        toast.error(error?.message || "Failed to save connector");
      });
  };

  const handleDeleteConnector = (connectorId: string) => {
    deleteConnector.mutate(
      { id: connectorId, kind: "outbound_connectors" },
      {
        onSuccess: () => {
          toast.success("Connector deleted");
          if (selectedConnectorId === connectorId) {
            closeConnectorEditor();
          }
        },
        onError: (error: any) => toast.error(error?.message || "Failed to delete connector"),
      },
    );
  };

  const handleSaveMethod = () => {
    if (!selectedConnectorId) {
      toast.error("Select a connector first");
      return;
    }
    if (!methodForm.name.trim() || !methodForm.action.trim()) {
      toast.error("Method name and action are required");
      return;
    }
    let metadataTemplate: any;
    try {
      metadataTemplate = parseJsonInput(methodForm.metadataTemplate, {});
    } catch (error: any) {
      toast.error(error?.message || "Metadata template must be valid JSON");
      return;
    }
    const payload = {
      connectorId: selectedConnectorId,
      name: methodForm.name.trim(),
      action: methodForm.action.trim(),
      observable_types: methodForm.observableTypes,
      message_template: methodForm.messageTemplate,
      metadata_template: metadataTemplate,
    };
    const mutation = selectedMethodId
      ? updateConnectorMethod.mutateAsync({ id: selectedMethodId, data: payload })
      : createConnectorMethod.mutateAsync(payload);
    mutation
      .then((saved: any) => {
        toast.success(selectedMethodId ? "Method updated" : "Method created");
        if (saved?.id) {
          setSelectedMethodId(String(saved.id));
        }
      })
      .catch((error: any) => toast.error(error?.message || "Failed to save method"));
  };

  const handleDeleteMethod = (methodId: string) => {
    deleteConnectorMethod.mutate(methodId, {
      onSuccess: () => {
        toast.success("Method deleted");
        if (selectedMethodId === methodId) {
          setSelectedMethodId("");
          setMethodForm(createEmptyMethodForm());
        }
      },
      onError: (error: any) => toast.error(error?.message || "Failed to delete method"),
    });
  };

  const handleManualRun = () => {
    if (!selectedConnectorId) {
      toast.error("Select a connector first");
      return;
    }
    const methodId = manualRunForm.methodId || selectedMethodId;
    if (!methodId) {
      toast.error("Choose a connector method to run");
      return;
    }
    let input: any;
    let metadata: any;
    try {
      input = parseJsonInput(manualRunForm.input, {});
      metadata = parseJsonInput(manualRunForm.metadata, {});
    } catch (error: any) {
      toast.error(error?.message || "Run payload must be valid JSON");
      return;
    }
    const selected = selectedConnectorMethods.find((item: any) => String(item.id) === String(methodId));
    executeConnectorHub.mutate(
      {
        connector_id: selectedConnectorId,
        method_id: methodId,
        action: String(selected?.data?.action || selected?.action || methodForm.action || "").trim(),
        message: manualRunForm.message,
        input,
        metadata,
        dry_run: manualRunForm.dryRun,
      },
      {
        onSuccess: (payload: any) => {
          toast.success(manualRunForm.dryRun ? "Dry run queued" : "Execution queued");
          const executionId = String(payload?.id || payload?.execution_id || "").trim();
          if (executionId) {
            setDrawerExecutionId(executionId);
            setDrawerOpen(true);
          }
        },
        onError: (error: any) => toast.error(error?.message || "Failed to run connector method"),
      },
    );
  };

  return (
    <AppLayout>
      <div className="mx-auto w-full max-w-[1380px] space-y-6">
        <div className="flex flex-wrap items-end justify-between gap-4 rounded-[24px] border border-[#202637] bg-[radial-gradient(circle_at_top_left,rgba(102,255,76,0.12),transparent_34%),linear-gradient(180deg,#111724_0%,#0b1018_100%)] px-6 py-6 shadow-[0_24px_70px_rgba(0,0,0,0.34)]">
          <div>
            <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-[#7ddf8a]">Connectors</div>
            <h1 className="mt-2 text-[30px] font-semibold tracking-[-0.04em] text-white">Connector Hub</h1>
            <p className="mt-2 max-w-[820px] text-sm text-[#98a4b8]">
              Typed outbound connectors for Hub, AI agents, workflows, case actions, alert actions, and communications.
            </p>
          </div>
          <div className="grid min-w-[280px] gap-3 sm:grid-cols-3">
            {[
              ["Total", String(connectors.length)],
              ["Active", String(enabledCount)],
              ["Comms", String(communicationCount)],
            ].map(([label, value]) => (
              <div key={label} className="rounded-2xl border border-[#253047] bg-[rgba(10,16,25,0.88)] px-4 py-3">
                <div className="text-[10px] uppercase tracking-[0.16em] text-[#6b7280]">{label}</div>
                <div className="mt-1 text-2xl font-semibold text-white">{value}</div>
              </div>
            ))}
          </div>
        </div>

        <Tabs value={connectorPane} onValueChange={(value) => setConnectorPane(value as ConnectorPane)} className="space-y-6">
          {connectorEditorOpen ? (
            <div className="flex flex-wrap items-center justify-between gap-3">
              <TabsList className="h-auto rounded-2xl border border-[#202637] bg-[#0d121b] p-1.5">
                <TabsTrigger value="catalog" className="rounded-xl px-4 py-2 text-sm data-[state=active]:bg-[#131b25] data-[state=active]:text-white">
                  Catalog
                </TabsTrigger>
                <TabsTrigger value="editor" className="max-w-[48vw] rounded-xl px-4 py-2 text-sm data-[state=active]:bg-[#131b25] data-[state=active]:text-white">
                  <span className="truncate">
                    {selectedConnectorId ? (connectorForm.name.trim() || "Edit connector") : "New connector"}
                  </span>
                </TabsTrigger>
              </TabsList>
              <Button
                type="button"
                size="icon"
                variant="outline"
                className="h-10 w-10 rounded-2xl border-[#202637] bg-[#0d121b] text-[#9ca3af] hover:bg-[#131b25] hover:text-white"
                onClick={closeConnectorEditor}
                aria-label="Close"
                title="Close"
                data-testid="button-close-connector-editor"
              >
                <X size={16} />
              </Button>
            </div>
          ) : null}

          <TabsContent value="catalog" className="m-0">
            <Card className="rounded-[24px] border border-[#202637] bg-[#0d121b] text-[#e5e7eb] shadow-[0_18px_44px_rgba(0,0,0,0.28)]">
              <CardHeader className="space-y-3">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <CardTitle className="text-lg text-white">Connector Catalog</CardTitle>
                    <p className="mt-1 text-sm text-[#8b97aa]">Create typed outbound connectors with explicit capabilities.</p>
                  </div>
                  <Button
                    type="button"
                    variant="outline"
                    className="rounded-xl border-[#2a3347] bg-[#0f1623] text-[#dbe4f0] hover:bg-[#172131]"
                    onClick={openNewConnector}
                    data-testid="button-new-connector"
                  >
                    <Plus size={14} className="mr-2" /> New connector
                  </Button>
                </div>
                <div className="grid gap-3 md:grid-cols-[1.2fr_0.9fr_0.7fr]">
                  <div className="relative">
                    <Search size={15} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[#6b7280]" />
                    <Input
                      value={searchQuery}
                      onChange={(event) => setSearchQuery(event.target.value)}
                      placeholder="Search connectors"
                      className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] pl-9 text-sm text-[#e5e7eb] placeholder:text-[#6b7280]"
                    />
                  </div>
                  <Select value={segment} onValueChange={setSegment}>
                    <SelectTrigger className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent className="rounded-xl border-[#2a3347] bg-[#101825] text-[#e5e7eb]">
                      {SEGMENTS.map((item) => (
                        <SelectItem key={item.value} value={item.value}>
                          {item.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <Select value={statusFilter} onValueChange={setStatusFilter}>
                    <SelectTrigger className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent className="rounded-xl border-[#2a3347] bg-[#101825] text-[#e5e7eb]">
                      <SelectItem value="all">All statuses</SelectItem>
                      <SelectItem value="enabled">Enabled</SelectItem>
                      <SelectItem value="disabled">Disabled</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </CardHeader>
              <CardContent className="space-y-4">
                {connectorsLoading ? (
                  <div className="rounded-2xl border border-[#283245] bg-[#0f1623] px-4 py-6 text-sm text-[#8b97aa]">Loading connectors...</div>
                ) : filteredConnectors.length === 0 ? (
                  <div className="rounded-2xl border border-dashed border-[#2f3b54] bg-[#0f1623] px-4 py-6 text-sm text-[#8b97aa]">
                    No connectors match the current filters.
                  </div>
                ) : (
                  <div className="grid gap-3">
                    {filteredConnectors.map((connector: any) => {
                      const Icon = connectorIcon(connector?.type);
                      const selected = String(connector.id) === selectedConnectorId;
                      return (
                        <button
                          key={connector.id}
                          type="button"
                          onClick={() => openConnectorEditor(String(connector.id))}
                          className={`w-full rounded-2xl border px-4 py-4 text-left transition ${selected ? "border-[#66ff4c]/40 bg-[#131b25] shadow-[0_0_0_1px_rgba(102,255,76,0.16)]" : "border-[#283245] bg-[#0f1623] hover:border-[#3a4865] hover:bg-[#131b25]"}`}
                          data-testid={`connector-card-${String(connector.id)}`}
                        >
                          <div className="flex flex-wrap items-start justify-between gap-3">
                            <div className="flex min-w-0 items-start gap-3">
                              <div className="rounded-2xl border border-[#2a3347] bg-[#101825] p-2 text-[#9fe870]">
                                <Icon size={18} />
                              </div>
                              <div className="min-w-0">
                                <div className="flex flex-wrap items-center gap-2">
                                  <span className="min-w-0 break-words text-sm font-semibold text-white">{connector.name || connector.id}</span>
                                  <Badge variant="outline" className="rounded-lg border-[#2f3b54] bg-[#101825] text-[10px] text-[#dbe4f0]">
                                    {connector.type}
                                  </Badge>
                                  {connector.category ? (
                                    <Badge
                                      variant="outline"
                                      className={`rounded-lg text-[10px] ${connector.category === "security" ? "border-[rgba(34,197,94,0.35)] bg-[rgba(22,101,52,0.42)] text-[#bbf7d0]" : "border-[#2f3b54] bg-[#101825] text-[#dbe4f0]"}`}
                                    >
                                      {connector.category}
                                    </Badge>
                                  ) : null}
                                  {connector.communicationMode ? (
                                    <Badge variant="outline" className="rounded-lg border-[#2f3b54] bg-[#101825] text-[10px] text-[#dbe4f0]">
                                      {connector.communicationMode}
                                    </Badge>
                                  ) : null}
                                </div>
                                <div className="mt-1 break-words text-sm text-[#8b97aa]">{connector.description || "No description"}</div>
                                {connectorSummary(connector) ? <div className="mt-1 break-words text-xs text-[#78d98a]">{connectorSummary(connector)}</div> : null}
                              </div>
                            </div>
                            <div className="flex items-center gap-2">
                              <Badge
                                className={`rounded-lg border text-[10px] ${connector.enabled ? "border-[rgba(34,197,94,0.35)] bg-[rgba(34,197,94,0.14)] text-[#86efac]" : "border-[#4b5563] bg-[#111827] text-[#cbd5e1]"}`}
                              >
                                {connector.enabled ? "Enabled" : "Disabled"}
                              </Badge>
                            </div>
                          </div>
                        </button>
                      );
                    })}
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="editor" className="m-0">
            <div className="mx-auto w-full max-w-[1120px] space-y-6">
              <div className="rounded-[24px] border border-[#202637] bg-[linear-gradient(180deg,#111724_0%,#0b1018_100%)] px-6 py-6 shadow-[0_18px_44px_rgba(0,0,0,0.28)]">
                <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-[#7ddf8a]">Connector editor</div>
                <h2 className="mt-2 text-[26px] font-semibold tracking-[-0.04em] text-white">
                  {selectedConnectorId ? "Edit connector" : "New connector"}
                </h2>
                <p className="mt-2 max-w-[860px] text-sm text-[#98a4b8]">Connector config, methods, and executions in one place.</p>
              </div>

              <div className="space-y-6 pb-12">
              <Card className="rounded-[24px] border border-[#202637] bg-[#0d121b] text-[#e5e7eb] shadow-[0_18px_44px_rgba(0,0,0,0.28)]">
                <CardHeader className="space-y-2">
                  <CardTitle className="text-lg text-white">Settings</CardTitle>
                  <p className="text-sm text-[#8b97aa]">Capabilities, routing, and driver-specific config.</p>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid gap-3 sm:grid-cols-2">
                    {CAPABILITY_OPTIONS.map((item) => (
                      <label
                        key={item.value}
                        className="grid min-h-[92px] min-w-0 grid-cols-[20px,minmax(0,1fr)] items-start gap-3 rounded-2xl border border-[#283245] bg-[#0f1623] px-4 py-4 text-left text-sm text-[#dbe4f0]"
                      >
                        <Checkbox
                          className="mt-0.5"
                          checked={connectorForm.capabilities.includes(item.value)}
                          onCheckedChange={(checked) => toggleCapability(item.value, Boolean(checked))}
                        />
                        <span className="min-w-0 whitespace-normal break-words text-[15px] leading-6">{item.label}</span>
                      </label>
                    ))}
                  </div>

                  <div className="grid gap-4 md:grid-cols-2">
                    <div className="space-y-2">
                      <Label>Name</Label>
                      <Input
                        value={connectorForm.name}
                        onChange={(event) => setConnectorForm((prev) => ({ ...prev, name: event.target.value }))}
                        className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]"
                      />
                    </div>
                    <div className="space-y-2">
                      <Label>Transport</Label>
                      <Select value={connectorForm.type} onValueChange={handleTransportChange}>
                        <SelectTrigger className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent className="rounded-xl border-[#2a3347] bg-[#101825] text-[#e5e7eb]">
                          {TRANSPORT_OPTIONS.map((item) => (
                            <SelectItem key={item.value} value={item.value}>
                              {item.label}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2 md:col-span-2">
                      <Label>Description</Label>
                      <Textarea
                        value={connectorForm.description}
                        onChange={(event) => setConnectorForm((prev) => ({ ...prev, description: event.target.value }))}
                        rows={3}
                        className="rounded-2xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]"
                      />
                    </div>
                    <div className="space-y-2">
                      <Label>Category</Label>
                      <Select value={connectorForm.category} onValueChange={(value) => setConnectorForm((prev) => ({ ...prev, category: value }))}>
                        <SelectTrigger className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent className="rounded-xl border-[#2a3347] bg-[#101825] text-[#e5e7eb]">
                          <SelectItem value="standard">Standard</SelectItem>
                          <SelectItem value="security">Security</SelectItem>
                          <SelectItem value="notifications">Notifications</SelectItem>
                          <SelectItem value="infra">Infrastructure</SelectItem>
                          <SelectItem value="custom">Custom</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2">
                      <Label>Communication mode</Label>
                      <Select value={connectorForm.communicationMode || "none"} onValueChange={(value) => setConnectorForm((prev) => ({ ...prev, communicationMode: value }))}>
                        <SelectTrigger className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent className="rounded-xl border-[#2a3347] bg-[#101825] text-[#e5e7eb]">
                          <SelectItem value="none">None</SelectItem>
                          <SelectItem value="chat">Chat</SelectItem>
                          <SelectItem value="email">Email</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                  </div>

                  <div className="flex flex-wrap items-center gap-3 rounded-2xl border border-[#283245] bg-[#0f1623] px-4 py-3 text-sm text-[#dbe4f0]">
                    <Checkbox
                      checked={connectorForm.enabled}
                      onCheckedChange={(checked) => setConnectorForm((prev) => ({ ...prev, enabled: Boolean(checked) }))}
                    />
                    <span>Enabled</span>
                    {connectorTypeOption?.channel ? (
                      <Badge variant="outline" className="rounded-lg border-[#2f3b54] bg-[#101825] text-[#dbe4f0]">
                        {connectorTypeOption.channel}
                      </Badge>
                    ) : null}
                  </div>

                  <div className="grid gap-3 md:grid-cols-2">
                    {(connectorForm.type === "HTTP" || connectorForm.type === "Webhook") ? (
                      <>
                        <div className="space-y-2">
                          <Label>Base URL</Label>
                          <Input
                            value={connectorForm.config.baseUrl || ""}
                            onChange={(event) => updateConfigField("baseUrl", event.target.value)}
                            className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]"
                          />
                        </div>
                        <div className="space-y-2">
                          <Label>Auth type</Label>
                          <Select value={String(connectorForm.config.authType || "none")} onValueChange={(value) => updateConfigField("authType", value)}>
                            <SelectTrigger className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]">
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent className="rounded-xl border-[#2a3347] bg-[#101825] text-[#e5e7eb]">
                              <SelectItem value="none">None</SelectItem>
                              <SelectItem value="api_key">API key</SelectItem>
                              <SelectItem value="bearer">Bearer</SelectItem>
                              <SelectItem value="basic">Basic</SelectItem>
                            </SelectContent>
                          </Select>
                        </div>
                        <div className="space-y-2 md:col-span-2">
                          <Label>Default headers (JSON)</Label>
                          <Textarea
                            value={String(connectorForm.config.defaultHeaders || "")}
                            onChange={(event) => updateConfigField("defaultHeaders", event.target.value)}
                            rows={4}
                            className="rounded-2xl border-[#2a3347] bg-[#0f1623] font-mono text-xs text-[#e5e7eb]"
                          />
                        </div>
                      </>
                    ) : null}

                    {connectorForm.type === "SQL" ? (
                      <>
                        <div className="space-y-2">
                          <Label>Host</Label>
                          <Input value={String(connectorForm.config.host || "")} onChange={(event) => updateConfigField("host", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" />
                        </div>
                        <div className="space-y-2">
                          <Label>Port</Label>
                          <Input value={String(connectorForm.config.port || "")} onChange={(event) => updateConfigField("port", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" />
                        </div>
                        <div className="space-y-2">
                          <Label>Database</Label>
                          <Input value={String(connectorForm.config.database || "")} onChange={(event) => updateConfigField("database", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" />
                        </div>
                        <div className="space-y-2">
                          <Label>Username</Label>
                          <Input value={String(connectorForm.config.username || "")} onChange={(event) => updateConfigField("username", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" />
                        </div>
                        <div className="space-y-2">
                          <Label>Password</Label>
                          <Input value={String(connectorForm.config.password || "")} onChange={(event) => updateConfigField("password", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" />
                        </div>
                        <div className="space-y-2">
                          <Label>SSL mode</Label>
                          <Input value={String(connectorForm.config.sslMode || "")} onChange={(event) => updateConfigField("sslMode", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" />
                        </div>
                        <div className="space-y-2 md:col-span-2">
                          <Label>DSN (optional)</Label>
                          <Input value={String(connectorForm.config.dsn || "")} onChange={(event) => updateConfigField("dsn", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" />
                        </div>
                      </>
                    ) : null}

                    {connectorForm.type === "S3" ? (
                      <>
                        <div className="space-y-2"><Label>Endpoint</Label><Input value={String(connectorForm.config.endpoint || "")} onChange={(event) => updateConfigField("endpoint", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2"><Label>Region</Label><Input value={String(connectorForm.config.region || "")} onChange={(event) => updateConfigField("region", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2"><Label>Bucket</Label><Input value={String(connectorForm.config.bucket || "")} onChange={(event) => updateConfigField("bucket", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2"><Label>Access key ID</Label><Input value={String(connectorForm.config.accessKeyId || "")} onChange={(event) => updateConfigField("accessKeyId", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2"><Label>Secret access key</Label><Input value={String(connectorForm.config.secretAccessKey || "")} onChange={(event) => updateConfigField("secretAccessKey", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                      </>
                    ) : null}

                    {connectorForm.type === "Kafka" ? (
                      <>
                        <div className="space-y-2 md:col-span-2"><Label>Brokers</Label><Input value={String(connectorForm.config.brokers || "")} onChange={(event) => updateConfigField("brokers", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2"><Label>Topic</Label><Input value={String(connectorForm.config.topic || "")} onChange={(event) => updateConfigField("topic", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                      </>
                    ) : null}

                    {connectorForm.type === "Redis" ? (
                      <>
                        <div className="space-y-2 md:col-span-2"><Label>Address</Label><Input value={String(connectorForm.config.address || "")} onChange={(event) => updateConfigField("address", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2"><Label>Password</Label><Input value={String(connectorForm.config.password || "")} onChange={(event) => updateConfigField("password", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2"><Label>DB</Label><Input value={String(connectorForm.config.db || "")} onChange={(event) => updateConfigField("db", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                      </>
                    ) : null}

                    {connectorForm.type === "SMTP" ? (
                      <>
                        <div className="space-y-2 md:col-span-2"><Label>Endpoint</Label><Input value={String(connectorForm.config.endpoint || "")} onChange={(event) => updateConfigField("endpoint", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2"><Label>Username</Label><Input value={String(connectorForm.config.username || "")} onChange={(event) => updateConfigField("username", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2"><Label>Password</Label><Input value={String(connectorForm.config.password || "")} onChange={(event) => updateConfigField("password", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2 md:col-span-2"><Label>From</Label><Input value={String(connectorForm.config.from || "")} onChange={(event) => updateConfigField("from", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                      </>
                    ) : null}

                    {connectorForm.type === "Telegram" ? (
                      <>
                        <div className="space-y-2 md:col-span-2"><Label>Bot token</Label><Input value={String(connectorForm.config.botToken || "")} onChange={(event) => updateConfigField("botToken", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2"><Label>Chat ID</Label><Input value={String(connectorForm.config.chatId || "")} onChange={(event) => updateConfigField("chatId", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2"><Label>Username</Label><Input value={String(connectorForm.config.username || "")} onChange={(event) => updateConfigField("username", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                      </>
                    ) : null}

                    {connectorForm.type === "Slack" ? (
                      <>
                        <div className="space-y-2 md:col-span-2"><Label>Bot token</Label><Input value={String(connectorForm.config.botToken || "")} onChange={(event) => updateConfigField("botToken", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2"><Label>Channel ID</Label><Input value={String(connectorForm.config.channelId || "")} onChange={(event) => updateConfigField("channelId", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2"><Label>Thread TS</Label><Input value={String(connectorForm.config.threadTs || "")} onChange={(event) => updateConfigField("threadTs", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                      </>
                    ) : null}

                    {connectorForm.type === "Outlook" ? (
                      <>
                        <div className="space-y-2"><Label>Tenant ID</Label><Input value={String(connectorForm.config.tenantId || "")} onChange={(event) => updateConfigField("tenantId", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2"><Label>Client ID</Label><Input value={String(connectorForm.config.clientId || "")} onChange={(event) => updateConfigField("clientId", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2 md:col-span-2"><Label>Client secret</Label><Input value={String(connectorForm.config.clientSecret || "")} onChange={(event) => updateConfigField("clientSecret", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2 md:col-span-2"><Label>Mailbox</Label><Input value={String(connectorForm.config.mailbox || "")} onChange={(event) => updateConfigField("mailbox", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2 md:col-span-2"><Label>Auth base URL</Label><Input value={String(connectorForm.config.authBaseURL || "")} onChange={(event) => updateConfigField("authBaseURL", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2 md:col-span-2"><Label>API base URL</Label><Input value={String(connectorForm.config.apiBaseURL || "")} onChange={(event) => updateConfigField("apiBaseURL", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                      </>
                    ) : null}

                    {connectorForm.type === "Time" ? (
                      <>
                        <div className="space-y-2 md:col-span-2"><Label>Endpoint</Label><Input value={String(connectorForm.config.endpoint || "")} onChange={(event) => updateConfigField("endpoint", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2 md:col-span-2"><Label>Token</Label><Input value={String(connectorForm.config.token || "")} onChange={(event) => updateConfigField("token", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2 md:col-span-2"><Label>Recipient</Label><Input value={String(connectorForm.config.recipient || "")} onChange={(event) => updateConfigField("recipient", event.target.value)} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                      </>
                    ) : null}
                  </div>

                  <div className="flex flex-wrap justify-end gap-2">
                    {selectedConnectorId ? (
                      <Button
                        type="button"
                        variant="outline"
                        className="rounded-xl border-[rgba(239,68,68,0.28)] bg-[rgba(239,68,68,0.12)] text-[#fda4af] hover:bg-[rgba(239,68,68,0.18)]"
                        onClick={() => handleDeleteConnector(selectedConnectorId)}
                      >
                        <Trash2 size={14} className="mr-2" /> Delete
                      </Button>
                    ) : null}
                    <Button
                      type="button"
                      className="rounded-xl bg-[#66ff4c] text-[#081108] hover:bg-[#7dff67]"
                      onClick={handleSaveConnector}
                      disabled={createConnector.isPending || updateConnector.isPending}
                    >
                      {(createConnector.isPending || updateConnector.isPending) ? <Loader2 size={14} className="mr-2 animate-spin" /> : <Save size={14} className="mr-2" />}
                      {selectedConnectorId ? "Save connector" : "Create connector"}
                    </Button>
                  </div>
                </CardContent>
              </Card>

              <Card className="rounded-[24px] border border-[#202637] bg-[#0d121b] text-[#e5e7eb] shadow-[0_18px_44px_rgba(0,0,0,0.28)]">
                <CardHeader className="space-y-2">
                  <CardTitle className="text-lg text-white">Methods</CardTitle>
                  <p className="text-sm text-[#8b97aa]">Reusable methods for observables, case actions, alert actions, and agent runs.</p>
                </CardHeader>
                <CardContent className="space-y-4">
                  {!selectedConnectorId ? (
                    <div className="rounded-2xl border border-dashed border-[#2f3b54] bg-[#0f1623] px-4 py-6 text-sm text-[#8b97aa]">Select a connector to manage its methods.</div>
                  ) : (
                    <>
                      <div className="grid gap-4 md:grid-cols-2">
                        <div className="space-y-2"><Label>Method name</Label><Input value={methodForm.name} onChange={(event) => setMethodForm((prev) => ({ ...prev, name: event.target.value }))} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2"><Label>Action</Label><Input value={methodForm.action} onChange={(event) => setMethodForm((prev) => ({ ...prev, action: event.target.value }))} className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2 md:col-span-2"><Label>Observable types</Label><Input value={methodForm.observableTypes} onChange={(event) => setMethodForm((prev) => ({ ...prev, observableTypes: event.target.value }))} placeholder="ip, hash, url, email" className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2 md:col-span-2"><Label>{connectorForm.type === "SQL" ? "Query / message template" : "Message template"}</Label><Textarea value={methodForm.messageTemplate} onChange={(event) => setMethodForm((prev) => ({ ...prev, messageTemplate: event.target.value }))} rows={4} className="rounded-2xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2 md:col-span-2"><Label>Metadata template (JSON)</Label><Textarea value={methodForm.metadataTemplate} onChange={(event) => setMethodForm((prev) => ({ ...prev, metadataTemplate: event.target.value }))} rows={5} className="rounded-2xl border-[#2a3347] bg-[#0f1623] font-mono text-xs text-[#e5e7eb]" /></div>
                      </div>
                      <div className="flex flex-wrap justify-end gap-2">
                        {selectedMethodId ? <Button type="button" variant="outline" className="rounded-xl border-[#2a3347] bg-[#0f1623] text-[#dbe4f0] hover:bg-[#172131]" onClick={() => { setSelectedMethodId(""); setMethodForm(createEmptyMethodForm()); }}>New method</Button> : null}
                        {selectedMethodId ? <Button type="button" variant="outline" className="rounded-xl border-[rgba(239,68,68,0.28)] bg-[rgba(239,68,68,0.12)] text-[#fda4af] hover:bg-[rgba(239,68,68,0.18)]" onClick={() => handleDeleteMethod(selectedMethodId)}><Trash2 size={14} className="mr-2" />Delete</Button> : null}
                        <Button type="button" className="rounded-xl bg-[#66ff4c] text-[#081108] hover:bg-[#7dff67]" onClick={handleSaveMethod} disabled={createConnectorMethod.isPending || updateConnectorMethod.isPending}>
                          {(createConnectorMethod.isPending || updateConnectorMethod.isPending) ? <Loader2 size={14} className="mr-2 animate-spin" /> : <Save size={14} className="mr-2" />}
                          {selectedMethodId ? "Save method" : "Create method"}
                        </Button>
                      </div>
                      <div className="grid gap-3">
                        {selectedConnectorMethods.length === 0 ? (
                          <div className="rounded-2xl border border-dashed border-[#2f3b54] bg-[#0f1623] px-4 py-6 text-sm text-[#8b97aa]">No methods yet for this connector.</div>
                        ) : (
                          selectedConnectorMethods.map((method: any) => {
                            const data = method?.data && typeof method.data === "object" ? method.data : method;
                            const selected = String(method.id) === selectedMethodId;
                            return (
                              <button key={method.id} type="button" onClick={() => setSelectedMethodId(String(method.id))} className={`w-full rounded-2xl border px-4 py-3 text-left transition ${selected ? "border-[#66ff4c]/40 bg-[#131b25]" : "border-[#283245] bg-[#0f1623] hover:border-[#3a4865]"}`}>
                                <div className="flex flex-wrap items-center justify-between gap-2">
                                  <div>
                                    <div className="text-sm font-semibold text-white">{data?.name || method.id}</div>
                                    <div className="mt-1 text-xs text-[#8b97aa]">{data?.action || "no action"}</div>
                                  </div>
                                  {String(data?.observable_types || data?.observableTypes || "").trim() ? (
                                    <Badge variant="outline" className="rounded-lg border-[#2f3b54] bg-[#101825] text-[10px] text-[#dbe4f0]">{String(data?.observable_types || data?.observableTypes)}</Badge>
                                  ) : null}
                                </div>
                              </button>
                            );
                          })
                        )}
                      </div>
                    </>
                  )}
                </CardContent>
              </Card>

              <Card className="rounded-[24px] border border-[#202637] bg-[#0d121b] text-[#e5e7eb] shadow-[0_18px_44px_rgba(0,0,0,0.28)]">
                <CardHeader className="space-y-2">
                  <div className="flex items-center justify-between gap-3">
                    <div>
                      <CardTitle className="text-lg text-white">Executions</CardTitle>
                      <p className="text-sm text-[#8b97aa]">Durable runtime executions, retries, and live event log.</p>
                    </div>
                    {selectedConnectorId ? (
                      <Button type="button" variant="outline" className="rounded-xl border-[#2a3347] bg-[#0f1623] text-[#dbe4f0] hover:bg-[#172131]" onClick={() => executionsQuery.refetch()}>
                        <RefreshCcw size={14} className="mr-2" /> Refresh
                      </Button>
                    ) : null}
                  </div>
                </CardHeader>
                <CardContent className="space-y-4">
                  {!selectedConnectorId ? (
                    <div className="rounded-2xl border border-dashed border-[#2f3b54] bg-[#0f1623] px-4 py-6 text-sm text-[#8b97aa]">Select a connector to inspect executions or run a method.</div>
                  ) : (
                    <>
                      <div className="grid gap-3 md:grid-cols-2">
                        <div className="space-y-2">
                          <Label>Run method</Label>
                          <Select value={manualRunForm.methodId || "none"} onValueChange={(value) => setManualRunForm((prev) => ({ ...prev, methodId: value === "none" ? "" : value }))}>
                            <SelectTrigger className="h-10 rounded-xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]">
                              <SelectValue placeholder="Select method" />
                            </SelectTrigger>
                            <SelectContent className="rounded-xl border-[#2a3347] bg-[#101825] text-[#e5e7eb]">
                              <SelectItem value="none">Select method</SelectItem>
                              {selectedConnectorMethods.map((method: any) => (
                                <SelectItem key={method.id} value={String(method.id)}>
                                  {String(method?.data?.name || method?.name || method.id)}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        </div>
                        <label className="mt-7 flex items-center gap-2 rounded-2xl border border-[#283245] bg-[#0f1623] px-3 py-3 text-sm text-[#dbe4f0]">
                          <Checkbox checked={manualRunForm.dryRun} onCheckedChange={(checked) => setManualRunForm((prev) => ({ ...prev, dryRun: Boolean(checked) }))} /> Dry run
                        </label>
                        <div className="space-y-2 md:col-span-2"><Label>Message</Label><Textarea value={manualRunForm.message} onChange={(event) => setManualRunForm((prev) => ({ ...prev, message: event.target.value }))} rows={3} className="rounded-2xl border-[#2a3347] bg-[#0f1623] text-[#e5e7eb]" /></div>
                        <div className="space-y-2"><Label>Input (JSON)</Label><Textarea value={manualRunForm.input} onChange={(event) => setManualRunForm((prev) => ({ ...prev, input: event.target.value }))} rows={5} className="rounded-2xl border-[#2a3347] bg-[#0f1623] font-mono text-xs text-[#e5e7eb]" /></div>
                        <div className="space-y-2"><Label>Metadata (JSON)</Label><Textarea value={manualRunForm.metadata} onChange={(event) => setManualRunForm((prev) => ({ ...prev, metadata: event.target.value }))} rows={5} className="rounded-2xl border-[#2a3347] bg-[#0f1623] font-mono text-xs text-[#e5e7eb]" /></div>
                      </div>
                      <div className="flex justify-end">
                        <Button type="button" className="rounded-xl bg-[#66ff4c] text-[#081108] hover:bg-[#7dff67]" onClick={handleManualRun} disabled={executeConnectorHub.isPending}>
                          {executeConnectorHub.isPending ? <Loader2 size={14} className="mr-2 animate-spin" /> : null}
                          Queue execution
                        </Button>
                      </div>
                      <div className="grid gap-3">
                        {executionsQuery.isLoading ? (
                          <div className="rounded-2xl border border-[#283245] bg-[#0f1623] px-4 py-6 text-sm text-[#8b97aa]">Loading executions...</div>
                        ) : executions.length === 0 ? (
                          <div className="rounded-2xl border border-dashed border-[#2f3b54] bg-[#0f1623] px-4 py-6 text-sm text-[#8b97aa]">No executions yet for this connector.</div>
                        ) : (
                          executions.map((execution: any) => (
                            <button
                              key={execution.id}
                              type="button"
                              onClick={() => {
                                setDrawerExecutionId(String(execution.id));
                                setDrawerOpen(true);
                              }}
                              className="w-full rounded-2xl border border-[#283245] bg-[#0f1623] px-4 py-4 text-left transition hover:border-[#3a4865] hover:bg-[#131b25]"
                            >
                              <div className="flex flex-wrap items-start justify-between gap-3">
                                <div>
                                  <div className="flex flex-wrap items-center gap-2">
                                    <span className="text-sm font-semibold text-white">{execution.method_name || execution.action || execution.id}</span>
                                    <Badge className={`rounded-lg border text-[10px] ${executionStatusClass(String(execution.status || ""))}`}>{String(execution.status || "accepted")}</Badge>
                                  </div>
                                  <div className="mt-1 text-xs text-[#8b97aa]">{execution.execution_mode || "manual"} • {formatExecutionTime(execution.updated_at || execution.created_at)}</div>
                                </div>
                                <div className="text-right text-xs text-[#8b97aa]">attempts {execution.attempt_count || 0}/{execution.max_attempts || 0}</div>
                              </div>
                            </button>
                          ))
                        )}
                      </div>
                    </>
                  )}
                </CardContent>
              </Card>
              </div>
            </div>
          </TabsContent>
        </Tabs>
      </div>

      <ConnectorExecutionDrawer executionId={drawerExecutionId} open={drawerOpen} onOpenChange={setDrawerOpen} />
    </AppLayout>
  );
}
