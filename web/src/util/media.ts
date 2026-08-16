export const getMediaUrl = (uuid?: string): string | undefined => {
    if (!uuid) return undefined;
    return `http://localhost:2021/api/v1/media/${uuid}`;
};