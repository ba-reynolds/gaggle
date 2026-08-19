import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import * as z from 'zod';
import api from '@/lib/api';
import type { Envelope, UserProfileResponse } from '@/types/api';
import { useLoginMutation } from './useAuth';
import { useUser } from '@/contexts/UserContext';

export const loginSchema = z.object({
  identifier: z.string()
    .min(1, 'Username or email is required')
    .min(3, 'Username or email must be at least 3 characters long')
    .max(96, 'Username or email must be at most 96 characters long'),
  password: z.string()
    .min(1, 'Password is required')
    .min(8, 'Password must be at least 8 characters long')
    .max(72, 'Password must be at most 72 characters long'),
});

export type LoginFormValues = z.infer<typeof loginSchema>;

export function useLoginFlow() {
  const navigate = useNavigate();
  const { setUser } = useUser();
  const loginMutation = useLoginMutation();

  const form = useForm<LoginFormValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: {
      identifier: '',
      password: '',
    },
  });

  const onSubmit = async (values: LoginFormValues) => {
    try {
      const loginResponse = await loginMutation.mutateAsync(values);
      const meResponse = await api.get<Envelope<UserProfileResponse>>('/users/me', {
        headers: {
          Authorization: `Bearer ${loginResponse.data.access_token}`,
        },
      });
      setUser({
        username: meResponse.data.data.username,
        displayName: meResponse.data.data.display_name,
        profilePictureUUID: meResponse.data.data.profile_picture_uuid ?? '',
        isAdmin: meResponse.data.data.is_admin ?? false,
      });
      toast.success('Login successful');
      navigate('/');
    } catch {
      toast.error('Login failed, invalid credentials');
    }
  };

  return { form, loginMutation, onSubmit };
}