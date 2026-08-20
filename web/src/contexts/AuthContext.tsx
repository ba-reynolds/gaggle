import {
  createContext,
  useContext,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
} from 'react';
import { type AxiosError, type AxiosResponse, type InternalAxiosRequestConfig } from 'axios';
import { toast } from "sonner";
import api from '@/lib/api';
import type { Envelope, RefreshTokenResponse } from '@/types/api';
import { useUser } from './UserContext';
import { translate, getCurrentLanguage } from '@/i18n';


interface AuthContextType {
  token: string | null | undefined;
  setToken: (token: string | null) => void;
}

// The bootstrap refresh-token /users/me round trip must never hold the app on
// the boot screen forever: if the box stops answering, abort after this long
// and fall back to the logged-out state (login page) instead of a dead
// "Loading..." splash.
const AUTH_BOOTSTRAP_TIMEOUT_MS = 10_000;

// The server reports "your session is over" distinctly from a generic auth
// failure so we can tell the user why they're being signed out.
const isSessionExpired = (err: unknown) =>
  (err as AxiosError<Envelope<unknown>> | undefined)?.response?.data?.error?.code === 'SESSION_EXPIRED';

// AbortSignal.timeout isn't universal yet; fall back to no timeout where it's
// missing. The refresh request keeps its own .finally() cleanup either way.
const bootstrapSignal = (): AbortSignal | undefined =>
  typeof AbortSignal !== 'undefined' && typeof AbortSignal.timeout === 'function'
    ? AbortSignal.timeout(AUTH_BOOTSTRAP_TIMEOUT_MS)
    : undefined;

const notifySessionExpired = () => {
  toast.error(translate(getCurrentLanguage(), "auth.sessionExpired"));
};

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
  // Mirror token in a ref so the response interceptor always sees the latest
  // value without being re-registered on every token change.
  const tokenRef = useRef<string | null | undefined>(undefined);
  // Single-flight: all concurrent 401s share one refresh request.
  const refreshPromiseRef = useRef<Promise<string> | null>(null);

  const refreshAccessToken = async (signal?: AbortSignal): Promise<string> => {
    if (!refreshPromiseRef.current) {
      refreshPromiseRef.current = api
        .post<Envelope<RefreshTokenResponse>>('/auth/refresh-token', undefined, { signal })
        .then((res) => {
          const accessToken = res.data.data.access_token;
          setToken(accessToken);
          return accessToken;
        })
        .finally(() => {
          refreshPromiseRef.current = null;
        });
    }
    return refreshPromiseRef.current;
  };

  // Bootstrap: try to restore the session from the refresh-token cookie and
  // hydrate the current user's profile.
  useEffect(() => {
    const fetchMe = async () => {
      try {
        const signal = bootstrapSignal();
        const accessToken = await refreshAccessToken(signal);

        const meResponse = await api.get<Envelope<{ username: string; display_name: string; profile_picture_uuid?: string; is_admin?: boolean }>>('/users/me', {
          headers: {
            Authorization: `Bearer ${accessToken}`,
          },
          signal,
        });
        setUser({
          username: meResponse.data.data.username,
          displayName: meResponse.data.data.display_name,
          profilePictureUUID: meResponse.data.data.profile_picture_uuid ?? '',
          isAdmin: meResponse.data.data.is_admin ?? false,
        });
      } catch (err) {
        if (isSessionExpired(err)) {
          notifySessionExpired();
        }
        setToken(null);
      }
    };

    fetchMe();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useLayoutEffect(() => {
    tokenRef.current = token;
  }, [token]);

  useLayoutEffect(() => {
    const authInterceptor = api.interceptors.request.use((config: InternalAxiosRequestConfig) => {
      const retry = (config as InternalAxiosRequestConfig & { _retry?: boolean })._retry;
      if (!retry && tokenRef.current) {
        config.headers.Authorization = `Bearer ${tokenRef.current}`;
      }

      return config;
    });

    return () => {
      api.interceptors.request.eject(authInterceptor);
    };
  }, []);

  useLayoutEffect(() => {
    const refreshInterceptor = api.interceptors.response.use(
      (response: AxiosResponse) => response,
      async (error: AxiosError) => {
        const originalRequest = error.config as (InternalAxiosRequestConfig & { _retry?: boolean }) | undefined;

        // We only try to refresh if we believe we have a session. When logged
        // out there is no refresh cookie to use, and hammering the endpoint
        // (once per 401, multiplied by Query retries) would trip rate limits.
        const shouldRefresh =
          error.response?.status === 401 &&
          originalRequest &&
          !originalRequest.url?.includes('/auth/refresh-token') &&
          !originalRequest._retry &&
          tokenRef.current != null;

        if (shouldRefresh) {
          try {
            const accessToken = await refreshAccessToken();
            originalRequest._retry = true;
            originalRequest.headers.Authorization = `Bearer ${accessToken}`;
            return api(originalRequest);
          } catch (refreshError) {
            if (isSessionExpired(refreshError)) {
              notifySessionExpired();
            }
            setToken(null);
          }
        }

        return Promise.reject(error);
      }
    );

    return () => {
      api.interceptors.response.eject(refreshInterceptor);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <AuthContext.Provider value={{ token, setToken }}>
      {children}
    </AuthContext.Provider>
  );
};

export default AuthProvider;
