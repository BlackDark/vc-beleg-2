const intlLocale = "de-DE";

/** Formats EUR cents, e.g. 767 → "7,67 €". */
export function formatCents(cents: number | undefined | null): string {
  return new Intl.NumberFormat(intlLocale, { style: "currency", currency: "EUR" }).format((cents ?? 0) / 100);
}

/** ISO date (YYYY-MM-DD…) → locale date without time-zone shifts. */
export function formatDate(iso: string | undefined | null, opts: Intl.DateTimeFormatOptions = {}): string {
  if (!iso) return "";
  const [y, m, d] = iso.slice(0, 10).split("-").map(Number);
  const date = new Date(Date.UTC(y!, m! - 1, d!));
  return new Intl.DateTimeFormat(intlLocale, {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    timeZone: "UTC",
    ...opts,
  }).format(date);
}

export function formatWeekday(iso: string): string {
  return formatDate(iso, { weekday: "short", day: "2-digit", month: "2-digit", year: undefined });
}

/** "2026-10" → "Oktober 2026". */
export function formatMonth(month: string): string {
  const [y, m] = month.split("-").map(Number);
  return new Intl.DateTimeFormat(intlLocale, { month: "long", year: "numeric", timeZone: "UTC" }).format(
    new Date(Date.UTC(y!, m! - 1, 1)),
  );
}

/**
 * Parses a user-entered amount ("12,50", "1.234,5", "12.50") into cents.
 * With both separators the last one is decimal; a single separator that
 * occurs once is decimal. Returns null for invalid input.
 */
export function parseAmount(input: string): number | null {
  let s = input.trim().replace(/[\s€]/g, "");
  if (!s) return null;
  const commas = s.split(",").length - 1;
  const dots = s.split(".").length - 1;
  let decimal: string | null = null;
  if (commas && dots) decimal = s.lastIndexOf(",") > s.lastIndexOf(".") ? "," : ".";
  else if (commas === 1) decimal = ",";
  else if (dots === 1) decimal = ".";
  const thousands = decimal === "," ? "." : decimal === "." ? "," : commas ? "," : ".";
  const intPart = decimal ? s.slice(0, s.lastIndexOf(decimal)) : s;
  const groups = intPart.split(thousands);
  if (groups.length > 1 && groups.slice(1).some((g) => g.length !== 3)) return null;
  s = s.split(thousands).join("");
  if (decimal) s = s.replace(decimal, ".");
  if (!/^\d+(\.\d+)?$/.test(s)) return null;
  const [int = "0", frac = ""] = s.split(".");
  if (frac.length > 2) return null;
  const cents = Number(int) * 100 + Number(frac.padEnd(2, "0") || "0");
  return Number.isSafeInteger(cents) ? cents : null;
}

/** Cents → editable string ("12,50"); 0 or missing → "". */
export function centsToInput(cents: number | undefined | null): string {
  if (!cents) return "";
  return (cents / 100).toFixed(2).replace(".", ",");
}

export function todayISO(now = new Date()): string {
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}-${String(now.getDate()).padStart(2, "0")}`;
}

export function currentMonth(now = new Date()): string {
  return todayISO(now).slice(0, 7);
}

/** Moves "YYYY-MM" by n months. */
export function shiftMonth(month: string, n: number): string {
  const [y, m] = month.split("-").map(Number);
  const d = new Date(Date.UTC(y!, m! - 1 + n, 1));
  return d.toISOString().slice(0, 7);
}
