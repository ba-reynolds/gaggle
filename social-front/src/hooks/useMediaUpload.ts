import { useMutation } from '@tanstack/react-query';
import { uploadMedia } from '@/api/media';
import type { Envelope, MediaUploadResponse } from '@/types/api';

export function useMediaUpload() {
  return useMutation<Envelope<MediaUploadResponse>, Error, File[]>({
    mutationFn: uploadMedia,
  });
} 