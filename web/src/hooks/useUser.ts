import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { blockUser, fetchPinnedPost, fetchProfile, fetchUserFollowers, fetchUserFollowing, followUser, muteUser, unblockUser, unfollowUser, unmuteUser, updateProfile } from '@/api/user';
import type { Envelope, UpdateProfilePayload, UserProfileResponse } from '@/types/api';
import { updateAuthorInPostQueries } from './usePost';

const invalidateRelationshipQueries = (queryClient: ReturnType<typeof useQueryClient>, username: string) => {
  void queryClient.invalidateQueries({ queryKey: ['profile', username] });
  void queryClient.invalidateQueries({ queryKey: ['user-followers', username] });
  void queryClient.invalidateQueries({ queryKey: ['user-following', username] });
};

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
      invalidateRelationshipQueries(queryClient, username);
    },
  });
};

export const useUnfollowUser = () => {
  const queryClient = useQueryClient();

  return useMutation<Envelope<{ success: boolean }>, Error, string>({
    mutationFn: unfollowUser,
    onSuccess: (_data, username) => {
      invalidateRelationshipQueries(queryClient, username);
    },
  });
};

export const useBlockUser = () => {
  const queryClient = useQueryClient();

  return useMutation<Envelope<{ success: boolean }>, Error, string>({
    mutationFn: blockUser,
    onSuccess: (_data, username) => {
      invalidateRelationshipQueries(queryClient, username);
    },
  });
};

export const useUnblockUser = () => {
  const queryClient = useQueryClient();

  return useMutation<Envelope<{ success: boolean }>, Error, string>({
    mutationFn: unblockUser,
    onSuccess: (_data, username) => {
      invalidateRelationshipQueries(queryClient, username);
    },
  });
};

export const useMuteUser = () => {
  const queryClient = useQueryClient();

  return useMutation<Envelope<{ success: boolean }>, Error, string>({
    mutationFn: muteUser,
    onSuccess: (_data, username) => {
      invalidateRelationshipQueries(queryClient, username);
    },
  });
};

export const useUnmuteUser = () => {
  const queryClient = useQueryClient();

  return useMutation<Envelope<{ success: boolean }>, Error, string>({
    mutationFn: unmuteUser,
    onSuccess: (_data, username) => {
      invalidateRelationshipQueries(queryClient, username);
    },
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
