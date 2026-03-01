import type { WorkflowNodeTemplate } from "../types";

export const node: WorkflowNodeTemplate = {
  id: "python_code",
  title: "Python Code",
  description: "Execute custom Python code on workflow payload",
  category: "Compute",
  defaultConfig: {
    timeoutSeconds: 10,
    resultKey: "python_result",
    code: "result = {\n  \"tenant\": scope.get(\"tenant_id\"),\n  \"case\": input.get(\"case_id\")\n}\noutput[\"python_ok\"] = True",
  },
  fields: [
    {
      id: "timeoutSeconds",
      label: "Timeout (seconds)",
      type: "number",
      min: 1,
      max: 120,
      step: 1,
    },
    {
      id: "resultKey",
      label: "Result key",
      type: "text",
      placeholder: "python_result",
    },
    {
      id: "code",
      label: "Python code",
      type: "json",
      placeholder: "result = {'ok': True}\noutput['value'] = 1",
      description: "Use variables: input, payload, scope, result, output",
    },
  ],
  summaryFallback: "python script configured",
};
