import { format } from "date-fns";
import { normalizeObservableConnectorType } from "@/lib/connectors";

const verdictColors: Record<string, string> = {
  Malicious: "border border-[rgba(239,68,68,0.28)] bg-[rgba(239,68,68,0.16)] text-[#fca5a5]",
  Suspicious: "border border-[rgba(245,158,11,0.3)] bg-[rgba(245,158,11,0.16)] text-[#fbbf24]",
  Benign: "border border-[rgba(34,197,94,0.28)] bg-[rgba(34,197,94,0.16)] text-[#86efac]",
  Unknown: "border border-[rgba(107,114,128,0.28)] bg-[rgba(107,114,128,0.16)] text-[#d1d5db]",
};

const aiAnalysisStatusColors: Record<string, string> = {
  completed: "border border-[rgba(16,185,129,0.28)] bg-[rgba(16,185,129,0.15)] text-[#6ee7b7]",
  failed: "border border-[rgba(244,63,94,0.28)] bg-[rgba(244,63,94,0.16)] text-[#fda4af]",
};

export const OBSERVABLE_TYPES = ["IP", "Domain", "Hash MD5", "Hash SHA256", "Email", "URL", "User Agent"];
export const VERDICTS = ["Malicious", "Suspicious", "Benign", "Unknown"];
export const TLP_VALUES = ["TLP:RED", "TLP:AMBER", "TLP:GREEN", "TLP:CLEAR"];
export const PAP_VALUES = ["PAP:RED", "PAP:AMBER", "PAP:GREEN", "PAP:CLEAR"];
export const TASK_STATUSES = ["Pending", "In Progress", "Done", "Cancelled"];
export const TASK_STATUS_KEYS: Record<string, string> = {
  Pending: "caseDetail.task.status.pending",
  "In Progress": "caseDetail.task.status.inProgress",
  Done: "caseDetail.task.status.done",
  Cancelled: "caseDetail.task.status.cancelled",
};
export const TASK_CHAIN_MODES = ["manual", "semi_automated", "automated"] as const;
export const TASK_CHAIN_MODE_KEYS: Record<(typeof TASK_CHAIN_MODES)[number], string> = {
  manual: "caseDetail.task.chain.modeManual",
  semi_automated: "caseDetail.task.chain.modeSemiAutomated",
  automated: "caseDetail.task.chain.modeAutomated",
};
export const CASE_SLA_MINUTES_BY_SEVERITY: Record<string, number> = {
  critical: 120,
  high: 240,
  medium: 480,
  low: 1440,
};
export const QUICK_REMINDER_PRESETS = [
  { id: "15m", minutes: 15 },
  { id: "1h", minutes: 60 },
  { id: "24h", minutes: 24 * 60 },
] as const;
export const DEFAULT_CASE_CATEGORIES = ["True Positive", "False Positive", "Needs Review"];
export const CLOSURE_APPROVAL_EVENT_TYPE = "closure_approval";
export const CLOSURE_REQUIRED_APPROVALS_FIELD = "closure_required_approvals";
export const CLOSURE_APPROVER_IDS_FIELD = "closure_approver_ids";
export const RELATED_CASES_LINK_BY_OPTIONS = ["observables", "incident_type", "source", "severity", "priority"] as const;
export type RelatedCasesLinkBy = (typeof RELATED_CASES_LINK_BY_OPTIONS)[number];
export const VERDICT_KEYS: Record<string, string> = {
  Malicious: "caseDetail.observable.verdict.malicious",
  Suspicious: "caseDetail.observable.verdict.suspicious",
  Benign: "caseDetail.observable.verdict.benign",
  Unknown: "caseDetail.observable.verdict.unknown",
};
export const TIMELINE_EVENT_KEYS: Record<string, string> = {
  alert_import: "caseDetail.timeline.event.alertImported",
  observable_added: "caseDetail.timeline.event.observableAdded",
  task_created: "caseDetail.timeline.event.taskCreated",
  status_changed: "caseDetail.timeline.event.statusChanged",
  comment_added: "caseDetail.timeline.event.commentAdded",
  note: "caseDetail.timeline.event.note",
  reminder: "caseDetail.timeline.event.reminder",
  closure_approval: "caseDetail.timeline.event.closureApproval",
  case_escalated: "caseDetail.timeline.event.caseEscalated",
  case_escalation_received: "caseDetail.timeline.event.caseEscalationReceived",
  workflow_run: "caseDetail.timeline.event.workflowRun",
  "Alert imported": "caseDetail.timeline.event.alertImported",
  "Observable added": "caseDetail.timeline.event.observableAdded",
  "Task created": "caseDetail.timeline.event.taskCreated",
  "Status changed": "caseDetail.timeline.event.statusChanged",
  "Comment added": "caseDetail.timeline.event.commentAdded",
  Reminder: "caseDetail.timeline.event.reminder",
  "Closure approved": "caseDetail.timeline.event.closureApproval",
};

