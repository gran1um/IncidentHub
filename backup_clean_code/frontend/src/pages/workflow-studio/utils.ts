import type { WorkflowNodeField } from "@/features/workflow-studio/node-library";

import type { WorkflowAISuggestion, WorkflowDefinition, WorkflowDraft, WorkflowEdge, WorkflowNode } from "./types";
import { WORKFLOW_VIEW_MODE_SET } from "./constants";
import { splitLocationPathAndSearch } from "@/lib/url-state";

export function resolveViewModeFromLocation(location: string): "list" | "editor" {
  const { params } = splitLocationPathAndSearch(location);
  const modeParam = String(params.get("mode") || "").trim().toLowerCase();
  if (WORKFLOW_VIEW_MODE_SET.has(modeParam as "list" | "editor")) {
    return modeParam as "list" | "editor";
  }
  return "list";
}

export function generateID(prefix: string): string {
  return `${prefix}_${Math.random().toString(36).slice(2, 10)}`;
}

export function defaultDefinition(): WorkflowDefinition {
  return {
    version: 1,
    entryNodeId: "",
    nodes: [],
    edges: [],
  };
}

export function defaultDraft(): WorkflowDraft {
  return {
    name: "",
    description: "",
    status: "draft",
    enabled: false,
    definition: defaultDefinition(),
  };
}

export function formatRunDate(value?: string): string {
  if (!value) return "n/a";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
}

export function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return "<1s";
  if (ms < 1000) return `${Math.round(ms)}ms`;
  const sec = ms / 1000;
  if (sec < 60) return `${sec.toFixed(1)}s`;
  const min = Math.floor(sec / 60);
  const rem = Math.round(sec % 60);
  return `${min}m ${rem}s`;
}

export function normalizeDefinition(input: any): WorkflowDefinition {
  const definition = input && typeof input === "object" ? input : {};
  const nodes: WorkflowNode[] = Array.isArray(definition.nodes)
    ? definition.nodes
        .map((node: any, index: number): WorkflowNode => ({
          id: String(node?.id || generateID(`node_${index}`)),
          type: String(node?.type || "transform"),
          label: String(node?.label || node?.name || "Step"),
          position: {
            x: Number(node?.position?.x || 120 + index * 32),
            y: Number(node?.position?.y || 120 + index * 32),
          },
          config: node?.config && typeof node.config === "object" ? node.config : {},
        }))
    : [];
  const edges: WorkflowEdge[] = Array.isArray(definition.edges)
    ? definition.edges
        .map((edge: any) => ({
          id: String(edge?.id || `${edge?.source || ""}-${edge?.target || ""}`),
          source: String(edge?.source || ""),
          target: String(edge?.target || ""),
          label: edge?.label ? String(edge.label) : "",
          condition: edge?.condition ? String(edge.condition) : "",
        }))
        .filter((edge: WorkflowEdge) => edge.source && edge.target)
    : [];
  const firstNodeId = nodes[0]?.id || "";
  const entryNodeId = String(definition.entryNodeId || definition.entry_node_id || firstNodeId || "");
  return {
    version: Number(definition.version || 1),
    entryNodeId: nodes.find((node) => node.id === entryNodeId)?.id || firstNodeId,
    nodes,
    edges,
  };
}

export function extractJSONBlock(text: string): string {
  const payload = String(text || "");
  const fenced = payload.match(/```json\s*([\s\S]*?)```/i);
  if (fenced?.[1]) {
    return fenced[1].trim();
  }
  const anyFenced = payload.match(/```\s*([\s\S]*?)```/);
  if (anyFenced?.[1]) {
    return anyFenced[1].trim();
  }
  const firstBrace = payload.indexOf("{");
  const lastBrace = payload.lastIndexOf("}");
  if (firstBrace >= 0 && lastBrace > firstBrace) {
    return payload.slice(firstBrace, lastBrace + 1).trim();
  }
  return payload.trim();
}

export function parseWorkflowSuggestion(answer: string, allowedNodeTypes: Set<string>): WorkflowAISuggestion | null {
  const candidate = extractJSONBlock(answer);
  if (!candidate) {
    return null;
  }
  let parsed: any;
  try {
    parsed = JSON.parse(candidate);
  } catch {
    return null;
  }
  if (!parsed || typeof parsed !== "object") {
    return null;
  }
  const definitionInput = parsed?.definition ?? parsed?.workflow ?? parsed;
  const definition = normalizeDefinition(definitionInput);
  if (!definition.nodes.length) {
    return null;
  }
  const unsupportedNode = definition.nodes.find((node) => !allowedNodeTypes.has(node.type));
  if (unsupportedNode) {
    return null;
  }
  const name = String(parsed?.name || parsed?.title || "AI workflow draft").trim();
  const description = String(parsed?.description || parsed?.summary || "").trim();
  return {
    name,
    description,
    definition,
  };
}

export function buildWorkflowAssistantPrompt(userPrompt: string, availableNodes: string[]): string {
  const normalizedPrompt = String(userPrompt || "").trim();
  const nodesLine = availableNodes.join(", ");
  return [
    "You are Workflow Studio assistant for SOC automation.",
    "Return only valid JSON. No markdown, no explanations.",
    "JSON schema:",
    '{"name":"...","description":"...","definition":{"version":1,"entryNodeId":"node_1","nodes":[{"id":"node_1","type":"trigger","label":"Trigger","position":{"x":120,"y":100},"config":{}}],"edges":[{"id":"edge_1","source":"node_1","target":"node_2","label":"next"}]}}',
    `Allowed node types: ${nodesLine}.`,
    "Prefer security nodes for SOC use-cases when relevant: ioc_extract, watchlist_match, mitre_map, risk_score, containment_decision.",
    "Ensure entryNodeId points to an existing node id.",
    "Keep node coordinates positive integers.",
    `Task: ${normalizedPrompt}`,
  ].join("\n");
}

export function edgePath(sx: number, sy: number, tx: number, ty: number): string {
  const cx1 = sx + 84;
  const cx2 = tx - 84;
  return `M ${sx} ${sy} C ${cx1} ${sy}, ${cx2} ${ty}, ${tx} ${ty}`;
}

export function connectorNameForID(connectors: any[], connectorId?: string): string {
  if (!connectorId) return "Connector";
  const item = connectors.find((connector: any) => connector.id === connectorId);
  return item?.name || connectorId;
}

export function shouldRenderNodeField(field: WorkflowNodeField, config: Record<string, any>): boolean {
  if (!field.when) {
    return true;
  }
  const currentValue = config?.[field.when.field];
  return currentValue === field.when.equals;
}
