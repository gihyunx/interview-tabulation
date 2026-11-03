/** Scores are integers from 0 to the criterion's max. Anything else is clamped. */
export function clampScore(value: number, max: number): number {
	if (!Number.isFinite(value)) {
		return 0;
	}
	return Math.min(max, Math.max(0, Math.round(value)));
}

export function totalOf(values: Iterable<number | undefined>): number {
	let sum = 0;
	for (const v of values) {
		sum += v ?? 0;
	}
	return sum;
}
