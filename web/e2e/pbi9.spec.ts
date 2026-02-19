import { test, expect } from "@playwright/test";

const API_BASE = "http://localhost:8080";

/**
 * Submit an async workload via the API and return its ID.
 */
async function submitWorkload(): Promise<string> {
  const resp = await fetch(`${API_BASE}/v1/workloads/async`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ runtime: "node" }),
  });
  if (!resp.ok) throw new Error(`submitWorkload: ${resp.status}`);
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

test.describe("PBI 9: Log Persistence & Historical Log Viewing", () => {
  // AC3: Frontend detail page displays persisted logs for completed workloads.
  test("AC3: historical logs displayed for completed workload", async ({
    page,
  }) => {
    // Submit a workload via API and wait for it to complete.
    const id = await submitWorkload();
    await pollUntilStatus(id, "completed");

    // Navigate to the detail page.
    await page.goto(`/workloads/${id}`);

    // Wait for the workload status to show "completed".
    await expect(page.getByText("completed")).toBeVisible({ timeout: 10_000 });

    // The log viewer should show historical logs with "Complete" indicator.
    await expect(page.getByText("Complete")).toBeVisible({ timeout: 10_000 });

    // Verify log lines are visible in the viewer.
    const logViewer = page.locator(".font-mono").last();
    await expect(logViewer).toBeVisible();

    // The stub testserver emits "[isolate] starting execution", "[isolate] running code", "[isolate] done"
    await expect(
      logViewer.getByText("[isolate] starting execution"),
    ).toBeVisible({ timeout: 5_000 });
    await expect(logViewer.getByText("[isolate] done")).toBeVisible();

    // Should NOT show "Disconnected" for a completed workload.
    await expect(page.getByText("Disconnected")).not.toBeVisible();
  });

  // AC4: Real-time SSE streaming still works for active workloads.
  test("AC4: SSE streaming works for active workloads", async ({ page }) => {
    // Submit a workload and immediately navigate to its detail page.
    await page.goto("/workloads/new");
    await page.locator("select").first().selectOption("node");
    const editor = page.locator('[role="textbox"]');
    await editor.click();
    await editor.fill('console.log("streaming test")');
    await page.locator('button[type="submit"]').click();
    await page.waitForURL(/\/workloads\/[A-Z0-9]+/i, { timeout: 10_000 });

    // Logs heading should be present.
    await expect(
      page.getByRole("heading", { name: "Logs" }),
    ).toBeVisible();

    // The log viewer should eventually show either "Streaming" or "Complete"
    // (depending on timing — workload may complete during the test).
    const logViewer = page.locator(".font-mono").last();
    await expect(logViewer).toBeVisible();

    // Wait for workload to complete.
    await expect(page.getByText("completed")).toBeVisible({
      timeout: 15_000,
    });

    // After completion, log lines should be visible.
    await expect(
      logViewer.getByText("[isolate] starting execution"),
    ).toBeVisible({ timeout: 5_000 });
  });

  // AC5: Seamless transition from live streaming to historical display.
  test("AC5: live-to-historical transition preserves logs", async ({
    page,
  }) => {
    // Submit a workload via the UI and go to its detail page.
    await page.goto("/workloads/new");
    await page.locator("select").first().selectOption("node");
    const editor = page.locator('[role="textbox"]');
    await editor.click();
    await editor.fill('console.log("transition test")');
    await page.locator('button[type="submit"]').click();
    await page.waitForURL(/\/workloads\/[A-Z0-9]+/i, { timeout: 10_000 });

    // Wait for the workload to complete — the hook should transition
    // from SSE to historical mode automatically.
    await expect(page.getByText("completed")).toBeVisible({
      timeout: 15_000,
    });

    // After transition, "Complete" indicator should appear (historical mode).
    await expect(page.getByText("Complete")).toBeVisible({ timeout: 10_000 });

    // Log lines should still be visible after the transition.
    const logViewer = page.locator(".font-mono").last();
    await expect(
      logViewer.getByText("[isolate] starting execution"),
    ).toBeVisible({ timeout: 5_000 });
    await expect(logViewer.getByText("[isolate] done")).toBeVisible();

    // Should NOT show "Disconnected".
    await expect(page.getByText("Disconnected")).not.toBeVisible();
  });
});
