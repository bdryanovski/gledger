/**
 * AppConfig — React context that fetches /api/config on startup and makes
 * credit_normal_prefixes (and other runtime config) available throughout
 * the app without any hardcoding.
 */
import { createContext, useContext, ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "./client";

interface AppConfigData {
  currency: string;
  credit_normal_prefixes: string[];
  date_format: string;
}

// Sensible defaults while the API call is in-flight (matches server defaults).
const DEFAULTS: AppConfigData = {
  currency: "USD",
  credit_normal_prefixes: ["income", "liabilities", "equity", "revenue", "revenues"],
  date_format: "2006-01-02",
};

const AppConfigContext = createContext<AppConfigData>(DEFAULTS);

export function AppConfigProvider({ children }: { children: ReactNode }) {
  const { data } = useQuery({
    queryKey: ["app-config"],
    queryFn: api.getAppConfig,
    staleTime: Infinity, // config doesn't change while server is running
  });

  return (
    <AppConfigContext.Provider value={data ?? DEFAULTS}>
      {children}
    </AppConfigContext.Provider>
  );
}

/** Hook to access the server-provided app config. */
export function useAppConfig(): AppConfigData {
  return useContext(AppConfigContext);
}
