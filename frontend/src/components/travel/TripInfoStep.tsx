import { useCallback } from "react";
import { FormField } from "../ui/FormField";
import type { FileStatus } from "./ReportTravelWizard";
import { FileStatusBadge } from "./FileStatusBadge";

interface TripInfoStepProps {
  tripName: string;
  onTripNameChange: (v: string) => void;
  multiCountry: boolean;
  onMultiCountryChange: (v: boolean) => void;
  countries: string[];
  onCountriesChange: (v: string[]) => void;
  passportNumber: string;
  onPassportNumberChange: (v: string) => void;
  files: File[];
  onFilesChange: (v: File[]) => void;
  fileStatuses: FileStatus[];
}

export function TripInfoStep({
  tripName,
  onTripNameChange,
  multiCountry,
  onMultiCountryChange,
  countries,
  onCountriesChange,
  passportNumber,
  onPassportNumberChange,
  files,
  onFilesChange,
  fileStatuses,
}: TripInfoStepProps) {
  function handleCountryChange(index: number, value: string) {
    const updated = [...countries];
    updated[index] = value;
    onCountriesChange(updated);
  }

  function addCountry() {
    onCountriesChange([...countries, ""]);
  }

  function removeCountry(index: number) {
    if (countries.length <= 1) return;
    onCountriesChange(countries.filter((_, i) => i !== index));
  }

  function handleMultiCountryToggle(checked: boolean) {
    onMultiCountryChange(checked);
    if (!checked && countries.length > 1) {
      onCountriesChange([countries[0]]);
    }
  }

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      const dropped = Array.from(e.dataTransfer.files);
      if (dropped.length > 0) {
        onFilesChange([...files, ...dropped]);
      }
    },
    [files, onFilesChange],
  );

  function handleFileInput(e: React.ChangeEvent<HTMLInputElement>) {
    const selected = e.target.files ? Array.from(e.target.files) : [];
    if (selected.length > 0) {
      onFilesChange([...files, ...selected]);
    }
    e.target.value = "";
  }

  function removeFile(index: number) {
    onFilesChange(files.filter((_, i) => i !== index));
  }

  return (
    <div>
      <FormField label="Trip Name" htmlFor="trip-name">
        <input
          id="trip-name"
          type="text"
          value={tripName}
          onChange={(e) => onTripNameChange(e.target.value)}
          placeholder="e.g. Berlin Conference 2026"
          required
        />
      </FormField>

      <div style={{ marginBottom: "1rem" }}>
        <label
          style={{
            display: "flex",
            alignItems: "center",
            gap: "0.5rem",
            fontSize: "0.85rem",
            color: "var(--color-text-secondary)",
            cursor: "pointer",
          }}
        >
          <input
            type="checkbox"
            checked={multiCountry}
            onChange={(e) => handleMultiCountryToggle(e.target.checked)}
          />
          Multi-country trip
        </label>
      </div>

      <div style={{ marginBottom: "1rem" }}>
        <label
          style={{
            display: "block",
            fontSize: "0.85rem",
            fontWeight: 500,
            marginBottom: "0.35rem",
            color: "var(--color-text-secondary)",
          }}
        >
          {multiCountry ? "Countries" : "Country"}
        </label>
        {countries.map((country, i) => (
          <div
            key={i}
            style={{
              display: "flex",
              gap: "0.5rem",
              alignItems: "center",
              marginBottom: "0.5rem",
            }}
          >
            <input
              type="text"
              value={country}
              onChange={(e) => handleCountryChange(i, e.target.value)}
              placeholder={`Country ${multiCountry ? i + 1 : ""}`}
              required
              style={{
                flex: 1,
                padding: "0.6rem 0.75rem",
                border: "1px solid var(--color-border-strong)",
                borderRadius: "var(--radius-md)",
                background: "var(--color-bg-elevated)",
                color: "inherit",
                fontSize: "1rem",
                fontFamily: "inherit",
              }}
            />
            {multiCountry && countries.length > 1 && (
              <button
                type="button"
                className="btn btn-sm"
                onClick={() => removeCountry(i)}
              >
                Remove
              </button>
            )}
          </div>
        ))}
        {multiCountry && (
          <button type="button" className="btn btn-sm" onClick={addCountry}>
            + Add Country
          </button>
        )}
      </div>

      <FormField label="Passport Number" htmlFor="passport-number">
        <input
          id="passport-number"
          type="text"
          value={passportNumber}
          onChange={(e) => onPassportNumberChange(e.target.value)}
          placeholder="Enter passport number"
        />
      </FormField>

      <div style={{ marginBottom: "1rem" }}>
        <label
          style={{
            display: "block",
            fontSize: "0.85rem",
            fontWeight: 500,
            marginBottom: "0.35rem",
            color: "var(--color-text-secondary)",
          }}
        >
          Supporting Documents
        </label>
        <div
          onDrop={handleDrop}
          onDragOver={(e) => e.preventDefault()}
          style={{
            border: "2px dashed var(--color-border-strong)",
            borderRadius: "var(--radius-lg)",
            padding: "1.5rem",
            textAlign: "center",
            color: "var(--color-text-muted)",
            fontSize: "0.85rem",
            cursor: "pointer",
          }}
          onClick={() => document.getElementById("travel-file-input")?.click()}
        >
          <p style={{ margin: "0 0 0.5rem" }}>
            Drag and drop files here, or click to select
          </p>
          <input
            id="travel-file-input"
            type="file"
            multiple
            onChange={handleFileInput}
            style={{ display: "none" }}
          />
        </div>
        {files.length > 0 && (
          <div style={{ marginTop: "0.5rem" }}>
            {files.map((file, i) => {
              const status = fileStatuses[i] ?? { kind: "idle" };
              return (
                <div
                  key={`${file.name}-${i}`}
                  style={{
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "space-between",
                    gap: "0.5rem",
                    padding: "0.4rem 0.6rem",
                    marginBottom: "0.25rem",
                    border: "1px solid var(--color-border)",
                    borderRadius: "var(--radius-sm)",
                    fontSize: "0.8rem",
                  }}
                >
                  <span style={{ flex: 1, minWidth: 0 }}>
                    {file.name}{" "}
                    <span style={{ color: "var(--color-text-muted)" }}>
                      ({(file.size / 1024).toFixed(1)} KB)
                    </span>
                  </span>
                  <FileStatusBadge status={status} />
                  <button
                    type="button"
                    className="btn btn-sm"
                    onClick={() => removeFile(i)}
                    disabled={status.kind === "scanning"}
                  >
                    Remove
                  </button>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
