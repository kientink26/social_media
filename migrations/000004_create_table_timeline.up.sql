CREATE TABLE timeline (
    id UUID NOT NULL PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users ON DELETE CASCADE,
    post_id UUID NOT NULL REFERENCES posts ON DELETE CASCADE
);

CREATE UNIQUE INDEX timeline_unique ON timeline (user_id, post_id);