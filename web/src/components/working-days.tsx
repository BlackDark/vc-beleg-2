import { useMutation } from "@tanstack/react-query";
import { setDayMarkMutation, setMonthProveDaysMutation } from "@/api/gen/@tanstack/react-query.gen";
import type { CalendarDay, Month } from "@/api/gen/types.gen";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/input";
import { formatDate, intlLocale } from "@/lib/format";
import { cn } from "@/lib/utils";
import { m } from "@/paraglide/messages";

type Mark = CalendarDay["mark"];

/** The cell label: plain working days and weekends stay unlabelled. */
const markText = (c: CalendarDay): string =>
  c.mark
    ? { vacation: m.mark_vacation(), sick: m.mark_sick(), off: m.mark_off(), worked: m.mark_worked() }[c.mark]
    : c.holiday || "";

const isWeekday = (date: string) => {
  const day = new Date(`${date}T00:00:00Z`).getUTCDay();
  return day !== 0 && day !== 6;
};

/**
 * A tap moves a date to its next entry: weekdays cycle through vacation,
 * sick and off; weekends and holidays toggle "worked".
 */
function nextMark(c: CalendarDay): Mark {
  if (!isWeekday(c.date) || c.holiday) return c.mark === "worked" ? "" : "worked";
  const cycle: Mark[] = ["", "vacation", "sick", "off"];
  return cycle[(cycle.indexOf(c.mark) + 1) % cycle.length]!;
}

/** The switch between the 15-day limit and proven working days. */
export function ProveDaysToggle({ month: mo, onSuccess }: { month: Month; onSuccess: (m: Month) => void }) {
  const prove = useMutation({ ...setMonthProveDaysMutation(), onSuccess });
  return (
    <label className="flex items-start gap-3 text-sm">
      <Checkbox
        className="mt-0.5"
        checked={mo.result.proveDays}
        disabled={mo.status !== "open" || prove.isPending}
        onChange={(e) => prove.mutate({ path: { month: mo.month }, body: { prove: e.target.checked } })}
      />
      <span className="grid gap-0.5">
        <span className="font-medium">{m.prove_days()}</span>
        <span className="text-xs text-muted-foreground">{m.prove_days_hint()}</span>
      </span>
    </label>
  );
}

/** Month grid of working days; editable while the month is open. */
export function WorkingDayCalendar({
  month: mo,
  onSuccess,
}: {
  month: Month;
  onSuccess: (m: Month) => void;
}) {
  const mark = useMutation({ ...setDayMarkMutation(), onSuccess });
  const cal = mo.result.calendar;
  if (cal.length === 0) return null;
  const open = mo.status === "open";
  const mealDays = new Set(mo.result.days.map((d) => d.date));
  const lead = (new Date(`${cal[0]!.date}T00:00:00Z`).getUTCDay() + 6) % 7;
  const weekdays = Array.from({ length: 7 }, (_, i) =>
    new Intl.DateTimeFormat(intlLocale(), { weekday: "short", timeZone: "UTC" }).format(
      new Date(Date.UTC(2026, 9, 5 + i)),
    ),
  );
  return (
    <Card>
      <CardHeader>
        <CardTitle>{m.working_days_title({ count: String(mo.result.totals.workingDays) })}</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-2">
        <div className="grid grid-cols-7 gap-1 text-center text-xs text-muted-foreground">
          {weekdays.map((w) => (
            <span key={w}>{w}</span>
          ))}
        </div>
        <div className="grid grid-cols-7 gap-1">
          {Array.from({ length: lead }, (_, i) => (
            <span key={i} />
          ))}
          {cal.map((c) => (
            <button
              key={c.date}
              type="button"
              disabled={!open || mark.isPending}
              aria-label={`${formatDate(c.date)}: ${markText(c) || (c.working ? m.mark_working() : m.mark_free())}`}
              onClick={() => mark.mutate({ body: { date: c.date, mark: nextMark(c) } })}
              className={cn(
                "flex min-h-12 flex-col items-start rounded-md p-1 text-left text-xs",
                c.working ? "bg-primary/15" : "bg-muted text-muted-foreground",
                c.mark && c.mark !== "worked" && "bg-warning/15",
                mealDays.has(c.date) && "ring-1 ring-primary",
                open && "hover:ring-2 hover:ring-primary/50",
              )}
            >
              <span className="font-medium">{Number(c.date.slice(8))}</span>
              <span className="w-full truncate text-[0.65rem] leading-tight">{markText(c)}</span>
            </button>
          ))}
        </div>
        <p className="text-xs text-muted-foreground">
          {open ? m.working_days_hint() : m.working_days_frozen()}
        </p>
      </CardContent>
    </Card>
  );
}
