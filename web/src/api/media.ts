import api from '@/lib/api';
import type { Envelope, MediaUploadResponse } from '@/types/api';

export const uploadMedia = async (files: File[]): Promise<Envelope<MediaUploadResponse>> => {
  const formData = new FormData();
  
  // Add up to 4 files to the form data
  files.slice(0, 4).forEach(file => {
    formData.append('media', file);
  });
  
  const response = await api.post<Envelope<MediaUploadResponse>>('/media/upload', formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  });
  
  return response.data;
};