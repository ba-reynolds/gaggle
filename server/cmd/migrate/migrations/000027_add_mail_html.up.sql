-- Mail intake: preserve the raw text/html part for link-based verification
-- flows. Verification mailers (Reddit et al.) put the actionable URL ONLY in
-- the text/html part; stripped body text cannot recover it. Nullable so rows
-- written before this column existed (and plain-text-only mails) keep a NULL.
ALTER TABLE mail_messages ADD COLUMN html TEXT;
