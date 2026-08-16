import api from "@/lib/api";
import type { DeepPartial, Envelope, UserSettings } from "@/types/api";

export const fetchUserSettings = async (): Promise<Envelope<UserSettings>> => {
    const response = await api.get<Envelope<UserSettings>>('/users/settings');
    return response.data;
};

export const updateUserSettings = async (settings: DeepPartial<UserSettings>): Promise<Envelope<UserSettings>> => {
    const response = await api.patch<Envelope<UserSettings>>('/users/settings', settings);
    return response.data;
};