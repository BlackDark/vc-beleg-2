import { useQuery } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import { FileTextIcon } from "lucide-react";
import { listMonthsOptions } from "@/api/gen/@tanstack/react-query.gen";
import { Money, MonthStatusBadge, PageHeader } from "@/components/common";
import { Card } from "@/components/ui/card";
import { EmptyState, Skeleton } from "@/components/ui/misc";
import { formatDate, formatMonth } from "@/lib/format";
import { m } from "@/paraglide/messages";

export const Route = createFileRoute("/_app/reports")({ component: Reports });

function Reports() {
  const q = useQuery(listMonthsOptions());
  const months = q.data ?? [];
  return (
    <>
      <PageHeader title={m.nav_reports()} description={m.reports_subtitle()} />
      {q.isPending ? (
        <Skeleton className="h-40" />
      ) : months.length === 0 ? (
        <EmptyState icon={FileTextIcon} title={m.reports_empty()} text={m.reports_empty_text()} />
      ) : (
        <Card className="divide-y overflow-hidden p-0">
          {months.map((mo) => (
            <div key={mo.month} className="grid gap-2 px-4 py-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <Link to="/month/$month" params={{ month: mo.month }} className="font-medium hover:underline">
                  {formatMonth(mo.month)}
                </Link>
                <span className="flex items-center gap-3">
                  <Money cents={mo.payout} strong />
                  <MonthStatusBadge status={mo.status} />
                </span>
              </div>
              {mo.paidOn ? (
                <p className="text-xs text-muted-foreground">{m.paid_on({ date: formatDate(mo.paidOn) })}</p>
              ) : null}
              {mo.reports.length ? (
                <ul className="flex flex-wrap gap-2">
                  {mo.reports.map((r) => (
                    <li key={r.id}>
                      <a
                        href={`/api/reports/${r.id}/pdf`}
                        download
                        title={r.reopenReason || undefined}
                        className="inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1 text-xs hover:bg-accent"
                      >
                        <FileTextIcon className="size-3.5" />
                        {m.report_version({ version: String(r.version), date: formatDate(r.createdAt) })}
                      </a>
                    </li>
                  ))}
                </ul>
              ) : null}
            </div>
          ))}
        </Card>
      )}
    </>
  );
}
