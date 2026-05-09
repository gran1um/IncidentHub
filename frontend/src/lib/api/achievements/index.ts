import {
  coreFetch,
  createCatalog,
  deleteCatalog,
  listCatalog,
  stringSliceFromUnknown,
  updateCatalog,
  useAppState,
  useMutation,
  useQuery,
  useQueryClient,
} from "../core";

export function useAchievements(tenantId: string) {
  return useQuery({
    queryKey: ["achievements", tenantId],
    queryFn: () => listCatalog("achievements", { include_global: true }),
    enabled: !!tenantId,
  });
}

export function useCreateAchievement() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: any) => createCatalog("achievements", data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["achievements"] }),
  });
}

export function useUploadAchievementIcon() {
  return useMutation({
    mutationFn: async ({ file }: { file: File }) => {
      const form = new FormData();
      form.set("file", file);
      const payload = await coreFetch("/api/v1/catalog/achievements/icon/upload", {
        method: "POST",
        body: form,
      });
      return {
        icon: String(payload?.icon || payload?.storage_uri || ""),
        iconURL: String(payload?.icon_url || payload?.url || ""),
        storageURI: String(payload?.storage_uri || payload?.icon || ""),
        storageKey: String(payload?.storage_key || ""),
        contentType: String(payload?.content_type || ""),
        fileSizeBytes: Number(payload?.file_size_bytes ?? 0),
        criteria: payload?.criteria || {},
      };
    },
  });
}

export function useDeleteAchievement() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteCatalog("achievements", id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["achievements"] }),
  });
}

export function useUserAchievements(userId: string) {
  return useQuery({
    queryKey: ["userAchievements", userId],
    queryFn: async () => {
      const [grants, achievements] = await Promise.all([
        listCatalog("user_achievements", { owner_id: userId }),
        listCatalog("achievements", { include_global: true }),
      ]);
      const achievementsByID: Record<string, any> = {};
      achievements.forEach((a: any) => {
        achievementsByID[a.id] = a;
      });
      return grants.map((grant: any) => ({
        ...grant,
        userId: grant.owner_id || grant.userId,
        grantedAt: grant.grantedAt || grant.granted_at || grant.created_at,
        achievement: achievementsByID[grant.achievementId || grant.achievement_id] || null,
      }));
    },
    enabled: !!userId,
  });
}

export function useGrantAchievement() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: any) =>
      createCatalog(
        "user_achievements",
        {
          achievementId: data?.achievementId || data?.achievement_id,
          grantedAt: data?.grantedAt || new Date().toISOString(),
          userId: data?.userId,
        },
        data?.userId,
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["userAchievements"] }),
  });
}

export function useRevokeAchievement() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteCatalog("user_achievements", id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["userAchievements"] }),
  });
}

export function useUserSpecializationBadges(userId: string) {
  const currentTenantId = useAppState((state) => state.currentTenantId);
  return useQuery({
    queryKey: ["userSpecializationBadges", currentTenantId, userId],
    queryFn: async () => {
      const items = await listCatalog("user_specializations", { owner_id: userId, limit: 1 });
      const item = items[0];
      const raw = item?.badges ?? item?.specializations ?? item?.labels ?? [];
      return stringSliceFromUnknown(raw).slice(0, 16);
    },
    enabled: Boolean(userId),
  });
}

export function useUserProfileBio(userId: string) {
  const currentTenantId = useAppState((state) => state.currentTenantId);
  return useQuery({
    queryKey: ["userProfileBio", currentTenantId, userId],
    queryFn: async () => {
      const items = await listCatalog("user_profile_bio", { owner_id: userId, limit: 1 });
      const item = items[0];
      return String(item?.bio ?? item?.about ?? item?.description ?? "").trim();
    },
    enabled: Boolean(userId),
  });
}

export function useSaveUserSpecializationBadges(userId: string) {
  const qc = useQueryClient();
  const currentTenantId = useAppState((state) => state.currentTenantId);
  return useMutation({
    mutationFn: async (badges: string[]) => {
      const normalized = Array.from(
        new Set(
          badges
            .map((item) => String(item || "").trim())
            .filter(Boolean),
        ),
      ).slice(0, 16);
      const existing = await listCatalog("user_specializations", { owner_id: userId, limit: 1 });
      if (existing.length > 0) {
        return updateCatalog("user_specializations", existing[0].id, { badges: normalized });
      }
      return createCatalog("user_specializations", { badges: normalized }, userId);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["userSpecializationBadges", currentTenantId, userId] });
    },
  });
}

export function useSaveUserProfileBio(userId: string) {
  const qc = useQueryClient();
  const currentTenantId = useAppState((state) => state.currentTenantId);
  return useMutation({
    mutationFn: async (bio: string) => {
      const normalized = String(bio || "").trim().slice(0, 1200);
      const existing = await listCatalog("user_profile_bio", { owner_id: userId, limit: 1 });
      if (existing.length > 0) {
        return updateCatalog("user_profile_bio", existing[0].id, { bio: normalized });
      }
      return createCatalog("user_profile_bio", { bio: normalized }, userId);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["userProfileBio", currentTenantId, userId] });
    },
  });
}
