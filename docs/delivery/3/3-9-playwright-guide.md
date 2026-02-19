# @playwright/test — Usage Guide

**Date**: 2026-02-19
**Version**: 1.58.2
**Docs**: https://playwright.dev/docs/intro

## Installation

```bash
pnpm add -D @playwright/test
npx playwright install chromium
```

## Configuration (playwright.config.ts)

```typescript
import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  use: {
    baseURL: "http://localhost:3000",
  },
  webServer: [
    {
      command: "pnpm dev",
      url: "http://localhost:3000",
      reuseExistingServer: !process.env.CI,
    },
  ],
});
```

## Basic Test

```typescript
import { test, expect } from "@playwright/test";

test("home page loads", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveTitle("Vulcan");
});
```

## Key APIs

- `page.goto(url)` — navigate
- `page.locator(selector)` — find elements
- `expect(locator).toBeVisible()` — assertion
- `expect(locator).toHaveText()` — text assertion
- `page.fill(selector, value)` — fill input
- `page.click(selector)` — click element
- `page.selectOption(selector, value)` — select dropdown
- `page.waitForURL(pattern)` — wait for navigation
