import { useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { toast } from "sonner";
import api from "@/lib/api";
import { useAuth } from "@/contexts/AuthContext";
import { useUser } from "@/contexts/UserContext";
import type { Envelope, UserProfileResponse } from "@/types/api";

export default function AuthCallbackPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { setToken } = useAuth();
  const { setUser } = useUser();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const accessToken = searchParams.get("access_token");
    const isNewUser = searchParams.get("is_new_user") === "1";
    if (!accessToken) {
      setError("Missing access token from Google. Please try again.");
      return;
    }
    // Persist token via context (refresh cookie already set by backend)
    setToken(accessToken);
    // Hydrate profile
    api
      .get<Envelope<UserProfileResponse>>("/users/me", {
        headers: { Authorization: `Bearer ${accessToken}` },
      })
      .then((res) => {
        setUser({
          username: res.data.data.username,
          displayName: res.data.data.display_name,
          profilePictureUUID: res.data.data.profile_picture_uuid ?? "",
          isAdmin: res.data.data.is_admin ?? false,
        });
        toast.success(isNewUser ? "Account created with Google" : "Signed in with Google");
        navigate("/", { replace: true });
      })
      .catch(() => {
        setError("Failed to load your profile after Google sign-in.");
        toast.error("Google callback failed");
      });
  }, [searchParams, setToken, setUser, navigate]);

  if (error) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background p-4">
        <div className="w-full max-w-md rounded-lg border bg-card p-6 text-center">
          <h1 className="mb-2 text-lg font-semibold">Google sign-in failed</h1>
          <p className="mb-4 text-sm text-muted-foreground">{error}</p>
          <button
            className="inline-flex h-10 items-center justify-center rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground"
            onClick={() => navigate("/login")}
          >
            Back to login
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="flex flex-col items-center gap-3">
        <span className="h-8 w-8 animate-spin rounded-full border-2 border-primary border-t-transparent" aria-hidden="true" />
        <p className="text-sm text-muted-foreground">Finishing Google sign-in…</p>
      </div>
    </div>
  );
}
