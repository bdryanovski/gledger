/**
 * Format a monetary amount for display.
 * Uses currency symbol for known currencies, trailing code for others.
 */
export function formatCurrency(amount: number, currency = "USD"): string {
  const neg = amount < 0 ? "-" : "";
  const abs = Math.abs(amount);
  const formatted = abs.toLocaleString("en-US", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
  const symbols: Record<string, string> = { USD: "$", GBP: "£", EUR: "€" };
  const sym = symbols[currency.toUpperCase()];
  if (sym) return `${neg}${sym}${formatted}`;
  return `${neg}${formatted} ${currency}`;
}

/**
 * Return whether `account` is credit-normal according to `creditPrefixes`.
 * Credit-normal accounts (income, liabilities, equity by default) are GREEN
 * when their balance is NEGATIVE.
 *
 * The prefix list comes from /api/config so users can extend it in
 * ~/.doublebook/config.yaml without rebuilding the frontend.
 */
export function isAccountCreditNormal(
  account: string,
  creditPrefixes: string[]
): boolean {
  const lower = account.toLowerCase().trim();
  return creditPrefixes.some((prefix) => {
    const p = prefix.toLowerCase().trim();
    return lower === p || lower.startsWith(p + ":") || lower.startsWith(p + "/");
  });
}

/**
 * Return a Tailwind color class for an amount, optionally account-type-aware.
 *
 * @param amount          - the numeric value
 * @param account         - optional account name for type-aware coloring
 * @param creditPrefixes  - from /api/config credit_normal_prefixes
 */
export function amountColor(
  amount: number,
  account?: string,
  creditPrefixes?: string[]
): string {
  if (account && creditPrefixes) {
    const creditNormal = isAccountCreditNormal(account, creditPrefixes);
    const healthy = creditNormal ? amount <= 0 : amount >= 0;
    return healthy ? "text-green-600" : "text-red-600";
  }
  // Fallback when no account context (e.g. running totals in register).
  return amount >= 0 ? "text-green-600" : "text-red-600";
}

/** Truncate a string to maxLen chars, appending … if needed. */
export function truncate(s: string, maxLen: number): string {
  if (s.length <= maxLen) return s;
  return s.slice(0, maxLen - 1) + "…";
}

/** Return today's date as YYYY-MM-DD. */
export function today(): string {
  return new Date().toISOString().slice(0, 10);
}

/** Return the first day of the current month as YYYY-MM-DD. */
export function firstDayOfMonth(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-01`;
}
