import { useMutation, useQueryClient } from "@tanstack/react-query";
import { login, logout, register } from "@/api/auth";
import type { Envelope, LoginPayload, LoginResponse, RegisterPayload, RegisterResponse } from "@/types/api";
import { useUser } from "@/contexts/UserContext";
import { useAuth } from "@/contexts/AuthContext";
import { useNavigate } from "react-router-dom";


export const useLoginMutation = () => {
    const queryClient = useQueryClient();
    const { setToken } = useAuth();

    return useMutation<Envelope<LoginResponse>, Error, LoginPayload>({
        mutationFn: login,
        onSuccess: (data) => {
            queryClient.invalidateQueries({ queryKey: ["auth"] });
            setToken(data.data.access_token);
        },
    });
};

export const useRegisterMutation = () => {
    const queryClient = useQueryClient();
    const { setToken } = useAuth();

    return useMutation<Envelope<RegisterResponse>, Error, RegisterPayload>({
        mutationFn: register,
        onSuccess: (data) => {
            queryClient.invalidateQueries({ queryKey: ["auth"] });
            setToken(data.data.access_token);
        },
    });
};

export const useLogoutMutation = () => {
    const queryClient = useQueryClient();
    const navigate = useNavigate();
    const { setToken } = useAuth();
    const { setUser } = useUser();

    return useMutation<Envelope<null>, Error>({
        mutationFn: logout,
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["auth"] });
            setToken(null);
            setUser({
                username: "",
                displayName: "",
                profilePictureUUID: "",
            });
            navigate("/login");
        },
    });
};