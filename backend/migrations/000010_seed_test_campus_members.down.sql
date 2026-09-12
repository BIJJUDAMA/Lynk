-- Remove seeded test member profiles
DELETE FROM profiles WHERE user_id IN (
    'a2be7abb-a253-4b8d-b3c3-5a5f39416239',
    '2e961701-96a1-43ef-9201-f9720bdef3d5',
    'e4264e59-8290-4a3f-91fe-4826d9165c89'
);

-- Remove seeded test member users
DELETE FROM users WHERE id IN (
    'a2be7abb-a253-4b8d-b3c3-5a5f39416239',
    '2e961701-96a1-43ef-9201-f9720bdef3d5',
    'e4264e59-8290-4a3f-91fe-4826d9165c89'
);

-- Remove seeded SuperTokens accounts if supertokens_db is accessible
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
            PERFORM dblink_exec('dbname=supertokens_db user=lynk_user password=lynk_password',
                'DELETE FROM user_roles WHERE user_id IN (''a2be7abb-a253-4b8d-b3c3-5a5f39416239'', ''2e961701-96a1-43ef-9201-f9720bdef3d5'', ''e4264e59-8290-4a3f-91fe-4826d9165c89'');
                 DELETE FROM emailverification_verified_emails WHERE user_id IN (''a2be7abb-a253-4b8d-b3c3-5a5f39416239'', ''2e961701-96a1-43ef-9201-f9720bdef3d5'', ''e4264e59-8290-4a3f-91fe-4826d9165c89'');
                 DELETE FROM emailpassword_user_to_tenant WHERE user_id IN (''a2be7abb-a253-4b8d-b3c3-5a5f39416239'', ''2e961701-96a1-43ef-9201-f9720bdef3d5'', ''e4264e59-8290-4a3f-91fe-4826d9165c89'');
                 DELETE FROM emailpassword_users WHERE user_id IN (''a2be7abb-a253-4b8d-b3c3-5a5f39416239'', ''2e961701-96a1-43ef-9201-f9720bdef3d5'', ''e4264e59-8290-4a3f-91fe-4826d9165c89'');
                 DELETE FROM all_auth_recipe_users WHERE user_id IN (''a2be7abb-a253-4b8d-b3c3-5a5f39416239'', ''2e961701-96a1-43ef-9201-f9720bdef3d5'', ''e4264e59-8290-4a3f-91fe-4826d9165c89'');
                 DELETE FROM app_id_to_user_id WHERE user_id IN (''a2be7abb-a253-4b8d-b3c3-5a5f39416239'', ''2e961701-96a1-43ef-9201-f9720bdef3d5'', ''e4264e59-8290-4a3f-91fe-4826d9165c89'');');
        END IF;
    END IF;
EXCEPTION
    WHEN OTHERS THEN
        RAISE NOTICE 'Skipping supertokens_db dblink cleanup: %', SQLERRM;
END $$;
