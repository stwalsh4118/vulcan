import { test, expect } from "@playwright/test";

const API_BASE = "http://localhost:8080";

/**
 * Submit an async microvm workload via the API and return its ID.
 */
async function submitMicrovmWorkload(): Promise<string> {
  const resp = await fetch(`${API_BASE}/v1/workloads/async`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ runtime: "go", isolation: "microvm" }),
  });
  if (!resp.ok) throw new Error(`submitMicrovmWorkload: ${resp.status}`);
  const body = await resp.json();
  return body.id as string;
}

/**
 * Poll the API until the workload reaches the expected status.
 */
async function pollUntilStatus(
  id: string,
  status: string,
  timeoutMs = 15_000,
): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const resp = await fetch(`${API_BASE}/v1/workloads/${id}`);
    const body = await resp.json();
    if (body.status === status) return;
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error(
    `Workload ${id} did not reach status "${status}" within ${timeoutMs}ms`,
  );
}

test.describe("PBI 4: Firecracker microVM Backend", () => {
  // AC1: The microvm backend appears in the backends list and isolation dropdown.
  test("AC1: microvm backend appears in isolation dropdown", async ({
    page,
  }) => {
    await page.goto("/workloads/new");

    const isolationSelect = page.locator("select").nth(1);
    await expect(isolationSelect).toBeVisible();

    const options = await isolationSelect
      .locator("option")
      .allTextContents();

    // Should have descriptive label "MicroVM (Firecracker)" not just "microvm"
    const microvmOption = options.find((o) => o.includes("MicroVM"));
    expect(microvmOption).toBeDefined();
    expect(microvmOption).toContain("Firecracker");
  });

  // AC8: Submit a microvm workload from the frontend and see the result.
  test("AC8: submit microvm workload via UI and view result", async ({
    page,
  }) => {
    await page.goto("/workloads/new");

    // Select runtime = go (routed to microvm via auto)
    await page.locator("select").first().selectOption("go");

    // Select isolation = microvm
    const isolationSelect = page.locator("select").nth(1);
    await isolationSelect.selectOption("microvm");

    // Enter inline code
    const editor = page.locator('[role="textbox"]');
    await editor.click();
    await editor.fill('fmt.Println("hello from firecracker")');

    // Submit
    await page.locator('button[type="submit"]').click();

    // Should navigate to detail page
    await page.waitForURL(/\/workloads\/[A-Z0-9]+/i, { timeout: 10_000 });

    // Verify the workload completes
    await expect(page.getByText("completed")).toBeVisible({
      timeout: 15_000,
    });

    // Verify isolation shows "microvm"
    await expect(page.getByText("microvm")).toBeVisible();

    // Verify MicroVM Details section appears
    await expect(
      page.getByRole("heading", { name: "MicroVM Details" }),
    ).toBeVisible();

    // Verify Firecracker badge
    await expect(page.getByText("Firecracker")).toBeVisible();
  });

  // AC8: Code source toggle between inline and archive modes.
  test("AC8: code source toggle between inline and archive", async ({
    page,
  }) => {
    await page.goto("/workloads/new");

    // Inline mode is the default — code editor should be visible
    await expect(page.locator('[role="textbox"]')).toBeVisible();

    // Click "Upload Archive" button
    await page.locator("button", { hasText: "Upload Archive" }).click();

    // Code editor should be hidden, file upload area should appear
    await expect(page.locator('[role="textbox"]')).not.toBeVisible();
    await expect(page.getByText("Click or drag a .tar.gz file here")).toBeVisible();

    // Click "Inline Code" to switch back
    await page.locator("button", { hasText: "Inline Code" }).click();

    // Code editor should reappear
    await expect(page.locator('[role="textbox"]')).toBeVisible();
    await expect(
      page.getByText("Click or drag a .tar.gz file here"),
    ).not.toBeVisible();
  });

  // AC8: Detail page shows MicroVM Details for microvm workloads.
  test("AC8: detail page shows MicroVM details section", async ({ page }) => {
    // Submit via API so we control the isolation mode.
    const id = await submitMicrovmWorkload();
    await pollUntilStatus(id, "completed");

    await page.goto(`/workloads/${id}`);

    // Wait for the page to load the workload data.
    await expect(page.getByText("completed")).toBeVisible({ timeout: 10_000 });

    // MicroVM Details section should be visible.
    await expect(
      page.getByRole("heading", { name: "MicroVM Details" }),
    ).toBeVisible();

    // Should show Firecracker badge.
    await expect(page.getByText("Firecracker")).toBeVisible();

    // Should show runtime badge.
    await expect(
      page.locator("section").filter({ hasText: "MicroVM Details" }).getByText("go"),
    ).toBeVisible();
  });

  // AC3 + AC8: Log streaming shows microvm log lines.
  test("AC8: microvm log lines displayed", async ({ page }) => {
    const id = await submitMicrovmWorkload();
    await pollUntilStatus(id, "completed");

    await page.goto(`/workloads/${id}`);

    await expect(page.getByText("completed")).toBeVisible({ timeout: 10_000 });

    // Logs section should be visible.
    await expect(
      page.getByRole("heading", { name: "Logs" }),
    ).toBeVisible();

    // The testserver emits "[microvm] booting vm", "[microvm] executing", "[microvm] done"
    const logViewer = page.locator(".font-mono").last();
    await expect(logViewer).toBeVisible();
    await expect(logViewer.getByText("[microvm] booting vm")).toBeVisible({
      timeout: 5_000,
    });
    await expect(logViewer.getByText("[microvm] done")).toBeVisible();
  });

  // Non-microvm workload should NOT show MicroVM Details section.
  test("detail page hides MicroVM details for non-microvm workloads", async ({
    page,
  }) => {
    // Submit an isolate workload.
    const resp = await fetch(`${API_BASE}/v1/workloads/async`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ runtime: "node", isolation: "isolate" }),
    });
    const body = await resp.json();
    const id = body.id as string;
    await pollUntilStatus(id, "completed");

    await page.goto(`/workloads/${id}`);
    await expect(page.getByText("completed")).toBeVisible({ timeout: 10_000 });

    // MicroVM Details section should NOT appear.
    await expect(
      page.getByRole("heading", { name: "MicroVM Details" }),
    ).not.toBeVisible();
  });
});