export const MITRE_TACTICS = [
  { id: "TA0001", name: "Initial Access" },
  { id: "TA0002", name: "Execution" },
  { id: "TA0003", name: "Persistence" },
  { id: "TA0004", name: "Privilege Escalation" },
  { id: "TA0005", name: "Defense Evasion" },
  { id: "TA0006", name: "Credential Access" },
  { id: "TA0007", name: "Discovery" },
  { id: "TA0008", name: "Lateral Movement" },
  { id: "TA0009", name: "Collection" },
  { id: "TA0010", name: "Exfiltration" },
  { id: "TA0011", name: "Command and Control" },
  { id: "TA0040", name: "Impact" },
  { id: "TA0042", name: "Resource Development" },
  { id: "TA0043", name: "Reconnaissance" },
];

export const MITRE_TECHNIQUES: Record<string, { id: string; name: string }[]> = {
  TA0001: [
    { id: "T1566", name: "Phishing" },
    { id: "T1190", name: "Exploit Public-Facing Application" },
    { id: "T1133", name: "External Remote Services" },
    { id: "T1078", name: "Valid Accounts" },
    { id: "T1195", name: "Supply Chain Compromise" },
  ],
  TA0002: [
    { id: "T1059", name: "Command and Scripting Interpreter" },
    { id: "T1204", name: "User Execution" },
    { id: "T1053", name: "Scheduled Task/Job" },
    { id: "T1203", name: "Exploitation for Client Execution" },
  ],
  TA0003: [
    { id: "T1547", name: "Boot or Logon Autostart Execution" },
    { id: "T1543", name: "Create or Modify System Process" },
    { id: "T1546", name: "Event Triggered Execution" },
  ],
  TA0004: [
    { id: "T1548", name: "Abuse Elevation Control Mechanism" },
    { id: "T1134", name: "Access Token Manipulation" },
    { id: "T1068", name: "Exploitation for Privilege Escalation" },
  ],
  TA0005: [
    { id: "T1070", name: "Indicator Removal" },
    { id: "T1036", name: "Masquerading" },
    { id: "T1027", name: "Obfuscated Files or Information" },
    { id: "T1562", name: "Impair Defenses" },
  ],
  TA0006: [
    { id: "T1110", name: "Brute Force" },
    { id: "T1003", name: "OS Credential Dumping" },
    { id: "T1555", name: "Credentials from Password Stores" },
  ],
  TA0007: [
    { id: "T1087", name: "Account Discovery" },
    { id: "T1083", name: "File and Directory Discovery" },
    { id: "T1057", name: "Process Discovery" },
  ],
  TA0008: [
    { id: "T1021", name: "Remote Services" },
    { id: "T1570", name: "Lateral Tool Transfer" },
    { id: "T1080", name: "Taint Shared Content" },
  ],
  TA0009: [
    { id: "T1005", name: "Data from Local System" },
    { id: "T1114", name: "Email Collection" },
    { id: "T1074", name: "Data Staged" },
  ],
  TA0010: [
    { id: "T1041", name: "Exfiltration Over C2 Channel" },
    { id: "T1048", name: "Exfiltration Over Alternative Protocol" },
    { id: "T1567", name: "Exfiltration Over Web Service" },
  ],
  TA0011: [
    { id: "T1071", name: "Application Layer Protocol" },
    { id: "T1105", name: "Ingress Tool Transfer" },
    { id: "T1572", name: "Protocol Tunneling" },
  ],
  TA0040: [
    { id: "T1486", name: "Data Encrypted for Impact" },
    { id: "T1489", name: "Service Stop" },
    { id: "T1490", name: "Inhibit System Recovery" },
  ],
  TA0042: [
    { id: "T1583", name: "Acquire Infrastructure" },
    { id: "T1588", name: "Obtain Capabilities" },
  ],
  TA0043: [
    { id: "T1595", name: "Active Scanning" },
    { id: "T1592", name: "Gather Victim Host Information" },
    { id: "T1589", name: "Gather Victim Identity Information" },
  ],
};

export const INLINE_AUTOSAVE_FIELDS = new Set<InlineCaseFieldKey>([
  "title",
  "incidentType",
  "category",
  "relatedProduct",
  "source",
  "severity",
  "priority",
  "stage",
  "tlp",
  "assignee",
  "owner",
  "impact",
  "confidence",
  "detectedAt",
  "tags",
  "description",
  "resolutionSummary",
]);


