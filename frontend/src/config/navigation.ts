import {
  LayoutDashboard,
  ListTodo,
  Inbox,
  FileText,
  Plane,
  Users,
  UserCog,
  FileBadge,
  BookOpen,
  ShieldCheck,
  Settings,
  MessageSquare,
  ClipboardList,
  type LucideIcon,
} from "lucide-react";

export interface NavItem {
  label: string;
  path: string;
  icon: LucideIcon;
  roles?: string[];
}

export const navItems: NavItem[] = [
  { label: "Dashboard", path: "/app/dashboard", icon: LayoutDashboard },
  {
    label: "Manage Tasks",
    path: "/app/tasks",
    icon: ListTodo,
    roles: ["administrator", "fso"],
  },
  { label: "My Action Items", path: "/app/tasks", icon: Inbox, roles: ["individual_contributor", "read_only_fso"] },
  {
    label: "Action Items",
    path: "/app/action-items",
    icon: Inbox,
    roles: ["administrator", "fso"],
  },
  { label: "Reports", path: "/app/reports", icon: FileText },
  { label: "Travel", path: "/app/travel", icon: Plane },
  { label: "Visits", path: "/app/visits", icon: Users },
  {
    label: "Team",
    path: "/app/team",
    icon: UserCog,
    roles: ["administrator", "fso", "read_only_fso"],
  },
  {
    label: "DD254s",
    path: "/app/dd254",
    icon: FileBadge,
    roles: ["administrator", "fso", "read_only_fso"],
  },
  { label: "Wiki", path: "/app/wiki", icon: BookOpen },
  { label: "FSO Assistant", path: "/app/chat", icon: MessageSquare },
  {
    label: "Audit Log",
    path: "/app/audit-log",
    icon: ClipboardList,
    roles: ["administrator", "fso"],
  },
  {
    label: "Admin",
    path: "/app/admin",
    icon: ShieldCheck,
    roles: ["administrator"],
  },
];

export const settingsNavItem: NavItem = {
  label: "Settings",
  path: "/app/settings",
  icon: Settings,
};
