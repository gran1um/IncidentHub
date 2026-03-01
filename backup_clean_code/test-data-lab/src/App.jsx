import React, { useEffect, useMemo, useState } from "react";

const STORAGE_KEY = "incidenthub_test_data_lab_settings_v1";

const CASE_SEVERITIES = ["critical", "high", "medium", "low"];
const ALERT_SEVERITIES = ["critical", "high", "medium", "low"];
const CASE_PRIORITIES = ["critical", "high", "medium", "low"];
const CASE_STATUSES = ["open", "in_progress", "resolved", "closed"];
const ALERT_STATUSES = ["new", "triaged", "closed"];
const TLP_VALUES = ["red", "amber", "green", "clear"];
const PAP_VALUES = ["red", "amber", "green", "clear"];
const OBSERVABLE_TYPES = ["ip", "domain", "url", "hash", "email", "user_agent"];
const OBSERVABLE_VERDICTS = ["unknown", "benign", "suspicious", "malicious"];

const CASE_AI_SCHEMA = {
  case: {
    case_number: "CASE-0000",
    title: "",
    description: "",
    source: "manual",
    incident_type: "",
    status: "open",
    priority: "high",
    impact: "medium",
    confidence: 65,
    severity: "high",
    tlp: "amber",
    pap: "amber",
    detected_at: "2026-03-05T12:00:00Z",
    occurred_at: "2026-03-05T11:40:00Z",
    resolution_summary: "",
    assigned_to: "",
  },
  observables: [
    {
      type: "ip",
      value: "185.XX.XX.XX",
      verdict: "suspicious",
      source: "manual",
      tags: ["ioc", "phishing"],
    },
  ],
};

const ALERT_AI_SCHEMA = {
  alert: {
    title: "",
    description: "",
    source: "siem",
    status: "new",
    severity: "high",
    tlp: "amber",
    pap: "amber",
  },
};

const BULK_AI_SCHEMA = {
  items: [
    {
      case: CASE_AI_SCHEMA.case,
      observables: CASE_AI_SCHEMA.observables,
    },
  ],
};

const envDefaults = {
  apiBaseUrl: String(import.meta.env.VITE_TEST_DATA_LAB_DEFAULT_API_BASE_URL || "").trim(),
  tenantId: String(import.meta.env.VITE_TEST_DATA_LAB_DEFAULT_TENANT_ID || "").trim(),
  loginEmail: String(import.meta.env.VITE_TEST_DATA_LAB_DEFAULT_LOGIN_EMAIL || "").trim(),
  loginPassword: String(import.meta.env.VITE_TEST_DATA_LAB_DEFAULT_LOGIN_PASSWORD || ""),
  language: String(import.meta.env.VITE_TEST_DATA_LAB_DEFAULT_LANGUAGE || "ru").trim() || "ru",
};

const defaultSettings = {
  apiBaseUrl: envDefaults.apiBaseUrl,
  tenantId: envDefaults.tenantId,
  accessToken: "",
  language: envDefaults.language,
  loginEmail: envDefaults.loginEmail,
  loginPassword: envDefaults.loginPassword,
};

const defaultCaseDraft = {
  case_number: "",
  title: "",
  description: "",
  source: "manual",
  incident_type: "Credential Access",
  status: "open",
  priority: "high",
  impact: "medium",
  confidence: 65,
  severity: "high",
  tlp: "amber",
  pap: "amber",
  detected_at: "",
  occurred_at: "",
  resolution_summary: "",
  assigned_to: "",
  observables: [createObservableDraft()],
  aiContext: "",
};

const defaultAlertDraft = {
  title: "",
  description: "",
  source: "siem",
  status: "new",
  severity: "high",
  tlp: "amber",
  pap: "amber",
  aiContext: "",
};

const defaultBulkDraft = {
  entity: "case",
  count: 5,
  scenario: "Фишинговая атака на сотрудников финансового отдела",
  useAI: true,
  includeObservables: true,
};

function createObservableDraft() {
  return {
    type: "ip",
    value: "",
    verdict: "unknown",
    source: "manual",
    tags: "",
  };
}

function loadSettings() {
  try {
    const parsed = JSON.parse(localStorage.getItem(STORAGE_KEY) || "{}");
    return { ...defaultSettings, ...parsed };
  } catch {
    return { ...defaultSettings };
  }
}

function sleep(ms) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}

