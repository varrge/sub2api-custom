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
Generate the ACL from `redis.acl.template`, replacing both placeholders with
SHA-256 password hashes. `TIME` must remain explicitly allowed after category
restrictions: the scheduler and Lua lock scripts use the Redis server clock.
Validate both direct `TIME` and `EVAL "return redis.call('TIME')" 0`, along with
login/recharge and active gateway traffic. Check ACL LOG for business denials;
successful login alone does not exercise the scheduling path.

Restrict admin paths on every public alias and direct-origin address. Preserve
public model, payment callback and user endpoints. Access logs must omit all
request/response headers, query secrets and request bodies; retain trusted client
IP, socket peer, path, status and correlation ID. Store copies off the app host.

Before enforcement, inventory upstream/proxy, DNS, SMTP, object storage and OAuth
destinations. Apply egress controls to both address families. Do not assume an
arbitrary HTTPS destination is safe merely because it uses port 443.

Credentials belong in a local mode-0600 handover file or password manager, never
in this repository. Record applied changes separately from prepared changes.

## Host archive and firewall scripts

`archive-send.py` runs as a host maintenance job, not inside the app. Install its
root-only configuration as `/etc/sub2api-security/archive.json`, containing
`certificate`, `ssh_key`, `known_hosts`, `port`, and `destination`. It uses a
public RSA recipient certificate and authenticated AES-256-GCM CMS encryption;
keep the encrypted private key and its password off the application host.
The persistent outbox advances its cursor only after an acknowledged transfer.
Audit windows overlap by 15 minutes to cover asynchronous inserts; consumers
deduplicate audit rows by ID and access records by correlation ID/time.
Log collection intentionally excludes request bodies, credentials and audit
extras. Caddy retention still bounds recovery after a prolonged outage.

The receiver SSH key must have `restrict` and a forced `archive-receive.py`
command in a root-owned authorized_keys. Only a `backup` or `logs` upload with
its SHA-256 is accepted; retries are idempotent. Install a root-owned
`/etc/sub2api-archive.json` with `require_mount: true`, `max_storage_bytes` and
`min_free_bytes`. Use a dedicated filesystem mounted at
`/var/lib/sub2api-archive`, owned by the restricted account. The receiver never
provides read/delete/command access. Root runs `archive-retention.py` separately:
backup retention is three days with at least two distinct restore points;
logs are retained seven days. Check that the quota accommodates this workload.
Sender jobs require bounded systemd timeouts and monitored failure status;
do not interpret an enabled timer as a successful backup.

`egress-policy.py` reads root-only `/etc/sub2api-security/egress.json` with
`app_addresses`, `database_addresses` and `redis_addresses`. These must match
fixed Compose IP addresses. Install it before Docker starts and make Docker
require the successful oneshot service so reboot does not open an unfiltered
startup window. The baseline permits DB/Redis destinations and public TCP
80/443/465/587, denies host/private-network access and Docker-forwarded IPv6.
It **does not** restrict HTTPS destinations; a separate destination policy
is required to prevent arbitrary HTTPS exfiltration.

Run the offline boundary checks without host privileges:

```sh
python3 deploy/security/tests/check_sender_failures.py deploy/security/archive-send.py
python3 deploy/security/tests/check_archive_bounds.py deploy/security/archive-receive.py deploy/security/archive-retention.py
python3 deploy/security/tests/check_egress.py deploy/security/egress-policy.py
```
