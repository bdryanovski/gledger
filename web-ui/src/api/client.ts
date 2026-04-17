import type {
  Transaction,
  Account,
  FQLResult,
  IncomeStatement,
  BalanceReport,
} from "./types";

const BASE_URL = "/api";

async function fetchJSON<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error((err as { error: string }).error || res.statusText);
  }
  return res.json() as Promise<T>;
}

export const api = {
  getTransactions: (params?: Record<string, string>) =>
    fetchJSON<{ transactions: Transaction[]; total: number }>(
      "/transactions" + (params ? "?" + new URLSearchParams(params) : "")
    ),

  getAccounts: (params?: Record<string, string>) =>
    fetchJSON<{ accounts: Account[] }>(
      "/accounts" + (params ? "?" + new URLSearchParams(params) : "")
    ),

  getBalance: (params?: Record<string, string>) =>
    fetchJSON<BalanceReport>(
      "/reports/balance" + (params ? "?" + new URLSearchParams(params) : "")
    ),

  getIncomeStatement: (params?: Record<string, string>) =>
    fetchJSON<IncomeStatement>(
      "/reports/income-statement" +
        (params ? "?" + new URLSearchParams(params) : "")
    ),

  runFQL: (query: string) =>
    fetchJSON<FQLResult>("/fql", {
      method: "POST",
      body: JSON.stringify({ query }),
    }),

  getHealth: () => fetchJSON<{ status: string; version: string }>("/health"),

  getAppConfig: () =>
    fetchJSON<{
      currency: string;
      credit_normal_prefixes: string[];
      date_format: string;
    }>("/config"),
};