function parseTagString(raw) {
  return String(raw || "")
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function normalizeSeverity(raw, allowed) {
  const normalized = String(raw || "").trim().toLowerCase();
  return allowed.includes(normalized) ? normalized : allowed[0];
}

function normalizeDateString(raw) {
  const value = String(raw || "").trim();
  if (!value) return "";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "";
  return parsed.toISOString();
}

function nowLabel() {
  return new Date().toLocaleTimeString("ru-RU", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function extractJSONFromText(text) {
  const raw = String(text || "").trim();
  if (!raw) {
    throw new Error("ИИ вернул пустой ответ");
  }

  const fencedJSON = raw.match(/```json\s*([\s\S]*?)```/i);
  if (fencedJSON?.[1]) {
    return JSON.parse(fencedJSON[1].trim());
  }

  const fencedAny = raw.match(/```\s*([\s\S]*?)```/i);
  if (fencedAny?.[1]) {
    try {
      return JSON.parse(fencedAny[1].trim());
    } catch {
      // continue
    }
  }

  try {
    return JSON.parse(raw);
  } catch {
    // continue
  }

  for (let i = 0; i < raw.length; i += 1) {
    const first = raw[i];
    if (first !== "{" && first !== "[") {
      continue;
    }
    const candidate = readBalancedJSONCandidate(raw, i);
    if (!candidate) {
      continue;
    }
    try {
      return JSON.parse(candidate);
    } catch {
      // continue
    }
  }

  throw new Error("Не удалось извлечь JSON из ответа ИИ");
}

function readBalancedJSONCandidate(text, start) {
  const stack = [];
  let inString = false;
  let escaping = false;

  for (let i = start; i < text.length; i += 1) {
    const char = text[i];

    if (inString) {
      if (escaping) {
        escaping = false;
      } else if (char === "\\") {
        escaping = true;
      } else if (char === '"') {
        inString = false;
      }
      continue;
    }

    if (char === '"') {
      inString = true;
      continue;
    }

    if (char === "{") stack.push("}");
    if (char === "[") stack.push("]");

    if (char === "}" || char === "]") {
      if (stack.length === 0) {
        return null;
      }
      const expected = stack.pop();
      if (expected !== char) {
        return null;
      }
      if (stack.length === 0) {
        return text.slice(start, i + 1);
      }
    }
  }

  return null;
}

function randomFrom(list) {
  return list[Math.floor(Math.random() * list.length)];
}

function buildLocalCaseSeed(index, scenario) {
  const suffix = `${Date.now().toString(36)}-${index + 1}`;
  const incidentType = randomFrom([
    "Credential Access",
    "Data Exfiltration",
    "Suspicious Login",
    "Malware Activity",
    "Unauthorized Access",
  ]);
  const severity = randomFrom(CASE_SEVERITIES);
  const priority = severity;
  const source = randomFrom(["siem", "edr", "manual", "ids"]);
  const title = `${scenario || "Security incident"} #${index + 1}`;

  return {
    case: {
      case_number: "",
      title,
      description: `Автогенерация тестового кейса: ${title}. Источник: ${source}.`,
      source,
      incident_type: incidentType,
      status: "open",
      priority,
      impact: severity,
      confidence: 60 + (index % 30),
      severity,
      tlp: "amber",
      pap: "amber",
      detected_at: new Date().toISOString(),
      occurred_at: new Date(Date.now() - (index + 1) * 10 * 60 * 1000).toISOString(),
      resolution_summary: "",
      assigned_to: "",
    },
    observables: [
      {
        type: "ip",
        value: `185.12.${(index % 200) + 10}.${(index % 240) + 10}`,
        verdict: severity === "critical" ? "malicious" : "suspicious",
        source: "generated",
        tags: ["autogen", incidentType.toLowerCase().replace(/\s+/g, "_")],
      },
      {
        type: "domain",
        value: `campaign-${suffix}.example`,
        verdict: "suspicious",
        source: "generated",
        tags: ["autogen"],
      },
    ],
  };
}

function buildLocalAlertSeed(index, scenario) {
  const source = randomFrom(["siem", "waf", "ids", "edr"]);
  const severity = randomFrom(ALERT_SEVERITIES);
  return {
    title: `${scenario || "Security alert"} #${index + 1}`,
    description: `Автогенерация тестового алерта #${index + 1}. Источник ${source}.`,
    source,
    status: "new",
    severity,
    tlp: "amber",
    pap: "amber",
  };
}

export default function App() {
  const [settings, setSettings] = useState(loadSettings);
  const [activeTab, setActiveTab] = useState("case");
  const [caseDraft, setCaseDraft] = useState({ ...defaultCaseDraft });
  const [alertDraft, setAlertDraft] = useState({ ...defaultAlertDraft });
  const [bulkDraft, setBulkDraft] = useState({ ...defaultBulkDraft });
  const [logs, setLogs] = useState([]);
  const [createdItems, setCreatedItems] = useState([]);
  const [busy, setBusy] = useState(false);
  const [bulkProgress, setBulkProgress] = useState(0);

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
  }, [settings]);

  const canSendRequests = Boolean(settings.tenantId.trim());

  const apiSummary = useMemo(() => {
    const base = settings.apiBaseUrl.trim();
    return base || "через Vite proxy (/api -> backend)";
  }, [settings.apiBaseUrl]);

  function log(message, level = "info") {
    setLogs((prev) => [{ id: crypto.randomUUID(), ts: nowLabel(), level, message }, ...prev].slice(0, 120));
  }

  function resolveURL(path) {
    const base = settings.apiBaseUrl.trim();
    if (!base) {
      return path;
    }
    return `${base.replace(/\/$/, "")}${path}`;
  }

  async function apiRequest(path, options = {}) {
    const method = options.method || "GET";
    const headers = new Headers(options.headers || {});
    const skipAuth = options.skipAuth === true;
    const skipTenant = options.skipTenant === true;

    if (!headers.has("Content-Type") && options.body !== undefined) {
      headers.set("Content-Type", "application/json");
    }
    if (!skipAuth && settings.accessToken.trim()) {
      headers.set("Authorization", `Bearer ${settings.accessToken.trim()}`);
    }
    if (!skipTenant && settings.tenantId.trim()) {
      headers.set("X-Tenant-ID", settings.tenantId.trim());
    }

    const response = await fetch(resolveURL(path), {
      method,
      headers,
      body: options.body !== undefined ? JSON.stringify(options.body) : undefined,
      credentials: "include",
      signal: options.signal,
    });

    const rawText = await response.text();
    let payload;
    if (rawText) {
      try {
        payload = JSON.parse(rawText);
      } catch {
        payload = rawText;
      }
    }

    if (!response.ok) {
      const message =
        (typeof payload === "object" && payload && (payload.message || payload.error)) ||
        (typeof payload === "string" ? payload : `HTTP ${response.status}`);
      throw new Error(String(message));
    }

    return payload;
  }

  async function waitForAsyncOperation(operationId) {
    const safeOperationID = String(operationId || "").trim();
    if (!safeOperationID) {
      throw new Error("operation_id отсутствует");
    }

    for (let attempt = 0; attempt < 80; attempt += 1) {
      const payload = await apiRequest(`/api/v1/operations/${encodeURIComponent(safeOperationID)}`);
      const status = String(payload?.status || "").toLowerCase();
      if (status === "done") {
        return payload;
      }
      if (status === "failed") {
        throw new Error(payload?.error || "Асинхронная операция завершилась ошибкой");
      }
      await sleep(750);
    }

    throw new Error("Таймаут ожидания асинхронной операции");
  }

  async function resolveCreatedEntity(createPayload, resourceType) {
    const resourcePlural = resourceType === "case" ? "cases" : "alerts";
    if (createPayload?.id) {
      return createPayload;
    }

    if (createPayload?.operation_id) {
      const operation = await waitForAsyncOperation(createPayload.operation_id);
      const resourceId = String(operation?.resource_id || createPayload?.resource_id || "").trim();
      if (!resourceId) {
        return operation;
      }
      return apiRequest(`/api/v1/${resourcePlural}/${encodeURIComponent(resourceId)}`);
    }

    if (createPayload?.resource_id) {
      const resourceId = String(createPayload.resource_id || "").trim();
      if (resourceId) {
        return apiRequest(`/api/v1/${resourcePlural}/${encodeURIComponent(resourceId)}`);
      }
    }

    return createPayload;
  }

  async function askAI(question) {
    const response = await apiRequest("/api/v1/ai/ask", {
      method: "POST",
      body: {
        question,
        language: settings.language || "ru",
      },
    });

    const answerText =
      String(response?.answer || "").trim() ||
      String(response?.assistant_message?.content || "").trim();

    if (!answerText) {
      throw new Error("ИИ не вернул текст ответа");
    }

    return {
      raw: answerText,
      parsed: extractJSONFromText(answerText),
      recommendations: Array.isArray(response?.recommendations) ? response.recommendations : [],
      model: String(response?.model || "").trim(),
    };
  }

  async function handleLogin(event) {
    event.preventDefault();
    if (busy) return;
    const email = settings.loginEmail.trim();
    const password = settings.loginPassword;
    if (!email || !password) {
      log("Для логина укажи email и пароль", "error");
      return;
    }

    setBusy(true);
    try {
      const payload = await apiRequest("/api/v1/auth/login", {
        method: "POST",
        body: { email, password },
        skipAuth: true,
        skipTenant: true,
      });

      const token = String(payload?.access_token || payload?.accessToken || "").trim();
      const tenantFromIdentity = String(payload?.identity?.tenant_id || payload?.identity?.tenantId || "").trim();
      const tenantFromMembership = String(payload?.memberships?.[0]?.tenant_id || "").trim();
      const tenantId = tenantFromIdentity || tenantFromMembership || settings.tenantId;

      setSettings((prev) => ({
        ...prev,
        accessToken: token,
        tenantId,
      }));
      log(`Логин успешен. tenant=${tenantId || "не найден"}`, "success");
    } catch (error) {
      log(`Ошибка логина: ${error.message}`, "error");
    } finally {
      setBusy(false);
    }
  }

  async function fillCaseByAI() {
    if (busy) return;
    if (!canSendRequests) {
      log("Укажи tenant_id перед запросом к ИИ", "error");
      return;
    }
    setBusy(true);
    try {
      const context = caseDraft.aiContext.trim() || caseDraft.title.trim() || "Нужен реалистичный кейс по ИБ";
      const question = [
        "Ты SOC-помощник. Сгенерируй тестовый incident case.",
        "Верни строго JSON, без markdown и без комментариев.",
        "Соблюдай схему и используй только значения перечислений.",
        `Схема:\n${JSON.stringify(CASE_AI_SCHEMA, null, 2)}`,
        `Контекст:\n${context}`,
      ].join("\n\n");

      const ai = await askAI(question);
      const payload = ai.parsed?.case ? ai.parsed : { case: ai.parsed, observables: [] };
      const nextCase = payload.case || {};
      const nextObservables = Array.isArray(payload.observables) ? payload.observables : [];

      setCaseDraft((prev) => ({
        ...prev,
        case_number: String(nextCase.case_number || prev.case_number || ""),
        title: String(nextCase.title || prev.title || ""),
        description: String(nextCase.description || prev.description || ""),
        source: String(nextCase.source || prev.source || "manual"),
        incident_type: String(nextCase.incident_type || prev.incident_type || "Credential Access"),
        status: normalizeSeverity(nextCase.status || prev.status, CASE_STATUSES),
        priority: normalizeSeverity(nextCase.priority || prev.priority, CASE_PRIORITIES),
        impact: String(nextCase.impact || prev.impact || "medium"),
        confidence: Number.isFinite(Number(nextCase.confidence)) ? Number(nextCase.confidence) : prev.confidence,
        severity: normalizeSeverity(nextCase.severity || prev.severity, CASE_SEVERITIES),
        tlp: normalizeSeverity(nextCase.tlp || prev.tlp, TLP_VALUES),
        pap: normalizeSeverity(nextCase.pap || prev.pap, PAP_VALUES),
        detected_at: String(nextCase.detected_at || ""),
        occurred_at: String(nextCase.occurred_at || ""),
        resolution_summary: String(nextCase.resolution_summary || prev.resolution_summary || ""),
        assigned_to: String(nextCase.assigned_to || prev.assigned_to || ""),
        observables:
          nextObservables.length > 0
            ? nextObservables.map((item) => ({
                type: normalizeSeverity(item?.type || "ip", OBSERVABLE_TYPES),
                value: String(item?.value || ""),
                verdict: normalizeSeverity(item?.verdict || "unknown", OBSERVABLE_VERDICTS),
                source: String(item?.source || "manual"),
                tags: Array.isArray(item?.tags) ? item.tags.join(", ") : String(item?.tags || ""),
              }))
            : prev.observables,
      }));

      log(`Поля кейса заполнены ИИ${ai.model ? ` (model: ${ai.model})` : ""}`, "success");
    } catch (error) {
      log(`Ошибка AI заполнения кейса: ${error.message}`, "error");
    } finally {
      setBusy(false);
    }
  }

  async function fillAlertByAI() {
    if (busy) return;
    if (!canSendRequests) {
      log("Укажи tenant_id перед запросом к ИИ", "error");
      return;
    }
    setBusy(true);
    try {
      const context = alertDraft.aiContext.trim() || alertDraft.title.trim() || "Нужен тестовый security alert";
      const question = [
        "Ты SOC-помощник. Сгенерируй тестовый алерт.",
        "Верни строго JSON, без markdown и комментариев.",
        `Схема:\n${JSON.stringify(ALERT_AI_SCHEMA, null, 2)}`,
        `Контекст:\n${context}`,
      ].join("\n\n");

      const ai = await askAI(question);
      const payload = ai.parsed?.alert ? ai.parsed.alert : ai.parsed;

      setAlertDraft((prev) => ({
        ...prev,
        title: String(payload?.title || prev.title || ""),
        description: String(payload?.description || prev.description || ""),
        source: String(payload?.source || prev.source || "siem"),
        status: normalizeSeverity(payload?.status || prev.status, ALERT_STATUSES),
        severity: normalizeSeverity(payload?.severity || prev.severity, ALERT_SEVERITIES),
        tlp: normalizeSeverity(payload?.tlp || prev.tlp, TLP_VALUES),
        pap: normalizeSeverity(payload?.pap || prev.pap, PAP_VALUES),
      }));

      log(`Поля алерта заполнены ИИ${ai.model ? ` (model: ${ai.model})` : ""}`, "success");
    } catch (error) {
      log(`Ошибка AI заполнения алерта: ${error.message}`, "error");
    } finally {
      setBusy(false);
    }
  }

  async function createCaseFromDraft(draft) {
    const payload = {
      case_number: String(draft.case_number || "").trim(),
      title: String(draft.title || "").trim(),
      description: String(draft.description || "").trim(),
      source: String(draft.source || "manual").trim(),
      incident_type: String(draft.incident_type || "").trim(),
      status: normalizeSeverity(draft.status, CASE_STATUSES),
      priority: normalizeSeverity(draft.priority, CASE_PRIORITIES),
      impact: String(draft.impact || "").trim(),
      confidence: Math.max(0, Math.min(100, Number(draft.confidence || 0))),
      severity: normalizeSeverity(draft.severity, CASE_SEVERITIES),
      tlp: normalizeSeverity(draft.tlp, TLP_VALUES),
      pap: normalizeSeverity(draft.pap, PAP_VALUES),
      detected_at: normalizeDateString(draft.detected_at),
      occurred_at: normalizeDateString(draft.occurred_at),
      resolution_summary: String(draft.resolution_summary || "").trim(),
      assigned_to: String(draft.assigned_to || "").trim(),
    };

    if (!payload.title) {
      throw new Error("title обязателен для кейса");
    }

    const created = await apiRequest("/api/v1/cases", {
      method: "POST",
      body: payload,
    });
    const resolved = await resolveCreatedEntity(created, "case");
    const caseId = String(resolved?.id || "").trim();

    if (!caseId) {
      throw new Error("Не удалось определить ID созданного кейса");
    }

    const observableRows = Array.isArray(draft.observables) ? draft.observables : [];
    for (const row of observableRows) {
      const value = String(row?.value || "").trim();
      if (!value) {
        continue;
      }
      await apiRequest(`/api/v1/cases/${encodeURIComponent(caseId)}/observables`, {
        method: "POST",
        body: {
          type: normalizeSeverity(row?.type || "ip", OBSERVABLE_TYPES),
          value,
          verdict: normalizeSeverity(row?.verdict || "unknown", OBSERVABLE_VERDICTS),
          source: String(row?.source || "manual").trim() || "manual",
          tags: parseTagString(row?.tags),
        },
      });
    }

    return {
      id: caseId,
      kind: "case",
      title: payload.title,
    };
  }

  async function createAlertFromDraft(draft) {
    const payload = {
      title: String(draft.title || "").trim(),
      description: String(draft.description || "").trim(),
      source: String(draft.source || "manual").trim() || "manual",
      status: normalizeSeverity(draft.status, ALERT_STATUSES),
      severity: normalizeSeverity(draft.severity, ALERT_SEVERITIES),
      tlp: normalizeSeverity(draft.tlp, TLP_VALUES),
      pap: normalizeSeverity(draft.pap, PAP_VALUES),
    };

    if (!payload.title) {
      throw new Error("title обязателен для алерта");
    }

    const created = await apiRequest("/api/v1/alerts", {
      method: "POST",
      body: payload,
    });
    const resolved = await resolveCreatedEntity(created, "alert");
    const alertId = String(resolved?.id || "").trim();

    if (!alertId) {
      throw new Error("Не удалось определить ID созданного алерта");
    }

    return {
      id: alertId,
      kind: "alert",
      title: payload.title,
    };
  }

  async function handleCreateCase(event) {
    event.preventDefault();
    if (busy) return;
    if (!canSendRequests) {
      log("Сначала укажи tenant_id", "error");
      return;
    }

    setBusy(true);
    try {
      const created = await createCaseFromDraft(caseDraft);
      setCreatedItems((prev) => [{ ...created, createdAt: nowLabel() }, ...prev].slice(0, 100));
      log(`Кейс создан: ${created.id}`, "success");
    } catch (error) {
      log(`Ошибка создания кейса: ${error.message}`, "error");
    } finally {
      setBusy(false);
    }
  }

  async function handleCreateAlert(event) {
    event.preventDefault();
    if (busy) return;
    if (!canSendRequests) {
      log("Сначала укажи tenant_id", "error");
      return;
    }

    setBusy(true);
    try {
      const created = await createAlertFromDraft(alertDraft);
      setCreatedItems((prev) => [{ ...created, createdAt: nowLabel() }, ...prev].slice(0, 100));
      log(`Алерт создан: ${created.id}`, "success");
    } catch (error) {
      log(`Ошибка создания алерта: ${error.message}`, "error");
    } finally {
      setBusy(false);
    }
  }

  async function handleBulkCreate(event) {
    event.preventDefault();
    if (busy) return;
    if (!canSendRequests) {
      log("Сначала укажи tenant_id", "error");
      return;
    }

    const count = Math.max(1, Math.min(40, Number(bulkDraft.count || 1)));
    setBusy(true);
    setBulkProgress(0);

    try {
      let seeds = [];

      if (bulkDraft.entity === "case") {
        if (bulkDraft.useAI) {
          const question = [
            `Сгенерируй ${count} тестовых кейсов SOC.`,
            "Верни строго JSON без markdown и комментариев.",
            "Каждый элемент должен быть реалистичным для ИБ-команды.",
            `Схема:\n${JSON.stringify(BULK_AI_SCHEMA, null, 2)}`,
            `Сценарий:\n${bulkDraft.scenario || "Общий SOC сценарий"}`,
          ].join("\n\n");
          const ai = await askAI(question);
          const items = Array.isArray(ai.parsed?.items) ? ai.parsed.items : [];
          if (items.length === 0) {
            throw new Error("ИИ не вернул items для пакетной генерации кейсов");
          }
          seeds = items.slice(0, count).map((item, index) => ({
            case: item?.case || buildLocalCaseSeed(index, bulkDraft.scenario).case,
            observables: Array.isArray(item?.observables) ? item.observables : [],
          }));
        } else {
          seeds = Array.from({ length: count }).map((_, index) => buildLocalCaseSeed(index, bulkDraft.scenario));
        }

        for (let index = 0; index < seeds.length; index += 1) {
          const seed = seeds[index];
          const created = await createCaseFromDraft({
            ...seed.case,
            observables: bulkDraft.includeObservables
              ? (seed.observables || []).map((item) => ({
                  type: normalizeSeverity(item?.type || "ip", OBSERVABLE_TYPES),
                  value: String(item?.value || ""),
                  verdict: normalizeSeverity(item?.verdict || "unknown", OBSERVABLE_VERDICTS),
                  source: String(item?.source || "generated"),
                  tags: Array.isArray(item?.tags) ? item.tags.join(", ") : String(item?.tags || ""),
                }))
              : [],
          });
          setCreatedItems((prev) => [{ ...created, createdAt: nowLabel() }, ...prev].slice(0, 100));
          setBulkProgress(Math.round(((index + 1) / seeds.length) * 100));
        }
      } else {
        if (bulkDraft.useAI) {
          const question = [
            `Сгенерируй ${count} тестовых алертов SOC.`,
            "Верни строго JSON без markdown и комментариев.",
            `Схема:\n${JSON.stringify({ items: [ALERT_AI_SCHEMA.alert] }, null, 2)}`,
            `Сценарий:\n${bulkDraft.scenario || "Общий SOC сценарий"}`,
          ].join("\n\n");
          const ai = await askAI(question);
          const items = Array.isArray(ai.parsed?.items) ? ai.parsed.items : [];
          if (items.length === 0) {
            throw new Error("ИИ не вернул items для пакетной генерации алертов");
          }
          seeds = items.slice(0, count);
        } else {
          seeds = Array.from({ length: count }).map((_, index) => buildLocalAlertSeed(index, bulkDraft.scenario));
        }

        for (let index = 0; index < seeds.length; index += 1) {
          const seed = seeds[index];
          const created = await createAlertFromDraft(seed);
          setCreatedItems((prev) => [{ ...created, createdAt: nowLabel() }, ...prev].slice(0, 100));
          setBulkProgress(Math.round(((index + 1) / seeds.length) * 100));
        }
      }

      log(`Пакетная генерация завершена: ${count} ${bulkDraft.entity === "case" ? "кейсов" : "алертов"}`, "success");
    } catch (error) {
      log(`Ошибка пакетной генерации: ${error.message}`, "error");
    } finally {
      setBusy(false);
    }
  }

  function updateCaseField(field, value) {
    setCaseDraft((prev) => ({ ...prev, [field]: value }));
  }

  function updateObservable(index, field, value) {
    setCaseDraft((prev) => ({
      ...prev,
      observables: prev.observables.map((item, row) => (row === index ? { ...item, [field]: value } : item)),
    }));
  }

  function removeObservable(index) {
    setCaseDraft((prev) => ({
      ...prev,
      observables: prev.observables.filter((_, row) => row !== index),
    }));
  }

  return (
    <div className="app-shell">
      <header className="app-header">
        <div>
          <h1>IncidentHub Test Data Lab</h1>
          <p>Отдельный фронтенд для генерации тестовых кейсов/алертов с AI-помощью.</p>
        </div>
        <div className="api-hint">API: {apiSummary}</div>
      </header>

      <section className="panel">
        <h2>Подключение</h2>
        <div className="grid two-cols">
          <label>
            Base URL (опционально)
            <input
              value={settings.apiBaseUrl}
              onChange={(event) => setSettings((prev) => ({ ...prev, apiBaseUrl: event.target.value }))}
              placeholder="Оставь пустым для proxy"
            />
          </label>
          <label>
            Tenant ID (обязательно)
            <input
              value={settings.tenantId}
              onChange={(event) => setSettings((prev) => ({ ...prev, tenantId: event.target.value }))}
              placeholder="uuid tenant"
            />
          </label>
          <label>
            Access Token (опционально)
            <input
              value={settings.accessToken}
              onChange={(event) => setSettings((prev) => ({ ...prev, accessToken: event.target.value }))}
              placeholder="Bearer token"
            />
          </label>
          <label>
            Язык AI
            <select
              value={settings.language}
              onChange={(event) => setSettings((prev) => ({ ...prev, language: event.target.value }))}
            >
              <option value="ru">ru</option>
              <option value="en">en</option>
            </select>
          </label>
        </div>

        <form className="inline-form" onSubmit={handleLogin}>
          <label>
            Email
            <input
              value={settings.loginEmail}
              onChange={(event) => setSettings((prev) => ({ ...prev, loginEmail: event.target.value }))}
              placeholder="analyst@example.com"
            />
          </label>
          <label>
            Password
            <input
              type="password"
              value={settings.loginPassword}
              onChange={(event) => setSettings((prev) => ({ ...prev, loginPassword: event.target.value }))}
              placeholder="password"
            />
          </label>
          <button type="submit" disabled={busy}>Login</button>
        </form>
      </section>

      <section className="panel">
        <div className="tabs">
          <button className={activeTab === "case" ? "active" : ""} onClick={() => setActiveTab("case")}>Кейс</button>
          <button className={activeTab === "alert" ? "active" : ""} onClick={() => setActiveTab("alert")}>Алерт</button>
          <button className={activeTab === "bulk" ? "active" : ""} onClick={() => setActiveTab("bulk")}>Пакетно</button>
        </div>

        {activeTab === "case" && (
          <form className="form-grid" onSubmit={handleCreateCase}>
            <div className="actions-row">
              <h2>Создать тестовый кейс</h2>
              <button type="button" onClick={fillCaseByAI} disabled={busy}>Заполнить AI</button>
            </div>
            <label>
              Контекст для AI
              <textarea
                rows={3}
                value={caseDraft.aiContext}
                onChange={(event) => updateCaseField("aiContext", event.target.value)}
                placeholder="Опиши тип инцидента, отрасль, желаемые observables"
              />
            </label>
            <div className="grid two-cols">
              <label>
                Case Number
                <input value={caseDraft.case_number} onChange={(event) => updateCaseField("case_number", event.target.value)} />
              </label>
              <label>
                Title *
                <input value={caseDraft.title} onChange={(event) => updateCaseField("title", event.target.value)} required />
              </label>
              <label>
                Source
                <input value={caseDraft.source} onChange={(event) => updateCaseField("source", event.target.value)} />
              </label>
              <label>
                Incident Type
                <input value={caseDraft.incident_type} onChange={(event) => updateCaseField("incident_type", event.target.value)} />
              </label>
              <label>
                Status
                <select value={caseDraft.status} onChange={(event) => updateCaseField("status", event.target.value)}>
                  {CASE_STATUSES.map((item) => <option key={item} value={item}>{item}</option>)}
                </select>
              </label>
              <label>
                Priority
                <select value={caseDraft.priority} onChange={(event) => updateCaseField("priority", event.target.value)}>
                  {CASE_PRIORITIES.map((item) => <option key={item} value={item}>{item}</option>)}
                </select>
              </label>
              <label>
                Severity
                <select value={caseDraft.severity} onChange={(event) => updateCaseField("severity", event.target.value)}>
                  {CASE_SEVERITIES.map((item) => <option key={item} value={item}>{item}</option>)}
                </select>
              </label>
              <label>
                Impact
                <input value={caseDraft.impact} onChange={(event) => updateCaseField("impact", event.target.value)} />
              </label>
              <label>
                Confidence
                <input
                  type="number"
                  min={0}
                  max={100}
                  value={caseDraft.confidence}
                  onChange={(event) => updateCaseField("confidence", Number(event.target.value || 0))}
                />
              </label>
              <label>
                Assigned To (uuid)
                <input value={caseDraft.assigned_to} onChange={(event) => updateCaseField("assigned_to", event.target.value)} />
              </label>
              <label>
                TLP
                <select value={caseDraft.tlp} onChange={(event) => updateCaseField("tlp", event.target.value)}>
                  {TLP_VALUES.map((item) => <option key={item} value={item}>{item}</option>)}
                </select>
              </label>
              <label>
                PAP
                <select value={caseDraft.pap} onChange={(event) => updateCaseField("pap", event.target.value)}>
                  {PAP_VALUES.map((item) => <option key={item} value={item}>{item}</option>)}
                </select>
              </label>
              <label>
                Detected At
                <input value={caseDraft.detected_at} onChange={(event) => updateCaseField("detected_at", event.target.value)} placeholder="2026-03-05T12:00:00Z" />
              </label>
              <label>
                Occurred At
                <input value={caseDraft.occurred_at} onChange={(event) => updateCaseField("occurred_at", event.target.value)} placeholder="2026-03-05T11:30:00Z" />
              </label>
            </div>

            <label>
              Description
              <textarea rows={4} value={caseDraft.description} onChange={(event) => updateCaseField("description", event.target.value)} />
            </label>

            <label>
              Resolution Summary
              <textarea rows={3} value={caseDraft.resolution_summary} onChange={(event) => updateCaseField("resolution_summary", event.target.value)} />
            </label>

            <div className="subpanel">
              <div className="actions-row">
                <h3>Observables</h3>
                <button
                  type="button"
                  onClick={() => setCaseDraft((prev) => ({ ...prev, observables: [...prev.observables, createObservableDraft()] }))}
                >
                  + Добавить observable
                </button>
              </div>

              {caseDraft.observables.length === 0 && <p className="muted">Нет observables.</p>}
              {caseDraft.observables.map((observable, index) => (
                <div key={`observable-${index}`} className="observable-row">
                  <select value={observable.type} onChange={(event) => updateObservable(index, "type", event.target.value)}>
                    {OBSERVABLE_TYPES.map((item) => <option key={item} value={item}>{item}</option>)}
                  </select>
                  <input
                    value={observable.value}
                    onChange={(event) => updateObservable(index, "value", event.target.value)}
                    placeholder="value"
                  />
                  <select value={observable.verdict} onChange={(event) => updateObservable(index, "verdict", event.target.value)}>
                    {OBSERVABLE_VERDICTS.map((item) => <option key={item} value={item}>{item}</option>)}
                  </select>
                  <input
                    value={observable.source}
                    onChange={(event) => updateObservable(index, "source", event.target.value)}
                    placeholder="source"
                  />
                  <input
                    value={observable.tags}
                    onChange={(event) => updateObservable(index, "tags", event.target.value)}
                    placeholder="tags через запятую"
                  />
                  <button type="button" className="danger" onClick={() => removeObservable(index)}>Удалить</button>
                </div>
              ))}
            </div>

            <button type="submit" disabled={busy}>Создать кейс</button>
          </form>
        )}

        {activeTab === "alert" && (
          <form className="form-grid" onSubmit={handleCreateAlert}>
            <div className="actions-row">
              <h2>Создать тестовый алерт</h2>
              <button type="button" onClick={fillAlertByAI} disabled={busy}>Заполнить AI</button>
            </div>

            <label>
              Контекст для AI
              <textarea
                rows={3}
                value={alertDraft.aiContext}
                onChange={(event) => setAlertDraft((prev) => ({ ...prev, aiContext: event.target.value }))}
                placeholder="Например: suspicious auth impossible travel"
              />
            </label>

            <div className="grid two-cols">
              <label>
                Title *
                <input value={alertDraft.title} onChange={(event) => setAlertDraft((prev) => ({ ...prev, title: event.target.value }))} required />
              </label>
              <label>
                Source
                <input value={alertDraft.source} onChange={(event) => setAlertDraft((prev) => ({ ...prev, source: event.target.value }))} />
              </label>
              <label>
                Status
                <select value={alertDraft.status} onChange={(event) => setAlertDraft((prev) => ({ ...prev, status: event.target.value }))}>
                  {ALERT_STATUSES.map((item) => <option key={item} value={item}>{item}</option>)}
                </select>
              </label>
              <label>
                Severity
                <select value={alertDraft.severity} onChange={(event) => setAlertDraft((prev) => ({ ...prev, severity: event.target.value }))}>
                  {ALERT_SEVERITIES.map((item) => <option key={item} value={item}>{item}</option>)}
                </select>
              </label>
              <label>
                TLP
                <select value={alertDraft.tlp} onChange={(event) => setAlertDraft((prev) => ({ ...prev, tlp: event.target.value }))}>
                  {TLP_VALUES.map((item) => <option key={item} value={item}>{item}</option>)}
                </select>
              </label>
              <label>
                PAP
                <select value={alertDraft.pap} onChange={(event) => setAlertDraft((prev) => ({ ...prev, pap: event.target.value }))}>
                  {PAP_VALUES.map((item) => <option key={item} value={item}>{item}</option>)}
                </select>
              </label>
            </div>

            <label>
              Description
              <textarea
                rows={4}
                value={alertDraft.description}
                onChange={(event) => setAlertDraft((prev) => ({ ...prev, description: event.target.value }))}
              />
            </label>

            <button type="submit" disabled={busy}>Создать алерт</button>
          </form>
        )}

        {activeTab === "bulk" && (
          <form className="form-grid" onSubmit={handleBulkCreate}>
            <h2>Пакетная генерация</h2>
            <div className="grid two-cols">
              <label>
                Сущность
                <select value={bulkDraft.entity} onChange={(event) => setBulkDraft((prev) => ({ ...prev, entity: event.target.value }))}>
                  <option value="case">Кейсы</option>
                  <option value="alert">Алерты</option>
                </select>
              </label>
              <label>
                Количество
                <input
                  type="number"
                  min={1}
                  max={40}
                  value={bulkDraft.count}
                  onChange={(event) => setBulkDraft((prev) => ({ ...prev, count: Number(event.target.value || 1) }))}
                />
              </label>
            </div>

            <label>
              Сценарий
              <textarea
                rows={4}
                value={bulkDraft.scenario}
                onChange={(event) => setBulkDraft((prev) => ({ ...prev, scenario: event.target.value }))}
                placeholder="Опиши бизнес-контекст для генерации"
              />
            </label>

            <label className="checkbox-row">
              <input
                type="checkbox"
                checked={bulkDraft.useAI}
                onChange={(event) => setBulkDraft((prev) => ({ ...prev, useAI: event.target.checked }))}
              />
              Использовать ИИ (если выключено, локальный генератор)
            </label>

            <label className="checkbox-row">
              <input
                type="checkbox"
                checked={bulkDraft.includeObservables}
                onChange={(event) => setBulkDraft((prev) => ({ ...prev, includeObservables: event.target.checked }))}
                disabled={bulkDraft.entity !== "case"}
              />
              Для кейсов добавлять observables
            </label>

            <button type="submit" disabled={busy}>Запустить пакетную генерацию</button>
            {busy && <div className="progress">Прогресс: {bulkProgress}%</div>}
          </form>
        )}
      </section>

      <section className="panel">
        <h2>Созданные сущности</h2>
        {createdItems.length === 0 ? (
          <p className="muted">Пока пусто.</p>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Время</th>
                  <th>Тип</th>
                  <th>ID</th>
                  <th>Title</th>
                </tr>
              </thead>
              <tbody>
                {createdItems.map((item) => (
                  <tr key={`${item.kind}-${item.id}-${item.createdAt}`}>
                    <td>{item.createdAt}</td>
                    <td>{item.kind}</td>
                    <td className="mono">{item.id}</td>
                    <td>{item.title}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className="panel">
        <h2>Логи</h2>
        {logs.length === 0 ? (
          <p className="muted">Пока пусто.</p>
        ) : (
          <ul className="log-list">
            {logs.map((entry) => (
              <li key={entry.id} className={`log-item ${entry.level}`}>
                <span className="mono">[{entry.ts}]</span> {entry.message}
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
