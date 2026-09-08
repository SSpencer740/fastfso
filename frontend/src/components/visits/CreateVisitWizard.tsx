import { useEffect, useState } from "react";
import { Modal } from "../ui/Modal";
import { Wizard } from "../ui/Wizard";
import { Alert } from "../ui/Alert";
import { DestinationStep } from "./DestinationStep";
import { AccessPurposeStep } from "./AccessPurposeStep";
import { ContactsStep } from "./ContactsStep";
import { CloneFromPrevious } from "./CloneFromPrevious";
import {
  type VisitRequestRow,
  type VisitRequest,
  type MyDd254Authorization,
  createVisitRequest,
  getMyVisit,
  getMyDd254Authorizations,
} from "../../api/visits";
import { ApiError } from "../../api/client";

interface CreateVisitWizardProps {
  open: boolean;
  onClose: () => void;
  onCreated: () => void;
}

export function CreateVisitWizard({ open, onClose, onCreated }: CreateVisitWizardProps) {
  const [currentStep, setCurrentStep] = useState(0);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [showClone, setShowClone] = useState(false);
  const [clonedFromId, setClonedFromId] = useState<string | null>(null);

  // Form fields
  const [destinationName, setDestinationName] = useState("");
  const [dissSmoCode, setDissSmoCode] = useState("");
  const [visitAddress, setVisitAddress] = useState("");
  const [visitStartDate, setVisitStartDate] = useState("");
  const [visitEndDate, setVisitEndDate] = useState("");
  const [accessLevel, setAccessLevel] = useState("");
  const [visitDescription, setVisitDescription] = useState("");
  const [pocName, setPocName] = useState("");
  const [pocEmail, setPocEmail] = useState("");
  const [pocPhone, setPocPhone] = useState("");
  const [securityPocName, setSecurityPocName] = useState("");
  const [securityPocEmail, setSecurityPocEmail] = useState("");
  const [securityPocPhone, setSecurityPocPhone] = useState("");
  const [dd254Id, setDd254Id] = useState("");
  const [authorizations, setAuthorizations] = useState<MyDd254Authorization[]>([]);

  useEffect(() => {
    if (!open) return;
    getMyDd254Authorizations()
      .then(setAuthorizations)
      .catch(() => setAuthorizations([])); // soft fail — user can still submit without picking a contract
  }, [open]);

  function resetForm() {
    setCurrentStep(0);
    setError("");
    setShowClone(false);
    setClonedFromId(null);
    setDestinationName("");
    setDissSmoCode("");
    setVisitAddress("");
    setVisitStartDate("");
    setVisitEndDate("");
    setAccessLevel("");
    setVisitDescription("");
    setPocName("");
    setPocEmail("");
    setPocPhone("");
    setSecurityPocName("");
    setSecurityPocEmail("");
    setSecurityPocPhone("");
    setDd254Id("");
  }

  function handleClose() {
    resetForm();
    onClose();
  }

  function populateFromVisit(visit: VisitRequest) {
    setDestinationName(visit.destination_name);
    setDissSmoCode(visit.diss_smo_code);
    setVisitAddress(visit.visit_address);
    // API returns full ISO timestamps; date inputs need YYYY-MM-DD only
    setVisitStartDate(visit.visit_start_date.split("T")[0]);
    setVisitEndDate(visit.visit_end_date.split("T")[0]);
    setAccessLevel(visit.access_level);
    setVisitDescription(visit.visit_description);
    setPocName(visit.poc_name);
    setPocEmail(visit.poc_email);
    setPocPhone(visit.poc_phone);
    setSecurityPocName(visit.security_poc_name);
    setSecurityPocEmail(visit.security_poc_email);
    setSecurityPocPhone(visit.security_poc_phone);
  }

  async function handleCloneSelect(row: VisitRequestRow) {
    try {
      const visit = await getMyVisit(row.id);
      populateFromVisit(visit);
      setClonedFromId(row.id);
      setShowClone(false);
      setCurrentStep(0); // Land on Destination so user can review/update dates
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load request details");
    }
  }

  async function handleSubmit() {
    setError("");

    if (!destinationName || !dissSmoCode || !visitAddress || !visitStartDate || !visitEndDate) {
      setError("Please complete all destination fields (step 1).");
      return;
    }
    if (!accessLevel || !visitDescription) {
      setError("Please complete all access & purpose fields (step 2).");
      return;
    }
    if (!pocName || !pocEmail || !pocPhone || !securityPocName || !securityPocEmail || !securityPocPhone) {
      setError("Please complete all contact fields (step 3).");
      return;
    }
    const emailRe = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    if (!emailRe.test(pocEmail)) {
      setError("POC email address is not valid.");
      return;
    }
    if (!emailRe.test(securityPocEmail)) {
      setError("Security POC email address is not valid.");
      return;
    }
    if (visitEndDate < visitStartDate) {
      setError("End date cannot be before start date.");
      return;
    }

    setSubmitting(true);
    try {
      await createVisitRequest({
        destination_name: destinationName,
        diss_smo_code: dissSmoCode,
        visit_address: visitAddress,
        visit_start_date: visitStartDate,
        visit_end_date: visitEndDate,
        access_level: accessLevel,
        visit_description: visitDescription,
        poc_name: pocName,
        poc_email: pocEmail,
        poc_phone: pocPhone,
        security_poc_name: securityPocName,
        security_poc_email: securityPocEmail,
        security_poc_phone: securityPocPhone,
        cloned_from_id: clonedFromId,
        dd_254_id: dd254Id || null,
      });
      onCreated();
      handleClose();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to create visit request");
    } finally {
      setSubmitting(false);
    }
  }

  const steps = [
    {
      label: "Destination",
      content: showClone ? (
        <CloneFromPrevious
          onSelect={handleCloneSelect}
          onCancel={() => setShowClone(false)}
        />
      ) : (
        <DestinationStep
          destinationName={destinationName}
          onDestinationNameChange={setDestinationName}
          dissSmoCode={dissSmoCode}
          onDissSmoCodeChange={setDissSmoCode}
          visitAddress={visitAddress}
          onVisitAddressChange={setVisitAddress}
          visitStartDate={visitStartDate}
          onVisitStartDateChange={setVisitStartDate}
          visitEndDate={visitEndDate}
          onVisitEndDateChange={setVisitEndDate}
          onCloneRequest={() => setShowClone(true)}
        />
      ),
    },
    {
      label: "Access & Purpose",
      content: (
        <AccessPurposeStep
          accessLevel={accessLevel}
          onAccessLevelChange={setAccessLevel}
          visitDescription={visitDescription}
          onVisitDescriptionChange={setVisitDescription}
          visitStartDate={visitStartDate}
          visitEndDate={visitEndDate}
          authorizations={authorizations}
          dd254Id={dd254Id}
          onDd254IdChange={setDd254Id}
        />
      ),
    },
    {
      label: "Contacts",
      content: (
        <ContactsStep
          pocName={pocName}
          onPocNameChange={setPocName}
          pocEmail={pocEmail}
          onPocEmailChange={setPocEmail}
          pocPhone={pocPhone}
          onPocPhoneChange={setPocPhone}
          securityPocName={securityPocName}
          onSecurityPocNameChange={setSecurityPocName}
          securityPocEmail={securityPocEmail}
          onSecurityPocEmailChange={setSecurityPocEmail}
          securityPocPhone={securityPocPhone}
          onSecurityPocPhoneChange={setSecurityPocPhone}
        />
      ),
    },
  ];

  return (
    <Modal open={open} onClose={handleClose} title="New Visit Request">
      {error && <Alert variant="error" onDismiss={() => setError("")}>{error}</Alert>}
      <Wizard
        steps={steps}
        currentStep={currentStep}
        onStepChange={setCurrentStep}
        onSubmit={handleSubmit}
        submitLabel="Submit Request"
        submitting={submitting}
      />
    </Modal>
  );
}
