import { FormEvent, useState } from "react";
import { useLocation } from "wouter";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { useTheme } from "@/lib/theme";
import { useI18n, useT } from "@/lib/i18n";
import { login, useAppState } from "@/lib/api";
import { Globe } from "lucide-react";
import { withTenantPath } from "@/lib/tenant-url";

const PANEL_CLASS =
  "w-full max-w-[440px] rounded-2xl border border-[rgba(255,255,255,0.08)] bg-[linear-gradient(180deg,rgba(18,20,30,0.98),rgba(14,17,27,0.98))] p-6 md:p-8 shadow-[0_24px_70px_rgba(0,0,0,0.46)]";
const INPUT_CLASS =
  "h-11 border-[#2a2c3c] bg-[#0b0c10] text-[#f3f4f6] placeholder:text-[#6b7280] focus-visible:ring-1 focus-visible:ring-[#3b4a79]";
const MUTED_TEXT_CLASS = "text-xs text-[#8b91a3]";
const CONTROL_BUTTON_CLASS = "h-9 border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]";

export default function LoginPage() {
  const [, setLocation] = useLocation();
  const currentTenantSlug = useAppState((state) => state.currentTenantSlug);
  const { theme, setTheme } = useTheme();
  const { language, setLanguage } = useI18n();
  const t = useT();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await login(email, password);
      setLocation(withTenantPath(useAppState.getState().currentTenantSlug || currentTenantSlug, "/dashboard"), { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : t("login.failed"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden bg-[#070910] p-6">
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(1100px_650px_at_80%_-20%,rgba(58,90,255,0.22),transparent_60%),radial-gradient(900px_550px_at_8%_115%,rgba(250,204,21,0.14),transparent_62%)]" />
      <Card className={PANEL_CLASS}>
        <div className="mb-6 space-y-1">
          <h1 className="text-[30px] font-semibold tracking-[-0.4px] text-white">{t("layout.brand")}</h1>
          <p className={MUTED_TEXT_CLASS}>{t("login.subtitle")}</p>
        </div>

        <div className="mb-5 flex flex-wrap items-center gap-2">
          <Button
            variant={theme === "light" ? "default" : "outline"}
            size="sm"
            className={theme === "light" ? "h-9" : CONTROL_BUTTON_CLASS}
            onClick={() => setTheme("light")}
          >
            {t("login.light")}
          </Button>
          <Button
            variant={theme === "dark" ? "default" : "outline"}
            size="sm"
            className={theme === "dark" ? "h-9" : CONTROL_BUTTON_CLASS}
            onClick={() => setTheme("dark")}
          >
            {t("login.dark")}
          </Button>
          <Button
            variant="outline"
            size="sm"
            className={`ml-auto ${CONTROL_BUTTON_CLASS}`}
            onClick={() => setLanguage(language === "en" ? "ru" : "en")}
          >
            <Globe size={14} className="mr-2" />
            {t("lang.switch")}: {language.toUpperCase()}
          </Button>
        </div>

        <form className="space-y-3" onSubmit={submit}>
          <div className="space-y-1">
            <label className={MUTED_TEXT_CLASS} htmlFor="email">{t("login.email")}</label>
            <Input
              id="email"
              className={INPUT_CLASS}
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              data-testid="login-email"
            />
          </div>
          <div className="space-y-1">
            <label className={MUTED_TEXT_CLASS} htmlFor="password">{t("login.password")}</label>
            <Input
              id="password"
              className={INPUT_CLASS}
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              data-testid="login-password"
            />
          </div>

          {error && (
            <div className="rounded-lg border border-[rgba(239,68,68,0.38)] bg-[rgba(220,38,38,0.16)] px-3 py-2 text-xs text-[#fca5a5]">
              {error}
            </div>
          )}

          <Button className="h-11 w-full" disabled={loading} type="submit" data-testid="login-submit">
            {loading ? t("login.signingIn") : t("login.signIn")}
          </Button>
        </form>
      </Card>
    </div>
  );
}
