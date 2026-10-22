import { Dialog as Base } from "@base-ui/react/dialog";
import { XIcon } from "lucide-react";
import type * as React from "react";
import { cn } from "@/lib/utils";
import { m } from "@/paraglide/messages";

export const Dialog = Base.Root;
export const DialogTrigger = Base.Trigger;
export const DialogClose = Base.Close;

/** Bottom sheet on phones, centred dialog from sm upwards. */
export function DialogContent({
  title,
  description,
  className,
  children,
  wide,
}: {
  title: React.ReactNode;
  description?: React.ReactNode;
  className?: string;
  children: React.ReactNode;
  wide?: boolean;
}) {
  return (
    <Base.Portal>
      <Base.Backdrop className="fixed inset-0 z-50 bg-black/40 transition-opacity data-[ending-style]:opacity-0 data-[starting-style]:opacity-0" />
      <Base.Popup
        className={cn(
          "fixed z-50 flex max-h-[92dvh] flex-col overflow-hidden border bg-popover text-popover-foreground shadow-xl outline-none transition-all",
          "inset-x-0 bottom-0 rounded-t-2xl data-[ending-style]:translate-y-8 data-[starting-style]:translate-y-8 data-[ending-style]:opacity-0 data-[starting-style]:opacity-0",
          "sm:inset-auto sm:top-1/2 sm:left-1/2 sm:w-[calc(100vw-2rem)] sm:-translate-x-1/2 sm:-translate-y-1/2 sm:rounded-xl sm:data-[starting-style]:translate-y-[-48%]",
          wide ? "sm:max-w-3xl" : "sm:max-w-lg",
          className,
        )}
      >
        <div className="flex items-start justify-between gap-4 border-b px-5 py-4">
          <div className="grid gap-1">
            <Base.Title className="text-base font-semibold">{title}</Base.Title>
            {description ? (
              <Base.Description className="text-sm text-muted-foreground">{description}</Base.Description>
            ) : null}
          </div>
          <Base.Close
            className="-m-2 rounded-md p-2 text-muted-foreground hover:bg-accent"
            aria-label={m.close()}
          >
            <XIcon className="size-4" />
          </Base.Close>
        </div>
        <div className="overflow-y-auto px-5 py-4 pb-[max(1rem,env(safe-area-inset-bottom))]">{children}</div>
      </Base.Popup>
    </Base.Portal>
  );
}