export function getAIVerdictBadgeClass(verdict?: string): string {
  const normalized = (verdict || "").toLowerCase();
  if (normalized === "malicious") return verdictColors.Malicious;
  if (normalized === "suspicious") return verdictColors.Suspicious;
  if (normalized === "benign") return verdictColors.Benign;
  return verdictColors.Unknown;
}

export function getAIStatusBadgeClass(status?: string): string {
  const normalized = (status || "").toLowerCase();
  return aiAnalysisStatusColors[normalized] || "bg-gray-100 text-gray-700 border-gray-200";
}

export function getConnectorHubStatusBadgeClass(status?: string): string {
  const normalized = (status || "").toLowerCase();
  if (normalized === "completed") {
    return "border border-[rgba(34,197,94,0.28)] bg-[rgba(34,197,94,0.16)] text-[#86efac]";
  }
  if (normalized === "dry_run" || normalized === "dispatching" || normalized === "provider_accepted") {
    return "border border-[rgba(59,130,246,0.28)] bg-[rgba(59,130,246,0.16)] text-[#93c5fd]";
  }
  if (normalized === "accepted" || normalized === "queued" || normalized === "retry_scheduled") {
    return "border border-[rgba(245,158,11,0.3)] bg-[rgba(245,158,11,0.16)] text-[#fcd34d]";
  }
  if (normalized === "cancelled") {
    return "border border-[rgba(148,163,184,0.28)] bg-[rgba(51,65,85,0.4)] text-[#cbd5e1]";
  }
  return "border border-[rgba(244,63,94,0.3)] bg-[rgba(244,63,94,0.16)] text-[#fda4af]";
}

export function formatAnalysisDate(value?: string): string {
  if (!value) {
    return "—";
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return "—";
  }
  return format(parsed, "dd.MM.yyyy HH:mm");
}

export function formatTimelineDate(value?: string): string {
  if (!value) return "—";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "—";
  return format(parsed, "dd.MM HH:mm");
}

export function formatDateTimeWithSeconds(value?: string): string {
  if (!value) return "—";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "—";
  return format(parsed, "dd.MM.yyyy HH:mm:ss");
}

