-- +goose Up

-- Login takes only {email, password}, so an email must identify exactly one live user.
CREATE UNIQUE INDEX ux_users_email_active ON users (lower(email)) WHERE deleted_at IS NULL;

-- Login runs before the tenant is known, and RLS hides every user from the app role
-- until it is. This is the one narrow read across tenants: it returns only the ids,
-- and everything else (hash, lockout, writes) happens under WithTenant.
-- +goose StatementBegin
CREATE FUNCTION auth_login_candidate(p_email text)
RETURNS TABLE (user_id uuid, tenant_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
    SELECT u.id, u.tenant_id
    FROM users u
    JOIN tenants t ON t.id = u.tenant_id
    WHERE lower(u.email) = lower(btrim(p_email))
      AND u.deleted_at IS NULL
      AND t.deleted_at IS NULL
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION auth_login_candidate(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION auth_login_candidate(text) TO pharmachat_app;

-- +goose Down
DROP FUNCTION IF EXISTS auth_login_candidate(text);
DROP INDEX IF EXISTS ux_users_email_active;
