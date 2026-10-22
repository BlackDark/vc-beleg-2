import { createFileRoute, redirect } from "@tanstack/react-router";
import { useEffect } from "react";
import { AppShell } from "@/components/app-shell";
import { sessionQuery, useSession } from "@/lib/session";
import { applyTheme } from "@/lib/theme";

/** Authenticated layout: redirects to login or setup when needed. */
export const Route = createFileRoute("/_app")({
  beforeLoad: async ({ context, location }) => {
    const session = await context.queryClient.ensureQueryData(sessionQuery);
    if (!session.user) {
      throw redirect({ to: session.needsSetup ? "/setup" : "/login", search: { next: location.href } });
    }
  },
  component: Layout,
});

function Layout() {
  const { data } = useSession();
  const user = data?.user;
  useEffect(() => {
    if (user) applyTheme((user.theme as "system" | "light" | "dark") ?? "dark");
  }, [user]);
  if (!user) return null;
  return <AppShell />;
}
