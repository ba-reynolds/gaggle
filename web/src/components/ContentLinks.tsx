import React from 'react';
import { Link } from 'react-router-dom';

// Matches hashtag and @mention tokens. Mirrors backend postutil charset
// (unicode letters/digits/underscore); hashtags cap at 100 on the server and
// mention usernames at 16 (users.username).
const TOKEN_REGEX = /(#[\p{L}\p{N}_]+|@[\p{L}\p{N}_]+)/gu;
const HASHTAG_TOKEN = /^#[\p{L}\p{N}_]+$/u;
const MENTION_TOKEN = /^@[\p{L}\p{N}_]+$/u;

interface ContentLinksProps {
  content: string;
}

// Renders post content with clickable, accent-colored hashtags and user
// mentions. Shared by FeedPost (display) and ComposeContent (live highlight
// while typing).
const ContentLinks: React.FC<ContentLinksProps> = ({ content }) => {
  return (
    <>
      {content.split(TOKEN_REGEX).map((part, index) => {
        if (HASHTAG_TOKEN.test(part)) {
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
        }
        if (MENTION_TOKEN.test(part)) {
          const username = part.slice(1);
          return (
            <Link
              key={`${part}-${index}`}
              to={`/profile/${encodeURIComponent(username)}`}
              onClick={(event) => event.stopPropagation()}
              className="text-blue-600 dark:text-blue-400 hover:underline"
            >
              {part}
            </Link>
          );
        }
        return <React.Fragment key={`${part}-${index}`}>{part}</React.Fragment>;
      })}
    </>
  );
};

export default ContentLinks;