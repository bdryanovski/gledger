import { useQuery } from "@tanstack/react-query";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
} from "recharts";
import { api } from "../api/client";
import { formatCurrency, amountColor, firstDayOfMonth, today } from "../utils/format";

function StatCard({
  title,
  value,
  sub,
  color = "text-gray-900",
}: {
  title: string;
  value: string;
  sub?: string;
  color?: string;
}) {
  return (
    <div className="bg-white rounded-xl shadow-sm border border-gray-100 p-5">
      <p className="text-sm text-gray-500 mb-1">{title}</p>
      <p className={`text-2xl font-bold ${color}`}>{value}</p>
      {sub && <p className="text-xs text-gray-400 mt-1">{sub}</p>}
    </div>
  );
}

export default function Dashboard() {
  const { data: accounts } = useQuery({
    queryKey: ["accounts"],
    queryFn: api.getAccounts,
  });

  const { data: recent } = useQuery({
    queryKey: ["transactions-recent"],
    queryFn: () => api.getTransactions({ limit: "10" }),
  });

  const { data: is } = useQuery({
    queryKey: ["income-statement-month"],
    queryFn: () =>
      api.getIncomeStatement({ begin: firstDayOfMonth(), end: today() }),
  });

  const netWorth =
    accounts?.accounts
      .filter((a) => a.type === "assets")
      .reduce((s, a) => s + a.balance, 0) ?? 0;

  const totalRevenue = Object.values(is?.revenues ?? {}).reduce(
    (s, a) => s + a.value,
    0
  );
  const totalExpenses = Object.values(is?.expenses ?? {}).reduce(
    (s, a) => s + a.value,
    0
  );

  const spendingData = Object.entries(is?.expenses ?? {})
    .map(([acc, a]) => ({
      name: acc.replace("expenses:", ""),
      amount: Math.abs(a.value),
    }))
    .sort((a, b) => b.amount - a.amount)
    .slice(0, 8);

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>

      {/* Summary cards */}
      <div className="grid grid-cols-3 gap-4">
        <StatCard
          title="Net Worth"
          value={formatCurrency(netWorth)}
          sub="total assets"
          color={amountColor(netWorth)}
        />
        <StatCard
          title="Monthly Revenue"
          value={formatCurrency(totalRevenue)}
          sub="this month"
          color="text-green-700"
        />
        <StatCard
          title="Monthly Expenses"
          value={formatCurrency(totalExpenses)}
          sub="this month"
          color="text-red-700"
        />
      </div>

      {/* Spending chart */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-100 p-5">
        <h2 className="text-lg font-semibold text-gray-800 mb-4">
          Spending by Category (this month)
        </h2>
        {spendingData.length === 0 ? (
          <p className="text-gray-400 text-sm">No expense data for this month.</p>
        ) : (
          <ResponsiveContainer width="100%" height={220}>
            <BarChart data={spendingData} layout="vertical">
              <XAxis type="number" tick={{ fontSize: 11 }} />
              <YAxis
                type="category"
                dataKey="name"
                width={120}
                tick={{ fontSize: 11 }}
              />
              <Tooltip formatter={(v) => formatCurrency(Number(v))} />
              <Bar dataKey="amount" fill="#7C3AED" radius={[0, 4, 4, 0]} />
            </BarChart>
          </ResponsiveContainer>
        )}
      </div>

      {/* Recent transactions */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-100">
        <div className="px-5 py-4 border-b border-gray-100">
          <h2 className="text-lg font-semibold text-gray-800">
            Recent Transactions
          </h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="bg-gray-50">
              <tr>
                {["Date", "Description", "Account", "Amount"].map((h) => (
                  <th
                    key={h}
                    className="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide"
                  >
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {recent?.transactions.flatMap((txn) =>
                txn.postings.map((p, i) => (
                  <tr key={`${txn.id}-${i}`} className="hover:bg-gray-50">
                    <td className="px-4 py-2.5 text-gray-500 font-mono text-xs">
                      {i === 0 ? txn.date : ""}
                    </td>
                    <td className="px-4 py-2.5 text-gray-800">
                      {i === 0 ? txn.description : ""}
                    </td>
                    <td className="px-4 py-2.5 text-blue-600 font-mono text-xs">
                      {p.account}
                    </td>
                    <td
                      className={`px-4 py-2.5 text-right font-mono text-xs font-medium ${amountColor(p.amount)}`}
                    >
                      {formatCurrency(p.amount, p.currency)}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
