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

CREATE UNIQUE INDEX jwt_signing_keys_active_token_type_idx
ON jwt_signing_keys(token_type)
WHERE state = 'active';

CREATE TRIGGER jwt_signing_keys_set_updated_at
BEFORE UPDATE ON jwt_signing_keys
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
