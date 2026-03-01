import type { WorkflowNodeTemplate } from "../types";

export const node: WorkflowNodeTemplate = {
  id: "switch",
  title: "Switcher",
  description: "Route flow by value to custom labels",
  category: "Control",
  defaultConfig: {
    sourcePath: "input.severity",
    cases: "[\n  { \"value\": \"critical\", \"label\": \"critical\" },\n  { \"value\": \"high\", \"label\": \"high\" }\n]",
    defaultLabel: "default",
  },
  fields: [
    {
      id: "sourcePath",
      label: "Source path",
      type: "text",
      placeholder: "input.severity",
    },
    {
      id: "cases",
      label: "Cases (JSON array)",
      type: "json",
      placeholder: "[{\"value\":\"critical\",\"label\":\"critical\"}]",
    },
    {
      id: "defaultLabel",
      label: "Default label",
      type: "text",
      placeholder: "default",
    },
  ],
  summaryField: "sourcePath",
  summaryFallback: "configured",
};
