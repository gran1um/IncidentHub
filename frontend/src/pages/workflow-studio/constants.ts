import type { WorkflowCatalogKind } from "@/lib/api";

export const NODE_WIDTH = 210;
export const NODE_HEIGHT = 96;
export const EDGE_LABEL_OPTIONS = ["success", "failure", "true", "false", "next"];
export const WORKFLOW_PAGE_SIZE_OPTIONS = [10, 30, 50];
export const WORKFLOW_VIEW_MODE_SET = new Set<"list" | "editor">(["list", "editor"]);
export const WORKFLOW_CATALOG_KIND_SET = new Set<WorkflowCatalogKind>(["workflows"]);
export const WORKFLOW_TEMPLATE_DRAG_TYPE = "application/x-workflow-template-id";
export const WORKFLOW_PAGE_SHELL_CLASS = "mx-auto w-full max-w-[1240px] space-y-4";
export const WORKFLOW_PANEL_CLASS = "rounded-2xl border border-[#2a2c3c] bg-[#13141c] shadow-none";
export const WORKFLOW_SUBPANEL_CLASS = "rounded-xl border border-[#2a2c3c] bg-[#111622]";
export const WORKFLOW_MUTED_TEXT_CLASS = "text-[#9ca3af]";
export const WORKFLOW_ACTION_BUTTON_CLASS =
  "border-[#2a2c3c] bg-[#0f131d] text-[#d1d5db] hover:bg-[#171b2a] hover:text-white";
