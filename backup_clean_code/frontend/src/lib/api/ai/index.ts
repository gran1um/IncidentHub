import {
  boolFromUnknown,
  catalogQuery,
  coreFetch,
  createCatalog,
  deleteCatalog,
  ensureArray,
  listCatalog,
  mapAIAgentCatalogItem,
  mapAIAgentRunCatalogItem,
  mapAIMessage,
  mapAISession,
  numberFromUnknown,
  stringSliceFromUnknown,
  updateCatalog,
  useMutation,
  useQuery,
  useQueryClient,
} from "../core";

function normalizeAIAgentPayload(input: any): any {
  const maxCases = Math.max(1, Math.min(100, Math.round(numberFromUnknown(input?.maxCasesPerRun ?? input?.max_cases_per_run ?? input?.max_cases ?? input?.maxCases, 10))));
  const autoActionMinConfidence = Math.max(
    0,
    Math.min(100, numberFromUnknown(input?.autoActionMinConfidence ?? input?.auto_action_min_confidence ?? input?.autoActionConfidence ?? input?.auto_action_confidence, 85)),
  );
  const rawPlan = ensureArray(input?.investigationPlan ?? input?.investigation_plan ?? input?.plan ?? input?.stages);
  const normalizedPlan = rawPlan
    .map((stage: any, index: number) => ({
      id: String(stage?.id || `stage-${index + 1}`).trim() || `stage-${index + 1}`,
      name: String(stage?.name || stage?.title || "").trim(),
      description: String(stage?.description || "").trim(),
      prompt: String(stage?.prompt || stage?.instruction || stage?.instructions || "").trim(),
      case_tags: stringSliceFromUnknown(stage?.caseTags ?? stage?.case_tags ?? stage?.add_tags ?? stage?.addTags),
      enrichment_connector_ids: stringSliceFromUnknown(
        stage?.enrichmentConnectorIds ?? stage?.enrichment_connector_ids ?? stage?.connectorIds ?? stage?.connector_ids,
      ),
    }))
    .filter((stage) => stage.name || stage.prompt || stage.description || stage.case_tags.length > 0 || stage.enrichment_connector_ids.length > 0);
  return {
    name: String(input?.name || input?.title || "AI Agent").trim(),
    description: String(input?.description || "").trim(),
    prompt: String(input?.prompt || input?.instructions || "").trim(),
    model: String(input?.model || "").trim(),
    provider: String(input?.provider || input?.ai_provider || "").trim(),
    endpoint: String(input?.endpoint || input?.base_url || input?.api_base || "").trim(),
    language: String(input?.language || "").trim(),
    enabled: boolFromUnknown(input?.enabled, true),
    target_types: stringSliceFromUnknown(input?.targetTypes ?? input?.target_types ?? input?.targets),
    case_tags: stringSliceFromUnknown(input?.caseTags ?? input?.case_tags ?? input?.tags),
    alert_sources: stringSliceFromUnknown(input?.alertSources ?? input?.alert_sources ?? input?.sources),
    auto_case_tags: stringSliceFromUnknown(input?.autoCaseTags ?? input?.auto_case_tags),
    auto_close_case: boolFromUnknown(input?.autoCloseCase ?? input?.auto_close_case ?? input?.auto_close, false),
    auto_close_verdicts: stringSliceFromUnknown(input?.autoCloseVerdicts ?? input?.auto_close_verdicts),
    auto_create_case_from_alert: boolFromUnknown(input?.autoCreateCaseFromAlert ?? input?.auto_create_case_from_alert, false),
    enrichment_connector_ids: stringSliceFromUnknown(input?.enrichmentConnectorIds ?? input?.enrichment_connector_ids ?? input?.connectorIds ?? input?.connector_ids),
    notification_connector_ids: stringSliceFromUnknown(
      input?.notificationConnectorIds ?? input?.notification_connector_ids ?? input?.notifyConnectorIds ?? input?.notify_connector_ids,
    ),
    investigation_plan: normalizedPlan,
    max_cases_per_run: maxCases,
    auto_create_tasks: boolFromUnknown(input?.autoCreateTasks ?? input?.auto_create_tasks, true),
    auto_comment: boolFromUnknown(input?.autoComment ?? input?.auto_comment, true),
    task_assignee_id: String(input?.taskAssigneeId ?? input?.task_assignee_id ?? "").trim(),
    triad_enabled: boolFromUnknown(input?.triadEnabled ?? input?.triad_enabled ?? input?.threeAgentMode ?? input?.three_agent_mode, false),
    triad_critical_only: boolFromUnknown(input?.triadCriticalOnly ?? input?.triad_critical_only ?? input?.criticalOnlyTriad ?? input?.critical_only_triad, true),
    investigator_prompt: String(input?.investigatorPrompt ?? input?.investigator_prompt ?? "").trim(),
    reviewer_prompt: String(input?.reviewerPrompt ?? input?.reviewer_prompt ?? "").trim(),
    arbiter_prompt: String(input?.arbiterPrompt ?? input?.arbiter_prompt ?? "").trim(),
    require_reviewer_consensus: boolFromUnknown(
      input?.requireReviewerConsensus ?? input?.require_reviewer_consensus ?? input?.reviewerConsensusRequired ?? input?.reviewer_consensus_required,
      true,
    ),
    execution_policy: String(input?.executionPolicy ?? input?.execution_policy ?? input?.queueExecutionPolicy ?? input?.queue_execution_policy ?? "all_matching").trim() || "all_matching",
    execution_priority: Math.max(-1000, Math.min(1000, Math.round(numberFromUnknown(input?.executionPriority ?? input?.execution_priority ?? input?.queueExecutionPriority ?? input?.queue_execution_priority, 0)))),
    auto_action_min_confidence: autoActionMinConfidence,
  };
}

