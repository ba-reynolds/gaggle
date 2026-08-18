import React from 'react';
import { Link } from 'react-router-dom';

// Matches backend postutil.Extract: unicode letters/digits/underscore.
export const HASHTAG_REGEX = /(#[\p{L}\p{N}_]+)/gu;

interface HashtagTextProps {
  content: string;
}

// Renders post content with clickable, accent-colored hashtags. Shared by
// FeedPost (display) and ComposeContent (live highlight while typing).
const HashtagText: React.FC<HashtagTextProps> = ({ content }) => {
  return (
    <>
      {content.split(HASHTAG_REGEX).map((part, index) => {
        if (!part.startsWith('#')) {
          return <React.Fragment key={`${part}-${index}`}>{part}</React.Fragment>;
        }
        const tag = part.slice(1).toLowerCase();
        return (
          <Link
            key={`${part}-${index}`}
            to={`/hashtags/${encodeURIComponent(tag)}`}
            onClick={(event) => event.stopPropagation()}
            className="text-blue-600 dark:text-blue-400 hover:underline"
          >
            {part}
          </Link>
        );
      })}
    </>
  );
};

export default HashtagText;