export type ChartPeriod = "week" | "month";

type SeverityKey = "Critical" | "High" | "Medium" | "Low";

interface ResolutionChartOptions {
  locale?: string;
  weekPrefix?: string;
}

export function formatAvgResponseMinutes(value?: number | null): string {
  if (!value || Number.isNaN(value)) {
    return "0.00m";
  }
  return `${value.toFixed(2)}m`;
}

export function getOnDutyUserIDs(shifts: any[], now: Date): string[] {
  if (!Array.isArray(shifts) || shifts.length === 0) {
    return [];
  }

  const active = new Set<string>();
  for (const shift of shifts) {
    const analystID = (shift?.analystId || shift?.analyst_id || "").toString();
    if (!analystID) {
      continue;
    }
    if (isShiftActiveAt(shift, now)) {
      active.add(analystID);
    }
  }
  return Array.from(active);
}

export function buildResolutionBySeverity(
  cases: any[],
  period: ChartPeriod,
  now: Date,
  options?: ResolutionChartOptions,
): Array<Record<string, number | string>> {
  const locale = options?.locale || "en-US";
  const weekPrefix = options?.weekPrefix || "W";
  if (period === "month") {
    return buildMonthlyResolution(cases, now, weekPrefix);
  }
  return buildWeeklyResolution(cases, now, locale);
}

export function isShiftActiveAt(shift: any, now: Date): boolean {
  const dayRaw = Number(shift?.day);
  const monthRaw = Number(shift?.month);
  const yearRaw = Number(shift?.year);
  const startRaw = (shift?.start || "08:00").toString();
  const endRaw = (shift?.end || "16:00").toString();

  if (!Number.isFinite(dayRaw) || !Number.isFinite(monthRaw) || !Number.isFinite(yearRaw)) {
    return false;
  }

  const [startHour, startMinute] = parseTime(startRaw);
  const [endHour, endMinute] = parseTime(endRaw);

  const start = new Date(yearRaw, monthRaw - 1, dayRaw, startHour, startMinute, 0, 0);
  const end = new Date(yearRaw, monthRaw - 1, dayRaw, endHour, endMinute, 0, 0);
  if (end <= start) {
    end.setDate(end.getDate() + 1);
  }

  return now >= start && now <= end;
}

function buildWeeklyResolution(cases: any[], now: Date, locale: string): Array<Record<string, number | string>> {
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const weekday = startOfToday.getDay();
  const mondayOffset = (weekday + 6) % 7;
  const startOfWeek = new Date(startOfToday);
  startOfWeek.setDate(startOfToday.getDate() - mondayOffset);
  const points: Array<Record<string, number | string>> = [];
  const byDay = new Map<string, Record<string, number | string>>();

  for (let i = 0; i < 7; i++) {
    const d = new Date(startOfWeek);
    d.setDate(startOfWeek.getDate() + i);
    const key = dateKey(d);
    const point: Record<string, number | string> = {
      name: d.toLocaleDateString(locale, { weekday: "short" }),
      Critical: 0,
      High: 0,
      Medium: 0,
      Low: 0,
    };
    byDay.set(key, point);
    points.push(point);
  }

  for (const item of cases || []) {
    if (!isResolvedStatus(item?.status)) {
      continue;
    }
    const dt = parseCaseDate(item);
    if (!dt) {
      continue;
    }
    const key = dateKey(dt);
    const bucket = byDay.get(key);
    if (!bucket) {
      continue;
    }
    const sev = normalizeSeverity(item?.sev || item?.severity);
    bucket[sev] = Number(bucket[sev] || 0) + 1;
  }

  return points;
}

function buildMonthlyResolution(cases: any[], now: Date, weekPrefix: string): Array<Record<string, number | string>> {
  const year = now.getFullYear();
  const month = now.getMonth();
  const daysInMonth = new Date(year, month + 1, 0).getDate();
  const weekCount = Math.ceil(daysInMonth / 7);

  const points: Array<Record<string, number | string>> = [];
  for (let i = 1; i <= weekCount; i++) {
    points.push({ name: `${weekPrefix}${i}`, Critical: 0, High: 0, Medium: 0, Low: 0 });
  }

  for (const item of cases || []) {
    if (!isResolvedStatus(item?.status)) {
      continue;
    }
    const dt = parseCaseDate(item);
    if (!dt) {
      continue;
    }
    if (dt.getFullYear() !== year || dt.getMonth() !== month) {
      continue;
    }

    const weekIndex = Math.floor((dt.getDate() - 1) / 7);
    const bucket = points[weekIndex];
    if (!bucket) {
      continue;
    }
    const sev = normalizeSeverity(item?.sev || item?.severity);
    bucket[sev] = Number(bucket[sev] || 0) + 1;
  }

  return points;
}

function parseTime(raw: string): [number, number] {
  const [h, m] = raw.split(":").map((v) => Number(v));
  const hour = Number.isFinite(h) ? h : 0;
  const minute = Number.isFinite(m) ? m : 0;
  return [hour, minute];
}

function parseCaseDate(item: any): Date | null {
  const raw = item?.updatedAt || item?.updated_at || item?.time || item?.createdAt || item?.created_at;
  if (!raw) {
    return null;
  }
  const dt = new Date(raw);
  if (Number.isNaN(dt.getTime())) {
    return null;
  }
  return dt;
}

function dateKey(dt: Date): string {
  return `${dt.getFullYear()}-${String(dt.getMonth() + 1).padStart(2, "0")}-${String(dt.getDate()).padStart(2, "0")}`;
}

function isResolvedStatus(status: string): boolean {
  const s = (status || "").toString().toLowerCase();
  return s === "resolved" || s === "closed";
}

function normalizeSeverity(input: string): SeverityKey {
  const s = (input || "").toString().toLowerCase();
  if (s === "critical") return "Critical";
  if (s === "high") return "High";
  if (s === "low") return "Low";
  return "Medium";
}
