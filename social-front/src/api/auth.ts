import api from "@/lib/api";
import type { Envelope, LoginPayload, LoginResponse, RefreshTokenResponse, RegisterPayload, RegisterResponse } from "@/types/api";

export const login = async ({ identifier, password }: LoginPayload): Promise<Envelope<LoginResponse>> => {
    const response = await api.post<Envelope<LoginResponse>>('/auth/login', { identifier, password });
    return response.data;
};

export const register = async (payload: RegisterPayload): Promise<Envelope<RegisterResponse>> => {
    const response = await api.post<Envelope<RegisterResponse>>('/auth/register', payload);
    return response.data;
};

export const refreshToken = async (): Promise<Envelope<RefreshTokenResponse>> => {
    const response = await api.post<Envelope<RefreshTokenResponse>>('/auth/refresh-token');
    return response.data;
};

export const logout = async (): Promise<Envelope<null>> => {
    const response = await api.post<Envelope<null>>('/auth/logout');
    return response.data;
};