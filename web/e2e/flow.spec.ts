import { expect, test, type Browser, type Page } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { strFromU8, unzipSync } from "fflate";

const BASE = "http://localhost:5174";
const PASSWORD = "password123";
const CRITERIA = ["Content", "Delivery"]; // 60 + 40 points

async function login(page: Page, username: string, password: string) {
	await page.goto("/login");
	await page.getByLabel("Username").fill(username);
	await page.getByLabel("Password").fill(password);
	await page.getByRole("button", { name: "Log in" }).click();
}

async function newSession(browser: Browser, username: string, password = PASSWORD) {
	const context = await browser.newContext({ baseURL: BASE });
	const page = await context.newPage();
	await login(page, username, password);
	return page;
}

async function addUser(page: Page, first: string, last: string, username: string) {
	await page.getByRole("button", { name: "New user" }).click();
	const dialog = page.getByRole("dialog");
	await dialog.getByLabel("First name").fill(first);
	await dialog.getByLabel("Last name").fill(last);
	await dialog.getByLabel("Username").fill(username);
	await dialog.getByLabel("Password").fill(PASSWORD);
	await dialog.getByRole("button", { name: "Save" }).click();
	await expect(page.getByRole("cell", { name: username, exact: true })).toBeVisible();
}

async function addCandidate(page: Page, first: string, last: string) {
	await page.getByRole("button", { name: "New candidate" }).click();
	const dialog = page.getByRole("dialog");
	await dialog.getByLabel("First name").fill(first);
	await dialog.getByLabel("Last name").fill(last);
	await dialog.getByRole("button", { name: "Save" }).click();
	await expect(page.getByRole("cell", { name: `${last}, ${first}` })).toBeVisible();
}

async function addInterview(page: Page, title: string, mode: "1-on-1" | "Panel", judges: string[]) {
	await page.getByRole("button", { name: "New interview" }).click();
	const dialog = page.getByRole("dialog");
	await dialog.getByLabel("Title").fill(title);
	if (mode === "Panel") {
		await dialog.getByRole("combobox", { name: "Mode" }).click();
		await page.getByRole("option", { name: "Panel" }).click();
	}
	await dialog.getByRole("button", { name: "Save" }).click();
	const card = page.locator("section", { has: page.getByRole("heading", { name: title }) });
	await expect(card).toBeVisible();

	await card.getByRole("button", { name: "Judges" }).click();
	const judgesDialog = page.getByRole("dialog");
	for (const j of judges) {
		await judgesDialog.getByRole("checkbox", { name: new RegExp(`\\(${j}\\)`) }).click();
	}
	await judgesDialog.getByRole("button", { name: "Save" }).click();
	await expect(card.getByText(`0 of`).first()).toBeVisible();
}

async function score(page: Page, candidate: string, values: number[], mode: "Submit" | "Update") {
	await page.getByRole("option", { name: new RegExp(candidate) }).click();
	for (const [i, v] of values.entries()) {
		await page.getByLabel(`${CRITERIA[i]} score`, { exact: true }).fill(String(v));
	}
	await page.getByRole("button", { name: `${mode} scores` }).click();
	await page.getByRole("alertdialog").getByRole("button", { name: mode, exact: true }).click();
	await expect(
		page.getByText(mode === "Submit" ? "Scores submitted." : "Scores updated.")
	).toBeVisible();
}

const row = (page: Page, last: string) => page.getByRole("row", { name: new RegExp(last) });