export function escapeHTML(value: string): string {
  return String(value || "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

export function sanitizeStyleAttribute(styleValue: string): string {
  const safeDeclarations: string[] = [];
  const declarations = String(styleValue || "")
    .split(";")
    .map((item) => item.trim())
    .filter(Boolean);
  for (const declaration of declarations) {
    const separatorIndex = declaration.indexOf(":");
    if (separatorIndex <= 0) continue;
    const property = declaration.slice(0, separatorIndex).trim().toLowerCase();
    const value = declaration.slice(separatorIndex + 1).trim();
    if (!property || !value) continue;
    if (property === "color" || property === "background-color") {
      if (/^(#[0-9a-f]{3,8}|rgb(a)?\([^()]+\)|hsl(a)?\([^()]+\)|[a-z]+)$/i.test(value)) {
        safeDeclarations.push(`${property}:${value}`);
      }
      continue;
    }
    if (property === "font-weight") {
      if (/^(normal|bold|[1-9]00)$/i.test(value)) {
        safeDeclarations.push(`${property}:${value}`);
      }
      continue;
    }
    if (property === "font-style") {
      if (/^(normal|italic|oblique)$/i.test(value)) {
        safeDeclarations.push(`${property}:${value}`);
      }
      continue;
    }
    if (property === "text-decoration") {
      if (/^(none|underline|line-through|overline)$/i.test(value)) {
        safeDeclarations.push(`${property}:${value}`);
      }
    }
  }
  return safeDeclarations.join("; ");
}

export function renderMarkdownInline(input: string): string {
  let text = String(input || "");
  text = text.replace(/!\[([^\]]*)\]\(([^)\s]+)(?:\s+"([^"]*)")?\)/g, (_match, alt, src, title) => {
    const altSafe = escapeHTML(String(alt || ""));
    const srcSafe = escapeHTML(String(src || ""));
    const titleSafe = title ? ` title="${escapeHTML(String(title))}"` : "";
    return `<img src="${srcSafe}" alt="${altSafe}"${titleSafe} />`;
  });
  text = text.replace(/\[([^\]]+)\]\(([^)\s]+)(?:\s+"([^"]*)")?\)/g, (_match, label, href, title) => {
    const labelSafe = String(label || "");
    const hrefSafe = escapeHTML(String(href || ""));
    const titleSafe = title ? ` title="${escapeHTML(String(title))}"` : "";
    return `<a href="${hrefSafe}" target="_blank" rel="noopener noreferrer"${titleSafe}>${labelSafe}</a>`;
  });
  text = text.replace(/`([^`]+)`/g, (_match, code) => `<code>${escapeHTML(String(code || ""))}</code>`);
  text = text.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
  text = text.replace(/__([^_]+)__/g, "<strong>$1</strong>");
  text = text.replace(/\*([^*]+)\*/g, "<em>$1</em>");
  text = text.replace(/_([^_]+)_/g, "<em>$1</em>");
  text = text.replace(/~~([^~]+)~~/g, "<del>$1</del>");
  return text;
}

export function renderMarkdownToHTML(input: string): string {
  const normalized = String(input || "").replace(/\r\n?/g, "\n");
  if (!normalized.trim()) {
    return "";
  }
  const lines = normalized.split("\n");
  const output: string[] = [];
  const paragraphLines: string[] = [];
  const listItems: string[] = [];
  let listType: "ul" | "ol" | null = null;
  let codeFenceOpen = false;
  let codeFenceLanguage = "";
  let codeFenceLines: string[] = [];

  const flushParagraph = () => {
    if (!paragraphLines.length) return;
    output.push(`<p>${renderMarkdownInline(paragraphLines.join("<br/>"))}</p>`);
    paragraphLines.length = 0;
  };
  const flushList = () => {
    if (!listType || !listItems.length) return;
    output.push(`<${listType}>${listItems.join("")}</${listType}>`);
    listType = null;
    listItems.length = 0;
  };
  const flushCodeFence = () => {
    if (!codeFenceOpen) return;
    const languageClass = codeFenceLanguage ? ` class="language-${escapeHTML(codeFenceLanguage)}"` : "";
    output.push(`<pre><code${languageClass}>${escapeHTML(codeFenceLines.join("\n"))}</code></pre>`);
    codeFenceOpen = false;
    codeFenceLanguage = "";
    codeFenceLines = [];
  };

  for (const line of lines) {
    const trimmed = line.trim();
    if (trimmed.startsWith("```")) {
      flushParagraph();
      flushList();
      if (codeFenceOpen) {
        flushCodeFence();
      } else {
        codeFenceOpen = true;
        codeFenceLanguage = trimmed.slice(3).trim();
      }
      continue;
    }
    if (codeFenceOpen) {
      codeFenceLines.push(line);
      continue;
    }
    if (!trimmed) {
      flushParagraph();
      flushList();
      continue;
    }

    const headingMatch = line.match(/^\s{0,3}(#{1,6})\s+(.+)$/);
    if (headingMatch) {
      flushParagraph();
      flushList();
      const level = headingMatch[1].length;
      output.push(`<h${level}>${renderMarkdownInline(headingMatch[2].trim())}</h${level}>`);
      continue;
    }

    if (/^\s{0,3}([-_*])(?:\s*\1){2,}\s*$/.test(line)) {
      flushParagraph();
      flushList();
      output.push("<hr />");
      continue;
    }

    const unorderedMatch = line.match(/^\s{0,3}[-*+]\s+(.+)$/);
    if (unorderedMatch) {
      flushParagraph();
      if (listType !== "ul") {
        flushList();
        listType = "ul";
      }
      listItems.push(`<li>${renderMarkdownInline(unorderedMatch[1].trim())}</li>`);
      continue;
    }

    const orderedMatch = line.match(/^\s{0,3}\d+\.\s+(.+)$/);
    if (orderedMatch) {
      flushParagraph();
      if (listType !== "ol") {
        flushList();
        listType = "ol";
      }
      listItems.push(`<li>${renderMarkdownInline(orderedMatch[1].trim())}</li>`);
      continue;
    }

    const blockQuoteMatch = line.match(/^\s{0,3}>\s?(.*)$/);
    if (blockQuoteMatch) {
      flushParagraph();
      flushList();
      output.push(`<blockquote><p>${renderMarkdownInline(blockQuoteMatch[1].trim())}</p></blockquote>`);
      continue;
    }

    flushList();
    paragraphLines.push(line.trim());
  }

  flushParagraph();
  flushList();
  flushCodeFence();
  return output.join("\n");
}

export function sanitizeDescriptionHTML(value: string): string {
  const input = String(value || "");
  if (!input.trim()) {
    return "";
  }
  if (typeof window === "undefined" || typeof DOMParser === "undefined") {
    return input;
  }
  const parser = new DOMParser();
  const documentNode = parser.parseFromString(input, "text/html");
  documentNode.querySelectorAll("script, iframe, object, embed, link, meta, style").forEach((node) => node.remove());
  documentNode.querySelectorAll("*").forEach((element) => {
    Array.from(element.attributes).forEach((attribute) => {
      const name = attribute.name.toLowerCase();
      const val = attribute.value;
      if (name.startsWith("on")) {
        element.removeAttribute(attribute.name);
        return;
      }
      if (name === "style") {
        const safeStyle = sanitizeStyleAttribute(val);
        if (safeStyle) {
          element.setAttribute("style", safeStyle);
        } else {
          element.removeAttribute(attribute.name);
        }
        return;
      }
      if ((name === "href" || name === "src") && /^\s*javascript:/i.test(val)) {
        element.removeAttribute(attribute.name);
      }
    });
    if (element.tagName.toLowerCase() === "a" && element.getAttribute("href")) {
      element.setAttribute("target", "_blank");
      element.setAttribute("rel", "noopener noreferrer");
    }
  });
  return documentNode.body.innerHTML;
}

