import {
  coreFetch,
  createCatalog,
  deleteCatalog,
  ensureArray,
  listCatalog,
  mapAdminNotificationSetting,
  mapMyNotificationSettings,
  mapNotificationBot,
  mapServiceAlertRule,
  updateCatalog,
  useMutation,
  useQuery,
  useQueryClient,
} from "./core";

export function useAdminNotificationBots(tenantId: string) {
  return useQuery({
    queryKey: ["adminNotificationBots", tenantId],
    queryFn: async () => {
      const items = await coreFetch("/api/v1/admin/notification-bots", { method: "GET" }, true, tenantId);
      return ensureArray(items).map(mapNotificationBot);
    },
    enabled: !!tenantId,
  });
}

export function useCreateAdminNotificationBot() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: any) => {
      const tenantId = String(data?.tenantId || data?.tenant_id || "").trim();
      const payload = await coreFetch(
        "/api/v1/admin/notification-bots",
        {
          method: "POST",
          body: JSON.stringify({
            name: String(data?.name || "").trim(),
            bot_token: String(data?.botToken || data?.bot_token || "").trim(),
            enabled: Boolean(data?.enabled ?? true),
          }),
        },
        true,
        tenantId || undefined,
      );
      return mapNotificationBot(payload);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["adminNotificationBots"] });
      qc.invalidateQueries({ queryKey: ["notificationBots"] });
    },
  });
}

export function useUpdateAdminNotificationBot() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, data }: { id: string; data: any }) => {
      const tenantId = String(data?.tenantId || data?.tenant_id || "").trim();
      const payload = await coreFetch(
        `/api/v1/admin/notification-bots/${encodeURIComponent(id)}`,
        {
          method: "PATCH",
          body: JSON.stringify({
            name: data?.name,
            bot_token: data?.botToken ?? data?.bot_token,
            enabled: data?.enabled,
          }),
        },
        true,
        tenantId || undefined,
      );
      return mapNotificationBot(payload);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["adminNotificationBots"] });
      qc.invalidateQueries({ queryKey: ["notificationBots"] });
      qc.invalidateQueries({ queryKey: ["myNotificationSettings"] });
    },
  });
}

export function useDeleteAdminNotificationBot() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: string | { id: string; tenantId?: string; tenant_id?: string }) => {
      const id = typeof input === "string" ? input : input.id;
      const tenantId = typeof input === "string" ? "" : String(input.tenantId || input.tenant_id || "").trim();
      return coreFetch(`/api/v1/admin/notification-bots/${encodeURIComponent(id)}`, { method: "DELETE" }, true, tenantId || undefined);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["adminNotificationBots"] });
      qc.invalidateQueries({ queryKey: ["notificationBots"] });
      qc.invalidateQueries({ queryKey: ["myNotificationSettings"] });
    },
  });
}

export function useNotificationBots(tenantId: string) {
  return useQuery({
    queryKey: ["notificationBots", tenantId],
    queryFn: async () => {
      const items = await coreFetch("/api/v1/notification-bots", { method: "GET" }, true, tenantId);
      return ensureArray(items).map(mapNotificationBot);
    },
    enabled: !!tenantId,
  });
}

export function useMyNotificationSettings(tenantId: string) {
  return useQuery({
    queryKey: ["myNotificationSettings", tenantId],
    queryFn: async () => mapMyNotificationSettings(await coreFetch("/api/v1/me/notification-settings", { method: "GET" }, true, tenantId)),
    enabled: !!tenantId,
  });
}

export function useSaveMyNotificationSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: any) => {
      const tenantId = String(data?.tenantId || data?.tenant_id || "").trim();
      const payload = await coreFetch(
        "/api/v1/me/notification-settings",
        {
          method: "PUT",
          body: JSON.stringify({
            delivery_enabled: Boolean(data?.deliveryEnabled ?? data?.delivery_enabled),
            delivery_channel: String(data?.deliveryChannel || data?.delivery_channel || "in_app").trim().toLowerCase(),
            telegram_bot_id: data?.telegramBotId ?? data?.telegram_bot_id ?? "",
            telegram_chat_id: data?.telegramChatId ?? data?.telegram_chat_id ?? "",
            telegram_username: data?.telegramUsername ?? data?.telegram_username ?? "",
            notification_email: data?.notificationEmail ?? data?.notification_email ?? "",
            time_recipient: data?.timeRecipient ?? data?.time_recipient ?? "",
          }),
        },
        true,
        tenantId || undefined,
      );
      return mapMyNotificationSettings(payload);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["myNotificationSettings"] });
    },
  });
}

export function useAdminNotificationSettings(tenantId: string) {
  return useQuery({
    queryKey: ["adminNotificationSettings", tenantId],
    queryFn: async () => {
      const items = await coreFetch("/api/v1/admin/notification-settings", { method: "GET" }, true, tenantId);
      return ensureArray(items).map(mapAdminNotificationSetting);
    },
    enabled: !!tenantId,
  });
}

