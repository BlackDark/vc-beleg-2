import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { DownloadIcon, LockIcon, RotateCcwIcon } from "lucide-react";
import { useState } from "react";
import {
  closeMonthMutation,
  combineDayMutation,
  getMonthOptions,
  listReceiptsOptions,
  reopenMonthMutation,
  selectDayReceiptMutation,
  setMonthPaidMutation,
} from "@/api/gen/@tanstack/react-query.gen";
import type { Day, Month, Receipt } from "@/api/gen/types.gen";
import { Money, MonthStatusBadge, PageHeader, Row, taxModelText, warningText } from "@/components/common";
import { MonthSwitcher } from "@/components/month-switcher";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { Field, Input, Select } from "@/components/ui/input";
import { Alert, EmptyState, Skeleton } from "@/components/ui/misc";
import { formatCents, formatDate, formatMonth, formatWeekday, todayISO } from "@/lib/format";
import { cn } from "@/lib/utils";
import { m } from "@/paraglide/messages";

export const Route = createFileRoute("/_app/month/$month")({
  params: {
    parse: (p) => {
      if (!/^\d{4}-\d{2}$/.test(p.month)) throw new Error("invalid month");
      return { month: p.month };
    },
  },
  component: MonthPage,
});

function MonthPage() {
  const { month } = Route.useParams();
  const navigate = useNavigate();
  const q = useQuery(getMonthOptions({ path: { month } }));
  const mo = q.data;
  return (
    <>
      <PageHeader
        title={formatMonth(month)}
        description={mo ? taxModelText(mo.result.model) : undefined}
        actions={
          <MonthSwitcher
            month={month}
            onChange={(next) => navigate({ to: "/month/$month", params: { month: next } })}
          />
        }
      />
      {q.isPending || !mo ? <Skeleton className="h-96" /> : <MonthView month={mo} />}
    </>
  );
}

