import { Tabs as Base } from "@base-ui/react/tabs";
import type * as React from "react";
import { cn } from "@/lib/utils";

export const Tabs = Base.Root;
export const TabsPanel = Base.Panel;

export function TabsList({ className, children, ...props }: React.ComponentProps<typeof Base.List>) {
  return (
    <Base.List
      className={cn("relative flex gap-1 overflow-x-auto border-b [scrollbar-width:none]", className)}
      {...props}
    >
      {children}
      <Base.Indicator className="absolute bottom-0 left-[var(--active-tab-left)] h-0.5 w-[var(--active-tab-width)] bg-primary transition-all" />
    </Base.List>
  );
}

export function TabsTab({ className, ...props }: React.ComponentProps<typeof Base.Tab>) {
  return (
    <Base.Tab
      className={cn(
        "flex h-10 shrink-0 items-center gap-1.5 px-3 text-sm font-medium text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50 data-[selected]:text-foreground",
        className,
      )}
      {...props}
    />
  );
}
