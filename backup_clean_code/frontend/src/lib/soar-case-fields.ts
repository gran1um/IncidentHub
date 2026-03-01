export type SOARCaseFieldPreset = {
  key: string;
};

export const SOAR_CASE_FIELD_PRESETS: SOARCaseFieldPreset[] = [
  { key: "analyst" },
  { key: "indicators" },
  { key: "status" },
  { key: "stages" },
  { key: "description" },
  { key: "inbound_event" },
  { key: "criticality" },
  { key: "role_types" },
  { key: "closure_required_approvals" },
  { key: "closure_approver_ids" },
];

export function normalizeSOARCaseFieldKey(value: string): string {
  return String(value || "").trim().toLowerCase().replace(/\s+/g, "_");
}

export function mergeMissingSOARCaseFields<Row extends { key: string }>(
  rows: Row[],
  createRow: (preset: SOARCaseFieldPreset) => Row,
  normalizeKey: (value: string) => string = normalizeSOARCaseFieldKey,
): Row[] {
  const nextRows = [...rows];
  const seen = new Set(nextRows.map((row) => normalizeKey(row.key)).filter(Boolean));
  for (const preset of SOAR_CASE_FIELD_PRESETS) {
    if (seen.has(preset.key)) {
      continue;
    }
    nextRows.push(createRow(preset));
    seen.add(preset.key);
  }
  return nextRows;
}

export function isSOARCaseFieldKey(value: string): boolean {
  const normalized = normalizeSOARCaseFieldKey(value);
  return SOAR_CASE_FIELD_PRESETS.some((preset) => preset.key === normalized);
}
