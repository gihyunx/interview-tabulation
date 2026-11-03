import { describe, expect, it } from "vitest";
import { clampScore, totalOf } from "./scoring";

describe("clampScore", () => {
	it("keeps values in range", () => expect(clampScore(7, 10)).toBe(7));
	it("clamps above max and below zero", () => {
		expect(clampScore(11, 10)).toBe(10);
		expect(clampScore(-3, 10)).toBe(0);
	});
	it("rounds to integers", () => expect(clampScore(6.6, 10)).toBe(7));
	it("treats NaN and Infinity as zero", () => {
		expect(clampScore(NaN, 10)).toBe(0);
		expect(clampScore(Infinity, 10)).toBe(0);
	});
});

describe("totalOf", () => {
	it("sums and ignores unset values", () => expect(totalOf([10, undefined, 5])).toBe(15));
	it("is zero when empty", () => expect(totalOf([])).toBe(0));
});
