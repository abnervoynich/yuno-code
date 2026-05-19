import { NavLink, Outlet } from "react-router-dom";
import { BarChart2, FileText, RefreshCw, Home, TrendingUp } from "lucide-react";

const navItems = [
  { to: "/dashboard", label: "Dashboard", icon: Home },
  { to: "/transactions", label: "Transactions", icon: FileText },
  { to: "/settlements", label: "Settlements", icon: BarChart2 },
  { to: "/reconciliation", label: "Reconciliation", icon: RefreshCw },
];

export function Layout() {
  return (
    <div className="flex h-screen bg-slate-50">
      {/* Sidebar */}
      <aside className="w-56 bg-slate-900 text-white flex flex-col shrink-0">
        <div className="p-5 border-b border-slate-700">
          <div className="flex items-center gap-2 mb-0.5">
            <TrendingUp size={20} className="text-blue-400" />
            <h1 className="text-lg font-bold text-white">LuxeCart</h1>
          </div>
          <p className="text-xs text-slate-400">Settlement Engine</p>
        </div>
        <nav className="flex-1 p-3">
          {navItems.map(({ to, label, icon: Icon }) => (
            <NavLink
              key={to}
              to={to}
              className={({ isActive }) =>
                `flex items-center gap-3 px-3 py-2.5 rounded-lg mb-1 text-sm font-medium transition-colors ${
                  isActive
                    ? "bg-blue-600 text-white"
                    : "text-slate-400 hover:bg-slate-800 hover:text-white"
                }`
              }
            >
              <Icon size={16} />
              {label}
            </NavLink>
          ))}
        </nav>
        <div className="p-4 border-t border-slate-700">
          <p className="text-xs text-slate-500">Abner Garcia 2026</p>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto">
        <Outlet />
      </main>
    </div>
  );
}
