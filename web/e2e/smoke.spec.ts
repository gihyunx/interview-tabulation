import { expect, test } from "@playwright/test";

test("admin can log in and sees the scores page", async ({ page }) => {
	await page.goto("/");
	await expect(page).toHaveURL(/\/login$/);
	await page.getByLabel("Username").fill("admin");
	await page.getByLabel("Password").fill("e2e-password");
	await page.getByRole("button", { name: "Log in" }).click();
	await expect(page).toHaveURL(/\/admin\/scores/);
	await expect(page.getByRole("heading", { name: "Candidates & Scores" })).toBeVisible();
	await expect(page.getByRole("status").filter({ hasText: "Live" })).toBeVisible();
});
