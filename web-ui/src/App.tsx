import { BrowserRouter, Routes, Route, NavLink } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import Dashboard from "./pages/Dashboard";
import Transactions from "./pages/Transactions";
import Accounts from "./pages/Accounts";
import Reports from "./pages/Reports";
import FQL from "./pages/FQL";
import Import from "./pages/Import";

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, staleTime: 30_000 } },
});

const navClass = ({ isActive }: { isActive: boolean }) =>
  `block px-3 py-2 rounded-md text-sm font-medium transition-colors ${
    isActive
      ? "bg-purple-600 text-white"
      : "text-gray-300 hover:bg-gray-700 hover:text-white"
  }`;

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <div className="flex h-screen bg-gray-50 overflow-hidden">
          {/* Sidebar */}
          <aside className="w-56 bg-gray-900 text-gray-100 flex flex-col shrink-0">
            <div className="p-4 border-b border-gray-700">
              <span className="text-xl font-bold text-purple-400">DoubleBook</span>
              <span className="block text-xs text-gray-500 mt-0.5">
                Plain-text accounting
              </span>
            </div>
            <nav className="flex-1 px-2 py-4 space-y-1">
              <NavLink to="/" end className={navClass}>
                📊 Dashboard
              </NavLink>
              <NavLink to="/transactions" className={navClass}>
                📋 Transactions
              </NavLink>
              <NavLink to="/accounts" className={navClass}>
                💼 Accounts
              </NavLink>
              <NavLink to="/reports" className={navClass}>
                📈 Reports
              </NavLink>
              <NavLink to="/fql" className={navClass}>
                🔍 FQL Query
              </NavLink>
              <NavLink to="/import" className={navClass}>
                📥 Import
              </NavLink>
            </nav>
            <div className="p-3 border-t border-gray-700 text-xs text-gray-500">
              v0.1.0
            </div>
          </aside>

          {/* Main */}
          <main className="flex-1 overflow-auto">
            <Routes>
              <Route path="/" element={<Dashboard />} />
              <Route path="/transactions" element={<Transactions />} />
              <Route path="/accounts" element={<Accounts />} />
              <Route path="/reports" element={<Reports />} />
              <Route path="/fql" element={<FQL />} />
              <Route path="/import" element={<Import />} />
            </Routes>
          </main>
        </div>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
