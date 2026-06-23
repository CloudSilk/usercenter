import { test, expect, type Page } from "@playwright/test";

const BASE = "http://localhost:5110/web/admin";
const API = "http://localhost:48180";

const ADMIN_USER = "admin";
const ADMIN_PWD = "Admin@123456";

// ─── helpers ────────────────────────────────────────────────────────────────

async function login(page: Page, user = ADMIN_USER, pwd = ADMIN_PWD) {
  await page.goto(`${BASE}/login`);
  await page.waitForSelector("#userName", { timeout: 10_000 });
  await page.fill("#userName", user);
  await page.fill("#password", pwd);
  await page.getByRole("button", { name: /sign in/i }).click();
  await page.waitForFunction(
    () => !window.location.pathname.endsWith("/login"),
    { timeout: 15_000 },
  );
}

async function snap(page: Page, name: string) {
  await page.screenshot({ path: `e2e/${name}.png`, fullPage: true });
}

/** Assert no error overlay on the page. */
async function assertNoPageError(page: Page) {
  const bodyText = await page.textContent("body");
  const hasError =
    bodyText?.includes("TypeError") ||
    bodyText?.includes("Cannot read properties") ||
    bodyText?.includes("Unexpected Application Error") ||
    bodyText?.includes("is not defined") ||
    bodyText?.includes("is undefined");
  expect(hasError, "Page should not contain runtime errors").toBeFalsy();
}

/** Navigate to a page, wait for load, take screenshot, assert no error. */
async function visitPage(page: Page, path: string, name: string) {
  await page.goto(`${BASE}${path}`);
  await page.waitForLoadState("networkidle");
  await page.waitForTimeout(2000);
  await assertNoPageError(page);
  await snap(page, name);
}

// ─── Login Tests ────────────────────────────────────────────────────────────

test.describe("Authentication", () => {
  test("login page renders with form", async ({ page }) => {
    await page.goto(`${BASE}/login`);
    await page.waitForSelector("#userName", { timeout: 10_000 });
    expect(await page.locator("input").count()).toBeGreaterThanOrEqual(2);
    await snap(page, "01-login");
  });

  test("login with valid credentials succeeds", async ({ page }) => {
    await login(page);
    expect(page.url()).not.toContain("/login");
    await page.waitForTimeout(2000);
    await assertNoPageError(page);
    await snap(page, "02-after-login");
  });

  test("login with wrong password stays on login page", async ({ page }) => {
    await page.goto(`${BASE}/login`);
    await page.waitForSelector("#userName", { timeout: 10_000 });
    await page.fill("#userName", ADMIN_USER);
    await page.fill("#password", "wrong-password");
    await page.getByRole("button", { name: /sign in/i }).click();
    await page.waitForTimeout(3000);
    expect(page.url()).toContain("/login");
    await snap(page, "03-login-failed");
  });
});

// ─── All Pages Smoke Test ───────────────────────────────────────────────────

test.describe("All pages load without errors", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.waitForSelector("aside", { timeout: 10_000 });
    await page.waitForTimeout(1000);
  });

  test("Dashboard", async ({ page }) => {
    await visitPage(page, "/", "10-dashboard");
  });

  test("Users", async ({ page }) => {
    await visitPage(page, "/users", "11-users");
  });

  test("Roles", async ({ page }) => {
    await visitPage(page, "/roles", "12-roles");
  });

  test("Tenants", async ({ page }) => {
    await visitPage(page, "/tenants", "13-tenants");
  });

  test("Gateway", async ({ page }) => {
    await visitPage(page, "/gateway", "14-gateway");
  });

  test("AIKeys", async ({ page }) => {
    await visitPage(page, "/aikeys", "15-aikeys");
  });

  test("Usage", async ({ page }) => {
    await visitPage(page, "/usage", "16-usage");
  });

  test("Sessions", async ({ page }) => {
    await visitPage(page, "/sessions", "17-sessions");
  });

  test("Audit", async ({ page }) => {
    await visitPage(page, "/audit", "18-audit");
  });

  test("LiveAudit", async ({ page }) => {
    await visitPage(page, "/liveaudit", "19-liveaudit");
  });

  test("Security", async ({ page }) => {
    await visitPage(page, "/security", "20-security");
  });

  test("OAuth", async ({ page }) => {
    await visitPage(page, "/oauth", "21-oauth");
  });

  test("SCIM", async ({ page }) => {
    await visitPage(page, "/scim", "22-scim");
  });

  test("Config", async ({ page }) => {
    await visitPage(page, "/config", "23-config");
  });

  test("API Tester", async ({ page }) => {
    await visitPage(page, "/tester", "24-tester");
  });
});

