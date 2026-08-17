export const getMediaUrl = (uuid?: string): string | undefined => {
    if (!uuid) return undefined;
    return `/api/v1/media/${uuid}`;
};