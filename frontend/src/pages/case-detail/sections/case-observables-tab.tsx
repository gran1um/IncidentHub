import { Eye, Plus, Trash2, XCircle } from "lucide-react";

import { ObservableConnectorMenu } from "@/features/connectors";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { TabsContent } from "@/components/ui/tabs";
import {
  OBSERVABLE_TYPES,
  VERDICTS,
  VERDICT_KEYS,
  getAIVerdictBadgeClass,
  observableTypeKey,
} from "../helpers";

type CaseObservablesTabProps = {
  t: (key: string, params?: Record<string, string>) => string;
  obsType: string;
  setObsType: (value: string) => void;
  obsValue: string;
  setObsValue: (value: string) => void;
  obsVerdict: string;
  setObsVerdict: (value: string) => void;
  obsTags: string;
  setObsTags: (value: string) => void;
  onCreateObservable: () => void;
  observableRows: any[];
  observableConnectorOptionsByType: Record<string, any[]>;
  observableTagDrafts: Record<string, string>;
  onObservableTagDraftChange: (observableID: string, value: string) => void;
  onAddObservableTag: (observable: any) => void | Promise<void>;
  onRemoveObservableTag: (observable: any, tag: string) => void | Promise<void>;
  onRunConnectorForObservable: (observable: any, option: any) => void | Promise<void>;
  onUpdateObservableType: (observable: any, value: string) => void | Promise<void>;
  onDeleteObservable: (observable: any) => void | Promise<void>;
  executePending: boolean;
  panelClass: string;
  inputClass: string;
  selectTriggerClass: string;
  selectContentClass: string;
};

