import { useQuery } from "@tanstack/react-query";
import { useAuthStore } from "../../stores/authStore";
import { listSubOrgs } from "../../api/customerAdmin";

interface SubOrgFilterProps {
  value: string;
  onChange: (value: string) => void;
}

/**
 * Renders a sub-org filter dropdown for administrator-role users only.
 * FSO/read_only_fso users are scoped on the backend and don't need this filter.
 */
export function SubOrgFilter({ value, onChange }: SubOrgFilterProps) {
  const role = useAuthStore((s) => s.user?.role);

  const { data } = useQuery({
    queryKey: ["sub-orgs"],
    queryFn: listSubOrgs,
    enabled: role === "administrator",
  });
  const subOrgs = (data?.sub_orgs ?? []).filter((o) => o.name !== "Default");

  if (role !== "administrator" || subOrgs.length === 0) {
    return null;
  }

  return (
    <select
      className="filter-select"
      value={value}
      onChange={(e) => onChange(e.target.value)}
    >
      <option value="">All Sub-Orgs</option>
      {subOrgs.map((org) => (
        <option key={org.id} value={org.id}>
          {org.name}
        </option>
      ))}
    </select>
  );
}
