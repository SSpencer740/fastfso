import { useState } from "react";
import { Eye, EyeOff, ExternalLink } from "lucide-react";
import type { Upload } from "../../api/tasks";
import { getUploadDownloadUrl } from "../../api/tasks";

// UploadPreview shows the upload filename plus an inline preview toggle for
// PDFs and common image types. The backend's signed-URL redirect works for
// both <img> and <embed> sources, so no extra fetch is needed.
//
// Collapsed by default to keep the modal compact; admin clicks "Preview" to
// expand. Falls back to a download-only link for unsupported content types.
export function UploadPreview({ upload }: { upload: Upload }) {
  const [open, setOpen] = useState(false);
  const url = getUploadDownloadUrl(upload.id);
  const isImage = upload.content_type.startsWith("image/");
  const isPDF = upload.content_type === "application/pdf";
  const canPreview = isImage || isPDF;

  return (
    <div style={{ marginBottom: 4 }}>
      <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
        <span style={{ fontSize: 13 }}>{upload.file_name}</span>
        <span style={{ fontSize: 12, color: "var(--color-text-muted)" }}>
          {(upload.file_size / 1024).toFixed(0)} KB
        </span>
        {canPreview && (
          <button
            type="button"
            onClick={() => setOpen(o => !o)}
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 4,
              padding: "2px 6px",
              fontSize: 12,
              border: "1px solid var(--color-border)",
              borderRadius: 4,
              background: "transparent",
              color: "var(--color-text)",
              cursor: "pointer",
            }}
          >
            {open ? <EyeOff size={12} /> : <Eye size={12} />}
            {open ? "Hide preview" : "Preview"}
          </button>
        )}
        <a
          href={url}
          target="_blank"
          rel="noreferrer"
          style={{
            display: "inline-flex",
            alignItems: "center",
            gap: 4,
            padding: "2px 6px",
            fontSize: 12,
            color: "var(--color-primary)",
            textDecoration: "none",
          }}
        >
          <ExternalLink size={12} />
          Open
        </a>
      </div>

      {open && isImage && (
        <img
          src={url}
          alt={upload.file_name}
          style={{
            marginTop: 6,
            maxWidth: "100%",
            maxHeight: 400,
            border: "1px solid var(--color-border)",
            borderRadius: "var(--radius-sm)",
            background: "var(--color-bg-elevated)",
          }}
        />
      )}
      {open && isPDF && (
        // iframe handles PDFs more reliably across browsers than <embed>;
        // Safari in particular sometimes refuses to render <embed> PDFs.
        <iframe
          src={url}
          title={upload.file_name}
          style={{
            marginTop: 6,
            width: "100%",
            height: 500,
            border: "1px solid var(--color-border)",
            borderRadius: "var(--radius-sm)",
            background: "var(--color-bg-elevated)",
          }}
        />
      )}
    </div>
  );
}
