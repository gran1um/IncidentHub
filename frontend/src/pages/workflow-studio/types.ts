export type WorkflowNode = {
  id: string;
  type: string;
  label: string;
  position: { x: number; y: number };
  config: Record<string, any>;
};

export type WorkflowEdge = {
  id: string;
  source: string;
  target: string;
  label?: string;
  condition?: string;
};

export type WorkflowDefinition = {
  version: number;
  entryNodeId: string;
  nodes: WorkflowNode[];
  edges: WorkflowEdge[];
};

export type WorkflowDraft = {
  name: string;
  description: string;
  status: "draft" | "published";
  enabled: boolean;
  definition: WorkflowDefinition;
};

export type WorkflowAISuggestion = {
  name: string;
  description: string;
  definition: WorkflowDefinition;
};

export type WorkflowAIMessage = {
  id: string;
  role: "user" | "assistant";
  content: string;
  createdAt: number;
  suggestion?: WorkflowAISuggestion;
};

export type DragState = {
  nodeId: string;
  pointerOffsetX: number;
  pointerOffsetY: number;
};

export type PanState = {
  startClientX: number;
  startClientY: number;
  startOffsetX: number;
  startOffsetY: number;
};

export type EdgeDragState = {
  sourceNodeId: string;
  pointer: { x: number; y: number };
};
