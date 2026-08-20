import React from 'react';
import { Link } from 'react-router-dom';

// Matches hashtag, @mention, and http(s) URL tokens. Mirrors backend
// postutil charset (unicode letters/digits/underscore); hashtags cap at 100
// on the server and mention usernames at 16 (users.username).
const TOKEN_REGEX = /(https?:\/\/[^\s]+|#[\p{L}\p{N}_]+|@[\p{L}\p{N}_]+)/gu;
const HASHTAG_TOKEN = /^#[\p{L}\p{N}_]+$/u;
const MENTION_TOKEN = /^@[\p{L}\p{N}_]+$/u;
const URL_TOKEN = /^https?:\/\/[^\s]+$/;

function splitUrlToken(part: string): { href: string; display: string; trailing: string } {
  // Mirror ComposeContent.extractUrl trailing-punctuation trimming so
  // "see https://x.com." links only the URL and trailing "." stays plain text.
  let href = part.replace(/[.,;:!?'"]+$/, "");
  const trailingPunct = part.slice(href.length);
  const opens = (href.match(/\(/g) || []).length;
  const closes = (href.match(/\)/g) || []).length;
  let extraTrailing = "";
  if (closes > opens) {
    extraTrailing = ")";
    href = href.slice(0, -1);
  }
  return { href, display: href, trailing: extraTrailing + trailingPunct };
}

interface ContentLinksProps {
  content: string;
}

// Renders post content with clickable, accent-colored hashtags, user
// mentions, and external URLs. Shared by FeedPost (display) and
// ComposeContent (live highlight while typing).
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
              onAuxClick={(event) => event.stopPropagation()}
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
              onAuxClick={(event) => event.stopPropagation()}
              className="text-blue-600 dark:text-blue-400 hover:underline"
            >
              {part}
            </Link>
          );
        }
        if (URL_TOKEN.test(part)) {
          const { href, display, trailing } = splitUrlToken(part);
          return (
            <React.Fragment key={`${part}-${index}`}>
              <a
                href={href}
                target="_blank"
                rel="noopener noreferrer"
                onClick={(event) => event.stopPropagation()}
                onAuxClick={(event) => event.stopPropagation()}
                className="text-blue-600 dark:text-blue-400 hover:underline underline"
              >
                {display}
              </a>
              {trailing}
            </React.Fragment>
          );
        }
        return <React.Fragment key={`${part}-${index}`}>{part}</React.Fragment>;
      })}
    </>
  );
};

export default ContentLinks;