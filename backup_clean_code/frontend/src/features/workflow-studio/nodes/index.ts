import type { WorkflowNodeTemplate } from "./types";

type NodeModule = {
  node: WorkflowNodeTemplate;
};

const modules = import.meta.glob<NodeModule>("./*/node.ts", { eager: true });

const loadedNodes = Object.values(modules)
  .map((module) => module.node)
  .filter((node): node is WorkflowNodeTemplate => Boolean(node?.id));

export const WORKFLOW_NODE_LIBRARY: WorkflowNodeTemplate[] = loadedNodes
  .slice()
  .sort((left, right) => left.title.localeCompare(right.title));

export function groupWorkflowNodeLibraryByCategory(
  templates: WorkflowNodeTemplate[] = WORKFLOW_NODE_LIBRARY,
): Record<string, WorkflowNodeTemplate[]> {
  const grouped: Record<string, WorkflowNodeTemplate[]> = {};
  templates.forEach((item) => {
    grouped[item.category] = grouped[item.category] || [];
    grouped[item.category].push(item);
  });
  return grouped;
}

export function workflowNodeTemplateById(
  id: string,
  templates: WorkflowNodeTemplate[] = WORKFLOW_NODE_LIBRARY,
): WorkflowNodeTemplate | null {
  return templates.find((item) => item.id === id) || null;
}

export function workflowNodeSummary(
  template: WorkflowNodeTemplate | null,
  config: Record<string, any>,
  connectorLabelById?: (connectorId: string) => string,
): string {
  if (!template) {
    return "Configured";
  }
  if (template.summaryField) {
    const value = String(config?.[template.summaryField] || "").trim();
    if (value) {
      if (template.summaryField === "connectorId" && connectorLabelById) {
        return connectorLabelById(value);
      }
      return value;
    }
  }
  if (template.summaryFallback) {
    return template.summaryFallback;
  }
  return "Configured";
}

export type { WorkflowNodeTemplate, WorkflowNodeField, WorkflowNodeFieldType } from "./types";

