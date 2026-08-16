import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { fetchUserSettings, updateUserSettings } from "@/api/settings";
import type { DeepPartial, UserSettings } from "@/types/api";
import { toast } from "sonner";

export const useSettings = () => {
    const queryClient = useQueryClient();

    const { data: settings, isLoading } = useQuery({
        queryKey: ['settings'],
        queryFn: fetchUserSettings,
    });

    const updateSettingsMutation = useMutation({
        mutationFn: updateUserSettings,
        onSuccess: (data) => {
            queryClient.setQueryData(['settings'], data);
            toast.success("Settings updated successfully");
        },
        onError: () => {
            toast.error("Failed to update settings");
        },
    });

    const updateSettings = (newSettings: DeepPartial<UserSettings>) => {
        updateSettingsMutation.mutate(newSettings);
    };

    return {
        settings: settings?.data,
        isLoading,
        updateSettings,
        isUpdating: updateSettingsMutation.isPending,
    };
}; 