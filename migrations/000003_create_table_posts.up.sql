CREATE TABLE posts (
    id UUID NOT NULL PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users ON DELETE CASCADE,
    content VARCHAR NOT NULL,
    spoiler_of VARCHAR,
    nsfw BOOLEAN NOT NULL DEFAULT false,
    likes_count INT NOT NULL DEFAULT 0 CHECK (likes_count >= 0),
    comments_count INT NOT NULL DEFAULT 0 CHECK (comments_count >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sorted_posts ON posts (created_at DESC, id);