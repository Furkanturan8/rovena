import "server-only";
import type { ApiEnvelope } from "./types";

const API_BASE_URL = (process.env.API_BASE_URL ?? "http://localhost:18080").replace(/\/+$/, "");

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

interface Options {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  body?: unknown;
  token?: string;
  headers?: Record<string, string>;
  /** Seconds the Next data cache may serve this response; omit for no caching. */
  revalidate?: number;
}

/** Calls the gateway and unwraps the {success, data} envelope; non-2xx responses throw ApiError. */
export async function apiFetch<T>(path: string, opts: Options = {}): Promise<T> {
  const headers: Record<string, string> = { Accept: "application/json", ...opts.headers };
  if (opts.body !== undefined) headers["Content-Type"] = "application/json";
  if (opts.token) headers.Authorization = `Bearer ${opts.token}`;

  let res: Response;
  try {
    res = await fetch(`${API_BASE_URL}/api/v1${path}`, {
      method: opts.method ?? "GET",
      headers,
      body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
      ...(opts.revalidate !== undefined ? { next: { revalidate: opts.revalidate } } : { cache: "no-store" as const }),
    });
  } catch (e) {
    // Next signals "this route needs dynamic rendering" by throwing from fetch; that is not an outage.
    if (typeof e === "object" && e !== null && "digest" in e) throw e;
    throw new ApiError(503, "The service is not reachable right now, try again shortly");
  }

  if (res.status === 204) return undefined as T;
  const json = (await res.json().catch(() => null)) as ApiEnvelope<T> | null;
  if (!res.ok || !json?.success) {
    throw new ApiError(res.status, json?.message ?? `Request failed (${res.status})`);
  }
  return json.data;
}
