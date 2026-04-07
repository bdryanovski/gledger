import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";
import { formatCurrency, amountColor } from "../utils/format";

const TYPE_ORDER = ["assets", "liabilities", "equity", "income", "expenses", "other"];
const TYPE_LABELS: Record<string, string> = {
  assets: "Assets", liabilities: "Liabilities", equity: "Equity",
  income: "Income", expenses: "Expenses", other: "Other",
};

export default function Accounts() {
  const { data, isLoading } = useQuery({
    queryKey: ["accounts"],
    queryFn: api.getAccounts,
  });

  if (isLoading) {
    return <div className="p-6 text-gray-400">Loading…</div>;
  }

  const grouped: Record<string, typeof data.accounts> = {};
  for (const acct of data?.accounts ?? []) {
    const t = acct.type || "other";
    (grouped[t] ??= []).push(acct);
  }

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-2xl font-bold text-gray-900">Accounts</h1>

      {TYPE_ORDER.map((type) => {
        const list = grouped[type];
        if (!list?.length) return null;

        const total = list.reduce((s, a) => s + a.balance, 0);

        return (
          <div key={type} className="bg-white rounded-xl border border-gray-100 shadow-sm overflow-hidden">
            <div className="px-5 py-3 border-b border-gray-100 flex justify-between items-center">
              <h2 className="font-semibold text-gray-800">{TYPE_LABELS[type]}</h2>
              <span className={`text-sm font-mono font-semibold ${amountColor(total)}`}>
                {formatCurrency(total)}
              </span>
            </div>
            <table className="w-full text-sm">
              <tbody className="divide-y divide-gray-50">
                {list.map((a) => (
                  <tr key={a.name} className="hover:bg-gray-50">
                    <td className="px-5 py-3 text-blue-700 font-mono text-xs">
                      {a.name}
                    </td>
                    <td className={`px-5 py-3 text-right font-mono text-sm font-semibold ${amountColor(a.balance)}`}>
                      {formatCurrency(a.balance, a.currency)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        );
      })}
    </div>
  );
}
