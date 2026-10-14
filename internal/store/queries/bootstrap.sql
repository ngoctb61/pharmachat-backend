-- Bootstrap queries used by pcctl. They run as the schema owner, outside RLS.

-- name: UpsertTenant :one
INSERT INTO tenants (slug, display_name)
VALUES (@slug, @display_name)
ON CONFLICT (slug) DO UPDATE SET display_name = EXCLUDED.display_name
RETURNING id;

-- name: GetTenantIDBySlug :one
SELECT id FROM tenants WHERE slug = @slug AND deleted_at IS NULL;

-- name: UpsertRole :one
INSERT INTO roles (tenant_id, name, description, is_system)
VALUES (@tenant_id, @name, @description, true)
ON CONFLICT (tenant_id, name) DO UPDATE SET description = EXCLUDED.description
RETURNING id;

-- name: GetRoleIDByName :one
SELECT id FROM roles WHERE tenant_id = @tenant_id AND name = @name;

-- name: GrantRolePermissions :execrows
INSERT INTO role_permissions (tenant_id, role_id, permission_id)
SELECT @tenant_id, @role_id, p.id FROM permissions p WHERE p.code = ANY(@codes::text[])
ON CONFLICT DO NOTHING;

-- name: CountPermissions :one
SELECT count(*) FROM permissions WHERE code = ANY(@codes::text[]);

-- name: CreateUser :one
INSERT INTO users (tenant_id, display_name, email, password_hash)
VALUES (@tenant_id, @display_name, @email, @password_hash)
RETURNING id;

-- name: AssignUserRole :exec
INSERT INTO user_roles (tenant_id, user_id, role_id)
VALUES (@tenant_id, @user_id, @role_id)
ON CONFLICT DO NOTHING;

-- name: HasActiveLLMConfig :one
SELECT EXISTS (SELECT 1 FROM llm_configs WHERE tenant_id = @tenant_id AND is_active);

-- name: InsertLLMConfig :exec
INSERT INTO llm_configs (tenant_id, provider, model_id, api_key_encrypted, input_usd_per_mtok, output_usd_per_mtok)
VALUES (@tenant_id, @provider, @model_id, @api_key_encrypted, @input_usd_per_mtok, @output_usd_per_mtok);

-- name: HasActiveEmbeddingConfig :one
SELECT EXISTS (SELECT 1 FROM embedding_configs WHERE tenant_id = @tenant_id AND is_active);

-- name: InsertEmbeddingConfig :exec
INSERT INTO embedding_configs (tenant_id, provider, model_id, api_key_encrypted)
VALUES (@tenant_id, @provider, @model_id, @api_key_encrypted);