test("a whole interview day", async ({ browser, page }) => {
	page.on("pageerror", (e) => {
		throw e;
	});
	await test.step("admin sets everything up", async () => {
		await login(page, "admin", "e2e-password");
		await expect(page).toHaveURL(/\/admin\/scores/);

		await page.getByRole("link", { name: "Users" }).click();
		for (const n of [1, 2, 3, 4]) {
			await addUser(page, `Judge${n}`, "Tester", `judge${n}`);
		}

		await page.getByRole("link", { name: "Candidates" }).click();
		await addCandidate(page, "Alice", "Reyes");
		await addCandidate(page, "Ben", "Santos");

		// CSV import: preview first, then save.
		await page.getByRole("button", { name: "Import CSV" }).click();
		const dialog = page.getByRole("dialog");
		await dialog.getByLabel("CSV file").setInputFiles({
			name: "c.csv",
			mimeType: "text/csv",
			buffer: Buffer.from("first_name,last_name\nCara,Dela Cruz\n")
		});
		await dialog.getByRole("button", { name: "Preview" }).click();
		await expect(dialog.getByText("1 rows, 0 with errors.")).toBeVisible();
		await dialog.getByRole("button", { name: "Import 1 candidates" }).click();
		await expect(page.getByRole("cell", { name: "Dela Cruz, Cara" })).toBeVisible();

		await page.getByRole("link", { name: "Rubrics" }).click();
		await page.getByLabel("Rubric name").fill("Interview Rubric");
		for (const [i, [title, points]] of [
			[CRITERIA[0], 60],
			[CRITERIA[1], 40]
		].entries()) {
			await page.getByRole("button", { name: "+ Add criterion" }).click();
			await page.getByLabel(`Criterion ${i + 1} title`).fill(String(title));
			await page.getByLabel(`Criterion ${i + 1} points`).fill(String(points));
		}
		await expect(page.getByRole("button", { name: "Save rubric" })).toBeEnabled();
		await page.getByRole("button", { name: "Save rubric" }).click();
		await expect(page.getByText("Rubric saved.")).toBeVisible();

		// Coming back to a saved rubric must show it, not loop.
		await page.getByRole("link", { name: "Users" }).click();
		await page.getByRole("link", { name: "Rubrics" }).click();
		await expect(page.getByLabel("Rubric name")).toHaveValue("Interview Rubric");
		await expect(page.getByLabel("Criterion 2 points")).toHaveValue("40");

		await page.getByRole("link", { name: "Interviews" }).click();
		await addInterview(page, "One on one", "1-on-1", ["judge1"]);
		await addInterview(page, "Panel round", "Panel", ["judge2", "judge3", "judge4"]);
	});

	// The admin watches the table in this tab for the rest of the day.
	await page.getByRole("link", { name: "Scores" }).click();
	await expect(page.getByRole("heading", { name: "Candidates & Scores" })).toBeVisible();
	await expect(row(page, "Reyes")).toContainText("Pending");

	const judge1 = await newSession(browser, "judge1");
	await test.step("judge 1 scores the 1-on-1 and the admin sees it live", async () => {
		await expect(judge1).toHaveURL(/\/judge\/[0-9a-f-]+$/);
		await score(judge1, "Reyes", [50, 30], "Submit");
		await expect(row(page, "Reyes")).toContainText("80");
	});

	const panel = await Promise.all(
		["judge2", "judge3", "judge4"].map((u) => newSession(browser, u))
	);
	await test.step("three panel judges score the same candidate", async () => {
		const totals = [
			[60, 40],
			[50, 40],
			[40, 40]
		];
		for (const [i, p] of panel.entries()) {
			await score(p, "Reyes", totals[i], "Submit");
		}
		// (80 + 100 + 90 + 80) / 4
		await expect(row(page, "Reyes")).toContainText("87.50");
	});

	await test.step("a judge edits a score and the average moves", async () => {
		await judge1.getByRole("option", { name: /Reyes/ }).click();
		await expect(judge1.getByText("Scored 80/100")).toBeVisible();
		await judge1.getByLabel("Content score", { exact: true }).fill("60");
		await judge1.getByRole("button", { name: "Update scores" }).click();
		await judge1
			.getByRole("alertdialog")
			.getByRole("button", { name: "Update", exact: true })
			.click();
		await expect(judge1.getByText("Scores updated.")).toBeVisible();
		// (90 + 100 + 90 + 80) / 4
		await expect(row(page, "Reyes")).toContainText("90.00");
	});

	await test.step("a second tab of the same judge sees the saved score", async () => {
		const other = await newSession(browser, "judge1"); // same judge, second tab: sees its own
		await expect(other.getByRole("option", { name: /Reyes/ })).toContainText("Scored 90/100");
	});

	await test.step("a draft survives a reload", async () => {
		const p = panel[1];
		await p.getByRole("option", { name: /Santos/ }).click();
		await p.getByLabel("Content score", { exact: true }).fill("33");
		await p.reload();
		await expect(p.getByText("Draft restored")).toBeVisible();
		await expect(p.getByLabel("Content score", { exact: true })).toHaveValue("33");
	});

	await test.step("export downloads a workbook with every candidate", async () => {
		const download = page.waitForEvent("download");
		await page.getByRole("link", { name: "Export .xlsx" }).click();
		const file = await download;
		expect(file.suggestedFilename()).toMatch(/^results-\d{4}-\d{2}-\d{2}-\d{4}\.xlsx$/);
		const files = unzipSync(new Uint8Array(await readFile(await file.path())));
		expect(Object.keys(files)).toContain("xl/workbook.xml");
		const text = strFromU8(files["xl/sharedStrings.xml"]);
		for (const name of ["Reyes", "Santos", "Dela Cruz", "Summary"]) {
			expect(text + strFromU8(files["xl/workbook.xml"])).toContain(name);
		}
	});
});
