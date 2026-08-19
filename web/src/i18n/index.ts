import en from "./en";
import es from "./es";
import fr from "./fr";
import de from "./de";
import type { MessageKey } from "./en";

export type { MessageKey } from "./en";

export type Language = "en" | "es" | "fr" | "de";

export const SUPPORTED_LANGUAGES: Language[] = ["en", "es", "fr", "de"];

const dictionaries: Record<Language, MessageKey> = { en, es, fr, de };

export const DEFAULT_LANGUAGE: Language = "en";

// Module-level current language, kept in sync by I18nProvider. Lets
// non-React code (e.g. toast helpers) pick the active language.
let currentLanguage: Language = DEFAULT_LANGUAGE;

export function setCurrentLanguage(language: Language): void {
  currentLanguage = language;
}

export function getCurrentLanguage(): Language {
  return currentLanguage;
}

export const isSupportedLanguage = (code: string): code is Language =>
  SUPPORTED_LANGUAGES.includes(code as Language);

/**
 * Maps a browser language tag (e.g. "es-ES", "fr") to an app language.
 * Falls back to the default when the browser language isn't supported.
 */
export function detectBrowserLanguage(languages?: readonly string[]): Language {
  const candidates = languages && languages.length > 0
    ? languages
    : (typeof navigator !== "undefined" ? navigator.languages : undefined) ?? [DEFAULT_LANGUAGE];

  for (const tag of candidates) {
    const base = tag.toLowerCase().split("-")[0];
    if (isSupportedLanguage(base)) {
      return base;
    }
  }
  return DEFAULT_LANGUAGE;
}

export function translate(
  language: Language,
  key: string,
  params?: Record<string, string | number>,
): string {
  const dict = dictionaries[language];
  let value: unknown = dict;
  for (const part of key.split(".")) {
    if (typeof value !== "object" || value === null) {
      return key;
    }
    value = (value as Record<string, unknown>)[part];
  }
  let text = typeof value === "string" ? value : key;

  if (params) {
    for (const [name, replacement] of Object.entries(params)) {
      text = text.split(`{${name}}`).join(String(replacement));
    }
  }
  return text;
}