import { useTheme, FONT_STACKS, THEME_CATALOG, type Theme, type ThemeFont, type ThemeFontSize } from "@/contexts/ThemeContext";
import { useSettings } from "@/hooks/useSettings";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";
import { ThemeToggle } from "@/components/ThemeToggle";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";

const FONTS: { id: ThemeFont; label: string }[] = [
  { id: "inter", label: "Inter" },
  { id: "geist", label: "Geist" },
  { id: "jetbrains", label: "JetBrains Mono" },
  { id: "lora", label: "Lora" },
  { id: "comic", label: "Comic" },
];

const FONT_SIZES: { id: ThemeFontSize; label: string }[] = [
  { id: "small", label: "Small" },
  { id: "medium", label: "Medium" },
  { id: "large", label: "Large" },
];

const ThemeCustomizer: React.FC = () => {
  const { themeId, setThemeId, font, setFont, fontSize, setFontSize } = useTheme();
  const { settings, updateSettings } = useSettings();
  const groups = ["Brands", "Catppuccin", "Editor", "Fun"] as const;

  // Light/dark/system + font size persist to the account so they survive
  // across browsers; the theme catalog and font family stay local (no server
  // fields exist for them yet).
  const handleThemeChange = (theme: Theme) => {
    updateSettings({ appearance: { ...settings?.appearance, theme } });
  };

  const handleFontSizeChange = (size: ThemeFontSize) => {
    updateSettings({ appearance: { ...settings?.appearance, fontSize: size } });
  };

  return (
    <div className="space-y-4">
      {/* Light / dark / system */}
      <ThemeToggle onThemeChange={handleThemeChange} />

      {/* Font size */}
      <div>
        <Label className="text-primary">Font Size</Label>
        <Tabs
          value={fontSize}
          onValueChange={(value) => {
            const next = value as ThemeFontSize;
            setFontSize(next);
            handleFontSizeChange(next);
          }}
        >
          <TabsList className="w-full grid grid-cols-3 bg-foreground/10">
            {FONT_SIZES.map((f) => (
              <TabsTrigger key={f.id} value={f.id} className="flex items-center justify-center">
                <span>{f.label}</span>
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      </div>

      {/* Theme catalog grouped by source */}
      {groups.map((group) => (
        <div key={group}>
          <Label className="text-primary">{group}</Label>
          <div className="mt-2 grid grid-cols-2 gap-2">
            {THEME_CATALOG.filter((t) => t.group === group).map((t) => (
              <button
                key={t.id}
                onClick={() => setThemeId(t.id)}
                className={cn(
                  "flex items-center gap-2 rounded-md border px-2 py-1.5 text-left text-xs transition-colors",
                  themeId === t.id
                    ? "border-primary bg-primary/10 font-medium ring-2 ring-ring"
                    : "border-border hover:border-primary/50"
                )}
              >
                <span className="h-5 w-5 shrink-0 rounded-full border border-border" style={{ backgroundColor: t.swatch }} />
                <span className="truncate text-primary">{t.label}</span>
              </button>
            ))}
          </div>
        </div>
      ))}

      {/* Font */}
      <div>
        <Label className="text-primary">Font</Label>
        <div className="mt-2 grid grid-cols-2 gap-2">
          {FONTS.map((f) => (
            <button
              key={f.id}
              onClick={() => setFont(f.id)}
              className={cn(
                "rounded-md border px-2 py-1.5 text-left text-xs transition-colors",
                font === f.id
                  ? "border-primary bg-primary/10 font-medium ring-2 ring-ring"
                  : "border-border hover:border-primary/50"
              )}
              style={{ fontFamily: FONT_STACKS[f.id] }}
            >
              <span className="text-primary">{f.label}</span>
            </button>
          ))}
        </div>
      </div>
    </div>
  );
};

export default ThemeCustomizer;