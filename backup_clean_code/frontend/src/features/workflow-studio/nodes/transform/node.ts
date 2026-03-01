import type { WorkflowNodeTemplate } from "../types";

export const node: WorkflowNodeTemplate = {
  id: "transform",
  title: "Transform",
  description: "Prepare payload for next steps",
  category: "Data",
  defaultConfig: {
    mapping: "{\n  \"ioc\": \"{{input.observable}}\"\n}",
  },
  fields: [
    {
      id: "mapping",
      label: "JSON mapping",
      type: "json",
      placeholder: "{ \"field\": \"{{input.value}}\" }",
    },
  ],
  summaryFallback: "configured",
};

