import { useState, useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { X } from "lucide-react";
import type { TenantUser } from "../../api/tasks";
import { listTenantUsers } from "../../api/tasks";

interface PeoplePickerProps {
  selected: TenantUser[];
  onChange: (users: TenantUser[]) => void;
}

export function PeoplePicker({ selected, onChange }: PeoplePickerProps) {
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQuery(query), 300);
    return () => clearTimeout(timer);
  }, [query]);

  const { data: allResults = [] } = useQuery({
    queryKey: ["tenant-users", debouncedQuery],
    queryFn: () => listTenantUsers(debouncedQuery),
    enabled: !!debouncedQuery.trim(),
  });
  const results = allResults.filter(u => !selected.some(s => s.user_id === u.user_id));

  function add(user: TenantUser) {
    onChange([...selected, user]);
    setQuery("");
  }

  function remove(userId: string) {
    onChange(selected.filter(u => u.user_id !== userId));
  }

  return (
    <div>
      <div className="people-picker-chips">
        {selected.map(u => (
          <span key={u.user_id} className="people-chip">
            {u.name}
            <button type="button" className="people-chip-remove" onClick={() => remove(u.user_id)}>
              <X size={10} />
            </button>
          </span>
        ))}
      </div>
      <div style={{ position: "relative", marginTop: 8 }}>
        <input
          className="filter-input"
          style={{ width: "100%" }}
          placeholder="Add people..."
          value={query}
          onChange={e => { setQuery(e.target.value); setOpen(true); }}
          onFocus={() => setOpen(true)}
          onBlur={() => setTimeout(() => setOpen(false), 200)}
        />
        {open && results.length > 0 && (
          <div style={{
            position: "absolute", top: "100%", left: 0, right: 0, zIndex: 10,
            background: "#fff", border: "1px solid #E8ECF0", borderRadius: 6,
            boxShadow: "0 4px 12px rgba(0,0,0,0.08)", maxHeight: 200, overflowY: "auto",
          }}>
            {results.map(u => (
              <div key={u.user_id} style={{
                padding: "8px 12px", cursor: "pointer", fontSize: 14,
              }} onMouseDown={() => add(u)}>
                <strong>{u.name}</strong> <span style={{ color: "#6B8294" }}>{u.email}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
