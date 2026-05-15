import { AppLayout } from "@/components/layout";
import {
  useAppState,
  useForumThread,
  useCreateForumPost,
  useCreateForumPostWithAttachments,
  useUser,
  useUsers,
  useCases,
  useCaseCommunicationConnectors,
  useProxyForumSend,
  useProxyForumSync,
} from "@/lib/api";
import { formatDistanceToNow } from "date-fns";
import {
  MessageSquare,
  Send,
  ChevronLeft,
  ChevronDown,
  ChevronUp,
  FolderKanban,
  Paperclip,
  X,
  Loader2,
  RefreshCcw,
  Download,
  FileImage,
  FileText,
  Link2,
  AtSign,
} from "lucide-react";
import { Link, useParams } from "wouter";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Input } from "@/components/ui/input";
import { useEffect, useLayoutEffect, useMemo, useRef, useState, type ChangeEvent } from "react";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { useT } from "@/lib/i18n";
import { EllipsisText } from "@/components/ui/ellipsis-text";
import { UserAvatar } from "@/components/user-avatar";
import { withTenantPath } from "@/lib/tenant-url";
import { useMinimumLoading } from "@/lib/use-minimum-loading";
import { toast } from "sonner";

const PAGE_SHELL_CLASS =
  "mx-auto flex h-[calc(100vh-112px)] w-full min-h-[620px] flex-col overflow-hidden rounded-[12px] border border-[#1d1e29] bg-[#0b0c10] shadow-[0_20px_48px_rgba(0,0,0,0.35)]";
const HEADER_BLOCK_CLASS = "border-b border-[#1d1e29] bg-[#13141c] px-4 py-2";
const CASE_PANEL_CLASS = "border-b border-[#1d1e29] bg-[#13141c] px-4 py-2";
const CHAT_STREAM_CLASS = "min-h-0 flex-1 overflow-y-auto bg-[#050813]";
const COMPOSER_CLASS = "border-t border-[#1d1e29] bg-[#10141f] px-5 py-2.5";
const OUTLINE_BUTTON_CLASS = "border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]";
const IMAGE_EXTENSIONS = new Set(["png", "jpg", "jpeg", "gif", "webp", "bmp", "svg", "avif"]);
const CUSTOM_PROXY_PROFILE_ID = "__custom__";
type ConnectorSendDraft = {
  name: string;
  connectorId: string;
  chatId: string;
  externalUserId: string;
  username: string;
};

function createEmptyConnectorSendDraft(defaultConnectorId = ""): ConnectorSendDraft {
  return {
    name: "",
    connectorId: defaultConnectorId,
    chatId: "",
    externalUserId: "",
    username: "",
  };
}

function connectorChannel(connector: any): string {
  return String(connector?.channel || connector?.data?.channel || connector?.type || connector?.data?.type || "").trim().toLowerCase();
}

function connectorCommunicationMode(connector: any): string {
  return String(connector?.communicationMode || connector?.communication_mode || connector?.data?.communication_mode || "").trim().toLowerCase();
}

function pickFirstString(...values: unknown[]): string {
  for (const value of values) {
    const normalized = String(value || "").trim();
    if (normalized) {
      return normalized;
    }
  }
  return "";
}

function isSameConnectorSendDraft(left: ConnectorSendDraft, right: ConnectorSendDraft): boolean {
  return (
    left.name === right.name &&
    left.connectorId === right.connectorId &&
    left.chatId === right.chatId &&
    left.externalUserId === right.externalUserId &&
    left.username === right.username
  );
}

function buildConnectorRoutePayload(draft: ConnectorSendDraft, connector: any): { metadata: Record<string, any>; bindingKey: string; error: string } {
  const channel = connectorChannel(connector);
  const communicationMode = connectorCommunicationMode(connector);
  const chatID = String(draft.chatId || "").trim();
  const externalUserID = String(draft.externalUserId || "").trim();
  const username = String(draft.username || "").trim().replace(/^@+/, "");
  const metadata: Record<string, any> = {};

  if (draft.name.trim()) {
    metadata.target_name = draft.name.trim();
  }

  let bindingKey = chatID || externalUserID || username;
  if (communicationMode === "email") {
    if (!chatID) {
      return { metadata: {}, bindingKey: "", error: "Recipient is required for email connectors" };
    }
    metadata.to = chatID;
    metadata.recipient = chatID;
    metadata.target = chatID;
    if (externalUserID) {
      metadata.cc = externalUserID;
    }
    if (username) {
      metadata.subject = username;
    }
    metadata.participant = {
      email: chatID,
      ...(externalUserID ? { cc: externalUserID } : {}),
    };
    bindingKey = chatID;
  } else if (channel === "slack") {
    if (!chatID) {
      return { metadata: {}, bindingKey: "", error: "channel_id is required for Slack" };
    }
    metadata.channel_id = chatID;
    metadata.channelId = chatID;
    if (externalUserID) {
      metadata.thread_ts = externalUserID;
      metadata.threadTs = externalUserID;
    }
    metadata.participant = {
      channel_id: chatID,
      ...(externalUserID ? { thread_ts: externalUserID } : {}),
    };
    bindingKey = [chatID, externalUserID].filter(Boolean).join(":") || chatID;
  } else {
    if (channel === "telegram" && !chatID) {
      return { metadata: {}, bindingKey: "", error: "chat_id is required for Telegram" };
    }
    if (chatID) {
      metadata.chat_id = chatID;
      metadata.chatId = chatID;
    }
    if (externalUserID) {
      metadata.external_user_id = externalUserID;
      metadata.externalUserId = externalUserID;
    }
    if (username) {
      metadata.username = username;
    }
    if (chatID || externalUserID || username) {
      metadata.participant = {
        ...(chatID ? { chat_id: chatID } : {}),
        ...(externalUserID ? { external_user_id: externalUserID } : {}),
        ...(username ? { username } : {}),
      };
    }
  }

  return { metadata, bindingKey, error: "" };
}

function connectorDraftFromRoute(connectorId: string, metadata: any): ConnectorSendDraft {
  const routeMetadata = metadata && typeof metadata === "object" ? metadata : {};
  const participant = routeMetadata?.participant && typeof routeMetadata.participant === "object" ? routeMetadata.participant : {};
  const mode = String(routeMetadata?.communication_mode || routeMetadata?.communicationMode || "").trim().toLowerCase();
  const channel = String(routeMetadata?.channel || routeMetadata?.channelId || "").trim().toLowerCase();
  return {
    name: pickFirstString(routeMetadata?.target_name, routeMetadata?.targetName, participant?.name),
    connectorId: connectorId,
    chatId:
      mode === "email"
        ? pickFirstString(routeMetadata?.to, routeMetadata?.recipient, routeMetadata?.target, participant?.email, participant?.target)
        : channel === "slack"
          ? pickFirstString(routeMetadata?.channel_id, routeMetadata?.channelId, participant?.channel_id, participant?.channelId, routeMetadata?.target)
          : pickFirstString(routeMetadata?.chat_id, routeMetadata?.chatId, participant?.chat_id, participant?.chatId, routeMetadata?.target),
    externalUserId:
      mode === "email"
        ? pickFirstString(routeMetadata?.cc, participant?.cc)
        : channel === "slack"
          ? pickFirstString(routeMetadata?.thread_ts, routeMetadata?.threadTs, routeMetadata?.conversation_id, routeMetadata?.conversationId, participant?.thread_ts, participant?.threadTs)
          : pickFirstString(routeMetadata?.external_user_id, routeMetadata?.externalUserId, participant?.external_user_id, participant?.externalUserId),
    username:
      mode === "email"
        ? pickFirstString(routeMetadata?.subject, routeMetadata?.topic)
        : pickFirstString(routeMetadata?.username, participant?.username),
  };
}

