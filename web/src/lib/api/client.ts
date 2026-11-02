import createClient from "openapi-fetch";
import { BACKEND_URL } from "$app/env/public";
import type { paths } from "./schema";

export const api = createClient<paths>({ baseUrl: BACKEND_URL, credentials: "include" });
