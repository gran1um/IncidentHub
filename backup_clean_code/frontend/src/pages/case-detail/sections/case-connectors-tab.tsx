import type { Dispatch, SetStateAction } from "react";

import { Play, Plug } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { TabsContent } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";

import { formatAnalysisDate, getConnectorHubStatusBadgeClass } from "../helpers";

type ConnectorHubDraft = {
  methodId: string;
  action: string;
  message: string;
  inputJson: string;
  metadataJson: string;
  dryRun: boolean;
};

type CaseConnectorsTabProps = {
  outboundConnectors: any[];
  connectorHubConnectorID: string;
  setConnectorHubConnectorID: (value: string) => void;
  connectorHubMethods: any[];
  connectorHubDraft: ConnectorHubDraft;
  setConnectorHubDraft: Dispatch<SetStateAction<ConnectorHubDraft>>;
  selectedConnectorHubMethod: any;
  connectorHubRuns: any[];
  executePending: boolean;
  onRun: () => void;
  onOpenExecution: (executionID: string) => void;
  panelClass: string;
  inputClass: string;
  selectContentClass: string;
};

export function CaseConnectorsTab({
  outboundConnectors,
  connectorHubConnectorID,
  setConnectorHubConnectorID,
  connectorHubMethods,
  connectorHubDraft,
  setConnectorHubDraft,
  selectedConnectorHubMethod,
  connectorHubRuns,
  executePending,
  onRun,
  onOpenExecution,
  panelClass,
  inputClass,
  selectContentClass,
}: CaseConnectorsTabProps) {
  return (
    <TabsContent value="connectors" className="mt-6 space-y-6" data-testid="tab-content-connectors">
      <Card className={panelClass} data-testid="card-case-connector-hub">
        <CardHeader>
          <div className="flex items-center justify-between gap-3">
            <CardTitle className="flex items-center gap-2 text-sm font-semibold uppercase tracking-[0.12em] text-[#9ca3af]">
              <Plug size={16} className="text-primary" /> Connector Hub
            </CardTitle>
            <Button
              variant="outline"
              size="sm"
              className="h-8 rounded-lg text-xs"
              disabled={executePending || !connectorHubConnectorID}
              onClick={onRun}
              data-testid="button-case-connector-hub-run"
            >
              <Play size={13} className="mr-1" />
              {executePending ? "Running..." : "Run"}
            </Button>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          {outboundConnectors.length === 0 ? (
            <div className="rounded-xl border border-dashed p-4 text-sm text-[#9ca3af]" data-testid="case-connector-hub-empty">
              No outbound connectors available for this tenant.
            </div>
          ) : (
            <>
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                <div className="space-y-1.5">
                  <Label className="text-xs uppercase tracking-wide text-[#9ca3af]">Connector</Label>
                  <Select
                    value={connectorHubConnectorID || "__none__"}
                    onValueChange={(value) => setConnectorHubConnectorID(value === "__none__" ? "" : value)}
                  >
                    <SelectTrigger className={inputClass} data-testid="select-case-connector-hub-connector">
                      <SelectValue placeholder="Select connector" />
                    </SelectTrigger>
                    <SelectContent className={selectContentClass}>
                      <SelectItem value="__none__">Select connector</SelectItem>
                      {outboundConnectors.map((connector: any) => (
                        <SelectItem key={`case-connector-hub-connector-${connector.id}`} value={String(connector.id)}>
                          {connector.name || connector.id}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <Label className="text-xs uppercase tracking-wide text-[#9ca3af]">Method</Label>
                  <Select
                    value={connectorHubDraft.methodId || "__none__"}
                    onValueChange={(value) => {
                      const methodID = value === "__none__" ? "" : value;
                      const method = connectorHubMethods.find((item: any) => String(item?.id || "") === methodID) || null;
                      setConnectorHubDraft((prev) => ({
                        ...prev,
                        methodId: methodID,
                        action: method ? String(method.action || method.slug || method.name || "").trim() : prev.action,
                      }));
                    }}
                  >
                    <SelectTrigger className={inputClass} data-testid="select-case-connector-hub-method">
                      <SelectValue placeholder="Without method" />
                    </SelectTrigger>
                    <SelectContent className={selectContentClass}>
                      <SelectItem value="__none__">Without method</SelectItem>
                      {connectorHubMethods.map((method: any) => (
                        <SelectItem key={`case-connector-hub-method-${method.id}`} value={String(method.id)}>
                          {method.name || method.id}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>

              <div className="grid grid-cols-1 items-end gap-3 md:grid-cols-[minmax(0,1fr)_auto]">
                <div className="space-y-1.5">
                  <Label className="text-xs uppercase tracking-wide text-[#9ca3af]">Action key</Label>
                  <Input
                    value={connectorHubDraft.action}
                    onChange={(event) => setConnectorHubDraft((prev) => ({ ...prev, action: event.target.value }))}
                    placeholder="lookup_hash"
                    className={inputClass}
                    data-testid="input-case-connector-hub-action"
                  />
                </div>
                <label className="flex h-9 items-center gap-2 rounded-lg border border-[#2a2c3c] bg-[#0f1118] px-3 text-xs text-[#d1d5db]">
                  <Checkbox
                    checked={connectorHubDraft.dryRun}
                    onCheckedChange={(checked) =>
                      setConnectorHubDraft((prev) => ({
                        ...prev,
                        dryRun: checked !== false,
                      }))
                    }
                    data-testid="checkbox-case-connector-hub-dry-run"
                  />
                  Dry run
                </label>
              </div>

              <div className="space-y-1.5">
                <Label className="text-xs uppercase tracking-wide text-[#9ca3af]">Message override</Label>
                <Textarea
                  value={connectorHubDraft.message}
                  onChange={(event) => setConnectorHubDraft((prev) => ({ ...prev, message: event.target.value }))}
                  rows={2}
                  placeholder={selectedConnectorHubMethod ? `Uses template from ${selectedConnectorHubMethod.name || selectedConnectorHubMethod.id}` : "Optional raw message"}
                  className="min-h-[72px] rounded-lg border-[#2a2c3c] bg-[#0f1118] font-mono text-xs text-[#e5e7eb] placeholder:text-[#6b7280] focus-visible:ring-[#3b82f6]/40"
                  data-testid="input-case-connector-hub-message"
                />
              </div>

              <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
                <div className="space-y-1.5">
                  <Label className="text-xs uppercase tracking-wide text-[#9ca3af]">Input JSON</Label>
                  <Textarea
                    value={connectorHubDraft.inputJson}
                    onChange={(event) => setConnectorHubDraft((prev) => ({ ...prev, inputJson: event.target.value }))}
                    rows={6}
                    className="min-h-[140px] rounded-lg border-[#2a2c3c] bg-[#0f1118] font-mono text-xs text-[#e5e7eb] placeholder:text-[#6b7280] focus-visible:ring-[#3b82f6]/40"
                    data-testid="input-case-connector-hub-input-json"
                  />
                </div>
                <div className="space-y-1.5">
                  <Label className="text-xs uppercase tracking-wide text-[#9ca3af]">Metadata JSON</Label>
                  <Textarea
                    value={connectorHubDraft.metadataJson}
                    onChange={(event) => setConnectorHubDraft((prev) => ({ ...prev, metadataJson: event.target.value }))}
                    rows={6}
                    className="min-h-[140px] rounded-lg border-[#2a2c3c] bg-[#0f1118] font-mono text-xs text-[#e5e7eb] placeholder:text-[#6b7280] focus-visible:ring-[#3b82f6]/40"
                    data-testid="input-case-connector-hub-metadata-json"
                  />
                </div>
              </div>
            </>
          )}

          <Separator />
          <div className="space-y-2">
            <div className="flex items-center justify-between gap-2">
              <div className="text-xs font-bold uppercase tracking-wider text-[#9ca3af]">Recent Runs</div>
              <Badge variant="outline" className="rounded-lg text-[10px]">
                {connectorHubRuns.length}
              </Badge>
            </div>
            {connectorHubRuns.length === 0 ? (
              <p className="text-xs text-[#9ca3af]">No connector hub runs for this case yet.</p>
            ) : (
              <div className="max-h-56 space-y-2 overflow-y-auto pr-1" data-testid="case-connector-hub-runs">
                {connectorHubRuns.map((run: any) => (
                  <button
                    key={run.id}
                    type="button"
                    className="w-full rounded-lg border border-[#2a2c3c] bg-[#0f1118] p-3 text-left transition-colors hover:bg-[#141826]"
                    onClick={() => onOpenExecution(String(run?.id || ""))}
                  >
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div className="min-w-0">
                        <div className="truncate text-xs font-semibold text-[#f3f4f6]">
                          {String(run?.method_name || run?.action || run?.method_key || "connector_action")}
                        </div>
                        <div className="text-[11px] text-[#9ca3af]">
                          {formatAnalysisDate(run?.created_at || run?.updated_at)}
                          {run?.connector_name || run?.connector_id ? ` · ${String(run?.connector_name || run?.connector_id)}` : ""}
                        </div>
                      </div>
                      <Badge className={`${getConnectorHubStatusBadgeClass(run?.status)} rounded-md text-[10px]`}>
                        {String(run?.status || "unknown")}
                      </Badge>
                    </div>
                    {run?.error ? (
                      <p className="mt-1 text-[11px] text-[#fda4af]">{String(run.error)}</p>
                    ) : (
                      <p className="mt-1 line-clamp-2 text-[11px] text-[#9ca3af]">
                        {String(run?.response?.reply || run?.request?.message || "Execution completed")}
                      </p>
                    )}
                  </button>
                ))}
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </TabsContent>
  );
}