export function useAskAI() {
  return useMutation({
    mutationFn: async (
      input:
        | string
        | {
            question: string;
            sessionId?: string;
            session_id?: string;
            language?: "en" | "ru" | string;
            signal?: AbortSignal;
          },
    ) => {
      const payloadInput = typeof input === "string"
        ? { question: input }
        : {
            question: input?.question,
            session_id: input?.sessionId || input?.session_id || undefined,
            language: input?.language || undefined,
          };
      const signal = typeof input === "string" ? undefined : input?.signal;
      const payload = await coreFetch("/api/v1/ai/ask", {
        method: "POST",
        body: JSON.stringify(payloadInput),
        signal,
      });
      return payload;
    },
  });
}

export function useAISession(tenantId: string) {
  return useQuery({
    queryKey: ["aiSession", tenantId],
    queryFn: async () => mapAISession(await coreFetch("/api/v1/ai/session", { method: "GET" }, true, tenantId)),
    enabled: !!tenantId,
  });
}

export function useAISessions(tenantId: string, limit = 30) {
  return useQuery({
    queryKey: ["aiSessions", tenantId, limit],
    queryFn: async () => {
      const safeLimit = Number.isFinite(limit) ? Math.min(Math.max(Number(limit), 1), 200) : 30;
      const response = await coreFetch(`/api/v1/ai/sessions?limit=${encodeURIComponent(String(safeLimit))}`, { method: "GET" }, true, tenantId);
      return ensureArray(response).map(mapAISession);
    },
    enabled: !!tenantId,
  });
}

export function useCreateAISession() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: { title?: string; tenantId?: string }) => {
      const tenantId = (input?.tenantId || "").trim();
      const payload = await coreFetch(
        "/api/v1/ai/sessions",
        {
          method: "POST",
          body: JSON.stringify({ title: input?.title || "" }),
        },
        true,
        tenantId || undefined,
      );
      return mapAISession(payload);
    },
    onSuccess: (_, vars) => {
      const tenantId = (vars?.tenantId || "").trim();
      qc.invalidateQueries({ queryKey: ["aiSessions", tenantId] });
      qc.invalidateQueries({ queryKey: ["aiSession", tenantId] });
      qc.invalidateQueries({ queryKey: ["aiMessages", tenantId] });
    },
  });
}

