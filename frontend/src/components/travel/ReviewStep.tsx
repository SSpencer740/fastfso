import type { CountryDetail } from "./TravelDetailsStep";
import type { FileStatus } from "./ReportTravelWizard";
import { FileStatusBadge } from "./FileStatusBadge";

interface ReviewStepProps {
  tripName: string;
  multiCountry: boolean;
  countries: string[];
  passportNumber: string;
  countryDetails: Record<string, CountryDetail>;
  emergencyFirstName: string;
  emergencyLastName: string;
  emergencyPhone: string;
  additionalComments: string;
  files: File[];
  fileStatuses: FileStatus[];
}

function maskPassport(value: string): string {
  if (value.length <= 4) return value;
  return "*".repeat(value.length - 4) + value.slice(-4);
}

const sectionStyle: React.CSSProperties = {
  marginBottom: "1.5rem",
  padding: "1rem",
  border: "1px solid var(--color-border)",
  borderRadius: "var(--radius-lg)",
};

const labelStyle: React.CSSProperties = {
  fontSize: "0.75rem",
  color: "var(--color-text-muted)",
  textTransform: "uppercase",
  letterSpacing: "0.05em",
  marginBottom: "0.25rem",
};

const valueStyle: React.CSSProperties = {
  fontSize: "0.9rem",
  marginBottom: "0.75rem",
};

export function ReviewStep({
  tripName,
  multiCountry,
  countries,
  passportNumber,
  countryDetails,
  emergencyFirstName,
  emergencyLastName,
  emergencyPhone,
  additionalComments,
  files,
  fileStatuses,
}: ReviewStepProps) {
  return (
    <div>
      <p
        style={{
          fontSize: "0.85rem",
          color: "var(--color-text-muted)",
          marginTop: 0,
          marginBottom: "1rem",
        }}
      >
        Please review your travel report before submitting.
      </p>

      {/* Trip Info */}
      <div style={sectionStyle}>
        <h4 style={{ margin: "0 0 0.75rem", fontSize: "0.95rem" }}>Trip Information</h4>
        <div style={labelStyle}>Trip Name</div>
        <div style={valueStyle}>{tripName || "-"}</div>
        <div style={labelStyle}>Type</div>
        <div style={valueStyle}>{multiCountry ? "Multi-country" : "Single country"}</div>
        <div style={labelStyle}>{countries.length > 1 ? "Countries" : "Country"}</div>
        <div style={valueStyle}>{countries.filter(Boolean).join(", ") || "-"}</div>
        <div style={labelStyle}>Passport Number</div>
        <div style={valueStyle}>{passportNumber ? maskPassport(passportNumber) : "-"}</div>
      </div>

      {/* Country Details */}
      {countries.filter(Boolean).map((country) => {
        const detail = countryDetails[country];
        if (!detail) return null;
        return (
          <div key={country} style={sectionStyle}>
            <h4 style={{ margin: "0 0 0.75rem", fontSize: "0.95rem" }}>{country}</h4>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 1rem" }}>
              <div>
                <div style={labelStyle}>Start Date</div>
                <div style={valueStyle}>{detail.startDate || "-"}</div>
              </div>
              <div>
                <div style={labelStyle}>End Date</div>
                <div style={valueStyle}>{detail.endDate || "-"}</div>
              </div>
            </div>
            <div style={labelStyle}>Reason</div>
            <div style={valueStyle}>{detail.reason || "-"}</div>
            <div style={labelStyle}>Transportation</div>
            <div style={valueStyle}>
              {detail.transportation.length > 0 ? detail.transportation.join(", ") : "-"}
            </div>
            {detail.hasCompanions && (
              <>
                <div style={labelStyle}>Companions</div>
                <div style={valueStyle}>{detail.companionsDetail || "-"}</div>
              </>
            )}
            {detail.hasForeignContacts && (
              <>
                <div style={labelStyle}>Foreign Contacts</div>
                <div style={valueStyle}>{detail.contactsDetail || "-"}</div>
              </>
            )}
          </div>
        );
      })}

      {/* Emergency Contact */}
      <div style={sectionStyle}>
        <h4 style={{ margin: "0 0 0.75rem", fontSize: "0.95rem" }}>Emergency Contact</h4>
        <div style={labelStyle}>Name</div>
        <div style={valueStyle}>
          {emergencyFirstName || emergencyLastName
            ? `${emergencyFirstName} ${emergencyLastName}`.trim()
            : "-"}
        </div>
        <div style={labelStyle}>Phone</div>
        <div style={valueStyle}>{emergencyPhone || "-"}</div>
      </div>

      {/* Additional Comments */}
      {additionalComments && (
        <div style={sectionStyle}>
          <h4 style={{ margin: "0 0 0.75rem", fontSize: "0.95rem" }}>Additional Comments</h4>
          <div style={{ fontSize: "0.9rem", whiteSpace: "pre-wrap" }}>{additionalComments}</div>
        </div>
      )}

      {/* Files */}
      {files.length > 0 && (
        <div style={sectionStyle}>
          <h4 style={{ margin: "0 0 0.75rem", fontSize: "0.95rem" }}>
            Attached Files ({files.length})
          </h4>
          {files.map((file, i) => (
            <div
              key={`${file.name}-${i}`}
              style={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                gap: "0.5rem",
                fontSize: "0.85rem",
                padding: "0.3rem 0",
                color: "var(--color-text-secondary)",
              }}
            >
              <span style={{ flex: 1, minWidth: 0 }}>
                {file.name}{" "}
                <span style={{ color: "var(--color-text-muted)" }}>
                  ({(file.size / 1024).toFixed(1)} KB)
                </span>
              </span>
              <FileStatusBadge status={fileStatuses[i] ?? { kind: "idle" }} />
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
