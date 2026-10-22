import type * as React from "react";
import { cn } from "@/lib/utils";

const field =
  "flex w-full min-w-0 rounded-md border border-input bg-card px-3 text-base shadow-xs transition-colors outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/40 disabled:cursor-not-allowed disabled:opacity-60 aria-invalid:border-destructive aria-invalid:ring-destructive/20 md:text-sm";

export function Input({ className, ...props }: React.ComponentProps<"input">) {
  return <input className={cn(field, "h-10 py-2", className)} {...props} />;
}

export function Textarea({ className, ...props }: React.ComponentProps<"textarea">) {
  return <textarea className={cn(field, "min-h-20 py-2", className)} {...props} />;
}

/** Native select: best keyboard and mobile behaviour. */
export function Select({ className, ...props }: React.ComponentProps<"select">) {
  return <select className={cn(field, "h-10 py-2 pr-8", className)} {...props} />;
}

export function Label({ className, ...props }: React.ComponentProps<"label">) {
  return <label className={cn("text-sm font-medium leading-none text-foreground", className)} {...props} />;
}

export function Checkbox({ className, ...props }: Omit<React.ComponentProps<"input">, "type">) {
  return (
    <input
      type="checkbox"
      className={cn("size-4.5 shrink-0 rounded border-input accent-[var(--primary)]", className)}
      {...props}
    />
  );
}

/**
 * A labelled form field with optional hint and error. Without htmlFor the
 * label wraps the control, which associates them implicitly.
 */
export function Field({
  label,
  htmlFor,
  hint,
  error,
  className,
  children,
}: {
  label: React.ReactNode;
  htmlFor?: string;
  hint?: React.ReactNode;
  error?: React.ReactNode;
  className?: string;
  children: React.ReactNode;
}) {
  const note = error ? (
    <p className="text-xs text-destructive" role="alert">
      {error}
    </p>
  ) : hint ? (
    <p className="text-xs font-normal text-muted-foreground">{hint}</p>
  ) : null;
  if (!htmlFor) {
    return (
      <div className={cn("grid gap-1.5", className)}>
        <label className="grid gap-1.5 text-sm font-medium leading-none">
          <span>{label}</span>
          {children}
        </label>
        {note}
      </div>
    );
  }
  return (
    <div className={cn("grid gap-1.5", className)}>
      <Label htmlFor={htmlFor}>{label}</Label>
      {children}
      {note}
    </div>
  );
}
