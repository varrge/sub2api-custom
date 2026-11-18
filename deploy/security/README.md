# Hardened deployment

Production application releases use a fixed GitHub Actions image digest. Do not
replace a running container's binary through the web updater.

## Host-only controls

- `SUB2API_DISABLE_WEB_SYSTEM_MUTATIONS=true` disables HTTP update, rollback and
  restart, even for an administrator with a recent MFA grant.
- HTTP database restore is disabled by default. SQL input to `psql` is executable
  input, not inert data. Restore only in an isolated maintenance environment.
  `SUB2API_ALLOW_WEB_DATABASE_RESTORE=true` is an explicit host-level opt-in,
  still subject to the mandatory MFA check. Do not enable it in production.
- `DATABASE_SKIP_MIGRATIONS=true` verifies all embedded migration checksums without
  executing schema changes. Missing or changed migrations prevent startup.

Run each release's migrations with a separate maintenance credential and
`/app/sub2api --migrate-only` before starting that release with the runtime role.
This command exits without HTTP listeners, schedulers or gateway workers.
Keep maintenance credentials outside the app container and its mounted data.

The runtime role needs DML on business tables, sequence access, and SELECT on
`schema_migrations`. It must not own the database or schema, inherit a maintenance
role, or have SUPERUSER, CREATEDB, CREATEROLE, REPLICATION or BYPASSRLS. Configure
default privileges for future objects, then revoke runtime writes to migration
metadata. Test startup and billing before changing production credentials.

If `usage_logs` is partitioned, monthly partition creation/removal must also run
as an external maintenance job. The runtime role cannot create or drop partitions.
Check `pg_partitioned_table` before enabling this mode; an unpartitioned table
does not need this maintenance. Never grant schema ownership to the runtime role
to work around a failed partition job.

## Container and network policy

Run the app as UID/GID 1000, use `cap_drop: [ALL]`,
`security_opt: [no-new-privileges:true]`, a read-only root filesystem, and a
bounded `/tmp` tmpfs. Keep only `/app/data` writable. Override the image entrypoint
with the binary when its root initialization would otherwise need write access.

Redis must use named ACL users and a read-only ACL file. Disable the unauthenticated
default user. Restrict the app to the commands it uses, including Lua and
transactions, and exclude administrative commands, replication and module loading.
Use a separate maintenance user and non-root Redis process.

Restrict admin paths on every public alias and direct-origin address. Preserve
public model, payment callback and user endpoints. Access logs must omit all
request/response headers, query secrets and request bodies; retain trusted client
IP, socket peer, path, status and correlation ID. Store copies off the app host.

Before enforcement, inventory upstream/proxy, DNS, SMTP, object storage and OAuth
destinations. Apply egress controls to both address families. Do not assume an
arbitrary HTTPS destination is safe merely because it uses port 443.

Credentials belong in a local mode-0600 handover file or password manager, never
in this repository. Record applied changes separately from prepared changes.