export function useDeleteAISession() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: { sessionId: string; tenantId?: string }) => {
      const sessionId = (input?.sessionId || "").trim();
      if (!sessionId) {
        throw new Error("Session id is required");
      }
      const tenantId = (input?.tenantId || "").trim();
      return coreFetch(
        `/api/v1/ai/sessions/${encodeURIComponent(sessionId)}`,
        { method: "DELETE" },
        true,
        tenantId || undefined,
      );
    },
    onSuccess: (_, vars) => {
      const tenantId = (vars?.tenantId || "").trim();
      qc.invalidateQueries({ queryKey: ["aiSessions", tenantId] });
      qc.invalidateQueries({ queryKey: ["aiSession", tenantId] });
      qc.invalidateQueries({ queryKey: ["aiMessages", tenantId] });
    },
  });
}

export function useReorderAISessions() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: { tenantId?: string; sessionIds: string[] }) => {
      const tenantId = (input?.tenantId || "").trim();
      const sessionIds = ensureArray(input?.sessionIds).map((item) => String(item || "").trim()).filter(Boolean);
      return coreFetch(
        "/api/v1/ai/sessions/reorder",
        {
          method: "PATCH",
          body: JSON.stringify({ session_ids: sessionIds }),
        },
        true,
        tenantId || undefined,
      );
    },
    onSuccess: (_, vars) => {
      const tenantId = (vars?.tenantId || "").trim();
      qc.invalidateQueries({ queryKey: ["aiSessions", tenantId] });
      qc.invalidateQueries({ queryKey: ["aiSession", tenantId] });
    },
  });
}

export function useAIMessages(sessionId: string | undefined, limit = 80, tenantId?: string) {
  return useQuery({
    queryKey: ["aiMessages", tenantId || "default-tenant", sessionId || "default", limit],
    queryFn: async () => {
      const params = new URLSearchParams();
      if (sessionId) params.set("session_id", sessionId);
      params.set("limit", String(limit));
      const response = await coreFetch(`/api/v1/ai/messages?${params.toString()}`, { method: "GET" }, true, tenantId);
      return ensureArray(response?.messages).map(mapAIMessage);
    },
    enabled: !!tenantId && !!sessionId,
  });
}

export function useAIAgents(tenantId: string) {
  return useQuery({
    queryKey: ["aiAgents", tenantId],
    queryFn: async () => {
      const items = await listCatalog("ai_agents", { limit: 500 });
      return ensureArray(items).map(mapAIAgentCatalogItem);
    },
    enabled: !!tenantId,
  });
}

export function useCreateAIAgent() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: any) => createCatalog("ai_agents", normalizeAIAgentPayload(data)),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["aiAgents"] }),
  });
}

export function useUpdateAIAgent() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: any }) => updateCatalog("ai_agents", id, normalizeAIAgentPayload(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["aiAgents"] });
      qc.invalidateQueries({ queryKey: ["aiAgentRuns"] });
    },
  });
}

export function useDeleteAIAgent() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteCatalog("ai_agents", id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["aiAgents"] });
      qc.invalidateQueries({ queryKey: ["aiAgentRuns"] });
    },
  });
}

export function useRunAIAgent() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ agentId, payload }: { agentId: string; payload?: any }) => {
      const body = payload && typeof payload === "object" ? payload : {};
      return coreFetch(`/api/v1/ai/agents/${encodeURIComponent(agentId)}/run`, {
        method: "POST",
        body: JSON.stringify(body),
      });
    },
    onSuccess: (_result, vars) => {
      qc.invalidateQueries({ queryKey: ["aiAgentRuns", vars.agentId] });
      qc.invalidateQueries({ queryKey: ["aiAgentRunsFeed"] });
      qc.invalidateQueries({ queryKey: ["aiAgentOpsOverview"] });
      qc.invalidateQueries({ queryKey: ["aiAgents"] });
    },
  });
}

