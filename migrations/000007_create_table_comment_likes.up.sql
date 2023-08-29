CREATE TABLE comment_likes (
    user_id UUID NOT NULL REFERENCES users ON DELETE CASCADE,
    comment_id UUID NOT NULL REFERENCES comments ON DELETE CASCADE,
    PRIMARY KEY (user_id, comment_id)
);