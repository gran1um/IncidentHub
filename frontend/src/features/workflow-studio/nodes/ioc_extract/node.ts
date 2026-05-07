import type { WorkflowNodeTemplate } from "../types";

export const node: WorkflowNodeTemplate = {
  id: "ioc_extract",
  title: "IOC Extract",
  description: "Extract IPs, domains, URLs, hashes and CVEs from text payload",
  category: "Security",
  defaultConfig: {
    sourcePath: "payload.description",
    targetKey: "iocs",
    includeURL: true,
    includeEmail: true,
    includeIP: true,
    includeDomain: true,
    includeHash: true,
    includeCVE: true,
  },
  fields: [
    {
      id: "sourcePath",
      label: "Source path",
      type: "text",
      placeholder: "payload.description",
      description: "Path in workflow scope to parse for indicators",
    },
    {
      id: "targetKey",
      label: "Target key",
      type: "text",
      placeholder: "iocs",
      description: "Payload key for extracted IOC list",
    },
    { id: "includeURL", label: "Extract URLs", type: "switch" },
    { id: "includeEmail", label: "Extract emails", type: "switch" },
    { id: "includeIP", label: "Extract IPv4", type: "switch" },
    { id: "includeDomain", label: "Extract domains", type: "switch" },
    { id: "includeHash", label: "Extract hashes", type: "switch" },
    { id: "includeCVE", label: "Extract CVE IDs", type: "switch" },
  ],
  summaryField: "sourcePath",
  summaryFallback: "parses payload.description",
};