export function toTimestamp(value?: string): number {
  if (!value) return 0;
  const parsed = new Date(value).getTime();
  if (!Number.isFinite(parsed)) return 0;
  return parsed;
}

export function formatDurationCompact(durationMs: number): string {
  const totalMinutes = Math.max(0, Math.round(durationMs / 60000));
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

export function humanizeTimelineEventType(eventType: string): string {
  const normalized = String(eventType || "").trim();
  if (!normalized) return "Event";
  return normalized
    .replace(/_/g, " ")
    .replace(/\s+/g, " ")
    .trim()
    .replace(/^\w/, (letter) => letter.toUpperCase());
}

export function summarizeWorkflowRunPayload(value: any): string {
  if (!value || typeof value !== "object") {
    return "";
  }
  try {
    const raw = JSON.stringify(value);
    if (raw.length <= 180) {
      return raw;
    }
    return `${raw.slice(0, 180)}...`;
  } catch {
    return "";
  }
}

export function isImageAttachment(contentType?: string, fileName?: string): boolean {
  const normalizedType = String(contentType || "").toLowerCase();
  if (normalizedType.startsWith("image/")) {
    return true;
  }
  const normalizedName = String(fileName || "").toLowerCase();
  return [".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp"].some((extension) =>
    normalizedName.endsWith(extension),
  );
}

export function uniqueStrings(values: string[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  values.forEach((value) => {
    const normalized = String(value || "").trim();
    if (!normalized || seen.has(normalized)) return;
    seen.add(normalized);
    out.push(normalized);
  });
  return out;
}

export function observableTypeKey(value: string): string {
  return normalizeObservableConnectorType(value);
}

export function extractHostFromObservable(type: string, value: string): string {
  const normalizedType = observableTypeKey(type);
  const normalizedValue = String(value || "").trim();
  if (!normalizedValue) return "";
  if (normalizedType.includes("domain") || normalizedType.includes("host")) {
    return normalizedValue;
  }
  if (normalizedType.includes("ip")) {
    return normalizedValue;
  }
  if (normalizedType.includes("url")) {
    try {
      const parsed = new URL(normalizedValue);
      return parsed.hostname || normalizedValue;
    } catch {
      return normalizedValue;
    }
  }
  return "";
}

export function defaultPlaybookContext(caseData: any, observables: any[]): Record<string, any> {
  const safeCaseData = caseData && typeof caseData === "object" ? caseData : {};
  const safeObservables = Array.isArray(observables) ? observables : [];
  const indicators = uniqueStrings(
    safeObservables
      .map((observable: any) => String(observable?.value || "").trim())
      .filter(Boolean),
  );
  const hosts = uniqueStrings(
    safeObservables
      .map((observable: any) => extractHostFromObservable(observable?.type, observable?.value))
      .filter(Boolean),
  );
  const users = uniqueStrings(
    safeObservables
      .filter((observable: any) => {
        const type = observableTypeKey(observable?.type);
        return type.includes("email") || type.includes("user") || type.includes("account");
      })
      .map((observable: any) => String(observable?.value || "").trim())
      .filter(Boolean),
  );
  const customFields = safeCaseData?.customFields && typeof safeCaseData.customFields === "object"
    ? safeCaseData.customFields
    : {};
  const node = String(
    customFields?.node ||
      customFields?.asset ||
      customFields?.endpoint ||
      customFields?.host ||
      hosts[0] ||
      "",
  ).trim();
  const host = String(hosts[0] || node || "").trim();

  const observablesByType = safeObservables.reduce<Record<string, string[]>>((acc, observable: any) => {
    const type = observableTypeKey(observable?.type);
    const value = String(observable?.value || "").trim();
    if (!type || !value) return acc;
    const current = acc[type] || [];
    if (!current.includes(value)) {
      acc[type] = [...current, value];
    }
    return acc;
  }, {});

  return {
    case_id: String(safeCaseData?.id || "").trim(),
    case_number: String(safeCaseData?.caseNumber || "").trim(),
    tenant_id: String(safeCaseData?.tenantId || "").trim(),
    node,
    host,
    indicators,
    users,
    observables_by_type: observablesByType,
    case: {
      id: String(safeCaseData?.id || "").trim(),
      number: String(safeCaseData?.caseNumber || "").trim(),
      title: String(safeCaseData?.title || "").trim(),
      severity: String(safeCaseData?.sev || "").trim(),
      status: String(safeCaseData?.status || "").trim(),
      stage: String(safeCaseData?.stage || "").trim(),
      source: String(safeCaseData?.source || "").trim(),
      incident_type: String(safeCaseData?.incidentType || "").trim(),
      tags: Array.isArray(safeCaseData?.tags) ? safeCaseData.tags : [],
    },
    observables: safeObservables.map((observable: any) => ({
      type: String(observable?.type || "").trim(),
      value: String(observable?.value || "").trim(),
      verdict: String(observable?.verdict || "").trim(),
      tags: Array.isArray(observable?.tags) ? observable.tags : [],
    })),
  };
}

export function tokensFromText(value: string): string[] {
  return String(value || "")
    .toLowerCase()
    .split(/[^a-zа-яё0-9]+/i)
    .map((token) => token.trim())
    .filter((token) => token.length >= 3);
}

export type CaseEditDraft = {
  id: string;
  caseNumber: string;
  title: string;
  description: string;
  source: string;
  incidentType: string;
  category: string;
  relatedProduct: string;
  statusCode: string;
  severity: string;
  priority: string;
  impact: string;
  confidence: string;
  tlp: string;
  pap: string;
  assignee: string;
  detectedAt: string;
  occurredAt: string;
  closedAt: string;
  resolutionSummary: string;
  stage: string;
  verdict: string;
  recommendations: string;
};

export type CustomFieldDraftRow = {
  id: string;
  key: string;
  value: string;
};

export type VisualizationSourceFilter = "all" | "network" | "authorization" | "other";
export type VisualizationTypeFilter = "all" | string;
export type VisualizationAttachmentFilter = "all" | "images" | "files";

export type InlineCaseFieldKey =
  | "title"
  | "incidentType"
  | "category"
  | "relatedProduct"
  | "source"
  | "statusCode"
  | "severity"
  | "priority"
  | "stage"
  | "tlp"
  | "assignee"
  | "owner"
  | "impact"
  | "confidence"
  | "detectedAt"
  | "tags"
  | "description"
  | "resolutionSummary";

export type ClosureApprovalConfig = {
  requiredApprovals: number;
  requiredApproverIDs: string[];
};

export function toDateTimeLocal(value?: string): string {
  if (!value) return "";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "";
  const pad = (num: number) => String(num).padStart(2, "0");
  return `${parsed.getFullYear()}-${pad(parsed.getMonth() + 1)}-${pad(parsed.getDate())}T${pad(parsed.getHours())}:${pad(parsed.getMinutes())}`;
}

export function fromDateTimeLocal(value: string): string | undefined {
  const trimmed = value.trim();
  if (!trimmed) return undefined;
  const parsed = new Date(trimmed);
  if (Number.isNaN(parsed.getTime())) return undefined;
  return parsed.toISOString();
}

export function normalizeTrafficLight(value: string): string | undefined {
  const trimmed = value.trim();
  if (!trimmed) return undefined;
  const normalized = trimmed
    .replace(/^TLP:/i, "")
    .replace(/^PAP:/i, "")
    .trim()
    .toLowerCase();
  if (!normalized) return undefined;
  return normalized;
}

export function normalizeObservableType(value: string): string {
  return String(value || "").trim().toLowerCase();
}

export function isNetworkObservableType(value: string): boolean {
  const normalized = normalizeObservableType(value);
  return (
    normalized.includes("ip") ||
    normalized.includes("domain") ||
    normalized.includes("url") ||
    normalized.includes("host") ||
    normalized.includes("dns") ||
    normalized.includes("network") ||
    normalized.includes("hash")
  );
}

export function isAuthorizationObservableType(value: string): boolean {
  const normalized = normalizeObservableType(value);
  return (
    normalized.includes("user") ||
    normalized.includes("account") ||
    normalized.includes("email") ||
    normalized.includes("auth") ||
    normalized.includes("login") ||
    normalized.includes("session") ||
    normalized.includes("token")
  );
}

export function classifyObservableSource(type: string, value: string, tags: string[]): Exclude<VisualizationSourceFilter, "all"> {
  if (isNetworkObservableType(type)) {
    return "network";
  }
  if (isAuthorizationObservableType(type)) {
    return "authorization";
  }
  const loweredTags = Array.isArray(tags) ? tags.map((item) => String(item || "").toLowerCase()) : [];
  if (loweredTags.some((tag) => tag.includes("network") || tag.includes("dns") || tag.includes("net"))) {
    return "network";
  }
  if (loweredTags.some((tag) => tag.includes("auth") || tag.includes("account") || tag.includes("login"))) {
    return "authorization";
  }
  const normalizedValue = String(value || "").toLowerCase();
  if (normalizedValue.includes("@")) {
    return "authorization";
  }
  return "other";
}

export function parseObservableTagsInput(value: string): string[] {
  return value
    .split(/[\n,;]+/g)
    .map((item) => item.trim().toLowerCase())
    .filter(Boolean);
}

export function normalizeObservableTags(input: string[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const tag of input) {
    const normalized = String(tag || "").trim().toLowerCase();
    if (!normalized || seen.has(normalized)) continue;
    seen.add(normalized);
    out.push(normalized);
  }
  return out;
}

export function isRelatedCasesLinkBy(value: string): value is RelatedCasesLinkBy {
  return (RELATED_CASES_LINK_BY_OPTIONS as readonly string[]).includes(value);
}

export function parseRelatedCasesLinkBy(value: string | null | undefined): RelatedCasesLinkBy {
  const normalized = String(value || "").trim().toLowerCase();
  if (isRelatedCasesLinkBy(normalized)) {
    return normalized;
  }
  return "observables";
}

export function parseClosureApprovalConfig(customFields: any): ClosureApprovalConfig {
  if (!customFields || typeof customFields !== "object" || Array.isArray(customFields)) {
    return { requiredApprovals: 0, requiredApproverIDs: [] };
  }
  const fields = customFields as Record<string, any>;
  const parseCount = (raw: any): number => {
    if (typeof raw === "number" && Number.isFinite(raw)) {
      return Math.max(0, Math.floor(raw));
    }
    const asString = String(raw ?? "").trim();
    if (!asString) return 0;
    const parsed = Number.parseInt(asString, 10);
    return Number.isFinite(parsed) ? Math.max(0, parsed) : 0;
  };
  const parseApproverIDs = (raw: any): string[] => {
    const unique = new Set<string>();
    const push = (value: any) => {
      const normalized = String(value || "").trim();
      if (normalized) {
        unique.add(normalized);
      }
    };
    if (Array.isArray(raw)) {
      raw.forEach((entry) => push(entry));
    } else if (typeof raw === "string") {
      const trimmed = raw.trim();
      if (!trimmed) {
        return [];
      }
      if (trimmed.startsWith("[") && trimmed.endsWith("]")) {
        try {
          const parsed = JSON.parse(trimmed);
          if (Array.isArray(parsed)) {
            parsed.forEach((entry) => push(entry));
            return Array.from(unique);
          }
        } catch {
          // fallback to comma-separated parsing
        }
      }
      trimmed.split(",").forEach((entry) => push(entry));
    }
    return Array.from(unique);
  };

  const requiredApprovals = parseCount(fields[CLOSURE_REQUIRED_APPROVALS_FIELD]);
  const requiredApproverIDs = parseApproverIDs(fields[CLOSURE_APPROVER_IDS_FIELD]);
  return {
    requiredApprovals: Math.max(requiredApprovals, requiredApproverIDs.length),
    requiredApproverIDs,
  };
}

export function createCaseEditDraft(item: any): CaseEditDraft {
  return {
    id: item?.id || "",
    caseNumber: item?.caseNumber || "",
    title: item?.title || "",
    description: item?.description || "",
    source: item?.source || "",
    incidentType: item?.incidentType || "",
    category: item?.category || "",
    relatedProduct: item?.relatedProduct || "",
    statusCode: (item?.statusCode || "open").toLowerCase(),
    severity: item?.sev || "Medium",
    priority: item?.priority || "medium",
    impact: item?.impact || "",
    confidence: String(item?.confidence ?? 0),
    tlp: item?.tlp || "TLP:AMBER",
    pap: item?.pap ? `PAP:${String(item.pap).replace(/^PAP:/i, "").replace(/^TLP:/i, "").toUpperCase()}` : "PAP:AMBER",
    assignee: item?.assignee || item?.owner || "",
    detectedAt: toDateTimeLocal(item?.detectedAt),
    occurredAt: toDateTimeLocal(item?.occurredAt),
    closedAt: toDateTimeLocal(item?.closedAt),
    resolutionSummary: item?.resolutionSummary || "",
    stage: item?.stage || "",
    verdict: item?.verdict || "Unknown",
    recommendations: Array.isArray(item?.recommendations) ? item.recommendations.join("\n") : "",
  };
}

export function createCustomFieldDraftID(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return `cf-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

export function toCustomFieldDraftRows(value: any): CustomFieldDraftRow[] {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return [];
  }
  return Object.entries(value)
    .map(([key, rawValue]) => {
      const normalizedKey = String(key || "").trim();
      if (!normalizedKey) return null;
      if (rawValue === null || rawValue === undefined) {
        return { id: createCustomFieldDraftID(), key: normalizedKey, value: "" };
      }
      if (typeof rawValue === "string") {
        return { id: createCustomFieldDraftID(), key: normalizedKey, value: rawValue };
      }
      try {
        return { id: createCustomFieldDraftID(), key: normalizedKey, value: JSON.stringify(rawValue) };
      } catch {
        return { id: createCustomFieldDraftID(), key: normalizedKey, value: String(rawValue) };
      }
    })
    .filter((item): item is CustomFieldDraftRow => Boolean(item));
}

export function csvEscape(value: any): string {
  const text = String(value ?? "");
  if (!/[",\n]/.test(text)) {
    return text;
  }
  return `"${text.replace(/"/g, "\"\"")}"`;
}

