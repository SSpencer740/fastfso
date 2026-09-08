import type { ReactNode } from "react";

interface FilterOption {
  label: string;
  value: string;
}

interface FilterSelectProps {
  value: string;
  options: FilterOption[];
  onChange: (value: string) => void;
  placeholder?: string;
}

function FilterSelect({ value, options, onChange, placeholder }: FilterSelectProps) {
  return (
    <select
      className="filter-select"
      value={value}
      onChange={(e) => onChange(e.target.value)}
    >
      {placeholder && <option value="">{placeholder}</option>}
      {options.map((opt) => (
        <option key={opt.value} value={opt.value}>
          {opt.label}
        </option>
      ))}
    </select>
  );
}

interface FilterBarProps {
  children: ReactNode;
}

export function FilterBar({ children }: FilterBarProps) {
  return <div className="filter-bar">{children}</div>;
}

interface FilterSearchProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
}

function FilterSearch({ value, onChange, placeholder }: FilterSearchProps) {
  return (
    <input
      className="filter-input"
      type="text"
      value={value}
      onChange={(e) => onChange(e.target.value)}
      placeholder={placeholder ?? "Search..."}
    />
  );
}

FilterBar.Select = FilterSelect;
FilterBar.Search = FilterSearch;
