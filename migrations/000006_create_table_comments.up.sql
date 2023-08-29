CREATE TABLE comments (
    id UUID NOT NULL PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users ON DELETE CASCADE,
    post_id UUID NOT NULL REFERENCES posts ON DELETE CASCADE,
    content VARCHAR NOT NULL,
    likes_count INT NOT NULL DEFAULT 0 CHECK (likes_count >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sorted_comments ON comments (created_at DESC, id);