function connectorRouteSummary(connector: any, metadata: any): string {
  const routeMetadata = metadata && typeof metadata === "object" ? metadata : {};
  const participant = routeMetadata?.participant && typeof routeMetadata.participant === "object" ? routeMetadata.participant : {};
  const mode = connectorCommunicationMode(connector) || String(routeMetadata?.communication_mode || routeMetadata?.communicationMode || "").trim().toLowerCase();
  const channel = connectorChannel(connector) || String(routeMetadata?.channel || "").trim().toLowerCase();
  if (mode === "email") {
    const target = pickFirstString(routeMetadata?.to, routeMetadata?.recipient, routeMetadata?.target, participant?.email, participant?.target);
    const subject = pickFirstString(routeMetadata?.subject, routeMetadata?.topic);
    const cc = pickFirstString(routeMetadata?.cc, participant?.cc);
    return [target ? `To ${target}` : "", subject ? `Subject ${subject}` : "", cc ? `CC ${cc}` : ""].filter(Boolean).join(" • ") || "Email route";
  }
  if (channel === "slack") {
    const channelID = pickFirstString(routeMetadata?.channel_id, routeMetadata?.channelId, participant?.channel_id, participant?.channelId, routeMetadata?.target);
    const threadTS = pickFirstString(routeMetadata?.thread_ts, routeMetadata?.threadTs, routeMetadata?.conversation_id, routeMetadata?.conversationId, participant?.thread_ts, participant?.threadTs);
    return [channelID ? `Channel ${channelID}` : "", threadTS ? `Thread ${threadTS}` : ""].filter(Boolean).join(" • ") || "Slack route";
  }
  if (channel === "telegram") {
    const chatID = pickFirstString(routeMetadata?.chat_id, routeMetadata?.chatId, participant?.chat_id, participant?.chatId, routeMetadata?.target);
    const username = pickFirstString(routeMetadata?.username, participant?.username);
    return [chatID ? `Chat ${chatID}` : "", username ? `@${username.replace(/^@+/, "")}` : ""].filter(Boolean).join(" • ") || "Telegram route";
  }
  return pickFirstString(routeMetadata?.target_name, participant?.name, routeMetadata?.target, routeMetadata?.recipient, routeMetadata?.subject, "Custom route");
}

function formatRouteSyncTime(raw: unknown): string {
  const normalized = String(raw || "").trim();
  if (!normalized) {
    return "Never";
  }
  const parsed = new Date(normalized);
  if (Number.isNaN(parsed.getTime())) {
    return normalized;
  }
  return parsed.toLocaleString();
}

function formatBytes(size: unknown): string {
  const value = Number(size || 0);
  if (!Number.isFinite(value) || value <= 0) {
    return "";
  }
  if (value < 1024) {
    return `${value} B`;
  }
  const kb = value / 1024;
  if (kb < 1024) {
    return `${kb.toFixed(kb >= 100 ? 0 : 1)} KB`;
  }
  const mb = kb / 1024;
  return `${mb.toFixed(mb >= 100 ? 0 : 1)} MB`;
}

function hasImageExtension(fileName: unknown): boolean {
  const normalizedName = String(fileName || "").trim().toLowerCase();
  if (!normalizedName.includes(".")) {
    return false;
  }
  const extension = normalizedName.split(".").pop() || "";
  return IMAGE_EXTENSIONS.has(extension);
}

function isImageAttachment(attachment: any): boolean {
  const contentType = String(attachment?.contentType || attachment?.content_type || "").toLowerCase();
  if (contentType.startsWith("image/")) {
    return true;
  }
  return hasImageExtension(attachment?.fileName || attachment?.filename || attachment?.name);
}

function resolveAttachmentLink(attachment: any): string {
  return String(attachment?.url || attachment?.previewUrl || attachment?.downloadUrl || attachment?.download_url || "").trim();
}

function severityClass(severity: string): string {
  const normalized = String(severity || "").trim().toLowerCase();
  if (normalized === "critical") {
    return "border-[#7f1d1d] bg-[rgba(239,68,68,0.12)] text-[#f87171]";
  }
  if (normalized === "high") {
    return "border-[#7c2d12] bg-[rgba(249,115,22,0.12)] text-[#fb923c]";
  }
  if (normalized === "medium") {
    return "border-[#854d0e] bg-[rgba(234,179,8,0.12)] text-[#facc15]";
  }
  return "border-[#1e3a8a] bg-[rgba(59,130,246,0.12)] text-[#93c5fd]";
}

function shortThreadCode(id: string): string {
  const raw = String(id || "").trim().toUpperCase();
  if (!raw) {
    return "THR-0000";
  }
  if (raw.length <= 10) {
    return raw;
  }
  return `THR-${raw.replace(/[^A-Z0-9]/g, "").slice(0, 6)}`;
}

function formatMessageDayLabel(rawDate: unknown): string {
  const value = new Date(String(rawDate || ""));
  if (Number.isNaN(value.getTime())) {
    return "";
  }
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const target = new Date(value.getFullYear(), value.getMonth(), value.getDate());
  const oneDay = 24 * 60 * 60 * 1000;
  const diff = Math.round((today.getTime() - target.getTime()) / oneDay);
  if (diff === 0) {
    return "Today";
  }
  if (diff === 1) {
    return "Yesterday";
  }
  return value.toLocaleDateString();
}

function isSameCalendarDay(leftRaw: unknown, rightRaw: unknown): boolean {
  const left = new Date(String(leftRaw || ""));
  const right = new Date(String(rightRaw || ""));
  if (Number.isNaN(left.getTime()) || Number.isNaN(right.getTime())) {
    return false;
  }
  return (
    left.getFullYear() === right.getFullYear() &&
    left.getMonth() === right.getMonth() &&
    left.getDate() === right.getDate()
  );
}

