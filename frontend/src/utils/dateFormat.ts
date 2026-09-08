// Date display helpers. Use formatDate for things tracked at day granularity
// (travel, reports, due dates); use formatDateTime only where the time of day
// is meaningful (e.g. admin activity/audit logs).

export function formatDate(value: string | Date | null | undefined): string {
  if (!value) return "—";
  return new Date(value).toLocaleDateString();
}

export function formatDateTime(value: string | Date | null | undefined): string {
  if (!value) return "—";
  return new Date(value).toLocaleString();
}
