import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { blockUser, fetchPinnedPost, fetchProfile, fetchUserFollowers, fetchUserFollowing, followUser, unblockUser, unfollowUser, updateProfile } from '@/api/user';
import type { Envelope, UpdateProfilePayload, UserProfileResponse } from '@/types/api';
import { updateAuthorInPostQueries } from './usePost';

export const useUpdateProfile = () => {
  const queryClient = useQueryClient();

  return useMutation<Envelope<UserProfileResponse>, Error, UpdateProfilePayload>({
    mutationFn: updateProfile,
    onSuccess: (data) => {
      const { data: profile } = data;

      // Update profile cache for the affected user
      queryClient.setQueryData(['profile', profile.username], data);

      // Update author information in all post queries
      updateAuthorInPostQueries(queryClient, profile.username, {
        display_name: profile.display_name,
        profile_picture_uuid: profile.profile_picture_uuid,
      });
    },
  });
};

export const useFetchProfile = (username: string, enabled: boolean = true) => {
  return useQuery<Envelope<UserProfileResponse>, Error>({
    queryKey: ['profile', username],
    queryFn: () => fetchProfile(username),
    enabled,
  });
};

export const usePinnedPost = (username: string) => useQuery({
  queryKey: ['pinned-post', username],
  queryFn: () => fetchPinnedPost(username),
  enabled: !!username,
  retry: false,
});

export const useFollowUser = () => {
  const queryClient = useQueryClient();

  return useMutation<Envelope<{ success: boolean }>, Error, string>({
    mutationFn: followUser,
    onSuccess: (_data, username) => {
      queryClient.invalidateQueries({ queryKey: ['profile', username] });
    },
  });
};

export const useUnfollowUser = () => {
  const queryClient = useQueryClient();

  return useMutation<Envelope<{ success: boolean }>, Error, string>({
    mutationFn: unfollowUser,
    onSuccess: (_data, username) => {
      queryClient.invalidateQueries({ queryKey: ['profile', username] });
    },
  });
};

export const useBlockUser = () => {
  return useMutation<Envelope<{ success: boolean }>, Error, string>({
    mutationFn: blockUser,
  });
};

export const useUnblockUser = () => {
  return useMutation<Envelope<{ success: boolean }>, Error, string>({
    mutationFn: unblockUser,
  });
};

export const useFetchUserFollowers = (username: string) => {
  return useQuery<Envelope<{ items: UserProfileResponse[]; next_cursor: string | null; has_more: boolean }>, Error>({
    queryKey: ['user-followers', username],
    queryFn: () => fetchUserFollowers(username),
  });
};

export const useFetchUserFollowing = (username: string) => {
  return useQuery<Envelope<{ items: UserProfileResponse[]; next_cursor: string | null; has_more: boolean }>, Error>({
    queryKey: ['user-following', username],
    queryFn: () => fetchUserFollowing(username),
  });
};
