import { useMutation } from "@tanstack/react-query";
import { checkConnectionMutation } from "@/api/gen/@tanstack/react-query.gen";
import { Button } from "@/components/ui/button";
import { Alert } from "@/components/ui/misc";
import { m } from "@/paraglide/messages";

export function ConnectionCheck({
  service,
  enabled,
  label,
}: {
  service: "reader" | "mail" | "folder";
  enabled: boolean;
  label: string;
}) {
  const check = useMutation(checkConnectionMutation());
  return (
    <div className="grid gap-2">
      <Button
        type="button"
        variant="outline"
        aria-label={label}
        aria-busy={check.isPending}
        disabled={!enabled || check.isPending}
        onClick={() => check.mutate({ path: { service } })}
      >
        {check.isPending ? m.connection_checking() : label}
      </Button>
      {!check.isPending && check.data ? (
        <Alert variant={check.data.ok ? "info" : "error"}>
          <p>
            {check.data.ok
              ? m.connection_success()
              : check.data.errorCode === "timeout"
                ? m.connection_timeout()
                : m.connection_failed()}
          </p>
          <p>{m.connection_duration({ duration: String(check.data.durationMs) })}</p>
        </Alert>
      ) : null}
      {check.isError ? <Alert variant="error">{m.connection_failed()}</Alert> : null}
    </div>
  );
}
