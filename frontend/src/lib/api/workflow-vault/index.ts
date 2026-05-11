import { coreFetch, ensureArray, useMutation, useQuery, useQueryClient } from "../core";

export function useWorkflowVaultSecrets(
  tenantId: string,
  filters?: { field?: string; nodeType?: string },
) {
  return useQuery({
    queryKey: ["workflowVaultSecrets", tenantId, filters?.field || "", filters?.nodeType || ""],
    queryFn: async () => {
      const params = new URLSearchParams();
      if (filters?.field) params.set("field", String(filters.field));
      if (filters?.nodeType) params.set("node_type", String(filters.nodeType));
      const query = params.toString();
      return ensureArray(await coreFetch(`/api/v1/workflows/vault/secrets${query ? `?${query}` : ""}`, { method: "GET" }));
    },
    enabled: !!tenantId,
  });
}

export function useCreateWorkflowVaultSecret() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: { name: string; value: string; field?: string; node_type?: string }) =>
      coreFetch("/api/v1/workflows/vault/secrets", { method: "POST", body: JSON.stringify(payload) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["workflowVaultSecrets"] }),
  });
}

export function useDeleteWorkflowVaultSecret() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (secretId: string) => coreFetch(`/api/v1/workflows/vault/secrets/${encodeURIComponent(secretId)}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["workflowVaultSecrets"] }),
  });
}
