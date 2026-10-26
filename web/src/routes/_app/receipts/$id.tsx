import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import {
  ArrowLeftIcon,
  CheckIcon,
  ExternalLinkIcon,
  Loader2Icon,
  RotateCcwIcon,
  SparklesIcon,
  Trash2Icon,
} from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import {
  checkReceiptMutation,
  deleteReceiptMutation,
  getReceiptOptions,
  retryExtractionMutation,
  uncheckReceiptMutation,
  updateReceiptMutation,
} from "@/api/gen/@tanstack/react-query.gen";
import { listReceipts } from "@/api/gen/sdk.gen";
import type { Receipt, ReceiptInput } from "@/api/gen/types.gen";
import { ReceiptStatusBadge, warningText } from "@/components/common";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Field, Input } from "@/components/ui/input";
import { Alert, Skeleton } from "@/components/ui/misc";
import { centsToInput, formatDate, parseAmount } from "@/lib/format";
import { cn } from "@/lib/utils";
import { m } from "@/paraglide/messages";

export const Route = createFileRoute("/_app/receipts/$id")({ component: CheckReceipt });

type Form = { date: string; total: string; merchant: string; note: string };

const toForm = (r: Receipt): Form => ({
  date: r.date,
  total: centsToInput(r.total),
  merchant: r.merchant,
  note: r.note,
});

function CheckReceipt() {
  const id = Number(Route.useParams().id);
  const q = useQuery({
    ...getReceiptOptions({ path: { id } }),
    refetchInterval: (q) => (q.state.data?.extraction === "pending" ? 1500 : false),
  });
  const r = q.data;
  if (q.isPending) return <Skeleton className="h-96" />;
  if (!r) return <Alert variant="error">{m.not_found()}</Alert>;
  return <ReceiptForm key={r.id} receipt={r} />;
}

