import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";
import { formatCurrency, amountColor, firstDayOfMonth, today } from "../utils/format";

type Tab = "income" | "balance";

export default function Reports() {
  const [tab, setTab] = useState<Tab>("income");
  const [begin, setBegin] = useState(firstDayOfMonth());
  const [end, setEnd] = useState(today());

  const { data: is } = useQuery({
    queryKey: ["income-statement", begin, end],
    queryFn: () => api.getIncomeStatement({ begin, end }),
    enabled: !!begin && !!end,
  });

  const { data: bal } = useQuery({
    queryKey: ["balance", begin, end],
    queryFn: () => api.getBalance({ begin, end }),
    enabled: tab === "balance",
  });

  const totalRevenue = Object.values(is?.revenues ?? {}).reduce((s, a) => s + a.value, 0);
  const totalExpenses = Object.values(is?.expenses ?? {}).reduce((s, a) => s + a.value, 0);

  return (
    <div className="p-6 space-y-5 print:p-0">
      <div className="flex items-center justify-between print:hidden">
        <h1 className="text-2xl font-bold text-gray-900">Reports</h1>
        <button onClick={() => window.print()}
          className="px-4 py-2 bg-purple-600 text-white rounded-lg text-sm hover:bg-purple-700">
          🖨 Print
        </button>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-gray-200 print:hidden">
        {(["income", "balance"] as Tab[]).map((t) => (
          <button key={t} onClick={() => setTab(t)}
            className={`px-5 py-2.5 text-sm font-medium border-b-2 -mb-px transition-colors ${
              tab === t ? "border-purple-600 text-purple-600" : "border-transparent text-gray-500 hover:text-gray-700"
            }`}>
            {t === "income" ? "Income Statement" : "Balance Sheet"}
          </button>
        ))}
      </div>

      {/* Date range */}
      <div className="flex gap-3 bg-white border border-gray-100 rounded-xl p-3 shadow-sm print:hidden">
        <label className="flex items-center gap-1.5 text-sm text-gray-600">
          From <input type="date" value={begin} onChange={(e) => setBegin(e.target.value)}
            className="border border-gray-200 rounded px-2 py-1 text-sm focus:outline-none focus:ring-2 focus:ring-purple-400" />
        </label>
        <label className="flex items-center gap-1.5 text-sm text-gray-600">
          To <input type="date" value={end} onChange={(e) => setEnd(e.target.value)}
            className="border border-gray-200 rounded px-2 py-1 text-sm focus:outline-none focus:ring-2 focus:ring-purple-400" />
        </label>
      </div>

      {tab === "income" && is && (
        <div className="bg-white rounded-xl border border-gray-100 shadow-sm print:shadow-none print:border-none">
          <div className="px-6 py-4 border-b border-gray-100">
            <h2 className="text-lg font-semibold">Income Statement</h2>
            <p className="text-sm text-gray-500">{begin} — {end}</p>
          </div>

          {/* Revenues */}
          <Section title="Revenues">
            {Object.entries(is.revenues).sort((a, b) => b[1].value - a[1].value).map(([acc, a]) => (
              <Row key={acc} label={acc} value={a.value} currency={a.currency} />
            ))}
            <TotalRow label="Total Revenues" value={totalRevenue} />
          </Section>

          {/* Expenses */}
          <Section title="Expenses">
            {Object.entries(is.expenses).sort((a, b) => b[1].value - a[1].value).map(([acc, a]) => (
              <Row key={acc} label={acc} value={a.value} currency={a.currency} />
            ))}
            <TotalRow label="Total Expenses" value={totalExpenses} />
          </Section>

          {/* Net */}
          <div className="px-6 py-4 bg-gray-50 flex justify-between items-center font-bold text-base">
            <span>Net Income</span>
            <span className={amountColor(is.net_income.value)}>
              {formatCurrency(is.net_income.value, is.net_income.currency)}
            </span>
          </div>
        </div>
      )}

      {tab === "balance" && bal && (
        <div className="bg-white rounded-xl border border-gray-100 shadow-sm">
          <div className="px-6 py-4 border-b border-gray-100">
            <h2 className="text-lg font-semibold">Balance Sheet</h2>
          </div>
          {(["assets","liabilities","equity","income","expenses"] as const).map((type) => {
            const entries = (bal as Record<string, { account: string; amount: number; currency: string }[]>)[type];
            if (!entries?.length) return null;
            return (
              <Section key={type} title={type.charAt(0).toUpperCase() + type.slice(1)}>
                {entries.map((e) => <Row key={e.account} label={e.account} value={e.amount} currency={e.currency} />)}
              </Section>
            );
          })}
        </div>
      )}
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <div className="px-6 py-2 bg-gray-50 border-y border-gray-100">
        <span className="text-xs font-semibold text-gray-500 uppercase tracking-wide">{title}</span>
      </div>
      {children}
    </div>
  );
}

function Row({ label, value, currency = "USD" }: { label: string; value: number; currency?: string }) {
  return (
    <div className="px-6 py-2.5 flex justify-between items-center hover:bg-gray-50 text-sm">
      <span className="text-blue-700 font-mono text-xs">{label}</span>
      <span className={`font-mono text-xs font-semibold ${amountColor(value)}`}>
        {formatCurrency(value, currency)}
      </span>
    </div>
  );
}

function TotalRow({ label, value }: { label: string; value: number }) {
  return (
    <div className="px-6 py-3 flex justify-between items-center border-t border-gray-200 font-semibold text-sm">
      <span className="text-gray-700">{label}</span>
      <span className={`font-mono ${amountColor(value)}`}>{formatCurrency(value)}</span>
    </div>
  );
}
