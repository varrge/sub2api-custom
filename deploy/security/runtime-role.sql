-- Run with psql as the database owner; role passwords are set separately.
-- Assumes the existing maintenance owner is sub2api and the DB is sub2api.
DO $$
BEGIN
  IF EXISTS (
    SELECT FROM pg_auth_members m JOIN pg_roles r ON r.oid=m.member
    WHERE r.rolname='sub2api_runtime'
  ) OR EXISTS (
    SELECT FROM pg_class c JOIN pg_roles r ON r.oid=c.relowner
    WHERE r.rolname='sub2api_runtime'
  ) OR EXISTS (
    SELECT FROM pg_namespace n JOIN pg_roles r ON r.oid=n.nspowner
    WHERE r.rolname='sub2api_runtime'
  ) OR EXISTS (
    SELECT FROM pg_database d JOIN pg_roles r ON r.oid=d.datdba
    WHERE r.rolname='sub2api_runtime'
  ) THEN
    RAISE EXCEPTION 'Existing runtime role owns objects or inherits roles; audit it before applying';
  END IF;
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'sub2api_runtime') THEN
    CREATE ROLE sub2api_runtime LOGIN;
  END IF;
END
$$;
ALTER ROLE sub2api_runtime NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
REVOKE ALL ON DATABASE sub2api FROM sub2api_runtime;
REVOKE ALL ON DATABASE sub2api FROM PUBLIC;
GRANT CONNECT ON DATABASE sub2api TO sub2api_runtime;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON SCHEMA public FROM sub2api_runtime;
GRANT USAGE ON SCHEMA public TO sub2api_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO sub2api_runtime;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO sub2api_runtime;
ALTER DEFAULT PRIVILEGES FOR ROLE sub2api IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO sub2api_runtime;
ALTER DEFAULT PRIVILEGES FOR ROLE sub2api IN SCHEMA public
  GRANT USAGE, SELECT ON SEQUENCES TO sub2api_runtime;
REVOKE ALL ON schema_migrations FROM sub2api_runtime;
GRANT SELECT ON schema_migrations TO sub2api_runtime;