export function CaseObservablesTab({
  t,
  obsType,
  setObsType,
  obsValue,
  setObsValue,
  obsVerdict,
  setObsVerdict,
  obsTags,
  setObsTags,
  onCreateObservable,
  observableRows,
  observableConnectorOptionsByType,
  observableTagDrafts,
  onObservableTagDraftChange,
  onAddObservableTag,
  onRemoveObservableTag,
  onRunConnectorForObservable,
  onUpdateObservableType,
  onDeleteObservable,
  executePending,
  panelClass,
  inputClass,
  selectTriggerClass,
  selectContentClass,
}: CaseObservablesTabProps) {
  return (
    <TabsContent value="observables" className="mt-6 space-y-6" data-testid="tab-content-observables">
      <Card className={panelClass}>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm font-semibold uppercase tracking-[0.12em] text-[#9ca3af]">
            <Plus size={16} className="text-primary" /> {t("caseDetail.observable.add")}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-4">
            <Select value={obsType} onValueChange={setObsType}>
              <SelectTrigger className={selectTriggerClass} data-testid="select-observable-type">
                <SelectValue />
              </SelectTrigger>
              <SelectContent className={selectContentClass}>
                {OBSERVABLE_TYPES.map((item) => (
                  <SelectItem key={item} value={item}>
                    {item}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Input
              placeholder={t("caseDetail.observable.valuePlaceholder")}
              value={obsValue}
              onChange={(event) => setObsValue(event.target.value)}
              className={inputClass}
              data-testid="input-observable-value"
            />
            <Select value={obsVerdict} onValueChange={setObsVerdict}>
              <SelectTrigger className={selectTriggerClass} data-testid="select-observable-verdict">
                <SelectValue />
              </SelectTrigger>
              <SelectContent className={selectContentClass}>
                {VERDICTS.map((verdict) => (
                  <SelectItem key={verdict} value={verdict}>
                    {t(VERDICT_KEYS[verdict] || verdict)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Input
              placeholder={t("caseDetail.observable.tagsPlaceholder")}
              value={obsTags}
              onChange={(event) => setObsTags(event.target.value)}
              className={inputClass}
              data-testid="input-observable-tags"
            />
          </div>
          <div className="mt-3 flex justify-end">
            <Button
              onClick={onCreateObservable}
              className="rounded-xl gap-2 bg-[#11141d] font-semibold text-[#f3f4f6] hover:bg-[#1d2433]"
              disabled={!obsValue.trim()}
              data-testid="button-add-observable"
            >
              <Plus size={14} /> {t("caseDetail.observable.add")}
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card className={`${panelClass} overflow-hidden`}>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("caseDetail.overview.type")}</TableHead>
              <TableHead>{t("caseDetail.observable.value")}</TableHead>
              <TableHead>{t("caseDetail.observable.verdict")}</TableHead>
              <TableHead>{t("caseDetail.observable.tagsWithEnter")}</TableHead>
              <TableHead className="w-[120px] text-right">{t("common.actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {observableRows.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className="py-12 text-center text-[#9ca3af]" data-testid="observables-empty">
                  <Eye size={32} className="mx-auto mb-3 opacity-50" />
                  <p className="text-sm">{t("caseDetail.observable.empty")}</p>
                </TableCell>
              </TableRow>
            ) : (
              observableRows.map((observable: any) => {
                const typeKey = observableTypeKey(observable.type || "");
                const connectorOptions = observableConnectorOptionsByType[typeKey] || [];
                return (
                  <TableRow key={observable.id} data-testid={`observable-row-${observable.id}`}>
                    <TableCell>
                      <Select value={observable.type} onValueChange={(value) => void onUpdateObservableType(observable, value)}>
                        <SelectTrigger className="h-8 min-w-[132px] rounded-lg border-[#2a2c3c] bg-[#0f1118] text-xs text-[#f3f4f6]" data-testid={`observable-type-${observable.id}`}>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent className={selectContentClass}>
                          {OBSERVABLE_TYPES.map((item) => (
                            <SelectItem key={`${observable.id}-${item}`} value={item}>{item}</SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </TableCell>
                    <TableCell>
                      <code className="rounded bg-[#171b28] px-2 py-1 font-mono text-xs text-[#d1d5db]" data-testid={`observable-value-${observable.id}`}>
                        {observable.value}
                      </code>
                    </TableCell>
                    <TableCell>
                      <Badge className={`${getAIVerdictBadgeClass(observable.verdict)} rounded-lg border text-xs`} data-testid={`observable-verdict-${observable.id}`}>
                        {t(VERDICT_KEYS[observable.verdict] || observable.verdict)}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap items-center gap-1.5">
                        {(observable.tags || []).map((tag: string) => (
                          <Badge key={tag} variant="secondary" className="gap-1 rounded-lg pr-1 text-[10px]">
                            {tag}
                            <Button
                              variant="ghost"
                              size="icon"
                              className="h-4 w-4 rounded-full p-0 text-[#9ca3af] hover:bg-[#1a1f2d] hover:text-white"
                              onClick={() => {
                                void onRemoveObservableTag(observable, tag);
                              }}
                              data-testid={`button-remove-observable-tag-${observable.id}-${tag}`}
                            >
                              <XCircle size={10} />
                            </Button>
                          </Badge>
                        ))}
                        <Input
                          value={observableTagDrafts[observable.id] || ""}
                          onChange={(event) => onObservableTagDraftChange(observable.id, event.target.value)}
                          onKeyDown={(event) => {
                            if (event.key === "Enter") {
                              event.preventDefault();
                              void onAddObservableTag(observable);
                            }
                          }}
                          onBlur={() => {
                            if ((observableTagDrafts[observable.id] || "").trim()) {
                              void onAddObservableTag(observable);
                            }
                          }}
                          placeholder={t("caseDetail.observable.tagsInputEnter")}
                          className="h-7 w-[180px] rounded-lg border-[#2a2c3c] bg-[#0f1118] text-xs text-[#f3f4f6] placeholder:text-[#6b7280]"
                          data-testid={`input-observable-tags-${observable.id}`}
                        />
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center justify-end gap-2">
                        <ObservableConnectorMenu
                          options={connectorOptions}
                          onSelect={(option) => onRunConnectorForObservable(observable, option)}
                          emptyLabel={t("alerts.observableConnectors.noMethods")}
                          disabled={executePending}
                          triggerTestId={`button-enrich-observable-connectors-${observable.id}`}
                        />
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8 text-[#fda4af] hover:bg-[rgba(244,63,94,0.16)] hover:text-[#fda4af]"
                          onClick={() => {
                            void onDeleteObservable(observable);
                          }}
                          data-testid={`button-delete-observable-${observable.id}`}
                        >
                          <Trash2 size={14} />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                );
              })
            )}
          </TableBody>
        </Table>
      </Card>
    </TabsContent>
  );
}
