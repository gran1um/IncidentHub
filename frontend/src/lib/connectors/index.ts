export type ConnectorDirection = "inbound" | "outbound";

export function normalizeConnectorDirection(value: unknown): ConnectorDirection {
  const raw = String(value ?? "").trim().toLowerCase();
  return raw === "inbound" ? "inbound" : "outbound";
}

export function splitConnectorsByDirection<T extends { direction?: unknown }>(
  connectors: T[],
): { inbound: T[]; outbound: T[] } {
  const inbound: T[] = [];
  const outbound: T[] = [];

  for (const connector of connectors) {
    if (normalizeConnectorDirection(connector.direction) === "inbound") {
      inbound.push(connector);
    } else {
      outbound.push(connector);
    }
  }

  return { inbound, outbound };
}

export function normalizeObservableConnectorType(value: unknown): string {
  const normalized = String(value ?? "")
    .trim()
    .toLowerCase()
    .replace(/[/_-]+/g, " ")
    .replace(/\s+/g, " ");
  if (!normalized) return "";

  const slug = normalized.replace(/\s+/g, "");
  if (["ip", "ipaddress", "ipv4", "ipv6", "srcip", "dstip"].includes(slug) || normalized.includes("ip address")) return "ip";
  if (["domain", "fqdn"].includes(slug) || normalized.includes("domain")) return "domain";
  if (["host", "hostname", "computer", "device", "asset"].includes(slug) || normalized.includes("host name")) return "host";
  if (["url", "uri", "link"].includes(slug) || normalized.includes("url")) return "url";
  if (["email", "mail", "e mail"].includes(normalized) || ["email", "mail", "emailaddress", "e-mail"].includes(slug)) return "email";
  if (["hash", "filehash", "md5", "sha1", "sha224", "sha256", "sha384", "sha512"].includes(slug) || normalized.includes("hash")) return "hash";
  if (["useragent", "ua"].includes(slug) || normalized.includes("user agent")) return "user_agent";
  if (["user", "username", "account", "principal"].includes(slug) || normalized.includes("account")) return "user";
  if (["process", "processname", "image"].includes(slug) || normalized.includes("process")) return "process";
  if (["file", "filename", "filepath", "path"].includes(slug) || normalized.includes("file")) return "file";

  return normalized;
}

export function splitObservableConnectorTypes(value: unknown): string[] {
  const values = Array.isArray(value)
    ? value
    : String(value ?? "")
        .split(/[,;\s]+/)
        .map((entry) => entry.trim())
        .filter(Boolean);

  const seen = new Set<string>();
  const result: string[] = [];
  values.forEach((entry) => {
    const normalized = normalizeObservableConnectorType(entry);
    if (!normalized || seen.has(normalized)) return;
    seen.add(normalized);
    result.push(normalized);
  });
  return result;
}

export type ObservableConnectorOption = {
  connectorId: string;
  methodId: string;
  label: string;
  action: string;
  category?: string;
  connectorType?: string;
};

export function buildObservableConnectorOptionsByType(
  connectorsInput: any[],
  methodsInput: any[],
): Record<string, ObservableConnectorOption[]> {
  const connectors = Array.isArray(connectorsInput) ? connectorsInput : [];
  const methods = Array.isArray(methodsInput) ? methodsInput : [];
  const connectorsById = new Map<string, any>();

  connectors.forEach((connector) => {
    const connectorId = String(connector?.id || "").trim();
    if (!connectorId) return;
    if (normalizeConnectorDirection(connector?.direction) === "inbound") return;
    if (connector?.enabled === false || String(connector?.enabled ?? "").toLowerCase() === "false") return;
    connectorsById.set(connectorId, connector);
  });

  const byType: Record<string, ObservableConnectorOption[]> = {};
  methods.forEach((method) => {
    const connectorId = String(
      method?.refId || method?.ref_id || method?.connectorId || method?.connector_id || "",
    ).trim();
    if (!connectorId) return;
    const connector = connectorsById.get(connectorId);
    if (!connector) return;

    const observableTypes = splitObservableConnectorTypes(
      method?.observable_types || method?.observableTypes || [],
    );
    if (observableTypes.length === 0) return;

    const connectorName = String(connector?.name || connectorId || "Connector").trim();
    const connectorType = String(connector?.type || "").trim();
    const connectorCategory = String(connector?.category || "").trim().toLowerCase();
    const methodName = String(
      method?.name || method?.title || method?.action || method?.slug || method?.key || method?.id || "Method",
    ).trim();
    const methodId = String(method?.id || "").trim();
    if (!methodId) return;

    const option: ObservableConnectorOption = {
      connectorId,
      methodId,
      label:
        connectorName && methodName
          ? `${connectorName} — ${methodName}`
          : methodName || connectorName || "Connector method",
      action: String(method?.action || method?.slug || method?.key || "").trim(),
      category: connectorCategory || undefined,
      connectorType: connectorType || undefined,
    };

    observableTypes.forEach((observableType) => {
      if (!byType[observableType]) {
        byType[observableType] = [];
      }
      byType[observableType].push(option);
    });
  });

  Object.values(byType).forEach((options) => {
    options.sort((left, right) => left.label.localeCompare(right.label));
  });

  return byType;
}
