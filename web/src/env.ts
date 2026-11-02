import { defineEnvVars } from "@sveltejs/kit/env";

export const variables = defineEnvVars({
	BACKEND_URL: {
		public: true,
		static: true,
		description: "Backend server base URL. Empty means same origin."
	}
});