export default function ForumThreadPage() {
  const t = useT();
  const params = useParams<{ id: string }>();
  const { currentTenantId, currentTenantSlug, currentUserId } = useAppState();
  const { data: currentUser, isLoading: currentUserLoading } = useUser(currentUserId);
  const currentViewer = currentUser || { id: currentUserId || "unknown", name: "", username: "", avatar: "" };
  const { data: tenantUsers = [] } = useUsers(currentTenantId);
  const { data: cases = [] } = useCases(currentTenantId);
  const { data: communicationConnectors = [] } = useCaseCommunicationConnectors(currentTenantId);
  const createForumPost = useCreateForumPost();
  const createForumPostWithAttachments = useCreateForumPostWithAttachments();
  const proxyForumSend = useProxyForumSend();
  const proxyForumSync = useProxyForumSync();
  const [replyContent, setReplyContent] = useState("");
  const [pendingFiles, setPendingFiles] = useState<File[]>([]);
  const [connectorDialogOpen, setConnectorDialogOpen] = useState(false);
  const [connectorDraft, setConnectorDraft] = useState<ConnectorSendDraft>(createEmptyConnectorSendDraft());
  const [selectedProxyProfileId, setSelectedProxyProfileId] = useState(CUSTOM_PROXY_PROFILE_ID);
  const [lastProxySyncSummary, setLastProxySyncSummary] = useState<{ profileId: string; createdCount: number; syncedAt: string } | null>(null);
  const [threadDetailsExpanded, setThreadDetailsExpanded] = useState(false);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const messagesScrollRef = useRef<HTMLDivElement | null>(null);

  const threadId = params?.id || "";
  const { data: thread, isLoading: threadLoading } = useForumThread(threadId);
  const showForumThreadLoadingSkeleton = useMinimumLoading(currentUserLoading || threadLoading);
  const linkedCase = thread ? cases.find((item: any) => String(item.id) === String(thread.caseId)) : null;

  const usersByID = useMemo(() => {
    const byID = new Map<string, any>();
    tenantUsers.forEach((user: any) => {
      if (!user?.id) {
        return;
      }
      byID.set(String(user.id), user);
    });
    return byID;
  }, [tenantUsers]);

  const linkedCaseOwner = linkedCase?.owner ? usersByID.get(String(linkedCase.owner)) : null;
  const posts = useMemo(() => (Array.isArray(thread?.posts) ? thread.posts : []), [thread?.posts]);
  const threadPostsCount = Number(thread?.postsCount ?? posts.length) || posts.length;
  const threadParticipantsCount = useMemo(() => {
    const participants = new Set<string>();
    posts.forEach((post: any) => {
      const participantID = String(post?.authorId || post?.authorName || "").trim();
      if (participantID) {
        participants.add(participantID);
      }
    });
    return Math.max(1, participants.size || (currentViewer ? 1 : 0));
  }, [posts, currentViewer]);
  const outboundConnectors = useMemo(
    () =>
      communicationConnectors.filter((item: any) => {
        const direction = String(item?.direction || item?.data?.direction || "outbound").toLowerCase();
        return direction !== "inbound";
      }),
    [communicationConnectors],
  );
  const threadProxyProfiles = useMemo(
    () => (Array.isArray(thread?.proxyProfiles) ? thread.proxyProfiles : []),
    [thread?.proxyProfiles],
  );
  const selectedProxyProfile = useMemo(
    () => threadProxyProfiles.find((item: any) => String(item?.id) === String(selectedProxyProfileId || "")) || null,
    [threadProxyProfiles, selectedProxyProfileId],
  );
  const activeConnector = useMemo(() => {
    const activeConnectorId = selectedProxyProfile?.connectorId || connectorDraft.connectorId;
    return outboundConnectors.find((item: any) => String(item?.id) === String(activeConnectorId || "")) || null;
  }, [outboundConnectors, selectedProxyProfile?.connectorId, connectorDraft.connectorId]);
  const activeConnectorChannel = connectorChannel(activeConnector);
  const activeConnectorMode = connectorCommunicationMode(activeConnector);
  const defaultConnectorID = String(outboundConnectors[0]?.id || "");
  const outboundConnectorIDsSignature = useMemo(
    () => outboundConnectors.map((item: any) => String(item?.id || "").trim()).filter(Boolean).join("|"),
    [outboundConnectors],
  );
  const pendingImagePreviewUrls = useMemo(
    () => pendingFiles.map((file) => (file.type.startsWith("image/") || hasImageExtension(file.name) ? URL.createObjectURL(file) : "")),
    [pendingFiles],
  );
  const connectorDraftStorageKey = useMemo(
    () => (threadId ? `incidenthub:forum-thread:${threadId}:connector-draft` : ""),
    [threadId],
  );

  useLayoutEffect(() => {
    const node = messagesScrollRef.current;
    if (!node) {
      return;
    }
    node.scrollTop = node.scrollHeight;
  }, [threadId]);

  useEffect(() => {
    const node = messagesScrollRef.current;
    if (!node) {
      return;
    }
    node.scrollTop = node.scrollHeight;
  }, [posts.length]);

  useEffect(
    () => () => {
      pendingImagePreviewUrls.forEach((url) => {
        if (!url) {
          return;
        }
        URL.revokeObjectURL(url);
      });
    },
    [pendingImagePreviewUrls],
  );

  useEffect(() => {
    if (connectorDraft.connectorId || !defaultConnectorID) {
      return;
    }
    setConnectorDraft((prev) => ({ ...prev, connectorId: defaultConnectorID }));
  }, [connectorDraft.connectorId, defaultConnectorID]);

  useEffect(() => {
    if (!connectorDraftStorageKey) {
      return;
    }
    const savedRaw = window.localStorage.getItem(connectorDraftStorageKey);
    if (!savedRaw) {
      setConnectorDraft((prev) => {
        const next = createEmptyConnectorSendDraft(defaultConnectorID);
        if (
          prev.name === next.name &&
          prev.connectorId === next.connectorId &&
          prev.chatId === next.chatId &&
          prev.externalUserId === next.externalUserId &&
          prev.username === next.username
        ) {
          return prev;
        }
        return next;
      });
      return;
    }
    try {
      const saved = JSON.parse(savedRaw) as Partial<ConnectorSendDraft>;
      const savedConnectorID = String(saved.connectorId || "").trim();
      const connectorRegistry = `|${outboundConnectorIDsSignature}|`;
      const connectorID =
        savedConnectorID && connectorRegistry.includes(`|${savedConnectorID}|`)
          ? savedConnectorID
          : defaultConnectorID;
      setConnectorDraft((prev) => {
        const next = {
          name: String(saved.name || "").trim(),
          connectorId: connectorID,
          chatId: String(saved.chatId || "").trim(),
          externalUserId: String(saved.externalUserId || "").trim(),
          username: String(saved.username || "").trim(),
        };
        if (
          prev.name === next.name &&
          prev.connectorId === next.connectorId &&
          prev.chatId === next.chatId &&
          prev.externalUserId === next.externalUserId &&
          prev.username === next.username
        ) {
          return prev;
        }
        return next;
      });
    } catch {
      setConnectorDraft((prev) => {
        const next = createEmptyConnectorSendDraft(defaultConnectorID);
        if (
          prev.name === next.name &&
          prev.connectorId === next.connectorId &&
          prev.chatId === next.chatId &&
          prev.externalUserId === next.externalUserId &&
          prev.username === next.username
        ) {
          return prev;
        }
        return next;
      });
    }
  }, [connectorDraftStorageKey, outboundConnectorIDsSignature, defaultConnectorID]);

  useEffect(() => {
    if (!connectorDraftStorageKey) {
      return;
    }
    window.localStorage.setItem(connectorDraftStorageKey, JSON.stringify(connectorDraft));
  }, [connectorDraftStorageKey, connectorDraft]);

  useEffect(() => {
    if (threadProxyProfiles.length === 0) {
      return;
    }
    if (selectedProxyProfileId === CUSTOM_PROXY_PROFILE_ID) {
      return;
    }
    if (!threadProxyProfiles.some((profile: any) => String(profile?.id) === String(selectedProxyProfileId))) {
      setSelectedProxyProfileId(String(threadProxyProfiles[0]?.id || CUSTOM_PROXY_PROFILE_ID));
    }
  }, [threadProxyProfiles, selectedProxyProfileId]);

  useEffect(() => {
    if (!selectedProxyProfile) {
      return;
    }
    const nextDraft = connectorDraftFromRoute(String(selectedProxyProfile.connectorId || ""), selectedProxyProfile.metadata || {});
    setConnectorDraft((prev) => (isSameConnectorSendDraft(prev, nextDraft) ? prev : nextDraft));
  }, [selectedProxyProfile?.id, selectedProxyProfile?.updatedAt, selectedProxyProfile?.connectorId, selectedProxyProfile?.metadata]);

  const openConnectorDialog = () => {
    if (outboundConnectors.length === 0) {
      toast.error("No outbound connectors configured");
      return;
    }
    if (!connectorDraft.connectorId && defaultConnectorID) {
      setConnectorDraft((prev) => ({ ...prev, connectorId: defaultConnectorID }));
    }
    if (threadProxyProfiles.length > 0 && !threadProxyProfiles.some((profile: any) => String(profile?.id) === String(selectedProxyProfileId || ""))) {
      setSelectedProxyProfileId(String(threadProxyProfiles[0]?.id || CUSTOM_PROXY_PROFILE_ID));
    }
    setConnectorDialogOpen(true);
  };

  const closeConnectorDialog = () => {
    setConnectorDialogOpen(false);
  };

  const handleSelectFiles = (event: ChangeEvent<HTMLInputElement>) => {
    const nextFiles = Array.from(event.target.files || []);
    if (nextFiles.length === 0) {
      return;
    }
    setPendingFiles((prev) => [...prev, ...nextFiles]);
    event.target.value = "";
  };

  const removePendingFile = (index: number) => {
    setPendingFiles((prev) => prev.filter((_, idx) => idx !== index));
  };

  const resetComposer = () => {
    setReplyContent("");
    setPendingFiles([]);
  };

  if (showForumThreadLoadingSkeleton || !thread) {
    return (
      <AppLayout>
        <div className={PAGE_SHELL_CLASS}>
          <div className={HEADER_BLOCK_CLASS}>
            <div className="flex items-center justify-between gap-4">
              <div className="flex items-center gap-3">
                <Skeleton className="h-5 w-5 rounded-full" />
                <div className="space-y-1.5">
                  <Skeleton className="h-5 w-56" />
                </div>
              </div>
              <Skeleton className="h-4 w-40" />
            </div>
          </div>
          <div className="border-b border-[#1d1e29] bg-[#13141c] px-4 py-1.5">
            <Skeleton className="h-8 w-32 rounded-lg" />
          </div>
          <div className={`${CHAT_STREAM_CLASS} px-8 py-6`}>
            <div className="mx-auto w-full max-w-[1024px] space-y-5">
              <Skeleton className="h-[128px] w-[72%] rounded-[16px]" />
              <Skeleton className="ml-auto h-[112px] w-[67%] rounded-[16px]" />
              <Skeleton className="h-[190px] w-[34%] rounded-[16px]" />
              <Skeleton className="h-[104px] w-[68%] rounded-[16px]" />
              <Skeleton className="ml-auto h-[98px] w-[66%] rounded-[16px]" />
            </div>
          </div>
          <div className={COMPOSER_CLASS}>
            <Skeleton className="h-24 w-full rounded-[8px]" />
          </div>
        </div>
      </AppLayout>
    );
  }

  const handleSend = () => {
    const trimmedContent = replyContent.trim();
    const hasFiles = pendingFiles.length > 0;
    if (!trimmedContent && !hasFiles) {
      return;
    }

    if (hasFiles) {
      createForumPostWithAttachments.mutate(
        {
          threadId: thread.id,
          authorId: currentViewer.id,
          content: trimmedContent,
          files: pendingFiles,
        },
        {
          onSuccess: () => {
            resetComposer();
          },
          onError: (error: any) => {
            toast.error(error?.message || "Failed to send post with attachments");
          },
        },
      );
      return;
    }

    createForumPost.mutate(
      {
        threadId: thread.id,
        authorId: currentViewer.id,
        content: trimmedContent,
        timestamp: new Date().toISOString(),
      },
      {
        onSuccess: () => {
          resetComposer();
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to send post");
        },
      },
    );
  };

  const handleProxySend = () => {
    openConnectorDialog();
  };

  const handleConnectorSend = () => {
    const content = replyContent.trim();
    if (!content) {
      toast.error("Write message first");
      return;
    }
    if (pendingFiles.length > 0) {
      toast.error("Attachments are only available for local messages");
      return;
    }

    const activeProfile = selectedProxyProfileId !== CUSTOM_PROXY_PROFILE_ID ? selectedProxyProfile : null;
    const connectorID = String(activeProfile?.connectorId || connectorDraft.connectorId || "").trim();
    if (!connectorID) {
      toast.error("Choose connector");
      return;
    }

    const routePayload = buildConnectorRoutePayload(connectorDraft, activeConnector);
    if (routePayload.error) {
      toast.error(routePayload.error);
      return;
    }

    proxyForumSend.mutate(
      {
        threadId: thread.id,
        connectorId: connectorID,
        profileId: activeProfile?.id || "",
        bindingKey: routePayload.bindingKey || activeProfile?.bindingKey || "",
        content,
        author: currentViewer.name || currentViewer.username || currentViewer.id,
        metadata: routePayload.metadata,
      },
      {
        onSuccess: (payload: any) => {
          const profileId = String(payload?.profile_id || "").trim();
          if (profileId) {
            setSelectedProxyProfileId(profileId);
          }
          resetComposer();
          setConnectorDialogOpen(false);
          toast.success("Message sent via connector");
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to send message via connector");
        },
      },
    );
  };

  const handleConnectorSync = () => {
    const activeProfile = selectedProxyProfileId !== CUSTOM_PROXY_PROFILE_ID ? selectedProxyProfile : null;
    if (!activeProfile) {
      toast.error("Select a linked external route before syncing replies");
      return;
    }
    proxyForumSync.mutate(
      {
        threadId: thread.id,
        connectorId: activeProfile.connectorId,
        profileId: activeProfile.id,
        bindingKey: activeProfile.bindingKey,
      },
      {
        onSuccess: (payload: any) => {
          const createdCount = Number(payload?.created_count ?? payload?.createdCount ?? 0) || 0;
          const syncedAt = String(payload?.synced_at || payload?.syncedAt || new Date().toISOString()).trim();
          setLastProxySyncSummary({
            profileId: activeProfile.id,
            createdCount,
            syncedAt,
          });
          toast.success(createdCount > 0 ? `Pulled ${createdCount} repl${createdCount === 1 ? "y" : "ies"}` : "Sync finished, no new replies");
        },
        onError: (error: any) => {
          toast.error(error?.message || "Failed to sync replies");
        },
      },
    );
  };

  const pendingSend = createForumPost.isPending || createForumPostWithAttachments.isPending;
  const pendingProxySend = proxyForumSend.isPending;
  const pendingProxySync = proxyForumSync.isPending;
  const pendingProxy = pendingProxySend || pendingProxySync;
  const canSendLocal = !pendingSend && !pendingProxy && (replyContent.trim().length > 0 || pendingFiles.length > 0);
  const canSendViaConnector =
    !pendingSend &&
    !pendingProxy &&
    outboundConnectors.length > 0;
  const canSyncViaConnector = !pendingSend && !pendingProxy && !!selectedProxyProfile && selectedProxyProfileId !== CUSTOM_PROXY_PROFILE_ID;

  const activeRoutePayload = buildConnectorRoutePayload(connectorDraft, activeConnector);
  const selectedRouteSummary = selectedProxyProfile
    ? connectorRouteSummary(activeConnector, selectedProxyProfile.metadata || {})
    : activeRoutePayload.error
      ? "Custom route is not configured yet"
      : connectorRouteSummary(activeConnector, activeRoutePayload.metadata);
  const selectedRouteLastSyncedAt = selectedProxyProfile
    ? formatRouteSyncTime(lastProxySyncSummary?.profileId === selectedProxyProfile.id ? lastProxySyncSummary.syncedAt : selectedProxyProfile.lastSyncedAt)
    : "Never";
  const selectedRouteLastPullCount = selectedProxyProfile && lastProxySyncSummary?.profileId === selectedProxyProfile.id
    ? lastProxySyncSummary.createdCount
    : null;
  const threadCode = shortThreadCode(thread.id);
  const caseCreatedLabel = linkedCase?.time
    ? `${formatDistanceToNow(new Date(linkedCase.time), { addSuffix: true })}`
    : "n/a";

  return (
    <AppLayout>
      <div className={PAGE_SHELL_CLASS}>
        <div className={HEADER_BLOCK_CLASS}>
          <div className="flex flex-col gap-2">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="flex min-w-0 items-center gap-2.5">
                <Link href={withTenantPath(currentTenantSlug, "/forum")}>
                  <button
                    type="button"
                    className="inline-flex h-6 w-6 items-center justify-center rounded-md text-[#9ca3af] transition-colors hover:bg-[#1d1e29] hover:text-white"
                    aria-label="Back to forum"
                  >
                    <ChevronLeft size={16} />
                  </button>
                </Link>
                <div className="flex min-w-0 items-center">
                  <h1 className="truncate text-[18px] font-normal leading-6 tracking-[-0.4px] text-white">{thread.title}</h1>
                </div>
              </div>

              <div className="flex items-center gap-2 text-[11px] leading-4 tracking-[-0.4px] text-[#9ca3af]">
                <span>{threadPostsCount} messages</span>
                <span className="h-4 w-px bg-[#2a2c3c]" />
                <span>{threadParticipantsCount} participants</span>
              </div>
            </div>

            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="flex min-w-0 flex-wrap items-center gap-2 text-[11px] leading-4 tracking-[-0.35px] text-[#9ca3af]">
                <span className="rounded-[6px] border border-[#2a2c3c] bg-[#0b0c10] px-2 py-1 text-[#d1d5db]">#{threadCode}</span>
                {linkedCase ? (
                  <>
                    <span>{linkedCase.id}</span>
                    <Badge
                      className={`h-[20px] rounded-[4px] border px-2 text-[10px] font-normal uppercase tracking-[0.03em] ${severityClass(linkedCase.sev)}`}
                    >
                      {linkedCase.sev || "Unknown"}
                    </Badge>
                  </>
                ) : null}
              </div>

              {linkedCase ? (
                <Button
                  type="button"
                  variant="outline"
                  className={`${OUTLINE_BUTTON_CLASS} h-7 rounded-[8px] px-2.5 text-[11px]`}
                  onClick={() => setThreadDetailsExpanded((prev) => !prev)}
                  data-testid="button-toggle-forum-case-context"
                >
                  {threadDetailsExpanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
                  <span className="ml-1">{threadDetailsExpanded ? "Hide details" : "Show details"}</span>
                </Button>
              ) : null}
            </div>
          </div>
        </div>

        {linkedCase && threadDetailsExpanded && (
          <div className={CASE_PANEL_CLASS}>
            <div className="rounded-[10px] border border-[#2a2c3c] bg-[#0b0c10] px-3 py-2">
              <div className="flex flex-wrap items-center justify-between gap-2.5">
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <h2 className="min-w-0 flex-1 truncate text-[15px] font-normal leading-6 tracking-[-0.4px] text-white">{linkedCase.title}</h2>
                  </div>

                  <div className="mt-1 flex flex-wrap items-center gap-2 text-[11px] leading-4 tracking-[-0.4px]">
                    <span className="text-[#9ca3af]">{t("forumThread.owner")}:</span>
                    {linkedCase.owner ? (
                      <span className="inline-flex items-center gap-1.5">
                        <UserAvatar
                          name={linkedCaseOwner?.name || linkedCase.owner}
                          avatar={linkedCaseOwner?.avatar}
                          fallback={linkedCase.owner}
                          className="h-5 w-5 border border-[#2a2c3c]"
                          fallbackClassName="text-[10px] font-bold"
                        />
                        <span className="text-white">{linkedCaseOwner?.name || linkedCase.owner}</span>
                      </span>
                    ) : (
                      <span className="text-white">{t("forumThread.unassigned")}</span>
                    )}
                    <span className="text-[#2a2c3c]">•</span>
                    <span className="text-[#6b7280]">{`${t("forumThread.created")}: ${caseCreatedLabel}`}</span>
                  </div>

                  {(linkedCase.tags || []).length > 0 && (
                    <div className="mt-2 flex flex-wrap items-center gap-1.5">
                      {(linkedCase.tags || []).slice(0, 6).map((tag: string) => (
                        <span
                          key={tag}
                          className="inline-flex h-[20px] items-center rounded-full border border-[#2a2c3c] bg-[#1d1e29] px-2 text-[10px] leading-[14px] tracking-[-0.2px] text-[#9ca3af]"
                        >
                          {tag}
                        </span>
                      ))}
                    </div>
                  )}
                </div>

                <div className="flex items-center gap-2 self-center">
                  <Link href={withTenantPath(currentTenantSlug, `/cases/${linkedCase.id}`)}>
                    <Button className="h-8 rounded-[8px] border border-[#4ed938] bg-[#4ed938] px-3 text-[11px] font-normal text-[#0b0c10] shadow-[0_0_10px_rgba(102,255,76,0.2)] hover:bg-[#63ed4f]">
                      <FolderKanban size={13} />
                      <span className="ml-1.5">{t("forumThread.viewFullCase")}</span>
                    </Button>
                  </Link>
                </div>
              </div>
            </div>
          </div>
        )}

        <div ref={messagesScrollRef} className={CHAT_STREAM_CLASS}>
          <div className="mx-auto flex w-full max-w-[1280px] flex-col gap-5 px-4 py-5 md:px-6">
            {posts.map((post: any, index: number) => {
              const isMe = String(post.authorId || "") === String(currentViewer.id || "");
              const authorUser = post.authorId ? usersByID.get(String(post.authorId)) : null;
              const authorLabel = authorUser?.name || post.authorName || post.authorId || "User";
              const previousPost = index > 0 ? posts[index - 1] : null;
              const showDayDivider =
                index > 0 &&
                previousPost?.timestamp &&
                post?.timestamp &&
                !isSameCalendarDay(previousPost.timestamp, post.timestamp);
              const dayLabel = formatMessageDayLabel(post.timestamp);
              const attachments = Array.isArray(post.attachments) ? post.attachments : [];
              const imageAttachments = attachments.filter((attachment: any) => isImageAttachment(attachment));
              const fileAttachments = attachments.filter((attachment: any) => !isImageAttachment(attachment));

              return (
                <div key={post.id} className="space-y-4">
                  {showDayDivider && dayLabel ? (
                    <div className="flex items-center gap-3">
                      <div className="h-px flex-1 bg-[#1d1e29]" />
                      <span className="text-[10px] leading-4 tracking-[-0.5px] text-[#6b7280]">{dayLabel}</span>
                      <div className="h-px flex-1 bg-[#1d1e29]" />
                    </div>
                  ) : null}

                  <div className={`flex w-full items-start gap-3 ${isMe ? "justify-end" : ""}`}>
                    {!isMe ? (
                      <UserAvatar
                        name={authorLabel}
                        avatar={authorUser?.avatar}
                        fallback={post.authorId}
                        className="h-10 w-10 border-2 border-[#2a2c3c]"
                        fallbackClassName="bg-[#0f131d] text-[11px] font-semibold text-[#f3f4f6]"
                      />
                    ) : null}

                    <div className={`max-w-[min(84%,980px)] ${isMe ? "order-1" : ""}`}>
                      <div
                        className={`rounded-[16px] border px-4 py-4 shadow-[0_4px_20px_rgba(0,0,0,0.3)] ${
                          isMe
                            ? "rounded-tr-[2px] border-[rgba(78,217,56,0.3)] bg-[linear-gradient(171deg,rgba(78,217,56,0.2)_0%,rgba(5,150,105,0.2)_70%)]"
                            : "rounded-tl-[2px] border-[#1d1e29] bg-[#13141c]"
                        }`}
                      >
                        <div className={`mb-2 flex items-center gap-2 text-[12px] leading-4 tracking-[-0.5px] ${isMe ? "justify-end" : ""}`}>
                          {isMe ? (
                            <>
                              <span className="text-[#6b7280]">{formatDistanceToNow(new Date(post.timestamp), { addSuffix: true })}</span>
                              <span className="text-white">{authorLabel}</span>
                            </>
                          ) : (
                            <>
                              <span className="text-white">{authorLabel}</span>
                              <span className="text-[#6b7280]">{formatDistanceToNow(new Date(post.timestamp), { addSuffix: true })}</span>
                            </>
                          )}
                        </div>

                        <p className="whitespace-pre-wrap text-[14px] font-normal leading-[23px] tracking-[-0.5px] text-[#d1d5db]">{post.content}</p>

                        {fileAttachments.length > 0 && (
                          <div className="mt-3 space-y-2">
                            {fileAttachments.map((attachment: any, idx: number) => {
                              const href = resolveAttachmentLink(attachment);
                              const fileName = String(attachment.fileName || attachment.filename || attachment.name || "attachment");
                              const fileSize = formatBytes(attachment.size || attachment.sizeBytes || attachment.size_bytes);

                              return (
                                <div key={`${attachment.id || fileName}-${idx}`} className="rounded-[8px] border border-[#2a2c3c] bg-[#0b0c10] p-3">
                                  <div className="mb-2 flex items-center gap-1.5 text-[11px] uppercase tracking-[0.06em] text-[#9ca3af]">
                                    <Paperclip size={11} className="text-[#4ed938]" />
                                    Attachment
                                  </div>
                                  <div className="flex items-center justify-between gap-3">
                                    <div className="min-w-0 flex-1">
                                      <div className="flex min-w-0 items-center gap-2">
                                        <span className="inline-flex h-10 w-10 items-center justify-center rounded-[8px] bg-[rgba(37,99,235,0.1)] text-[#60a5fa]">
                                          <FileText size={14} />
                                        </span>
                                        <div className="min-w-0">
                                          <EllipsisText text={fileName} className="max-w-[300px] text-[13px] font-normal text-white" />
                                          {fileSize ? <p className="text-[11px] text-[#6b7280]">{fileSize}</p> : null}
                                        </div>
                                      </div>
                                    </div>
                                    {href ? (
                                      <a
                                        href={href}
                                        target="_blank"
                                        rel="noopener noreferrer"
                                        className="inline-flex h-7 w-7 items-center justify-center rounded-md text-[#9ca3af] transition-colors hover:text-white"
                                      >
                                        <Download size={13} />
                                      </a>
                                    ) : null}
                                  </div>
                                </div>
                              );
                            })}
                          </div>
                        )}

                        {imageAttachments.length > 0 && (
                          <div className={`mt-3 grid gap-2 ${imageAttachments.length > 1 ? "sm:grid-cols-2" : ""}`}>
                            {imageAttachments.map((attachment: any, idx: number) => {
                              const href = resolveAttachmentLink(attachment);
                              const previewUrl =
                                String(attachment.previewUrl || attachment.preview_url || "").trim() || href;
                              const fileName = String(attachment.fileName || attachment.filename || attachment.name || "image");
                              return (
                                <div key={`${attachment.id || fileName}-${idx}`} className="overflow-hidden rounded-[6px] border border-[#2a2c3c] bg-[#0b0c10]">
                                  {previewUrl ? (
                                    <a href={href || undefined} target="_blank" rel="noopener noreferrer" className="block">
                                      <img src={previewUrl} alt={fileName} className="h-36 w-full object-cover" />
                                    </a>
                                  ) : (
                                    <div className="flex h-36 w-full items-center justify-center bg-[#111625] text-[#4b5563]">
                                      <FileImage size={18} />
                                    </div>
                                  )}
                                  <p className="truncate border-t border-[#2a2c3c] px-2 py-1.5 text-[10px] text-[#9ca3af]">{fileName}</p>
                                </div>
                              );
                            })}
                          </div>
                        )}
                      </div>
                    </div>

                    {isMe ? (
                      <UserAvatar
                        name={authorLabel}
                        avatar={authorUser?.avatar}
                        fallback={post.authorId}
                        className="h-10 w-10 border-2 border-[#2a2c3c]"
                        fallbackClassName="bg-[#4ed938] text-[11px] font-semibold text-[#0b0c10]"
                      />
                    ) : null}
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        <div className={`${COMPOSER_CLASS} relative`}>
          <input
            ref={fileInputRef}
            type="file"
            className="hidden"
            multiple
            onChange={handleSelectFiles}
            data-testid="input-forum-attachments"
          />

          {pendingFiles.length > 0 && (
            <div className="mb-2 grid gap-2 sm:grid-cols-2 lg:grid-cols-3" data-testid="forum-attachment-list">
              {pendingFiles.map((file, idx) => {
                const previewUrl = pendingImagePreviewUrls[idx];
                const fileSize = formatBytes(file.size);
                return (
                  <div
                    key={`${file.name}-${idx}`}
                    className="group relative overflow-hidden rounded-[8px] border border-[#2a2c3c] bg-[#0b0c10]"
                  >
                    {previewUrl ? <img src={previewUrl} alt={file.name} className="h-24 w-full border-b border-[#2a2c3c] object-cover" /> : null}

                    <div className="flex items-start gap-2 p-2">
                      <div className="mt-0.5 inline-flex h-8 w-8 items-center justify-center rounded-[6px] bg-[rgba(37,99,235,0.1)] text-[#60a5fa]">
                        {previewUrl ? <FileImage size={13} className="text-[#4ed938]" /> : <FileText size={13} />}
                      </div>
                      <div className="min-w-0 flex-1">
                        <EllipsisText text={file.name} className="max-w-[220px] text-xs font-normal text-[#f3f4f6]" />
                        <p className="mt-0.5 text-[10px] text-[#9ca3af]">{[file.type || "application/octet-stream", fileSize].filter(Boolean).join(" • ")}</p>
                      </div>
                    </div>

                    <button
                      type="button"
                      className="absolute right-1.5 top-1.5 inline-flex h-6 w-6 items-center justify-center rounded-full border border-[#2a2c3c] bg-[#0f131d]/90 text-[#9ca3af] opacity-0 transition-all hover:border-[#4b5168] hover:text-white group-hover:opacity-100"
                      onClick={() => removePendingFile(idx)}
                      data-testid={`button-remove-forum-attachment-${idx}`}
                    >
                      <X size={12} />
                    </button>
                  </div>
                );
              })}
            </div>
          )}

          {connectorDialogOpen ? (
            <div className="pointer-events-none absolute bottom-[calc(100%+10px)] left-5 right-5 z-40">
              <div className="pointer-events-auto ml-auto w-full max-w-[900px] rounded-[18px] border border-[#2a2c3c] bg-[linear-gradient(145deg,rgba(28,32,50,0.96),rgba(15,18,30,0.96))] p-3 shadow-[0_18px_40px_rgba(0,0,0,0.45)] backdrop-blur">
                <div className="mb-2 flex items-center justify-between gap-3">
                  <div className="min-w-0">
                    <p className="truncate text-[11px] uppercase tracking-[0.07em] text-[#6b7280]">Send via connector</p>
                    <p className="truncate text-[12px] text-[#d1d5db]">Choose connector and recipient parameters</p>
                  </div>
                  <button
                    type="button"
                    onClick={closeConnectorDialog}
                    className="inline-flex h-7 w-7 items-center justify-center rounded-md text-[#9ca3af] transition-colors hover:bg-[#1d1e29] hover:text-white"
                    aria-label="Close connector send dialog"
                  >
                    <X size={14} />
                  </button>
                </div>

                {threadProxyProfiles.length > 0 ? (
                <div className="mb-3 rounded-[14px] border border-[#2a2c3c] bg-[#0d1220] p-3">
                  <div className="mb-2 flex items-center justify-between gap-3">
                    <div className="min-w-0">
                      <p className="truncate text-[11px] uppercase tracking-[0.07em] text-[#6b7280]">Linked external route</p>
                      <p className="truncate text-[12px] text-[#d1d5db]">Select a saved route for reply sync or switch to custom routing for a new send.</p>
                    </div>
                    <Badge variant="outline" className="border-[#2a2c3c] bg-[#0b0c10] text-[10px] text-[#9ca3af]">
                      {threadProxyProfiles.length} linked
                    </Badge>
                  </div>
                  <div className="grid gap-3 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)]">
                    <div className="space-y-1">
                      <label className="text-[10px] uppercase tracking-[0.06em] text-[#6b7280]">Route</label>
                      <Select value={selectedProxyProfileId || CUSTOM_PROXY_PROFILE_ID} onValueChange={setSelectedProxyProfileId}>
                        <SelectTrigger className="h-8 rounded-md border-[#2a2c3c] bg-[#0b0c10] px-2 text-[11px] text-[#d1d5db]" data-testid="select-forum-thread-proxy-profile">
                          <SelectValue placeholder="Choose linked route" />
                        </SelectTrigger>
                        <SelectContent className="min-w-[260px]" searchable searchPlaceholder="Search linked routes...">
                          {threadProxyProfiles.map((profile: any) => {
                            const connector = outboundConnectors.find((item: any) => String(item?.id) === String(profile?.connectorId || "")) || null;
                            return (
                              <SelectItem key={`forum-profile-${profile.id}`} value={String(profile.id)}>
                                {profile?.name || connectorRouteSummary(connector, profile?.metadata || {})}
                              </SelectItem>
                            );
                          })}
                          <SelectItem value={CUSTOM_PROXY_PROFILE_ID}>Custom route</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="rounded-[12px] border border-[#2a2c3c] bg-[#0b0c10] px-3 py-2">
                      <div className="flex flex-wrap items-center gap-2 text-[10px] uppercase tracking-[0.06em] text-[#6b7280]">
                        <span>Route summary</span>
                        {activeConnector?.name ? (
                          <Badge variant="outline" className="border-[#2a2c3c] bg-transparent text-[10px] text-[#9ca3af]">
                            {activeConnector.name}
                          </Badge>
                        ) : null}
                      </div>
                      <p className="mt-2 text-[12px] text-[#d1d5db]">{selectedRouteSummary}</p>
                      {selectedProxyProfile ? (
                        <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-[11px] text-[#9ca3af]">
                          <span>Last synced: {selectedRouteLastSyncedAt}</span>
                          <span>{selectedRouteLastPullCount == null ? "Last pull: —" : `Last pull: ${selectedRouteLastPullCount} repl${selectedRouteLastPullCount === 1 ? "y" : "ies"}`}</span>
                        </div>
                      ) : (
                        <p className="mt-2 text-[11px] text-[#6b7280]" data-testid="forum-proxy-profile-empty">
                          Send the first message via connector to create a linked route for reply sync.
                        </p>
                      )}
                    </div>
                  </div>
                </div>
                ) : null}

                <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
                  <div className="space-y-1 lg:col-span-2">
                    <label className="text-[10px] uppercase tracking-[0.06em] text-[#6b7280]">Connector</label>
                    <Select
                      value={connectorDraft.connectorId || "none"}
                      onValueChange={(value) => {
                        setSelectedProxyProfileId(CUSTOM_PROXY_PROFILE_ID);
                        setConnectorDraft((prev) => ({ ...prev, connectorId: value === "none" ? "" : value }));
                      }}
                      disabled={outboundConnectors.length === 0}
                    >
                      <SelectTrigger className="h-8 rounded-md border-[#2a2c3c] bg-[#0b0c10] px-2 text-[11px] text-[#d1d5db]" data-testid="select-forum-thread-connector-trigger">
                        <SelectValue placeholder="Select connector..." />
                      </SelectTrigger>
                      <SelectContent className="min-w-[220px]" searchable searchPlaceholder="Search connectors...">
                        <SelectItem value="none">Select connector...</SelectItem>
                        {outboundConnectors.map((connector: any) => (
                          <SelectItem key={`forum-connector-${connector.id}`} value={String(connector.id)}>
                            {connector?.name || connector?.title || connector?.id}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <input
                      type="hidden"
                      value={connectorDraft.connectorId}
                      onChange={(event) => setConnectorDraft((prev) => ({ ...prev, connectorId: event.target.value }))}
                      data-testid="select-forum-thread-connector"
                    />
                  </div>
                  <div className="space-y-1 lg:col-span-2">
                    <label className="text-[10px] uppercase tracking-[0.06em] text-[#6b7280]">Label (optional)</label>
                    <Input
                      value={connectorDraft.name}
                      onChange={(event) => {
                        setSelectedProxyProfileId(CUSTOM_PROXY_PROFILE_ID);
                        setConnectorDraft((prev) => ({ ...prev, name: event.target.value }));
                      }}
                      placeholder="Recipient label"
                      className="h-8 border-[#2a2c3c] bg-[#0b0c10] text-[12px] text-[#d1d5db] placeholder:text-[#6b7280]"
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-[10px] uppercase tracking-[0.06em] text-[#6b7280]">{activeConnectorMode === "email" ? "To" : activeConnectorChannel === "slack" ? "Channel ID" : activeConnectorChannel === "telegram" ? "Chat ID" : "Target"}</label>
                    <Input
                      value={connectorDraft.chatId}
                      onChange={(event) => {
                        setSelectedProxyProfileId(CUSTOM_PROXY_PROFILE_ID);
                        setConnectorDraft((prev) => ({ ...prev, chatId: event.target.value }));
                      }}
                      placeholder={activeConnectorMode === "email" ? "recipient@example.com" : activeConnectorChannel === "slack" ? "C123456789" : activeConnectorChannel === "telegram" ? "Telegram chat id" : "Routing target"}
                      className="h-8 border-[#2a2c3c] bg-[#0b0c10] text-[12px] text-[#d1d5db] placeholder:text-[#6b7280]"
                      data-testid="input-forum-thread-connector-chat-id"
                    />
                  </div>
                  {activeConnectorChannel !== "slack" ? (
                    <div className="space-y-1">
                      <label className="text-[10px] uppercase tracking-[0.06em] text-[#6b7280]">{activeConnectorMode === "email" ? "Subject" : "Username (optional)"}</label>
                      <Input
                        value={connectorDraft.username}
                        onChange={(event) => {
                          setSelectedProxyProfileId(CUSTOM_PROXY_PROFILE_ID);
                          setConnectorDraft((prev) => ({ ...prev, username: event.target.value }));
                        }}
                        placeholder={activeConnectorMode === "email" ? "Thread subject" : "@username"}
                        className="h-8 border-[#2a2c3c] bg-[#0b0c10] text-[12px] text-[#d1d5db] placeholder:text-[#6b7280]"
                      />
                    </div>
                  ) : null}
                  {activeConnectorChannel !== "slack" ? (
                    <div className="space-y-1">
                      <label className="text-[10px] uppercase tracking-[0.06em] text-[#6b7280]">{activeConnectorMode === "email" ? "CC (optional)" : "External user ID (optional)"}</label>
                      <Input
                        value={connectorDraft.externalUserId}
                        onChange={(event) => {
                          setSelectedProxyProfileId(CUSTOM_PROXY_PROFILE_ID);
                          setConnectorDraft((prev) => ({ ...prev, externalUserId: event.target.value }));
                        }}
                        placeholder={activeConnectorMode === "email" ? "cc@example.com" : "External user id"}
                        className="h-8 border-[#2a2c3c] bg-[#0b0c10] text-[12px] text-[#d1d5db] placeholder:text-[#6b7280]"
                      />
                    </div>
                  ) : (
                    <div className="space-y-1">
                      <label className="text-[10px] uppercase tracking-[0.06em] text-[#6b7280]">Thread TS (optional)</label>
                      <Input
                        value={connectorDraft.externalUserId}
                        onChange={(event) => {
                          setSelectedProxyProfileId(CUSTOM_PROXY_PROFILE_ID);
                          setConnectorDraft((prev) => ({ ...prev, externalUserId: event.target.value }));
                        }}
                        placeholder="1741439188.021300"
                        className="h-8 border-[#2a2c3c] bg-[#0b0c10] text-[12px] text-[#d1d5db] placeholder:text-[#6b7280]"
                      />
                    </div>
                  )}
                </div>

                <div className="mt-2 flex items-center justify-end gap-2">
                  <Button
                    type="button"
                    variant="ghost"
                    onClick={closeConnectorDialog}
                    className="h-7 rounded-md px-2 text-[11px] text-[#9ca3af] hover:bg-[#161b2a] hover:text-white"
                  >
                    Cancel
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={handleConnectorSync}
                    disabled={!canSyncViaConnector}
                    className={`h-7 rounded-md px-3 text-[11px] font-normal ${OUTLINE_BUTTON_CLASS}`}
                    data-testid="button-forum-sync-replies"
                  >
                    {proxyForumSync.isPending ? <Loader2 size={12} className="animate-spin" /> : <RefreshCcw size={12} />}
                    Sync replies
                  </Button>
                  <Button
                    type="button"
                    onClick={handleConnectorSend}
                    disabled={pendingProxy}
                    className="h-7 rounded-md border border-[#4ed938] bg-[#4ed938] px-3 text-[11px] font-normal text-[#0b0c10] hover:bg-[#66ff4c]"
                    data-testid="button-forum-send-via-connector-confirm"
                  >
                    {proxyForumSend.isPending ? <Loader2 size={12} className="animate-spin" /> : <MessageSquare size={12} />}
                    Send via connector
                  </Button>
                </div>
              </div>
            </div>
          ) : null}

          <div className="rounded-[8px] border border-[#1d1e29] bg-[#070b15] px-3 pt-2">

            <Textarea
              placeholder={t("forumThread.replyPlaceholder")}
              data-testid="textarea-forum-thread-reply"
              className="min-h-[32px] resize-none border-0 bg-transparent p-0 text-[13px] leading-5 text-[#d1d5db] placeholder:text-[#6b7280] focus-visible:ring-0 focus-visible:ring-offset-0"
              value={replyContent}
              onChange={(event) => setReplyContent(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter" && !event.shiftKey) {
                  event.preventDefault();
                  handleSend();
                }
              }}
            />

            <div className="mt-2 flex flex-wrap items-center justify-between gap-2 border-t border-[#1d1e29] py-2">
              <div className="flex min-w-0 flex-wrap items-center gap-1">
                <button
                  type="button"
                  className="inline-flex h-7 w-7 items-center justify-center rounded-md text-[#9ca3af] transition-colors hover:bg-[#161b2a] hover:text-white"
                  onClick={() => fileInputRef.current?.click()}
                  data-testid="button-add-forum-attachment"
                  aria-label={t("forumThread.attachFiles")}
                >
                  <Paperclip size={13} />
                </button>
                <button
                  type="button"
                  className="inline-flex h-7 w-7 items-center justify-center rounded-md text-[#9ca3af] transition-colors hover:bg-[#161b2a] hover:text-white"
                  aria-label="Add link"
                >
                  <Link2 size={13} />
                </button>
                <button
                  type="button"
                  className="inline-flex h-7 w-7 items-center justify-center rounded-md text-[#9ca3af] transition-colors hover:bg-[#161b2a] hover:text-white"
                  aria-label="Mention"
                >
                  <AtSign size={13} />
                </button>
              </div>

              <div className="flex items-center gap-2">
                <span className="hidden text-[10px] text-[#6b7280] md:inline">Press Enter to send</span>
                <Button
                  type="button"
                  variant="outline"
                  className={`h-7 gap-1 rounded-md px-2 text-[11px] font-normal ${OUTLINE_BUTTON_CLASS}`}
                  onClick={handleProxySend}
                  disabled={!canSendViaConnector}
                  data-testid="button-forum-send-via-connector"
                  title={activeConnector ? `Send through ${activeConnector.name || activeConnector.id}` : "Configure connector send"}
                >
                  {proxyForumSend.isPending ? <Loader2 size={12} className="animate-spin" /> : <MessageSquare size={12} />}
                  Via connector
                </Button>
                <Button
                  className="h-7 gap-1 rounded-md border border-[#4ed938] bg-[#4ed938] px-3 text-[11px] font-normal text-[#0b0c10] shadow-[0_0_10px_rgba(102,255,76,0.2)] hover:bg-[#66ff4c]"
                  onClick={handleSend}
                  disabled={!canSendLocal}
                  data-testid="button-forum-send"
                >
                  {pendingSend ? <Loader2 size={12} className="animate-spin" /> : <Send size={12} />}
                  {t("forumThread.send")}
                </Button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </AppLayout>
  );
}
