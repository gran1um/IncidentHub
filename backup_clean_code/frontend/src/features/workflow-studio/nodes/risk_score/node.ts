import type { WorkflowNodeTemplate } from "../types";

export const node: WorkflowNodeTemplate = {
  id: "risk_score",
  title: "Risk Score",
  description: "Calculate normalized security risk score (0-100)",
  category: "Security",
  defaultConfig: {
    severityPath: "payload.severity",
    confidencePath: "payload.confidence",
    iocCountPath: "payload.ioc_count",
    watchlistHitPath: "payload.watchlist_hit",
  },
  fields: [
    {
      id: "severityPath",
      label: "Severity path",
      type: "text",
      placeholder: "payload.severity",
    },
    {
      id: "confidencePath",
      label: "Confidence path",
      type: "text",
      placeholder: "payload.confidence",
    },
    {
      id: "iocCountPath",
      label: "IOC count path",
      type: "text",
      placeholder: "payload.ioc_count",
    },
    {
      id: "watchlistHitPath",
      label: "Watchlist hit path",
      type: "text",
      placeholder: "payload.watchlist_hit",
    },
  ],
  summaryFallback: "risk level calculation",
};
