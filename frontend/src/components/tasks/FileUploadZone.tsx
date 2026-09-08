import { useState, useRef } from "react";
import { Upload as UploadIcon, X, File } from "lucide-react";
import type { Upload } from "../../api/tasks";
import { uploadFile, deleteUpload } from "../../api/tasks";

interface FileUploadZoneProps {
  taskId: string;
  requirementId: string;
  uploads: Upload[];
  onUploadsChange: (uploads: Upload[]) => void;
  // When true, hide the upload dropzone and disable delete buttons.
  // Already-uploaded files stay listed (read-only) but can't be changed.
  readOnly?: boolean;
}

export function FileUploadZone({ taskId, requirementId, uploads, onUploadsChange, readOnly = false }: FileUploadZoneProps) {
  const [dragover, setDragover] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [status, setStatus] = useState<{ kind: "success" | "error"; message: string } | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  async function handleFiles(files: FileList) {
    setUploading(true);
    setStatus(null);
    let next = uploads;
    try {
      for (const file of Array.from(files)) {
        const upload = await uploadFile(taskId, requirementId, file);
        next = [...next, upload];
        onUploadsChange(next);
        setStatus({ kind: "success", message: `${file.name} uploaded and scanned` });
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : "upload failed";
      setStatus({ kind: "error", message });
    } finally {
      setUploading(false);
      // Reset the input value so re-selecting the same filename fires
      // onChange again. Without this, picking the same file after deletion
      // is a silent no-op because the input's value hasn't changed.
      if (inputRef.current) inputRef.current.value = "";
    }
  }

  async function handleDelete(uploadId: string) {
    try {
      await deleteUpload(taskId, uploadId);
      onUploadsChange(uploads.filter(u => u.id !== uploadId));
      // Clear the stale "uploaded and scanned" banner — the file it
      // refers to is gone. Next upload (which the backend always
      // re-scans) will set a fresh banner.
      setStatus(null);
    } catch (err) {
      console.error("Delete failed", err);
    }
  }

  const relevantUploads = uploads.filter(u => u.requirement_id === requirementId);

  return (
    <div>
      {relevantUploads.map(u => (
        <div key={u.id} style={{
          display: "flex", alignItems: "center", gap: 8, padding: "8px 12px",
          border: "1px solid #E8ECF0", borderRadius: 6, marginBottom: 8, fontSize: 13,
        }}>
          <File size={16} />
          <span style={{ flex: 1 }}>{u.file_name}</span>
          <span style={{ color: "#6B8294" }}>{(u.file_size / 1024).toFixed(0)} KB</span>
          {!readOnly && (
            <button type="button" className="people-chip-remove" onClick={() => handleDelete(u.id)}>
              <X size={10} />
            </button>
          )}
        </div>
      ))}
      {!readOnly && (
        <div
          className={`file-upload-zone${dragover ? " dragover" : ""}`}
          onClick={() => inputRef.current?.click()}
          onDragOver={e => { e.preventDefault(); setDragover(true); }}
          onDragLeave={() => setDragover(false)}
          onDrop={e => { e.preventDefault(); setDragover(false); handleFiles(e.dataTransfer.files); }}
        >
          <UploadIcon size={24} style={{ marginBottom: 8, color: "#8CA8BE" }} />
          <div className="file-upload-zone-label">
            {uploading ? "Uploading..." : "Drop file here or click to browse"}
          </div>
        </div>
      )}
      {status && (
        <div
          role={status.kind === "error" ? "alert" : "status"}
          style={{
            marginTop: 8,
            padding: "6px 10px",
            fontSize: 13,
            borderRadius: 6,
            border: `1px solid ${status.kind === "error" ? "#E5484D" : "#2A9E5C"}`,
            color: status.kind === "error" ? "#B01A1F" : "#1F7A46",
            background: status.kind === "error" ? "#FDEAEA" : "#E7F6EC",
          }}
        >
          {status.kind === "success" ? "✓ " : "✗ "}
          {status.message}
        </div>
      )}
      <input
        ref={inputRef}
        type="file"
        style={{ display: "none" }}
        onChange={e => { if (e.target.files) handleFiles(e.target.files); }}
      />
    </div>
  );
}
