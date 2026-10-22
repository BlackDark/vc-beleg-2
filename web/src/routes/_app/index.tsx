import { useQuery } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import { ChevronRightIcon, ReceiptIcon } from "lucide-react";
import { getMonthOptions, listReceiptsOptions } from "@/api/gen/@tanstack/react-query.gen";
import { Money, MonthStatusBadge, PageHeader, Row, warningText } from "@/components/common";
import { ReceiptCapture } from "@/components/receipt-capture";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Alert, EmptyState, Skeleton } from "@/components/ui/misc";
import { currentMonth, formatDate, formatMonth } from "@/lib/format";
import { m } from "@/paraglide/messages";

export const Route = createFileRoute("/_app/")({ component: Start });

function Start() {
  const month = currentMonth();
  const monthQ = useQuery(getMonthOptions({ path: { month } }));
  const pendingQ = useQuery({
    ...listReceiptsOptions({ query: { status: "pending" } }),
    refetchInterval: (q) => (q.state.data?.some((r) => r.extraction === "pending") ? 2000 : false),
  });
  const mo = monthQ.data;
  const pending = pendingQ.data ?? [];
  const t = mo?.result.totals;
  const limit = mo?.result.dayLimit ?? 15;
  const days = t?.days ?? 0;

  return (
    <>
      <PageHeader title={formatMonth(month)} description={m.start_subtitle()} actions={<ReceiptCapture />} />
      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader className="flex-row items-center justify-between">
            <CardTitle>{m.start_progress()}</CardTitle>
            {mo ? <MonthStatusBadge status={mo.status} /> : null}
          </CardHeader>
          <CardContent className="grid gap-3">
            {monthQ.isPending ? (
              <Skeleton className="h-24" />
            ) : (
              <>
                <div className="flex items-baseline gap-2">
                  <span className="text-4xl font-semibold tabular">{days}</span>
                  <span className="text-muted-foreground">{m.start_of_days({ limit: String(limit) })}</span>
                </div>
                <div className="h-2 overflow-hidden rounded-full bg-muted" aria-hidden>
                  <div
                    className="h-full bg-primary"
                    style={{ width: `${Math.min(100, (days / limit) * 100)}%` }}
                  />
                </div>
                <Row label={m.payout()} strong>
                  <Money cents={t?.allowance} />
                </Row>
                <Row label={m.tax_free()}>
                  <Money cents={t?.taxFree} />
                </Row>
                <Row label={m.taxable()}>
                  <Money cents={t?.taxable} />
                </Row>
                {mo?.blockers.includes("values_missing") ? (
                  <Alert variant="warning">{warningText("values_missing")}</Alert>
                ) : null}
                <Link to="/month/$month" params={{ month }} className="text-sm font-medium text-primary">
                  {m.start_open_month()}
                </Link>
              </>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{m.start_to_check({ count: String(pending.length) })}</CardTitle>
          </CardHeader>
          <CardContent>
            {pendingQ.isPending ? (
              <Skeleton className="h-24" />
            ) : pending.length === 0 ? (
              <EmptyState
                icon={ReceiptIcon}
                title={m.start_nothing_to_check()}
                text={m.start_nothing_to_check_text()}
              />
            ) : (
              <ul className="-mx-2 grid">
                {pending.map((r) => (
                  <li key={r.id}>
                    <Link
                      to="/receipts/$id"
                      params={{ id: String(r.id) }}
                      className="flex items-center gap-3 rounded-lg px-2 py-2 hover:bg-accent"
                    >
                      <img
                        src={`/api/receipts/${r.id}/preview`}
                        alt=""
                        className="size-10 shrink-0 rounded border object-cover"
                      />
                      <span className="grid min-w-0 flex-1">
                        <span className="truncate text-sm font-medium">{r.merchant || r.filename}</span>
                        <span className="text-xs text-muted-foreground">
                          {r.extraction === "pending"
                            ? m.extraction_pending()
                            : formatDate(r.date) || m.no_date()}
                        </span>
                      </span>
                      {r.total ? <Money cents={r.total} className="text-sm" /> : null}
                      <ChevronRightIcon className="size-4 text-muted-foreground" />
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
      </div>
    </>
  );
}
