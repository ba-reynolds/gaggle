import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useSettings } from "@/hooks/useSettings";
import ThemeCustomizer from "@/components/ThemeCustomizer";
import { Bell, Eye, Globe, Palette } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";

const SettingsPage = () => {
    const { settings, isLoading, updateSettings, isUpdating } = useSettings();

    if (isLoading) {
        return <SettingsSkeleton />;
    }

    return (
        <div className="container max-w-4xl py-6 space-y-6">
            <div className="flex items-center justify-between">
                <h1 className="text-3xl font-bold">Settings</h1>
                <Button
                    variant="outline"
                    onClick={() => {
                      if (settings) {
                        updateSettings(settings);
                      }
                    }}
                    disabled={isUpdating}
                >
                    {isUpdating ? "Saving..." : "Save Changes"}
                </Button>
            </div>

            <div className="grid gap-6">
                {/* Notifications Section */}
                <Card>
                    <CardHeader>
                        <CardTitle className="flex items-center gap-2">
                            <Bell className="w-5 h-5" />
                            Notifications
                        </CardTitle>
                        <CardDescription>
                            Manage how you receive notifications
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        <div className="flex items-center justify-between">
                            <Label htmlFor="email-notifications">Email Notifications</Label>
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
                            <Label htmlFor="push-notifications">Push Notifications</Label>
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
                            <Label htmlFor="mention-notifications">Mention Notifications</Label>
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
                            Privacy
                        </CardTitle>
                        <CardDescription>
                            Control your privacy settings
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        <div className="space-y-2">
                            <Label>Profile Visibility</Label>
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
                                    <SelectItem value="public">Public</SelectItem>
                                    <SelectItem value="private">Private</SelectItem>
                                    <SelectItem value="friends">Friends Only</SelectItem>
                                </SelectContent>
                            </Select>
                        </div>
                        <div className="flex items-center justify-between">
                            <Label htmlFor="online-status">Show Online Status</Label>
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
                            <Label htmlFor="allow-tagging">Allow Tagging</Label>
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
                            Appearance
                        </CardTitle>
                        <CardDescription>
                            Customize how the app looks
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        <div className="space-y-2">
                            <Label>Theme</Label>
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
                                    <SelectItem value="light">Light</SelectItem>
                                    <SelectItem value="dark">Dark</SelectItem>
                                    <SelectItem value="system">System</SelectItem>
                                </SelectContent>
                            </Select>
                        </div>
                        <div className="space-y-2">
                            <Label>Font Size</Label>
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
                                    <SelectItem value="small">Small</SelectItem>
                                    <SelectItem value="medium">Medium</SelectItem>
                                    <SelectItem value="large">Large</SelectItem>
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
                            Language
                        </CardTitle>
                        <CardDescription>
                            Choose your preferred language
                        </CardDescription>
                    </CardHeader>
                    <CardContent>
                        <div className="space-y-2">
                            <Select
                                value={settings?.language}
                                onValueChange={(value) =>
                                    updateSettings({ language: value })
                                }
                            >
                                <SelectTrigger>
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="en">English</SelectItem>
                                    <SelectItem value="es">Español</SelectItem>
                                    <SelectItem value="fr">Français</SelectItem>
                                    <SelectItem value="de">Deutsch</SelectItem>
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