import type { WorkflowNodeTemplate } from "../types";

export const node: WorkflowNodeTemplate = {
  id: "aggregate",
  title: "Aggregate",
  description: "Aggregate arrays from payload/scope (count, sum, avg, unique)",
  category: "Data",
  defaultConfig: {
    sourcePath: "payload.items",
    valuePath: "",
    operation: "count",
    targetKey: "aggregate_result",
    separator: ", ",
  },
  fields: [
    {
      id: "sourcePath",
      label: "Source path",
      type: "text",
      placeholder: "payload.items",
    },
    {
      id: "valuePath",
      label: "Value path (inside each item)",
      type: "text",
      placeholder: "score",
    },
    {
      id: "operation",
      label: "Operation",
      type: "select",
      options: [
        { label: "Count", value: "count" },
        { label: "Sum", value: "sum" },
        { label: "Average", value: "avg" },
        { label: "Min", value: "min" },
        { label: "Max", value: "max" },
        { label: "Concat", value: "concat" },
        { label: "Unique", value: "unique" },
        { label: "Collect", value: "collect" },
      ],
    },
    {
      id: "separator",
      label: "Concat separator",
      type: "text",
      placeholder: ", ",
      when: { field: "operation", equals: "concat" },
    },
    {
      id: "targetKey",
      label: "Write result to key",
      type: "text",
      placeholder: "aggregate_result",
    },
  ],
  summaryField: "operation",
  summaryFallback: "count",
};
