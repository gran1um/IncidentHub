import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { Plus, Save, Trash2 } from "lucide-react";
import { useAppState, useCaseStatuses, useSaveCaseStatuses, useTenants } from "@/lib/api";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useT } from "@/lib/i18n";

type CaseStatusDraft = {
  id?: string;
  code: string;
  label: string;
  order: number;
  isClosed: boolean;
  color: string;
};

function sanitizeStatusCode(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/\s+/g, "_")
    .replace(/[^a-z0-9_-]/g, "");
}

export function CaseStatusesPanel({ tenantId }: { tenantId: string }) {
  const t = useT();
  const { session } = useAppState();
  const isPlatformAdmin = Boolean(session?.identity?.is_platform_admin);
  const { data: tenants = [] } = useTenants();
  const [selectedTenantId, setSelectedTenantId] = useState(tenantId);
  const effectiveTenantId = isPlatformAdmin ? (selectedTenantId || tenantId) : tenantId;
  const { data: statuses = [], isLoading } = useCaseStatuses(effectiveTenantId);
  const saveStatuses = useSaveCaseStatuses();
  const [draft, setDraft] = useState<CaseStatusDraft[]>([]);

  useEffect(() => {
    if (!isPlatformAdmin) {
      setSelectedTenantId(tenantId);
      return;
    }
    if (!selectedTenantId && tenantId) {
      setSelectedTenantId(tenantId);
    }
  }, [tenantId, isPlatformAdmin, selectedTenantId]);

  const tenantOptions = useMemo(() => {
    if (!isPlatformAdmin) {
      return tenants.filter((tenant: any) => tenant.id === tenantId);
    }
    return tenants;
  }, [isPlatformAdmin, tenantId, tenants]);

  useEffect(() => {
    const normalized = (statuses || [])
      .map((item: any, index: number) => ({
        id: item.id,
        code: item.code,
        label: item.label,
        order: Number(item.order ?? (index + 1) * 10),
        isClosed: Boolean(item.isClosed),
        color: item.color || "",
      }))
      .sort((a: CaseStatusDraft, b: CaseStatusDraft) => a.order - b.order || a.code.localeCompare(b.code));
    setDraft(normalized);
  }, [statuses]);

  const canSave = useMemo(() => {
    if (draft.length === 0) return false;
    const codes = new Set<string>();
    let hasOpen = false;
    for (const item of draft) {
      const code = sanitizeStatusCode(item.code);
      if (!code) return false;
      if (codes.has(code)) return false;
      codes.add(code);
      if (!item.isClosed) {
        hasOpen = true;
      }
    }
    return hasOpen;
  }, [draft]);

  const updateItem = (index: number, patch: Partial<CaseStatusDraft>) => {
    setDraft((prev) =>
      prev.map((item, i) => {
        if (i !== index) return item;
        const next = { ...item, ...patch };
        if (patch.code !== undefined) {
          next.code = sanitizeStatusCode(patch.code);
        }
        return next;
      }),
    );
  };

  const addStatus = () => {
    setDraft((prev) => [
      ...prev,
      {
        code: "",
        label: "",
        order: (prev.length + 1) * 10,
        isClosed: false,
        color: "",
      },
    ]);
  };

  const removeStatus = (index: number) => {
    setDraft((prev) => prev.filter((_, i) => i !== index));
  };

  const save = () => {
    if (!canSave) {
      toast.error("At least one open status and unique codes are required");
      return;
    }
    saveStatuses.mutate(
      {
        tenantId: effectiveTenantId,
        statuses: draft.map((item, index) => ({
          code: sanitizeStatusCode(item.code),
          label: item.label.trim(),
          order: Number(item.order || (index + 1) * 10),
          isClosed: Boolean(item.isClosed),
          color: item.color.trim(),
        })),
      },
      {
        onSuccess: () => {
          toast.success(t("caseStatuses.toast.saved"));
        },
      },
    );
  };

  return (
    <Card className="fintech-card p-6 space-y-4">
      <div className="flex items-center justify-between gap-2">
        <div>
          <h3 className="font-bold text-lg">Case Statuses</h3>
          <p className="text-xs text-muted-foreground">Tenant-specific case workflow states.</p>
        </div>
        {isPlatformAdmin && tenantOptions.length > 0 && (
          <div className="min-w-[260px]">
            <Label className="text-xs">Tenant</Label>
            <Select value={effectiveTenantId} onValueChange={setSelectedTenantId}>
              <SelectTrigger data-testid="select-case-status-tenant">
                <SelectValue placeholder="Select tenant" />
              </SelectTrigger>
              <SelectContent>
                {tenantOptions.map((tenant: any) => (
                  <SelectItem key={tenant.id} value={tenant.id}>
                    {(tenant.slug || tenant.id) + (tenant.name && tenant.name !== tenant.slug ? ` — ${tenant.name}` : "")}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}
        <div className="flex items-center gap-2">
          <Button variant="outline" className="gap-2" onClick={addStatus} data-testid="button-add-case-status">
            <Plus size={14} />
            {t("caseStatuses.button.add")}
          </Button>
          <Button className="gap-2" onClick={save} disabled={!canSave || saveStatuses.isPending} data-testid="button-save-case-statuses">
            <Save size={14} />
            {saveStatuses.isPending ? t("caseStatuses.button.saving") : t("caseStatuses.button.save")}
          </Button>
        </div>
      </div>

      {isLoading ? (
        <div className="text-sm text-muted-foreground">Loading statuses...</div>
      ) : (
        <div className="space-y-3">
          {draft.map((item, index) => (
            <div key={`${item.id || "new"}-${index}`} className="rounded-xl border p-3 space-y-3">
              <div className="grid md:grid-cols-5 gap-3">
                <div className="space-y-1">
                  <Label className="text-xs">Code</Label>
                  <Input
                    value={item.code}
                    onChange={(e) => updateItem(index, { code: e.target.value })}
                    placeholder="in_progress"
                    data-testid={`input-case-status-code-${index}`}
                  />
                </div>
                <div className="space-y-1 md:col-span-2">
                  <Label className="text-xs">Label</Label>
                  <Input
                    value={item.label}
                    onChange={(e) => updateItem(index, { label: e.target.value })}
                    placeholder="In Progress"
                    data-testid={`input-case-status-label-${index}`}
                  />
                </div>
                <div className="space-y-1">
                  <Label className="text-xs">Order</Label>
                  <Input
                    type="number"
                    value={item.order}
                    onChange={(e) => updateItem(index, { order: Number(e.target.value || 0) })}
                    data-testid={`input-case-status-order-${index}`}
                  />
                </div>
                <div className="space-y-1">
                  <Label className="text-xs">Color</Label>
                  <Input
                    value={item.color}
                    onChange={(e) => updateItem(index, { color: e.target.value })}
                    placeholder="#3b82f6"
                    data-testid={`input-case-status-color-${index}`}
                  />
                </div>
              </div>
              <div className="flex items-center justify-between">
                <label className="flex items-center gap-2 text-xs text-muted-foreground">
                  <Switch
                    checked={item.isClosed}
                    onCheckedChange={(checked) => updateItem(index, { isClosed: checked })}
                    data-testid={`switch-case-status-closed-${index}`}
                  />
                  Closed status
                </label>
                <Button
                  variant="ghost"
                  className="text-red-500 hover:text-red-700"
                  onClick={() => removeStatus(index)}
                  data-testid={`button-delete-case-status-${index}`}
                >
                  <Trash2 size={14} />
                </Button>
              </div>
            </div>
          ))}
          {draft.length === 0 && (
            <div className="text-sm text-muted-foreground">No statuses configured yet.</div>
          )}
        </div>
      )}
    </Card>
  );
}
