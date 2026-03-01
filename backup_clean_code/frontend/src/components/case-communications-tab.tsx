import { useEffect, useMemo, useState } from "react";
import {
  useCaseCommunication,
  useCaseCommunications,
  useCaseCommunicationConnectors,
  useCommunicationTemplates,
  useCreateCaseCommunication,
  useSendCaseCommunicationMessage,
  useSyncCaseCommunication,
  useUsers,
} from "@/lib/api";
import { useT } from "@/lib/i18n";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { EllipsisText } from "@/components/ui/ellipsis-text";
import { MessageSquare, Plus, RefreshCcw, Send, UserRound } from "lucide-react";
import { toast } from "sonner";
import { UserAvatar } from "@/components/user-avatar";

interface CaseCommunicationsTabProps {
  caseId: string;
  tenantId: string;
  currentUserId: string;
  currentUserName?: string;
}

function formatMessageTime(raw?: string): string {
  if (!raw) return "—";
  const parsed = new Date(raw);
  if (Number.isNaN(parsed.getTime())) return "—";
  return parsed.toLocaleString();
}

function participantLabel(thread: any): string {
  const participant = thread?.participant || {};
  const pieces = [
    participant?.name,
    participant?.target,
    participant?.chat_id,
    participant?.email,
    participant?.external_user_id,
  ]
    .map((value) => String(value || "").trim())
    .filter(Boolean);
  return pieces[0] || "";
}


function communicationRouteMeta(thread: any, connector: any): Array<{ label: string; value: string }> {
  const participant = thread?.participant || {};
  const metadata = thread?.metadata || {};
  const channel = String(thread?.channel || connectorChannel(connector)).trim().toLowerCase();
  const communicationMode = String(thread?.communicationMode || thread?.communication_mode || connectorCommunicationMode(connector)).trim().toLowerCase();
  if (communicationMode === "email") {
    const target = String(metadata?.to || metadata?.recipient || participant?.email || participant?.target || "").trim();
    const subject = String(thread?.subject || metadata?.subject || metadata?.email_subject || metadata?.topic || "").trim();
    return [
      ...(target ? [{ label: "To", value: target }] : []),
      ...(subject ? [{ label: "Subject", value: subject }] : []),
    ];
  }
  if (channel === "slack") {
    const channelID = String(participant?.channel_id || participant?.channelId || metadata?.channel_id || metadata?.channelId || participant?.target || "").trim();
    const threadTS = String(metadata?.thread_ts || metadata?.threadTs || metadata?.conversation_id || metadata?.conversationId || "").trim();
    return [
      ...(channelID ? [{ label: "Channel", value: channelID }] : []),
      ...(threadTS ? [{ label: "Thread", value: threadTS }] : []),
    ];
  }
  if (channel === "telegram") {
    const chatID = String(participant?.chat_id || participant?.chatId || participant?.target || metadata?.chat_id || metadata?.chatId || "").trim();
    const username = String(participant?.username || metadata?.username || "").trim().replace(/^@+/, "");
    return [
      ...(chatID ? [{ label: "Chat ID", value: chatID }] : []),
      ...(username ? [{ label: "Username", value: `@${username}` }] : []),
    ];
  }
  const target = String(participant?.target || metadata?.target || metadata?.recipient || "").trim();
  return target ? [{ label: "Target", value: target }] : [];
}

function communicationRouteSummary(thread: any, connector: any): string {
  return communicationRouteMeta(thread, connector)
    .map((item) => `${item.label}: ${item.value}`)
    .join(" • ");
}

function connectorChannel(connector: any): string {
  return String(connector?.channel || connector?.type || "").trim().toLowerCase();
}

function connectorCommunicationMode(connector: any): string {
  return String(connector?.communicationMode || connector?.communication_mode || "").trim().toLowerCase();
}

function resolveTimeRecipient(user: any): string {
  const candidates = [
    user?.timeRecipient,
    user?.time_recipient,
    user?.username,
    user?.email,
    user?.id,
  ];
  for (const candidate of candidates) {
    const normalized = String(candidate || "").trim();
    if (normalized) {
      return normalized;
    }
  }
  return "";
}

