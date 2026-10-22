import { AlertTriangleIcon, InfoIcon, OctagonXIcon } from "lucide-react";
import type * as React from "react";
import { cn } from "@/lib/utils";

export function Skeleton({ className, ...props }: React.ComponentProps<"div">) {
  return <div className={cn("animate-pulse rounded-md bg-muted", className)} {...props} />;
}

export function Separator({ className }: { className?: string }) {
  return <hr className={cn("border-border", className)} />;
}

const alertStyles = {
  info: { box: "border-border bg-muted/60", icon: InfoIcon, iconClass: "text-primary" },
  warning: { box: "border-warning/40 bg-warning-soft", icon: AlertTriangleIcon, iconClass: "text-warning" },
  error: { box: "border-destructive/30 bg-destructive/5", icon: OctagonXIcon, iconClass: "text-destructive" },
} as const;

export function Alert({
  variant = "info",
  title,
  children,
  className,
}: {
  variant?: keyof typeof alertStyles;
  title?: React.ReactNode;
  children?: React.ReactNode;
  className?: string;
}) {
  const s = alertStyles[variant];
  const Icon = s.icon;
  return (
    <div
      role={variant === "error" ? "alert" : "status"}
      className={cn("flex gap-3 rounded-lg border p-3 text-sm", s.box, className)}
    >
      <Icon className={cn("mt-0.5 size-4 shrink-0", s.iconClass)} />
      <div className="grid gap-1">
        {title ? <p className="font-medium">{title}</p> : null}
        {children ? (
          <div className="text-muted-foreground [&_li]:ml-4 [&_li]:list-disc">{children}</div>
        ) : null}
      </div>
    </div>
  );
}

export function EmptyState({
  icon: Icon,
  title,
  text,
  action,
}: {
  icon: React.ComponentType<{ className?: string }>;
  title: string;
  text?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col items-center gap-3 rounded-xl border border-dashed px-6 py-12 text-center">
      <div className="rounded-full bg-accent p-3 text-accent-foreground">
        <Icon className="size-6" />
      </div>
      <div className="grid gap-1">
        <p className="font-medium">{title}</p>
        {text ? <p className="max-w-sm text-sm text-muted-foreground">{text}</p> : null}
      </div>
      {action}
    </div>
  );
}
