import { parseISO, formatDistanceToNow, format } from "date-fns";

// Format the post date
export const formatPostDate = (dateString: string) => {
    try {
        const date = parseISO(dateString);
        const now = new Date();
        const diffInHours = Math.abs(now.getTime() - date.getTime()) / (1000 * 60 * 60);

        // If less than 24 hours ago, show relative time
        if (diffInHours < 24) {
            return formatDistanceToNow(date, { addSuffix: false });
        }

        // Otherwise show month and day
        return format(date, 'MMM d');
    } catch (error) {
        console.error('Error formatting date:', error);
        return 'Unknown date';
    }
};

// Get full tooltip date format
export const getFullDateFormat = (dateString: string) => {
    try {
        const date = parseISO(dateString);
        return format(date, 'yyyy/MM/dd HH:mm');
    } catch (error) {
        console.error('Error formatting full date:', error);
        return 'Unknown date';
    }
};