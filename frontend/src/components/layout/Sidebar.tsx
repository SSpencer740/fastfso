import { NavLink } from "react-router";
import { useAuthStore } from "../../stores/authStore";
import { navItems, settingsNavItem, type NavItem } from "../../config/navigation";

function SidebarLink({ item }: { item: NavItem }) {
  const Icon = item.icon;
  return (
    <NavLink
      to={item.path}
      className={({ isActive }) =>
        "sidebar-link" + (isActive ? " active" : "")
      }
    >
      <Icon size={18} />
      <span>{item.label}</span>
    </NavLink>
  );
}

export function Sidebar() {
  const role = useAuthStore((s) => s.user?.role);

  const visibleItems = navItems.filter(
    (item) => !item.roles || (role && item.roles.includes(role)),
  );

  return (
    <nav className="sidebar">
      <div className="sidebar-logo">
        <span className="sidebar-logo-fast">fast</span>
        <span className="sidebar-logo-fso">FSO</span>
      </div>
      <div className="sidebar-nav">
        {visibleItems.map((item) => (
          <SidebarLink key={item.path} item={item} />
        ))}
      </div>
      <div className="sidebar-footer">
        <SidebarLink item={settingsNavItem} />
      </div>
    </nav>
  );
}
