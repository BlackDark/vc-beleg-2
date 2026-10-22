import type { QueryClient } from "@tanstack/react-query";
import { createRootRouteWithContext, Outlet } from "@tanstack/react-router";
import { useSyncExternalStore } from "react";
import { Toaster } from "sonner";
import { errorText } from "@/lib/api";
import { isDark, subscribeDark } from "@/lib/theme";
import { m } from "@/paraglide/messages";

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  component: Root,
  errorComponent: ({ error, reset }) => (
    <div className="mx-auto grid max-w-md gap-3 p-10 text-center">
      <p className="font-medium">{m.err_unknown()}</p>
      <p className="text-sm text-muted-foreground">{errorText(error)}</p>
      <button type="button" className="text-sm text-primary underline" onClick={reset}>
        {m.retry()}
      </button>
    </div>
  ),
  notFoundComponent: () => (
    <div className="mx-auto max-w-md p-10 text-center text-muted-foreground">{m.not_found()}</div>
  ),
});

function Root() {
  const dark = useSyncExternalStore(subscribeDark, isDark);
  return (
    <>
      <Outlet />
      <Toaster position="top-center" richColors closeButton theme={dark ? "dark" : "light"} />
    </>
  );
}
