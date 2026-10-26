import { useQuery } from "@tanstack/react-query";
import { getFolderStatusOptions, getMailStatusOptions } from "@/api/gen/@tanstack/react-query.gen";
import { ConnectionCheck } from "@/components/connection-check";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Alert, Skeleton } from "@/components/ui/misc";
import { formatDateTime } from "@/lib/format";
import { m } from "@/paraglide/messages";
export function MailCard() {
  const q = useQuery({ ...getMailStatusOptions(), refetchInterval: 30_000 });
  return (
    <ImportCard
      service="mail"
      title={m.settings_mail()}
      off={m.mail_off()}
      status={q.data && { ...q.data, location: q.data.mailbox }}
    />
  );
}

export function FolderCard() {
  const q = useQuery({ ...getFolderStatusOptions(), refetchInterval: 30_000 });
  return (
    <ImportCard
      service="folder"
      title={m.settings_folder()}
      off={m.folder_off()}
      status={q.data && { ...q.data, location: q.data.folder }}
    />
  );
}

type ImportStatus = {
  enabled: boolean;
  location: string;
  lastRun: string;
  lastError: string;
  imported: number;
};

/** State of a background import (mail or folder). */
function ImportCard({
  title,
  off,
  service,
  status: s,
}: {
  title: string;
  off: string;
  service: "mail" | "folder";
  status?: ImportStatus;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-2 text-sm">
        {!s ? (
          <Skeleton className="h-12" />
        ) : !s.enabled ? (
          <p className="text-muted-foreground">{off}</p>
        ) : (
          <>
            <p className="break-all">{s.location}</p>
            <p className="text-muted-foreground">
              {s.lastRun ? m.mail_last_run({ date: formatDateTime(s.lastRun) }) : m.mail_not_run()}
              {" · "}
              {m.mail_imported({ count: String(s.imported) })}
            </p>
            {s.lastError ? <Alert variant="error">{s.lastError}</Alert> : null}
          </>
        )}
        <ConnectionCheck
          service={service}
          enabled={s?.enabled ?? false}
          label={m.connection_test_service({ service: title })}
        />
      </CardContent>
    </Card>
  );
}
