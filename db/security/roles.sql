\set ON_ERROR_STOP on

BEGIN;

DO $$
DECLARE
    service_role text;
BEGIN
    FOREACH service_role IN ARRAY ARRAY[
        'auth_app', 'profile_app', 'matching_app', 'chat_app', 'onlineconf_app', 'monitor'
    ]
    LOOP
        IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = service_role) THEN
            EXECUTE format(
                'CREATE ROLE %I LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS',
                service_role
            );
        END IF;
    END LOOP;
END;
$$;

COMMIT;
