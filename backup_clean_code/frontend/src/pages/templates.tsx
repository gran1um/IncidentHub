import { useMemo, useState } from "react";
import {
  Cloud,
  Database,
  Globe,
  Mail,
  MessageSquare,
  Plug,
  Plus,
  Radio,
  RotateCcw,
  Save,
  Shield,
  Timer,
  Trash2,
  Webhook,
  X,
} from "lucide-react";
import { toast } from "sonner";

import { AppLayout } from "@/components/layout";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import {
  useAppState,
  useCaseStatuses,
  useCaseTemplates,
  useCommunicationTemplates,
  useConnectorTemplates,
  useCreateCaseTemplate,
  useCreateCommunicationTemplate,
  useUpdateCommunicationTemplate,
  useDeleteCommunicationTemplate,
  useCreateConnectorTemplate,
  useCreateObservableType,
  useDeleteCaseTemplate,
  useDeleteConnectorTemplate,
  useDeleteObservableType,
  useObservableTypes,
  useUpdateCaseTemplate,
  useUpdateConnectorTemplate,
  useUpdateObservableType,
  useUsers,
} from "@/lib/api";
import {
  BUILTIN_CONNECTOR_TEMPLATE_PRESETS,
  CONNECTOR_TEMPLATE_ICON_OPTIONS,
  type ConnectorTemplateField,
  type ConnectorTemplateFieldType,
  cloneConnectorTemplatePreset,
  findConnectorTemplatePresetByType,
  formatConnectorFieldOptions,
  parseConnectorFieldOptions,
} from "@/lib/connectors/template-presets";
import { isSOARCaseFieldKey, mergeMissingSOARCaseFields, normalizeSOARCaseFieldKey } from "@/lib/soar-case-fields";
import { useMinimumLoading } from "@/lib/use-minimum-loading";

type IconComponent = typeof Globe;

type EditorPane = "list" | "editor";

type ConnectorTemplateForm = {
  name: string;
  description: string;
  type: string;
  channel: string;
  category: string;
  communicationMode: string;
  capabilities: string[];
  tags: string[];
  icon: string;
  configSchema: ConnectorTemplateField[];
  forms: {
    manualRun: ConnectorTemplateField[];
    forumSend: ConnectorTemplateField[];
    caseCommunication: ConnectorTemplateField[];
  };
};

type CommunicationTemplateForm = {
  name: string;
  description: string;
  channel: string;
  subjectTemplate: string;
  bodyTemplate: string;
};

type CaseCustomFieldRow = {
  id: string;
  key: string;
  value: string;
};

type CaseTaskRow = {
  id: string;
  value: string;
};

type CaseTemplateForm = {
  name: string;
  description: string;
  status: string;
  severity: string;
  source: string;
  incidentType: string;
  priority: string;
  confidence: number;
  tlp: string;
  pap: string;
  detectedAt: string;
  occurredAt: string;
  assigneeId: string;
  tags: string[];
  tasks: CaseTaskRow[];
  customFields: CaseCustomFieldRow[];
};

type ObservableTypeForm = {
  id: string;
  name: string;
  dataType: string;
  validatorRegex: string;
};

const PAGE_SHELL_CLASS = "mx-auto w-full max-w-[1200px] space-y-6 pb-6";
const PANEL_CLASS =
  "overflow-hidden rounded-[24px] border border-[#1f2433] bg-[linear-gradient(180deg,rgba(17,21,31,0.98),rgba(12,16,24,0.98))] shadow-[0_18px_46px_rgba(0,0,0,0.34)]";
const CARD_CLASS = "rounded-2xl border border-[#293246] bg-[#101621]";
const INPUT_CLASS =
  "h-10 rounded-xl border-[#2a3347] bg-[#0b1018] text-[#f3f4f6] placeholder:text-[#6b7280] focus-visible:ring-1 focus-visible:ring-[#66ff4c]/45";
const TEXTAREA_CLASS =
  "rounded-2xl border-[#2a3347] bg-[#0b1018] text-[#f3f4f6] placeholder:text-[#6b7280] focus-visible:ring-1 focus-visible:ring-[#66ff4c]/45";
const SELECT_TRIGGER_CLASS =
  "h-10 rounded-xl border-[#2a3347] bg-[#0b1018] text-[#f3f4f6] focus:ring-1 focus:ring-[#66ff4c]/45";
const SELECT_CONTENT_CLASS = "rounded-xl border border-[#2a3347] bg-[#101825] text-[#f3f4f6]";
const OUTLINE_BUTTON_CLASS = "border-[#2a3347] bg-[#0b1018] text-[#d1d5db] hover:border-[#42516a] hover:bg-[#151b28]";
const PRIMARY_BUTTON_CLASS = "border border-[#66ff4c] bg-[#66ff4c] text-[#08110a] hover:bg-[#81ff69]";
const MUTED_TEXT_CLASS = "text-sm text-[#8b91a3]";
const TABS_LIST_CLASS = "h-[54px] w-full justify-start gap-0 rounded-none border-b border-[#1f2433] bg-transparent p-0";
const TABS_TRIGGER_CLASS =
  "h-[54px] rounded-none border-b-2 border-transparent px-5 text-sm font-medium text-[#9ca3af] hover:text-[#d1d5db] data-[state=active]:border-[#66ff4c] data-[state=active]:bg-transparent data-[state=active]:text-[#66ff4c]";

const CAPABILITY_OPTIONS = [
  { value: "hub_execute", label: "Hub execute" },
  { value: "case_communications", label: "Case communications" },
  { value: "forum_threads", label: "Forum threads" },
  { value: "sync_messages", label: "Sync messages" },
] as const;

const FIELD_TYPE_OPTIONS: { value: ConnectorTemplateFieldType; label: string }[] = [
  { value: "text", label: "Text" },
  { value: "textarea", label: "Textarea" },
  { value: "secret", label: "Secret" },
  { value: "select", label: "Select" },
  { value: "boolean", label: "Boolean" },
  { value: "json", label: "JSON" },
];

const CATEGORY_OPTIONS = ["standard", "security", "notifications", "infra", "custom"] as const;
const COMMUNICATION_MODE_OPTIONS = ["none", "chat", "email"] as const;
const OBSERVABLE_DATA_TYPE_OPTIONS = ["string", "number", "boolean", "json"] as const;
const SEVERITY_OPTIONS = ["Critical", "High", "Medium", "Low"] as const;
const PRIORITY_OPTIONS = ["critical", "high", "medium", "low"] as const;
const TLP_OPTIONS = ["red", "amber", "green", "clear"] as const;
const PAP_OPTIONS = ["red", "amber", "green", "clear"] as const;
const COMMUNICATION_CHANNEL_OPTIONS = ["email", "chat"] as const;
const DEFAULT_CASE_STATUS_OPTIONS = [
  { code: "new", label: "New", isClosed: false },
  { code: "open", label: "Open", isClosed: false },
  { code: "resolved", label: "Resolved", isClosed: true },
  { code: "closed", label: "Closed", isClosed: true },
];

const iconComponentByKey: Record<string, IconComponent> = {
  globe: Globe,
  webhook: Webhook,
  database: Database,
  cloud: Cloud,
  mail: Mail,
  "message-square": MessageSquare,
  shield: Shield,
  radio: Radio,
  timer: Timer,
  plug: Plug,
};

function newID(prefix: string): string {
  return `${prefix}-${Math.random().toString(36).slice(2, 10)}`;
}

function uniqueStrings(values: string[]): string[] {
  const seen = new Set<string>();
  return values.filter((item) => {
    const normalized = String(item || "").trim();
    if (!normalized) {
      return false;
    }
    const key = normalized.toLowerCase();
    if (seen.has(key)) {
      return false;
    }
    seen.add(key);
    return true;
  });
}

function splitTags(raw: string): string[] {
  return uniqueStrings(String(raw || "").split(",").map((item) => item.trim()));
}

function iconLabel(icon: string): string {
  return CONNECTOR_TEMPLATE_ICON_OPTIONS.find((item) => item.value === icon)?.label || icon || "Icon";
}

function renderIcon(icon: string, className = "h-4 w-4") {
  const Icon = iconComponentByKey[icon] || Plug;
  return <Icon className={className} />;
}

function cloneSchema(fields: ConnectorTemplateField[]): ConnectorTemplateField[] {
  return (fields || []).map((field) => ({
    ...field,
    options: (field.options || []).map((option) => ({ ...option })),
  }));
}

function createEmptySchemaField(): ConnectorTemplateField {
  return {
    id: "",
    label: "",
    type: "text",
    required: false,
    placeholder: "",
    default_value: "",
    options: [],
    target_path: "",
    help_text: "",
  };
}

function createEmptyConnectorTemplateForm(type = "HTTP"): ConnectorTemplateForm {
  const preset = findConnectorTemplatePresetByType(type);
  if (preset) {
    return {
      name: preset.name,
      description: preset.description,
      type: preset.type,
      channel: preset.channel,
      category: preset.category,
      communicationMode: preset.communication_mode || "none",
      capabilities: [...preset.capabilities],
      tags: [...preset.tags],
      icon: preset.icon,
      configSchema: cloneSchema(preset.config_schema),
      forms: {
        manualRun: cloneSchema(preset.forms.manual_run),
        forumSend: cloneSchema(preset.forms.forum_send),
        caseCommunication: cloneSchema(preset.forms.case_communication),
      },
    };
  }
  return {
    name: "",
    description: "",
    type,
    channel: "",
    category: "standard",
    communicationMode: "none",
    capabilities: ["hub_execute"],
    tags: [],
    icon: "plug",
    configSchema: [],
    forms: {
      manualRun: [],
      forumSend: [],
      caseCommunication: [],
    },
  };
}

function connectorTemplateFormFromItem(item: any): ConnectorTemplateForm {
  return {
    name: String(item?.name || "").trim(),
    description: String(item?.description || "").trim(),
    type: String(item?.type || "HTTP").trim() || "HTTP",
    channel: String(item?.channel || "").trim().toLowerCase(),
    category: String(item?.category || "standard").trim().toLowerCase() || "standard",
    communicationMode: String(item?.communicationMode || item?.communication_mode || "none").trim().toLowerCase() || "none",
    capabilities: uniqueStrings(Array.isArray(item?.capabilities) ? item.capabilities : []),
    tags: uniqueStrings(Array.isArray(item?.tags) ? item.tags : []),
    icon: String(item?.icon || "plug").trim() || "plug",
    configSchema: cloneSchema(Array.isArray(item?.configSchema) ? item.configSchema : []),
    forms: {
      manualRun: cloneSchema(Array.isArray(item?.forms?.manualRun) ? item.forms.manualRun : []),
      forumSend: cloneSchema(Array.isArray(item?.forms?.forumSend) ? item.forms.forumSend : []),
      caseCommunication: cloneSchema(Array.isArray(item?.forms?.caseCommunication) ? item.forms.caseCommunication : []),
    },
  };
}