export function useSaveAdminNotificationSetting() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ userId, data }: { userId: string; data: any }) => {
      const tenantId = String(data?.tenantId || data?.tenant_id || "").trim();
      const payload = await coreFetch(
        `/api/v1/admin/notification-settings/${encodeURIComponent(userId)}`,
        {
          method: "PUT",
          body: JSON.stringify({
            delivery_enabled: Boolean(data?.deliveryEnabled ?? data?.delivery_enabled),
            delivery_channel: String(data?.deliveryChannel || data?.delivery_channel || "in_app").trim().toLowerCase(),
            telegram_bot_id: data?.telegramBotId ?? data?.telegram_bot_id ?? "",
            telegram_chat_id: data?.telegramChatId ?? data?.telegram_chat_id ?? "",
            telegram_username: data?.telegramUsername ?? data?.telegram_username ?? "",
            notification_email: data?.notificationEmail ?? data?.notification_email ?? "",
            time_recipient: data?.timeRecipient ?? data?.time_recipient ?? "",
          }),
        },
        true,
        tenantId || undefined,
      );
      return mapAdminNotificationSetting(payload);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["adminNotificationSettings"] });
      qc.invalidateQueries({ queryKey: ["myNotificationSettings"] });
    },
  });
}

export function useServiceAlertRules(tenantId: string) {
  return useQuery({
    queryKey: ["serviceAlertRules", tenantId],
    queryFn: async () => {
      const items = await coreFetch("/api/v1/catalog/service_alert_rules?limit=500", { method: "GET" }, true, tenantId);
      return ensureArray(items).map(mapServiceAlertRule);
    },
    enabled: !!tenantId,
  });
}

export function useCreateServiceAlertRule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: any) => {
      const tenantId = String(data?.tenantId || data?.tenant_id || "").trim();
      const payload = await coreFetch(
        "/api/v1/catalog/service_alert_rules",
        {
          method: "POST",
          body: JSON.stringify({
            data: {
              name: String(data?.name || "Service alert").trim(),
              description: String(data?.description || "").trim(),
              metric_type: String(data?.metricType || data?.metric_type || "low_rps").trim().toLowerCase(),
              module: String(data?.module || "api").trim().toLowerCase(),
              min_rps: Number(data?.minRps ?? data?.min_rps ?? 0),
              max_latency_ms: Number(data?.maxLatencyMs ?? data?.max_latency_ms ?? 0),
              sla_seconds: Number(data?.slaSeconds ?? data?.sla_seconds ?? 0),
              cases_threshold: Number(data?.casesThreshold ?? data?.cases_threshold ?? data?.caseThreshold ?? data?.case_threshold ?? data?.threshold ?? 0),
              threshold_mode: String(data?.thresholdMode || data?.threshold_mode || data?.scope || "in_work").trim().toLowerCase(),
              criticality_levels: ensureArray(
                data?.criticalityLevels ?? data?.criticality_levels ?? data?.criticalSeverities ?? data?.critical_severities ?? [],
              )
                .map((value: any) => String(value || "").trim().toLowerCase())
                .filter(Boolean),
              escalation_event_types: ensureArray(data?.escalationEventTypes ?? data?.escalation_event_types ?? data?.eventTypes ?? data?.event_types ?? [])
                .map((value: any) => String(value || "").trim().toLowerCase())
                .filter(Boolean),
              ping_inactivity_seconds: Number(data?.pingInactivitySeconds ?? data?.ping_inactivity_seconds ?? data?.pingSeconds ?? data?.ping_seconds ?? 0),
              open_statuses: ensureArray(data?.openStatuses ?? data?.open_statuses ?? data?.statuses ?? [])
                .map((value: any) => String(value || "").trim().toLowerCase())
                .filter(Boolean),
              window_seconds: Math.max(30, Number(data?.windowSeconds ?? data?.window_seconds ?? 300)),
              cooldown_seconds: Math.max(30, Number(data?.cooldownSeconds ?? data?.cooldown_seconds ?? 900)),
              severity: String(data?.severity || "warning").trim().toLowerCase(),
              enabled: Boolean(data?.enabled ?? true),
            },
          }),
        },
        true,
        tenantId || undefined,
      );
      return mapServiceAlertRule(payload);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["serviceAlertRules"] });
    },
  });
}

