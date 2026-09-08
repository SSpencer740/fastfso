import { useState } from "react";
import { Modal } from "../ui/Modal";
import { Wizard } from "../ui/Wizard";
import { Alert } from "../ui/Alert";
import { TripInfoStep } from "./TripInfoStep";
import { TravelDetailsStep, type CountryDetail } from "./TravelDetailsStep";
import { ContactsSafetyStep } from "./ContactsSafetyStep";
import { ReviewStep } from "./ReviewStep";
import {
  createTravelReport,
  uploadTravelFile,
  submitTravelReport,
} from "../../api/travel";

interface ReportTravelWizardProps {
  open: boolean;
  onClose: () => void;
  onCreated: () => void;
}

export type FileStatus =
  | { kind: "idle" }
  | { kind: "scanning" }
  | { kind: "clean" }
  | { kind: "rejected"; error: string };

export function ReportTravelWizard({ open, onClose, onCreated }: ReportTravelWizardProps) {
  // Step 1: Trip info
  const [tripName, setTripName] = useState("");
  const [multiCountry, setMultiCountry] = useState(false);
  const [countries, setCountries] = useState<string[]>([""]);
  const [passportNumber, setPassportNumber] = useState("");
  const [files, setFiles] = useState<File[]>([]);
  const [fileStatuses, setFileStatuses] = useState<FileStatus[]>([]);

  // Step 2: Country details
  const [countryDetails, setCountryDetails] = useState<Record<string, CountryDetail>>({});

  // Step 3: Emergency contact
  const [emergencyFirstName, setEmergencyFirstName] = useState("");
  const [emergencyLastName, setEmergencyLastName] = useState("");
  const [emergencyPhone, setEmergencyPhone] = useState("");
  const [additionalComments, setAdditionalComments] = useState("");

  // Wizard state
  const [currentStep, setCurrentStep] = useState(0);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  function resetForm() {
    setTripName("");
    setMultiCountry(false);
    setCountries([""]);
    setPassportNumber("");
    setFiles([]);
    setFileStatuses([]);
    setCountryDetails({});
    setEmergencyFirstName("");
    setEmergencyLastName("");
    setEmergencyPhone("");
    setAdditionalComments("");
    setCurrentStep(0);
    setSubmitting(false);
    setError("");
  }

  function handleFilesChange(next: File[]) {
    // Keep statuses aligned by File reference when the list grows or shrinks.
    const nextStatuses = next.map((f) => {
      const prev = files.indexOf(f);
      return prev >= 0 ? fileStatuses[prev] : { kind: "idle" as const };
    });
    setFiles(next);
    setFileStatuses(nextStatuses);
  }

  function handleClose() {
    resetForm();
    onClose();
  }

  async function handleSubmit() {
    setError("");
    setSubmitting(true);
    setFileStatuses(files.map(() => ({ kind: "idle" as const })));

    try {
      // Build country request data
      const countryRequests = countries
        .filter((c) => c.trim() !== "")
        .map((country, i) => {
          const detail = countryDetails[country] ?? {
            startDate: "",
            endDate: "",
            reason: "",
            transportation: [],
            hasCompanions: false,
            companionsDetail: "",
            hasForeignContacts: false,
            contactsDetail: "",
          };
          return {
            country_name: country,
            sort_order: i,
            start_date: detail.startDate || null,
            end_date: detail.endDate || null,
            reason: detail.reason,
            transportation: detail.transportation,
            has_companions: detail.hasCompanions,
            companions_detail: detail.companionsDetail,
            has_foreign_contacts: detail.hasForeignContacts,
            contacts_detail: detail.contactsDetail,
          };
        });

      // 1. Create report as draft
      const report = await createTravelReport({
        trip_name: tripName,
        multi_country: multiCountry,
        passport_number: passportNumber,
        emergency_first_name: emergencyFirstName,
        emergency_last_name: emergencyLastName,
        emergency_phone: emergencyPhone,
        additional_comments: additionalComments,
        countries: countryRequests,
      });

      // 2. Upload files one at a time, showing per-file scan status.
      for (let i = 0; i < files.length; i++) {
        const file = files[i];
        setFileStatuses((prev) =>
          prev.map((s, idx) => (idx === i ? { kind: "scanning" } : s)),
        );
        try {
          await uploadTravelFile(report.id, file);
          setFileStatuses((prev) =>
            prev.map((s, idx) => (idx === i ? { kind: "clean" } : s)),
          );
        } catch (err) {
          const msg = err instanceof Error ? err.message : "upload failed";
          setFileStatuses((prev) =>
            prev.map((s, idx) => (idx === i ? { kind: "rejected", error: msg } : s)),
          );
          throw new Error(`${file.name}: ${msg}`);
        }
      }

      // 3. Submit
      await submitTravelReport(report.id);

      // 4. Done
      resetForm();
      onCreated();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to submit travel report");
    } finally {
      setSubmitting(false);
    }
  }

  const steps = [
    {
      label: "Trip Info",
      content: (
        <TripInfoStep
          tripName={tripName}
          onTripNameChange={setTripName}
          multiCountry={multiCountry}
          onMultiCountryChange={setMultiCountry}
          countries={countries}
          onCountriesChange={setCountries}
          passportNumber={passportNumber}
          onPassportNumberChange={setPassportNumber}
          files={files}
          onFilesChange={handleFilesChange}
          fileStatuses={fileStatuses}
        />
      ),
    },
    {
      label: "Travel Details",
      content: (
        <TravelDetailsStep
          countries={countries.filter((c) => c.trim() !== "")}
          countryDetails={countryDetails}
          onCountryDetailsChange={setCountryDetails}
        />
      ),
    },
    {
      label: "Contacts & Safety",
      content: (
        <ContactsSafetyStep
          emergencyFirstName={emergencyFirstName}
          onEmergencyFirstNameChange={setEmergencyFirstName}
          emergencyLastName={emergencyLastName}
          onEmergencyLastNameChange={setEmergencyLastName}
          emergencyPhone={emergencyPhone}
          onEmergencyPhoneChange={setEmergencyPhone}
          additionalComments={additionalComments}
          onAdditionalCommentsChange={setAdditionalComments}
        />
      ),
    },
    {
      label: "Review",
      content: (
        <ReviewStep
          tripName={tripName}
          multiCountry={multiCountry}
          countries={countries.filter((c) => c.trim() !== "")}
          passportNumber={passportNumber}
          countryDetails={countryDetails}
          emergencyFirstName={emergencyFirstName}
          emergencyLastName={emergencyLastName}
          emergencyPhone={emergencyPhone}
          additionalComments={additionalComments}
          files={files}
          fileStatuses={fileStatuses}
        />
      ),
    },
  ];

  return (
    <Modal open={open} onClose={handleClose} title="Report Foreign Travel">
      {error && (
        <Alert variant="error" onDismiss={() => setError("")}>
          {error}
        </Alert>
      )}
      <Wizard
        steps={steps}
        currentStep={currentStep}
        onStepChange={setCurrentStep}
        onSubmit={handleSubmit}
        submitLabel="Submit Report"
        submitting={submitting}
      />
    </Modal>
  );
}
