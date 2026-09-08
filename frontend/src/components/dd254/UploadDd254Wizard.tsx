import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useAuthStore } from "../../stores/authStore";
import { Modal } from "../ui/Modal";
import { Wizard, type WizardStep } from "../ui/Wizard";
import { FormField } from "../ui/FormField";
import { Alert } from "../ui/Alert";
import { uploadDd254, type UploadDd254Result } from "../../api/dd254";
import { listSubOrgs } from "../../api/customerAdmin";
import { clearanceLabels, type ClearanceLevel } from "../../api/team";

interface UploadDd254WizardProps {
  open: boolean;
  onClose: () => void;
  onUploaded: (result: UploadDd254Result) => void;
}

const classificationOptions: ClearanceLevel[] = [
  "confidential",
  "secret",
  "top_secret",
  "ts_sci",
];

export function UploadDd254Wizard({ open, onClose, onUploaded }: UploadDd254WizardProps) {
  const role = useAuthStore((s) => s.user?.role);
  const [step, setStep] = useState(0);
  const [file, setFile] = useState<File | null>(null);
  const [contractNumber, setContractNumber] = useState("");
  const [primeContractor, setPrimeContractor] = useState("");
  const [classMax, setClassMax] = useState<ClearanceLevel>("secret");
  const [periodStart, setPeriodStart] = useState("");
  const [periodEnd, setPeriodEnd] = useState("");
  const [subOrgId, setSubOrgId] = useState("");
  const [attested, setAttested] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { data: subOrgsResp } = useQuery({
    queryKey: ["sub-orgs"],
    queryFn: listSubOrgs,
    enabled: role === "administrator",
  });
  const subOrgs = (subOrgsResp?.sub_orgs ?? []).filter((o) => o.name !== "Default");

  function reset() {
    setStep(0);
    setFile(null);
    setContractNumber("");
    setPrimeContractor("");
    setClassMax("secret");
    setPeriodStart("");
    setPeriodEnd("");
    setSubOrgId("");
    setAttested(false);
    setError(null);
  }

  function handleClose() {
    if (submitting) return;
    reset();
    onClose();
  }

  async function handleSubmit() {
    if (!file || !attested || !contractNumber) return;
    setSubmitting(true);
    setError(null);
    try {
      const result = await uploadDd254({
        file,
        contract_number: contractNumber,
        prime_contractor: primeContractor,
        classification_max: classMax,
        period_start: periodStart || undefined,
        period_end: periodEnd || undefined,
        sub_org_id: subOrgId || undefined,
      });
      onUploaded(result);
      reset();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Upload failed";
      setError(message);
    } finally {
      setSubmitting(false);
    }
  }

  const fileStep: WizardStep = {
    label: "File",
    content: (
      <div>
        <FormField label="DD254 PDF" htmlFor="dd-file">
          <input
            id="dd-file"
            type="file"
            accept="application/pdf,.pdf"
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
          />
        </FormField>
        {file && (
          <p style={{ color: "var(--color-text-secondary)", fontSize: 13 }}>
            {file.name} ({Math.round(file.size / 1024)} KB)
          </p>
        )}
      </div>
    ),
  };

  const detailsStep: WizardStep = {
    label: "Details",
    content: (
      <div>
        <FormField label="Contract number" htmlFor="dd-contract">
          <input
            id="dd-contract"
            type="text"
            className="filter-input"
            value={contractNumber}
            onChange={(e) => setContractNumber(e.target.value)}
            placeholder="e.g. N00000-00-C-0000"
            required
          />
        </FormField>
        <FormField label="Prime contractor (optional)" htmlFor="dd-prime">
          <input
            id="dd-prime"
            type="text"
            className="filter-input"
            value={primeContractor}
            onChange={(e) => setPrimeContractor(e.target.value)}
            placeholder="e.g. Acme Defense Systems"
          />
        </FormField>
        <FormField label="Classification cap" htmlFor="dd-class">
          <select
            id="dd-class"
            className="filter-select"
            value={classMax}
            onChange={(e) => setClassMax(e.target.value as ClearanceLevel)}
          >
            {classificationOptions.map((c) => (
              <option key={c} value={c}>{clearanceLabels[c]}</option>
            ))}
          </select>
        </FormField>
        <div style={{ display: "flex", gap: 12 }}>
          <FormField label="Period start" htmlFor="dd-ps">
            <input
              id="dd-ps"
              type="date"
              className="filter-input"
              value={periodStart}
              onChange={(e) => setPeriodStart(e.target.value)}
            />
          </FormField>
          <FormField label="Period end" htmlFor="dd-pe">
            <input
              id="dd-pe"
              type="date"
              className="filter-input"
              value={periodEnd}
              onChange={(e) => setPeriodEnd(e.target.value)}
            />
          </FormField>
        </div>
        {role === "administrator" && subOrgs.length > 0 && (
          <FormField label="Sub-organization (optional — leave blank for tenant-wide)" htmlFor="dd-so">
            <select
              id="dd-so"
              className="filter-select"
              value={subOrgId}
              onChange={(e) => setSubOrgId(e.target.value)}
            >
              <option value="">Tenant-wide</option>
              {subOrgs.map((s) => (
                <option key={s.id} value={s.id}>{s.name}</option>
              ))}
            </select>
          </FormField>
        )}
      </div>
    ),
  };

  const attestStep: WizardStep = {
    label: "Confirm",
    content: (
      <div>
        <Alert variant="warning">
          <strong>fastFSO is CMMC Level 1 (self-assessed).</strong> This system is not
          authorized to store CUI, FOUO, PROPIN, or any other controlled marking.
          Verify the document is marked <strong>UNCLASSIFIED</strong> with no
          controlled markings before uploading.
        </Alert>
        <label className="dd254-attest-label">
          <input
            type="checkbox"
            checked={attested}
            onChange={(e) => setAttested(e.target.checked)}
          />{" "}
          I have reviewed the document and confirm it is marked UNCLASSIFIED with no CUI/FOUO/PROPIN.
        </label>
        {error && <p style={{ color: "var(--color-error)", marginTop: 12 }}>{error}</p>}
      </div>
    ),
  };

  const steps: WizardStep[] = [fileStep, detailsStep, attestStep];

  const canSubmit = !!file && !!contractNumber && attested && !submitting;

  return (
    <Modal open={open} onClose={handleClose} title="Upload DD254" size="wide">
      <Wizard
        steps={steps}
        currentStep={step}
        onStepChange={setStep}
        onSubmit={canSubmit ? handleSubmit : () => undefined}
        submitLabel="Upload"
        submitting={submitting}
      />
    </Modal>
  );
}
