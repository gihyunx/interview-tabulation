/** Unsubmitted scores, kept in localStorage so a reload or crash loses nothing. */
export type Draft = Record<string, number>;

const key = (judgeId: string, interviewId: string, candidateId: string) =>
	`draft:${judgeId}:${interviewId}:${candidateId}`;

function store(): Storage | null {
	try {
		return globalThis.localStorage ?? null;
	} catch {
		return null;
	}
}

export function saveDraft(
	judgeId: string,
	interviewId: string,
	candidateId: string,
	draft: Draft,
	s: Storage | null = store()
) {
	try {
		s?.setItem(key(judgeId, interviewId, candidateId), JSON.stringify(draft));
	} catch {
		// Storage full or blocked: the draft just is not kept.
	}
}

export function loadDraft(
	judgeId: string,
	interviewId: string,
	candidateId: string,
	s: Storage | null = store()
): Draft | null {
	try {
		const raw = s?.getItem(key(judgeId, interviewId, candidateId));
		if (!raw) {
			return null;
		}
		const parsed: unknown = JSON.parse(raw);
		if (!parsed || typeof parsed !== "object") {
			return null;
		}
		const draft: Draft = {};
		for (const [k, v] of Object.entries(parsed)) {
			if (typeof v === "number" && Number.isFinite(v)) {
				draft[k] = v;
			}
		}
		return draft;
	} catch {
		return null;
	}
}

export function clearDraft(
	judgeId: string,
	interviewId: string,
	candidateId: string,
	s: Storage | null = store()
) {
	try {
		s?.removeItem(key(judgeId, interviewId, candidateId));
	} catch {
		// Nothing to clear.
	}
}