export function buildCaseExportCSV(payload: any): string {
  const lines = [
    ["section", "key", "value"],
    ["case", "id", payload?.case?.id || ""],
    ["case", "case_number", payload?.case?.caseNumber || ""],
    ["case", "title", payload?.case?.title || ""],
    ["case", "status", payload?.case?.status || ""],
    ["case", "severity", payload?.case?.sev || ""],
    ["case", "owner", payload?.case?.owner || ""],
    ["case", "assignee", payload?.case?.assignee || ""],
    ["case", "created_at", payload?.case?.time || ""],
  ];

  (payload?.timeline || []).forEach((event: any, index: number) => {
    lines.push(["timeline", `${index + 1}`, `${event?.createdAt || ""} ${event?.title || ""} ${event?.description || ""}`]);
  });
  (payload?.tasks || []).forEach((task: any, index: number) => {
    lines.push(["tasks", `${index + 1}`, `${task?.status || ""} ${task?.title || ""}`]);
  });
  (payload?.observables || []).forEach((observable: any, index: number) => {
    lines.push(["observables", `${index + 1}`, `${observable?.type || ""} ${observable?.value || ""}`]);
  });
  (payload?.attachments || []).forEach((attachment: any, index: number) => {
    lines.push(["attachments", `${index + 1}`, `${attachment?.fileName || ""} (${attachment?.contentType || ""})`]);
  });
  (payload?.playbookRuns || []).forEach((run: any, index: number) => {
    lines.push(["playbooks", `${index + 1}`, `${run?.status || ""} ${run?.workflowName || run?.workflowId || ""}`]);
  });

  return lines.map((row) => row.map(csvEscape).join(",")).join("\n");
}

