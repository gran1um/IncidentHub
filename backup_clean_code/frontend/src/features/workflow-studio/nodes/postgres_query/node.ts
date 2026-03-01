import type { WorkflowNodeTemplate } from "../types";

export const node: WorkflowNodeTemplate = {
  id: "postgres_query",
  title: "Postgres Query",
  description: "Run read-only SQL query in IncidentHub Postgres",
  category: "Data",
  defaultConfig: {
    sql: "SELECT id, title, severity, status FROM cases WHERE tenant_id = '{{tenant_id}}' ORDER BY created_at DESC",
    limit: 100,
    params: "[]",
  },
  fields: [
    {
      id: "sql",
      label: "SQL (read-only)",
      type: "json",
      placeholder: "SELECT ...",
      description: "Single SELECT/WITH/EXPLAIN statement",
    },
    {
      id: "params",
      label: "Query params (JSON array)",
      type: "json",
      placeholder: "[\"{{input.case_id}}\"]",
    },
    {
      id: "limit",
      label: "Max rows",
      type: "number",
      min: 1,
      max: 1000,
      step: 1,
    },
  ],
  summaryFallback: "SQL configured",
};

