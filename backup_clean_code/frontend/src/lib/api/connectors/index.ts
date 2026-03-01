import {
  boolFromUnknown,
  coreFetch,
  createCatalog,
  deleteCatalog,
  ensureArray,
  listCatalog,
  mapCatalogItem,
  stringSliceFromUnknown,
  updateCatalog,
  useMutation,
  useQuery,
  useQueryClient,
} from "../core";

function mapConnectorCatalogItem(item: any) {
  const base = mapCatalogItem(item);
  const data = item?.data && typeof item.data === "object" ? item.data : {};
  return {
    ...base,
    ...data,
    kind: String(item?.kind || base?.kind || "outbound_connectors").trim() || "outbound_connectors",
    name: String(data?.name || item?.name || item?.title || "").trim(),
    description: String(data?.description || item?.description || "").trim(),
    direction: String(data?.direction || item?.direction || "outbound").trim().toLowerCase() || "outbound",
    enabled: boolFromUnknown(data?.enabled ?? item?.enabled, true),
    type: String(data?.type || item?.type || "HTTP").trim(),
    channel: String(data?.channel || item?.channel || "").trim().toLowerCase(),
    category: String(data?.category || item?.category || "standard").trim().toLowerCase() || "standard",
    communicationMode: String(data?.communication_mode || data?.communicationMode || item?.communication_mode || item?.communicationMode || "").trim().toLowerCase(),
    capabilities: stringSliceFromUnknown(data?.capabilities || item?.capabilities),
    config: data?.config && typeof data.config === "object" ? data.config : (item?.config && typeof item.config === "object" ? item.config : {}),
  };
}

export function useOutboundConnectors(tenantId: string) {
  return useQuery({
    queryKey: ["connectors", "outbound", tenantId],
    queryFn: async () => {
      const items = await listCatalog("outbound_connectors");
      return ensureArray(items).map(mapConnectorCatalogItem);
    },
    enabled: !!tenantId,
  });
}

export function useCreateConnector() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: any) =>
      createCatalog("outbound_connectors", {
        name: data?.name,
        description: data?.description,
        type: data?.type,
        channel: data?.channel,
        direction: "outbound",
        category: data?.category,
        capabilities: data?.capabilities || [],
        communication_mode: data?.communicationMode || data?.communication_mode || "",
        enabled: data?.enabled ?? true,
        config: data?.config || {},
        tenantId: data?.tenantId,
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["connectors"] }),
  });
}

export function useUpdateConnector() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data, kind }: { id: string; data: any; kind?: string }) => {
      const connectorKind = kind || data?.kind || "outbound_connectors";
      return updateCatalog(connectorKind, id, { ...data, direction: "outbound" });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["connectors"] });
    },
  });
}

export function useDeleteConnector() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: string | { id: string; kind?: string }) => {
      if (typeof input === "string") {
        // Default to outbound connectors when kind is unknown.
        return deleteCatalog("outbound_connectors", input);
      }
      const connectorKind = input.kind || "outbound_connectors";
      return deleteCatalog(connectorKind, input.id);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["connectors"] }),
  });
}

export function useConnectorMethods(connectorId: string) {
  return useQuery({
    queryKey: ["connectorMethods", connectorId],
    queryFn: () => listCatalog("connector_methods", { ref_id: connectorId }),
    enabled: !!connectorId,
  });
}

export function useTenantConnectorMethods(tenantId: string) {
  return useQuery({
    queryKey: ["connectorMethods", "tenant", tenantId],
    queryFn: () => listCatalog("connector_methods"),
    enabled: !!tenantId,
  });
}

export function useCreateConnectorMethod() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: any) => createCatalog("connector_methods", data, undefined, data?.connectorId || data?.connector_id),
    onSuccess: (_result, vars) => {
      qc.invalidateQueries({ queryKey: ["connectorMethods"] });
      const connectorId = String(vars?.connectorId || vars?.connector_id || "").trim();
      if (connectorId) {
        qc.invalidateQueries({ queryKey: ["connectorMethods", connectorId] });
      }
    },
  });
}

export function useUpdateConnectorMethod() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: any }) => updateCatalog("connector_methods", id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["connectorMethods"] }),
  });
}

export function useDeleteConnectorMethod() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteCatalog("connector_methods", id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["connectorMethods"] }),
  });
}

type ConnectorHubExecutionFilters = {
  caseId?: string;
  alertId?: string;
  action?: string;
  status?: string;
  executionMode?: string;
  limit?: number;
  refetchInterval?: number | false;
};

