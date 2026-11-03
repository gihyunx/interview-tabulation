import { api, errorMessage } from "#lib/api/client.ts";

/** Unwraps an openapi-fetch result: data, or an Error with a readable message. */
function unwrap<T>(r: { data?: T; error?: unknown }, fallback: string): T {
	if (r.error || r.data === undefined) {
		throw new Error(errorMessage(r.error, fallback));
	}
	return r.data;
}

export const usersQuery = () => ({
	queryKey: ["users"],
	queryFn: async () => unwrap(await api.GET("/v1/users"), "Could not load users.") ?? []
});

export const candidatesQuery = () => ({
	queryKey: ["candidates"],
	queryFn: async () => unwrap(await api.GET("/v1/candidates"), "Could not load candidates.") ?? []
});

export const rubricsQuery = () => ({
	queryKey: ["rubrics"],
	queryFn: async () => unwrap(await api.GET("/v1/rubrics"), "Could not load rubrics.") ?? []
});

export const interviewsQuery = () => ({
	queryKey: ["interviews"],
	queryFn: async () => unwrap(await api.GET("/v1/interviews"), "Could not load interviews.") ?? []
});

export const myInterviewsQuery = () => ({
	queryKey: ["my-interviews"],
	queryFn: async () =>
		unwrap(await api.GET("/v1/me/interviews"), "Could not load your interviews.") ?? []
});

export const boardQuery = (interviewId: string) => ({
	queryKey: ["board", interviewId],
	queryFn: async () =>
		unwrap(
			await api.GET("/v1/interviews/{id}/board", { params: { path: { id: interviewId } } }),
			"Could not load candidates."
		)
});

export const resultsQuery = (interviewIds: string[]) => ({
	queryKey: ["results", interviewIds],
	queryFn: async () =>
		unwrap(
			await api.GET("/v1/results", { params: { query: { interview: interviewIds } } }),
			"Could not load results."
		)
});