export function buildCaseExportMarkdown(payload: any): string {
  const lines: string[] = [];
  lines.push(`# Case ${payload?.case?.caseNumber || payload?.case?.id || ""}`);
  lines.push("");
  lines.push(`- Title: ${payload?.case?.title || "—"}`);
  lines.push(`- Status: ${payload?.case?.status || "—"}`);
  lines.push(`- Severity: ${payload?.case?.sev || "—"}`);
  lines.push(`- Owner: ${payload?.case?.owner || "—"}`);
  lines.push(`- Assignee: ${payload?.case?.assignee || "—"}`);
  lines.push("");

  lines.push("## Timeline");
  const timeline = Array.isArray(payload?.timeline) ? payload.timeline : [];
  if (timeline.length === 0) {
    lines.push("- No timeline events.");
  } else {
    timeline.forEach((event: any) => {
      lines.push(`- ${event?.createdAt || ""}: ${event?.title || "Event"}${event?.description ? ` — ${event.description}` : ""}`);
    });
  }
  lines.push("");

  lines.push("## Tasks");
  const tasks = Array.isArray(payload?.tasks) ? payload.tasks : [];
  if (tasks.length === 0) {
    lines.push("- No tasks.");
  } else {
    tasks.forEach((task: any) => {
      lines.push(`- [${task?.status || "Pending"}] ${task?.title || "Untitled task"}`);
    });
  }
  lines.push("");

  lines.push("## Observables");
  const observables = Array.isArray(payload?.observables) ? payload.observables : [];
  if (observables.length === 0) {
    lines.push("- No observables.");
  } else {
    observables.forEach((observable: any) => {
      lines.push(`- ${observable?.type || "observable"}: ${observable?.value || "—"} (${observable?.verdict || "unknown"})`);
    });
  }
  lines.push("");

  lines.push("## Playbook Runs");
  const playbookRuns = Array.isArray(payload?.playbookRuns) ? payload.playbookRuns : [];
  if (playbookRuns.length === 0) {
    lines.push("- No playbook runs linked to this case.");
  } else {
    playbookRuns.forEach((run: any) => {
      lines.push(`- ${run?.startedAt || run?.createdAt || "—"}: ${run?.workflowName || run?.workflowId || "Workflow"} (${run?.status || "unknown"})`);
    });
  }

  return lines.join("\n");
}
