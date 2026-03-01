import type { WorkflowNodeTemplate } from "../types";

export const node: WorkflowNodeTemplate = {
  id: "containment_decision",
  title: "Containment Decision",
  description: "Choose initial IR containment action from risk + ATT&CK context",
  category: "Security",
  defaultConfig: {
    riskLevelPath: "payload.risk_level",
    watchlistHitPath: "payload.watchlist_hit",
    mitrePath: "payload.mitre_matches",
  },
  fields: [
    {
      id: "riskLevelPath",
      label: "Risk level path",
      type: "text",
      placeholder: "payload.risk_level",
    },
    {
      id: "watchlistHitPath",
      label: "Watchlist hit path",
      type: "text",
      placeholder: "payload.watchlist_hit",
    },
    {
      id: "mitrePath",
      label: "MITRE matches path",
      type: "text",
      placeholder: "payload.mitre_matches",
    },
  ],
  summaryFallback: "monitor / isolate / reset",
};
