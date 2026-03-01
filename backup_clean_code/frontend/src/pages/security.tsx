import { AppLayout } from "@/components/layout";
import { Card } from "@/components/ui/card";
import {
  useAppState,
  useUpdateUser,
  useAuthSessions,
  useRevokeOtherSessions,
  useNotificationBots,
  useMyNotificationSettings,
  useSaveMyNotificationSettings,
} from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { Shield, Clock, Eye, EyeOff, Save, BellRing } from "lucide-react";
import { useT } from "@/lib/i18n";
import { Skeleton } from "@/components/ui/skeleton";
import { useMinimumLoading } from "@/lib/use-minimum-loading";

const PAGE_SHELL_CLASS = "mx-auto w-full max-w-[1144px] space-y-6 pb-6";
const PANEL_CLASS =
  "rounded-2xl border border-[rgba(255,255,255,0.06)] bg-[linear-gradient(180deg,rgba(19,20,28,0.97),rgba(17,20,32,0.97))] shadow-[0_14px_34px_rgba(0,0,0,0.28)]";
const SUBPANEL_CLASS = "rounded-xl border border-[#2a2c3c] bg-[#111624]";
const MUTED_TEXT_CLASS = "text-xs text-[#8b91a3]";
const INPUT_CLASS =
  "h-[40px] border-[#2a2c3c] bg-[#0b0c10] text-[#f3f4f6] placeholder:text-[#6b7280] focus-visible:ring-1 focus-visible:ring-[#3b4a79]";
const TABS_LIST_CLASS = "inline-flex h-11 w-full items-stretch justify-start rounded-full border border-[#2a2c3c] bg-[#10121a] p-0 overflow-hidden";
const TAB_TRIGGER_CLASS =
  "relative flex flex-1 items-center justify-center gap-2 px-4 text-xs font-semibold text-[#9ca3af] transition-colors hover:bg-[#171b2a] hover:text-[#e5e7eb] data-[state=active]:bg-[#111827] data-[state=active]:text-white data-[state=active]:after:absolute data-[state=active]:after:bottom-0 data-[state=active]:after:left-3 data-[state=active]:after:right-3 data-[state=active]:after:h-[2px] data-[state=active]:after:rounded-full data-[state=active]:after:bg-[#66ff4c]";
const DELIVERY_CHANNEL_LIST_CLASS =
  "mt-1 grid w-full grid-cols-2 md:grid-cols-4 rounded-full border border-[#2a2c3c] bg-[#10121a] p-0 overflow-hidden";
const DELIVERY_CHANNEL_TRIGGER_CLASS =
  "h-9 text-xs font-semibold text-[#9ca3af] hover:bg-[#171b2a] hover:text-[#d1d5db] data-[state=active]:bg-[#10141f] data-[state=active]:text-white border-r border-[#1f2535] last:border-r-0 rounded-none flex items-center justify-center transition-colors";