function threadMatchesTimeUser(thread: any, connectorId: string, userId: string, recipient: string): boolean {
  if (String(thread?.connectorId || "").trim() !== connectorId) {
    return false;
  }
  if (String(thread?.channel || "").trim().toLowerCase() !== "time") {
    return false;
  }
  const participant = thread?.participant || {};
  const metadata = thread?.metadata || {};
  const threadUserID = String(participant?.user_id || participant?.userId || metadata?.user_id || metadata?.userId || "").trim();
  if (threadUserID && userId && threadUserID === userId) {
    return true;
  }
  const threadTarget = String(
    participant?.target ||
      participant?.recipient ||
      participant?.external_user_id ||
      participant?.externalUserId ||
      participant?.username ||
      metadata?.recipient ||
      metadata?.target ||
      "",
  )
    .trim()
    .toLowerCase();
  return threadTarget !== "" && threadTarget === recipient.trim().toLowerCase();
}

export function CaseCommunicationsTab(props: CaseCommunicationsTabProps) {
  const { caseId, tenantId, currentUserId, currentUserName } = props;
  const t = useT();

  const { data: connectors = [] } = useCaseCommunicationConnectors(tenantId);
  const { data: communicationTemplates = [] } = useCommunicationTemplates(tenantId);
  const { data: tenantUsers = [] } = useUsers(tenantId);
  const { data: threads = [], isLoading: threadsLoading } = useCaseCommunications(caseId);
  const [selectedThreadId, setSelectedThreadId] = useState("");
  const [composerText, setComposerText] = useState("");
  const [composerConnectorId, setComposerConnectorId] = useState("");

  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [newThreadTitle, setNewThreadTitle] = useState("");
  const [newThreadConnectorId, setNewThreadConnectorId] = useState("");
  const [newThreadTarget, setNewThreadTarget] = useState("");
  const [newThreadParticipantName, setNewThreadParticipantName] = useState("");
  const [newThreadSubject, setNewThreadSubject] = useState("");
  const [timeConnectorId, setTimeConnectorId] = useState("");
  const [timeUserId, setTimeUserId] = useState("");
  const [composerSubject, setComposerSubject] = useState("");
  const [composerTemplateId, setComposerTemplateId] = useState("__none__");
  const [composerTemplateVars, setComposerTemplateVars] = useState("{}");

  const createThread = useCreateCaseCommunication();
  const sendMessage = useSendCaseCommunicationMessage();
  const syncThread = useSyncCaseCommunication();

  const activeThread = useCaseCommunication(caseId, selectedThreadId);
  const timeConnectors = useMemo(
    () => connectors.filter((connector: any) => connectorChannel(connector) === "time"),
    [connectors],
  );
  const newThreadConnector = useMemo(
    () => connectors.find((connector: any) => String(connector.id) === newThreadConnectorId) || null,
    [connectors, newThreadConnectorId],
  );
  const composerConnector = useMemo(
    () => connectors.find((connector: any) => String(connector.id) === String(composerConnectorId || activeThread.data?.connectorId || "")) || null,
    [connectors, composerConnectorId, activeThread.data?.connectorId],
  );
  const activeThreadConnector = useMemo(
    () => connectors.find((connector: any) => String(connector.id) === String(activeThread.data?.connectorId || "")) || null,
    [connectors, activeThread.data?.connectorId],
  );

  const connectorsByID = useMemo(() => {
    const byID = new Map<string, any>();
    connectors.forEach((connector: any) => {
      if (!connector?.id) {
        return;
      }
      byID.set(String(connector.id), connector);
    });
    return byID;
  }, [connectors]);
  const threadOptions = useMemo(
    () =>
      threads.map((thread: any) => {
        const connector = connectorsByID.get(String(thread?.connectorId || thread?.connector_id || "")) || null;
        return {
          ...thread,
          participantLabel: participantLabel(thread),
          routeSummary: communicationRouteSummary(thread, connector),
        };
      }),
    [threads, connectorsByID],
  );
  const timeUserOptions = useMemo(
    () =>
      tenantUsers.map((user: any) => ({
        ...user,
        recipientTarget: resolveTimeRecipient(user),
      })),
    [tenantUsers],
  );
  const selectedTimeUser = useMemo(
    () => timeUserOptions.find((user: any) => String(user.id) === timeUserId) || null,
    [timeUserOptions, timeUserId],
  );
  const usersByID = useMemo(() => {
    const byID = new Map<string, any>();
    tenantUsers.forEach((user: any) => {
      if (!user?.id) return;
      byID.set(String(user.id), user);
    });
    return byID;
  }, [tenantUsers]);

  useEffect(() => {
    if (!selectedThreadId && threadOptions.length > 0) {
      setSelectedThreadId(threadOptions[0].id);
    }
    if (selectedThreadId && threadOptions.length > 0 && !threadOptions.some((thread: any) => thread.id === selectedThreadId)) {
      setSelectedThreadId(threadOptions[0].id);
    }
  }, [selectedThreadId, threadOptions]);

  useEffect(() => {
    if (!newThreadConnectorId && connectors.length > 0) {
      setNewThreadConnectorId(connectors[0].id);
    }
  }, [newThreadConnectorId, connectors]);

  useEffect(() => {
    if (!timeConnectorId && timeConnectors.length > 0) {
      setTimeConnectorId(timeConnectors[0].id);
      return;
    }
    if (timeConnectorId && timeConnectors.length > 0 && !timeConnectors.some((connector: any) => connector.id === timeConnectorId)) {
      setTimeConnectorId(timeConnectors[0].id);
    }
  }, [timeConnectorId, timeConnectors]);

  useEffect(() => {
    const preferredUser =
      timeUserOptions.find((user: any) => user.recipientTarget) ||
      timeUserOptions[0];
    if (!timeUserId && preferredUser?.id) {
      setTimeUserId(String(preferredUser.id));
      return;
    }
    if (timeUserId && preferredUser?.id && !timeUserOptions.some((user: any) => String(user.id) === timeUserId)) {
      setTimeUserId(String(preferredUser.id));
    }
  }, [timeUserId, timeUserOptions]);

  useEffect(() => {
    if (!composerConnectorId && activeThread.data?.connectorId) {
      setComposerConnectorId(activeThread.data.connectorId);
    }
  }, [composerConnectorId, activeThread.data?.connectorId]);

  useEffect(() => {
    const thread = activeThread.data;
    if (!thread) {
      setComposerSubject("");
      return;
    }
    const metadata = thread?.metadata || {};
    const subject = String(
      thread?.subject ||
        metadata?.subject ||
        metadata?.email_subject ||
        metadata?.topic ||
        "",
    ).trim();
    setComposerSubject(subject);
  }, [activeThread.data?.id, activeThread.data?.subject, activeThread.data?.metadata]);

  useEffect(() => {
    setComposerTemplateId("__none__");
    setComposerTemplateVars("{}");
  }, [activeThread.data?.id]);

  const activeThreadRouteMeta = useMemo(
    () => communicationRouteMeta(activeThread.data, activeThreadConnector),
    [activeThread.data, activeThreadConnector],
  );

  const handleStartTimeChat = () => {
    const connectorID = String(timeConnectorId || "").trim();
    if (!connectorID) {
      toast.error(t("case.communications.toast.timeConnectorRequired"));
      return;
    }
    const user = selectedTimeUser;
    if (!user) {
      toast.error(t("case.communications.toast.userRequired"));
      return;
    }
    const userID = String(user.id || "").trim();
    const recipient = String(user.recipientTarget || "").trim();
    if (!recipient) {
      toast.error(t("case.communications.toast.timeRecipientRequired"));
      return;
    }
    const participantName = String(user.name || user.username || user.email || user.id || "").trim();
    const existingThread = threadOptions.find((thread: any) =>
      threadMatchesTimeUser(thread, connectorID, userID, recipient),
    );
    if (existingThread) {
      setSelectedThreadId(existingThread.id);
      setComposerConnectorId(connectorID);
      toast.success(t("case.communications.toast.timeThreadOpened"));
      return;
    }

    createThread.mutate(
      {
        caseId,
        title: `${participantName || recipient} · Time`,
        channel: "time",
        connectorId: connectorID,
        participant: {
          name: participantName,
          target: recipient,
          recipient,
          user_id: userID,
          username: String(user.username || "").trim(),
          email: String(user.email || "").trim(),
        },
        metadata: {
          channel: "time",
          recipient,
          target: recipient,
          user_id: userID,
          user_name: participantName,
          user_email: String(user.email || "").trim(),
          username: String(user.username || "").trim(),
        },
      },
      {
        onSuccess: (created: any) => {
          setSelectedThreadId(created.id);
          setComposerConnectorId(created.connectorId || connectorID);
          toast.success(t("case.communications.toast.threadCreated"));
        },
      },
    );
  };

  const handleCreateThread = () => {
    if (!newThreadConnectorId) {
      toast.error(t("case.communications.toast.connectorRequired"));
      return;
    }
    if (!newThreadTarget.trim()) {
      toast.error(t("case.communications.toast.targetRequired"));
      return;
    }

    const connectorMode = connectorCommunicationMode(newThreadConnector);
    const connectorType = connectorChannel(newThreadConnector);
    createThread.mutate(
      {
        caseId,
        title: newThreadTitle.trim(),
        connectorId: newThreadConnectorId,
        subject: connectorMode === "email" ? newThreadSubject.trim() : "",
        participant: {
          name: newThreadParticipantName.trim(),
          target: newThreadTarget.trim(),
          ...(connectorMode === "email" ? { email: newThreadTarget.trim() } : {}),
          ...(connectorType === "telegram" ? { chat_id: newThreadTarget.trim() } : {}),
          ...(connectorType === "slack" ? { channel_id: newThreadTarget.trim() } : {}),
        },
        metadata: connectorMode === "email" && newThreadSubject.trim()
          ? { subject: newThreadSubject.trim(), recipient: newThreadTarget.trim(), to: newThreadTarget.trim() }
          : {},
      },
      {
        onSuccess: (created: any) => {
          setSelectedThreadId(created.id);
          setComposerConnectorId(created.connectorId || newThreadConnectorId);
          setCreateDialogOpen(false);
          setNewThreadTitle("");
          setNewThreadTarget("");
          setNewThreadParticipantName("");
          setNewThreadSubject("");
          toast.success(t("case.communications.toast.threadCreated"));
        },
      },
    );
  };

  const handleSendMessage = () => {
    const thread = activeThread.data;
    if (!thread || !selectedThreadId) return;
    const content = composerText.trim();
    if (!content) return;

    const connectorID = (composerConnectorId || thread.connectorId || "").trim();
    if (!connectorID) {
      toast.error(t("case.communications.toast.connectorRequired"));
      return;
    }

    let parsedTemplateVars: Record<string, any> = {};
    const normalizedTemplateID = String(composerTemplateId || "").trim();
    if (normalizedTemplateID && normalizedTemplateID !== "__none__") {
      const rawTemplateVars = String(composerTemplateVars || "").trim();
      if (rawTemplateVars) {
        try {
          const parsed = JSON.parse(rawTemplateVars);
          if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
            toast.error(t("case.communications.toast.templateVarsInvalid"));
            return;
          }
          parsedTemplateVars = parsed as Record<string, any>;
        } catch {
          toast.error(t("case.communications.toast.templateVarsInvalid"));
          return;
        }
      }
    }

    sendMessage.mutate(
      {
        caseId,
        threadId: selectedThreadId,
        connectorId: connectorID,
        content,
        author: currentUserName || "",
        templateId: normalizedTemplateID === "__none__" ? "" : normalizedTemplateID,
        templateVars: parsedTemplateVars,
        subject: connectorCommunicationMode(composerConnector) === "email" ? composerSubject.trim() : "",
        metadata: {
          participant: thread.participant || {},
          ...(connectorCommunicationMode(composerConnector) === "email" && composerSubject.trim() ? { subject: composerSubject.trim() } : {}),
        },
      },
      {
        onSuccess: () => {
          setComposerText("");
          toast.success(t("case.communications.toast.messageSent"));
        },
      },
    );
  };

  const handleSync = () => {
    const thread = activeThread.data;
    if (!thread || !selectedThreadId) return;
    const connectorID = (composerConnectorId || thread.connectorId || "").trim();
    if (!connectorID) {
      toast.error(t("case.communications.toast.connectorRequired"));
      return;
    }
    syncThread.mutate(
      {
        caseId,
        threadId: selectedThreadId,
        connectorId: connectorID,
        subject: composerSubject.trim(),
      },
      {
        onSuccess: (payload: any) => {
          const createdCount = Number(payload?.created_count ?? payload?.createdCount ?? 0) || 0;
          toast.success(createdCount > 0 ? `Pulled ${createdCount} repl${createdCount === 1 ? "y" : "ies"}` : "Sync finished, no new replies");
        },
      },
    );
  };

  return (
    <div className="space-y-4" data-testid="tab-content-communications">
      <div className="flex items-center justify-between gap-3">
        <h3 className="font-bold text-sm uppercase tracking-wide text-muted-foreground">{t("case.communications.title")}</h3>
        <Dialog open={createDialogOpen} onOpenChange={setCreateDialogOpen}>
          <DialogTrigger asChild>
            <Button
              size="sm"
              className="rounded-xl gap-2"
              data-testid="button-create-communication-thread"
            >
              <Plus size={14} />
              {t("case.communications.new")}
            </Button>
          </DialogTrigger>
          <DialogContent className="rounded-2xl">
            <DialogHeader>
              <DialogTitle>{t("case.communications.new")}</DialogTitle>
            </DialogHeader>
            <div className="space-y-4">
              <div className="space-y-1">
                <Label>{t("case.communications.field.title")}</Label>
                <Input
                  value={newThreadTitle}
                  onChange={(event) => setNewThreadTitle(event.target.value)}
                  placeholder={t("case.communications.field.titlePlaceholder")}
                  data-testid="input-new-communication-title"
                />
              </div>
              <div className="space-y-1">
                <Label>{t("case.communications.field.connector")}</Label>
                <Select value={newThreadConnectorId} onValueChange={setNewThreadConnectorId}>
                  <SelectTrigger data-testid="select-new-communication-connector">
                    <SelectValue placeholder={t("case.communications.field.connectorPlaceholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    {connectors.map((connector: any) => (
                      <SelectItem key={connector.id} value={connector.id}>
                        <EllipsisText text={connector.name || connector.id} className="max-w-[220px]" />
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1">
                <Label>{connectorCommunicationMode(newThreadConnector) === "email" ? "To" : connectorChannel(newThreadConnector) === "slack" ? "Channel ID" : connectorChannel(newThreadConnector) === "telegram" ? "Chat ID" : t("case.communications.field.target")}</Label>
                <Input
                  value={newThreadTarget}
                  onChange={(event) => setNewThreadTarget(event.target.value)}
                  placeholder={connectorCommunicationMode(newThreadConnector) === "email" ? "recipient@example.com" : connectorChannel(newThreadConnector) === "slack" ? "C123456789" : connectorChannel(newThreadConnector) === "telegram" ? "Telegram chat id" : t("case.communications.field.targetPlaceholder")}
                  data-testid="input-new-communication-target"
                />
              </div>
              <div className="space-y-1">
                <Label>{t("case.communications.field.participantName")}</Label>
                <Input
                  value={newThreadParticipantName}
                  onChange={(event) => setNewThreadParticipantName(event.target.value)}
                  placeholder={t("case.communications.field.participantNamePlaceholder")}
                  data-testid="input-new-communication-participant-name"
                />
              </div>
              {connectorCommunicationMode(newThreadConnector) === "email" ? (
                <div className="space-y-1">
                  <Label>{t("case.communications.field.subject")}</Label>
                  <Input
                    value={newThreadSubject}
                    onChange={(event) => setNewThreadSubject(event.target.value)}
                    placeholder={t("case.communications.field.subjectPlaceholder")}
                    data-testid="input-new-communication-subject"
                  />
                </div>
              ) : null}
              <Button
                className="w-full rounded-xl"
                disabled={createThread.isPending}
                onClick={handleCreateThread}
                data-testid="button-submit-new-communication-thread"
              >
                {createThread.isPending ? t("common.loading") : t("case.communications.create")}
              </Button>
            </div>
          </DialogContent>
        </Dialog>
      </div>

      <Card className="rounded-2xl border-primary/30 bg-primary/5" data-testid="case-time-chat-window">
        <CardContent className="pt-5">
          <div className="flex flex-col xl:flex-row xl:items-center xl:justify-between gap-3">
            <div className="space-y-1">
              <p className="text-sm font-semibold">{t("case.communications.timeWindow")}</p>
              <p className="text-xs text-muted-foreground">{t("case.communications.timeWindowHint")}</p>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-[240px_300px_auto] gap-2 items-end">
              <div className="space-y-1">
                <Label className="text-xs">{t("case.communications.field.timeConnector")}</Label>
                <Select value={timeConnectorId} onValueChange={setTimeConnectorId}>
                  <SelectTrigger data-testid="select-time-chat-connector">
                    <SelectValue placeholder={t("case.communications.field.connectorPlaceholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    {timeConnectors.map((connector: any) => (
                      <SelectItem key={connector.id} value={connector.id}>
                        <EllipsisText text={connector.name || connector.id} className="max-w-[220px]" />
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1">
                <Label className="text-xs">{t("case.communications.field.timeUser")}</Label>
                <Select value={timeUserId} onValueChange={setTimeUserId}>
                  <SelectTrigger data-testid="select-time-chat-user">
                    <SelectValue placeholder={t("case.communications.field.timeUserPlaceholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    {timeUserOptions.map((user: any) => {
                      const label = String(user.name || user.username || user.email || user.id || "").trim();
                      const recipient = String(user.recipientTarget || "").trim();
                      const summary = recipient
                        ? `${label} · ${recipient}`
                        : `${label} · ${t("case.communications.field.timeUserNoRecipient")}`;
                      return (
                        <SelectItem key={String(user.id)} value={String(user.id)}>
                          <EllipsisText text={summary} className="max-w-[320px]" />
                        </SelectItem>
                      );
                    })}
                  </SelectContent>
                </Select>
              </div>
              <Button
                className="rounded-xl gap-2 h-10"
                onClick={handleStartTimeChat}
                disabled={createThread.isPending || timeConnectors.length === 0}
                data-testid="button-start-time-chat"
              >
                <MessageSquare size={14} />
                {createThread.isPending ? t("common.loading") : t("case.communications.startTimeChat")}
              </Button>
            </div>
          </div>
          {timeConnectors.length === 0 ? (
            <p className="text-xs text-muted-foreground mt-3">{t("case.communications.noTimeConnectors")}</p>
          ) : null}
        </CardContent>
      </Card>

      <div className="grid grid-cols-1 xl:grid-cols-[320px_minmax(0,1fr)] gap-4">
        <Card className="rounded-2xl">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm">{t("case.communications.threads")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {threadsLoading ? (
              <div className="space-y-2">
                <Skeleton className="h-16 w-full rounded-xl" />
                <Skeleton className="h-16 w-full rounded-xl" />
              </div>
            ) : threadOptions.length === 0 ? (
              <div className="rounded-xl border border-dashed p-4 text-sm text-muted-foreground" data-testid="communications-empty">
                {t("case.communications.empty")}
              </div>
            ) : (
              <div className="space-y-2 max-h-[520px] overflow-y-auto pr-1">
                {threadOptions.map((thread: any) => {
                  const active = selectedThreadId === thread.id;
                  return (
                    <button
                      type="button"
                      key={thread.id}
                      onClick={() => setSelectedThreadId(thread.id)}
                      data-testid={`communication-thread-${thread.id}`}
                      className={`w-full text-left rounded-xl border p-3 transition-all ${
                        active
                          ? "border-primary bg-primary/5 shadow-sm"
                          : "border-border hover:border-primary/40 hover:bg-muted/40"
                      }`}
                    >
                      <div className="flex items-center justify-between gap-2">
                        <EllipsisText text={thread.title} className="font-semibold text-sm max-w-[200px]" />
                        <Badge variant="outline" className="text-[10px] uppercase">
                          {thread.channel || "custom"}
                        </Badge>
                      </div>
                      {thread.participantLabel && (
                        <div className="text-xs text-muted-foreground mt-1 flex items-center gap-1">
                          <UserRound size={12} />
                          {thread.participantLabel}
                        </div>
                      )}
                      {thread.routeSummary && (
                        <div className="mt-1 text-[11px] text-muted-foreground line-clamp-2">{thread.routeSummary}</div>
                      )}
                      {thread.lastSyncedAt ? (
                        <div className="mt-2 flex flex-wrap gap-2 text-[10px] text-muted-foreground">
                          <span>Last synced: {formatMessageTime(thread.lastSyncedAt)}</span>
                          <span>Last pull: {thread.lastSyncCreatedCount ?? 0}</span>
                        </div>
                      ) : null}
                      {thread.lastMessagePreview && (
                        <div className="text-xs text-muted-foreground mt-2 line-clamp-2">{thread.lastMessagePreview}</div>
                      )}
                    </button>
                  );
                })}
              </div>
            )}
          </CardContent>
        </Card>

        <Card className="rounded-2xl">
          {!selectedThreadId ? (
            <CardContent className="py-16 text-center text-muted-foreground">
              <MessageSquare size={28} className="mx-auto mb-3 opacity-40" />
              <p>{t("case.communications.selectThread")}</p>
            </CardContent>
          ) : activeThread.isLoading ? (
            <CardContent className="space-y-3 py-4">
              <Skeleton className="h-8 w-2/5 rounded-xl" />
              <Skeleton className="h-24 w-full rounded-xl" />
              <Skeleton className="h-24 w-full rounded-xl" />
            </CardContent>
          ) : !activeThread.data ? (
            <CardContent className="py-16 text-center text-muted-foreground">
              {t("common.noData")}
            </CardContent>
          ) : (
            <>
              <CardHeader className="pb-3">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <CardTitle className="text-base">
                      <EllipsisText text={activeThread.data.title} className="max-w-[360px]" />
                    </CardTitle>
                    <div className="text-xs text-muted-foreground mt-1 flex flex-wrap gap-2">
                      <span>{t("case.communications.lastActivity")}: {formatMessageTime(activeThread.data.lastMessageAt)}</span>
                      {participantLabel(activeThread.data) && <span>{t("case.communications.participant")}: {participantLabel(activeThread.data)}</span>}
                      {activeThread.data.lastSyncedAt ? <span>Last synced: {formatMessageTime(activeThread.data.lastSyncedAt)}</span> : null}
                      {activeThread.data.lastSyncedAt ? <span>Last pull: {activeThread.data.lastSyncCreatedCount ?? 0}</span> : null}
                    </div>
                    {activeThreadRouteMeta.length > 0 ? (
                      <div className="mt-2 flex flex-wrap gap-2 text-[11px] text-muted-foreground">
                        {activeThreadRouteMeta.map((item) => (
                          <span key={`${item.label}:${item.value}`} className="rounded-full border border-border/60 bg-muted/20 px-2 py-1">
                            {item.label}: {item.value}
                          </span>
                        ))}
                      </div>
                    ) : null}
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    className="rounded-xl gap-2"
                    onClick={handleSync}
                    disabled={syncThread.isPending}
                    data-testid="button-sync-communication-thread"
                  >
                    <RefreshCcw size={14} className={syncThread.isPending ? "animate-spin" : ""} />
                    Sync replies
                  </Button>
                </div>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="max-h-[360px] overflow-y-auto rounded-xl border bg-muted/20 p-3 space-y-3" data-testid="communication-messages-list">
                  {activeThread.data.messages.length === 0 ? (
                    <p className="text-sm text-muted-foreground">{t("case.communications.noMessages")}</p>
                  ) : (
                    activeThread.data.messages.map((message: any) => {
                      const isMine =
                        message.direction === "outbound" ||
                        (message.authorId && currentUserId && message.authorId === currentUserId);
                      const authorUser = message.authorId ? usersByID.get(String(message.authorId)) : null;
                      const authorLabel =
                        authorUser?.name ||
                        message.authorName ||
                        message.authorId ||
                        (isMine ? t("layout.ai.you") : t("case.communications.external"));
                      return (
                        <div
                          key={message.id}
                          className={`flex items-end gap-2 ${isMine ? "justify-end" : "justify-start"}`}
                          data-testid={`communication-message-${message.id}`}
                        >
                          {!isMine ? (
                            <UserAvatar
                              name={authorLabel}
                              avatar={authorUser?.avatar}
                              fallback={message.authorId}
                              className="h-8 w-8 border border-border/60"
                              fallbackClassName="text-[10px] font-bold"
                            />
                          ) : null}
                          <div
                            className={`max-w-[82%] rounded-2xl px-3 py-2 text-sm shadow-sm transition-colors ${
                              isMine
                                ? "bg-primary text-primary-foreground rounded-tr-md"
                                : "bg-white border rounded-tl-md"
                            }`}
                          >
                            <div className="text-[10px] opacity-80 mb-1">
                              {authorLabel}
                            </div>
                            <div className="whitespace-pre-wrap">{message.content}</div>
                            <div className="text-[10px] opacity-70 mt-1">{formatMessageTime(message.timestamp)}</div>
                          </div>
                          {isMine ? (
                            <UserAvatar
                              name={authorLabel || currentUserName}
                              avatar={authorUser?.avatar}
                              fallback={currentUserName || currentUserId}
                              className="h-8 w-8 border border-border/60"
                              fallbackClassName="bg-primary text-primary-foreground text-[10px] font-bold"
                            />
                          ) : null}
                        </div>
                      );
                    })
                  )}
                </div>

                <div className="space-y-2">
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-2 items-end">
                    <div className="space-y-1">
                      <Label className="text-xs">{t("case.communications.field.connector")}</Label>
                      <Select value={composerConnectorId} onValueChange={setComposerConnectorId}>
                        <SelectTrigger data-testid="select-communication-composer-connector">
                          <SelectValue placeholder={t("case.communications.field.connectorPlaceholder")} />
                        </SelectTrigger>
                        <SelectContent>
                          {connectors.map((connector: any) => (
                            <SelectItem key={connector.id} value={connector.id}>
                              <EllipsisText text={connector.name || connector.id} className="max-w-[220px]" />
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                    {connectorCommunicationMode(composerConnector) === "email" ? (
                      <div className="space-y-1">
                        <Label className="text-xs">{t("case.communications.field.subject")}</Label>
                        <Input
                          value={composerSubject}
                          onChange={(event) => setComposerSubject(event.target.value)}
                          placeholder={t("case.communications.field.subjectPlaceholder")}
                          data-testid="input-communication-subject"
                        />
                      </div>
                    ) : null}
                  </div>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-2 items-end">
                    <div className="space-y-1">
                      <Label className="text-xs">{t("case.communications.field.template")}</Label>
                      <Select value={composerTemplateId} onValueChange={setComposerTemplateId}>
                        <SelectTrigger data-testid="select-communication-template">
                          <SelectValue placeholder={t("case.communications.field.templatePlaceholder")} />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="__none__">{t("case.communications.field.templateNone")}</SelectItem>
                          {communicationTemplates.map((template: any) => (
                            <SelectItem key={template.id} value={template.id}>
                              <EllipsisText text={template.name || template.id} className="max-w-[220px]" />
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                    {composerTemplateId !== "__none__" ? (
                      <div className="space-y-1">
                        <Label className="text-xs">{t("case.communications.field.templateVars")}</Label>
                        <Input
                          value={composerTemplateVars}
                          onChange={(event) => setComposerTemplateVars(event.target.value)}
                          placeholder={t("case.communications.field.templateVarsPlaceholder")}
                          data-testid="input-communication-template-vars"
                        />
                      </div>
                    ) : null}
                  </div>
                  <div className="grid grid-cols-1 md:grid-cols-[minmax(0,1fr)_auto] gap-2 items-end">
                    <div className="space-y-1">
                      <Label className="text-xs">{t("case.communications.field.message")}</Label>
                      <Textarea
                        value={composerText}
                        onChange={(event) => setComposerText(event.target.value)}
                        placeholder={t("case.communications.field.messagePlaceholder")}
                        className="min-h-[84px] rounded-xl"
                        data-testid="textarea-communication-message"
                      />
                    </div>
                    <Button
                      className="rounded-xl gap-2 h-10"
                      onClick={handleSendMessage}
                      disabled={sendMessage.isPending || !composerText.trim()}
                      data-testid="button-send-communication-message"
                    >
                      <Send size={14} />
                      {sendMessage.isPending ? t("common.loading") : t("case.communications.send")}
                    </Button>
                  </div>
                </div>
              </CardContent>
            </>
          )}
        </Card>
      </div>
    </div>
  );
}