function ReceiptForm({ receipt: r }: { receipt: Receipt }) {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const [form, setForm] = useState<Form>(() => toForm(r));
  const [dirty, setDirty] = useState(false);
  // Values read in the background fill the form until the user types.
  useEffect(() => {
    if (!dirty) setForm(toForm(r));
  }, [r, dirty]);

  const set = (k: keyof Form) => (e: React.ChangeEvent<HTMLInputElement>) => {
    setDirty(true);
    setForm((f) => ({ ...f, [k]: e.target.value }));
  };
  const cents = parseAmount(form.total);
  const totalInvalid = form.total !== "" && cents === null;
  const body = (): ReceiptInput => ({
    date: form.date,
    total: cents ?? 0,
    merchant: form.merchant,
    note: form.note,
  });
  const refresh = () => {
    qc.invalidateQueries({ queryKey: [{ _id: "listReceipts" }] });
    qc.invalidateQueries({ queryKey: [{ _id: "getMonth" }] });
    qc.invalidateQueries({ queryKey: [{ _id: "getReceipt" }] });
  };

  const next = async () => {
    const { data } = await listReceipts({ query: { status: "pending" }, throwOnError: true });
    const other = data.find((x) => x.id !== r.id);
    if (other) navigate({ to: "/receipts/$id", params: { id: String(other.id) } });
    else navigate({ to: "/receipts" });
  };
  const check = useMutation({
    ...checkReceiptMutation(),
    onSuccess: () => {
      refresh();
      toast.success(m.receipt_checked());
      next();
    },
  });
  const save = useMutation({
    ...updateReceiptMutation(),
    onSuccess: () => {
      refresh();
      setDirty(false);
      toast.success(m.saved());
    },
  });
  const uncheck = useMutation({ ...uncheckReceiptMutation(), onSuccess: refresh });
  const retry = useMutation({ ...retryExtractionMutation(), onSuccess: refresh });
  const remove = useMutation({
    ...deleteReceiptMutation(),
    onSuccess: () => {
      refresh();
      navigate({ to: "/receipts" });
    },
  });
  const busy = check.isPending || save.isPending || remove.isPending || uncheck.isPending;
  const uncertain = (field: string) =>
    r.extraction === "done" && !dirty && (r.extracted.uncertain ?? []).includes(field);
  const highlight = (field: string) => cn(uncertain(field) && "border-warning ring-2 ring-warning/30");

  return (
    <>
      <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
        <Link
          to="/receipts"
          className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
        >
          <ArrowLeftIcon className="size-4" />
          {m.nav_receipts()}
        </Link>
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          {r.number ? <span className="tabular">{r.number}</span> : null}
          <ReceiptStatusBadge status={r.status} />
        </div>
      </div>
      <div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_22rem]">
        <Card className="overflow-hidden p-0">
          <a
            href={`/api/receipts/${r.id}/file`}
            target="_blank"
            rel="noreferrer"
            className="group relative block"
          >
            <img
              src={`/api/receipts/${r.id}/preview`}
              alt={m.receipt_image({ name: r.filename })}
              className="max-h-[70dvh] w-full bg-muted object-contain"
            />
            <span className="absolute right-2 bottom-2 flex items-center gap-1 rounded-md bg-background/90 px-2 py-1 text-xs">
              <ExternalLinkIcon className="size-3" />
              {r.pages > 1 ? m.open_original_pages({ pages: String(r.pages) }) : m.open_original()}
            </span>
          </a>
        </Card>

        <Card>
          <CardContent className="grid gap-4 py-5">
            <ExtractionState
              receipt={r}
              onRetry={() => retry.mutate({ path: { id: r.id } })}
              retrying={retry.isPending}
            />
            {r.warnings.length ? (
              <Alert variant="warning">
                <ul>
                  {r.warnings.map((w) => (
                    <li key={w}>{warningText(w)}</li>
                  ))}
                </ul>
              </Alert>
            ) : null}
            <form
              className="grid gap-4"
              onSubmit={(e) => {
                e.preventDefault();
                if (r.status === "pending") check.mutate({ path: { id: r.id }, body: body() });
                else save.mutate({ path: { id: r.id }, body: body() });
              }}
            >
              <Field
                label={m.field_date()}
                htmlFor="date"
                hint={uncertain("date") ? m.please_verify() : undefined}
              >
                <Input
                  id="date"
                  type="date"
                  value={form.date}
                  onChange={set("date")}
                  className={highlight("date")}
                  required={r.status === "checked"}
                />
              </Field>
              <Field
                label={m.field_total()}
                htmlFor="total"
                error={totalInvalid ? m.amount_invalid() : undefined}
                hint={uncertain("total") ? m.please_verify() : undefined}
              >
                <Input
                  id="total"
                  inputMode="decimal"
                  placeholder="0,00"
                  value={form.total}
                  onChange={set("total")}
                  className={highlight("total")}
                  aria-invalid={totalInvalid}
                />
              </Field>
              <Field
                label={m.field_merchant()}
                htmlFor="merchant"
                hint={uncertain("merchant") ? m.please_verify() : undefined}
              >
                <Input
                  id="merchant"
                  value={form.merchant}
                  onChange={set("merchant")}
                  className={highlight("merchant")}
                />
              </Field>
              <Field label={m.field_note()} htmlFor="note">
                <Input id="note" value={form.note} onChange={set("note")} />
              </Field>
              {r.source === "mail" ? (
                <p className="text-xs text-muted-foreground">{m.received_by_mail({ from: r.mailFrom })}</p>
              ) : r.source === "folder" ? (
                <p className="text-xs text-muted-foreground">{m.received_from_folder()}</p>
              ) : null}
              {r.status === "pending" ? (
                <div className="grid gap-2">
                  <Button type="submit" size="lg" disabled={busy || !form.date || !cents}>
                    {check.isPending ? <Loader2Icon className="animate-spin" /> : <CheckIcon />}
                    {m.check_and_next()}
                  </Button>
                  <div className="flex gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      className="flex-1"
                      disabled={busy || totalInvalid}
                      onClick={() => save.mutate({ path: { id: r.id }, body: body() })}
                    >
                      {m.save()}
                    </Button>
                  </div>
                </div>
              ) : (
                <div className="grid gap-2">
                  <Button type="submit" disabled={busy || !dirty || !cents || !form.date}>
                    {m.save()}
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => uncheck.mutate({ path: { id: r.id } })}
                    disabled={uncheck.isPending}
                  >
                    <RotateCcwIcon />
                    {m.uncheck()}
                  </Button>
                  <p className="text-xs text-muted-foreground">
                    {m.checked_on({ date: formatDate(r.checkedAt) })}
                  </p>
                </div>
              )}
              <div className="grid gap-2 border-t pt-4">
                <Button
                  type="button"
                  variant="outline"
                  disabled={r.archived || busy}
                  onClick={() => {
                    if (window.confirm(m.delete_receipt_confirm())) remove.mutate({ path: { id: r.id } });
                  }}
                >
                  <Trash2Icon />
                  {m.delete_receipt()}
                </Button>
                {r.archived ? (
                  <p className="text-xs text-muted-foreground">{m.receipt_delete_archived()}</p>
                ) : null}
              </div>
            </form>
          </CardContent>
        </Card>
      </div>
    </>
  );
}

function ExtractionState({
  receipt: r,
  onRetry,
  retrying,
}: {
  receipt: Receipt;
  onRetry: () => void;
  retrying: boolean;
}) {
  switch (r.extraction) {
    case "pending":
      return (
        <p className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2Icon className="size-4 animate-spin" />
          {m.extraction_pending()}
        </p>
      );
    case "failed":
      return (
        <Alert variant="warning" title={m.extraction_failed()}>
          {r.status === "pending" ? (
            <Button variant="outline" size="sm" className="mt-2" onClick={onRetry} disabled={retrying}>
              {m.extraction_retry()}
            </Button>
          ) : null}
        </Alert>
      );
    case "done":
      return (
        <p className="flex items-center gap-2 text-sm text-muted-foreground">
          <SparklesIcon className="size-4 text-primary" />
          {m.extraction_done()}
        </p>
      );
    default:
      return null;
  }
}
