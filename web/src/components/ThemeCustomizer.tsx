import { useTheme, FONT_STACKS, THEME_CATALOG, type ThemeFont } from "@/contexts/ThemeContext";
import { Label } from "@/components/ui/label";
import { Slider } from "@/components/ui/slider";
import { cn } from "@/lib/utils";
import { ThemeToggle } from "@/components/ThemeToggle";

const FONTS: { id: ThemeFont; label: string }[] = [
  { id: "inter", label: "Inter" },
  { id: "geist", label: "Geist" },
  { id: "jetbrains", label: "JetBrains Mono" },
  { id: "lora", label: "Lora" },
  { id: "comic", label: "Comic" },
];

const ThemeCustomizer: React.FC = () => {
  const { themeId, setThemeId, font, setFont, radius, setRadius } = useTheme();
  const groups = ["Classic", "Brands", "Catppuccin", "Editor", "Fun"] as const;

  return (
    <div className="space-y-4">
      {/* Light / dark / system */}
      <ThemeToggle />

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
                    ? "border-primary ring-2 ring-ring"
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
                font === f.id ? "border-primary ring-2 ring-ring" : "border-border hover:border-primary/50"
              )}
              style={{ fontFamily: FONT_STACKS[f.id] }}
            >
              <span className="text-primary">{f.label}</span>
            </button>
          ))}
        </div>
      </div>

      {/* Radius */}
      <div>
        <Label className="text-primary">Rounded corners</Label>
        <div className="mt-2">
          <Slider
            min={0}
            max={1.25}
            step={0.05}
            value={[radius]}
            onValueChange={(values) => setRadius(values[0])}
          />
        </div>
        <p className="mt-1 text-xs text-muted-foreground">{radius.toFixed(2)}rem</p>
      </div>
    </div>
  );
};

export default ThemeCustomizer;