import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";
import { formatCurrency, amountColor, truncate } from "../utils/format";

const PAGE_SIZE = 25;

export default function Transactions() {
  const [search, setSearch] = useState("");
  const [begin, setBegin] = useState("");
  const [end, setEnd] = useState("");
  const [page, setPage] = useState(0);

  const params: Record<string, string> = {
    limit: String(PAGE_SIZE),
    offset: String(page * PAGE_SIZE),
  };
  if (search) params.desc = search;
  if (begin) params.begin = begin;
  if (end) params.end = end;

  const { data, isLoading } = useQuery({
    queryKey: ["transactions", params],
    queryFn: () => api.getTransactions(params),
  });

  const total = data?.total ?? 0;
  const totalPages = Math.ceil(total / PAGE_SIZE);

  return (
    <div className="p-6 space-y-4">
      <h1 className="text-2xl font-bold text-gray-900">Transactions</h1>

      {/* Filters */}
      <div className="bg-white rounded-xl border border-gray-100 shadow-sm p-4 flex flex-wrap gap-3">
        <input
          type="text"
          placeholder="Search description…"
          value={search}
          onChange={(e) => { setSearch(e.target.value); setPage(0); }}
          className="border border-gray-200 rounded-lg px-3 py-1.5 text-sm flex-1 min-w-40 focus:outline-none focus:ring-2 focus:ring-purple-400"
        />
        <label className="flex items-center gap-1.5 text-sm text-gray-600">
          From
          <input type="date" value={begin} onChange={(e) => { setBegin(e.target.value); setPage(0); }}
            className="border border-gray-200 rounded-lg px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-purple-400" />
        </label>
        <label className="flex items-center gap-1.5 text-sm text-gray-600">
          To
          <input type="date" value={end} onChange={(e) => { setEnd(e.target.value); setPage(0); }}
            className="border border-gray-200 rounded-lg px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-purple-400" />
        </label>
        <button onClick={() => { setSearch(""); setBegin(""); setEnd(""); setPage(0); }}
          className="text-sm text-gray-400 hover:text-gray-700 px-2">
          Clear
        </button>
      </div>

      {/* Table */}
      <div className="bg-white rounded-xl border border-gray-100 shadow-sm overflow-hidden">
        {isLoading ? (
          <div className="p-8 text-center text-gray-400">Loading…</div>
        ) : (
          <>
            <table className="w-full text-sm">
              <thead className="bg-gray-50 border-b border-gray-100">
                <tr>
                  {["Date", "Description", "Account", "Amount"].map((h) => (
                    <th key={h} className="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-50">
                {data?.transactions.length === 0 && (
                  <tr><td colSpan={4} className="px-4 py-8 text-center text-gray-400">No transactions found.</td></tr>
                )}
                {data?.transactions.flatMap((txn) =>
                  txn.postings.map((p, i) => (
                    <tr key={`${txn.id}-${i}`} className="hover:bg-purple-50/30">
                      <td className="px-4 py-2.5 text-gray-400 font-mono text-xs whitespace-nowrap">
                        {i === 0 ? txn.date : ""}
                      </td>
                      <td className="px-4 py-2.5 text-gray-800 max-w-xs">
                        {i === 0 ? truncate(txn.description, 40) : ""}
                      </td>
                      <td className="px-4 py-2.5 text-blue-600 font-mono text-xs">
                        {p.account}
                      </td>
                      <td className={`px-4 py-2.5 text-right font-mono text-xs font-semibold ${amountColor(p.amount)}`}>
                        {formatCurrency(p.amount, p.currency)}
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>

            {/* Pagination */}
            {totalPages > 1 && (
              <div className="flex items-center justify-between px-4 py-3 border-t border-gray-100 text-sm text-gray-500">
                <span>Showing {page * PAGE_SIZE + 1}–{Math.min((page + 1) * PAGE_SIZE, total)} of {total}</span>
                <div className="flex gap-2">
                  <button disabled={page === 0} onClick={() => setPage(p => p - 1)}
                    className="px-3 py-1 rounded border border-gray-200 disabled:opacity-40 hover:bg-gray-50">
                    ←
                  </button>
                  <button disabled={page >= totalPages - 1} onClick={() => setPage(p => p + 1)}
                    className="px-3 py-1 rounded border border-gray-200 disabled:opacity-40 hover:bg-gray-50">
                    →
                  </button>
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
