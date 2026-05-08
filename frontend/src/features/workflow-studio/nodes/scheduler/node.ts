import type { WorkflowNodeTemplate } from "../types";

export const node: WorkflowNodeTemplate = {
  id: "scheduler",
  title: "Scheduler",
  description: "Wait by delay or until specific date-time",
  category: "Control",
  defaultConfig: {
    mode: "delay",
    delaySeconds: 60,
    runAt: "",
    maxWaitSeconds: 300,
  },
  fields: [
    {
      id: "mode",
      label: "Mode",
      type: "select",
      options: [
        { label: "Delay", value: "delay" },
        { label: "Run at date-time", value: "at" },
      ],
    },
    {
      id: "delaySeconds",
      label: "Delay (seconds)",
      type: "number",
      min: 0,
      max: 86400,
      step: 1,
      when: { field: "mode", equals: "delay" },
    },
    {
      id: "runAt",
      label: "Run at (RFC3339)",
      type: "text",
      placeholder: "2026-02-19T10:00:00Z",
      when: { field: "mode", equals: "at" },
    },
    {
      id: "maxWaitSeconds",
      label: "Max wait cap (seconds)",
      type: "number",
      min: 1,
      max: 3600,
      step: 1,
    },
  ],
  summaryField: "mode",
  summaryFallback: "delay",
};
