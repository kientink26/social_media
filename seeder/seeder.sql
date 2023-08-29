DELETE FROM users;

INSERT INTO users (id, email, username) VALUES
    ('24ca6ce6-b3e9-4276-a99a-45c77115cc9f', 'john@example.com', 'john'),
    ('93dfcef9-0b45-46ae-933c-ea52fbf80edb', 'jane@example.com', 'jane');

INSERT INTO users(email, username) VALUES
    ('alice@example.com', 'alice'),
    ('bob@example.com', 'bob'),
    ('rachel@example.com', 'rachel'),
    ('ted@example.com', 'ted');

INSERT INTO posts (id, user_id, content, comments_count) VALUES
    ('c592451b-fdd2-430d-8d49-e75f058c3dce', '24ca6ce6-b3e9-4276-a99a-45c77115cc9f', 'sample post', 1);

INSERT INTO post_subscriptions (user_id, post_id) VALUES
     ('24ca6ce6-b3e9-4276-a99a-45c77115cc9f', 'c592451b-fdd2-430d-8d49-e75f058c3dce');

INSERT INTO timeline (id, user_id, post_id) VALUES
    ('d7490258-1f2f-4a75-8fbb-1846ccde9543', '24ca6ce6-b3e9-4276-a99a-45c77115cc9f', 'c592451b-fdd2-430d-8d49-e75f058c3dce');

INSERT INTO comments (id, user_id, post_id, content) VALUES
    ('648e60bf-b0ab-42e6-8e48-10f797b19c49', '24ca6ce6-b3e9-4276-a99a-45c77115cc9f', 'c592451b-fdd2-430d-8d49-e75f058c3dce', 'sample comment');