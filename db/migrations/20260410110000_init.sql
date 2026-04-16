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
    display_name TEXT NOT NULL DEFAULT '',
    bio TEXT NOT NULL DEFAULT '',
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

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, role_id)
);

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

INSERT INTO roles (name, description)
VALUES
    ('user', 'Authenticated user'),
    ('referee', 'Referee'),
    ('admin', 'Administrator');
