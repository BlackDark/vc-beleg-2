import { QueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { client } from "@/api/gen/client.gen";
import { m } from "@/paraglide/messages";

client.setConfig({ baseUrl: "", credentials: "same-origin", throwOnError: true });

/** Error body of the API (see internal/api ErrorModel). */
export type ApiError = {
  status?: number;
  code?: string;
  message?: string;
  params?: Record<string, string>;
  errors?: { message?: string; location?: string }[];
};

type MessageFn = (params?: Record<string, string>) => string;
const messages = m as unknown as Record<string, MessageFn | undefined>;

/** Translates an API error code into the UI language. */
export function errorText(err: unknown): string {
  const e = (err ?? {}) as ApiError;
  const fn = e.code ? messages[`err_${e.code.replace(/[.-]/g, "_")}`] : undefined;
  if (fn) return fn(e.params ?? {});
  if (e.code === "validation" && e.errors?.length) {
    return `${m.err_validation()} (${e.errors.map((x) => x.location?.replace(/^body\./, "") ?? x.message).join(", ")})`;
  }
  if (e.status === 401) return m.err_unauthorized();
  return e.message ?? m.err_unknown();
}

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: (count, err) => (err as ApiError)?.status === undefined && count < 2,
      staleTime: 10_000,
    },
    mutations: {
      onError: (err) => {
        toast.error(errorText(err));
      },
    },
  },
});
