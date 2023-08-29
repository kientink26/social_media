CREATE TABLE post_subscriptions (
    user_id UUID NOT NULL REFERENCES users ON DELETE CASCADE,
    post_id UUID NOT NULL REFERENCES posts ON DELETE CASCADE,
    PRIMARY KEY (user_id, post_id)
);