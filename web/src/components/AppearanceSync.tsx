import { useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { fetchUserSettings } from "@/api/settings";
import { useAuth } from "@/contexts/AuthContext";
import { useTheme, type Theme, type ThemeFontSize } from "@/contexts/ThemeContext";

const VALID_THEMES: Theme[] = ["light", "dark", "system"];
const VALID_FONT_SIZES: ThemeFontSize[] = ["small", "medium", "large"];

/**
 * Adopts the account's persisted appearance (light/dark/system theme + font
 * size) into ThemeContext on load, so the appearance settings actually apply —
 * including across browsers, not just the current localStorage. A choice the
 * user made in this session (appearanceTouched) always wins over a refetch.
 */
const AppearanceSync = () => {
  const { token } = useAuth();
  const { data: settings } = useQuery({
    queryKey: ["settings"],
    queryFn: fetchUserSettings,
    enabled: typeof token === "string",
  });
  const { theme, setTheme, fontSize, setFontSize, appearanceTouched } = useTheme();

  useEffect(() => {
    if (appearanceTouched) return;
    const appearance = settings?.data?.appearance;
    if (!appearance) return;

    if (VALID_THEMES.includes(appearance.theme as Theme) && appearance.theme !== theme) {
      setTheme(appearance.theme as Theme);
    }
    if (VALID_FONT_SIZES.includes(appearance.fontSize as ThemeFontSize) && appearance.fontSize !== fontSize) {
      setFontSize(appearance.fontSize as ThemeFontSize);
    }
  }, [settings, appearanceTouched, theme, fontSize, setTheme, setFontSize]);

  return null;
};

export default AppearanceSync;