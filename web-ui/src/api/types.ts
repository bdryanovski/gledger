export interface Transaction {
  id: string;
  date: string;
  description: string;
  status: string;
  postings: Posting[];
  tags: Record<string, string>;
}

export interface Posting {
  account: string;
  amount: number;
  currency: string;
}

export interface Account {
  name: string;
  type: string;
  balance: number;
  currency: string;
}

export interface FQLResult {
  columns: string[];
  rows: unknown[][];
  row_count: number;
}

export interface IncomeStatement {
  revenues: Record<string, { value: number; currency: string }>;
  expenses: Record<string, { value: number; currency: string }>;
  net_income: { value: number; currency: string };
}

export interface BalanceReport {
  assets?: BalanceEntry[];
  liabilities?: BalanceEntry[];
  equity?: BalanceEntry[];
  income?: BalanceEntry[];
  expenses?: BalanceEntry[];
}

export interface BalanceEntry {
  account: string;
  amount: number;
  currency: string;
}
