/** Lowercase, accents stripped, punctuation turned into spaces. */
export function normalize(s: string): string {
	return s
		.normalize("NFD")
		.replace(/\p{M}/gu, "")
		.toLowerCase()
		.replace(/[^\p{L}\p{N}]+/gu, " ")
		.trim();
}

/** Edit distance counting a swap of two neighbours as one edit. */
function distance(a: string, b: string): number {
	const d: number[][] = Array.from({ length: a.length + 1 }, (_, i) => [i]);
	for (let j = 1; j <= b.length; j++) {
		d[0][j] = j;
	}
	for (let i = 1; i <= a.length; i++) {
		for (let j = 1; j <= b.length; j++) {
			d[i][j] = Math.min(
				d[i - 1][j] + 1,
				d[i][j - 1] + 1,
				d[i - 1][j - 1] + (a[i - 1] === b[j - 1] ? 0 : 1)
			);
			if (i > 1 && j > 1 && a[i - 1] === b[j - 2] && a[i - 2] === b[j - 1]) {
				d[i][j] = Math.min(d[i][j], d[i - 2][j - 2] + 1);
			}
		}
	}
	return d[a.length][b.length];
}

/** Score of one query word against one name word; 0 means no match. */
function wordScore(q: string, w: string): number {
	if (w === q) {
		return 100;
	}
	if (w.startsWith(q)) {
		return 80;
	}
	if (w.includes(q)) {
		return 50;
	}
	// Typos: allow 1 edit for short words, 2 for longer ones, against the same-length prefix too.
	const allowed = q.length <= 3 ? 0 : q.length <= 6 ? 1 : 2;
	if (allowed === 0) {
		return 0;
	}
	const best = Math.min(distance(q, w), distance(q, w.slice(0, q.length)));
	return best <= allowed ? 30 - best : 0;
}

/**
 * How well a name matches a search. Every word typed must match some word in
 * the name (any order). 0 = no match; higher is better. An empty query matches all.
 */
export function matchScore(query: string, name: string): number {
	const qs = normalize(query).split(" ").filter(Boolean);
	if (qs.length === 0) {
		return 1;
	}
	const words = normalize(name).split(" ").filter(Boolean);
	let total = 0;
	for (const q of qs) {
		const best = Math.max(0, ...words.map((w) => wordScore(q, w)));
		if (best === 0) {
			return 0;
		}
		total += best;
	}
	return total / qs.length;
}
