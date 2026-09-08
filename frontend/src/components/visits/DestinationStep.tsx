import { FormField } from "../ui/FormField";

interface DestinationStepProps {
  destinationName: string;
  onDestinationNameChange: (v: string) => void;
  dissSmoCode: string;
  onDissSmoCodeChange: (v: string) => void;
  visitAddress: string;
  onVisitAddressChange: (v: string) => void;
  visitStartDate: string;
  onVisitStartDateChange: (v: string) => void;
  visitEndDate: string;
  onVisitEndDateChange: (v: string) => void;
  onCloneRequest: () => void;
}

export function DestinationStep({
  destinationName,
  onDestinationNameChange,
  dissSmoCode,
  onDissSmoCodeChange,
  visitAddress,
  onVisitAddressChange,
  visitStartDate,
  onVisitStartDateChange,
  visitEndDate,
  onVisitEndDateChange,
  onCloneRequest,
}: DestinationStepProps) {
  return (
    <div>
      <div style={{ marginBottom: "1rem" }}>
        <button type="button" className="link-button" onClick={onCloneRequest}>
          Clone from a previous request
        </button>
      </div>

      <FormField label="Company / Facility Name" htmlFor="destination-name">
        <input
          id="destination-name"
          type="text"
          value={destinationName}
          onChange={(e) => onDestinationNameChange(e.target.value)}
          required
        />
      </FormField>

      <FormField label="DISS SMO Code" htmlFor="diss-smo-code">
        <input
          id="diss-smo-code"
          type="text"
          value={dissSmoCode}
          onChange={(e) => onDissSmoCodeChange(e.target.value)}
          required
        />
      </FormField>

      <FormField label="Address" htmlFor="visit-address">
        <input
          id="visit-address"
          type="text"
          value={visitAddress}
          onChange={(e) => onVisitAddressChange(e.target.value)}
          required
        />
      </FormField>

      <FormField label="Start Date" htmlFor="visit-start-date">
        <input
          id="visit-start-date"
          type="date"
          value={visitStartDate}
          onChange={(e) => {
            onVisitStartDateChange(e.target.value);
            if (visitEndDate && e.target.value > visitEndDate) {
              onVisitEndDateChange("");
            }
          }}
          required
        />
      </FormField>

      <FormField label="End Date" htmlFor="visit-end-date">
        <input
          id="visit-end-date"
          type="date"
          value={visitEndDate}
          min={visitStartDate || undefined}
          onChange={(e) => onVisitEndDateChange(e.target.value)}
          required
        />
      </FormField>
    </div>
  );
}
