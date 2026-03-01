import type { WorkflowNodeTemplate } from "../types";

export const node: WorkflowNodeTemplate = {
  id: "delay",
  title: "Delay",
  description: "Wait before next step",
  category: "Control",
  defaultConfig: { seconds: 60 },
  fields: [
    {
      id: "seconds",
      label: "Delay (seconds)",
      type: "number",
      min: 0,
      max: 3600,
      step: 1,
    },
  ],
  summaryField: "seconds",
  summaryFallback: "0",
};

