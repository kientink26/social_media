CREATE TABLE users (
    id UUID NOT NULL PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR NOT NULL UNIQUE,
    username VARCHAR NOT NULL UNIQUE,
    avatar VARCHAR,
    followers_count INT NOT NULL DEFAULT 0 CHECK (followers_count >= 0),
    followees_count INT NOT NULL DEFAULT 0 CHECK (followees_count >= 0)
);