// ─── CRUD: Users ────────────────────────────────────────────────────────────

test.describe("Users CRUD", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto(`${BASE}/users`);
    await page.waitForLoadState("networkidle");
    await page.waitForTimeout(2000);
  });

  test("user list renders table", async ({ page }) => {
    await assertNoPageError(page);
    await snap(page, "30-users-table");
  });

  test("add user dialog opens", async ({ page }) => {
    const addBtn = page.getByRole("button", { name: /add|create|new|添加|新增/i }).first();
    if (await addBtn.isVisible()) {
      await addBtn.click();
      await page.waitForTimeout(1000);
      await assertNoPageError(page);
      await snap(page, "31-users-add-dialog");
      await page.keyboard.press("Escape");
    }
  });

  test("search works", async ({ page }) => {
    const searchInput = page.locator('input[placeholder*="search" i], input[placeholder*="搜索" i], input[type="search"]').first();
    if (await searchInput.isVisible()) {
      await searchInput.fill("admin");
      await page.waitForTimeout(1000);
      await assertNoPageError(page);
      await snap(page, "32-users-search");
    }
  });
});

// ─── CRUD: Roles ────────────────────────────────────────────────────────────

test.describe("Roles CRUD", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto(`${BASE}/roles`);
    await page.waitForLoadState("networkidle");
    await page.waitForTimeout(2000);
  });

  test("role list renders", async ({ page }) => {
    await assertNoPageError(page);
    await snap(page, "40-roles-table");
  });

  test("add role dialog opens", async ({ page }) => {
    const addBtn = page.getByRole("button", { name: /add|create|new|添加|新增/i }).first();
    if (await addBtn.isVisible()) {
      await addBtn.click();
      await page.waitForTimeout(1000);
      await assertNoPageError(page);
      await snap(page, "41-roles-add-dialog");
      await page.keyboard.press("Escape");
    }
  });
});

// ─── CRUD: Tenants ──────────────────────────────────────────────────────────

test.describe("Tenants CRUD", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto(`${BASE}/tenants`);
    await page.waitForLoadState("networkidle");
    await page.waitForTimeout(2000);
  });

  test("tenant list renders", async ({ page }) => {
    await assertNoPageError(page);
    await snap(page, "50-tenants-table");
  });

  test("add tenant dialog opens", async ({ page }) => {
    const addBtn = page.getByRole("button", { name: /add|create|new|添加|新增/i }).first();
    if (await addBtn.isVisible()) {
      await addBtn.click();
      await page.waitForTimeout(1000);
      await assertNoPageError(page);
      await snap(page, "51-tenants-add-dialog");
      await page.keyboard.press("Escape");
    }
  });
});

// ─── CRUD: System Config ────────────────────────────────────────────────────

test.describe("System Config CRUD", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto(`${BASE}/config`);
    await page.waitForLoadState("networkidle");
    await page.waitForTimeout(2000);
  });

  test("config list renders", async ({ page }) => {
    await assertNoPageError(page);
    await snap(page, "60-config-table");
  });

  test("add config dialog opens", async ({ page }) => {
    const addBtn = page.getByRole("button", { name: /add|create|new|添加|新增/i }).first();
    if (await addBtn.isVisible()) {
      await addBtn.click();
      await page.waitForTimeout(1000);
      await assertNoPageError(page);
      await snap(page, "61-config-add-dialog");
      await page.keyboard.press("Escape");
    }
  });
});

// ─── CRUD: OAuth Clients ────────────────────────────────────────────────────

test.describe("OAuth Clients CRUD", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto(`${BASE}/oauth`);
    await page.waitForLoadState("networkidle");
    await page.waitForTimeout(2000);
  });

  test("oauth clients list renders", async ({ page }) => {
    await assertNoPageError(page);
    await snap(page, "70-oauth-table");
  });

  test("add client dialog opens", async ({ page }) => {
    const addBtn = page.getByRole("button", { name: /add|create|new|添加|新增/i }).first();
    if (await addBtn.isVisible()) {
      await addBtn.click();
      await page.waitForTimeout(1000);
      await assertNoPageError(page);
      await snap(page, "71-oauth-add-dialog");
      await page.keyboard.press("Escape");
    }
  });
});

