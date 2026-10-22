import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { CameraIcon, Loader2Icon } from "lucide-react";
import { useRef } from "react";
import { toast } from "sonner";
import { uploadReceipt } from "@/api/gen/sdk.gen";
import { Button } from "@/components/ui/button";
import { errorText } from "@/lib/api";
import { cn } from "@/lib/utils";
import { m } from "@/paraglide/messages";

/** Uploads files as receipts to check; returns the created receipts. */
export function useUploadReceipts() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (files: File[]) => {
      const results = [];
      for (const file of files) {
        const { data } = await uploadReceipt({
          query: { filename: file.name },
          body: file,
          throwOnError: true,
        });
        results.push(data);
      }
      return results;
    },
    onSuccess: (r) => {
      qc.invalidateQueries({ queryKey: [{ _id: "listReceipts" }] });
      qc.invalidateQueries({ queryKey: [{ _id: "getMonth" }] });
      toast.success(m.receipt_uploaded({ count: String(r.length) }));
    },
    onError: (err) => toast.error(errorText(err)),
  });
}

/**
 * Capture button. Without a `capture` attribute phones offer camera, photo
 * library and files (iOS has no share target, so this covers existing files).
 * After the upload the check screen of the first receipt opens.
 */
export function ReceiptCapture({
  iconOnly,
  variant = "default",
  className,
}: {
  iconOnly?: boolean;
  variant?: "default" | "outline";
  className?: string;
}) {
  const input = useRef<HTMLInputElement>(null);
  const upload = useUploadReceipts();
  const navigate = useNavigate();
  const onFiles = (files: FileList | null) => {
    if (!files?.length) return;
    upload.mutate(Array.from(files), {
      onSuccess: (r) => {
        if (r[0]) navigate({ to: "/receipts/$id", params: { id: String(r[0].id) } });
      },
    });
    if (input.current) input.current.value = "";
  };
  const icon = upload.isPending ? <Loader2Icon className="animate-spin" /> : <CameraIcon />;
  return (
    <>
      <input
        ref={input}
        type="file"
        accept="image/jpeg,image/png,application/pdf"
        multiple
        className="sr-only"
        tabIndex={-1}
        aria-hidden
        data-testid="receipt-input"
        onChange={(e) => onFiles(e.target.files)}
      />
      {iconOnly ? (
        <button
          type="button"
          onClick={() => input.current?.click()}
          disabled={upload.isPending}
          aria-label={m.capture_receipt()}
          className="-mt-5 grid size-14 place-items-center rounded-full bg-primary text-primary-foreground shadow-lg ring-4 ring-background [&_svg]:size-6"
        >
          {icon}
        </button>
      ) : (
        <Button
          variant={variant}
          className={cn(className)}
          onClick={() => input.current?.click()}
          disabled={upload.isPending}
        >
          {icon}
          {m.capture_receipt()}
        </Button>
      )}
    </>
  );
}
