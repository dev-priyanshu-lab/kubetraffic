import { NavLink, Outlet } from "react-router-dom";

const navItems = [
  { to: "/", label: "Routes", end: true },
  { to: "/audit", label: "Audit Log" },
  { to: "/playground", label: "Playground" },
];

export function Layout() {
  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      <header className="border-b border-slate-800">
        <div className="mx-auto flex max-w-6xl items-center gap-8 px-4 py-4">
          <span className="text-lg font-semibold tracking-tight">
            Kube<span className="text-sky-400">Traffic</span>
          </span>
          <nav className="flex gap-1">
            {navItems.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.end}
                className={({ isActive }) =>
                  `rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
                    isActive ? "bg-slate-800 text-white" : "text-slate-400 hover:text-slate-200"
                  }`
                }
              >
                {item.label}
              </NavLink>
            ))}
          </nav>
        </div>
      </header>
      <main className="mx-auto max-w-6xl px-4 py-8">
        <Outlet />
      </main>
    </div>
  );
}
