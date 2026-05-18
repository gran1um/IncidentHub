import { describe, expect, it } from "vitest";

import {
  buildObservableConnectorOptionsByType,
  normalizeObservableConnectorType,
  splitObservableConnectorTypes,
} from "@/lib/connectors";

describe("connector observable routing", () => {
  it("normalizes common observable aliases into canonical keys", () => {
    expect(normalizeObservableConnectorType("IPv4")).toBe("ip");
    expect(normalizeObservableConnectorType("fqdn")).toBe("domain");
    expect(normalizeObservableConnectorType("hostname")).toBe("host");
    expect(normalizeObservableConnectorType("sha256")).toBe("hash");
    expect(normalizeObservableConnectorType("user agent")).toBe("user_agent");
  });

  it("splits and deduplicates observable type declarations", () => {
    expect(splitObservableConnectorTypes("ip, ipv4; sha256 url ip")).toEqual(["ip", "hash", "url"]);
  });

  it("builds generic connector options by canonical observable type", () => {
    const options = buildObservableConnectorOptionsByType(
      [
        { id: "connector-b", name: "Beta Intel", direction: "outbound", enabled: true },
        { id: "connector-a", name: "Alpha Intel", direction: "outbound", enabled: true },
        { id: "connector-disabled", name: "Disabled", direction: "outbound", enabled: false },
        { id: "connector-inbound", name: "Inbound", direction: "inbound", enabled: true },
      ],
      [
        { id: "method-1", ref_id: "connector-a", name: "Lookup hash", action: "scan_hash", observable_types: ["sha256"] },
        { id: "method-2", ref_id: "connector-b", name: "Lookup IP", action: "scan_ip", observable_types: ["ip", "ipv4"] },
        { id: "method-3", ref_id: "connector-disabled", name: "Disabled method", action: "noop", observable_types: ["ip"] },
        { id: "method-4", ref_id: "connector-inbound", name: "Inbound method", action: "noop", observable_types: ["ip"] },
      ],
    );

    expect(options.hash).toHaveLength(1);
    expect(options.hash[0]).toMatchObject({
      connectorId: "connector-a",
      methodId: "method-1",
      label: "Alpha Intel — Lookup hash",
      action: "scan_hash",
    });

    expect(options.ip).toHaveLength(1);
    expect(options.ip[0]).toMatchObject({
      connectorId: "connector-b",
      methodId: "method-2",
      label: "Beta Intel — Lookup IP",
      action: "scan_ip",
    });
  });
});
