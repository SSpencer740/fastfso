import { useEffect, useState } from "react";
import { Modal } from "../ui/Modal";
import { Spinner } from "../ui/Spinner";
import { Alert } from "../ui/Alert";
import { Button } from "../ui/Button";
import { VisitRequestDetails } from "./VisitRequestDetails";
import { DD254AuthorizationPanel, type AuthState } from "./DD254AuthorizationPanel";
import { type VisitRequest, getMyVisit, getAdminVisit, updateVisitStatus, cancelVisitRequest } from "../../api/visits";
import { ApiError } from "../../api/client";
import { useAuthStore } from "../../stores/authStore";
import { isReadOnlyFSO } from "../../utils/roles";

interface ViewVisitModalProps {
  open: boolean;
  onClose: () => void;
  visitId: string | null;
  onUpdated: () => void;
  isAdminView?: boolean;
}

export function ViewVisitModal({ open, onClose, visitId, onUpdated, isAdminView }: ViewVisitModalProps) {
  const role = useAuthStore((s) => s.user?.role);
  const readOnly = isReadOnlyFSO(role);
  const [visit, setVisit] = useState<VisitRequest | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [reviewNotes, setReviewNotes] = useState("");
  const [actionLoading, setActionLoading] = useState(false);
  const [authState, setAuthState] = useState<AuthState>("loading");

  useEffect(() => {
    if (!open || !visitId) return;

    async function load() {
      setLoading(true);
      setError("");
      try {
        const data = isAdminView
          ? await getAdminVisit(visitId!)
          : await getMyVisit(visitId!);
        setVisit(data);
      } catch (err) {
        setError(err instanceof ApiError ? err.message : "Failed to load visit request");
      } finally {
        setLoading(false);
      }
    }
    load();
  }, [open, visitId, isAdminView]);

  function handleClose() {
    setVisit(null);
    setError("");
    setReviewNotes("");
    onClose();
  }

  async function handleStatusUpdate(status: string) {
    if (!visitId) return;
    setActionLoading(true);
    setError("");
    try {
      await updateVisitStatus(visitId, status, reviewNotes);
      onUpdated();
      handleClose();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to update status");
    } finally {
      setActionLoading(false);
    }
  }

  async function handleCancel() {
    if (!visitId) return;
    setActionLoading(true);
    setError("");
    try {
      await cancelVisitRequest(visitId);
      onUpdated();
      handleClose();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to cancel visit request");
    } finally {
      setActionLoading(false);
    }
  }

  return (
    <Modal open={open} onClose={handleClose} title="Visit Request" size="wide">
      {loading && <Spinner text="Loading..." />}
      {error && <Alert variant="error">{error}</Alert>}

      {visit && (
        <div>
          <VisitRequestDetails vr={visit} />

          {isAdminView && (
            <DD254AuthorizationPanel
              visitRequestId={visit.id}
              currentDD254Id={visit.dd_254_id ?? null}
              visitAccessLevel={visit.access_level}
              onAuthStateChange={setAuthState}
            />
          )}

          {!isAdminView && visit.status === "submitted" && (
            <div style={{ marginTop: 16, paddingTop: 16, borderTop: "1px solid var(--color-border)" }}>
              <Button
                size="sm"
                variant="secondary"
                onClick={handleCancel}
                loading={actionLoading}
                loadingText="Cancelling..."
              >
                Cancel Request
              </Button>
            </div>
          )}

          {isAdminView && !readOnly && (visit.status === "submitted" || visit.status === "under_review") && (
            <div className="visit-review-actions">
              <div className="form-field" style={{ maxWidth: "100%" }}>
                <label htmlFor="review-notes">Review Notes</label>
                <textarea
                  id="review-notes"
                  rows={3}
                  value={reviewNotes}
                  onChange={(e) => setReviewNotes(e.target.value)}
                  placeholder="Optional notes..."
                  style={{
                    width: "100%",
                    padding: "0.6rem 0.75rem",
                    border: "1px solid var(--color-border-strong)",
                    borderRadius: "var(--radius-md)",
                    background: "var(--color-bg-elevated)",
                    color: "inherit",
                    fontSize: "0.9rem",
                    fontFamily: "inherit",
                    resize: "vertical",
                  }}
                />
              </div>
              <div style={{ display: "flex", gap: "0.5rem" }}>
                <Button
                  size="sm"
                  onClick={() => handleStatusUpdate("approved")}
                  loading={actionLoading}
                  loadingText="Updating..."
                >
                  {authState === "no_match" ? "Approve anyway" : "Approve"}
                </Button>
                <Button
                  size="sm"
                  variant="danger"
                  onClick={() => handleStatusUpdate("rejected")}
                  loading={actionLoading}
                  loadingText="Updating..."
                >
                  Reject
                </Button>
                {visit.status === "submitted" && (
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => handleStatusUpdate("under_review")}
                    loading={actionLoading}
                    loadingText="Updating..."
                  >
                    Mark Under Review
                  </Button>
                )}
              </div>
            </div>
          )}
        </div>
      )}
    </Modal>
  );
}
