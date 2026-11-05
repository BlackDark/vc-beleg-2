import { useQuery } from "@tanstack/react-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { AlertTriangleIcon, ChevronRightIcon, FolderIcon, MailIcon, ReceiptIcon } from "lucide-react";
import { listReceiptsOptions } from "@/api/gen/@tanstack/react-query.gen";
import type { ListReceiptsData } from "@/api/gen/types.gen";
import {
  extractionText,
  isExtractionActive,
  Money,
  PageHeader,
  ReceiptStatusBadge,
  warningText,
} from "@/components/common";
import { MonthSwitcher } from "@/components/month-switcher";
import { ReceiptCapture } from "@/components/receipt-capture";
import { Card } from "@/components/ui/card";
import { Checkbox, Select } from "@/components/ui/input";
import { EmptyState, Skeleton } from "@/components/ui/misc";
import { currentMonth, formatWeekday } from "@/lib/format";
import { cn } from "@/lib/utils";
import { m } from "@/paraglide/messages";

type Status = "" | "pending" | "checked" | "undated";
// Derived from the generated API type: a new enum value breaks the Record below until it has a label.
type Query = NonNullable<ListReceiptsData["query"]>;
type Reading = Exclude<Query["reading"], "" | undefined>;
type Source = Exclude<Query["source"], "" | undefined>;
type Search = { month?: string; status?: Status; reading?: Reading; source?: Source; warnings?: true };

export const Route = createFileRoute("/_app/receipts/")({
  validateSearch: (s: Record<string, unknown>): Search => ({
    month: typeof s.month === "string" && /^\d{4}-\d{2}$/.test(s.month) ? s.month : undefined,
    status: s.status === "pending" || s.status === "checked" || s.status === "undated" ? s.status : undefined,
    reading: readings.find((v) => v === s.reading),
    source: sources.find((v) => v === s.source),
    warnings: s.warnings === true || s.warnings === "true" ? true : undefined,
  }),
  component: Receipts,
});

const filters: { value: Status; label: () => string }[] = [
  { value: "", label: m.filter_all },
  { value: "pending", label: m.receipt_status_pending },
  { value: "checked", label: m.receipt_status_checked },
  { value: "undated", label: m.filter_undated },
];

const readingLabels: Record<Reading, () => string> = {
  done: m.filter_reading_done,
  failed: m.filter_reading_failed,
  off: m.filter_reading_off,
  active: m.filter_reading_active,
};
const sourceLabels: Record<Source, () => string> = {
  upload: m.filter_source_upload,
  mail: m.filter_source_mail,
  folder: m.filter_source_folder,
  data: m.filter_source_data,
};

const readings = Object.keys(readingLabels) as Reading[];
const sources = Object.keys(sourceLabels) as Source[];

