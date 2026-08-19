import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { fetchUserSettings } from "@/api/settings";
import { useAuth } from "@/contexts/AuthContext";
import {
  DEFAULT_LANGUAGE,
  detectBrowserLanguage,
  isSupportedLanguage,
  setCurrentLanguage,
  translate,
  type Language,
} from "@/i18n";

type I18nContextType = {
  language: Language;
  setLanguage: (language: Language) => void;
  t: (key: string, params?: Record<string, string | number>) => string;
};

const I18nContext = createContext<I18nContextType | undefined>(undefined);

export function I18nProvider({ children }: { children: ReactNode }) {
  const { token } = useAuth();
  const { data: settings } = useQuery({
    queryKey: ["settings"],
    queryFn: fetchUserSettings,
    enabled: typeof token === "string",
  });
  const [language, setLanguage] = useState<Language>(() => detectBrowserLanguage());
  const [isUserSelect, setIsUserSelect] = useState(false);

  // Adopt the persisted setting once an account exists (but never overwrite a
  // language the user just picked in this session).
  useEffect(() => {
    if (!isUserSelect && settings?.data?.language && isSupportedLanguage(settings.data.language)) {
      setLanguage(settings.data.language as Language);
    }
  }, [settings?.data?.language, isUserSelect]);

  const handleSetLanguage = useCallback((next: Language) => {
    setIsUserSelect(true);
    setLanguage(next);
  }, []);

  useEffect(() => {
    document.documentElement.lang = language;
    setCurrentLanguage(language);
  }, [language]);

  const t = useCallback(
    (key: string, params?: Record<string, string | number>) => translate(language, key, params),
    [language],
  );

  const value = useMemo<I18nContextType>(() => ({
    language,
    setLanguage: handleSetLanguage,
    t,
  }), [language, handleSetLanguage, t]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nContextType {
  const context = useContext(I18nContext);
  if (context === undefined) {
    throw new Error("useI18n must be used within an I18nProvider");
  }
  return context;
}

export { DEFAULT_LANGUAGE };