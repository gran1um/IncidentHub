import type { WorkflowNodeTemplate } from "../types";

export const node: WorkflowNodeTemplate = {
  id: "condition",
  title: "Condition",
  description: "Branch by payload or case fields",
  category: "Control",
  defaultConfig: { expression: "input.severity == 'critical'" },
  fields: [
    {
      id: "expression",
      label: "Condition expression",
      type: "textarea",
      placeholder: "input.severity == 'critical'",
      description: "Supports operators: ==, !=, >, >=, <, <=, contains",
    },
  ],
  summaryField: "expression",
  summaryFallback: "configured",
};

