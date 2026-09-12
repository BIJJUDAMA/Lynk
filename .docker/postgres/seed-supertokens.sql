-- Direct seed script for supertokens_db test accounts
-- Execute with: psql -U lynk_user -d supertokens_db -f .docker/postgres/seed-supertokens.sql

INSERT INTO roles (app_id, role)
VALUES ('public', 'member')
ON CONFLICT DO NOTHING;

INSERT INTO app_id_to_user_id (app_id, user_id, recipe_id, primary_or_recipe_user_id, is_linked_or_is_a_primary_user)
VALUES
    ('public', 'a2be7abb-a253-4b8d-b3c3-5a5f39416239', 'emailpassword', 'a2be7abb-a253-4b8d-b3c3-5a5f39416239', false),
    ('public', '2e961701-96a1-43ef-9201-f9720bdef3d5', 'emailpassword', '2e961701-96a1-43ef-9201-f9720bdef3d5', false),
    ('public', 'e4264e59-8290-4a3f-91fe-4826d9165c89', 'emailpassword', 'e4264e59-8290-4a3f-91fe-4826d9165c89', false)
ON CONFLICT DO NOTHING;

INSERT INTO all_auth_recipe_users (app_id, tenant_id, user_id, primary_or_recipe_user_id, is_linked_or_is_a_primary_user, recipe_id, time_joined, primary_or_recipe_user_time_joined)
VALUES
    ('public', 'public', 'a2be7abb-a253-4b8d-b3c3-5a5f39416239', 'a2be7abb-a253-4b8d-b3c3-5a5f39416239', false, 'emailpassword', 1789185755150, 1789185755150),
    ('public', 'public', '2e961701-96a1-43ef-9201-f9720bdef3d5', '2e961701-96a1-43ef-9201-f9720bdef3d5', false, 'emailpassword', 1789185755715, 1789185755715),
    ('public', 'public', 'e4264e59-8290-4a3f-91fe-4826d9165c89', 'e4264e59-8290-4a3f-91fe-4826d9165c89', false, 'emailpassword', 1789185756222, 1789185756222)
ON CONFLICT DO NOTHING;

INSERT INTO emailpassword_users (app_id, user_id, email, password_hash, time_joined)
VALUES
    ('public', 'a2be7abb-a253-4b8d-b3c3-5a5f39416239', 'poster@campus.edu', '$2a$11$v5JNboZL7mAk8.yaHSO9.usuxMoGQi7S0QhDQndqclF8ruXpIBIUK', 1789185755150),
    ('public', '2e961701-96a1-43ef-9201-f9720bdef3d5', 'applicant@campus.edu', '$2a$11$v5JNboZL7mAk8.yaHSO9.usuxMoGQi7S0QhDQndqclF8ruXpIBIUK', 1789185755715),
    ('public', 'e4264e59-8290-4a3f-91fe-4826d9165c89', 'unverified@campus.edu', '$2a$11$v5JNboZL7mAk8.yaHSO9.usuxMoGQi7S0QhDQndqclF8ruXpIBIUK', 1789185756222)
ON CONFLICT DO NOTHING;

INSERT INTO emailpassword_user_to_tenant (app_id, tenant_id, user_id, email)
VALUES
    ('public', 'public', 'a2be7abb-a253-4b8d-b3c3-5a5f39416239', 'poster@campus.edu'),
    ('public', 'public', '2e961701-96a1-43ef-9201-f9720bdef3d5', 'applicant@campus.edu'),
    ('public', 'public', 'e4264e59-8290-4a3f-91fe-4826d9165c89', 'unverified@campus.edu')
ON CONFLICT DO NOTHING;

INSERT INTO emailverification_verified_emails (app_id, user_id, email)
VALUES
    ('public', 'a2be7abb-a253-4b8d-b3c3-5a5f39416239', 'poster@campus.edu'),
    ('public', '2e961701-96a1-43ef-9201-f9720bdef3d5', 'applicant@campus.edu')
ON CONFLICT DO NOTHING;

INSERT INTO user_roles (app_id, tenant_id, user_id, role)
VALUES
    ('public', 'public', 'a2be7abb-a253-4b8d-b3c3-5a5f39416239', 'member'),
    ('public', 'public', '2e961701-96a1-43ef-9201-f9720bdef3d5', 'member'),
    ('public', 'public', 'e4264e59-8290-4a3f-91fe-4826d9165c89', 'member')
ON CONFLICT DO NOTHING;
