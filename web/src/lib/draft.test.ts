import { describe, expect, it } from "vitest";
import { clearDraft, loadDraft, saveDraft } from "./draft";

function fakeStorage(): Storage {
	const m = new Map<string, string>();
	return {
		get length() {
			return m.size;
		},
		clear: () => m.clear(),
		getItem: (k) => m.get(k) ?? null,
		key: (i) => [...m.keys()][i] ?? null,
		removeItem: (k) => void m.delete(k),
		setItem: (k, v) => void m.set(k, v)
	};
}

describe("drafts", () => {
	it("round-trips per judge, interview and candidate", () => {
		const s = fakeStorage();
		saveDraft("j", "i", "c1", { a: 5 }, s);
		saveDraft("j", "i", "c2", { a: 9 }, s);
		expect(loadDraft("j", "i", "c1", s)).toEqual({ a: 5 });
		expect(loadDraft("j", "i", "c2", s)).toEqual({ a: 9 });
		expect(loadDraft("other", "i", "c1", s)).toBeNull();
	});

	it("clears", () => {
		const s = fakeStorage();
		saveDraft("j", "i", "c", { a: 1 }, s);
		clearDraft("j", "i", "c", s);
		expect(loadDraft("j", "i", "c", s)).toBeNull();
	});

	it("ignores corrupt data", () => {
		const s = fakeStorage();
		s.setItem("draft:j:i:c", "{nope");
		expect(loadDraft("j", "i", "c", s)).toBeNull();
		s.setItem("draft:j:i:c", JSON.stringify({ a: "x", b: 3 }));
		expect(loadDraft("j", "i", "c", s)).toEqual({ b: 3 });
	});

	it("survives missing or throwing storage", () => {
		expect(() => saveDraft("j", "i", "c", { a: 1 }, null)).not.toThrow();
		expect(loadDraft("j", "i", "c", null)).toBeNull();
		const broken = {
			getItem() {
				throw new Error("blocked");
			},
			setItem() {
				throw new Error("full");
			}
		} as unknown as Storage;
		expect(() => saveDraft("j", "i", "c", { a: 1 }, broken)).not.toThrow();
		expect(loadDraft("j", "i", "c", broken)).toBeNull();
	});
});
