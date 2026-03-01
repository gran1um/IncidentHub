import { Plus, Target, Trash2, XCircle } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { TabsContent } from "@/components/ui/tabs";
import { MITRE_TACTICS, MITRE_TECHNIQUES } from "../helpers";

type CaseMitreTabProps = {
  t: (key: string, params?: Record<string, string>) => string;
  caseData: any;
  mitreDialogOpen: boolean;
  setMitreDialogOpen: (value: boolean) => void;
  onAddMitre: (kind: "tactic" | "technique", value: string) => void;
  onRemoveMitre: (kind: "tactic" | "technique", value: string) => void;
  panelClass: string;
  subpanelClass: string;
};

export function CaseMitreTab({
  t,
  caseData,
  mitreDialogOpen,
  setMitreDialogOpen,
  onAddMitre,
  onRemoveMitre,
  panelClass,
  subpanelClass,
}: CaseMitreTabProps) {
  return (
    <TabsContent value="mitre" className="mt-6 space-y-6" data-testid="tab-content-mitre">
      <div className="flex items-center justify-between">
        <h2 className="flex items-center gap-2 text-lg font-semibold text-white">
          <Target size={20} className="text-[#f87171]" /> {t("caseDetail.mitre.title")}
        </h2>
        <Dialog open={mitreDialogOpen} onOpenChange={setMitreDialogOpen}>
          <DialogTrigger asChild>
            <Button className="rounded-xl gap-2 bg-[#11141d] font-semibold text-[#f3f4f6] hover:bg-[#1d2433]" data-testid="button-add-mitre">
              <Plus size={14} /> {t("caseDetail.mitre.add")}
            </Button>
          </DialogTrigger>
          <DialogContent className="max-h-[80vh] max-w-2xl overflow-y-auto rounded-2xl border border-[#2a2c3c] bg-[#13141c] text-[#f3f4f6]">
            <DialogHeader>
              <DialogTitle>{t("caseDetail.mitre.addDialogTitle")}</DialogTitle>
            </DialogHeader>
            <div className="mt-4 space-y-4">
              {MITRE_TACTICS.map((tactic) => {
                const isAdded = (caseData.tactics || []).includes(tactic.id);
                return (
                  <div key={tactic.id} className={`${subpanelClass} p-3`}>
                    <div className="flex items-center justify-between">
                      <div>
                        <span className="font-mono text-xs text-[#9ca3af]">{tactic.id}</span>
                        <span className="ml-2 text-sm font-bold">{tactic.name}</span>
                      </div>
                      <Button
                        variant={isAdded ? "secondary" : "outline"}
                        size="sm"
                        className={`rounded-lg text-xs ${isAdded ? "bg-[rgba(59,130,246,0.2)] text-[#dbeafe] hover:bg-[rgba(59,130,246,0.28)]" : "border-[#2a2c3c] bg-[#0f1118] text-[#d1d5db] hover:bg-[#1a1f2d]"}`}
                        onClick={() => (isAdded ? onRemoveMitre("tactic", tactic.id) : onAddMitre("tactic", tactic.id))}
                        data-testid={`mitre-tactic-${tactic.id}`}
                      >
                        {isAdded ? t("caseDetail.mitre.remove") : t("caseDetail.mitre.addShort")}
                      </Button>
                    </div>
                    {MITRE_TECHNIQUES[tactic.id] ? (
                      <div className="ml-4 mt-2 space-y-1">
                        {MITRE_TECHNIQUES[tactic.id].map((technique) => {
                          const techAdded = (caseData.techniques || []).includes(technique.id);
                          return (
                            <div key={technique.id} className="flex items-center justify-between py-1">
                              <div>
                                <span className="font-mono text-[10px] text-[#9ca3af]">{technique.id}</span>
                                <span className="ml-2 text-xs">{technique.name}</span>
                              </div>
                              <Button
                                variant={techAdded ? "secondary" : "ghost"}
                                size="sm"
                                className={`h-6 rounded-lg text-[10px] ${techAdded ? "bg-[rgba(59,130,246,0.2)] text-[#dbeafe] hover:bg-[rgba(59,130,246,0.28)]" : "text-[#d1d5db] hover:bg-[#1a1f2d]"}`}
                                onClick={() => (techAdded ? onRemoveMitre("technique", technique.id) : onAddMitre("technique", technique.id))}
                                data-testid={`mitre-technique-${technique.id}`}
                              >
                                {techAdded ? "✓" : "+"}
                              </Button>
                            </div>
                          );
                        })}
                      </div>
                    ) : null}
                  </div>
                );
              })}
            </div>
          </DialogContent>
        </Dialog>
      </div>

      {!caseData.tactics?.length && !caseData.techniques?.length ? (
        <Card className={panelClass}>
          <CardContent className="py-12 text-center text-[#9ca3af]" data-testid="mitre-empty">
            <Target size={32} className="mx-auto mb-3 opacity-50" />
            <p className="text-sm">{t("caseDetail.mitre.empty")}</p>
            <p className="mt-1 text-xs">{t("caseDetail.mitre.emptyHint")}</p>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          {(caseData.tactics || []).map((tacticId: string) => {
            const tactic = MITRE_TACTICS.find((item) => item.id === tacticId);
            const relatedTechniques = (caseData.techniques || [])
              .filter((techId: string) => MITRE_TECHNIQUES[tacticId]?.some((technique) => technique.id === techId))
              .map((techId: string) => {
                const allTechniques = Object.values(MITRE_TECHNIQUES).flat();
                return allTechniques.find((technique) => technique.id === techId);
              })
              .filter(Boolean);

            return (
              <Card key={tacticId} className={panelClass} data-testid={`mitre-card-${tacticId}`}>
                <CardHeader className="pb-3">
                  <CardTitle className="flex items-center gap-3">
                    <Badge className="rounded-lg border border-[rgba(239,68,68,0.3)] bg-[rgba(239,68,68,0.16)] font-mono text-xs text-[#fca5a5]">{tacticId}</Badge>
                    <span className="text-sm font-bold">{tactic?.name || tacticId}</span>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="ml-auto h-7 text-xs text-[#fda4af] hover:bg-[rgba(244,63,94,0.16)] hover:text-[#fda4af]"
                      onClick={() => onRemoveMitre("tactic", tacticId)}
                      data-testid={`button-remove-tactic-${tacticId}`}
                    >
                      <Trash2 size={12} />
                    </Button>
                  </CardTitle>
                </CardHeader>
                {relatedTechniques.length > 0 ? (
                  <CardContent className="pt-0">
                    <div className="flex flex-wrap gap-2">
                      {relatedTechniques.map((technique: any) => (
                        <Badge
                          key={technique.id}
                          variant="outline"
                          className="gap-1 rounded-lg pr-1 text-xs"
                          data-testid={`mitre-tech-badge-${technique.id}`}
                        >
                          <span className="font-mono text-[10px] text-[#9ca3af]">{technique.id}</span>
                          {technique.name}
                          <Button
                            variant="ghost"
                            size="icon"
                            className="ml-1 h-5 w-5 rounded-full p-0 text-[#fda4af] hover:bg-[rgba(244,63,94,0.16)] hover:text-[#fda4af]"
                            onClick={() => onRemoveMitre("technique", technique.id)}
                          >
                            <XCircle size={10} />
                          </Button>
                        </Badge>
                      ))}
                    </div>
                  </CardContent>
                ) : null}
              </Card>
            );
          })}

          {(() => {
            const addedTacticTechIDs = new Set(
              (caseData.tactics || []).flatMap((tacticId: string) => (MITRE_TECHNIQUES[tacticId] || []).map((technique) => technique.id)),
            );
            const orphanTechniques = (caseData.techniques || []).filter((techId: string) => !addedTacticTechIDs.has(techId));
            if (orphanTechniques.length === 0) {
              return null;
            }
            const allTechniques = Object.values(MITRE_TECHNIQUES).flat();
            return (
              <Card className={panelClass} data-testid="mitre-orphan-techniques">
                <CardHeader className="pb-3">
                  <CardTitle className="text-sm font-bold text-[#9ca3af]">{t("caseDetail.mitre.otherTechniques")}</CardTitle>
                </CardHeader>
                <CardContent className="pt-0">
                  <div className="flex flex-wrap gap-2">
                    {orphanTechniques.map((techniqueId: string) => {
                      const technique = allTechniques.find((item) => item.id === techniqueId);
                      return (
                        <Badge key={techniqueId} variant="outline" className="gap-1 rounded-lg pr-1 text-xs">
                          <span className="font-mono text-[10px] text-[#9ca3af]">{techniqueId}</span>
                          {technique?.name || techniqueId}
                          <Button
                            variant="ghost"
                            size="icon"
                            className="ml-1 h-5 w-5 rounded-full p-0 text-[#fda4af] hover:bg-[rgba(244,63,94,0.16)] hover:text-[#fda4af]"
                            onClick={() => onRemoveMitre("technique", techniqueId)}
                          >
                            <XCircle size={10} />
                          </Button>
                        </Badge>
                      );
                    })}
                  </div>
                </CardContent>
              </Card>
            );
          })()}
        </div>
      )}
    </TabsContent>
  );
}