function createEmptyCommunicationTemplateForm(): CommunicationTemplateForm {
  return {
    name: "",
    description: "",
    channel: "email",
    subjectTemplate: "",
    bodyTemplate: "",
  };
}

function createCustomFieldRow(input?: Partial<CaseCustomFieldRow>): CaseCustomFieldRow {
  return {
    id: input?.id || newID("case-template-field"),
    key: String(input?.key || "").trim(),
    value: String(input?.value || ""),
  };
}

function createTaskRow(value = ""): CaseTaskRow {
  return {
    id: newID("case-template-task"),
    value,
  };
}

function emptyCaseTemplateForm(): CaseTemplateForm {
  return {
    name: "",
    description: "",
    status: "new",
    severity: "Medium",
    source: "manual",
    incidentType: "",
    priority: "medium",
    confidence: 0,
    tlp: "amber",
    pap: "amber",
    detectedAt: "",
    occurredAt: "",
    assigneeId: "",
    tags: [],
    tasks: [createTaskRow()],
    customFields: [],
  };
}

function isoToLocalDateTime(value: string): string {
  const raw = String(value || "").trim();
  if (!raw) {
    return "";
  }
  const date = new Date(raw);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  const shifted = new Date(date.getTime() - date.getTimezoneOffset() * 60000);
  return shifted.toISOString().slice(0, 16);
}

function localDateTimeToISO(value: string): string {
  const raw = String(value || "").trim();
  if (!raw) {
    return "";
  }
  const date = new Date(raw);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return date.toISOString();
}

function caseTemplateFormFromItem(item: any): CaseTemplateForm {
  const customFieldsObject = item?.customFields && typeof item.customFields === "object" ? item.customFields : {};
  const tasks = Array.isArray(item?.tasks) ? item.tasks : [];
  return {
    name: String(item?.name || "").trim(),
    description: String(item?.description || "").trim(),
    status: String(item?.status || "new").trim() || "new",
    severity: String(item?.severity || "Medium").trim() || "Medium",
    source: String(item?.source || "manual").trim() || "manual",
    incidentType: String(item?.incidentType || item?.incident_type || "").trim(),
    priority: String(item?.priority || "medium").trim() || "medium",
    confidence: Number(item?.confidence ?? 0) || 0,
    tlp: String(item?.tlp || "amber").trim().toLowerCase() || "amber",
    pap: String(item?.pap || "amber").trim().toLowerCase() || "amber",
    detectedAt: isoToLocalDateTime(String(item?.detectedAt || item?.detected_at || "")),
    occurredAt: isoToLocalDateTime(String(item?.occurredAt || item?.occurred_at || "")),
    assigneeId: String(item?.assigneeId || item?.assignee_id || "").trim(),
    tags: uniqueStrings(Array.isArray(item?.tags) ? item.tags : []),
    tasks: tasks.length > 0 ? tasks.map((task: any) => createTaskRow(String(task || ""))) : [createTaskRow()],
    customFields: Object.entries(customFieldsObject).map(([key, value]) =>
      createCustomFieldRow({ key: String(key || ""), value: value === null || value === undefined ? "" : String(value) }),
    ),
  };
}

function createEmptyObservableTypeForm(): ObservableTypeForm {
  return {
    id: "",
    name: "",
    dataType: "string",
    validatorRegex: "",
  };
}

function normalizeCaseCustomFields(rows: CaseCustomFieldRow[]): Record<string, string> {
  const payload: Record<string, string> = {};
  for (const row of rows) {
    const normalizedKey = normalizeSOARCaseFieldKey(row.key);
    const value = String(row.value || "");
    if (!normalizedKey && !value.trim()) {
      continue;
    }
    if (!normalizedKey) {
      throw new Error("Custom field key is required");
    }
    if (Object.prototype.hasOwnProperty.call(payload, normalizedKey)) {
      throw new Error(`Duplicate custom field: ${normalizedKey}`);
    }
    payload[normalizedKey] = value;
  }
  return payload;
}

function TagInput(props: { label: string; value: string[]; onChange: (next: string[]) => void; placeholder?: string; dataTestId?: string }) {
  const [draft, setDraft] = useState("");

  const commit = () => {
    const next = uniqueStrings([...props.value, ...splitTags(draft)]);
    props.onChange(next);
    setDraft("");
  };

  return (
    <div className="space-y-2">
      <Label>{props.label}</Label>
      <div className="rounded-2xl border border-[#2a3347] bg-[#0b1018] p-3">
        <div className="flex flex-wrap gap-2">
          {props.value.map((tag) => (
            <Badge key={tag} variant="outline" className="gap-1 rounded-full border-[#36511d] bg-[#15210f] px-2 py-1 text-[#c8f3bc]">
              {tag}
              <button type="button" onClick={() => props.onChange(props.value.filter((item) => item !== tag))} className="text-[#95a3b8] hover:text-white">
                x
              </button>
            </Badge>
          ))}
        </div>
        <div className="mt-3 flex gap-2">
          <Input
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault();
                commit();
              }
            }}
            placeholder={props.placeholder || "tag-1, tag-2"}
            className={INPUT_CLASS}
            data-testid={props.dataTestId}
          />
          <Button type="button" variant="outline" className={OUTLINE_BUTTON_CLASS} onClick={commit}>
            Add
          </Button>
        </div>
      </div>
    </div>
  );
}

