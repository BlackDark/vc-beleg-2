import { useQuery } from "@tanstack/react-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { AlertTriangleIcon, ChevronRightIcon, FolderIcon, MailIcon, ReceiptIcon } from "lucide-react";
import { listReceiptsOptions } from "@/api/gen/@tanstack/react-query.gen";
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
import { EmptyState, Skeleton } from "@/components/ui/misc";
import { currentMonth, formatWeekday } from "@/lib/format";
import { cn } from "@/lib/utils";
import { m } from "@/paraglide/messages";

type Status = "" | "pending" | "checked";
type Search = { month?: string; status?: Status };

export const Route = createFileRoute("/_app/receipts/")({
  validateSearch: (s: Record<string, unknown>): Search => ({
    month: typeof s.month === "string" && /^\d{4}-\d{2}$/.test(s.month) ? s.month : undefined,
    status: s.status === "pending" || s.status === "checked" ? s.status : undefined,
  }),
  component: Receipts,
});

const filters: { value: Status; label: () => string }[] = [
  { value: "", label: m.filter_all },
  { value: "pending", label: m.receipt_status_pending },
  { value: "checked", label: m.receipt_status_checked },
];

function Receipts() {
  const search = Route.useSearch();
  const month = search.month ?? currentMonth();
  const status = search.status ?? "";
  const navigate = useNavigate({ from: Route.fullPath });
  const q = useQuery({
    ...listReceiptsOptions({ query: { month, status } }),
    refetchInterval: (q) => (q.state.data?.some((r) => isExtractionActive(r.extraction)) ? 2000 : false),
  });
  const receipts = q.data ?? [];

  return (
    <>
      <PageHeader title={m.nav_receipts()} actions={<ReceiptCapture variant="outline" />} />
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <MonthSwitcher
          month={month}
          onChange={(mo) => navigate({ search: { month: mo, status: search.status } })}
        />
        <fieldset className="flex rounded-lg border p-0.5">
          <legend className="sr-only">{m.filter_status()}</legend>
          {filters.map((f) => (
            <button
              key={f.value}
              type="button"
              aria-pressed={status === f.value}
              onClick={() => navigate({ search: { month: search.month, status: f.value || undefined } })}
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
      {q.isPending ? (
        <Skeleton className="h-40" />
      ) : receipts.length === 0 ? (
        <EmptyState icon={ReceiptIcon} title={m.receipts_empty()} text={m.receipts_empty_text()} />
      ) : (
        <Card className="divide-y overflow-hidden p-0">
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
