import api from "@/lib/api";
import type { Envelope, LoginPayload, LoginResponse, RefreshTokenResponse, RegisterPayload, RegisterResponse } from "@/types/api";

export interface GoogleAuthPayload {
  id_token?: string;
  credential?: string;
  language?: string;
}
export interface GoogleAuthResponse {
  access_token: string;
  is_new_user?: boolean;
}
export interface GoogleConfigResponse {
  enabled: boolean;
  client_id: string;
}

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

export const googleAuth = async (payload: GoogleAuthPayload): Promise<Envelope<GoogleAuthResponse>> => {
    const body: Record<string, string> = {};
    if (payload.id_token) body.id_token = payload.id_token;
    if (payload.credential) body.credential = payload.credential;
    if (!body.id_token && !body.credential) {
        // tolerate either key; if caller passed id_token as credential, normalize
        if (payload.id_token) body.id_token = payload.id_token;
    }
    if (payload.language) body.language = payload.language;
    const response = await api.post<Envelope<GoogleAuthResponse>>('/auth/google', body);
    return response.data;
};

export const getGoogleConfig = async (): Promise<GoogleConfigResponse> => {
    const response = await api.get<GoogleConfigResponse>('/auth/google/config');
    return response.data;
};