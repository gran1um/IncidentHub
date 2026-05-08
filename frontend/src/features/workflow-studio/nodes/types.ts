export type WorkflowNodeFieldType =
  | "text"
  | "textarea"
  | "number"
  | "switch"
  | "select"
  | "connector"
  | "connector_method"
  | "json";

export type WorkflowNodeFieldOption = {
  label: string;
  value: string;
};

export type WorkflowNodeFieldVisibility = {
  field: string;
  equals: string | number | boolean;
};

export type WorkflowNodeField = {
  id: string;
  label: string;
  type: WorkflowNodeFieldType;
  placeholder?: string;
  description?: string;
  min?: number;
  max?: number;
  step?: number;
  options?: WorkflowNodeFieldOption[];
  when?: WorkflowNodeFieldVisibility;
  sensitive?: boolean;
};

export type WorkflowNodeTemplate = {
  id: string;
  title: string;
  description: string;
  category: string;
  defaultConfig: Record<string, any>;
  fields?: WorkflowNodeField[];
  summaryField?: string;
  summaryFallback?: string;
};

