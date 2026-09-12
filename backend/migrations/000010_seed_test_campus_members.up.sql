-- Seed default campus member test accounts in lynk_db
INSERT INTO users (id, email, role, created_at, updated_at)
VALUES
    ('a2be7abb-a253-4b8d-b3c3-5a5f39416239', 'poster@campus.edu', 'member', NOW(), NOW()),
    ('2e961701-96a1-43ef-9201-f9720bdef3d5', 'applicant@campus.edu', 'member', NOW(), NOW()),
    ('e4264e59-8290-4a3f-91fe-4826d9165c89', 'unverified@campus.edu', 'member', NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
    email = EXCLUDED.email,
    role = EXCLUDED.role,
    updated_at = NOW();

-- Seed unified campus member profiles in lynk_db
INSERT INTO profiles (
    user_id,
    first_name,
    last_name,
    bio,
    department,
    graduation_year,
    skills,
    portfolio_links,
    organization,
    organization_website,
    created_at,
    updated_at
)
VALUES
    (
        'a2be7abb-a253-4b8d-b3c3-5a5f39416239',
        'Campus',
        'Poster',
        'Campus peer job poster looking for talented student collaborators.',
        'Computer Science',
        2026,
        ARRAY['Project Management', 'UI/UX', 'Full Stack'],
        '[]'::jsonb,
        'Campus Lab',
        'https://campus.edu',
        NOW(),
        NOW()
    ),
    (
        '2e961701-96a1-43ef-9201-f9720bdef3d5',
        'Campus',
        'Applicant',
        'Campus peer student freelancer specializing in Go and Next.js development.',
        'Software Engineering',
        2027,
        ARRAY['Go', 'TypeScript', 'React', 'PostgreSQL'],
        '[]'::jsonb,
        '',
        '',
        NOW(),
        NOW()
    ),
    (
        'e4264e59-8290-4a3f-91fe-4826d9165c89',
        'New',
        'Student',
        'Fresh campus student exploring peer opportunities.',
        'Design',
        2028,
        ARRAY['Figma', 'Design'],
        '[]'::jsonb,
        '',
        '',
        NOW(),
        NOW()
    )
ON CONFLICT (user_id) DO UPDATE SET
    first_name = EXCLUDED.first_name,
    last_name = EXCLUDED.last_name,
    bio = EXCLUDED.bio,
    department = EXCLUDED.department,
    graduation_year = EXCLUDED.graduation_year,
    skills = EXCLUDED.skills,
    organization = EXCLUDED.organization,
    organization_website = EXCLUDED.organization_website,
    updated_at = NOW();

-- Seed SuperTokens Core database via dblink if available in local PostgreSQL cluster
DO $$
DECLARE
    st_db_exists BOOLEAN := FALSE;
    st_tbl_exists BOOLEAN := FALSE;
BEGIN
    SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'supertokens_db') INTO st_db_exists;
    IF st_db_exists THEN
        CREATE EXTENSION IF NOT EXISTS dblink;

        SELECT EXISTS (
            SELECT 1 FROM dblink('dbname=supertokens_db user=lynk_user password=lynk_password',
                                 'SELECT 1 FROM information_schema.tables WHERE table_schema=''public'' AND table_name=''app_id_to_user_id''')
            AS t(found int)
        ) INTO st_tbl_exists;

        IF st_tbl_exists THEN
            -- Roles
            PERFORM dblink_exec('dbname=supertokens_db user=lynk_user password=lynk_password',
                'INSERT INTO roles (app_id, role) VALUES (''public'', ''member'') ON CONFLICT DO NOTHING;');

            -- User mappings
            PERFORM dblink_exec('dbname=supertokens_db user=lynk_user password=lynk_password',
                'INSERT INTO app_id_to_user_id (app_id, user_id, recipe_id, primary_or_recipe_user_id, is_linked_or_is_a_primary_user) VALUES
                    (''public'', ''a2be7abb-a253-4b8d-b3c3-5a5f39416239'', ''emailpassword'', ''a2be7abb-a253-4b8d-b3c3-5a5f39416239'', false),
                    (''public'', ''2e961701-96a1-43ef-9201-f9720bdef3d5'', ''emailpassword'', ''2e961701-96a1-43ef-9201-f9720bdef3d5'', false),
                    (''public'', ''e4264e59-8290-4a3f-91fe-4826d9165c89'', ''emailpassword'', ''e4264e59-8290-4a3f-91fe-4826d9165c89'', false)
                 ON CONFLICT DO NOTHING;');

            -- Auth recipe users
            PERFORM dblink_exec('dbname=supertokens_db user=lynk_user password=lynk_password',
                'INSERT INTO all_auth_recipe_users (app_id, tenant_id, user_id, primary_or_recipe_user_id, is_linked_or_is_a_primary_user, recipe_id, time_joined, primary_or_recipe_user_time_joined) VALUES
                    (''public'', ''public'', ''a2be7abb-a253-4b8d-b3c3-5a5f39416239'', ''a2be7abb-a253-4b8d-b3c3-5a5f39416239'', false, ''emailpassword'', 1789185755150, 1789185755150),
                    (''public'', ''public'', ''2e961701-96a1-43ef-9201-f9720bdef3d5'', ''2e961701-96a1-43ef-9201-f9720bdef3d5'', false, ''emailpassword'', 1789185755715, 1789185755715),
                    (''public'', ''public'', ''e4264e59-8290-4a3f-91fe-4826d9165c89'', ''e4264e59-8290-4a3f-91fe-4826d9165c89'', false, ''emailpassword'', 1789185756222, 1789185756222)
                 ON CONFLICT DO NOTHING;');

            -- EmailPassword credentials (password: password123)
            PERFORM dblink_exec('dbname=supertokens_db user=lynk_user password=lynk_password',
                'INSERT INTO emailpassword_users (app_id, user_id, email, password_hash, time_joined) VALUES
                    (''public'', ''a2be7abb-a253-4b8d-b3c3-5a5f39416239'', ''poster@campus.edu'', ''$2a$11$v5JNboZL7mAk8.yaHSO9.usuxMoGQi7S0QhDQndqclF8ruXpIBIUK'', 1789185755150),
                    (''public'', ''2e961701-96a1-43ef-9201-f9720bdef3d5'', ''applicant@campus.edu'', ''$2a$11$v5JNboZL7mAk8.yaHSO9.usuxMoGQi7S0QhDQndqclF8ruXpIBIUK'', 1789185755715),
                    (''public'', ''e4264e59-8290-4a3f-91fe-4826d9165c89'', ''unverified@campus.edu'', ''$2a$11$v5JNboZL7mAk8.yaHSO9.usuxMoGQi7S0QhDQndqclF8ruXpIBIUK'', 1789185756222)
                 ON CONFLICT DO NOTHING;');

            -- Tenant associations
            PERFORM dblink_exec('dbname=supertokens_db user=lynk_user password=lynk_password',
                'INSERT INTO emailpassword_user_to_tenant (app_id, tenant_id, user_id, email) VALUES
                    (''public'', ''public'', ''a2be7abb-a253-4b8d-b3c3-5a5f39416239'', ''poster@campus.edu''),
                    (''public'', ''public'', ''2e961701-96a1-43ef-9201-f9720bdef3d5'', ''applicant@campus.edu''),
                    (''public'', ''public'', ''e4264e59-8290-4a3f-91fe-4826d9165c89'', ''unverified@campus.edu'')
                 ON CONFLICT DO NOTHING;');

            -- Verified emails (poster and applicant verified; unverified left unverified)
            PERFORM dblink_exec('dbname=supertokens_db user=lynk_user password=lynk_password',
                'INSERT INTO emailverification_verified_emails (app_id, user_id, email) VALUES
                    (''public'', ''a2be7abb-a253-4b8d-b3c3-5a5f39416239'', ''poster@campus.edu''),
                    (''public'', ''2e961701-96a1-43ef-9201-f9720bdef3d5'', ''applicant@campus.edu'')
                 ON CONFLICT DO NOTHING;');

            -- User roles
            PERFORM dblink_exec('dbname=supertokens_db user=lynk_user password=lynk_password',
                'INSERT INTO user_roles (app_id, tenant_id, user_id, role) VALUES
                    (''public'', ''public'', ''a2be7abb-a253-4b8d-b3c3-5a5f39416239'', ''member''),
                    (''public'', ''public'', ''2e961701-96a1-43ef-9201-f9720bdef3d5'', ''member''),
                    (''public'', ''public'', ''e4264e59-8290-4a3f-91fe-4826d9165c89'', ''member'')
                 ON CONFLICT DO NOTHING;');
        END IF;
    END IF;
EXCEPTION
    WHEN OTHERS THEN
        RAISE NOTICE 'Skipping supertokens_db dblink seeding: %', SQLERRM;
END $$;
