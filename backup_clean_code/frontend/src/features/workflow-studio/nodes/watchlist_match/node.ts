import type { WorkflowNodeTemplate } from "../types";

export const node: WorkflowNodeTemplate = {
  id: "watchlist_match",
  title: "Watchlist Match",
  description: "Match extracted IOCs against a static watchlist",
  category: "Security",
  defaultConfig: {
    sourcePath: "payload.iocs",
    valueKey: "value",
    matchMode: "exact",
    caseInsensitive: true,
    watchlist: "[\"evil.example.com\", \"203.0.113.10\", \"CVE-2024-3400\"]",
    targetKey: "watchlist_matches",
  },
  fields: [
    {
      id: "sourcePath",
      label: "Source path",
      type: "text",
      placeholder: "payload.iocs",
      description: "Array path with IOC objects/values",
    },
    {
      id: "valueKey",
      label: "Value key",
      type: "text",
      placeholder: "value",
      description: "Field to read from IOC objects",
    },
    {
      id: "matchMode",
      label: "Match mode",
      type: "select",
      options: [
        { label: "Exact", value: "exact" },
        { label: "Contains", value: "contains" },
        { label: "Regex", value: "regex" },
      ],
    },
    {
      id: "caseInsensitive",
      label: "Case insensitive",
      type: "switch",
    },
    {
      id: "watchlist",
      label: "Watchlist (JSON array or CSV)",
      type: "json",
      placeholder: "[\"evil.example.com\", \"203.0.113.10\"]",
    },
    {
      id: "targetKey",
      label: "Target key",
      type: "text",
      placeholder: "watchlist_matches",
    },
  ],
  summaryField: "matchMode",
  summaryFallback: "exact match",
};
