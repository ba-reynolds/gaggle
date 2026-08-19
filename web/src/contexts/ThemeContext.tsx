import { createContext, useContext, useEffect, useState } from "react"

type Theme = "dark" | "light" | "system"

export type ThemeFont = "inter" | "geist" | "jetbrains" | "lora" | "sketch" | "comic"

export interface ThemeDefinition {
  id: string
  label: string
  group: "Catppuccin" | "Brands" | "Editor" | "Fun"
  swatch: string // primary color preview
  defaultRadius: number
  font: ThemeFont
}

export const FONT_STACKS: Record<ThemeFont, string> = {
  inter: "'Inter', ui-sans-serif, system-ui, sans-serif",
  geist: "'Geist', ui-sans-serif, system-ui, sans-serif",
  jetbrains: "'JetBrains Mono', ui-monospace, SFMono-Regular, monospace",
  lora: "'Lora', ui-serif, Georgia, serif",
  sketch: "'Gochi Hand', 'Architects Daughter', cursive",
  comic: "'Comic Neue', 'Comic Sans MS', 'Gochi Hand', cursive",
}

export const THEME_CATALOG: ThemeDefinition[] = [
  // Brand / identity presets (shadcnstudio)
  { id: "studio-claude", label: "Claude", group: "Brands", swatch: "#d97757", defaultRadius: 0.5, font: "inter" },
  { id: "studio-caffeine", label: "Caffeine", group: "Brands", swatch: "#5f4b32", defaultRadius: 0.5, font: "inter" },
  { id: "studio-perplexity", label: "Perplexity", group: "Brands", swatch: "#20808d", defaultRadius: 0.5, font: "inter" },
  // Catppuccin (dark flavors pair with latte for light mode)
  { id: "catppuccin-mocha-mauve", label: "Catppuccin Mocha", group: "Catppuccin", swatch: "#cba6f7", defaultRadius: 0.625, font: "geist" },
  { id: "catppuccin-mocha-blue", label: "Catppuccin Mocha Blue", group: "Catppuccin", swatch: "#89b4fa", defaultRadius: 0.625, font: "geist" },
  { id: "catppuccin-mocha-peach", label: "Catppuccin Mocha Peach", group: "Catppuccin", swatch: "#fab387", defaultRadius: 0.625, font: "geist" },
  { id: "catppuccin-macchiato-mauve", label: "Catppuccin Macchiato", group: "Catppuccin", swatch: "#c6a0f6", defaultRadius: 0.625, font: "geist" },
  { id: "catppuccin-macchiato-blue", label: "Catppuccin Macchiato Blue", group: "Catppuccin", swatch: "#8aadf4", defaultRadius: 0.625, font: "geist" },
  { id: "catppuccin-macchiato-peach", label: "Catppuccin Macchiato Peach", group: "Catppuccin", swatch: "#f5a97f", defaultRadius: 0.625, font: "geist" },
  { id: "catppuccin-frappe-mauve", label: "Catppuccin Frappé", group: "Catppuccin", swatch: "#ca9ee6", defaultRadius: 0.625, font: "geist" },
  { id: "catppuccin-frappe-blue", label: "Catppuccin Frappé Blue", group: "Catppuccin", swatch: "#8caaee", defaultRadius: 0.625, font: "geist" },
  { id: "catppuccin-frappe-peach", label: "Catppuccin Frappé Peach", group: "Catppuccin", swatch: "#ef9f76", defaultRadius: 0.625, font: "geist" },
  // Iconic code-editor themes (ui.jln.dev gallery)
  { id: "icon-kanagawa", label: "Kanagawa", group: "Editor", swatch: "#7aa89f", defaultRadius: 0.5, font: "jetbrains" },
  // Fun themes
  { id: "fun-neobrutalism", label: "Neo-brutalism", group: "Fun", swatch: "#3333ff", defaultRadius: 0, font: "inter" },
  { id: "fun-comic", label: "Comic", group: "Fun", swatch: "#4dd2ff", defaultRadius: 0.625, font: "comic" },
]

const DEFAULT_THEME_ID = "studio-claude"

