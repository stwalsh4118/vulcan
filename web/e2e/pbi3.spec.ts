import { test, expect } from "@playwright/test";

test.describe("PBI 3: Frontend Foundation & Workload Dashboard", () => {
  // AC1: pnpm dev starts the Next.js development server and loads the dashboard.
  test("AC1: dashboard loads at root URL", async ({ page }) => {
    await page.goto("/");
    await expect(page).toHaveTitle("Vulcan");
    await expect(
      page.getByRole("heading", { name: "Dashboard" }),
    ).toBeVisible();
  });

  // AC2: Dashboard home page shows recent workloads and basic stats.
  test("AC2: dashboard shows stats and recent workloads sections", async ({
    page,
  }) => {
    await page.goto("/");
    await expect(
      page.getByRole("heading", { name: "Overview" }),
    ).toBeVisible();
    await expect(page.getByText("Total Workloads")).toBeVisible();
    await expect(page.getByText("Avg Duration")).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "Recent Workloads" }),
    ).toBeVisible();
  });

  // AC3: Workload submission form allows selecting runtime, isolation mode,
  // entering code, providing JSON input, and setting resource limits.
  test("AC3: submission form has all required fields", async ({ page }) => {
    await page.goto("/workloads/new");

    // Runtime selector
    const runtimeSelect = page.locator("select").first();
    await expect(runtimeSelect).toBeVisible();
    const runtimeOptions = await runtimeSelect
      .locator("option")
      .allTextContents();
    expect(runtimeOptions).toEqual(
      expect.arrayContaining(["go", "node", "python", "wasm", "oci"]),
    );

    // Isolation selector
    const isolationSelect = page.locator("select").nth(1);
    await expect(isolationSelect).toBeVisible();
    const isolationOptions = await isolationSelect
      .locator("option")
      .allTextContents();
    expect(
      isolationOptions.map((o) => o.replace(" (not available)", "")),
    ).toEqual(expect.arrayContaining(["auto", "microvm", "isolate", "gvisor"]));

    // Code editor (CodeMirror renders a div with role="textbox")
    await expect(page.locator('[role="textbox"]')).toBeVisible();

    // JSON input textarea
    await expect(page.locator("textarea")).toBeVisible();

    // Resource limits (3 number inputs)
    const numberInputs = page.locator('input[type="number"]');
    await expect(numberInputs).toHaveCount(3);

    // Submit button
    await expect(page.locator('button[type="submit"]')).toBeVisible();
    await expect(page.locator('button[type="submit"]')).toHaveText(
      "Submit Workload",
    );
  });

  // AC4: Submitting a workload calls the API and navigates to the workload detail page.
  test("AC4: submit workload and navigate to detail page", async ({
    page,
  }) => {
    await page.goto("/workloads/new");

    // Select runtime = node (default)
    await page.locator("select").first().selectOption("node");

    // Type code in CodeMirror editor
    const editor = page.locator('[role="textbox"]');
    await editor.click();
    await editor.fill('console.log("hello")');

    // Submit
    await page.locator('button[type="submit"]').click();

    // Should navigate to detail page
    await page.waitForURL(/\/workloads\/[A-Z0-9]+/i, { timeout: 10_000 });
    await expect(page.getByText("Runtime").first()).toBeVisible();
    await expect(page.getByText("Isolation").first()).toBeVisible();
  });

  // AC5: Workload list page shows all workloads with pagination and status filtering.
  test("AC5: workload list page with table and filters", async ({ page }) => {
    // First create a workload so the list isn't empty
    await page.goto("/workloads/new");
    await page.locator("select").first().selectOption("node");
    const editor = page.locator('[role="textbox"]');
    await editor.click();
    await editor.fill('console.log("test")');
    await page.locator('button[type="submit"]').click();
    await page.waitForURL(/\/workloads\/[A-Z0-9]+/i, { timeout: 10_000 });

    // Navigate to list page
    await page.goto("/workloads");

    // Filter buttons should be present
    await expect(page.locator("button", { hasText: "All" })).toBeVisible();
    await expect(
      page.locator("button", { hasText: "pending" }),
    ).toBeVisible();
    await expect(
      page.locator("button", { hasText: "completed" }),
    ).toBeVisible();

    // Table should have at least one row
    const rows = page.locator("tbody tr");
    await expect(rows.first()).toBeVisible({ timeout: 10_000 });

    // Row should link to detail page
    const firstLink = rows.first().locator("a").first();
    await expect(firstLink).toBeVisible();
  });

  // AC6: Workload detail page shows status, output, error, duration, isolation, metadata.
  test("AC6: detail page shows all workload fields", async ({ page }) => {
    // Submit a workload and wait for it to complete
    await page.goto("/workloads/new");
    await page.locator("select").first().selectOption("node");
    const editor = page.locator('[role="textbox"]');
    await editor.click();
    await editor.fill('console.log("detail test")');
    await page.locator('button[type="submit"]').click();
    await page.waitForURL(/\/workloads\/[A-Z0-9]+/i, { timeout: 10_000 });

    // Wait for workload to complete (polling will update the status)
    await expect(page.getByText("completed")).toBeVisible({
      timeout: 15_000,
    });

    // Verify detail fields
    await expect(page.getByText("Runtime").first()).toBeVisible();
    await expect(page.getByText("Isolation").first()).toBeVisible();
    await expect(page.getByText("Duration").first()).toBeVisible();
    await expect(page.getByText("Created").first()).toBeVisible();

    // Output should show (decoded from base64)
    await expect(
      page.getByRole("heading", { name: "Output" }),
    ).toBeVisible();
    await expect(page.getByText("hello from isolate")).toBeVisible();
  });

  // AC7: Workload detail page streams logs in real time via SSE when running.
  test("AC7: log streaming displays logs", async ({ page }) => {
    // Submit a workload
    await page.goto("/workloads/new");
    await page.locator("select").first().selectOption("node");
    const editor = page.locator('[role="textbox"]');
    await editor.click();
    await editor.fill('console.log("log test")');
    await page.locator('button[type="submit"]').click();
    await page.waitForURL(/\/workloads\/[A-Z0-9]+/i, { timeout: 10_000 });

    // Logs heading should be present
    await expect(
      page.getByRole("heading", { name: "Logs" }),
    ).toBeVisible();

    // Wait for workload to complete
    await expect(page.getByText("completed")).toBeVisible({
      timeout: 15_000,
    });

    // Log viewer component should be present
    const logViewer = page.locator(".font-mono").last();
    await expect(logViewer).toBeVisible();
  });

  // AC8: The UI gracefully handles backends that aren't available yet.
  test("AC8: unavailable backends shown as disabled", async ({ page }) => {
    await page.goto("/workloads/new");

    // The stub test server only registers isolate and microvm backends.
    // gvisor should show as "not available"
    const isolationSelect = page.locator("select").nth(1);
    await expect(isolationSelect).toBeVisible();

    // Check that gvisor option contains "not available" text
    const gvisorOption = isolationSelect.locator('option[value="gvisor"]');
    const gvisorText = await gvisorOption.textContent();
    expect(gvisorText).toContain("not available");

    // The gvisor option should be disabled
    await expect(gvisorOption).toBeDisabled();
  });
});
