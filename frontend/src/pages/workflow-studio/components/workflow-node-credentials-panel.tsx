import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

export function WorkflowNodeCredentialsPanel({
  visible,
  credentialId,
  credentials,
  actionButtonClass,
  onSelectCredential,
  onManageCredential,
}: {
  visible: boolean;
  credentialId: string;
  credentials: any[];
  actionButtonClass: string;
  onSelectCredential: (credentialId: string) => void;
  onManageCredential: (mode: "new" | "edit", credentialId?: string) => void;
}) {
  if (!visible) {
    return null;
  }

  return (
    <div className="space-y-2 rounded-lg border border-[#2a2c3c] bg-[#0f131d] px-3 py-2">
      <div className="flex items-center justify-between gap-2">
        <div className="min-w-0">
          <div className="flex items-center gap-2 text-sm font-medium text-[#e5e7eb]">
            <span>Credentials</span>
            <Badge variant="outline" className="rounded-md border-[#2a2c3c] bg-[#111622] text-[10px] uppercase tracking-[0.12em] text-[#9ca3af]">
              Vault-backed
            </Badge>
          </div>
          <p className="text-xs text-[#9ca3af]">
            Select stored credentials for sensitive fields of this node. Secrets stay encrypted in workflow vault.
          </p>
        </div>
        <Button
          type="button"
          size="sm"
          variant="outline"
          className={actionButtonClass}
          onClick={() => onManageCredential(credentialId ? "edit" : "new", credentialId || undefined)}
        >
          Manage
        </Button>
      </div>
      <Select
        value={credentialId || "__none__"}
        onValueChange={(value) => onSelectCredential(value === "__none__" ? "" : value)}
      >
        <SelectTrigger data-testid="select-node-credential">
          <SelectValue placeholder="No credentials" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="__none__">No credentials</SelectItem>
          {credentials.map((credential: any) => (
            <SelectItem key={credential.id} value={credential.id}>
              {credential.name || credential.id}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
