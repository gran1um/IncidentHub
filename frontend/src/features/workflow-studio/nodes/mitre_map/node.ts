import type { WorkflowNodeTemplate } from "../types";

export const node: WorkflowNodeTemplate = {
  id: "mitre_map",
  title: "MITRE Map",
  description: "Map observable context to MITRE ATT&CK techniques",
  category: "Security",
  defaultConfig: {
    sourcePath: "payload.description",
    iocPath: "payload.iocs",
    targetKey: "mitre_matches",
  },
  fields: [
    {
      id: "sourcePath",
      label: "Text source path",
      type: "text",
      placeholder: "payload.description",
      description: "Narrative/context path used for keyword matching",
    },
    {
      id: "iocPath",
      label: "IOC source path",
      type: "text",
      placeholder: "payload.iocs",
      description: "Indicator path appended to context",
    },
    {
      id: "targetKey",
      label: "Target key",
      type: "text",
      placeholder: "mitre_matches",
    },
  ],
  summaryFallback: "ATT&CK enrichment",
};