function SchemaFieldListEditor(props: {
  title: string;
  description: string;
  fields: ConnectorTemplateField[];
  onChange: (next: ConnectorTemplateField[]) => void;
  dataTestIdPrefix: string;
}) {
  const updateField = (index: number, updater: (field: ConnectorTemplateField) => ConnectorTemplateField) => {
    props.onChange(props.fields.map((field, fieldIndex) => (fieldIndex === index ? updater(field) : field)));
  };

  return (
    <Card className={`${CARD_CLASS} p-4`}>
      <div className="mb-4 flex items-start justify-between gap-3">
        <div>
          <h4 className="text-sm font-semibold text-[#f3f4f6]">{props.title}</h4>
          <p className="mt-1 text-xs text-[#8b91a3]">{props.description}</p>
        </div>
        <Button
          type="button"
          variant="outline"
          className={OUTLINE_BUTTON_CLASS}
          onClick={() => props.onChange([...props.fields, createEmptySchemaField()])}
          data-testid={`button-add-${props.dataTestIdPrefix}-field`}
        >
          <Plus size={14} className="mr-2" /> Add field
        </Button>
      </div>
      <div className="space-y-3">
        {props.fields.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-[#2a3347] px-4 py-5 text-sm text-[#7f8798]">No fields yet.</div>
        ) : null}
        {props.fields.map((field, index) => (
          <div key={`${props.dataTestIdPrefix}-${index}`} className="rounded-2xl border border-[#243045] bg-[#0b1018] p-4" data-testid={`${props.dataTestIdPrefix}-field-${index}`}>
            <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
              <div className="space-y-2">
                <Label>Field ID</Label>
                <Input value={field.id} onChange={(event) => updateField(index, (current) => ({ ...current, id: event.target.value }))} className={INPUT_CLASS} />
              </div>
              <div className="space-y-2">
                <Label>Label</Label>
                <Input value={field.label} onChange={(event) => updateField(index, (current) => ({ ...current, label: event.target.value }))} className={INPUT_CLASS} />
              </div>
              <div className="space-y-2">
                <Label>Type</Label>
                <Select value={field.type || "text"} onValueChange={(value) => updateField(index, (current) => ({ ...current, type: value as ConnectorTemplateFieldType }))}>
                  <SelectTrigger className={SELECT_TRIGGER_CLASS}><SelectValue /></SelectTrigger>
                  <SelectContent className={SELECT_CONTENT_CLASS}>
                    {FIELD_TYPE_OPTIONS.map((option) => (
                      <SelectItem key={`${props.dataTestIdPrefix}-type-${option.value}`} value={option.value}>{option.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>Required</Label>
                <button
                  type="button"
                  className={`flex h-10 w-full items-center justify-center rounded-xl border text-sm transition-colors ${field.required ? "border-[#66ff4c] bg-[#162112] text-[#c8f3bc]" : "border-[#2a3347] bg-[#0b1018] text-[#9ca3af]"}`}
                  onClick={() => updateField(index, (current) => ({ ...current, required: !current.required }))}
                >
                  {field.required ? "Required" : "Optional"}
                </button>
              </div>
              <div className="space-y-2 xl:col-span-2">
                <Label>Placeholder</Label>
                <Input value={String(field.placeholder || "")} onChange={(event) => updateField(index, (current) => ({ ...current, placeholder: event.target.value }))} className={INPUT_CLASS} />
              </div>
              <div className="space-y-2 xl:col-span-2">
                <Label>Target path</Label>
                <Input value={String(field.target_path || "")} onChange={(event) => updateField(index, (current) => ({ ...current, target_path: event.target.value }))} className={INPUT_CLASS} />
              </div>
              <div className="space-y-2 xl:col-span-2">
                <Label>Default value</Label>
                {field.type === "boolean" ? (
                  <Select
                    value={field.default_value === true ? "true" : field.default_value === false ? "false" : "false"}
                    onValueChange={(value) => updateField(index, (current) => ({ ...current, default_value: value === "true" }))}
                  >
                    <SelectTrigger className={SELECT_TRIGGER_CLASS}><SelectValue /></SelectTrigger>
                    <SelectContent className={SELECT_CONTENT_CLASS}>
                      <SelectItem value="false">False</SelectItem>
                      <SelectItem value="true">True</SelectItem>
                    </SelectContent>
                  </Select>
                ) : (
                  <Input
                    value={field.default_value === undefined ? "" : String(field.default_value)}
                    onChange={(event) => updateField(index, (current) => ({ ...current, default_value: event.target.value }))}
                    className={INPUT_CLASS}
                  />
                )}
              </div>
              <div className="space-y-2 xl:col-span-2">
                <Label>Options</Label>
                <Input
                  value={formatConnectorFieldOptions(field.options)}
                  onChange={(event) => updateField(index, (current) => ({ ...current, options: parseConnectorFieldOptions(event.target.value) }))}
                  placeholder="Label:value, Label:value"
                  className={INPUT_CLASS}
                />
              </div>
              <div className="space-y-2 xl:col-span-4">
                <Label>Help text</Label>
                <Textarea value={String(field.help_text || "")} onChange={(event) => updateField(index, (current) => ({ ...current, help_text: event.target.value }))} rows={2} className={TEXTAREA_CLASS} />
              </div>
            </div>
            <div className="mt-3 flex justify-end">
              <Button type="button" variant="destructive" className="rounded-xl" onClick={() => props.onChange(props.fields.filter((_, fieldIndex) => fieldIndex !== index))}>
                <Trash2 size={14} className="mr-2" /> Remove
              </Button>
            </div>
          </div>
        ))}
      </div>
    </Card>
  );
}

function ConnectorTemplateCard(props: { item: any; onEdit: () => void; onDelete: () => void }) {
  return (
    <Card className={`${CARD_CLASS} p-4`} data-testid={`card-connector-template-${props.item.id}`}>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl border border-[#2a3347] bg-[#0b1018] text-[#d1d5db]">
              {renderIcon(String(props.item.icon || "plug"))}
            </div>
            <div>
              <p className="text-sm font-semibold text-[#f3f4f6]">{props.item.name || "Untitled template"}</p>
              <p className="text-xs text-[#8b91a3]">{props.item.type || "Custom"} / {props.item.channel || "-"}</p>
            </div>
          </div>
          <p className="mt-3 text-sm text-[#c8ced9]">{props.item.description || "No description"}</p>
          <div className="mt-3 flex flex-wrap gap-2">
            <Badge variant="outline" className="border-[#2a3347] bg-[#0b1018] text-[#dbe4f0]">{props.item.category || "standard"}</Badge>
            {String(props.item.communicationMode || "").trim() ? (
              <Badge variant="outline" className="border-[#36511d] bg-[#15210f] text-[#c8f3bc]">{props.item.communicationMode}</Badge>
            ) : null}
            {Array.isArray(props.item.tags) ? props.item.tags.map((tag: string) => (
              <Badge key={`${props.item.id}-${tag}`} variant="outline" className="border-[#31445f] bg-[#101825] text-[#d1d5db]">#{tag}</Badge>
            )) : null}
          </div>
          <div className="mt-3 grid gap-2 sm:grid-cols-3 text-xs text-[#8b91a3]">
            <span>Config fields: {Array.isArray(props.item.configSchema) ? props.item.configSchema.length : 0}</span>
            <span>Forum fields: {Array.isArray(props.item.forms?.forumSend) ? props.item.forms.forumSend.length : 0}</span>
            <span>Manual run: {Array.isArray(props.item.forms?.manualRun) ? props.item.forms.manualRun.length : 0}</span>
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <Button type="button" variant="outline" className={OUTLINE_BUTTON_CLASS} onClick={props.onEdit}>Edit</Button>
          <Button type="button" variant="destructive" onClick={props.onDelete}>Delete</Button>
        </div>
      </div>
    </Card>
  );
}

function CaseTemplateCard(props: { item: any; usersByID: Record<string, any>; onEdit: () => void; onDelete: () => void }) {
  const assigneeName = props.item?.assigneeId ? (props.usersByID[props.item.assigneeId]?.name || props.usersByID[props.item.assigneeId]?.email || props.item.assigneeId) : "Unassigned";
  return (
    <Card className={`${CARD_CLASS} p-4`} data-testid={`card-case-template-${props.item.id}`}>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-sm font-semibold text-[#f3f4f6]">{props.item.name || "Untitled case template"}</p>
          <p className="mt-1 text-sm text-[#c8ced9]">{props.item.description || "No description"}</p>
          <div className="mt-3 flex flex-wrap gap-2">
            <Badge variant="outline" className="border-[#2a3347] bg-[#0b1018] text-[#dbe4f0]">{props.item.severity || "Medium"}</Badge>
            <Badge variant="outline" className="border-[#2a3347] bg-[#0b1018] text-[#dbe4f0]">{props.item.status || "new"}</Badge>
            <Badge variant="outline" className="border-[#2a3347] bg-[#0b1018] text-[#dbe4f0]">Priority {props.item.priority || "medium"}</Badge>
            <Badge variant="outline" className="border-[#2a3347] bg-[#0b1018] text-[#dbe4f0]">TLP {String(props.item.tlp || "amber").toUpperCase()}</Badge>
            <Badge variant="outline" className="border-[#2a3347] bg-[#0b1018] text-[#dbe4f0]">PAP {String(props.item.pap || "amber").toUpperCase()}</Badge>
          </div>
          <div className="mt-3 grid gap-2 sm:grid-cols-2 text-xs text-[#8b91a3]">
            <span>Source: {props.item.source || "manual"}</span>
            <span>Incident type: {props.item.incidentType || "-"}</span>
            <span>Confidence: {Number(props.item.confidence ?? 0)}</span>
            <span>Assignee: {assigneeName}</span>
            <span>Tasks: {Array.isArray(props.item.tasks) ? props.item.tasks.length : 0}</span>
            <span>Custom fields: {props.item.customFields ? Object.keys(props.item.customFields).length : 0}</span>
          </div>
          <div className="mt-3 flex flex-wrap gap-2">
            {Array.isArray(props.item.tags) ? props.item.tags.map((tag: string) => (
              <Badge key={`${props.item.id}-${tag}`} variant="outline" className="border-[#36511d] bg-[#15210f] text-[#c8f3bc]">#{tag}</Badge>
            )) : null}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <Button type="button" variant="outline" className={OUTLINE_BUTTON_CLASS} onClick={props.onEdit}>Edit</Button>
          <Button type="button" variant="destructive" onClick={props.onDelete}>Delete</Button>
        </div>
      </div>
    </Card>
  );
}

export default function TemplatesPage() {
  const { currentTenantId } = useAppState();
  const { data: connectorTemplates = [], isLoading: connectorTemplatesLoading } = useConnectorTemplates(currentTenantId);
  const { data: communicationTemplates = [], isLoading: communicationTemplatesLoading } = useCommunicationTemplates(currentTenantId);
  const { data: caseTemplates = [], isLoading: caseTemplatesLoading } = useCaseTemplates(currentTenantId);
  const { data: observableTypes = [], isLoading: observableTypesLoading } = useObservableTypes(currentTenantId);
  const { data: users = [] } = useUsers(currentTenantId);
  const { data: caseStatuses = [] } = useCaseStatuses(currentTenantId);

  const createConnectorTemplate = useCreateConnectorTemplate();
  const updateConnectorTemplate = useUpdateConnectorTemplate();
  const deleteConnectorTemplate = useDeleteConnectorTemplate();
  const createCommunicationTemplate = useCreateCommunicationTemplate();
  const updateCommunicationTemplate = useUpdateCommunicationTemplate();
  const deleteCommunicationTemplate = useDeleteCommunicationTemplate();
  const createCaseTemplate = useCreateCaseTemplate();
  const updateCaseTemplate = useUpdateCaseTemplate();
  const deleteCaseTemplate = useDeleteCaseTemplate();
  const createObservableType = useCreateObservableType();
  const updateObservableType = useUpdateObservableType();
  const deleteObservableType = useDeleteObservableType();

  const [editingConnectorTemplateId, setEditingConnectorTemplateId] = useState("");
  const [connectorTemplateForm, setConnectorTemplateForm] = useState<ConnectorTemplateForm>(() => createEmptyConnectorTemplateForm("HTTP"));
  const [communicationTemplateForm, setCommunicationTemplateForm] = useState<CommunicationTemplateForm>(createEmptyCommunicationTemplateForm());
  const [editingCommunicationTemplateId, setEditingCommunicationTemplateId] = useState("");
  const [editingCaseTemplateId, setEditingCaseTemplateId] = useState("");
  const [caseTemplateForm, setCaseTemplateForm] = useState<CaseTemplateForm>(emptyCaseTemplateForm());
  const [editingObservableTypeId, setEditingObservableTypeId] = useState("");
  const [observableTypeForm, setObservableTypeForm] = useState<ObservableTypeForm>(createEmptyObservableTypeForm());

  const [connectorTemplatePane, setConnectorTemplatePane] = useState<EditorPane>("list");
  const [connectorTemplateEditorOpen, setConnectorTemplateEditorOpen] = useState(false);
  const [communicationTemplatePane, setCommunicationTemplatePane] = useState<EditorPane>("list");
  const [communicationTemplateEditorOpen, setCommunicationTemplateEditorOpen] = useState(false);
  const [caseTemplatePane, setCaseTemplatePane] = useState<EditorPane>("list");
  const [caseTemplateEditorOpen, setCaseTemplateEditorOpen] = useState(false);
  const [observableTypePane, setObservableTypePane] = useState<EditorPane>("list");
  const [observableTypeEditorOpen, setObservableTypeEditorOpen] = useState(false);

  const isPageLoading = connectorTemplatesLoading || communicationTemplatesLoading || caseTemplatesLoading || observableTypesLoading;
  const showPageSkeleton = useMinimumLoading(isPageLoading);
  const usersByID = useMemo(() => Object.fromEntries(users.map((user: any) => [String(user.id), user])), [users]);
  const statusOptions = caseStatuses.length > 0 ? caseStatuses : DEFAULT_CASE_STATUS_OPTIONS;

  const handleLoadConnectorPreset = (presetKey: string) => {
    const preset = BUILTIN_CONNECTOR_TEMPLATE_PRESETS.find((item) => item.presetKey === presetKey);
    if (!preset) {
      return;
    }
    const next = cloneConnectorTemplatePreset(preset);
    setEditingConnectorTemplateId("");
    setConnectorTemplateForm({
      name: next.name,
      description: next.description,
      type: next.type,
      channel: next.channel,
      category: next.category,
      communicationMode: next.communication_mode || "none",
      capabilities: [...next.capabilities],
      tags: [...next.tags],
      icon: next.icon,
      configSchema: cloneSchema(next.config_schema),
      forms: {
        manualRun: cloneSchema(next.forms.manual_run),
        forumSend: cloneSchema(next.forms.forum_send),
        caseCommunication: cloneSchema(next.forms.case_communication),
      },
    });
    setConnectorTemplateEditorOpen(true);
    setConnectorTemplatePane("editor");
  };

  const handleConnectorTypeChange = (type: string) => {
    const preset = findConnectorTemplatePresetByType(type);
    setConnectorTemplateForm((prev) => ({
      ...prev,
      type,
      channel: preset?.channel || prev.channel,
      communicationMode: preset?.communication_mode || prev.communicationMode,
      icon: preset?.icon || prev.icon,
      capabilities: prev.capabilities.length > 0 ? prev.capabilities : preset?.capabilities || ["hub_execute"],
      configSchema: prev.configSchema.length > 0 ? prev.configSchema : cloneSchema(preset?.config_schema || []),
      forms: {
        manualRun: prev.forms.manualRun.length > 0 ? prev.forms.manualRun : cloneSchema(preset?.forms.manual_run || []),
        forumSend: prev.forms.forumSend.length > 0 ? prev.forms.forumSend : cloneSchema(preset?.forms.forum_send || []),
        caseCommunication: prev.forms.caseCommunication.length > 0 ? prev.forms.caseCommunication : cloneSchema(preset?.forms.case_communication || []),
      },
    }));
  };

  const handleSaveConnectorTemplate = () => {
    if (!connectorTemplateForm.name.trim()) {
      toast.error("Connector template name is required");
      return;
    }
    if (!connectorTemplateForm.type.trim()) {
      toast.error("Transport is required");
      return;
    }
    const payload = {
      name: connectorTemplateForm.name.trim(),
      description: connectorTemplateForm.description.trim(),
      type: connectorTemplateForm.type.trim(),
      channel: connectorTemplateForm.channel.trim().toLowerCase(),
      category: connectorTemplateForm.category.trim().toLowerCase(),
      communicationMode: connectorTemplateForm.communicationMode === "none" ? "" : connectorTemplateForm.communicationMode,
      capabilities: uniqueStrings(connectorTemplateForm.capabilities),
      tags: uniqueStrings(connectorTemplateForm.tags),
      icon: connectorTemplateForm.icon,
      configSchema: connectorTemplateForm.configSchema,
      forms: {
        manualRun: connectorTemplateForm.forms.manualRun,
        forumSend: connectorTemplateForm.forms.forumSend,
        caseCommunication: connectorTemplateForm.forms.caseCommunication,
      },
    };

    if (editingConnectorTemplateId) {
      updateConnectorTemplate.mutate(
        { id: editingConnectorTemplateId, data: payload },
        {
          onSuccess: () => {
            toast.success("Connector template updated");
            setEditingConnectorTemplateId("");
            setConnectorTemplateForm(createEmptyConnectorTemplateForm("HTTP"));
            setConnectorTemplateEditorOpen(false);
            setConnectorTemplatePane("list");
          },
          onError: (error: any) => toast.error(error?.message || "Failed to update connector template"),
        },
      );
      return;
    }

    createConnectorTemplate.mutate(payload, {
      onSuccess: () => {
        toast.success("Connector template created");
        setConnectorTemplateForm(createEmptyConnectorTemplateForm("HTTP"));
        setConnectorTemplateEditorOpen(false);
        setConnectorTemplatePane("list");
      },
      onError: (error: any) => toast.error(error?.message || "Failed to create connector template"),
    });
  };

  const handleSaveCommunicationTemplate = () => {
    if (!communicationTemplateForm.name.trim()) {
      toast.error("Communication template name is required");
      return;
    }

    const payload = {
      name: communicationTemplateForm.name.trim(),
      description: communicationTemplateForm.description.trim(),
      channel: communicationTemplateForm.channel,
      subjectTemplate: communicationTemplateForm.subjectTemplate,
      bodyTemplate: communicationTemplateForm.bodyTemplate,
    };

    if (editingCommunicationTemplateId) {
      updateCommunicationTemplate.mutate(
        { id: editingCommunicationTemplateId, data: payload },
        {
          onSuccess: () => {
            toast.success("Communication template updated");
            setEditingCommunicationTemplateId("");
            setCommunicationTemplateForm(createEmptyCommunicationTemplateForm());
            setCommunicationTemplateEditorOpen(false);
            setCommunicationTemplatePane("list");
          },
          onError: (error: any) => toast.error(error?.message || "Failed to update communication template"),
        },
      );
      return;
    }

    createCommunicationTemplate.mutate(payload, {
      onSuccess: () => {
        toast.success("Communication template created");
        setCommunicationTemplateForm(createEmptyCommunicationTemplateForm());
        setCommunicationTemplateEditorOpen(false);
        setCommunicationTemplatePane("list");
      },
      onError: (error: any) => toast.error(error?.message || "Failed to create communication template"),
    });
  };

  const handleAddSOARPresetFields = () => {
    const merged = mergeMissingSOARCaseFields(
      caseTemplateForm.customFields,
      (preset) => createCustomFieldRow({ key: preset.key, value: "" }),
      normalizeSOARCaseFieldKey,
    );
    setCaseTemplateForm((prev) => ({ ...prev, customFields: merged }));
    toast.success(merged.length > caseTemplateForm.customFields.length ? "SOAR fields added" : "SOAR fields already present");
  };

  const handleSaveCaseTemplate = () => {
    if (!caseTemplateForm.name.trim()) {
      toast.error("Case template name is required");
      return;
    }
    let customFieldsPayload: Record<string, string>;
    try {
      customFieldsPayload = normalizeCaseCustomFields(caseTemplateForm.customFields);
    } catch (error: any) {
      toast.error(error?.message || "Invalid custom fields");
      return;
    }

    const payload = {
      name: caseTemplateForm.name.trim(),
      description: caseTemplateForm.description.trim(),
      status: caseTemplateForm.status,
      severity: caseTemplateForm.severity,
      source: caseTemplateForm.source.trim(),
      incidentType: caseTemplateForm.incidentType.trim(),
      priority: caseTemplateForm.priority,
      confidence: Math.max(0, Math.min(100, Number(caseTemplateForm.confidence || 0))),
      tlp: caseTemplateForm.tlp,
      pap: caseTemplateForm.pap,
      detectedAt: localDateTimeToISO(caseTemplateForm.detectedAt),
      occurredAt: localDateTimeToISO(caseTemplateForm.occurredAt),
      assigneeId: caseTemplateForm.assigneeId,
      tags: uniqueStrings(caseTemplateForm.tags),
      tasks: caseTemplateForm.tasks.map((task) => task.value.trim()).filter(Boolean),
      customFields: customFieldsPayload,
    };

    if (editingCaseTemplateId) {
      updateCaseTemplate.mutate(
        { id: editingCaseTemplateId, data: payload },
        {
          onSuccess: () => {
            toast.success("Case template updated");
            setEditingCaseTemplateId("");
            setCaseTemplateForm(emptyCaseTemplateForm());
            setCaseTemplateEditorOpen(false);
            setCaseTemplatePane("list");
          },
          onError: (error: any) => toast.error(error?.message || "Failed to update case template"),
        },
      );
      return;
    }

    createCaseTemplate.mutate(payload, {
      onSuccess: () => {
        toast.success("Case template created");
        setCaseTemplateForm(emptyCaseTemplateForm());
        setCaseTemplateEditorOpen(false);
        setCaseTemplatePane("list");
      },
      onError: (error: any) => toast.error(error?.message || "Failed to create case template"),
    });
  };

  const handleSaveObservableType = () => {
    if (!observableTypeForm.id.trim()) {
      toast.error("Observable type ID is required");
      return;
    }
    if (!observableTypeForm.name.trim()) {
      toast.error("Observable type name is required");
      return;
    }

    const payload = {
      id: observableTypeForm.id.trim(),
      name: observableTypeForm.name.trim(),
      dataType: observableTypeForm.dataType,
      validatorRegex: observableTypeForm.validatorRegex.trim(),
    };

    if (editingObservableTypeId) {
      updateObservableType.mutate(
        { id: editingObservableTypeId, data: payload },
        {
          onSuccess: () => {
            toast.success("Observable type updated");
            setEditingObservableTypeId("");
            setObservableTypeForm(createEmptyObservableTypeForm());
            setObservableTypeEditorOpen(false);
            setObservableTypePane("list");
          },
          onError: (error: any) => toast.error(error?.message || "Failed to update observable type"),
        },
      );
      return;
    }

    createObservableType.mutate(payload, {
      onSuccess: () => {
        toast.success("Observable type created");
        setObservableTypeForm(createEmptyObservableTypeForm());
        setObservableTypeEditorOpen(false);
        setObservableTypePane("list");
      },
      onError: (error: any) => toast.error(error?.message || "Failed to create observable type"),
    });
  };

  if (showPageSkeleton) {
    return (
      <AppLayout>
        <div className={PAGE_SHELL_CLASS}>
          <div className="space-y-2">
            <Skeleton className="h-8 w-56 rounded-lg" />
            <Skeleton className="h-4 w-96 max-w-full rounded-lg" />
          </div>
          <div className={`${PANEL_CLASS} p-6`}>
            <Skeleton className="h-[54px] w-full rounded-xl" />
            <div className="mt-6 grid gap-6 lg:grid-cols-2">
              {Array.from({ length: 4 }).map((_, index) => (
                <Skeleton key={`templates-skeleton-${index}`} className="h-64 rounded-2xl" />
              ))}
            </div>
          </div>
        </div>
      </AppLayout>
    );
  }

  return (
    <AppLayout>
      <div className={PAGE_SHELL_CLASS}>
        <div>
          <h1 className="text-[32px] font-semibold tracking-[-0.5px] text-white">Templates</h1>
          <p className={`mt-1 ${MUTED_TEXT_CLASS}`}>Connector schemas, communication content, case defaults, and observable definitions.</p>
        </div>

        <Tabs defaultValue="connectors" className={PANEL_CLASS}>
          <TabsList className={TABS_LIST_CLASS} data-testid="tabs-templates">
            <TabsTrigger value="connectors" className={TABS_TRIGGER_CLASS} data-testid="tab-connector-templates">Connectors Templates</TabsTrigger>
            <TabsTrigger value="communication" className={TABS_TRIGGER_CLASS} data-testid="tab-communication-templates">Communication Templates</TabsTrigger>
            <TabsTrigger value="case" className={TABS_TRIGGER_CLASS} data-testid="tab-case-templates">Case Templates</TabsTrigger>
            <TabsTrigger value="observable" className={TABS_TRIGGER_CLASS} data-testid="tab-observable-types">Observable Types</TabsTrigger>
          </TabsList>

          <TabsContent value="connectors" className="m-0 px-6 py-6">
            <Tabs value={connectorTemplatePane} onValueChange={(value) => setConnectorTemplatePane(value as EditorPane)} className="space-y-6">
              {connectorTemplateEditorOpen ? (
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <TabsList className="h-auto rounded-2xl border border-[#2a3347] bg-[#0b1018] p-1.5">
                    <TabsTrigger value="list" className="rounded-xl px-4 py-2 text-sm data-[state=active]:bg-[#131b25] data-[state=active]:text-white">
                      Saved templates
                    </TabsTrigger>
                    <TabsTrigger value="editor" className="max-w-[48vw] rounded-xl px-4 py-2 text-sm data-[state=active]:bg-[#131b25] data-[state=active]:text-white">
                      <span className="truncate">{editingConnectorTemplateId ? (connectorTemplateForm.name.trim() || "Edit template") : "New template"}</span>
                    </TabsTrigger>
                  </TabsList>
                  <Button
                    type="button"
                    size="icon"
                    variant="outline"
                    className="h-10 w-10 rounded-2xl border-[#2a3347] bg-[#0b1018] text-[#9ca3af] hover:bg-[#131b25] hover:text-white"
                    onClick={() => {
                      setConnectorTemplateEditorOpen(false);
                      setConnectorTemplatePane("list");
                      setEditingConnectorTemplateId("");
                      setConnectorTemplateForm(createEmptyConnectorTemplateForm("HTTP"));
                    }}
                    aria-label="Close"
                    title="Close"
                    data-testid="button-close-connector-template-editor"
                  >
                    <X size={16} />
                  </Button>
                </div>
              ) : null}

              <TabsContent value="list" className="m-0 space-y-6">
                <div className="flex flex-wrap items-start justify-between gap-4">
                  <div>
                    <h2 className="text-[28px] font-semibold tracking-[-0.4px] text-white">Connectors Templates</h2>
                    <p className="mt-1 text-sm text-[#9ca3af]">Reusable connector definitions with icons, tags, capabilities, and schema-driven forms.</p>
                  </div>
                  <Button
                    type="button"
                    className={PRIMARY_BUTTON_CLASS}
                    onClick={() => {
                      setEditingConnectorTemplateId("");
                      setConnectorTemplateForm(createEmptyConnectorTemplateForm("HTTP"));
                      setConnectorTemplateEditorOpen(true);
                      setConnectorTemplatePane("editor");
                    }}
                    data-testid="button-new-connector-template"
                  >
                    <Plus size={16} className="mr-2" /> New template
                  </Button>
                </div>

                <div className="space-y-4">
                  <h3 className="text-lg font-semibold text-[#f3f4f6]">Saved connector templates</h3>
                  {connectorTemplates.length === 0 ? (
                    <Card className={`${CARD_CLASS} p-8 text-center text-[#8b91a3]`}>No saved connector templates yet. Load a preset and save it.</Card>
                  ) : (
                    connectorTemplates.map((item: any) => (
                      <ConnectorTemplateCard
                        key={item.id}
                        item={item}
                        onEdit={() => {
                          setEditingConnectorTemplateId(String(item.id));
                          setConnectorTemplateForm(connectorTemplateFormFromItem(item));
                          setConnectorTemplateEditorOpen(true);
                          setConnectorTemplatePane("editor");
                        }}
                        onDelete={() => {
                          deleteConnectorTemplate.mutate(String(item.id), {
                            onSuccess: () => toast.success("Connector template deleted"),
                            onError: (error: any) => toast.error(error?.message || "Failed to delete connector template"),
                          });
                        }}
                      />
                    ))
                  )}
                </div>
              </TabsContent>

              <TabsContent value="editor" className="m-0">
                <div className="mx-auto w-full max-w-[920px] space-y-6 pb-12">
                  <div className="rounded-[24px] border border-[#2a3347] bg-[#0b1018] px-6 py-6 shadow-[0_18px_44px_rgba(0,0,0,0.28)]">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-[#7ddf8a]">Connector template editor</div>
                    <h2 className="mt-2 text-[26px] font-semibold tracking-[-0.04em] text-white">{editingConnectorTemplateId ? "Edit template" : "New template"}</h2>
                    <p className="mt-2 max-w-[760px] text-sm text-[#98a4b8]">Define config schema and runtime forms for Connector Hub, forum send, and case communications.</p>
                  </div>

                  <div className="space-y-5">
                    <div className="space-y-2">
                      <Label>Start from preset</Label>
                      <Select onValueChange={handleLoadConnectorPreset}>
                        <SelectTrigger className={SELECT_TRIGGER_CLASS} data-testid="select-connector-template-preset">
                          <SelectValue placeholder="Choose preset" />
                        </SelectTrigger>
                        <SelectContent className={SELECT_CONTENT_CLASS}>
                          {BUILTIN_CONNECTOR_TEMPLATE_PRESETS.map((preset) => (
                            <SelectItem key={preset.presetKey} value={preset.presetKey}>
                              {preset.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>

                    <div className="space-y-6">
                      <Card className={`${CARD_CLASS} p-5`}>
                        <div className="mb-5 flex items-center justify-between gap-3">
                          <div>
                            <h3 className="text-lg font-semibold text-[#f3f4f6]">{editingConnectorTemplateId ? "Edit connector template" : "Create connector template"}</h3>
                            <p className="mt-1 text-sm text-[#8b91a3]">Built-in or custom connectors use the same schema model.</p>
                          </div>
                          <Button
                            type="button"
                            variant="outline"
                            className={OUTLINE_BUTTON_CLASS}
                            onClick={() => {
                              setEditingConnectorTemplateId("");
                              setConnectorTemplateForm(createEmptyConnectorTemplateForm("HTTP"));
                              setConnectorTemplateEditorOpen(false);
                              setConnectorTemplatePane("list");
                            }}
                          >
                            <RotateCcw size={14} className="mr-2" /> Cancel
                          </Button>
                        </div>

                        <div className="grid gap-4 md:grid-cols-2">
                          <div className="space-y-2">
                            <Label>Name</Label>
                            <Input value={connectorTemplateForm.name} onChange={(event) => setConnectorTemplateForm((prev) => ({ ...prev, name: event.target.value }))} className={INPUT_CLASS} data-testid="input-connector-template-name" />
                          </div>
                          <div className="space-y-2">
                            <Label>Transport</Label>
                            <Select value={connectorTemplateForm.type} onValueChange={handleConnectorTypeChange}>
                              <SelectTrigger className={SELECT_TRIGGER_CLASS} data-testid="select-connector-template-type"><SelectValue /></SelectTrigger>
                              <SelectContent className={SELECT_CONTENT_CLASS}>
                                {BUILTIN_CONNECTOR_TEMPLATE_PRESETS.map((preset) => (
                                  <SelectItem key={`connector-template-type-${preset.type}`} value={preset.type}>{preset.type}</SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          </div>
                          <div className="space-y-2 md:col-span-2">
                            <Label>Description</Label>
                            <Textarea value={connectorTemplateForm.description} onChange={(event) => setConnectorTemplateForm((prev) => ({ ...prev, description: event.target.value }))} rows={3} className={TEXTAREA_CLASS} />
                          </div>
                          <div className="space-y-2">
                            <Label>Channel</Label>
                            <Input value={connectorTemplateForm.channel} onChange={(event) => setConnectorTemplateForm((prev) => ({ ...prev, channel: event.target.value }))} className={INPUT_CLASS} />
                          </div>
                          <div className="space-y-2">
                            <Label>Category</Label>
                            <Select value={connectorTemplateForm.category} onValueChange={(value) => setConnectorTemplateForm((prev) => ({ ...prev, category: value }))}>
                              <SelectTrigger className={SELECT_TRIGGER_CLASS}><SelectValue /></SelectTrigger>
                              <SelectContent className={SELECT_CONTENT_CLASS}>
                                {CATEGORY_OPTIONS.map((option) => <SelectItem key={`connector-category-${option}`} value={option}>{option}</SelectItem>)}
                              </SelectContent>
                            </Select>
                          </div>
                          <div className="space-y-2">
                            <Label>Communication mode</Label>
                            <Select value={connectorTemplateForm.communicationMode || "none"} onValueChange={(value) => setConnectorTemplateForm((prev) => ({ ...prev, communicationMode: value }))}>
                              <SelectTrigger className={SELECT_TRIGGER_CLASS}><SelectValue /></SelectTrigger>
                              <SelectContent className={SELECT_CONTENT_CLASS}>
                                {COMMUNICATION_MODE_OPTIONS.map((option) => <SelectItem key={`connector-mode-${option}`} value={option}>{option}</SelectItem>)}
                              </SelectContent>
                            </Select>
                          </div>
                          <div className="space-y-2">
                            <Label>Icon</Label>
                            <Select value={connectorTemplateForm.icon} onValueChange={(value) => setConnectorTemplateForm((prev) => ({ ...prev, icon: value }))}>
                              <SelectTrigger className={SELECT_TRIGGER_CLASS} data-testid="select-connector-template-icon"><SelectValue placeholder="Choose icon" /></SelectTrigger>
                              <SelectContent className={SELECT_CONTENT_CLASS}>
                                {CONNECTOR_TEMPLATE_ICON_OPTIONS.map((option) => (
                                  <SelectItem key={`connector-icon-${option.value}`} value={option.value}>{option.label}</SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                            <div className="flex items-center gap-2 text-xs text-[#8b91a3]">
                              <span className="flex h-7 w-7 items-center justify-center rounded-lg border border-[#2a3347] bg-[#0b1018] text-[#d1d5db]">{renderIcon(connectorTemplateForm.icon)}</span>
                              <span>{iconLabel(connectorTemplateForm.icon)}</span>
                            </div>
                          </div>
                        </div>

                        <div className="mt-5 space-y-4">
                          <TagInput
                            label="Tags"
                            value={connectorTemplateForm.tags}
                            onChange={(tags) => setConnectorTemplateForm((prev) => ({ ...prev, tags }))}
                            dataTestId="input-connector-template-tags"
                          />

                          <div className="space-y-2">
                            <Label>Capabilities</Label>
                            <div className="grid gap-2 sm:grid-cols-2">
                              {CAPABILITY_OPTIONS.map((option) => {
                                const checked = connectorTemplateForm.capabilities.includes(option.value);
                                return (
                                  <button
                                    key={option.value}
                                    type="button"
                                    className={`flex items-center gap-3 rounded-2xl border px-4 py-3 text-left text-sm transition-colors ${checked ? "border-[#66ff4c] bg-[#162112] text-[#c8f3bc]" : "border-[#2a3347] bg-[#0b1018] text-[#d1d5db] hover:border-[#42516a]"}`}
                                    onClick={() =>
                                      setConnectorTemplateForm((prev) => ({
                                        ...prev,
                                        capabilities: checked
                                          ? prev.capabilities.filter((item) => item !== option.value)
                                          : uniqueStrings([...prev.capabilities, option.value]),
                                      }))
                                    }
                                  >
                                    <span className={`flex h-4 w-4 items-center justify-center rounded border ${checked ? "border-[#66ff4c] bg-[#66ff4c] text-[#08110a]" : "border-current bg-transparent text-transparent"}`}>
                                      <span className="text-[10px] leading-none">{checked ? "x" : ""}</span>
                                    </span>
                                    <span>{option.label}</span>
                                  </button>
                                );
                              })}
                            </div>
                          </div>
                        </div>
                      </Card>

                      <SchemaFieldListEditor
                        title="Config schema"
                        description="Fields shown when creating or editing connector instances."
                        fields={connectorTemplateForm.configSchema}
                        onChange={(next) => setConnectorTemplateForm((prev) => ({ ...prev, configSchema: next }))}
                        dataTestIdPrefix="connector-template-config-schema"
                      />
                      <SchemaFieldListEditor
                        title="Manual run form"
                        description="Fields used by the global manual connector runner."
                        fields={connectorTemplateForm.forms.manualRun}
                        onChange={(next) => setConnectorTemplateForm((prev) => ({ ...prev, forms: { ...prev.forms, manualRun: next } }))}
                        dataTestIdPrefix="connector-template-manual-run"
                      />
                      <SchemaFieldListEditor
                        title="Forum send form"
                        description="Fields rendered in forum send-via-connector flow."
                        fields={connectorTemplateForm.forms.forumSend}
                        onChange={(next) => setConnectorTemplateForm((prev) => ({ ...prev, forms: { ...prev.forms, forumSend: next } }))}
                        dataTestIdPrefix="connector-template-forum-send"
                      />
                      <SchemaFieldListEditor
                        title="Case communication form"
                        description="Fields rendered inside case communications."
                        fields={connectorTemplateForm.forms.caseCommunication}
                        onChange={(next) => setConnectorTemplateForm((prev) => ({ ...prev, forms: { ...prev.forms, caseCommunication: next } }))}
                        dataTestIdPrefix="connector-template-case-communication"
                      />

                      <div className="flex gap-3">
                        <Button type="button" className={PRIMARY_BUTTON_CLASS} onClick={handleSaveConnectorTemplate} data-testid="button-save-connector-template">
                          <Save size={16} className="mr-2" /> {editingConnectorTemplateId ? "Update template" : "Save template"}
                        </Button>
                      </div>
                    </div>
                  </div>
                </div>
              </TabsContent>
            </Tabs>
          </TabsContent>
          <TabsContent value="communication" className="m-0 px-6 py-6">
            <Tabs value={communicationTemplatePane} onValueChange={(value) => setCommunicationTemplatePane(value as EditorPane)} className="space-y-6">
              {communicationTemplateEditorOpen ? (
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <TabsList className="h-auto rounded-2xl border border-[#2a3347] bg-[#0b1018] p-1.5">
                    <TabsTrigger value="list" className="rounded-xl px-4 py-2 text-sm data-[state=active]:bg-[#131b25] data-[state=active]:text-white">
                      Saved templates
                    </TabsTrigger>
                    <TabsTrigger value="editor" className="max-w-[48vw] rounded-xl px-4 py-2 text-sm data-[state=active]:bg-[#131b25] data-[state=active]:text-white">
                      <span className="truncate">{editingCommunicationTemplateId ? (communicationTemplateForm.name.trim() || "Edit template") : "New template"}</span>
                    </TabsTrigger>
                  </TabsList>
                  <Button
                    type="button"
                    size="icon"
                    variant="outline"
                    className="h-10 w-10 rounded-2xl border-[#2a3347] bg-[#0b1018] text-[#9ca3af] hover:bg-[#131b25] hover:text-white"
                    onClick={() => {
                      setCommunicationTemplateEditorOpen(false);
                      setCommunicationTemplatePane("list");
                      setEditingCommunicationTemplateId("");
                      setCommunicationTemplateForm(createEmptyCommunicationTemplateForm());
                    }}
                    aria-label="Close"
                    title="Close"
                    data-testid="button-close-communication-template-editor"
                  >
                    <X size={16} />
                  </Button>
                </div>
              ) : null}

              <TabsContent value="list" className="m-0 space-y-6">
                <div className="flex flex-wrap items-start justify-between gap-4">
                  <div>
                    <h2 className="text-[28px] font-semibold tracking-[-0.4px] text-white">Communication Templates</h2>
                    <p className="mt-1 text-sm text-[#9ca3af]">Reusable email and chat content for communication workflows.</p>
                  </div>
                  <Button
                    type="button"
                    className={PRIMARY_BUTTON_CLASS}
                    onClick={() => {
                      setEditingCommunicationTemplateId("");
                      setCommunicationTemplateForm(createEmptyCommunicationTemplateForm());
                      setCommunicationTemplateEditorOpen(true);
                      setCommunicationTemplatePane("editor");
                    }}
                    data-testid="button-new-communication-template"
                  >
                    <Plus size={16} className="mr-2" /> New template
                  </Button>
                </div>

                <div className="space-y-4">
                  {communicationTemplates.length === 0 ? (
                    <Card className={`${CARD_CLASS} p-8 text-center text-[#8b91a3]`}>No communication templates created yet.</Card>
                  ) : (
                    communicationTemplates.map((template: any) => (
                      <Card key={template.id} className={`${CARD_CLASS} p-4`} data-testid={`card-communication-template-${template.id}`}>
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0">
                            <p className="text-sm font-semibold text-[#f3f4f6]">{template.name}</p>
                            <p className="mt-1 text-sm text-[#c8ced9]">{template.description || "No description"}</p>
                            <div className="mt-3 flex flex-wrap gap-2">
                              <Badge variant="outline" className="border-[#2a3347] bg-[#0b1018] text-[#dbe4f0]">{template.channel}</Badge>
                            </div>
                            {template.subjectTemplate ? <p className="mt-3 text-xs text-[#9ca3af]">Subject: {template.subjectTemplate}</p> : null}
                          </div>
                          <div className="flex shrink-0 items-center gap-2">
                            <Button
                              type="button"
                              variant="outline"
                              className={OUTLINE_BUTTON_CLASS}
                              onClick={() => {
                                setEditingCommunicationTemplateId(String(template.id));
                                setCommunicationTemplateForm({
                                  name: template.name,
                                  description: template.description,
                                  channel: template.channel,
                                  subjectTemplate: template.subjectTemplate,
                                  bodyTemplate: template.bodyTemplate,
                                });
                                setCommunicationTemplateEditorOpen(true);
                                setCommunicationTemplatePane("editor");
                              }}
                            >
                              Edit
                            </Button>
                            <Button
                              type="button"
                              variant="destructive"
                              onClick={() => {
                                deleteCommunicationTemplate.mutate(String(template.id), {
                                  onSuccess: () => toast.success("Communication template deleted"),
                                  onError: (error: any) => toast.error(error?.message || "Failed to delete communication template"),
                                });
                              }}
                            >
                              Delete
                            </Button>
                          </div>
                        </div>
                        <pre className="mt-3 overflow-auto whitespace-pre-wrap rounded-2xl border border-[#2a3347] bg-[#0b1018] p-3 text-xs text-[#d1d5db]">{template.bodyTemplate || "No body"}</pre>
                      </Card>
                    ))
                  )}
                </div>
              </TabsContent>

              <TabsContent value="editor" className="m-0">
                <div className="mx-auto w-full max-w-[820px] space-y-6 pb-12">
                  <div className="rounded-[24px] border border-[#2a3347] bg-[#0b1018] px-6 py-6 shadow-[0_18px_44px_rgba(0,0,0,0.28)]">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-[#7ddf8a]">Communication template editor</div>
                    <h2 className="mt-2 text-[26px] font-semibold tracking-[-0.04em] text-white">{editingCommunicationTemplateId ? "Edit template" : "New template"}</h2>
                    <p className="mt-2 max-w-[760px] text-sm text-[#98a4b8]">Templates used for email/chat send flows.</p>
                  </div>

                  <Card className={`${CARD_CLASS} p-5`}>
                    <div className="grid gap-4">
                      <div className="space-y-2">
                        <Label>Name</Label>
                        <Input value={communicationTemplateForm.name} onChange={(event) => setCommunicationTemplateForm((prev) => ({ ...prev, name: event.target.value }))} className={INPUT_CLASS} />
                      </div>
                      <div className="space-y-2">
                        <Label>Channel</Label>
                        <Select value={communicationTemplateForm.channel} onValueChange={(value) => setCommunicationTemplateForm((prev) => ({ ...prev, channel: value }))}>
                          <SelectTrigger className={SELECT_TRIGGER_CLASS}><SelectValue /></SelectTrigger>
                          <SelectContent className={SELECT_CONTENT_CLASS}>
                            {COMMUNICATION_CHANNEL_OPTIONS.map((option) => <SelectItem key={`communication-channel-${option}`} value={option}>{option}</SelectItem>)}
                          </SelectContent>
                        </Select>
                      </div>
                      <div className="space-y-2">
                        <Label>Description</Label>
                        <Textarea value={communicationTemplateForm.description} onChange={(event) => setCommunicationTemplateForm((prev) => ({ ...prev, description: event.target.value }))} rows={3} className={TEXTAREA_CLASS} />
                      </div>
                      <div className="space-y-2">
                        <Label>Subject template</Label>
                        <Input value={communicationTemplateForm.subjectTemplate} onChange={(event) => setCommunicationTemplateForm((prev) => ({ ...prev, subjectTemplate: event.target.value }))} className={INPUT_CLASS} />
                      </div>
                      <div className="space-y-2">
                        <Label>Body template</Label>
                        <Textarea value={communicationTemplateForm.bodyTemplate} onChange={(event) => setCommunicationTemplateForm((prev) => ({ ...prev, bodyTemplate: event.target.value }))} rows={6} className={TEXTAREA_CLASS} />
                      </div>
                      <div className="flex flex-wrap items-center gap-3">
                        <Button type="button" className={PRIMARY_BUTTON_CLASS} onClick={handleSaveCommunicationTemplate}>
                          <Save size={16} className="mr-2" /> Save communication template
                        </Button>
                        <Button
                          type="button"
                          variant="outline"
                          className={OUTLINE_BUTTON_CLASS}
                          onClick={() => {
                            setCommunicationTemplateEditorOpen(false);
                            setCommunicationTemplatePane("list");
                            setEditingCommunicationTemplateId("");
                            setCommunicationTemplateForm(createEmptyCommunicationTemplateForm());
                          }}
                        >
                          Cancel
                        </Button>
                      </div>
                    </div>
                  </Card>
                </div>
              </TabsContent>
            </Tabs>
          </TabsContent>
          <TabsContent value="case" className="m-0 px-6 py-6">
            <Tabs value={caseTemplatePane} onValueChange={(value) => setCaseTemplatePane(value as EditorPane)} className="space-y-6">
              {caseTemplateEditorOpen ? (
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <TabsList className="h-auto rounded-2xl border border-[#2a3347] bg-[#0b1018] p-1.5">
                    <TabsTrigger value="list" className="rounded-xl px-4 py-2 text-sm data-[state=active]:bg-[#131b25] data-[state=active]:text-white">
                      Saved templates
                    </TabsTrigger>
                    <TabsTrigger value="editor" className="max-w-[48vw] rounded-xl px-4 py-2 text-sm data-[state=active]:bg-[#131b25] data-[state=active]:text-white">
                      <span className="truncate">{editingCaseTemplateId ? (caseTemplateForm.name.trim() || "Edit template") : "New template"}</span>
                    </TabsTrigger>
                  </TabsList>
                  <Button
                    type="button"
                    size="icon"
                    variant="outline"
                    className="h-10 w-10 rounded-2xl border-[#2a3347] bg-[#0b1018] text-[#9ca3af] hover:bg-[#131b25] hover:text-white"
                    onClick={() => {
                      setCaseTemplateEditorOpen(false);
                      setCaseTemplatePane("list");
                      setEditingCaseTemplateId("");
                      setCaseTemplateForm(emptyCaseTemplateForm());
                    }}
                    aria-label="Close"
                    title="Close"
                    data-testid="button-close-case-template-editor"
                  >
                    <X size={16} />
                  </Button>
                </div>
              ) : null}

              <TabsContent value="list" className="m-0 space-y-6">
                <div className="flex flex-wrap items-start justify-between gap-4">
                  <div>
                    <h2 className="text-[28px] font-semibold tracking-[-0.4px] text-white">Case Templates</h2>
                    <p className="mt-1 text-sm text-[#9ca3af]">Template defaults now mirror the case creation form much more closely.</p>
                  </div>
                  <Button
                    type="button"
                    className={PRIMARY_BUTTON_CLASS}
                    onClick={() => {
                      setEditingCaseTemplateId("");
                      setCaseTemplateForm(emptyCaseTemplateForm());
                      setCaseTemplateEditorOpen(true);
                      setCaseTemplatePane("editor");
                    }}
                    data-testid="button-new-case-template"
                  >
                    <Plus size={16} className="mr-2" /> New template
                  </Button>
                </div>

                <div className="space-y-4">
                  <h3 className="text-lg font-semibold text-[#f3f4f6]">Saved case templates</h3>
                  {caseTemplates.length === 0 ? (
                    <Card className={`${CARD_CLASS} p-8 text-center text-[#8b91a3]`}>No case templates created yet.</Card>
                  ) : (
                    caseTemplates.map((item: any) => (
                      <CaseTemplateCard
                        key={item.id}
                        item={item}
                        usersByID={usersByID}
                        onEdit={() => {
                          setEditingCaseTemplateId(String(item.id));
                          setCaseTemplateForm(caseTemplateFormFromItem(item));
                          setCaseTemplateEditorOpen(true);
                          setCaseTemplatePane("editor");
                        }}
                        onDelete={() => {
                          deleteCaseTemplate.mutate(String(item.id), {
                            onSuccess: () => toast.success("Case template deleted"),
                            onError: (error: any) => toast.error(error?.message || "Failed to delete case template"),
                          });
                        }}
                      />
                    ))
                  )}
                </div>
              </TabsContent>

              <TabsContent value="editor" className="m-0">
                <div className="mx-auto w-full max-w-[980px] space-y-6 pb-12">
                  <div className="rounded-[24px] border border-[#2a3347] bg-[#0b1018] px-6 py-6 shadow-[0_18px_44px_rgba(0,0,0,0.28)]">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-[#7ddf8a]">Case template editor</div>
                    <h2 className="mt-2 text-[26px] font-semibold tracking-[-0.04em] text-white">{editingCaseTemplateId ? "Edit template" : "New template"}</h2>
                    <p className="mt-2 max-w-[820px] text-sm text-[#98a4b8]">Case defaults, tags, tasks, and custom fields.</p>
                  </div>

                  <div className="space-y-6">
                    <Card className={`${CARD_CLASS} p-5`}>
                  <div className="grid gap-4 md:grid-cols-2">
                    <div className="space-y-2">
                      <Label>Name</Label>
                      <Input value={caseTemplateForm.name} onChange={(event) => setCaseTemplateForm((prev) => ({ ...prev, name: event.target.value }))} className={INPUT_CLASS} data-testid="input-case-template-name" />
                    </div>
                    <div className="space-y-2">
                      <Label>Severity</Label>
                      <Select value={caseTemplateForm.severity} onValueChange={(value) => setCaseTemplateForm((prev) => ({ ...prev, severity: value }))}>
                        <SelectTrigger className={SELECT_TRIGGER_CLASS}><SelectValue /></SelectTrigger>
                        <SelectContent className={SELECT_CONTENT_CLASS}>
                          {SEVERITY_OPTIONS.map((option) => <SelectItem key={`case-template-severity-${option}`} value={option}>{option}</SelectItem>)}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2 md:col-span-2">
                      <Label>Description</Label>
                      <Textarea value={caseTemplateForm.description} onChange={(event) => setCaseTemplateForm((prev) => ({ ...prev, description: event.target.value }))} rows={4} className={TEXTAREA_CLASS} placeholder="Describe the case template..." />
                    </div>
                    <div className="space-y-2">
                      <Label>Status</Label>
                      <Select value={caseTemplateForm.status} onValueChange={(value) => setCaseTemplateForm((prev) => ({ ...prev, status: value }))}>
                        <SelectTrigger className={SELECT_TRIGGER_CLASS}><SelectValue /></SelectTrigger>
                        <SelectContent className={SELECT_CONTENT_CLASS}>
                          {statusOptions.map((status: any) => <SelectItem key={`case-template-status-${status.code}`} value={status.code}>{status.label}</SelectItem>)}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2">
                      <Label>Priority</Label>
                      <Select value={caseTemplateForm.priority} onValueChange={(value) => setCaseTemplateForm((prev) => ({ ...prev, priority: value }))}>
                        <SelectTrigger className={SELECT_TRIGGER_CLASS}><SelectValue /></SelectTrigger>
                        <SelectContent className={SELECT_CONTENT_CLASS}>
                          {PRIORITY_OPTIONS.map((option) => <SelectItem key={`case-template-priority-${option}`} value={option}>{option}</SelectItem>)}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2">
                      <Label>Source</Label>
                      <Input value={caseTemplateForm.source} onChange={(event) => setCaseTemplateForm((prev) => ({ ...prev, source: event.target.value }))} className={INPUT_CLASS} />
                    </div>
                    <div className="space-y-2">
                      <Label>Incident type</Label>
                      <Input value={caseTemplateForm.incidentType} onChange={(event) => setCaseTemplateForm((prev) => ({ ...prev, incidentType: event.target.value }))} className={INPUT_CLASS} />
                    </div>
                    <div className="space-y-2">
                      <Label>Confidence</Label>
                      <Input type="number" min={0} max={100} value={caseTemplateForm.confidence} onChange={(event) => setCaseTemplateForm((prev) => ({ ...prev, confidence: Math.max(0, Math.min(100, Number(event.target.value || 0))) }))} className={INPUT_CLASS} />
                    </div>
                    <div className="space-y-2">
                      <Label>Assignee</Label>
                      <Select value={caseTemplateForm.assigneeId || "none"} onValueChange={(value) => setCaseTemplateForm((prev) => ({ ...prev, assigneeId: value === "none" ? "" : value }))}>
                        <SelectTrigger className={SELECT_TRIGGER_CLASS}><SelectValue /></SelectTrigger>
                        <SelectContent className={SELECT_CONTENT_CLASS}>
                          <SelectItem value="none">Unassigned</SelectItem>
                          {users.map((user: any) => (
                            <SelectItem key={`case-template-assignee-${user.id}`} value={String(user.id)}>{user.name || user.email || user.id}</SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2">
                      <Label>TLP</Label>
                      <Select value={caseTemplateForm.tlp} onValueChange={(value) => setCaseTemplateForm((prev) => ({ ...prev, tlp: value }))}>
                        <SelectTrigger className={SELECT_TRIGGER_CLASS}><SelectValue /></SelectTrigger>
                        <SelectContent className={SELECT_CONTENT_CLASS}>
                          {TLP_OPTIONS.map((option) => <SelectItem key={`case-template-tlp-${option}`} value={option}>{option.toUpperCase()}</SelectItem>)}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2">
                      <Label>PAP</Label>
                      <Select value={caseTemplateForm.pap} onValueChange={(value) => setCaseTemplateForm((prev) => ({ ...prev, pap: value }))}>
                        <SelectTrigger className={SELECT_TRIGGER_CLASS}><SelectValue /></SelectTrigger>
                        <SelectContent className={SELECT_CONTENT_CLASS}>
                          {PAP_OPTIONS.map((option) => <SelectItem key={`case-template-pap-${option}`} value={option}>{option.toUpperCase()}</SelectItem>)}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2">
                      <Label>Detected at</Label>
                      <Input type="datetime-local" value={caseTemplateForm.detectedAt} onChange={(event) => setCaseTemplateForm((prev) => ({ ...prev, detectedAt: event.target.value }))} className={INPUT_CLASS} />
                    </div>
                    <div className="space-y-2">
                      <Label>Occurred at</Label>
                      <Input type="datetime-local" value={caseTemplateForm.occurredAt} onChange={(event) => setCaseTemplateForm((prev) => ({ ...prev, occurredAt: event.target.value }))} className={INPUT_CLASS} />
                    </div>
                  </div>
                </Card>

                <TagInput label="Tags" value={caseTemplateForm.tags} onChange={(tags) => setCaseTemplateForm((prev) => ({ ...prev, tags }))} dataTestId="input-case-template-tags" />

                <Card className={`${CARD_CLASS} p-5`}>
                  <div className="mb-4 flex items-center justify-between gap-3">
                    <div>
                      <h3 className="text-sm font-semibold text-[#f3f4f6]">Tasks</h3>
                      <p className="mt-1 text-xs text-[#8b91a3]">Tasks created from the template.</p>
                    </div>
                    <Button type="button" variant="outline" className={OUTLINE_BUTTON_CLASS} onClick={() => setCaseTemplateForm((prev) => ({ ...prev, tasks: [...prev.tasks, createTaskRow()] }))}>
                      <Plus size={14} className="mr-2" /> Add task
                    </Button>
                  </div>
                  <div className="space-y-3">
                    {caseTemplateForm.tasks.map((task, index) => (
                      <div key={task.id} className="flex gap-2" data-testid={`case-template-task-row-${index}`}>
                        <Input value={task.value} onChange={(event) => setCaseTemplateForm((prev) => ({ ...prev, tasks: prev.tasks.map((item) => item.id === task.id ? { ...item, value: event.target.value } : item) }))} className={INPUT_CLASS} data-testid={index === 0 ? "input-case-template-tasks" : undefined} />
                        <Button type="button" variant="outline" className={OUTLINE_BUTTON_CLASS} onClick={() => setCaseTemplateForm((prev) => ({ ...prev, tasks: prev.tasks.filter((item) => item.id !== task.id) }))}>
                          <Trash2 size={14} />
                        </Button>
                      </div>
                    ))}
                  </div>
                </Card>

                <Card className={`${CARD_CLASS} p-5`}>
                  <div className="mb-4 flex items-center justify-between gap-3">
                    <div>
                      <h3 className="text-sm font-semibold text-[#f3f4f6]">Custom fields</h3>
                      <p className="mt-1 text-xs text-[#8b91a3]">SOAR and tenant-specific defaults applied to new cases.</p>
                    </div>
                    <div className="flex gap-2">
                      <Button type="button" variant="outline" className={OUTLINE_BUTTON_CLASS} onClick={handleAddSOARPresetFields} data-testid="button-add-case-template-soar-fields">
                        Add SOAR fields
                      </Button>
                      <Button type="button" variant="outline" className={OUTLINE_BUTTON_CLASS} onClick={() => setCaseTemplateForm((prev) => ({ ...prev, customFields: [...prev.customFields, createCustomFieldRow()] }))}>
                        <Plus size={14} className="mr-2" /> Add field
                      </Button>
                    </div>
                  </div>
                  <div className="space-y-3">
                    {caseTemplateForm.customFields.length === 0 ? (
                      <div className="rounded-2xl border border-dashed border-[#2a3347] px-4 py-5 text-sm text-[#7f8798]">No custom fields yet.</div>
                    ) : null}
                    {caseTemplateForm.customFields.map((row, index) => {
                      const normalizedKey = normalizeSOARCaseFieldKey(row.key);
                      const isSOARField = isSOARCaseFieldKey(normalizedKey);
                      return (
                        <div key={row.id} className="grid gap-2 md:grid-cols-[1fr_1fr_auto]" data-testid={`case-template-custom-field-row-${index}`}>
                          <Input
                            value={row.key}
                            onChange={(event) => setCaseTemplateForm((prev) => ({ ...prev, customFields: prev.customFields.map((item) => item.id === row.id ? { ...item, key: event.target.value } : item) }))}
                            className={INPUT_CLASS}
                            data-testid={`input-case-template-custom-field-key-${index}`}
                          />
                          <Input
                            value={row.value}
                            onChange={(event) => setCaseTemplateForm((prev) => ({ ...prev, customFields: prev.customFields.map((item) => item.id === row.id ? { ...item, value: event.target.value } : item) }))}
                            className={INPUT_CLASS}
                            placeholder={isSOARField ? `${normalizedKey} default value` : "Default value"}
                            data-testid={`input-case-template-custom-field-value-${index}`}
                          />
                          <Button type="button" variant="outline" className={OUTLINE_BUTTON_CLASS} onClick={() => setCaseTemplateForm((prev) => ({ ...prev, customFields: prev.customFields.filter((item) => item.id !== row.id) }))}>
                            <Trash2 size={14} />
                          </Button>
                        </div>
                      );
                    })}
                  </div>
                </Card>

                <div className="flex gap-3">
                  <Button type="button" className={PRIMARY_BUTTON_CLASS} onClick={handleSaveCaseTemplate} data-testid="button-save-case-template">
                    <Save size={16} className="mr-2" /> {editingCaseTemplateId ? "Update case template" : "Save case template"}
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    className={OUTLINE_BUTTON_CLASS}
                    onClick={() => {
                      setCaseTemplateEditorOpen(false);
                      setCaseTemplatePane("list");
                      setEditingCaseTemplateId("");
                      setCaseTemplateForm(emptyCaseTemplateForm());
                    }}
                  >
                    Cancel
                  </Button>
                </div>
              </div>
                </div>
              </TabsContent>
            </Tabs>
          </TabsContent>
          <TabsContent value="observable" className="m-0 px-6 py-6">
            <Tabs value={observableTypePane} onValueChange={(value) => setObservableTypePane(value as EditorPane)} className="space-y-6">
              {observableTypeEditorOpen ? (
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <TabsList className="h-auto rounded-2xl border border-[#2a3347] bg-[#0b1018] p-1.5">
                    <TabsTrigger value="list" className="rounded-xl px-4 py-2 text-sm data-[state=active]:bg-[#131b25] data-[state=active]:text-white">
                      Observable types
                    </TabsTrigger>
                    <TabsTrigger value="editor" className="max-w-[48vw] rounded-xl px-4 py-2 text-sm data-[state=active]:bg-[#131b25] data-[state=active]:text-white">
                      <span className="truncate">{editingObservableTypeId ? (observableTypeForm.name.trim() || "Edit type") : "New type"}</span>
                    </TabsTrigger>
                  </TabsList>
                  <Button
                    type="button"
                    size="icon"
                    variant="outline"
                    className="h-10 w-10 rounded-2xl border-[#2a3347] bg-[#0b1018] text-[#9ca3af] hover:bg-[#131b25] hover:text-white"
                    onClick={() => {
                      setObservableTypeEditorOpen(false);
                      setObservableTypePane("list");
                      setEditingObservableTypeId("");
                      setObservableTypeForm(createEmptyObservableTypeForm());
                    }}
                    aria-label="Close"
                    title="Close"
                    data-testid="button-close-observable-type-editor"
                  >
                    <X size={16} />
                  </Button>
                </div>
              ) : null}

              <TabsContent value="list" className="m-0 space-y-6">
                <div className="flex flex-wrap items-start justify-between gap-4">
                  <div>
                    <h2 className="text-[28px] font-semibold tracking-[-0.4px] text-white">Observable Types</h2>
                    <p className="mt-1 text-sm text-[#9ca3af]">Edit observable definitions and validation rules.</p>
                  </div>
                  <Button
                    type="button"
                    className={PRIMARY_BUTTON_CLASS}
                    onClick={() => {
                      setEditingObservableTypeId("");
                      setObservableTypeForm(createEmptyObservableTypeForm());
                      setObservableTypeEditorOpen(true);
                      setObservableTypePane("editor");
                    }}
                    data-testid="button-new-observable-type"
                  >
                    <Plus size={16} className="mr-2" /> New type
                  </Button>
                </div>

                <div className="space-y-4">
                  {observableTypes.length === 0 ? (
                    <Card className={`${CARD_CLASS} p-8 text-center text-[#8b91a3]`}>No observable types found.</Card>
                  ) : (
                    observableTypes.map((item: any) => (
                      <Card key={item.id} className={`${CARD_CLASS} p-4`} data-testid={`card-observable-type-${item.id}`}>
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0">
                            <div className="flex items-center gap-2">
                              <p className="font-semibold text-[#f3f4f6]" data-testid={`text-observable-type-name-${item.id}`}>{item.name}</p>
                              <Badge variant="outline" className="border-[#2a3347] bg-[#0b1018] text-[#dbe4f0]" data-testid={`text-observable-type-id-${item.id}`}>{item.id}</Badge>
                              <Badge variant="outline" className="border-[#2a3347] bg-[#0b1018] text-[#dbe4f0]" data-testid={`badge-observable-datatype-${item.id}`}>{item.dataType}</Badge>
                            </div>
                            {item.validatorRegex ? (
                              <code className="mt-3 block overflow-auto rounded-2xl border border-[#2a3347] bg-[#0b1018] px-3 py-2 text-xs text-[#d1d5db]" data-testid={`text-observable-regex-${item.id}`}>{item.validatorRegex}</code>
                            ) : null}
                          </div>
                          <div className="flex shrink-0 gap-2">
                            <Button
                              type="button"
                              variant="outline"
                              className={OUTLINE_BUTTON_CLASS}
                              onClick={() => {
                                setEditingObservableTypeId(String(item.id));
                                setObservableTypeForm({
                                  id: String(item.id || "").trim(),
                                  name: String(item.name || "").trim(),
                                  dataType: String(item.dataType || "string"),
                                  validatorRegex: String(item.validatorRegex || ""),
                                });
                                setObservableTypeEditorOpen(true);
                                setObservableTypePane("editor");
                              }}
                            >
                              Edit
                            </Button>
                            <Button
                              type="button"
                              variant="destructive"
                              onClick={() =>
                                deleteObservableType.mutate(String(item.id), {
                                  onSuccess: () => toast.success("Observable type deleted"),
                                  onError: (error: any) => toast.error(error?.message || "Failed to delete observable type"),
                                })
                              }
                            >
                              Delete
                            </Button>
                          </div>
                        </div>
                      </Card>
                    ))
                  )}
                </div>
              </TabsContent>

              <TabsContent value="editor" className="m-0">
                <div className="mx-auto w-full max-w-[820px] space-y-6 pb-12">
                  <div className="rounded-[24px] border border-[#2a3347] bg-[#0b1018] px-6 py-6 shadow-[0_18px_44px_rgba(0,0,0,0.28)]">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-[#7ddf8a]">Observable type editor</div>
                    <h2 className="mt-2 text-[26px] font-semibold tracking-[-0.04em] text-white">{editingObservableTypeId ? "Edit type" : "New type"}</h2>
                    <p className="mt-2 max-w-[760px] text-sm text-[#98a4b8]">Update names, data types, and validators.</p>
                  </div>

                  <Card className={`${CARD_CLASS} p-5`}>
                    <div className="space-y-4">
                      <div className="space-y-2">
                        <Label>ID</Label>
                        <Input value={observableTypeForm.id} onChange={(event) => setObservableTypeForm((prev) => ({ ...prev, id: event.target.value }))} className={INPUT_CLASS} data-testid="input-observable-type-id" disabled={Boolean(editingObservableTypeId)} />
                      </div>
                      <div className="space-y-2">
                        <Label>Name</Label>
                        <Input value={observableTypeForm.name} onChange={(event) => setObservableTypeForm((prev) => ({ ...prev, name: event.target.value }))} className={INPUT_CLASS} data-testid="input-observable-type-name" />
                      </div>
                      <div className="space-y-2">
                        <Label>Data type</Label>
                        <Select value={observableTypeForm.dataType} onValueChange={(value) => setObservableTypeForm((prev) => ({ ...prev, dataType: value }))}>
                          <SelectTrigger className={SELECT_TRIGGER_CLASS} data-testid="select-observable-data-type"><SelectValue /></SelectTrigger>
                          <SelectContent className={SELECT_CONTENT_CLASS}>
                            {OBSERVABLE_DATA_TYPE_OPTIONS.map((option) => <SelectItem key={`observable-data-type-${option}`} value={option}>{option}</SelectItem>)}
                          </SelectContent>
                        </Select>
                      </div>
                      <div className="space-y-2">
                        <Label>Validator regex</Label>
                        <Input value={observableTypeForm.validatorRegex} onChange={(event) => setObservableTypeForm((prev) => ({ ...prev, validatorRegex: event.target.value }))} className={INPUT_CLASS} data-testid="input-observable-validator-regex" />
                      </div>
                      <div className="flex flex-wrap items-center gap-3">
                        <Button type="button" className={PRIMARY_BUTTON_CLASS} onClick={handleSaveObservableType} data-testid="button-save-observable-type">
                          <Save size={16} className="mr-2" /> {editingObservableTypeId ? "Update observable type" : "Save observable type"}
                        </Button>
                        <Button
                          type="button"
                          variant="outline"
                          className={OUTLINE_BUTTON_CLASS}
                          onClick={() => {
                            setObservableTypeEditorOpen(false);
                            setObservableTypePane("list");
                            setEditingObservableTypeId("");
                            setObservableTypeForm(createEmptyObservableTypeForm());
                          }}
                        >
                          Cancel
                        </Button>
                      </div>
                    </div>
                  </Card>
                </div>
              </TabsContent>
            </Tabs>
          </TabsContent>
        </Tabs>
      </div>
    </AppLayout>
  );
}