function invalidateConnectorHubExecutionQueries(qc: ReturnType<typeof useQueryClient>, executionId?: string) {
  qc.invalidateQueries({ queryKey: ["connectorHubExecutions"] });
  if (executionId) {
    qc.invalidateQueries({ queryKey: ["connectorHubExecution", executionId] });
    qc.invalidateQueries({ queryKey: ["connectorHubExecutionEvents", executionId] });
  }
}

export function useConnectorHubExecutions(
  connectorId?: string,
  filters?: ConnectorHubExecutionFilters,
) {
  return useQuery({
    queryKey: [
      "connectorHubExecutions",
      connectorId || "",
      filters?.caseId || "",
      filters?.alertId || "",
      filters?.action || "",
      filters?.status || "",
      filters?.executionMode || "",
      filters?.limit || 50,
    ],
    queryFn: async () => {
      const params = new URLSearchParams();
      if (connectorId) params.set("connector_id", connectorId);
      if (filters?.caseId) params.set("case_id", String(filters.caseId));
      if (filters?.alertId) params.set("alert_id", String(filters.alertId));
      if (filters?.action) params.set("action", String(filters.action));
      if (filters?.status) params.set("status", String(filters.status));
      if (filters?.executionMode) params.set("execution_mode", String(filters.executionMode));
      if (filters?.limit) params.set("limit", String(filters.limit));
      const query = params.toString();
      return ensureArray(await coreFetch(`/api/v1/connectors/hub/executions${query ? `?${query}` : ""}`, { method: "GET" }));
    },
    enabled: Boolean(connectorId || filters?.caseId || filters?.alertId),
    refetchInterval: filters?.refetchInterval === undefined ? 5000 : filters.refetchInterval,
  });
}

export function useConnectorHubExecutionDetail(executionId: string, options?: { enabled?: boolean; refetchInterval?: number | false }) {
  return useQuery({
    queryKey: ["connectorHubExecution", executionId],
    queryFn: () => coreFetch(`/api/v1/connectors/hub/executions/${encodeURIComponent(executionId)}`, { method: "GET" }),
    enabled: Boolean(executionId) && options?.enabled !== false,
    refetchInterval: options?.refetchInterval,
  });
}

export function useConnectorHubExecutionEvents(executionId: string, limit = 100, options?: { enabled?: boolean; refetchInterval?: number | false }) {
  return useQuery({
    queryKey: ["connectorHubExecutionEvents", executionId, limit],
    queryFn: () => coreFetch(`/api/v1/connectors/hub/executions/${encodeURIComponent(executionId)}/events?limit=${encodeURIComponent(String(limit))}`, { method: "GET" }),
    enabled: Boolean(executionId) && options?.enabled !== false,
    refetchInterval: options?.refetchInterval,
  });
}

export function useRetryConnectorHubExecution() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (executionId: string) => coreFetch(`/api/v1/connectors/hub/executions/${encodeURIComponent(executionId)}/retry`, { method: "POST" }),
    onSuccess: (_result, executionId) => invalidateConnectorHubExecutionQueries(qc, executionId),
  });
}

export function useCancelConnectorHubExecution() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (executionId: string) => coreFetch(`/api/v1/connectors/hub/executions/${encodeURIComponent(executionId)}/cancel`, { method: "POST" }),
    onSuccess: (_result, executionId) => invalidateConnectorHubExecutionQueries(qc, executionId),
  });
}

export function useRestartConnectorHubExecution() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (executionId: string) => coreFetch(`/api/v1/connectors/hub/executions/${encodeURIComponent(executionId)}/restart`, { method: "POST" }),
    onSuccess: (_result, executionId) => invalidateConnectorHubExecutionQueries(qc, executionId),
  });
}

export function useExecuteConnectorHub() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: any) => coreFetch("/api/v1/connectors/hub/execute", { method: "POST", body: JSON.stringify(payload) }),
    onSuccess: (_result, payload) => {
      invalidateConnectorHubExecutionQueries(qc);
      const connectorID = String(payload?.connector_id || "");
      if (connectorID) {
        qc.invalidateQueries({ queryKey: ["connectorHubExecutions", connectorID] });
      }
    },
  });
}

export function useAutomations(tenantId: string) {
  return useQuery({
    queryKey: ["automations", tenantId],
    queryFn: () => listCatalog("automations"),
    enabled: !!tenantId,
  });
}

export function useCreateAutomation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: any) => createCatalog("automations", data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["automations"] }),
  });
}

export function useUpdateAutomation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: any }) => updateCatalog("automations", id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["automations"] }),
  });
}

export function useDeleteAutomation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteCatalog("automations", id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["automations"] }),
  });
}
