import { Menu as Base } from "@base-ui/react/menu";
import type * as React from "react";
import { cn } from "@/lib/utils";

export const Menu = Base.Root;
export const MenuTrigger = Base.Trigger;

export function MenuContent({
  children,
  align = "end",
}: {
  children: React.ReactNode;
  align?: "start" | "end";
}) {
  return (
    <Base.Portal>
      <Base.Positioner sideOffset={6} align={align} className="z-50">
        <Base.Popup className="min-w-44 rounded-lg border bg-popover p-1 text-popover-foreground shadow-lg outline-none">
          {children}
        </Base.Popup>
      </Base.Positioner>
    </Base.Portal>
  );
}

export function MenuItem({ className, ...props }: React.ComponentProps<typeof Base.Item>) {
  return (
    <Base.Item
      className={cn(
        "flex cursor-default items-center gap-2 rounded-md px-2.5 py-2 text-sm outline-none select-none data-[highlighted]:bg-accent data-[disabled]:opacity-50 [&_svg]:size-4",
        className,
      )}
      {...props}
    />
  );
}
