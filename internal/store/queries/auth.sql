-- Login queries. LoginCandidate runs on the app pool before the tenant is known;
-- every other query runs inside db.WithTenant.

-- name: LoginCandidate :one
SELECT c.user_id::uuid AS user_id, c.tenant_id::uuid AS tenant_id
FROM auth_login_candidate(@email::text) c;

-- name: GetUserForLogin :one
SELECT u.id, u.tenant_id, u.display_name, u.email, u.password_hash, u.is_active,
       t.is_active AS tenant_is_active,
       (u.lockout_end IS NOT NULL AND u.lockout_end > now())::boolean AS locked
FROM users u
JOIN tenants t ON t.id = u.tenant_id
WHERE u.id = @id AND u.deleted_at IS NULL AND t.deleted_at IS NULL;

-- name: ListUserRoleNames :many
SELECT r.name
FROM user_roles ur
JOIN roles r ON r.id = ur.role_id
WHERE ur.user_id = @user_id
ORDER BY r.name;

-- name: RecordLoginFailure :one
-- Both writes re-check the lock, not only the earlier read, so a request still
-- hashing when the account got locked can neither count nor clear it.
UPDATE users SET
    access_failed_count = CASE WHEN access_failed_count + 1 >= @max_attempts::int THEN 0
                               ELSE access_failed_count + 1 END,
    lockout_end = CASE WHEN access_failed_count + 1 >= @max_attempts::int
                       THEN now() + make_interval(secs => @lockout_seconds::int)
                       ELSE lockout_end END
WHERE id = @id AND (lockout_end IS NULL OR lockout_end <= now())
RETURNING (lockout_end IS NOT NULL AND lockout_end > now())::boolean AS locked;

-- name: RecordLoginSuccess :execrows
UPDATE users SET access_failed_count = 0, lockout_end = NULL, last_login_at = now()
WHERE id = @id AND (lockout_end IS NULL OR lockout_end <= now());