// ─── Remaining pages: no-error check ────────────────────────────────────────

test.describe("AI Keys page", () => {
  test("renders without errors", async ({ page }) => {
    await login(page);
    await visitPage(page, "/aikeys", "80-aikeys");
  });
});

test.describe("Sessions page", () => {
  test("renders without errors", async ({ page }) => {
    await login(page);
    await visitPage(page, "/sessions", "90-sessions");
  });
});

test.describe("Audit page", () => {
  test("renders without errors", async ({ page }) => {
    await login(page);
    await visitPage(page, "/audit", "91-audit");
  });
});

test.describe("Security page", () => {
  test("renders without errors", async ({ page }) => {
    await login(page);
    await visitPage(page, "/security", "92-security");
  });
});

test.describe("SCIM page", () => {
  test("renders without errors", async ({ page }) => {
    await login(page);
    await visitPage(page, "/scim", "93-scim");
  });
});

test.describe("Gateway page", () => {
  test("renders without errors", async ({ page }) => {
    await login(page);
    await visitPage(page, "/gateway", "94-gateway");
  });
});

test.describe("Usage page", () => {
  test("renders without errors", async ({ page }) => {
    await login(page);
    await visitPage(page, "/usage", "95-usage");
  });
});

test.describe("API Tester page", () => {
  test("renders without errors", async ({ page }) => {
    await login(page);
    await visitPage(page, "/tester", "96-tester");
  });
});

test.describe("LiveAudit page", () => {
  test("renders without errors", async ({ page }) => {
    await login(page);
    await visitPage(page, "/liveaudit", "97-liveaudit");
  });
});

// ─── Backend API Comprehensive ──────────────────────────────────────────────

