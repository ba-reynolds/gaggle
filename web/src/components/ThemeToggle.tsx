import { Moon, Sun, Monitor } from "lucide-react";
import { useTheme, type Theme } from "@/contexts/ThemeContext";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";

export function ThemeToggle({
  onThemeChange,
}: {
  onThemeChange?: (theme: Theme) => void;
}) {
  const { theme, setTheme } = useTheme();

  return (
    <Tabs
      value={theme}
      onValueChange={(value) => {
        const next = value as Theme;
        setTheme(next);
        onThemeChange?.(next);
      }}
    >
      <TabsList className="w-full grid grid-cols-3 bg-foreground/10">
        <TabsTrigger value="system" className="flex items-center justify-center">
          <Monitor className="h-4 w-4 xl:mr-2" />
          <span className="hidden xl:inline">System</span>
        </TabsTrigger>
        <TabsTrigger value="light" className="flex items-center justify-center">
          <Sun className="h-4 w-4 xl:mr-2" />
          <span className="hidden xl:inline">Light</span>
        </TabsTrigger>
        <TabsTrigger value="dark" className="flex items-center justify-center">
          <Moon className="h-4 w-4 xl:mr-2" />
          <span className="hidden xl:inline">Dark</span>
        </TabsTrigger>
      </TabsList>
    </Tabs>
  );
}