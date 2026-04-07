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

/** Return "green" or "red" based on whether amount is positive or negative. */
export function amountColor(amount: number): string {
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
