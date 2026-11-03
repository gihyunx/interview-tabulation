import { describe, expect, it } from "vitest";
import { matchScore, normalize } from "./search";

describe("normalize", () => {
	it("drops accents, case and punctuation", () =>
		expect(normalize("  José-María  ")).toBe("jose maria"));
});

describe("matchScore", () => {
	const score = (q: string) => matchScore(q, "Cruz, José Maria");

	it("matches everything on an empty query", () => expect(score("")).toBeGreaterThan(0));
	it("ignores accents and case", () => expect(score("jose")).toBeGreaterThan(0));
	it("matches word order either way", () => {
		expect(score("maria cruz")).toBeGreaterThan(0);
		expect(score("cruz maria")).toBeGreaterThan(0);
	});
	it("tolerates typos in longer words", () => {
		expect(score("crux")).toBeGreaterThan(0);
		expect(score("mraia")).toBeGreaterThan(0);
	});
	it("does not tolerate typos in very short words", () => expect(score("cz")).toBe(0));
	it("rejects when any word is missing", () => expect(score("cruz santos")).toBe(0));
	it("ranks exact and prefix above fuzzy", () => {
		expect(matchScore("cruz", "Cruz")).toBeGreaterThan(matchScore("cruz", "Cruzado"));
		expect(matchScore("cruz", "Cruzado")).toBeGreaterThan(matchScore("cruz", "Crus"));
	});
});
