export type AchievementIconSource = "preset" | "uploaded" | "existing" | "emoji";

export type AchievementIconOption = {
  value: string;
  label: string;
  preview: string;
  source: AchievementIconSource;
};

export const ACHIEVEMENT_ICON_UPLOAD_CRITERIA = {
  maxBytes: 2 * 1024 * 1024,
  minDimensionPx: 64,
  maxDimensionPx: 512,
  squareRequired: true,
  allowedTypes: ["image/png", "image/jpeg", "image/svg+xml"] as const,
};

const PRESET_ROOT = "/achievement-icons";

export const ACHIEVEMENT_ICON_PRESETS: AchievementIconOption[] = [
  { value: `${PRESET_ROOT}/first-responder.svg`, label: "First Responder", preview: `${PRESET_ROOT}/first-responder.svg`, source: "preset" },
  { value: `${PRESET_ROOT}/night-watch.svg`, label: "Night Watch", preview: `${PRESET_ROOT}/night-watch.svg`, source: "preset" },
  { value: `${PRESET_ROOT}/ioc-hunter.svg`, label: "IOC Hunter", preview: `${PRESET_ROOT}/ioc-hunter.svg`, source: "preset" },
  { value: `${PRESET_ROOT}/containment-master.svg`, label: "Containment Master", preview: `${PRESET_ROOT}/containment-master.svg`, source: "preset" },
  { value: `${PRESET_ROOT}/intel-curator.svg`, label: "Intel Curator", preview: `${PRESET_ROOT}/intel-curator.svg`, source: "preset" },
  { value: `${PRESET_ROOT}/soc-architect.svg`, label: "SOC Architect", preview: `${PRESET_ROOT}/soc-architect.svg`, source: "preset" },
  { value: `${PRESET_ROOT}/incident-commander.svg`, label: "Incident Commander", preview: `${PRESET_ROOT}/incident-commander.svg`, source: "preset" },
  { value: `${PRESET_ROOT}/zero-day-sentinel.svg`, label: "Zero-Day Sentinel", preview: `${PRESET_ROOT}/zero-day-sentinel.svg`, source: "preset" },
  { value: `${PRESET_ROOT}/playbook-automator.svg`, label: "Playbook Automator", preview: `${PRESET_ROOT}/playbook-automator.svg`, source: "preset" },
  { value: `${PRESET_ROOT}/telemetry-guardian.svg`, label: "Telemetry Guardian", preview: `${PRESET_ROOT}/telemetry-guardian.svg`, source: "preset" },
];

export const ACHIEVEMENT_ICON_EMOJI_OPTIONS: AchievementIconOption[] = [
  { value: "🏆", label: "Trophy", preview: "🏆", source: "emoji" },
  { value: "🛡️", label: "Shield", preview: "🛡️", source: "emoji" },
  { value: "💎", label: "Diamond", preview: "💎", source: "emoji" },
];

export const DEFAULT_ACHIEVEMENT_ICON = ACHIEVEMENT_ICON_PRESETS[0]?.value || "🏆";

export function isAchievementImageIcon(value: unknown): boolean {
  const normalized = String(value || "").trim().toLowerCase();
  if (!normalized) {
    return false;
  }
  return normalized.startsWith("http://")
    || normalized.startsWith("https://")
    || normalized.startsWith("/")
    || normalized.startsWith("data:image/")
    || normalized.startsWith("blob:");
}

export function upsertAchievementIconOption(
  options: AchievementIconOption[],
  incoming: AchievementIconOption,
): AchievementIconOption[] {
  const deduped = options.filter((item) => item.value !== incoming.value);
  return [incoming, ...deduped];
}
