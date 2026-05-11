import { coreFetch, ensureArray, useMutation, useQuery, useQueryClient } from "../core";

export function useWorkflowCredentials(
  tenantId: string,
  filters?: { nodeType?: string },
) {
  return useQuery({
    queryKey: ["workflowCredentials", tenantId, filters?.nodeType || ""],
    queryFn: async () => {
      const params = new URLSearchParams();
      if (filters?.nodeType) params.set("node_type", String(filters.nodeType));
      const query = params.toString();
      return ensureArray(await coreFetch(`/api/v1/workflows/credentials${query ? `?${query}` : ""}`, { method: "GET" }));
    },
    enabled: !!tenantId,
  });
}

export function useCreateWorkflowCredential() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: { name: string; description?: string; node_type: string; fields: Record<string, string> }) =>
      coreFetch("/api/v1/workflows/credentials", { method: "POST", body: JSON.stringify(payload) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["workflowCredentials"] }),
  });
}

export function useUpdateWorkflowCredential() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: { id: string; name: string; description?: string; node_type: string; fields: Record<string, string> }) =>
      coreFetch(`/api/v1/workflows/credentials/${encodeURIComponent(payload.id)}`, {
        method: "PUT",
        body: JSON.stringify({
          name: payload.name,
          description: payload.description,
          node_type: payload.node_type,
          fields: payload.fields,
        }),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["workflowCredentials"] }),
  });
}

export function useDeleteWorkflowCredential() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (credentialId: string) => coreFetch(`/api/v1/workflows/credentials/${encodeURIComponent(credentialId)}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["workflowCredentials"] }),
  });
}
