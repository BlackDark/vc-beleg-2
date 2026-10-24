import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { FolderIcon, MailIcon, SparklesIcon, UploadIcon } from "lucide-react";
import { getImportLogOptions } from "@/api/gen/@tanstack/react-query.gen";
import type { LogEntry } from "@/api/gen/types.gen";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/misc";
import { formatDateTime } from "@/lib/format";
import { m } from "@/paraglide/messages";

const sourceIcon = { upload: UploadIcon, mail: MailIcon, folder: FolderIcon, reader: SparklesIcon };

const outcome: Record<
  LogEntry["outcome"],
  { text: () => string; variant: "success" | "warning" | "destructive" | "outline" }
> = {
  imported: { text: m.log_imported, variant: "success" },
  duplicate: { text: m.log_duplicate, variant: "outline" },
  rejected: { text: m.log_rejected, variant: "destructive" },
  read: { text: m.log_read, variant: "success" },
  read_failed: { text: m.log_read_failed, variant: "warning" },
};

/** What happened to incoming files and receipt readings, newest first. */
export function ImportLogCard() {
  const q = useQuery({ ...getImportLogOptions({ query: { limit: 50 } }), refetchInterval: 30_000 });
  return (
    <Card>
      <CardHeader>
        <CardTitle>{m.settings_import_log()}</CardTitle>
        <CardDescription>{m.settings_import_log_text()}</CardDescription>
      </CardHeader>
      <CardContent>
        {!q.data ? (
          <Skeleton className="h-24" />
        ) : q.data.length === 0 ? (
          <p className="text-sm text-muted-foreground">{m.log_empty()}</p>
        ) : (
          <ul className="max-h-[28rem] divide-y overflow-y-auto text-sm">
            {q.data.map((e) => {
              const Icon = sourceIcon[e.source];
              const o = outcome[e.outcome];
              return (
                <li key={e.id} className="grid gap-0.5 py-2">
                  <div className="flex items-center gap-2">
                    <Icon className="size-3.5 shrink-0 text-muted-foreground" aria-label={e.source} />
                    {e.receiptId ? (
                      <Link
                        to="/receipts/$id"
                        params={{ id: String(e.receiptId) }}
                        className="min-w-0 truncate underline"
                      >
                        {e.filename || m.receipt_n({ n: String(e.receiptId) })}
                      </Link>
                    ) : (
                      <span className="min-w-0 truncate">{e.filename}</span>
                    )}
                    <Badge variant={o.variant} className="ml-auto">
                      {o.text()}
                    </Badge>
                  </div>
                  <p className="break-words text-xs text-muted-foreground">
                    {formatDateTime(e.at)}
                    {e.message ? ` · ${e.message}` : ""}
                  </p>
                </li>
              );
            })}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
