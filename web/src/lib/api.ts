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
  entries?: {
    index: number;
    field: string;
    code: string;
    message: string;
    params?: Record<string, string>;
  }[];
};

type MessageFn = (params?: Record<string, string>) => string;
const messages = m as unknown as Record<string, MessageFn | undefined>;

const MAX_ENTRY_LINES = 6;

/** Translates an API error code into the UI language. */
export function errorText(err: unknown): string {
  const e = (err ?? {}) as ApiError;
  const fn = e.code ? messages[`err_${e.code.replace(/[.-]/g, "_")}`] : undefined;
  if (fn && e.entries?.length) {
    // A rejected data file: the reason per entry, one line each.
    const shown = e.entries.slice(0, MAX_ENTRY_LINES).map((x) => {
      const text =
        messages[`err_${x.code.replace(/[.-]/g, "_")}`]?.({ field: x.field, ...x.params }) ?? x.message;
      return m.data_entry({ n: String(x.index + 1), text });
    });
    const more = e.entries.length - shown.length;
    return [fn(e.params ?? {}), ...shown, ...(more > 0 ? [`… +${more}`] : [])].join("\n");
  }
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
