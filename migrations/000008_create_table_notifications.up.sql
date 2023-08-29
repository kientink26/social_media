CREATE TABLE notifications (
    id UUID NOT NULL PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users ON DELETE CASCADE,
    actors VARCHAR[] NOT NULL,
    type VARCHAR NOT NULL,
    post_id UUID REFERENCES posts ON DELETE CASCADE,
    read_at TIMESTAMPTZ,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sorted_notifications ON notifications(issued_at DESC, id);
CREATE UNIQUE INDEX unique_notifications ON notifications(user_id, type, post_id, read_at) NULLS NOT DISTINCT;