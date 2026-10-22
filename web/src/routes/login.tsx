import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router";
import { KeyRoundIcon, Loader2Icon } from "lucide-react";
import { useState } from "react";
import { loginMutation } from "@/api/gen/@tanstack/react-query.gen";
import { Logo } from "@/components/app-shell";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Field, Input } from "@/components/ui/input";
import { Alert } from "@/components/ui/misc";
import { errorText } from "@/lib/api";
import { sessionQuery } from "@/lib/session";
import { m } from "@/paraglide/messages";

type Search = { next?: string; error?: string };

export const Route = createFileRoute("/login")({
  validateSearch: (s: Record<string, unknown>): Search => ({
    next: typeof s.next === "string" && s.next.startsWith("/") ? s.next : undefined,
    error: typeof s.error === "string" ? s.error : undefined,
  }),
  beforeLoad: async ({ context }) => {
    const s = await context.queryClient.ensureQueryData(sessionQuery);
    if (s.user) throw redirect({ to: "/" });
    if (s.needsSetup) throw redirect({ to: "/setup" });
  },
  component: Login,
});

function Login() {
  const { next, error } = Route.useSearch();
  const { data: session } = useQuery(sessionQuery);
  const qc = useQueryClient();
  const navigate = useNavigate();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const login = useMutation({
    ...loginMutation(),
    onError: () => undefined,
    onSuccess: (data) => {
      qc.setQueryData(sessionQuery.queryKey, data);
      navigate({ to: next ?? "/" });
    },
  });

  return (
    <AuthLayout title={m.login_title()} subtitle={m.login_subtitle()}>
      {error ? <Alert variant="error">{errorText({ code: error })}</Alert> : null}
      {session?.oidcEnabled ? (
        <Button size="lg" className="w-full" onClick={() => window.location.assign("/auth/oidc/login")}>
          <KeyRoundIcon />
          {m.login_with({ provider: session.oidcLabel })}
        </Button>
      ) : null}
      {session?.oidcEnabled && session.passwordEnabled ? (
        <div className="flex items-center gap-3 text-xs text-muted-foreground">
          <span className="h-px flex-1 bg-border" />
          {m.or()}
          <span className="h-px flex-1 bg-border" />
        </div>
      ) : null}
      {session?.passwordEnabled !== false ? (
        <form
          className="grid gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            login.mutate({ body: { username, password } });
          }}
        >
          <Field label={m.username()} htmlFor="username">
            <Input
              id="username"
              autoComplete="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              autoFocus
            />
          </Field>
          <Field label={m.password()} htmlFor="password">
            <Input
              id="password"
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </Field>
          {login.error ? <Alert variant="error">{errorText(login.error)}</Alert> : null}
          <Button
            type="submit"
            size="lg"
            variant={session?.oidcEnabled ? "outline" : "default"}
            disabled={login.isPending}
          >
            {login.isPending ? <Loader2Icon className="animate-spin" /> : null}
            {m.login()}
          </Button>
        </form>
      ) : null}
    </AuthLayout>
  );
}

export function AuthLayout({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="grid min-h-dvh place-items-center bg-muted/40 px-4 py-10">
      <div className="w-full max-w-sm">
        <div className="mb-6 flex flex-col items-center gap-3 text-center">
          <Logo />
          <div>
            <h1 className="text-xl font-semibold">{title}</h1>
            {subtitle ? <p className="text-sm text-muted-foreground">{subtitle}</p> : null}
          </div>
        </div>
        <Card>
          <CardContent className="grid gap-4 py-5">{children}</CardContent>
        </Card>
      </div>
    </div>
  );
}
