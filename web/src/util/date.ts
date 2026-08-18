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

// Hour of day (24h) for a message timestamp.
export const formatMessageHour = (dateString: string) => {
    try {
        return format(parseISO(dateString), 'HH:mm');
    } catch (error) {
        console.error('Error formatting message hour:', error);
        return '';
    }
};

// Day key for grouping messages by day (YYYY-MM-DD).
export const getMessageDayKey = (dateString: string) => {
    try {
        return format(parseISO(dateString), 'yyyy-MM-dd');
    } catch (error) {
        console.error('Error formatting message day:', error);
        return '';
    }
};

// Human-friendly label for a message day divider.
export const formatMessageDayLabel = (dateString: string) => {
    try {
        const date = parseISO(dateString);
        const now = new Date();
        const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate());
        const diffDays = Math.round((startOfToday.getTime() - new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime()) / (1000 * 60 * 60 * 24));
        if (diffDays === 0) return 'Today';
        if (diffDays === 1) return 'Yesterday';
        return format(date, 'EEEE, MMMM d, yyyy');
    } catch (error) {
        console.error('Error formatting message day label:', error);
        return 'Unknown date';
    }
};