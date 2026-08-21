import { useEffect, useState, useRef, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import api from "@/lib/api";
import { getGoogleConfig } from "@/api/auth";
import { useGoogleAuthMutation } from "@/hooks/useAuth";
import { useUser } from "@/contexts/UserContext";
import { detectBrowserLanguage } from "@/i18n";
import type { Envelope, UserProfileResponse } from "@/types/api";
import { Button } from "@/components/ui/button";

// Minimal typing for Google Identity Services injected global
declare global {
  interface Window {
    google?: {
      accounts: {
        id: {
          initialize: (opts: { client_id: string; callback: (resp: { credential: string }) => void; auto_select?: boolean; cancel_on_tap_outside?: boolean }) => void;
          renderButton: (el: HTMLElement, opts: Record<string, unknown>) => void;
          prompt: () => void;
        };
      };
    };
  }
}

function useGoogleConfig() {
  const [config, setConfig] = useState<{ enabled: boolean; client_id: string } | null>(null);
  useEffect(() => {
    let cancelled = false;
    getGoogleConfig()
      .then((c) => {
        if (!cancelled) setConfig(c);
      })
      .catch(() => {
        if (!cancelled) setConfig({ enabled: false, client_id: "" });
      });
    return () => {
      cancelled = true;
    };
  }, []);
  return config;
}

export default function GoogleSignInButton({ mode = "signin" }: { mode?: "signin" | "signup" }) {
  const config = useGoogleConfig();
  const navigate = useNavigate();
  const { setUser } = useUser();
  const mutation = useGoogleAuthMutation();
  const gsiContainerRef = useRef<HTMLDivElement>(null);
  const [gsiReady, setGsiReady] = useState(false);

  const handleCredential = useCallback(
    async (credential: string) => {
      try {
        const res = await mutation.mutateAsync({ credential, language: detectBrowserLanguage() });
        const me = await api.get<Envelope<UserProfileResponse>>("/users/me", {
          headers: { Authorization: `Bearer ${res.data.access_token}` },
        });
        setUser({
          username: me.data.data.username,
          displayName: me.data.data.display_name,
          profilePictureUUID: me.data.data.profile_picture_uuid ?? "",
          isAdmin: me.data.data.is_admin ?? false,
        });
        const isNew = (res.data as unknown as { is_new_user?: boolean }).is_new_user;
        toast.success(isNew ? "Account created with Google" : "Signed in with Google");
        navigate("/");
      } catch {
        toast.error("Google sign-in failed");
      }
    },
    [mutation, navigate, setUser],
  );

  // Load GSI script when config indicates Google is enabled
  useEffect(() => {
    if (!config?.enabled || !config.client_id) return;
    if (window.google?.accounts?.id) {
      setGsiReady(true);
      return;
    }
    const script = document.createElement("script");
    script.src = "https://accounts.google.com/gsi/client";
    script.async = true;
    script.defer = true;
    script.onload = () => setGsiReady(true);
    script.onerror = () => setGsiReady(false);
    document.head.appendChild(script);
    return () => {
      // keep script for future mounts
    };
  }, [config]);

  // Initialize and render GIS button when ready
  useEffect(() => {
    if (!gsiReady || !config?.client_id || !window.google || !gsiContainerRef.current) return;
    try {
      window.google.accounts.id.initialize({
        client_id: config.client_id,
        callback: (resp: { credential: string }) => {
          void handleCredential(resp.credential);
        },
      });
      gsiContainerRef.current.innerHTML = "";
      window.google.accounts.id.renderButton(gsiContainerRef.current, {
        theme: "outline",
        size: "large",
        width: 320,
        text: mode === "signup" ? "signup_with" : "signin_with",
        shape: "rectangular",
      });
    } catch {
      // GIS render failed; fall back to redirect button
    }
  }, [gsiReady, config, mode, handleCredential]);

  if (config === null) {
    return (
      <div className="h-10 w-full animate-pulse rounded-md bg-muted" aria-hidden="true" />
    );
  }
  if (!config.enabled) {
    return null;
  }

  const handleRedirectLogin = () => {
    // Code flow: backend builds the Google consent URL and redirects; the
    // callback will land on /auth/callback with an access_token.
    const base = (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? "/api/v1";
    // VITE_API_BASE_URL may be "/api/v1" (same-origin proxied); we need an absolute for redirect.
    // When it's relative, the browser will resolve it against the current origin, which is correct.
    window.location.href = `${base}/auth/google/login`;
  };

  return (
    <div className="space-y-3">
      {/* GIS rendered button (when script loads) */}
      <div ref={gsiContainerRef} className="flex justify-center" />
      {/* Always show a fallback redirect button for the code flow; GIS hides via overlay if it rendered */}
      <div className="relative">
        <div className="absolute inset-0 flex items-center">
          <span className="w-full border-t" />
        </div>
        <div className="relative flex justify-center text-xs uppercase">
          <span className="bg-background px-2 text-muted-foreground">Or</span>
        </div>
      </div>
      <Button
        type="button"
        variant="outline"
        className="w-full"
        onClick={handleRedirectLogin}
        disabled={mutation.isPending}
      >
        <svg className="mr-2 h-4 w-4" viewBox="0 0 24 24" aria-hidden="true">
          <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" />
          <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" />
          <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" />
          <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" />
        </svg>
        {mode === "signup" ? "Continue with Google" : "Continue with Google"}
      </Button>
      {mutation.isPending && <p className="text-center text-xs text-muted-foreground">Verifying with Google…</p>}
    </div>
  );
}
