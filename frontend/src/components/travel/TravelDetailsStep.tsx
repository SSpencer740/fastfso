import { useState } from "react";
import { FormField } from "../ui/FormField";

export interface CountryDetail {
  startDate: string;
  endDate: string;
  reason: string;
  transportation: string[];
  hasCompanions: boolean;
  companionsDetail: string;
  hasForeignContacts: boolean;
  contactsDetail: string;
}

interface TravelDetailsStepProps {
  countries: string[];
  countryDetails: Record<string, CountryDetail>;
  onCountryDetailsChange: (details: Record<string, CountryDetail>) => void;
}

const REASON_OPTIONS = [
  { value: "", label: "Select reason..." },
  { value: "ngo_missionary", label: "NGO/Missionary" },
  { value: "official_non_dod", label: "Official Non-DoD" },
  { value: "vacation_personal", label: "Vacation/Personal" },
  { value: "other", label: "Other" },
];

const TRANSPORTATION_OPTIONS = [
  { value: "air", label: "Air" },
  { value: "car", label: "Car" },
  { value: "bus", label: "Bus" },
  { value: "train", label: "Train" },
  { value: "ship", label: "Ship" },
  { value: "other", label: "Other" },
];

export function TravelDetailsStep({
  countries,
  countryDetails,
  onCountryDetailsChange,
}: TravelDetailsStepProps) {
  const [activeTab, setActiveTab] = useState(0);

  function getDetail(country: string): CountryDetail {
    return (
      countryDetails[country] ?? {
        startDate: "",
        endDate: "",
        reason: "",
        transportation: [],
        hasCompanions: false,
        companionsDetail: "",
        hasForeignContacts: false,
        contactsDetail: "",
      }
    );
  }

  function updateDetail(country: string, partial: Partial<CountryDetail>) {
    onCountryDetailsChange({
      ...countryDetails,
      [country]: { ...getDetail(country), ...partial },
    });
  }

  function handleTransportChange(country: string, value: string, checked: boolean) {
    const detail = getDetail(country);
    const updated = checked
      ? [...detail.transportation, value]
      : detail.transportation.filter((t) => t !== value);
    updateDetail(country, { transportation: updated });
  }

  const currentCountry = countries[activeTab] ?? countries[0];
  const detail = getDetail(currentCountry);

  return (
    <div>
      {/* Tab bar for multiple countries */}
      {countries.length > 1 && (
        <div
          style={{
            display: "flex",
            gap: 0,
            marginBottom: "1.5rem",
            borderBottom: "1px solid var(--color-border)",
          }}
        >
          {countries.map((country, i) => (
            <button
              key={`${country}-${i}`}
              type="button"
              onClick={() => setActiveTab(i)}
              style={{
                padding: "0.6rem 1.2rem",
                border: "none",
                borderBottom: `2px solid ${i === activeTab ? "var(--color-primary)" : "transparent"}`,
                background: "none",
                color: i === activeTab ? "var(--color-primary)" : "var(--color-text-muted)",
                fontSize: "0.9rem",
                cursor: "pointer",
                fontFamily: "inherit",
                transition: "color 0.2s, border-color 0.2s",
              }}
            >
              {country || `Country ${i + 1}`}
            </button>
          ))}
        </div>
      )}

      {countries.length === 1 && (
        <h4 style={{ margin: "0 0 1rem", fontSize: "0.95rem" }}>
          {currentCountry || "Country Details"}
        </h4>
      )}

      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "1rem" }}>
        <FormField label="Start Date" htmlFor={`start-date-${activeTab}`}>
          <input
            id={`start-date-${activeTab}`}
            type="date"
            value={detail.startDate}
            onChange={(e) => {
              const newStart = e.target.value;
              const update: Partial<CountryDetail> = { startDate: newStart };
              if (detail.endDate && newStart > detail.endDate) update.endDate = "";
              updateDetail(currentCountry, update);
            }}
          />
        </FormField>

        <FormField label="End Date" htmlFor={`end-date-${activeTab}`}>
          <input
            id={`end-date-${activeTab}`}
            type="date"
            value={detail.endDate}
            min={detail.startDate || undefined}
            onChange={(e) => updateDetail(currentCountry, { endDate: e.target.value })}
          />
        </FormField>
      </div>

      <FormField label="Reason for Travel" htmlFor={`reason-${activeTab}`}>
        <select
          id={`reason-${activeTab}`}
          value={detail.reason}
          onChange={(e) => updateDetail(currentCountry, { reason: e.target.value })}
          style={{
            width: "100%",
            padding: "0.6rem 0.75rem",
            border: "1px solid var(--color-border-strong)",
            borderRadius: "var(--radius-md)",
            background: "var(--color-bg-elevated)",
            color: "inherit",
            fontSize: "1rem",
            fontFamily: "inherit",
            appearance: "auto",
          }}
        >
          {REASON_OPTIONS.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
      </FormField>

      <div style={{ marginBottom: "1rem" }}>
        <label
          style={{
            display: "block",
            fontSize: "0.85rem",
            fontWeight: 500,
            marginBottom: "0.5rem",
            color: "var(--color-text-secondary)",
          }}
        >
          Transportation
        </label>
        <div style={{ display: "flex", flexWrap: "wrap", gap: "0.75rem" }}>
          {TRANSPORTATION_OPTIONS.map((opt) => (
            <label
              key={opt.value}
              style={{
                display: "flex",
                alignItems: "center",
                gap: "0.35rem",
                fontSize: "0.85rem",
                cursor: "pointer",
                color: "var(--color-text-secondary)",
              }}
            >
              <input
                type="checkbox"
                checked={detail.transportation.includes(opt.value)}
                onChange={(e) =>
                  handleTransportChange(currentCountry, opt.value, e.target.checked)
                }
              />
              {opt.label}
            </label>
          ))}
        </div>
      </div>

      <div style={{ marginBottom: "1rem" }}>
        <label
          style={{
            display: "flex",
            alignItems: "center",
            gap: "0.5rem",
            fontSize: "0.85rem",
            color: "var(--color-text-secondary)",
            cursor: "pointer",
            marginBottom: "0.5rem",
          }}
        >
          <input
            type="checkbox"
            checked={detail.hasCompanions}
            onChange={(e) =>
              updateDetail(currentCountry, { hasCompanions: e.target.checked })
            }
          />
          Will you have companions?
        </label>
        {detail.hasCompanions && (
          <textarea
            value={detail.companionsDetail}
            onChange={(e) =>
              updateDetail(currentCountry, { companionsDetail: e.target.value })
            }
            placeholder="Describe your companions (names, relationships, etc.)"
            rows={3}
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
        )}
      </div>

      <div style={{ marginBottom: "1rem" }}>
        <label
          style={{
            display: "flex",
            alignItems: "center",
            gap: "0.5rem",
            fontSize: "0.85rem",
            color: "var(--color-text-secondary)",
            cursor: "pointer",
            marginBottom: "0.5rem",
          }}
        >
          <input
            type="checkbox"
            checked={detail.hasForeignContacts}
            onChange={(e) =>
              updateDetail(currentCountry, { hasForeignContacts: e.target.checked })
            }
          />
          Will you have foreign contacts?
        </label>
        {detail.hasForeignContacts && (
          <textarea
            value={detail.contactsDetail}
            onChange={(e) =>
              updateDetail(currentCountry, { contactsDetail: e.target.value })
            }
            placeholder="Describe your foreign contacts (names, organizations, etc.)"
            rows={3}
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
        )}
      </div>
    </div>
  );
}
