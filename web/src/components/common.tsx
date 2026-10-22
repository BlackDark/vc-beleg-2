import type * as React from "react";
import { Badge } from "@/components/ui/badge";
import { formatCents } from "@/lib/format";
import { cn } from "@/lib/utils";
import { m } from "@/paraglide/messages";

export function PageHeader({
  title,
  description,
  actions,
}: {
  title: React.ReactNode;
  description?: React.ReactNode;
  actions?: React.ReactNode;
}) {
  return (
    <div className="mb-6 flex flex-wrap items-end justify-between gap-3">
      <div className="grid min-w-0 gap-1">
        <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
        {description ? <p className="text-sm text-muted-foreground">{description}</p> : null}
      </div>
      {actions ? <div className="flex flex-wrap gap-2">{actions}</div> : null}
    </div>
  );
}

export function Money({
  cents,
  className,
  strong,
}: {
  cents: number | undefined;
  className?: string;
  strong?: boolean;
}) {
  return (
    <span className={cn("tabular whitespace-nowrap", strong && "font-semibold", className)}>
      {formatCents(cents)}
    </span>
  );
}

/** Key/value rows for summaries. */
export function Row({
  label,
  children,
  strong,
}: {
  label: React.ReactNode;
  children: React.ReactNode;
  strong?: boolean;
}) {
  return (
    <div
      className={cn("flex items-baseline justify-between gap-4 py-1.5 text-sm", strong && "font-semibold")}
    >
      <span className={cn(!strong && "text-muted-foreground")}>{label}</span>
      <span className="text-right">{children}</span>
    </div>
  );
}

export function MonthStatusBadge({ status }: { status: string }) {
  switch (status) {
    case "paid":
      return <Badge variant="success">{m.month_status_paid()}</Badge>;
    case "closed":
      return <Badge variant="primary">{m.month_status_closed()}</Badge>;
    default:
      return <Badge variant="outline">{m.month_status_open()}</Badge>;
  }
}

export function ReceiptStatusBadge({ status }: { status: string }) {
  return status === "checked" ? (
    <Badge variant="success">{m.receipt_status_checked()}</Badge>
  ) : (
    <Badge variant="warning">{m.receipt_status_pending()}</Badge>
  );
}

type MessageFn = () => string;
const messages = m as unknown as Record<string, MessageFn | undefined>;

/** Translates warning and blocker codes from the server (warning_<code>). */
export function warningText(code: string): string {
  return messages[`warning_${code}`]?.() ?? code;
}

export const taxModelText = (model: string): string =>
  ({ A: m.tax_model_a(), B: m.tax_model_b(), C: m.tax_model_c() })[model] ?? model;