export function useUpdateServiceAlertRule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, data }: { id: string; data: any }) => {
      const tenantId = String(data?.tenantId || data?.tenant_id || "").trim();
      const payload = await coreFetch(
        `/api/v1/catalog/service_alert_rules/${encodeURIComponent(id)}`,
        {
          method: "PATCH",
          body: JSON.stringify({
            data: {
              name: String(data?.name || "Service alert").trim(),
              description: String(data?.description || "").trim(),
              metric_type: String(data?.metricType || data?.metric_type || "low_rps").trim().toLowerCase(),
              module: String(data?.module || "api").trim().toLowerCase(),
              min_rps: Number(data?.minRps ?? data?.min_rps ?? 0),
              max_latency_ms: Number(data?.maxLatencyMs ?? data?.max_latency_ms ?? 0),
              sla_seconds: Number(data?.slaSeconds ?? data?.sla_seconds ?? 0),
              cases_threshold: Number(data?.casesThreshold ?? data?.cases_threshold ?? data?.caseThreshold ?? data?.case_threshold ?? data?.threshold ?? 0),
              threshold_mode: String(data?.thresholdMode || data?.threshold_mode || data?.scope || "in_work").trim().toLowerCase(),
              criticality_levels: ensureArray(
                data?.criticalityLevels ?? data?.criticality_levels ?? data?.criticalSeverities ?? data?.critical_severities ?? [],
              )
                .map((value: any) => String(value || "").trim().toLowerCase())
                .filter(Boolean),
              escalation_event_types: ensureArray(data?.escalationEventTypes ?? data?.escalation_event_types ?? data?.eventTypes ?? data?.event_types ?? [])
                .map((value: any) => String(value || "").trim().toLowerCase())
                .filter(Boolean),
              ping_inactivity_seconds: Number(data?.pingInactivitySeconds ?? data?.ping_inactivity_seconds ?? data?.pingSeconds ?? data?.ping_seconds ?? 0),
              open_statuses: ensureArray(data?.openStatuses ?? data?.open_statuses ?? data?.statuses ?? [])
                .map((value: any) => String(value || "").trim().toLowerCase())
                .filter(Boolean),
              window_seconds: Math.max(30, Number(data?.windowSeconds ?? data?.window_seconds ?? 300)),
              cooldown_seconds: Math.max(30, Number(data?.cooldownSeconds ?? data?.cooldown_seconds ?? 900)),
              severity: String(data?.severity || "warning").trim().toLowerCase(),
              enabled: Boolean(data?.enabled ?? true),
            },
          }),
        },
        true,
        tenantId || undefined,
      );
      return mapServiceAlertRule(payload);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["serviceAlertRules"] });
    },
  });
}

export function useDeleteServiceAlertRule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: string | { id: string; tenantId?: string; tenant_id?: string }) => {
      const id = typeof input === "string" ? input : input.id;
      const tenantId = typeof input === "string" ? "" : String(input?.tenantId || input?.tenant_id || "").trim();
      return coreFetch(`/api/v1/catalog/service_alert_rules/${encodeURIComponent(id)}`, { method: "DELETE" }, true, tenantId || undefined);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["serviceAlertRules"] });
    },
  });
}

export function useNotifications(userId: string, tenantId: string) {
  return useQuery({
    queryKey: ["notifications", userId, tenantId],
    queryFn: async () => {
      const items = await listCatalog("notifications", { owner_id: userId });
      return items.map((item: any) => ({
        id: item.id,
        userId: item.userId || item.user_id || item.owner_id,
        tenantId: item.tenantId || item.tenant_id,
        title: item.title,
        message: item.message,
        type: item.type || "info",
        read: Boolean(item.read),
        global: Boolean(item.global),
        createdAt: item.createdAt || item.created_at || new Date().toISOString(),
      }));
    },
    refetchInterval: 30000,
    enabled: !!userId && !!tenantId,
  });
}

export function useCreateNotification() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: any) =>
      createCatalog(
        "notifications",
        {
          title: data?.title,
          message: data?.message,
          type: data?.type || "info",
          read: Boolean(data?.read),
          global: Boolean(data?.global),
          createdAt: data?.createdAt || new Date().toISOString(),
          userId: data?.userId,
          tenantId: data?.tenantId,
        },
        data?.userId,
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }),
  });
}

export function useMarkNotificationRead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => updateCatalog("notifications", id, { read: true }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }),
  });
}

export function useMarkAllNotificationsRead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: { userId: string; tenantId: string }) => {
      const items = await listCatalog("notifications", { owner_id: data.userId });
      await Promise.all(items.filter((i: any) => !i.read).map((item: any) => updateCatalog("notifications", item.id, { read: true })));
      return { success: true };
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }),
  });
}

export function useDeleteNotification() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteCatalog("notifications", id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }),
  });
}

export function useClearNotifications() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: { userId: string; tenantId: string }) => {
      const items = await listCatalog("notifications", { owner_id: data.userId });
      await Promise.all(
        items
          .filter((item: any) => String(item?.id || "").trim().length > 0)
          .map((item: any) => deleteCatalog("notifications", String(item.id))),
      );
      return { success: true };
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }),
  });
}
