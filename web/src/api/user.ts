import api from "@/lib/api";
import type { Envelope, UpdateProfilePayload, UserProfileResponse } from "@/types/api";

export const getMe = async (): Promise<Envelope<UserProfileResponse>> => {
    const response = await api.get<Envelope<UserProfileResponse>>('/users/me');
    return response.data;
};

export const updateProfile = async (payload: UpdateProfilePayload): Promise<Envelope<UserProfileResponse>> => {
    const response = await api.patch<Envelope<UserProfileResponse>>('/users/me', payload);
    return response.data;
};

// fetch profile
export const fetchProfile = async (username: string): Promise<Envelope<UserProfileResponse>> => {
    const response = await api.get<Envelope<UserProfileResponse>>(`/users/${username}`);
    return response.data;
};

export const followUser = async (username: string): Promise<Envelope<{ success: boolean }>> => {
    const response = await api.post<Envelope<{ success: boolean }>>(`/users/${username}/follow`);
    return response.data;
};

export const unfollowUser = async (username: string): Promise<Envelope<{ success: boolean }>> => {
    const response = await api.delete<Envelope<{ success: boolean }>>(`/users/${username}/follow`);
    return response.data;
};

export const blockUser = async (username: string): Promise<Envelope<{ success: boolean }>> => {
    const response = await api.post<Envelope<{ success: boolean }>>(`/users/${username}/block`);
    return response.data;
};

export const unblockUser = async (username: string): Promise<Envelope<{ success: boolean }>> => {
    const response = await api.delete<Envelope<{ success: boolean }>>(`/users/${username}/block`);
    return response.data;
};

export const fetchUserFollowers = async (username: string, cursor?: string, limit?: number): Promise<Envelope<{ items: UserProfileResponse[]; next_cursor: string | null; has_more: boolean }>> => {
    const response = await api.get<Envelope<{ items: UserProfileResponse[]; next_cursor: string | null; has_more: boolean }>>(`/users/${username}/followers`, {
        params: { limit, cursor },
    });
    return response.data;
};

export const fetchUserFollowing = async (username: string, cursor?: string, limit?: number): Promise<Envelope<{ items: UserProfileResponse[]; next_cursor: string | null; has_more: boolean }>> => {
    const response = await api.get<Envelope<{ items: UserProfileResponse[]; next_cursor: string | null; has_more: boolean }>>(`/users/${username}/following`, {
        params: { limit, cursor },
    });
    return response.data;
};