/**
 * The last non-null value of `get`. A dialog that closes by setting its subject to null keeps
 * showing it through the closing animation instead of flashing "undefined".
 */
export function latest<T>(get: () => T | null) {
	let last: T | null = null;
	const value = $derived.by(() => {
		last = get() ?? last;
		return last;
	});
	return {
		get current() {
			return value;
		}
	};
}
