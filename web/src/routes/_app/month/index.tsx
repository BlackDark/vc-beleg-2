import { createFileRoute, redirect } from "@tanstack/react-router";
import { currentMonth } from "@/lib/format";

export const Route = createFileRoute("/_app/month/")({
  beforeLoad: () => {
    throw redirect({ to: "/month/$month", params: { month: currentMonth() } });
  },
});
