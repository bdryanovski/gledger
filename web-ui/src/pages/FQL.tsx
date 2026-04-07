import { useState, useRef } from "react";
import { api } from "../api/client";
import type { FQLResult } from "../api/types";

const EXAMPLE_QUERIES = [
  "SELECT account, total_amount FROM accounts ORDER BY total_amount DESC",
  "SELECT date, description, amount FROM transactions WHERE amount < -100 ORDER BY date DESC LIMIT 20",
  "SELECT month, SUM(total_amount) AS monthly FROM spending GROUP BY month ORDER BY month",
  "SELECT account, COUNT(*) AS cnt, AVG(amount) AS avg FROM transactions GROUP BY account ORDER BY cnt DESC LIMIT 10",
];

export default function FQL() {
  const [query, setQuery] = useState(EXAMPLE_QUERIES[0]);
  const [result, setResult] = useState<FQLResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [elapsed, setElapsed] = useState(0);
  const [history, setHistory] = useState<string[]>([]);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const run = async () => {
    if (!query.trim()) return;
    setLoading(true);
    setError(null);
    const t0 = Date.now();
    try {
      const r = await api.runFQL(query.trim());
      setResult(r);
      setElapsed(Date.now() - t0);
      setHistory((h) => [query, ...h.filter((q) => q !== query)].slice(0, 20));
    } catch (e) {
      setError(String(e instanceof Error ? e.message : e));
      setResult(null);
    } finally {
      setLoading(false);
    }
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if ((e.ctrlKey || e.metaKey) && e.key === "Enter") {
      e.preventDefault();
      run();
    }
  };

  return (
    <div className="p-6 space-y-4">
      <h1 className="text-2xl font-bold text-gray-900">FQL Query</h1>

      {/* Editor */}
      <div className="bg-white rounded-xl border border-gray-100 shadow-sm p-4 space-y-3">
        <textarea
          ref={textareaRef}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={onKeyDown}
          rows={4}
          className="w-full font-mono text-sm border border-gray-200 rounded-lg p-3 focus:outline-none focus:ring-2 focus:ring-purple-400 resize-none"
          placeholder="SELECT account, total_amount FROM accounts ORDER BY total_amount DESC"
        />
        <div className="flex items-center justify-between">
          <p className="text-xs text-gray-400">
            Tables: <code className="bg-gray-100 px-1 rounded">transactions</code>{" "}
            <code className="bg-gray-100 px-1 rounded">accounts</code>{" "}
            <code className="bg-gray-100 px-1 rounded">spending</code>
            {" · "}Ctrl+Enter to run
          </p>
          <div className="flex gap-2">
            <button onClick={() => { setQuery(""); setResult(null); setError(null); }}
              className="px-3 py-1.5 text-sm text-gray-500 hover:text-gray-700 border border-gray-200 rounded-lg">
              Clear
            </button>
            <button onClick={run} disabled={loading}
              className="px-4 py-1.5 text-sm bg-purple-600 text-white rounded-lg hover:bg-purple-700 disabled:opacity-50">
              {loading ? "Running…" : "Run Query"}
            </button>
          </div>
        </div>
      </div>

      {/* Examples */}
      <details className="text-sm">
        <summary className="cursor-pointer text-gray-500 hover:text-gray-700 select-none">
          Example queries
        </summary>
        <div className="mt-2 space-y-1 pl-2">
          {EXAMPLE_QUERIES.map((q, i) => (
            <button key={i} onClick={() => setQuery(q)}
              className="block text-left text-blue-600 hover:underline text-xs font-mono truncate w-full">
              {q}
            </button>
          ))}
        </div>
      </details>

      {/* History */}
      {history.length > 0 && (
        <details className="text-sm">
          <summary className="cursor-pointer text-gray-500 hover:text-gray-700 select-none">
            Query history ({history.length})
          </summary>
          <div className="mt-2 space-y-1 pl-2">
            {history.map((q, i) => (
              <button key={i} onClick={() => setQuery(q)}
                className="block text-left text-blue-600 hover:underline text-xs font-mono truncate w-full">
                {q}
              </button>
            ))}
          </div>
        </details>
      )}

      {/* Error */}
      {error && (
        <div className="bg-red-50 border border-red-200 rounded-xl p-4 text-red-700 text-sm">
          <strong>Error:</strong> {error}
        </div>
      )}

      {/* Results */}
      {result && (
        <div className="bg-white rounded-xl border border-gray-100 shadow-sm overflow-hidden">
          <div className="px-4 py-3 border-b border-gray-100 flex justify-between text-xs text-gray-500">
            <span>{result.row_count} row{result.row_count !== 1 ? "s" : ""}</span>
            <span>{elapsed}ms</span>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead className="bg-gray-50">
                <tr>
                  {result.columns.map((col) => (
                    <th key={col} className="px-4 py-2.5 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">
                      {col}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-50">
                {result.rows.map((row, ri) => (
                  <tr key={ri} className="hover:bg-gray-50">
                    {(row as unknown[]).map((cell, ci) => (
                      <td key={ci} className="px-4 py-2 font-mono text-gray-700">
                        {cell === null ? <span className="text-gray-300">NULL</span> : String(cell)}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
