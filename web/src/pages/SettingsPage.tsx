import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useSettings } from "@/hooks/useSettings";
import { useI18n } from "@/contexts/I18nContext";
import ThemeCustomizer from "@/components/ThemeCustomizer";
import { Bell, Eye, Globe, Palette } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { SUPPORTED_LANGUAGES, type Language } from "@/i18n";
import { useMemo } from "react";

const SettingsPage = () => {
    const { settings, isLoading, updateSettings, isUpdating } = useSettings();
    const { t, setLanguage, language } = useI18n();

    // Keep the i18n provider in sync with the persisted setting so the whole
    // UI re-renders in the new language immediately.
    const handleLanguageChange = (value: string) => {
        setLanguage(value as Language);
        updateSettings({ language: value });
    };

    const languageOptions = useMemo(
        () => SUPPORTED_LANGUAGES.map((code) => ({
            code,
            label: t(`settings.language.${code}`),
        })),
        [t],
    );

    if (isLoading) {
        return <SettingsSkeleton />;
    }

    return (
        <div className="container max-w-4xl py-6 space-y-6">
            <div className="flex items-center justify-between">
                <h1 className="text-3xl font-bold">{t("settings.title")}</h1>
                <Button
                    variant="outline"
                    onClick={() => {
                      if (settings) {
                        updateSettings(settings);
                      }
                    }}
                    disabled={isUpdating}
                >
                    {isUpdating ? t("settings.saving") : t("settings.saveChanges")}
                </Button>
            </div>

            <div className="grid gap-6">
                {/* Notifications Section */}
                <Card>
                    <CardHeader>
                        <CardTitle className="flex items-center gap-2">
                            <Bell className="w-5 h-5" />
                            {t("settings.notifications.title")}
                        </CardTitle>
                        <CardDescription>
                            {t("settings.notifications.description")}
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        <div className="flex items-center justify-between">
                            <Label htmlFor="email-notifications">{t("settings.notifications.email")}</Label>
                            <Switch
                                id="email-notifications"
                                checked={settings?.notifications.email}
                                onCheckedChange={(checked) =>
                                    updateSettings({
                                        notifications: { ...settings?.notifications, email: checked },
                                    })
                                }
                            />
                        </div>
                        <div className="flex items-center justify-between">
                            <Label htmlFor="push-notifications">{t("settings.notifications.push")}</Label>
                            <Switch
                                id="push-notifications"
                                checked={settings?.notifications.push}
                                onCheckedChange={(checked) =>
                                    updateSettings({
                                        notifications: { ...settings?.notifications, push: checked },
                                    })
                                }
                            />
                        </div>
                        <div className="flex items-center justify-between">
                            <Label htmlFor="mention-notifications">{t("settings.notifications.mentions")}</Label>
                            <Switch
                                id="mention-notifications"
                                checked={settings?.notifications.mentions}
                                onCheckedChange={(checked) =>
                                    updateSettings({
                                        notifications: { ...settings?.notifications, mentions: checked },
                                    })
                                }
                            />
                        </div>
                    </CardContent>
                </Card>

                {/* Privacy Section */}
                <Card>
                    <CardHeader>
                        <CardTitle className="flex items-center gap-2">
                            <Eye className="w-5 h-5" />
                            {t("settings.privacy.title")}
                        </CardTitle>
                        <CardDescription>
                            {t("settings.privacy.description")}
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        <div className="space-y-2">
                            <Label>{t("settings.privacy.profileVisibility")}</Label>
                            <Select
                                value={settings?.privacy.profileVisibility}
                                onValueChange={(value) =>
                                    updateSettings({
                                        privacy: { ...settings?.privacy, profileVisibility: value as "public" | "private" | "friends" },
                                    })
                                }
                            >
                                <SelectTrigger>
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="public">{t("settings.privacy.public")}</SelectItem>
                                    <SelectItem value="private">{t("settings.privacy.private")}</SelectItem>
                                    <SelectItem value="friends">{t("settings.privacy.friendsOnly")}</SelectItem>
                                </SelectContent>
                            </Select>
                        </div>
                        <div className="flex items-center justify-between">
                            <Label htmlFor="online-status">{t("settings.privacy.showOnlineStatus")}</Label>
                            <Switch
                                id="online-status"
                                checked={settings?.privacy.showOnlineStatus}
                                onCheckedChange={(checked) =>
                                    updateSettings({
                                        privacy: { ...settings?.privacy, showOnlineStatus: checked },
                                    })
                                }
                            />
                        </div>
                        <div className="flex items-center justify-between">
                            <Label htmlFor="allow-tagging">{t("settings.privacy.allowTagging")}</Label>
                            <Switch
                                id="allow-tagging"
                                checked={settings?.privacy.allowTagging}
                                onCheckedChange={(checked) =>
                                    updateSettings({
                                        privacy: { ...settings?.privacy, allowTagging: checked },
                                    })
                                }
                            />
                        </div>
                    </CardContent>
                </Card>

                {/* Appearance Section */}
                <Card>
                    <CardHeader>
                        <CardTitle className="flex items-center gap-2">
                            <Palette className="w-5 h-5" />
                            {t("settings.appearance.title")}
                        </CardTitle>
                        <CardDescription>
                            {t("settings.appearance.description")}
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        <div className="space-y-2">
                            <Label>{t("settings.appearance.theme")}</Label>
                            <Select
                                value={settings?.appearance.theme}
                                onValueChange={(value) =>
                                    updateSettings({
                                        appearance: { ...settings?.appearance, theme: value as "light" | "dark" | "system" },
                                    })
                                }
                            >
                                <SelectTrigger>
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="light">{t("settings.appearance.light")}</SelectItem>
                                    <SelectItem value="dark">{t("settings.appearance.dark")}</SelectItem>
                                    <SelectItem value="system">{t("settings.appearance.system")}</SelectItem>
                                </SelectContent>
                            </Select>
                        </div>
                        <div className="space-y-2">
                            <Label>{t("settings.appearance.fontSize")}</Label>
                            <Select
                                value={settings?.appearance.fontSize}
                                onValueChange={(value) =>
                                    updateSettings({
                                        appearance: { ...settings?.appearance, fontSize: value as "small" | "medium" | "large" },
                                    })
                                }
                            >
                                <SelectTrigger>
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="small">{t("settings.appearance.small")}</SelectItem>
                                    <SelectItem value="medium">{t("settings.appearance.medium")}</SelectItem>
                                    <SelectItem value="large">{t("settings.appearance.large")}</SelectItem>
                                </SelectContent>
                            </Select>
                        </div>
                        <div className="mt-4 border-t border-border pt-4">
                            <ThemeCustomizer />
                        </div>
                    </CardContent>
                </Card>

                {/* Language Section */}
                <Card>
                    <CardHeader>
                        <CardTitle className="flex items-center gap-2">
                            <Globe className="w-5 h-5" />
                            {t("settings.language.title")}
                        </CardTitle>
                        <CardDescription>
                            {t("settings.language.description")}
                        </CardDescription>
                    </CardHeader>
                    <CardContent>
                        <div className="space-y-2">
                            <Select
                                value={language}
                                onValueChange={handleLanguageChange}
                            >
                                <SelectTrigger>
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    {languageOptions.map(({ code, label }) => (
                                        <SelectItem key={code} value={code}>{label}</SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </div>
                    </CardContent>
                </Card>
            </div>
        </div>
    );
};

const SettingsSkeleton = () => {
    return (
        <div className="container max-w-4xl py-6 space-y-6">
            <div className="flex items-center justify-between">
                <Skeleton className="h-9 w-32" />
                <Skeleton className="h-10 w-24" />
            </div>
            <div className="grid gap-6">
                {[1, 2, 3, 4].map((i) => (
                    <Card key={i}>
                        <CardHeader>
                            <Skeleton className="h-6 w-48" />
                            <Skeleton className="h-4 w-72" />
                        </CardHeader>
                        <CardContent className="space-y-4">
                            {[1, 2, 3].map((j) => (
                                <div key={j} className="flex items-center justify-between">
                                    <Skeleton className="h-4 w-32" />
                                    <Skeleton className="h-6 w-12" />
                                </div>
                            ))}
                        </CardContent>
                    </Card>
                ))}
            </div>
        </div>
    );
};

export default SettingsPage;