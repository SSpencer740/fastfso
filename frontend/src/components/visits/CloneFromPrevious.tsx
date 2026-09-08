import { useEffect, useState } from "react";
import { Spinner } from "../ui/Spinner";
import { Alert } from "../ui/Alert";
import { Button } from "../ui/Button";
import { type VisitRequestRow, listCloneable, formatAccessLevel } from "../../api/visits";
import { ApiError } from "../../api/client";

interface CloneFromPreviousProps {
  onSelect: (request: VisitRequestRow) => void;
  onCancel: () => void;
}

export function CloneFromPrevious({ onSelect, onCancel }: CloneFromPreviousProps) {
  const [requests, setRequests] = useState<VisitRequestRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    async function load() {
      try {
        const data = await listCloneable();
        setRequests(data.requests ?? []);
      } catch (err) {
        setError(err instanceof ApiError ? err.message : "Failed to load previous requests");
      } finally {
        setLoading(false);
      }
    }
    load();
  }, []);

  if (loading) {
    return <Spinner text="Loading previous requests..." />;
  }

  return (
    <div>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: "1rem" }}>
        <h4 style={{ margin: 0 }}>Select a previous request to clone</h4>
        <Button size="sm" variant="secondary" onClick={onCancel}>Cancel</Button>
      </div>

      {error && <Alert variant="error">{error}</Alert>}

      {requests.length === 0 ? (
        <p style={{ color: "var(--color-text-muted)" }}>No previous requests to clone from.</p>
      ) : (
        <div className="task-list">
          {requests.map((req) => (
            <div
              key={req.id}
              className="task-row"
              onClick={() => onSelect(req)}
              role="button"
              tabIndex={0}
              onKeyDown={(e) => { if (e.key === "Enter") onSelect(req); }}
            >
              <div className="task-row-title">{req.destination_name}</div>
              <div className="task-row-meta">
                {formatAccessLevel(req.access_level)} &middot; {new Date(req.visit_start_date).toLocaleDateString()} &ndash; {new Date(req.visit_end_date).toLocaleDateString()}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
