import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Plus, Save } from "lucide-react";

import type { WorkflowNodeTemplate } from "@/features/workflow-studio/node-library";

export function WorkflowCredentialManagerDialog({
  open,
  onOpenChange,
  credentials,
  editingCredentialId,
  draftName,
  draftDescription,
  draftFields,
  selectedNodeTemplate,
  actionButtonClass,
  onCreateNew,
  onSelectCredential,
  onDraftNameChange,
  onDraftDescriptionChange,
  onDraftFieldChange,
  onDelete,
  onSave,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  credentials: any[];
  editingCredentialId: string | null;
  draftName: string;
  draftDescription: string;
  draftFields: Record<string, string>;
  selectedNodeTemplate: WorkflowNodeTemplate | null;
  actionButtonClass: string;
  onCreateNew: () => void;
  onSelectCredential: (credentialId: string) => void;
  onDraftNameChange: (value: string) => void;
  onDraftDescriptionChange: (value: string) => void;
  onDraftFieldChange: (fieldId: string, value: string) => void;
  onDelete: () => void;
  onSave: () => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-4xl border-[#2a2c3c] bg-[#111622] text-white">
        <DialogHeader>
          <DialogTitle>Workflow credentials</DialogTitle>
          <DialogDescription className="text-[#9ca3af]">
            Store and reuse one credential bundle per node type. Sensitive values are encrypted in workflow vault and injected at runtime.
          </DialogDescription>
        </DialogHeader>
        <div className="mt-3 grid gap-4 md:grid-cols-[minmax(0,0.45fr)_minmax(0,0.55fr)]">
          <div className="space-y-2">
            <p className="text-xs font-semibold uppercase text-[#9ca3af]">Saved credentials</p>
            <div className="max-h-[260px] space-y-2 overflow-y-auto rounded-lg border border-[#2a2c3c] bg-[#0f131d] p-2">
              {credentials.length === 0 ? (
                <p className="text-xs text-[#9ca3af]">No credentials for this node type yet.</p>
              ) : (
                credentials.map((credential: any) => (
                  <button
                    key={credential.id}
                    type="button"
                    onClick={() => onSelectCredential(String(credential.id))}
                    className={`flex w-full items-center justify-between gap-2 rounded-md border px-3 py-2 text-left text-xs transition-colors ${
                      editingCredentialId === String(credential.id)
                        ? "border-[#4b5563] bg-[#161927] text-white"
                        : "border-[#2a2c3c] bg-[#0f131d] text-[#e5e7eb] hover:border-[#4b5168] hover:bg-[#171b2a]"
                    }`}
                  >
                    <span className="truncate font-medium">{credential.name || credential.id}</span>
                    <span className="truncate text-[11px] text-[#9ca3af]">{credential.description}</span>
                  </button>
                ))
              )}
            </div>
            <Button type="button" size="sm" variant="outline" className={actionButtonClass} onClick={onCreateNew}>
              <Plus size={14} className="mr-2" />
              New credentials
            </Button>
          </div>
          <div className="space-y-3">
            <div className="space-y-2">
              <Label>Credential name</Label>
              <Input value={draftName} onChange={(event) => onDraftNameChange(event.target.value)} placeholder="S3 production access" />
            </div>
            <div className="space-y-2">
              <Label>Description</Label>
              <Textarea
                value={draftDescription}
                onChange={(event) => onDraftDescriptionChange(event.target.value)}
                placeholder="Used by object storage nodes in workflows"
                className="min-h-[72px]"
              />
            </div>
            <div className="space-y-2">
              <Label>Fields</Label>
              {selectedNodeTemplate ? (
                <div className="max-h-[220px] space-y-2 overflow-y-auto rounded-lg border border-[#2a2c3c] bg-[#0f131d] p-3">
                  {(selectedNodeTemplate.fields || []).map((field) => (
                    <div key={field.id} className="space-y-1">
                      <div className="flex items-center justify-between gap-2">
                        <span className="text-xs font-medium text-[#e5e7eb]">{field.label}</span>
                        {field.sensitive && (
                          <span className="rounded-md bg-[#111827] px-1.5 py-0.5 text-[10px] text-[#f97316]">secret</span>
                        )}
                      </div>
                      <Input
                        type={field.sensitive ? "password" : "text"}
                        value={draftFields[field.id] ?? ""}
                        onChange={(event) => onDraftFieldChange(field.id, event.target.value)}
                        placeholder={field.placeholder}
                        className="text-xs"
                      />
                      {field.description ? <p className="text-[11px] text-[#6b7280]">{field.description}</p> : null}
                    </div>
                  ))}
                </div>
              ) : (
                <p className="text-xs text-[#9ca3af]">Select a node to configure credentials.</p>
              )}
            </div>
          </div>
        </div>
        <DialogFooter className="mt-4 flex flex-wrap items-center justify-between gap-2">
          <div className="flex flex-wrap items-center gap-2 text-[11px] text-[#9ca3af]">
            <span>Secrets are never shown back after save and are injected only at execution time.</span>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            {editingCredentialId ? (
              <Button type="button" variant="outline" className="border-red-500/50 text-red-400" onClick={onDelete}>
                Delete
              </Button>
            ) : null}
            <Button type="button" variant="outline" className={actionButtonClass} onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="button" onClick={onSave}>
              <Save size={14} className="mr-2" />
              Save credentials
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
