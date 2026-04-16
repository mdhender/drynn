CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE roles (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    handle TEXT NOT NULL,
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT users_handle_key UNIQUE (handle),
    CONSTRAINT users_email_key UNIQUE (email),
    CONSTRAINT users_handle_lowercase CHECK (handle = lower(handle)),
    CONSTRAINT users_email_lowercase CHECK (email = lower(email)),
    CONSTRAINT users_handle_format CHECK (handle ~ '^[a-z0-9_]+$'),
    CONSTRAINT users_handle_length CHECK (char_length(handle) BETWEEN 3 AND 32),
    CONSTRAINT users_email_length CHECK (char_length(email) BETWEEN 3 AND 320)
);

CREATE TABLE jwt_signing_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_type TEXT NOT NULL,
    algorithm TEXT NOT NULL DEFAULT 'HS256',
    secret BYTEA NOT NULL,
    state TEXT NOT NULL DEFAULT 'active',
    verify_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT jwt_signing_keys_token_type CHECK (token_type IN ('access', 'refresh')),
    CONSTRAINT jwt_signing_keys_algorithm CHECK (algorithm = 'HS256'),
    CONSTRAINT jwt_signing_keys_state CHECK (state IN ('active', 'retired', 'revoked'))
);

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, role_id)
);

CREATE UNIQUE INDEX jwt_signing_keys_active_token_type_idx
ON jwt_signing_keys(token_type)
WHERE state = 'active';

CREATE INDEX user_roles_role_id_idx ON user_roles(role_id);

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER users_set_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER jwt_signing_keys_set_updated_at
BEFORE UPDATE ON jwt_signing_keys
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TABLE invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL,
    code TEXT NOT NULL UNIQUE,
    invited_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    used_by UUID REFERENCES users(id) ON DELETE SET NULL,
    used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT invitations_email_lowercase CHECK (email = lower(email)),
    CONSTRAINT invitations_email_length CHECK (char_length(email) BETWEEN 3 AND 320)
);

CREATE INDEX invitations_code_idx ON invitations(code);
CREATE INDEX invitations_email_idx ON invitations(email);

CREATE TABLE password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX password_reset_tokens_code_idx ON password_reset_tokens(code);
CREATE INDEX password_reset_tokens_user_id_idx ON password_reset_tokens(user_id);

INSERT INTO roles (name, description)
VALUES
    ('user', 'Authenticated user'),
    ('admin', 'Administrator'),
    ('tester', 'Seeded test account');
