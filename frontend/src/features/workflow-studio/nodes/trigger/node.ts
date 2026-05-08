import type { WorkflowNodeTemplate } from "../types";

export const node: WorkflowNodeTemplate = {
  id: "trigger",
  title: "Trigger",
  description: "Starts a workflow from an event or manual run",
  category: "Start",
  defaultConfig: { event: "manual", runOncePerEntity: true },
  fields: [
    {
      id: "event",
      label: "Event",
      type: "select",
      options: [
        { label: "Manual", value: "manual" },
        { label: "Case created", value: "case_created" },
        { label: "Case closed", value: "case_closed" },
        { label: "Case reopened", value: "case_reopened" },
        { label: "Alert created", value: "alert_created" },
      ],
    },
    {
      id: "runOncePerEntity",
      label: "Run once per entity",
      type: "switch",
    },
  ],
  summaryField: "event",
  summaryFallback: "manual",
};

