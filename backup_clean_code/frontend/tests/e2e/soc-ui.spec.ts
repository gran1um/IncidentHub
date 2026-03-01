import { expect, test, type BrowserContext, type Page } from "@playwright/test";
import { apiCreateOutboundConnector, apiListTenants, apiLogin, type APISession } from "./api";
import {
  DEFAULT_ADMIN_EMAIL,
  DEFAULT_ADMIN_PASSWORD,
  parseEntityIdFromURL,
  selectRadixOptionByText,
  selectRadixOptionWithSearch,
  uiLogin,
} from "./helpers";

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || "http://localhost:5173";
const ADMIN_EMAIL = process.env.PLAYWRIGHT_ADMIN_EMAIL || DEFAULT_ADMIN_EMAIL;
const ADMIN_PASSWORD = process.env.PLAYWRIGHT_ADMIN_PASSWORD || DEFAULT_ADMIN_PASSWORD;

test.describe.serial("SOC UI E2E", () => {
  let session: APISession;
  let tenantSlug = "admin";

  let context: BrowserContext;
  let page: Page;

  let mockConnectorName = "";
  let mockConnectorId = "";
  let caseId = "";
  let caseTitle = "";
  let forumThreadId = "";

  test.beforeAll(async ({ browser }) => {
    // One API login for test data setup.
    session = await apiLogin(BASE_URL, ADMIN_EMAIL, ADMIN_PASSWORD);

    const tenants = await apiListTenants(BASE_URL, session);
    const matched = tenants.find((tenant: any) => String(tenant?.id || "") === session.tenantId);
    tenantSlug = String(matched?.slug || tenants?.[0]?.slug || "admin").trim().toLowerCase() || "admin";

    // Ensure we always have a communication-capable connector for forum + case communications flows.
    mockConnectorName = `E2E Mock Connector ${Date.now()}`;
    const connectorResp = await apiCreateOutboundConnector(BASE_URL, session, {
      name: mockConnectorName,
      description: "Playwright E2E mock connector",
      type: "Webhook",
      direction: "outbound",
      channel: "mock",
      enabled: true,
      communication_mode: "chat",
      capabilities: ["hub_execute", "case_communications", "forum_threads", "sync_messages"],
      config: {},
    });

    expect(String(connectorResp?.id || "")).not.toBe("");
    mockConnectorId = String(connectorResp?.id || "").trim();
    expect(mockConnectorId).not.toBe("");

    // One UI login for the entire serial suite, to avoid tripping login rate limits.
    context = await browser.newContext();
    page = await context.newPage();
    await uiLogin(page, ADMIN_EMAIL, ADMIN_PASSWORD);
  });

  test.afterAll(async () => {
    await context?.close();
  });

  test("login works", async () => {
    await expect(page.getByTestId("dashboard-livestream-widget")).toBeVisible();
  });

  test("create case", async () => {
    await page.goto(`/${tenantSlug}/cases`);

    // Cases page no longer shows a dedicated "New case" button in the header.
    // Use the global Quick Action menu to create a case (real user flow).
    await page.getByTestId("quick-action").click();
    await page.getByTestId("quick-action-item-add-case").click();

    caseTitle = `E2E Case ${Date.now()}`;
    await page.getByTestId("quick-action-case-title").fill(caseTitle);
    await page.getByTestId("quick-action-case-description").fill("Playwright E2E case.");

    await page.getByTestId("quick-action-case-submit").click();

    await page.waitForURL(new RegExp(`/${tenantSlug}/cases/`));
    await expect(page.getByTestId("case-detail-page")).toBeVisible();

    caseId = parseEntityIdFromURL(page);
    expect(caseId).not.toBe("");

    await expect(page.getByTestId("text-case-title")).toContainText(caseTitle);
  });

  test("case tasks: due date is required and case card shows countdown", async () => {
    test.skip(!caseId, "case was not created");

    await page.goto(`/${tenantSlug}/cases/${caseId}?tab=tasks`);
    await page.getByTestId("tab-tasks").click();

    const titleInput = page.getByTestId("input-task-title");
    const dueInput = page.getByTestId("input-task-due-date");
    const createButton = page.getByTestId("button-create-task");

    await titleInput.fill(`E2E Task ${Date.now()}`);
    await expect(dueInput).toHaveValue("");
    await expect(createButton).toBeDisabled();

    await page.getByTestId("task-due-presets").getByRole("button", { name: "+1h" }).click();
    await expect(dueInput).not.toHaveValue("");
    await expect(createButton).toBeEnabled();

    await createButton.click();

    await page.goto(`/${tenantSlug}/cases`);
    await expect(page.getByTestId(`case-card-${caseId}`)).toBeVisible();
    await expect(page.getByTestId(`case-next-task-due-${caseId}`)).toBeVisible();
  });

  test("duplicate case asks confirmation and creates a new case", async () => {
    test.skip(!caseId, "case was not created");

    await page.goto(`/${tenantSlug}/cases/${caseId}`);

    await page.getByTestId("button-copy-case").click();
    await page.getByRole("button", { name: "Duplicate case" }).click();

    await page.waitForURL((url) => {
      const parts = url.pathname.split("/").filter(Boolean);
      return parts[0] == tenantSlug && parts[1] === "cases" && parts[2] && parts[2] !== caseId;
    });

    const duplicatedId = parseEntityIdFromURL(page);
    expect(duplicatedId).not.toBe("");
    expect(duplicatedId).not.toBe(caseId);
  });

  test("connectors page opens connector editor tab (list-first UX)", async () => {
    test.skip(!mockConnectorId, "mock connector was not created");

    await page.goto(`/${tenantSlug}/connectors`);

    await page.getByTestId(`connector-card-${mockConnectorId}`).click();
    await expect(page.getByText("Edit connector", { exact: true })).toBeVisible();

    await page.getByTestId("button-close-connector-editor").click();
    await expect(page.getByText("Edit connector", { exact: true })).toBeHidden();
    await expect(page.getByText("Connector Catalog", { exact: true })).toBeVisible();
  });

  test("forum: create thread and open it (no blank screen)", async () => {
    test.skip(!caseId || !caseTitle, "case was not created");

    await page.goto(`/${tenantSlug}/forum`);

    await page.getByTestId("button-forum-new-thread").click();

    const threadTitle = `E2E Thread ${Date.now()}`;
    await page.getByTestId("input-forum-thread-title").fill(threadTitle);
    await page.getByTestId("input-forum-thread-message").fill("Initial forum message from Playwright");

    // Select the case explicitly so the thread is linked deterministically.
    const caseTrigger = page.getByTestId("select-forum-thread-case");
    await selectRadixOptionByText(caseTrigger, new RegExp(caseId));

    await page.getByTestId("button-create-forum-thread").click();

    await page.waitForURL(new RegExp(`/${tenantSlug}/forum/`));
    forumThreadId = parseEntityIdFromURL(page);
    expect(forumThreadId).not.toBe("");

    await expect(page.getByRole("heading", { level: 1 })).toContainText(threadTitle);
  });

  test("forum: send via connector and sync replies", async () => {
    test.skip(!forumThreadId, "forum thread not created");

    await page.goto(`/${tenantSlug}/forum/${forumThreadId}`);

    const composer = page.getByTestId("textarea-forum-thread-reply");
    const pingText = `Ping via connector ${Date.now()}`;
    await composer.fill(pingText);

    await page.getByTestId("button-forum-send-via-connector").click();

    await selectRadixOptionWithSearch(
      page.getByTestId("select-forum-thread-connector-trigger"),
      mockConnectorName.slice(0, 14),
      new RegExp(mockConnectorName),
    );

    await page.getByTestId("input-forum-thread-connector-chat-id").fill("e2e-route");
    const sendConfirm = page.getByTestId("button-forum-send-via-connector-confirm");

    const [sendResp] = await Promise.all([
      page.waitForResponse(
        (resp) => resp.url().includes(`/api/v1/forum/threads/${forumThreadId}/proxy/send`) && resp.request().method() === "POST",
      ),
      sendConfirm.click(),
    ]);

    const sendBody = await sendResp.text();
    if (!sendResp.ok()) {
      throw new Error(`forum proxy send failed (${sendResp.status()}): ${sendBody}`);
    }

    await expect(sendConfirm).toBeHidden();

    await expect(page.getByText(pingText, { exact: true })).toBeVisible();

    // Sync is executed from the connector dialog; open it again after send.
    await page.getByTestId("button-forum-send-via-connector").click();

    const profileSelect = page.getByTestId("select-forum-thread-proxy-profile");
    await expect(profileSelect).toBeVisible({ timeout: 30000 });

    // Select the first linked route (enables sync).
    await profileSelect.click();
    await page.locator("[role=option]").first().click();

    const syncButton = page.getByTestId("button-forum-sync-replies");
    await expect(syncButton).toBeEnabled();
    await syncButton.click();

    await expect(page.getByText("Mock sync event", { exact: true })).toBeVisible();
  });

  test("case communications: create thread, send, sync", async () => {
    test.skip(!caseId, "case was not created");

    await page.goto(`/${tenantSlug}/cases/${caseId}?tab=communications`);

    await page.getByTestId("tab-communications").click();
    await page.getByTestId("button-create-communication-thread").click();

    await page.getByTestId("input-new-communication-title").fill(`E2E Comms ${Date.now()}`);

    await selectRadixOptionByText(
      page.getByTestId("select-new-communication-connector"),
      new RegExp(mockConnectorName),
    );

    await page.getByTestId("input-new-communication-target").fill("e2e-target");
    await page.getByTestId("button-submit-new-communication-thread").click();

    await expect(page.getByTestId("communication-messages-list")).toBeVisible();

    await page.getByTestId("textarea-communication-message").fill("Hello from Playwright communications.");
    await page.getByTestId("button-send-communication-message").click();

    await page.getByTestId("button-sync-communication-thread").click();
    await expect(page.getByTestId("communication-messages-list").getByText("Mock sync event")).toBeVisible();
  });

  test("AI workloads tab renders without 'Last updated' label", async () => {
    test.skip(!caseId, "case was not created");

    await page.goto(`/${tenantSlug}/cases/${caseId}?tab=ai`);
    await page.getByTestId("tab-ai").click();

    await expect(page.getByTestId("case-ai-workloads")).toBeVisible();
    await expect(page.locator("text=Last updated")).toHaveCount(0);
  });
});