export default function SecurityPage() {
  const t = useT();
  const { currentUserId, currentTenantId } = useAppState();
  const { data: authSessionsData, isLoading: authSessionsLoading } = useAuthSessions(40);
  const { data: notificationBots = [], isLoading: notificationBotsLoading } = useNotificationBots(currentTenantId);
  const { data: notificationSettings, isLoading: notificationSettingsLoading } = useMyNotificationSettings(currentTenantId);
  const saveMyNotificationSettings = useSaveMyNotificationSettings();
  const revokeOtherSessions = useRevokeOtherSessions();
  const updateUser = useUpdateUser();

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showCurrentPassword, setShowCurrentPassword] = useState(false);
  const [showNewPassword, setShowNewPassword] = useState(false);
  const [terminateOtherSessionsAfterPasswordChange, setTerminateOtherSessionsAfterPasswordChange] = useState(false);
  const [deliveryEnabled, setDeliveryEnabled] = useState(false);
  const [deliveryChannel, setDeliveryChannel] = useState<"in_app" | "telegram" | "email" | "time">("in_app");
  const [telegramBotID, setTelegramBotID] = useState("");
  const [telegramChatID, setTelegramChatID] = useState("");
  const [telegramUsername, setTelegramUsername] = useState("");
  const [notificationEmail, setNotificationEmail] = useState("");
  const [timeRecipient, setTimeRecipient] = useState("");
  const [notificationSettingsInitialized, setNotificationSettingsInitialized] = useState(false);
  const [tickNowMS, setTickNowMS] = useState(() => Date.now());

  useEffect(() => {
    const timer = window.setInterval(() => setTickNowMS(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, []);

  useEffect(() => {
    if (notificationSettingsInitialized || !notificationSettings) {
      return;
    }
    setDeliveryEnabled(Boolean(notificationSettings.deliveryEnabled));
    const channel = String(notificationSettings.deliveryChannel || "in_app").toLowerCase();
    if (channel === "telegram" || channel === "email" || channel === "time") {
      setDeliveryChannel(channel);
    } else {
      setDeliveryChannel("in_app");
    }
    setTelegramBotID(String(notificationSettings.telegramBotId || ""));
    setTelegramChatID(String(notificationSettings.telegramChatId || ""));
    setTelegramUsername(String(notificationSettings.telegramUsername || ""));
    setNotificationEmail(String(notificationSettings.notificationEmail || ""));
    setTimeRecipient(String(notificationSettings.timeRecipient || ""));
    setNotificationSettingsInitialized(true);
  }, [notificationSettings, notificationSettingsInitialized]);

  const sessions = authSessionsData?.sessions || [];
  const sessionTimeoutSeconds = authSessionsData?.sessionTimeoutSeconds || 0;
  const isSecurityPageLoading = authSessionsLoading || notificationBotsLoading || notificationSettingsLoading;
  const showSecurityPageSkeleton = useMinimumLoading(isSecurityPageLoading);
  const showSessionsLoadingState = useMinimumLoading(authSessionsLoading);
  const showNotificationSettingsLoadingState = useMinimumLoading(notificationSettingsLoading);

  const parseTimeMS = (value?: string | null): number => {
    if (!value) return 0;
    const ts = new Date(value).getTime();
    if (Number.isNaN(ts)) return 0;
    return ts;
  };

  const effectiveSessionStatus = (session: any): string => {
    if (session?.status !== "active") {
      return session?.status || "expired";
    }
    const expiresAtMS = parseTimeMS(session?.expiresAt);
    if (expiresAtMS > 0 && expiresAtMS <= tickNowMS) {
      return "expired";
    }
    return "active";
  };

  const sessionDurationSeconds = (session: any): number => {
    const status = effectiveSessionStatus(session);
    if (status !== "active") {
      return Number(session?.durationSeconds || 0);
    }
    const startedAtMS = parseTimeMS(session?.createdAt);
    if (startedAtMS <= 0) {
      return Number(session?.durationSeconds || 0);
    }
    return Math.max(0, Math.floor((tickNowMS - startedAtMS) / 1000));
  };

  const sessionRemainingSeconds = (session: any): number => {
    if (effectiveSessionStatus(session) !== "active") {
      return 0;
    }
    const expiresAtMS = parseTimeMS(session?.expiresAt);
    if (expiresAtMS <= 0) {
      return Number(session?.remainingSeconds || 0);
    }
    return Math.max(0, Math.floor((expiresAtMS - tickNowMS) / 1000));
  };


  const sessionsForDisplay = useMemo(() => {
    if (!Array.isArray(sessions) || sessions.length === 0) return [];
    const annotated = [...sessions];
    annotated.sort((left: any, right: any) => {
      const leftCreated = parseTimeMS(left?.createdAt);
      const rightCreated = parseTimeMS(right?.createdAt);
      return rightCreated - leftCreated;
    });
    const active = annotated.filter((session: any) => effectiveSessionStatus(session) === "active");
    const completed = annotated.filter((session: any) => effectiveSessionStatus(session) !== "active");
    const limitedCompleted = completed.slice(0, 3);
    return [...active, ...limitedCompleted];
  }, [sessions, tickNowMS]);

  const activeSessions = useMemo(
    () => sessions.filter((session: any) => effectiveSessionStatus(session) === "active").length,
    [sessions, tickNowMS],
  );
  const currentSession = useMemo(() => sessions.find((session: any) => session.isCurrent) || null, [sessions]);
  const hasOtherSessionsToTerminate = useMemo(
    () => sessions.some((session: any) => effectiveSessionStatus(session) === "active" && !session.isCurrent),
    [sessions, tickNowMS],
  );

  const handleChangePassword = () => {
    if (!currentPassword.trim()) {
      toast.error(t("security.toast.currentPasswordRequired"));
      return;
    }
    if (!newPassword) {
      toast.error(t("security.toast.passwordRequired"));
      return;
    }
    if (newPassword !== confirmPassword) {
      toast.error(t("security.toast.passwordMismatch"));
      return;
    }
    updateUser.mutate(
      { id: currentUserId, data: { password: newPassword, currentPassword } },
      {
        onSuccess: () => {
          toast.success(t("security.toast.passwordUpdated"));
          if (terminateOtherSessionsAfterPasswordChange && hasOtherSessionsToTerminate) {
            revokeOtherSessions.mutate(undefined, {
              onSuccess: (payload: any) => {
                const revoked = Number(payload?.revoked ?? 0);
                toast.success(t("security.toast.sessionsTerminated").replace("{count}", String(revoked)));
              },
              onError: (error: any) => {
                toast.error(error?.message || t("security.toast.sessionsTerminateFailed"));
              },
            });
          }
          setCurrentPassword("");
          setNewPassword("");
          setConfirmPassword("");
        },
      },
    );
  };

  const handleSaveNotificationSettings = () => {
    if (deliveryEnabled && deliveryChannel === "telegram") {
      if (!telegramBotID.trim()) {
        toast.error(t("security.notifications.toast.botRequired"));
        return;
      }
      if (!telegramChatID.trim() && !telegramUsername.trim()) {
        toast.error(t("security.notifications.toast.recipientRequired"));
        return;
      }
    }
    if (deliveryEnabled && deliveryChannel === "email" && !notificationEmail.trim()) {
      toast.error(t("security.notifications.toast.emailRequired"));
      return;
    }
    if (deliveryEnabled && deliveryChannel === "time" && !timeRecipient.trim()) {
      toast.error(t("security.notifications.toast.timeRecipientRequired"));
      return;
    }
    saveMyNotificationSettings.mutate(
      {
        deliveryEnabled,
        deliveryChannel,
        telegramBotId: telegramBotID.trim(),
        telegramChatId: telegramChatID.trim(),
        telegramUsername: telegramUsername.trim(),
        notificationEmail: notificationEmail.trim(),
        timeRecipient: timeRecipient.trim(),
      },
      {
        onSuccess: () => {
          toast.success(t("security.notifications.toast.saved"));
        },
        onError: (error: any) => {
          toast.error(error?.message || t("security.notifications.toast.saveFailed"));
        },
      },
    );
  };

  const formatElapsed = (totalSec: number) => {
    const safeSeconds = Math.max(0, Math.floor(totalSec));
    const h = Math.floor(safeSeconds / 3600);
    const m = Math.floor((safeSeconds % 3600) / 60);
    const s = safeSeconds % 60;
    return `${h.toString().padStart(2, "0")}:${m.toString().padStart(2, "0")}:${s.toString().padStart(2, "0")}`;
  };

  const formatDateTime = (value?: string | null) => {
    if (!value) {
      return "—";
    }
    const parsed = new Date(value);
    if (Number.isNaN(parsed.getTime())) {
      return "—";
    }
    return parsed.toLocaleString();
  };

  const statusClass = (status: string) => {
    if (status === "active") return "border-[rgba(34,197,94,0.32)] bg-[rgba(22,163,74,0.14)]";
    if (status === "revoked") return "border-[rgba(239,68,68,0.32)] bg-[rgba(220,38,38,0.14)]";
    return "border-[rgba(245,158,11,0.3)] bg-[rgba(217,119,6,0.14)]";
  };

  const statusLabel = (status: string) => {
    if (status === "active") return t("security.sessionStatus.active");
    if (status === "revoked") return t("security.sessionStatus.revoked");
    return t("security.sessionStatus.expired");
  };

  const handleTerminateSessions = () => {
    revokeOtherSessions.mutate(undefined, {
      onSuccess: (payload: any) => {
        const revoked = Number(payload?.revoked ?? 0);
        toast.success(t("security.toast.sessionsTerminated").replace("{count}", String(revoked)));
      },
      onError: (error: any) => {
        toast.error(error?.message || t("security.toast.sessionsTerminateFailed"));
      },
    });
  };

  if (showSecurityPageSkeleton) {
    return (
      <AppLayout>
        <div className={PAGE_SHELL_CLASS}>
          <div className="space-y-2">
            <Skeleton className="h-8 w-56 rounded-lg" />
            <Skeleton className="h-4 w-72 rounded-lg" />
          </div>
          <Card className={`${PANEL_CLASS} p-6`}>
            <Skeleton className="mb-4 h-12 w-full rounded-xl" />
            <div className="space-y-4">
              <Skeleton className="h-12 w-full rounded-lg" />
              <Skeleton className="h-12 w-full rounded-lg" />
              <Skeleton className="h-12 w-full rounded-lg" />
              <Skeleton className="h-40 w-full rounded-xl" />
            </div>
          </Card>
        </div>
      </AppLayout>
    );
  }

  return (
    <AppLayout>
      <div className={PAGE_SHELL_CLASS}>
        <div>
          <h1 className="text-[32px] font-semibold leading-8 tracking-[-0.5px] text-white">{t("security.title")}</h1>
          <p className={MUTED_TEXT_CLASS}>{t("security.subtitle")}</p>
        </div>

        <Card className={`${PANEL_CLASS} p-6`} data-testid="card-security">
          <Tabs defaultValue="password">
            <TabsList className={`mb-4 ${TABS_LIST_CLASS}`} data-testid="tabs-security">
              <TabsTrigger value="password" className={TAB_TRIGGER_CLASS} data-testid="tab-password">
                <Shield size={14} className="mr-1" /> {t("security.password")}
              </TabsTrigger>
              <TabsTrigger value="sessions" className={TAB_TRIGGER_CLASS} data-testid="tab-sessions">
                <Clock size={14} className="mr-1" /> {t("security.sessions")}
              </TabsTrigger>
              <TabsTrigger value="notifications" className={TAB_TRIGGER_CLASS} data-testid="tab-notifications">
                <BellRing size={14} className="mr-1" /> {t("security.notifications")}
              </TabsTrigger>
            </TabsList>

            <TabsContent value="password" className="space-y-4">
              <div className="max-w-md space-y-4">
                <div className="space-y-2">
                  <Label className="text-[#d1d5db]">{t("security.currentPassword")}</Label>
                  <div className="relative">
                    <Input
                      className={INPUT_CLASS}
                      type={showCurrentPassword ? "text" : "password"}
                      value={currentPassword}
                      onChange={(e) => setCurrentPassword(e.target.value)}
                      placeholder={t("security.currentPasswordPlaceholder")}
                      data-testid="input-current-password"
                    />
                    <button
                      className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-1 text-[#8b91a3] transition-colors hover:bg-[#171b2a] hover:text-white"
                      onClick={() => setShowCurrentPassword(!showCurrentPassword)}
                      data-testid="button-toggle-current-password"
                    >
                      {showCurrentPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                    </button>
                  </div>
                </div>

                <div className="space-y-2">
                  <Label className="text-[#d1d5db]">{t("security.newPassword")}</Label>
                  <div className="relative">
                    <Input
                      className={INPUT_CLASS}
                      type={showNewPassword ? "text" : "password"}
                      value={newPassword}
                      onChange={(e) => setNewPassword(e.target.value)}
                      placeholder={t("security.newPasswordPlaceholder")}
                      data-testid="input-new-password"
                    />
                    <button
                      className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-1 text-[#8b91a3] transition-colors hover:bg-[#171b2a] hover:text-white"
                      onClick={() => setShowNewPassword(!showNewPassword)}
                      data-testid="button-toggle-new-password"
                    >
                      {showNewPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                    </button>
                  </div>
                </div>

                <div className="space-y-2">
                  <Label className="text-[#d1d5db]">{t("security.confirmPassword")}</Label>
                  <Input
                    className={INPUT_CLASS}
                    type="password"
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    placeholder={t("security.confirmPasswordPlaceholder")}
                    data-testid="input-confirm-password"
                  />
                </div>

                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="terminate-sessions-after-password-change"
                    checked={terminateOtherSessionsAfterPasswordChange}
                    onCheckedChange={(checked) => setTerminateOtherSessionsAfterPasswordChange(checked === true)}
                  />
                  <Label htmlFor="terminate-sessions-after-password-change" className="text-sm font-normal text-[#d1d5db]">
                    {t("security.terminateOthersAfterPasswordChange")}
                  </Label>
                </div>

                <div className="flex items-center justify-between">
                  <p className={MUTED_TEXT_CLASS}>{t("security.passwordHint")}</p>
                  <Button onClick={handleChangePassword} disabled={updateUser.isPending} data-testid="button-save-password">
                    <Save size={14} className="mr-1" /> {t("security.savePassword")}
                  </Button>
                </div>
              </div>
            </TabsContent>

            <TabsContent value="sessions" className="space-y-4">
              <div className="space-y-4">
                {currentSession ? (
                  <div
                    className="rounded-lg border border-[rgba(34,197,94,0.32)] bg-[rgba(22,163,74,0.14)] p-4"
                    data-testid="session-active"
                  >
                    <div className="flex items-center gap-3">
                      <div className="w-3 h-3 bg-green-500 rounded-full animate-pulse" />
                      <div>
                        <p className="text-sm font-medium text-white">{t("security.currentSession")}</p>
                        <p className={MUTED_TEXT_CLASS}>{t("security.activeFor").replace("{time}", formatElapsed(sessionDurationSeconds(currentSession)))}</p>
                      </div>
                    </div>
                  </div>
                ) : null}
                <div className={`${SUBPANEL_CLASS} flex items-center justify-between p-4`}>
                  <div className="flex items-center gap-2">
                    <Clock size={16} className="text-[#8b91a3]" />
                    <span className="text-sm text-[#d1d5db]">{t("security.timeout")}</span>
                  </div>
                  <span className="text-sm font-mono tabular-nums text-white inline-flex w-[96px] justify-end" data-testid="text-session-remaining">{formatElapsed(sessionTimeoutSeconds)}</span>
                </div>
                {showSessionsLoadingState ? (
                  <div className="space-y-2">
                    <Skeleton className="h-14 w-full rounded-xl" />
                    <Skeleton className="h-14 w-full rounded-xl" />
                    <Skeleton className="h-14 w-full rounded-xl" />
                  </div>
                ) : sessionsForDisplay.length > 0 ? (
                  <div className="space-y-3" data-testid="session-list">
                    <p className={MUTED_TEXT_CLASS}>{t("security.activeSessionsCount").replace("{count}", String(activeSessions))}</p>
                    {sessionsForDisplay.map((session: any) => (
                      <div key={session.id} className={`rounded-lg border p-3 ${statusClass(effectiveSessionStatus(session))}`} data-testid={`session-item-${session.id}`}>
                        <div className="flex items-center justify-between gap-3">
                          <div>
                            <p className="text-sm font-medium text-white">
                              {session.userAgent || t("security.unknownDevice")}
                              {session.isCurrent ? (
                                <span className="ml-2 rounded-full bg-[#facc15]/15 px-2 py-0.5 text-xs text-[#facc15]">{t("security.current")}</span>
                              ) : null}
                            </p>
                            <p className={MUTED_TEXT_CLASS}>{session.ipAddress || "—"}</p>
                          </div>
                          <span className="text-xs font-medium text-[#d1d5db]">{statusLabel(effectiveSessionStatus(session))}</span>
                        </div>
                        <div className={`mt-2 grid gap-1 ${MUTED_TEXT_CLASS}`}>
                          <p>{t("security.sessionStarted").replace("{time}", formatDateTime(session.createdAt))}</p>
                          <p>{t("security.sessionExpires").replace("{time}", formatDateTime(session.expiresAt))}</p>
                          <p>{t("security.sessionDuration").replace("{time}", formatElapsed(sessionDurationSeconds(session)))}</p>
                          {effectiveSessionStatus(session) === "active" ? (
                            <p>{t("security.sessionRemaining").replace("{time}", formatElapsed(sessionRemainingSeconds(session)))}</p>
                          ) : null}
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className={`text-sm ${MUTED_TEXT_CLASS}`}>{t("security.noSessions")}</p>
                )}
                <Button
                  variant="destructive"
                  className="w-full"
                  data-testid="button-terminate-sessions"
                  disabled={revokeOtherSessions.isPending || !hasOtherSessionsToTerminate}
                  onClick={handleTerminateSessions}
                >
                  {t("security.terminateOthers")}
                </Button>
              </div>
            </TabsContent>

            <TabsContent value="notifications" className="space-y-4">
              <div className="max-w-xl space-y-4">
                {showNotificationSettingsLoadingState ? (
                  <div className="space-y-2">
                    <Skeleton className="h-10 w-full rounded-lg" />
                    <Skeleton className="h-10 w-full rounded-lg" />
                  </div>
                ) : null}
                <div className={`${SUBPANEL_CLASS} flex items-center justify-between px-4 py-3`}>
                  <div>
                    <p className="text-sm font-medium text-white">{t("security.notifications.deliveryEnabled")}</p>
                    <p className={MUTED_TEXT_CLASS}>{t("security.notifications.deliveryEnabledHint")}</p>
                  </div>
                  <Checkbox
                    checked={deliveryEnabled}
                    onCheckedChange={(checked) => setDeliveryEnabled(checked === true)}
                    data-testid="checkbox-delivery-enabled"
                  />
                </div>

                <div className="space-y-2">
                  <Label className="text-[#d1d5db]">{t("security.notifications.channel")}</Label>
                  <Tabs
                    value={deliveryChannel}
                    onValueChange={(value) => {
                      if (value === "telegram" || value === "email" || value === "time") {
                        setDeliveryChannel(value);
                        return;
                      }
                      setDeliveryChannel("in_app");
                    }}
                  >
                    <TabsList className={DELIVERY_CHANNEL_LIST_CLASS}>
                      <TabsTrigger
                        className={DELIVERY_CHANNEL_TRIGGER_CLASS}
                        value="in_app"
                        data-testid="tab-delivery-in-app"
                      >
                        {t("security.notifications.channelInApp")}
                      </TabsTrigger>
                      <TabsTrigger
                        className={DELIVERY_CHANNEL_TRIGGER_CLASS}
                        value="telegram"
                        data-testid="tab-delivery-telegram"
                      >
                        {t("security.notifications.channelTelegram")}
                      </TabsTrigger>
                      <TabsTrigger
                        className={DELIVERY_CHANNEL_TRIGGER_CLASS}
                        value="email"
                        data-testid="tab-delivery-email"
                      >
                        {t("security.notifications.channelEmail")}
                      </TabsTrigger>
                      <TabsTrigger
                        className={DELIVERY_CHANNEL_TRIGGER_CLASS}
                        value="time"
                        data-testid="tab-delivery-time"
                      >
                        {t("security.notifications.channelTime")}
                      </TabsTrigger>
                    </TabsList>
                  </Tabs>
                </div>

                {deliveryChannel === "telegram" ? (
                  <div className={`${SUBPANEL_CLASS} space-y-4 p-4`}>
                    <div className="space-y-2">
                      <Label className="text-[#d1d5db]">{t("security.notifications.bot")}</Label>
                      <Select value={telegramBotID} onValueChange={setTelegramBotID} disabled={notificationBotsLoading}>
                        <SelectTrigger
                          className={`h-10 w-full rounded-md px-3 text-sm ${INPUT_CLASS}`}
                          data-testid="select-notification-bot"
                        >
                          <SelectValue placeholder={t("security.notifications.botPlaceholder")} />
                        </SelectTrigger>
                        <SelectContent className="rounded-xl border border-[#2a2c3c] bg-[#0b0c10] text-[#f3f4f6]">
                          {notificationBots.map((bot: any) => (
                            <SelectItem key={bot.id} value={bot.id}>
                              {bot.name} (@{bot.botUsername})
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2">
                      <Label className="text-[#d1d5db]">{t("security.notifications.chatId")}</Label>
                      <Input
                        className={INPUT_CLASS}
                        value={telegramChatID}
                        onChange={(e) => setTelegramChatID(e.target.value)}
                        placeholder={t("security.notifications.chatIdPlaceholder")}
                        data-testid="input-telegram-chat-id"
                      />
                    </div>
                    <div className="space-y-2">
                      <Label className="text-[#d1d5db]">{t("security.notifications.username")}</Label>
                      <Input
                        className={INPUT_CLASS}
                        value={telegramUsername}
                        onChange={(e) => setTelegramUsername(e.target.value)}
                        placeholder={t("security.notifications.usernamePlaceholder")}
                        data-testid="input-telegram-username"
                      />
                    </div>
                    <p className={MUTED_TEXT_CLASS}>{t("security.notifications.recipientHint")}</p>
                  </div>
                ) : deliveryChannel === "email" ? (
                  <div className={`${SUBPANEL_CLASS} space-y-4 p-4`}>
                    <div className="space-y-2">
                      <Label className="text-[#d1d5db]">{t("security.notifications.email")}</Label>
                      <Input
                        className={INPUT_CLASS}
                        value={notificationEmail}
                        onChange={(e) => setNotificationEmail(e.target.value)}
                        placeholder={t("security.notifications.emailPlaceholder")}
                        data-testid="input-notification-email"
                      />
                    </div>
                    <p className={MUTED_TEXT_CLASS}>{t("security.notifications.emailHint")}</p>
                  </div>
                ) : deliveryChannel === "time" ? (
                  <div className={`${SUBPANEL_CLASS} space-y-4 p-4`}>
                    <div className="space-y-2">
                      <Label className="text-[#d1d5db]">{t("security.notifications.timeRecipient")}</Label>
                      <Input
                        className={INPUT_CLASS}
                        value={timeRecipient}
                        onChange={(e) => setTimeRecipient(e.target.value)}
                        placeholder={t("security.notifications.timeRecipientPlaceholder")}
                        data-testid="input-time-recipient"
                      />
                    </div>
                    <p className={MUTED_TEXT_CLASS}>{t("security.notifications.timeHint")}</p>
                  </div>
                ) : (
                  <p className={MUTED_TEXT_CLASS}>
                    {t("security.notifications.inAppHint")}
                  </p>
                )}

                <div className="flex justify-end">
                  <Button
                    onClick={handleSaveNotificationSettings}
                    disabled={saveMyNotificationSettings.isPending}
                    data-testid="button-save-notification-settings"
                  >
                    <Save size={14} className="mr-1" /> {t("security.notifications.save")}
                  </Button>
                </div>
              </div>
            </TabsContent>
          </Tabs>
        </Card>
      </div>
    </AppLayout>
  );
}
