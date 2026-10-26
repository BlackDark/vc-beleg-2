import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { getReaderStatusOptions } from "@/api/gen/@tanstack/react-query.gen";
import { ConnectionCheck } from "@/components/connection-check";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Alert, Skeleton } from "@/components/ui/misc";
import { m } from "@/paraglide/messages";

export function ReaderInfoCard() {
  const q = useQuery(getReaderStatusOptions());
  const info = q.data;
  const efforts: Record<string, string> = {
    low: m.reader_effort_low(),
    medium: m.reader_effort_medium(),
    high: m.reader_effort_high(),
    none: m.reader_effort_none(),
    provider_default: m.reader_effort_default(),
  };
  return (
    <Card>
      <CardHeader>
        <CardTitle>{m.reader_title()}</CardTitle>
        <CardDescription>{m.reader_description()}</CardDescription>
      </CardHeader>
      <CardContent className="grid gap-3 text-sm">
        {q.isError ? (
          <Alert variant="error">{m.reader_info_unavailable()}</Alert>
        ) : !info ? (
          <Skeleton className="h-24" />
        ) : (
          <>
            {!info.enabled ? <Alert>{m.reader_off()}</Alert> : null}
            <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2">
              <dt>{m.reader_provider()}</dt>
              <dd>{info.provider}</dd>
              <dt>{m.reader_model()}</dt>
              <dd className="break-all">{info.model}</dd>
              <dt>{m.reader_endpoint()}</dt>
              <dd className="break-all">{info.endpoint}</dd>
              <dt>{m.reader_effort()}</dt>
              <dd>{efforts[info.effort] ?? info.effort}</dd>
              {info.provider === "claude" ? (
                <>
                  <dt>{m.reader_fallback()}</dt>
                  <dd>{info.fallback ? m.reader_on() : m.reader_disabled()}</dd>
                </>
              ) : null}
            </dl>
            <p className="text-muted-foreground">
              {m.reader_execution({ seconds: String(info.timeoutSeconds) })}
            </p>
            <p className="text-muted-foreground">{m.reader_ping_hint()}</p>
            <ConnectionCheck service="reader" enabled={info.enabled} label={m.reader_test_connection()} />
            <Link to="/settings" search={{ section: "imports" }} className="text-primary underline">
              {m.reader_durations()}
            </Link>
          </>
        )}
      </CardContent>
    </Card>
  );
}
