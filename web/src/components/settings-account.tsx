import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import { changePasswordMutation, updateMeMutation } from "@/api/gen/@tanstack/react-query.gen";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Field, Input, Select } from "@/components/ui/input";
import { sessionQuery, useSession, useUser } from "@/lib/session";
import { applyTheme, type Theme } from "@/lib/theme";
import { m } from "@/paraglide/messages";
import { getLocale, setLocale } from "@/paraglide/runtime";
export function ProfileCard() {
  const user = useUser();
  const qc = useQueryClient();
  const [name, setName] = useState(user.name);
  const [email, setEmail] = useState(user.email);
  const update = useMutation({
    ...updateMeMutation(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: sessionQuery.queryKey });
      toast.success(m.saved());
    },
  });
  const setTheme = (theme: Theme) => {
    applyTheme(theme);
    update.mutate({ body: { name, email, theme } });
  };
  return (
    <Card>
      <CardHeader>
        <CardTitle>{m.settings_profile()}</CardTitle>
      </CardHeader>
      <CardContent>
        <form
          className="grid gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            update.mutate({ body: { name, email, theme: user.theme } });
          }}
        >
          <Field label={m.name()} htmlFor="name">
            <Input id="name" value={name} onChange={(e) => setName(e.target.value)} required />
          </Field>
          <Field label={m.email()} htmlFor="email">
            <Input id="email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
          </Field>
          <Field label={m.language()} htmlFor="language">
            <Select
              id="language"
              value={getLocale()}
              onChange={(e) => setLocale(e.target.value as "de" | "en")}
            >
              <option value="de">{m.language_de()}</option>
              <option value="en">{m.language_en()}</option>
            </Select>
          </Field>
          <Field label={m.theme()} htmlFor="theme">
            <Select id="theme" value={user.theme} onChange={(e) => setTheme(e.target.value as Theme)}>
              <option value="dark">{m.theme_dark()}</option>
              <option value="light">{m.theme_light()}</option>
              <option value="system">{m.theme_system()}</option>
            </Select>
          </Field>
          <Button type="submit" variant="outline" disabled={update.isPending}>
            {m.save()}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

export function PasswordCard() {
  const user = useUser();
  const { data: session } = useSession();
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const change = useMutation({
    ...changePasswordMutation(),
    onSuccess: () => {
      setCurrent("");
      setNext("");
      toast.success(m.password_changed());
    },
  });
  if (!session?.passwordEnabled) return null;
  return (
    <Card>
      <CardHeader>
        <CardTitle>{m.password()}</CardTitle>
        {!user.hasPassword ? (
          <CardDescription>{m.password_set_hint({ username: user.email || "admin" })}</CardDescription>
        ) : null}
      </CardHeader>
      <CardContent>
        <form
          className="grid gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            change.mutate({ body: { current, next } });
          }}
        >
          {user.hasPassword ? (
            <Field label={m.password_current()} htmlFor="current">
              <Input
                id="current"
                type="password"
                autoComplete="current-password"
                value={current}
                onChange={(e) => setCurrent(e.target.value)}
                required
              />
            </Field>
          ) : null}
          <Field label={m.password_new()} htmlFor="next" hint={m.password_rule()}>
            <Input
              id="next"
              type="password"
              autoComplete="new-password"
              minLength={10}
              value={next}
              onChange={(e) => setNext(e.target.value)}
              required
            />
          </Field>
          <Button type="submit" variant="outline" disabled={change.isPending}>
            {m.password_change()}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}
