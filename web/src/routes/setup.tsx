import { useMutation, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router";
import { Loader2Icon } from "lucide-react";
import { useState } from "react";
import { setupMutation } from "@/api/gen/@tanstack/react-query.gen";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/input";
import { Alert } from "@/components/ui/misc";
import { errorText } from "@/lib/api";
import { sessionQuery } from "@/lib/session";
import { m } from "@/paraglide/messages";
import { AuthLayout } from "./login";

export const Route = createFileRoute("/setup")({
  beforeLoad: async ({ context }) => {
    const s = await context.queryClient.ensureQueryData(sessionQuery);
    if (!s.needsSetup) throw redirect({ to: "/login" });
  },
  component: Setup,
});

function Setup() {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const [form, setForm] = useState({ token: "", username: "", name: "", password: "" });
  const set = (k: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm({ ...form, [k]: e.target.value });
  const setup = useMutation({
    ...setupMutation(),
    onError: () => undefined,
    onSuccess: (data) => {
      qc.setQueryData(sessionQuery.queryKey, data);
      navigate({ to: "/settings" });
    },
  });
  return (
    <AuthLayout title={m.setup_title()} subtitle={m.setup_subtitle()}>
      <form
        className="grid gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          setup.mutate({ body: form });
        }}
      >
        <Field label={m.setup_token()} htmlFor="token" hint={m.setup_token_hint()}>
          <Input id="token" value={form.token} onChange={set("token")} required autoComplete="off" />
        </Field>
        <Field label={m.your_name()} htmlFor="name">
          <Input id="name" value={form.name} onChange={set("name")} required autoComplete="name" />
        </Field>
        <Field label={m.username()} htmlFor="username">
          <Input
            id="username"
            value={form.username}
            onChange={set("username")}
            required
            autoComplete="username"
          />
        </Field>
        <Field label={m.password()} htmlFor="password" hint={m.password_hint()}>
          <Input
            id="password"
            type="password"
            minLength={10}
            value={form.password}
            onChange={set("password")}
            required
            autoComplete="new-password"
          />
        </Field>
        {setup.error ? <Alert variant="error">{errorText(setup.error)}</Alert> : null}
        <Button type="submit" size="lg" disabled={setup.isPending}>
          {setup.isPending ? <Loader2Icon className="animate-spin" /> : null}
          {m.setup_submit()}
        </Button>
      </form>
    </AuthLayout>
  );
}
