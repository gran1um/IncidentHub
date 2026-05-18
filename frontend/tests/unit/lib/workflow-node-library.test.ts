import { describe, expect, it } from "vitest";

import { WORKFLOW_NODE_LIBRARY, workflowNodeTemplateById } from "@/features/workflow-studio/node-library";
import { workflowNodeVisual } from "@/features/workflow-studio/node-visuals";

describe("workflow node library", () => {
  it("loads folder-based node modules into registry", () => {
    const nodeIDs = WORKFLOW_NODE_LIBRARY.map((item) => item.id);

    expect(nodeIDs).toContain("trigger");
    expect(nodeIDs).toContain("condition");
    expect(nodeIDs).toContain("delay");
    expect(nodeIDs).toContain("transform");
    expect(nodeIDs).toContain("postgres_query");
    expect(nodeIDs).toContain("telegram_send");
    expect(nodeIDs).toContain("redis");
    expect(nodeIDs).toContain("cassandra_query");
    expect(nodeIDs).toContain("scheduler");
    expect(nodeIDs).toContain("aggregate");
    expect(nodeIDs).toContain("switch");
    expect(nodeIDs).toContain("python_code");
    expect(nodeIDs).toContain("s3_object");
  });

  it("resolves node template by id", () => {
    const template = workflowNodeTemplateById("postgres_query");
    expect(template).not.toBeNull();
    expect(template?.title).toContain("Postgres");
  });

  it("provides visual metadata for node templates", () => {
    WORKFLOW_NODE_LIBRARY.forEach((template) => {
      const visual = workflowNodeVisual(template.id, template.category);
      expect(visual.icon).toBeTruthy();
      expect(visual.badgeClass.length).toBeGreaterThan(0);
      expect(visual.handleClass.length).toBeGreaterThan(0);
    });
  });
});
