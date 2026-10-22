import { ChevronLeftIcon, ChevronRightIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { formatMonth, shiftMonth } from "@/lib/format";
import { m } from "@/paraglide/messages";

export function MonthSwitcher({ month, onChange }: { month: string; onChange: (month: string) => void }) {
  return (
    <div className="flex items-center gap-1">
      <Button
        variant="ghost"
        size="icon"
        aria-label={m.month_previous()}
        onClick={() => onChange(shiftMonth(month, -1))}
      >
        <ChevronLeftIcon />
      </Button>
      <span className="min-w-36 text-center font-medium">{formatMonth(month)}</span>
      <Button
        variant="ghost"
        size="icon"
        aria-label={m.month_next()}
        onClick={() => onChange(shiftMonth(month, 1))}
      >
        <ChevronRightIcon />
      </Button>
    </div>
  );
}