export function useAIAgentRuns(agentId: string, tenantId: string, limit = 30) {
  return useQuery({
    queryKey: ["aiAgentRuns", agentId, tenantId, limit],
    queryFn: async () => {
      const response = await coreFetch(`/api/v1/ai/agents/${encodeURIComponent(agentId)}/runs${catalogQuery({ limit })}`, {
        method: "GET",
      });
      return ensureArray(response).map(mapAIAgentRunCatalogItem);
    },
    enabled: !!tenantId && !!agentId,
  });
}

type AIAgentLiveQueryOptions = {
  enabled?: boolean;
  refetchInterval?: number | false;
};

export function useAIAgentRunsFeed(tenantId: string, limit = 200) {
  return useQuery({
    queryKey: ["aiAgentRunsFeed", tenantId, limit],
    queryFn: async () => {
      const response = await listCatalog("ai_agent_runs", { limit: Math.max(1, Math.min(500, Math.round(limit))) });
      return ensureArray(response).map(mapAIAgentRunCatalogItem);
    },
    enabled: !!tenantId,
  });
}

export function useAIAgentEntityTrace(
  entityType: string,
  entityId: string,
  tenantId: string,
  limit = 8,
  options?: AIAgentLiveQueryOptions,
) {
  return useQuery({
    queryKey: ["aiAgentEntityTrace", tenantId, entityType, entityId, limit],
    queryFn: async () => {
      const normalizedType = String(entityType || "").trim().toLowerCase();
      const normalizedId = String(entityId || "").trim();
      const normalizedLimit = Math.max(1, Math.min(20, Math.round(limit)));
      return coreFetch(
        `/api/v1/ai/agents/entities/${encodeURIComponent(normalizedType)}/${encodeURIComponent(normalizedId)}?limit=${encodeURIComponent(String(normalizedLimit))}`,
        { method: "GET" },
      );
    },
    enabled: (options?.enabled ?? true) && !!tenantId && !!entityType && !!entityId,
    refetchInterval: options?.refetchInterval === undefined ? 5000 : options.refetchInterval,
  });
}

export function useAIAgentOperationsOverview(tenantId: string, limit = 20, options?: AIAgentLiveQueryOptions) {
  return useQuery({
    queryKey: ["aiAgentOpsOverview", tenantId, limit],
    queryFn: async () => {
      const normalizedLimit = Math.max(1, Math.min(100, Math.round(limit)));
      return coreFetch(`/api/v1/ai/agents/ops/overview?limit=${encodeURIComponent(String(normalizedLimit))}`, { method: "GET" });
    },
    enabled: (options?.enabled ?? true) && !!tenantId,
    refetchInterval: options?.refetchInterval === undefined ? 5000 : options.refetchInterval,
  });
}

export function useRestartAIAgentWorkload() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ workloadId }: { workloadId: string }) =>
      coreFetch(`/api/v1/ai/agents/ops/workloads/${encodeURIComponent(workloadId)}/restart`, {
        method: "POST",
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["aiAgentOpsOverview"] });
      qc.invalidateQueries({ queryKey: ["aiAgentRunsFeed"] });
      qc.invalidateQueries({ queryKey: ["aiAgentEntityTrace"] });
    },
  });
}

export function useCloseAIAgentWorkload() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ workloadId }: { workloadId: string }) =>
      coreFetch(`/api/v1/ai/agents/ops/workloads/${encodeURIComponent(workloadId)}/close`, {
        method: "POST",
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["aiAgentOpsOverview"] });
      qc.invalidateQueries({ queryKey: ["aiAgentRunsFeed"] });
      qc.invalidateQueries({ queryKey: ["aiAgentEntityTrace"] });
    },
  });
}
