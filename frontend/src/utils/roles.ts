const adminRoles = ["administrator", "fso", "read_only_fso"];
const icRoles = ["individual_contributor"];

// Note on naming: isAdmin is broader than just the "administrator" role — it
// returns true for any role that has admin-panel access (administrator, fso,
// read_only_fso). When you need to gate something on the strict
// administrator role specifically (e.g. tenant-wide task assignment that
// FSOs aren't allowed to do), use isAdministrator below.
export function isAdmin(role?: string): boolean {
  return !!role && adminRoles.includes(role);
}

export function isAdministrator(role?: string): boolean {
  return role === "administrator";
}

export function isIC(role?: string): boolean {
  return !!role && icRoles.includes(role);
}

export function isReadOnlyFSO(role?: string): boolean {
  return role === "read_only_fso";
}