function MonthView({ month: mo }: { month: Month }) {
  const qc = useQueryClient();
  const onSuccess = (data: Month) => {
    qc.setQueryData(getMonthOptions({ path: { month: mo.month } }).queryKey, data);
    qc.invalidateQueries({ queryKey: [{ _id: "listMonths" }] });
    qc.invalidateQueries({ queryKey: [{ _id: "listReceipts" }] });
  };
  const close = useMutation({ ...closeMonthMutation(), onSuccess });
  const combine = useMutation({ ...combineDayMutation(), onSuccess });
  const select = useMutation({ ...selectDayReceiptMutation(), onSuccess });
  const receipts = useQuery(listReceiptsOptions({ query: { month: mo.month, status: "checked" } }));
  const choosing = combine.isPending || select.isPending || close.isPending;
  const t = mo.result.totals;
  const open = mo.status === "open";
  const latest = mo.reports[0];
  const blockers = mo.blockers.filter((b) => b !== "pending_receipts");

  return (
    <div className="grid gap-4">
      {mo.reopenReason ? (
        <Alert variant="info" title={m.month_reopened()}>
          {mo.reopenReason}
        </Alert>
      ) : null}
      {mo.pending > 0 ? (
        <Alert variant="warning" title={m.month_pending({ count: String(mo.pending) })}>
          <Link
            to="/receipts"
            search={{ month: mo.month, status: "pending" }}
            className="text-primary underline"
          >
            {m.month_pending_link()}
          </Link>
        </Alert>
      ) : null}
      {blockers.map((b) => (
        <Alert key={b} variant="warning">
          {warningText(b)}
        </Alert>
      ))}

      <div className="grid items-start gap-4 md:grid-cols-[minmax(0,1fr)_20rem]">
        <Card className="overflow-hidden p-0">
          {mo.result.days.length === 0 ? (
            <div className="p-4">
              <EmptyState icon={LockIcon} title={m.month_no_days()} text={m.month_no_days_text()} />
            </div>
          ) : (
            <ul className="divide-y">
              {mo.result.days.map((d) => (
                <DayRow
                  key={d.date}
                  day={d}
                  editable={open}
                  receipts={receipts.data}
                  busy={choosing}
                  onChoose={(choice) => {
                    if (choice === "combined") combine.mutate({ body: { date: d.date, combined: true } });
                    else select.mutate({ body: { date: d.date, receiptId: Number(choice) } });
                  }}
                />
              ))}
            </ul>
          )}
        </Card>

        <div className="grid content-start gap-4">
          <Card>
            <CardHeader className="flex-row items-center justify-between">
              <CardTitle>{m.summary()}</CardTitle>
              <MonthStatusBadge status={mo.status} />
            </CardHeader>
            <CardContent>
              <Row label={m.counted_days()}>
                {t.days} / {mo.result.dayLimit}
              </Row>
              <Row label={m.receipts_total()}>
                <Money cents={t.receipts} />
              </Row>
              <Row label={m.tax_free()}>
                <Money cents={t.taxFree} />
              </Row>
              <Row label={m.taxable()}>
                <Money cents={t.taxable} />
              </Row>
              <Row label={m.payout()} strong>
                <Money cents={t.allowance} />
              </Row>
              {mo.result.model === "A" ? (
                <div className="mt-2 border-t pt-2">
                  <Row label={m.flat_tax()}>
                    <Money cents={t.flatTax} />
                  </Row>
                  <Row label={m.soli()}>
                    <Money cents={t.soli} />
                  </Row>
                  <Row label={m.church_tax()}>
                    <Money cents={t.churchTax} />
                  </Row>
                  <Row label={m.company_tax()} strong>
                    <Money cents={t.companyTax} />
                  </Row>
                </div>
              ) : null}
            </CardContent>
          </Card>

          <Card>
            <CardContent className="grid gap-3 py-5">
              {open ? (
                <Button
                  size="lg"
                  disabled={mo.blockers.length > 0 || choosing}
                  onClick={() => {
                    if (window.confirm(m.close_confirm())) close.mutate({ path: { month: mo.month } });
                  }}
                >
                  <LockIcon />
                  {m.close_month()}
                </Button>
              ) : (
                <>
                  {latest ? (
                    <a
                      href={`/api/reports/${latest.id}/pdf`}
                      download
                      className={buttonVariants({ size: "lg" })}
                    >
                      <DownloadIcon />
                      {m.download_report({ version: String(latest.version) })}
                    </a>
                  ) : null}
                  <PaidControl month={mo} onSuccess={onSuccess} />
                  {mo.status === "closed" ? <ReopenDialog month={mo.month} onSuccess={onSuccess} /> : null}
                </>
              )}
              {mo.reports.length > 1 ? (
                <Link to="/reports" className="text-sm text-primary">
                  {m.all_versions({ count: String(mo.reports.length) })}
                </Link>
              ) : null}
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}

function DayRow({
  day: d,
  editable,
  receipts,
  busy,
  onChoose,
}: {
  day: Day;
  editable: boolean;
  receipts: Receipt[] | undefined;
  busy: boolean;
  onChoose: (choice: string) => void;
}) {
  const ids = [...d.receiptIds, ...d.ignored].sort((a, b) => a - b);
  const several = ids.length > 1;

  return (
    <li className={cn("grid gap-1 px-4 py-3", !d.counted && "text-muted-foreground")}>
      <div className="flex items-center justify-between gap-3">
        <span className="font-medium">{formatWeekday(d.date)}</span>
        <span className="flex items-center gap-3">
          <Money cents={d.total} className="text-sm text-muted-foreground" />
          {d.counted ? (
            <Money cents={d.allowance} strong />
          ) : (
            <Badge variant="outline">{m.not_counted()}</Badge>
          )}
        </span>
      </div>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
        {ids.map((id) => (
          <Link
            key={id}
            to="/receipts/$id"
            params={{ id: String(id) }}
            className={cn("max-w-full break-words underline", d.ignored.includes(id) && "line-through")}
          >
            {receiptLabel(id, receipts)}
          </Link>
        ))}
        {d.warnings
          .filter((w) => w !== "over_limit" && w !== "several_receipts")
          .map((w) => (
            <span key={w} className="text-warning">
              {w === "holiday" && d.holiday ? d.holiday : warningText(w)}
            </span>
          ))}
      </div>
      {several || d.selectedReceiptId ? (
        <DayReceiptSelection
          day={d}
          editable={editable}
          busy={busy}
          receipts={receipts}
          onChoose={onChoose}
        />
      ) : null}
    </li>
  );
}

function receiptLabel(id: number, receipts: Receipt[] | undefined): string {
  const receipt = receipts?.find((r) => r.id === id);
  return receipt
    ? `${receipt.number} · ${receipt.merchant || receipt.filename} · ${formatCents(receipt.total)}`
    : m.receipt_n({ n: String(id) });
}

function DayReceiptSelection({
  day: d,
  editable,
  busy,
  receipts,
  onChoose,
}: {
  day: Day;
  editable: boolean;
  busy: boolean;
  receipts: Receipt[] | undefined;
  onChoose: (choice: string) => void;
}) {
  const ids = [...d.receiptIds, ...d.ignored].sort((a, b) => a - b);
  return (
    <Field label={m.receipt_selection()} htmlFor={`receipt-choice-${d.date}`} className="mt-2">
      <Select
        id={`receipt-choice-${d.date}`}
        aria-label={m.receipt_selection_for_day({ date: formatDate(d.date) })}
        value={d.combined ? "combined" : String(d.selectedReceiptId ?? 0)}
        disabled={!editable || busy || !receipts}
        onChange={(e) => onChoose(e.target.value)}
      >
        <option value="0">{m.receipt_selection_auto()}</option>
        <option value="combined" disabled={ids.length < 2}>
          {m.combine()}
        </option>
        {ids.map((id) => (
          <option key={id} value={id}>
            {receiptLabel(id, receipts)}
          </option>
        ))}
      </Select>
      {d.combined ? <p className="text-xs text-warning">{m.combine_warning()}</p> : null}
    </Field>
  );
}

function PaidControl({ month: mo, onSuccess }: { month: Month; onSuccess: (m: Month) => void }) {
  const [date, setDate] = useState(todayISO());
  const paid = useMutation({ ...setMonthPaidMutation(), onSuccess });
  if (mo.status === "paid") {
    return (
      <div className="flex items-center justify-between gap-2 text-sm">
        <span>{m.paid_on({ date: formatDate(mo.paidOn) })}</span>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => paid.mutate({ path: { month: mo.month }, body: { paidOn: "" } })}
        >
          {m.undo()}
        </Button>
      </div>
    );
  }
  return (
    <form
      className="flex items-end gap-2"
      onSubmit={(e) => {
        e.preventDefault();
        paid.mutate({ path: { month: mo.month }, body: { paidOn: date } });
      }}
    >
      <Field label={m.paid_on_label()} htmlFor="paid" className="flex-1">
        <Input id="paid" type="date" value={date} onChange={(e) => setDate(e.target.value)} required />
      </Field>
      <Button type="submit" variant="outline" disabled={paid.isPending}>
        {m.mark_paid()}
      </Button>
    </form>
  );
}

function ReopenDialog({ month, onSuccess }: { month: string; onSuccess: (m: Month) => void }) {
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("");
  const reopen = useMutation({
    ...reopenMonthMutation(),
    onSuccess: (data) => {
      onSuccess(data);
      setOpen(false);
    },
  });
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <Button variant="outline" onClick={() => setOpen(true)}>
        <RotateCcwIcon />
        {m.reopen_month()}
      </Button>
      <DialogContent title={m.reopen_month()} description={m.reopen_text()}>
        <form
          className="grid gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            reopen.mutate({ path: { month }, body: { reason } });
          }}
        >
          <Field label={m.reason()} htmlFor="reason">
            <Input
              id="reason"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              required
              autoFocus
            />
          </Field>
          <Button type="submit" disabled={!reason.trim() || reopen.isPending}>
            {m.reopen_month()}
          </Button>
        </form>
      </DialogContent>
    </Dialog>
  );
}