function Receipts() {
  const search = Route.useSearch();
  const month = search.month ?? currentMonth();
  const status = search.status ?? "";
  const navigate = useNavigate({ from: Route.fullPath });
  const q = useQuery({
    ...listReceiptsOptions({
      // "undated" is a view, not an API status: it maps to undated=true. The filters apply in both views.
      query: {
        ...(status === "undated" ? { undated: true } : { month, status }),
        reading: search.reading,
        source: search.source,
        warnings: search.warnings,
      },
    }),
    refetchInterval: (q) => (q.state.data?.some((r) => isExtractionActive(r.extraction)) ? 2000 : false),
  });
  const receipts = q.data ?? [];

  return (
    <>
      <PageHeader title={m.nav_receipts()} actions={<ReceiptCapture variant="outline" />} />
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        {status === "undated" ? (
          <p className="text-sm text-muted-foreground">{m.undated_hint()}</p>
        ) : (
          <MonthSwitcher month={month} onChange={(mo) => navigate({ search: { ...search, month: mo } })} />
        )}
        <fieldset className="flex rounded-lg border p-0.5">
          <legend className="sr-only">{m.filter_status()}</legend>
          {filters.map((f) => (
            <button
              key={f.value}
              type="button"
              aria-pressed={status === f.value}
              onClick={() => navigate({ search: { ...search, status: f.value || undefined } })}
              className={cn(
                "rounded-md px-3 py-1.5 text-sm text-muted-foreground",
                status === f.value && "bg-accent font-medium text-accent-foreground",
              )}
            >
              {f.label()}
            </button>
          ))}
        </fieldset>
      </div>
      <div className="mb-4 grid grid-cols-2 items-center gap-3 sm:flex sm:flex-wrap">
        <Select
          aria-label={m.filter_reading()}
          value={search.reading ?? ""}
          onChange={(e) =>
            navigate({ search: { ...search, reading: readings.find((v) => v === e.target.value) } })
          }
          className="sm:w-auto"
        >
          <option value="">
            {m.filter_reading()}: {m.filter_all()}
          </option>
          {readings.map((v) => (
            <option key={v} value={v}>
              {readingLabels[v]()}
            </option>
          ))}
        </Select>
        <Select
          aria-label={m.filter_source()}
          value={search.source ?? ""}
          onChange={(e) =>
            navigate({ search: { ...search, source: sources.find((v) => v === e.target.value) } })
          }
          className="sm:w-auto"
        >
          <option value="">
            {m.filter_source()}: {m.filter_all()}
          </option>
          {sources.map((v) => (
            <option key={v} value={v}>
              {sourceLabels[v]()}
            </option>
          ))}
        </Select>
        <label className="col-span-2 flex items-center gap-2 text-sm">
          <Checkbox
            checked={search.warnings === true}
            onChange={(e) => navigate({ search: { ...search, warnings: e.target.checked || undefined } })}
          />
          {m.filter_warnings()}
        </label>
      </div>
      {q.isPending ? (
        <Skeleton className="h-40" />
      ) : receipts.length === 0 ? (
        <EmptyState
          icon={ReceiptIcon}
          title={m.receipts_empty()}
          text={status === "undated" ? m.receipts_empty_undated_text() : m.receipts_empty_text()}
        />
      ) : (
        <Card
          // Bounded so header, month switcher and filter stay in view; focusable for keyboard scrolling.
          tabIndex={0}
          role="region"
          aria-label={m.nav_receipts()}
          className="max-h-[max(16rem,calc(100dvh-20rem))] divide-y overflow-y-auto p-0 outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
        >
          {receipts.map((r) => (
            <Link
              key={r.id}
              to="/receipts/$id"
              params={{ id: String(r.id) }}
              className="flex items-center gap-3 px-4 py-3 hover:bg-accent/60"
            >
              <img
                src={`/api/receipts/${r.id}/preview`}
                alt=""
                className="size-12 shrink-0 rounded border object-cover"
              />
              <span className="grid min-w-0 flex-1 gap-0.5">
                <span className="flex items-center gap-2 truncate text-sm font-medium">
                  {r.source === "mail" ? (
                    <MailIcon className="size-3.5 shrink-0 text-muted-foreground" />
                  ) : r.source === "folder" ? (
                    <FolderIcon className="size-3.5 shrink-0 text-muted-foreground" />
                  ) : null}
                  {r.merchant || r.filename}
                </span>
                <span className="text-xs text-muted-foreground">
                  {r.date ? formatWeekday(r.date) : m.no_date()}
                  {r.number ? ` · ${r.number}` : ""}
                  {extractionText(r.extraction) ? ` · ${extractionText(r.extraction)}` : ""}
                </span>
                {r.warnings.length ? (
                  <span className="flex items-center gap-1 text-xs text-warning">
                    <AlertTriangleIcon className="size-3" />
                    {r.warnings.map(warningText).join(", ")}
                  </span>
                ) : null}
              </span>
              <span className="grid justify-items-end gap-1">
                {r.total ? <Money cents={r.total} className="text-sm font-medium" /> : null}
                <ReceiptStatusBadge status={r.status} />
              </span>
              <ChevronRightIcon className="size-4 shrink-0 text-muted-foreground" />
            </Link>
          ))}
        </Card>
      )}
    </>
  );
}