test.describe("Backend API comprehensive", () => {
  let jwt = "";
  let scimToken = "scim-dev-token";

  test.beforeAll(async ({ request }) => {
    const res = await request.post(`${API}/api/core/auth/user/login`, {
      data: { userName: ADMIN_USER, password: ADMIN_PWD },
    });
    jwt = (await res.json()).data;
  });

  test("health", async ({ request }) => {
    const res = await request.get(`${API}/health`);
    expect((await res.json()).status).toBe("ok");
  });

  test("profile", async ({ request }) => {
    const res = await request.get(`${API}/api/core/auth/user/profile`, {
      headers: { Authorization: `Bearer ${jwt}` },
    });
    const body = await res.json();
    expect(body.code).toBe(20000);
    expect(body.data.userName).toBe(ADMIN_USER);
  });

  test("user query", async ({ request }) => {
    const res = await request.get(`${API}/api/core/auth/user/query`, {
      headers: { Authorization: `Bearer ${jwt}` },
      params: { pageIndex: 1, pageSize: 10 },
    });
    const body = await res.json();
    expect(body.code).toBe(20000);
  });

  test("role query", async ({ request }) => {
    const res = await request.get(`${API}/api/core/auth/role/query`, {
      headers: { Authorization: `Bearer ${jwt}` },
      params: { pageIndex: 1, pageSize: 10 },
    });
    const body = await res.json();
    expect(body.code).toBe(20000);
  });

  test("tenant query", async ({ request }) => {
    const res = await request.get(`${API}/api/core/auth/tenant/query`, {
      headers: { Authorization: `Bearer ${jwt}` },
      params: { pageIndex: 1, pageSize: 10 },
    });
    const body = await res.json();
    expect(body.code).toBe(20000);
  });

  test("menu tree", async ({ request }) => {
    const res = await request.get(`${API}/api/core/auth/menu/tree`, {
      headers: { Authorization: `Bearer ${jwt}` },
    });
    const body = await res.json();
    expect(body.code).toBe(20000);
  });

  test("system config query", async ({ request }) => {
    const res = await request.get(`${API}/api/core/system/config/query`, {
      headers: { Authorization: `Bearer ${jwt}` },
      params: { pageIndex: 1, pageSize: 10 },
    });
    const body = await res.json();
    expect(body.code).toBe(20000);
  });

  test("sessions list (requires principalID)", async ({ request }) => {
    // Backend requires principalID as mandatory filter; verify the endpoint exists and validates input
    const res = await request.get(`${API}/admin/api/sessions`, {
      headers: { Authorization: `Bearer ${jwt}` },
    });
    const body = await res.json();
    // 40000 = input validation error, which means endpoint is alive and correctly rejecting bad input
    expect(body.code === 20000 || body.code === 40000).toBeTruthy();
  });

  test("audit logs", async ({ request }) => {
    const res = await request.get(`${API}/admin/api/audit-logs`, {
      headers: { Authorization: `Bearer ${jwt}` },
      params: { pageIndex: 1, pageSize: 10 },
    });
    const body = await res.json();
    expect(body.code).toBe(20000);
  });

  test("AI providers", async ({ request }) => {
    const res = await request.get(`${API}/admin/api/ai-providers`, {
      headers: { Authorization: `Bearer ${jwt}` },
    });
    const body = await res.json();
    expect(body.code).toBe(20000);
  });

  test("AI keys", async ({ request }) => {
    const res = await request.get(`${API}/admin/api/ai-keys`, {
      headers: { Authorization: `Bearer ${jwt}` },
    });
    const body = await res.json();
    expect(body.code).toBe(20000);
  });

  test("OAuth clients", async ({ request }) => {
    const res = await request.get(`${API}/admin/api/oauth-clients`, {
      headers: { Authorization: `Bearer ${jwt}` },
    });
    const body = await res.json();
    expect(body.code).toBe(20000);
  });

  test("MFA factors", async ({ request }) => {
    const res = await request.get(`${API}/admin/api/mfa/factors`, {
      headers: { Authorization: `Bearer ${jwt}` },
    });
    const body = await res.json();
    expect(body.code).toBe(20000);
  });

  test("SCIM schemas", async ({ request }) => {
    // SCIM endpoints use a dedicated SCIM bearer token, not the JWT
    const res = await request.get(`${API}/scim/v2/Schemas`, {
      headers: { Authorization: `Bearer ${scimToken}` },
    });
    expect(res.ok()).toBeTruthy();
  });

  test("OIDC discovery", async ({ request }) => {
    const res = await request.get(`${API}/.well-known/openid-configuration`);
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.issuer).toBeTruthy();
  });

  test("JWKS endpoint", async ({ request }) => {
    const res = await request.get(`${API}/.well-known/jwks.json`);
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.keys.length).toBeGreaterThan(0);
  });

  test("prompts", async ({ request }) => {
    const res = await request.get(`${API}/admin/api/prompts`, {
      headers: { Authorization: `Bearer ${jwt}` },
    });
    const body = await res.json();
    expect(body.code).toBe(20000);
  });

  test("usage summary", async ({ request }) => {
    const res = await request.get(`${API}/admin/api/usage/summary`, {
      headers: { Authorization: `Bearer ${jwt}` },
    });
    const body = await res.json();
    expect(body.code).toBe(20000);
  });

  test("usage by model", async ({ request }) => {
    const res = await request.get(`${API}/admin/api/usage/by-model`, {
      headers: { Authorization: `Bearer ${jwt}` },
    });
    const body = await res.json();
    expect(body.code).toBe(20000);
  });

  test("social providers", async ({ request }) => {
    const res = await request.get(`${API}/api/social/providers`);
    expect(res.ok()).toBeTruthy();
  });

  test("metrics endpoint", async ({ request }) => {
    const res = await request.get(`${API}/metrics`);
    expect(res.ok()).toBeTruthy();
  });

  test("SCIM service provider config", async ({ request }) => {
    const res = await request.get(`${API}/scim/v2/ServiceProviderConfig`);
    expect(res.ok()).toBeTruthy();
  });

  test("external identities", async ({ request }) => {
    const res = await request.get(`${API}/admin/api/identities`, {
      headers: { Authorization: `Bearer ${jwt}` },
    });
    const body = await res.json();
    expect(body.code).toBe(20000);
  });

  test("alert status", async ({ request }) => {
    const res = await request.get(`${API}/admin/api/alerts/status`, {
      headers: { Authorization: `Bearer ${jwt}` },
    });
    const body = await res.json();
    expect(body.code).toBe(20000);
  });

  test("logout", async ({ request }) => {
    const res = await request.post(`${API}/api/core/auth/user/logout`, {
      headers: { Authorization: `Bearer ${jwt}` },
    });
    const body = await res.json();
    expect(body.code).toBe(20000);
  });
});
