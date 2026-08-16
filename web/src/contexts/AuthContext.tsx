import {
  createContext,
  useContext,
  useEffect,
  useLayoutEffect,
  useState,
  type ReactNode,
} from 'react';
import { type AxiosError, type AxiosResponse, type InternalAxiosRequestConfig } from 'axios';
import api from '@/lib/api';
import type { Envelope, RefreshTokenResponse } from '@/types/api';
import { useUser } from './UserContext';


interface AuthContextType {
  token: string | null | undefined;
  setToken: (token: string | null) => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const useAuth = () => {
  const authContext = useContext(AuthContext);

  if (!authContext) {
    throw new Error('useAuth must be used within an AuthProvider');
  }

  return authContext;
};

interface AuthProviderProps {
  children: ReactNode;
}

const AuthProvider = ({ children }: AuthProviderProps) => {
  // Token = undefined -> it's being loaded, show loading dialog
  // Token = null -> not logged in
  // Token = string -> logged in
  const [token, setToken] = useState<string | null | undefined>();
  const { setUser } = useUser();

  // Bootstrap: try to restore the session from the refresh-token cookie and
  // hydrate the current user's profile.
  useEffect(() => {
    const fetchMe = async () => {
      try {
        const refreshTokenResponse = await api.post<Envelope<RefreshTokenResponse>>('/auth/refresh-token');
        const accessToken = refreshTokenResponse.data.data.access_token;
        setToken(accessToken);

        const meResponse = await api.get<Envelope<{ username: string; display_name: string; profile_picture_uuid?: string }>>('/users/me', {
          headers: {
            Authorization: `Bearer ${accessToken}`,
          },
        });
        setUser({
          username: meResponse.data.data.username,
          displayName: meResponse.data.data.display_name,
          profilePictureUUID: meResponse.data.data.profile_picture_uuid ?? '',
        });
      } catch {
        setToken(null);
      }
    };

    fetchMe();
  }, [setUser]);

  useLayoutEffect(() => {
    const authInterceptor = api.interceptors.request.use((config: InternalAxiosRequestConfig) => {
      const retry = (config as InternalAxiosRequestConfig & { _retry?: boolean })._retry;
      if (!retry && token) {
        config.headers.Authorization = `Bearer ${token}`;
      }

      return config;
    });

    return () => {
      api.interceptors.request.eject(authInterceptor);
    };
  }, [token]);

  useLayoutEffect(() => {
    const refreshInterceptor = api.interceptors.response.use(
      (response: AxiosResponse) => response,
      async (error: AxiosError) => {
        const originalRequest = error.config as (InternalAxiosRequestConfig & { _retry?: boolean }) | undefined;

        // Don't try to refresh if we're already trying to refresh the token
        if (error.response?.status === 401 && originalRequest && !originalRequest.url?.includes('/auth/refresh-token')) {
          try {
            const response = await api.post<Envelope<RefreshTokenResponse>>('/auth/refresh-token');
            const accessToken = response.data.data.access_token;
            setToken(accessToken);

            originalRequest._retry = true;
            originalRequest.headers.Authorization = `Bearer ${accessToken}`;

            return api(originalRequest);
          } catch {
            setToken(null);
          }
        }

        return Promise.reject(error);
      }
    );

    return () => {
      api.interceptors.response.eject(refreshInterceptor);
    };
  }, []);

  return (
    <AuthContext.Provider value={{ token, setToken }}>
      {children}
    </AuthContext.Provider>
  );
};

export default AuthProvider;