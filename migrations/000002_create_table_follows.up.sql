CREATE TABLE follows (
    follower_id UUID NOT NULL REFERENCES users ON DELETE CASCADE,
    followee_id UUID NOT NULL REFERENCES users ON DELETE CASCADE,
    PRIMARY KEY (follower_id, followee_id)
);