function findTheme(id: string): ThemeDefinition {
  return THEME_CATALOG.find((t) => t.id === id) ?? THEME_CATALOG.find((t) => t.id === DEFAULT_THEME_ID)!
}

type ThemeProviderProps = {
  children: React.ReactNode
  defaultTheme?: Theme
  defaultThemeId?: string
  storageKey?: string
  themeIdStorageKey?: string
  fontStorageKey?: string
}

type ThemeProviderState = {
  theme: Theme
  setTheme: (theme: Theme) => void
  themeId: string
  setThemeId: (id: string) => void
  font: ThemeFont
  setFont: (font: ThemeFont) => void
  radius: number
  setRadius: (radius: number) => void
}

const initialState: ThemeProviderState = {
  theme: "system",
  setTheme: () => null,
  themeId: DEFAULT_THEME_ID,
  setThemeId: () => null,
  font: "inter",
  setFont: () => null,
  radius: 0.5,
  setRadius: () => null,
}

const ThemeProviderContext = createContext<ThemeProviderState>(initialState)

export function ThemeProvider({
  children,
  defaultTheme = "system",
  defaultThemeId = DEFAULT_THEME_ID,
  storageKey = "vite-ui-theme",
  themeIdStorageKey = "vite-ui-theme-id",
  fontStorageKey = "vite-ui-font",
  ...props
}: ThemeProviderProps) {
  const [theme, setTheme] = useState<Theme>(
    () => (localStorage.getItem(storageKey) as Theme) || defaultTheme
  )
  const [themeId, setThemeId] = useState<string>(
    () => localStorage.getItem(themeIdStorageKey) || defaultThemeId
  )
  const [font, setFont] = useState<ThemeFont>(() => {
    const stored = localStorage.getItem(fontStorageKey) as ThemeFont | null
    return stored && FONT_STACKS[stored] ? stored : "inter"
  })
  const [radius, setRadius] = useState<number>(() => {
    const stored = localStorage.getItem("vite-ui-radius")
    const parsed = stored ? Number.parseFloat(stored) : NaN
    return Number.isFinite(parsed)
      ? parsed
      : findTheme(localStorage.getItem(themeIdStorageKey) || defaultThemeId).defaultRadius
  })

  useEffect(() => {
    const root = window.document.documentElement

    root.classList.remove("light", "dark")

    if (theme === "system") {
      const systemTheme = window.matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark"
        : "light"
      root.classList.add(systemTheme)
      return
    }

    root.classList.add(theme)
  }, [theme])

  // Color scheme -> data-theme attribute on <html>; resets radius to the
  // theme's default when switching themes (manual radius overrides persist
  // only until a new theme is picked).
  useEffect(() => {
    window.document.documentElement.dataset.theme = themeId
    const definition = findTheme(themeId)
    setRadius(definition.defaultRadius)
  }, [themeId])

  // Font -> --app-font-sans custom property (Tailwind --font-sans maps to it).
  useEffect(() => {
    window.document.documentElement.style.setProperty("--app-font-sans", FONT_STACKS[font])
  }, [font])

  // Radius -> --radius custom property (rem).
  useEffect(() => {
    window.document.documentElement.style.setProperty("--radius", `${radius}rem`)
  }, [radius])

  const value = {
    theme,
    setTheme: (theme: Theme) => {
      localStorage.setItem(storageKey, theme)
      setTheme(theme)
    },
    themeId,
    setThemeId: (id: string) => {
      localStorage.setItem(themeIdStorageKey, id)
      setThemeId(id)
    },
    font,
    setFont: (font: ThemeFont) => {
      localStorage.setItem(fontStorageKey, font)
      setFont(font)
    },
    radius,
    setRadius: (radius: number) => {
      localStorage.setItem("vite-ui-radius", String(radius))
      setRadius(radius)
    },
  }

  return (
    <ThemeProviderContext.Provider {...props} value={value}>
      {children}
    </ThemeProviderContext.Provider>
  )
}

export const useTheme = () => {
  const context = useContext(ThemeProviderContext)

  if (context === undefined)
    throw new Error("useTheme must be used within a ThemeProvider")

  return context
}