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

-- ==========================================================================
-- Game schema
-- See project/reference/{game,world,empire,units,name-normalization}-model.md
-- Conventions: BIGINT ids (UUID for account_id FK); CHECK-based enums;
-- composite FKs carry game_id to prevent cross-game references (see A1 in
-- project/reconciliation-notes.md); UNIQUE (lower(name)) functional indexes
-- enforce case-insensitive name uniqueness per name-normalization.md.
-- ==========================================================================

CREATE TABLE games (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'setup',
    current_turn INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT games_status_check CHECK (status IN ('setup', 'active', 'completed')),
    CONSTRAINT games_current_turn_check CHECK (current_turn >= 0)
);

CREATE TRIGGER games_set_updated_at
BEFORE UPDATE ON games
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- Global catalogs (no game scope)

CREATE TABLE vessel_types (
    code TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    category TEXT NOT NULL,
    movement_points INT NOT NULL DEFAULT 0,
    cargo_capacity INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT vessel_types_category_check CHECK (category IN ('ship', 'colony')),
    CONSTRAINT vessel_types_movement_points_check CHECK (movement_points >= 0),
    CONSTRAINT vessel_types_cargo_capacity_check CHECK (cargo_capacity >= 0)
);

CREATE TABLE units (
    code TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    category TEXT,
    source TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT units_source_check CHECK (source IN ('mined', 'farmed', 'factory'))
);

CREATE TABLE unit_recipes (
    unit_code TEXT NOT NULL REFERENCES units(code) ON DELETE CASCADE,
    input_code TEXT NOT NULL REFERENCES units(code) ON DELETE RESTRICT,
    quantity INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (unit_code, input_code),
    CONSTRAINT unit_recipes_quantity_check CHECK (quantity > 0)
);

-- World (game-scoped)

CREATE TABLE star_systems (
    id BIGSERIAL PRIMARY KEY,
    game_id BIGINT NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    x INT NOT NULL,
    y INT NOT NULL,
    is_home_system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT star_systems_game_id_key UNIQUE (game_id, id),
    CONSTRAINT star_systems_game_coord_key UNIQUE (game_id, x, y)
);

CREATE TABLE jump_routes (
    id BIGSERIAL PRIMARY KEY,
    game_id BIGINT NOT NULL,
    system_a_id BIGINT NOT NULL,
    system_b_id BIGINT NOT NULL,
    cost INT NOT NULL,
    last_turn_used INT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT jump_routes_game_id_key UNIQUE (game_id, id),
    CONSTRAINT jump_routes_game_endpoints_key UNIQUE (game_id, system_a_id, system_b_id),
    CONSTRAINT jump_routes_ordering_check CHECK (system_a_id < system_b_id),
    CONSTRAINT jump_routes_cost_check CHECK (cost > 0),
    CONSTRAINT jump_routes_system_a_fk FOREIGN KEY (game_id, system_a_id) REFERENCES star_systems(game_id, id) ON DELETE CASCADE,
    CONSTRAINT jump_routes_system_b_fk FOREIGN KEY (game_id, system_b_id) REFERENCES star_systems(game_id, id) ON DELETE CASCADE
);

CREATE TABLE planets (
    id BIGSERIAL PRIMARY KEY,
    game_id BIGINT NOT NULL,
    system_id BIGINT NOT NULL,
    orbit INT NOT NULL,
    planet_type TEXT NOT NULL,
    lsn INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT planets_game_id_key UNIQUE (game_id, id),
    CONSTRAINT planets_game_system_orbit_key UNIQUE (game_id, system_id, orbit),
    CONSTRAINT planets_planet_type_check CHECK (planet_type IN ('rocky', 'gas giant', 'asteroid belt')),
    CONSTRAINT planets_lsn_check CHECK (lsn BETWEEN 0 AND 100),
    CONSTRAINT planets_orbit_check CHECK (orbit > 0),
    CONSTRAINT planets_system_fk FOREIGN KEY (game_id, system_id) REFERENCES star_systems(game_id, id) ON DELETE CASCADE
);

CREATE TABLE home_worlds (
    game_id BIGINT NOT NULL,
    planet_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (game_id, planet_id),
    CONSTRAINT home_worlds_planet_fk FOREIGN KEY (game_id, planet_id) REFERENCES planets(game_id, id) ON DELETE CASCADE
);

CREATE TABLE natural_resources (
    id BIGSERIAL PRIMARY KEY,
    game_id BIGINT NOT NULL,
    planet_id BIGINT NOT NULL,
    resource_type TEXT NOT NULL,
    capacity INT NOT NULL,
    base_extraction INT NOT NULL,
    yield_percent INT NOT NULL,
    reserves BIGINT NOT NULL DEFAULT 0,
    is_infinite BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT natural_resources_game_id_key UNIQUE (game_id, id),
    CONSTRAINT natural_resources_resource_type_check CHECK (resource_type IN ('ore', 'energy', 'gold', 'materials', 'farmland')),
    CONSTRAINT natural_resources_capacity_check CHECK (capacity > 0),
    CONSTRAINT natural_resources_base_extraction_check CHECK (base_extraction >= 0),
    CONSTRAINT natural_resources_yield_percent_check CHECK (yield_percent BETWEEN 1 AND 100),
    CONSTRAINT natural_resources_reserves_check CHECK (reserves >= 0),
    CONSTRAINT natural_resources_planet_fk FOREIGN KEY (game_id, planet_id) REFERENCES planets(game_id, id) ON DELETE CASCADE
);

-- Empire layer

CREATE TABLE agents (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX agents_name_lower_key ON agents(lower(name));

CREATE TRIGGER agents_set_updated_at
BEFORE UPDATE ON agents
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TABLE empires (
    id BIGSERIAL PRIMARY KEY,
    game_id BIGINT NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT empires_game_id_key UNIQUE (game_id, id)
);

CREATE UNIQUE INDEX empires_game_name_lower_key ON empires(game_id, lower(name));

CREATE TRIGGER empires_set_updated_at
BEFORE UPDATE ON empires
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TABLE players (
    id BIGSERIAL PRIMARY KEY,
    game_id BIGINT NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    account_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    is_gm BOOLEAN NOT NULL DEFAULT FALSE,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT players_game_id_key UNIQUE (game_id, id),
    CONSTRAINT players_game_account_key UNIQUE (game_id, account_id),
    CONSTRAINT players_status_check CHECK (status IN ('active', 'resigned', 'eliminated'))
);

CREATE TRIGGER players_set_updated_at
BEFORE UPDATE ON players
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TABLE empire_control (
    empire_id BIGINT PRIMARY KEY,
    game_id BIGINT NOT NULL,
    player_id BIGINT,
    agent_id BIGINT,
    gm_set BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT empire_control_empire_fk FOREIGN KEY (game_id, empire_id) REFERENCES empires(game_id, id) ON DELETE CASCADE,
    CONSTRAINT empire_control_player_fk FOREIGN KEY (game_id, player_id) REFERENCES players(game_id, id) ON DELETE SET NULL (player_id),
    CONSTRAINT empire_control_agent_fk FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE SET NULL,
    CONSTRAINT empire_control_gm_set_check CHECK (agent_id IS NOT NULL OR gm_set = FALSE)
);

CREATE UNIQUE INDEX empire_control_player_id_key ON empire_control(player_id) WHERE player_id IS NOT NULL;

CREATE TRIGGER empire_control_set_updated_at
BEFORE UPDATE ON empire_control
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- Per-empire world views

CREATE TABLE empire_system_names (
    game_id BIGINT NOT NULL,
    empire_id BIGINT NOT NULL,
    system_id BIGINT NOT NULL,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (empire_id, system_id),
    CONSTRAINT empire_system_names_empire_fk FOREIGN KEY (game_id, empire_id) REFERENCES empires(game_id, id) ON DELETE CASCADE,
    CONSTRAINT empire_system_names_system_fk FOREIGN KEY (game_id, system_id) REFERENCES star_systems(game_id, id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX empire_system_names_empire_name_lower_key ON empire_system_names(empire_id, lower(name));

CREATE TABLE empire_planet_names (
    game_id BIGINT NOT NULL,
    empire_id BIGINT NOT NULL,
    planet_id BIGINT NOT NULL,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (empire_id, planet_id),
    CONSTRAINT empire_planet_names_empire_fk FOREIGN KEY (game_id, empire_id) REFERENCES empires(game_id, id) ON DELETE CASCADE,
    CONSTRAINT empire_planet_names_planet_fk FOREIGN KEY (game_id, planet_id) REFERENCES planets(game_id, id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX empire_planet_names_empire_name_lower_key ON empire_planet_names(empire_id, lower(name));

CREATE TABLE empire_jump_point_knowledge (
    game_id BIGINT NOT NULL,
    empire_id BIGINT NOT NULL,
    route_id BIGINT NOT NULL,
    system_id BIGINT NOT NULL,
    detected BOOLEAN NOT NULL DEFAULT FALSE,
    range_band TEXT,
    destination_known BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (empire_id, route_id, system_id),
    CONSTRAINT empire_jump_point_knowledge_empire_fk FOREIGN KEY (game_id, empire_id) REFERENCES empires(game_id, id) ON DELETE CASCADE,
    CONSTRAINT empire_jump_point_knowledge_route_fk FOREIGN KEY (game_id, route_id) REFERENCES jump_routes(game_id, id) ON DELETE CASCADE,
    CONSTRAINT empire_jump_point_knowledge_system_fk FOREIGN KEY (game_id, system_id) REFERENCES star_systems(game_id, id) ON DELETE CASCADE
);

-- Vessels and inventory

CREATE TABLE vessels (
    id BIGSERIAL PRIMARY KEY,
    game_id BIGINT NOT NULL,
    empire_id BIGINT NOT NULL,
    vessel_type_code TEXT NOT NULL REFERENCES vessel_types(code) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    tech_level INT NOT NULL,
    planet_id BIGINT,
    system_id BIGINT,
    docked_at_vessel_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT vessels_game_id_key UNIQUE (game_id, id),
    CONSTRAINT vessels_status_check CHECK (status IN ('active', 'abandoned', 'destroyed')),
    CONSTRAINT vessels_tech_level_check CHECK (tech_level BETWEEN 0 AND 10),
    CONSTRAINT vessels_location_xor_check CHECK (
        (CASE WHEN planet_id IS NOT NULL THEN 1 ELSE 0 END) +
        (CASE WHEN system_id IS NOT NULL THEN 1 ELSE 0 END) +
        (CASE WHEN docked_at_vessel_id IS NOT NULL THEN 1 ELSE 0 END) = 1
    ),
    CONSTRAINT vessels_empire_fk FOREIGN KEY (game_id, empire_id) REFERENCES empires(game_id, id) ON DELETE CASCADE,
    CONSTRAINT vessels_planet_fk FOREIGN KEY (game_id, planet_id) REFERENCES planets(game_id, id) ON DELETE RESTRICT,
    CONSTRAINT vessels_system_fk FOREIGN KEY (game_id, system_id) REFERENCES star_systems(game_id, id) ON DELETE RESTRICT,
    CONSTRAINT vessels_docked_fk FOREIGN KEY (game_id, docked_at_vessel_id) REFERENCES vessels(game_id, id) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX vessels_game_empire_name_lower_key ON vessels(game_id, empire_id, lower(name));

CREATE TRIGGER vessels_set_updated_at
BEFORE UPDATE ON vessels
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TABLE vessel_inventory (
    game_id BIGINT NOT NULL,
    vessel_id BIGINT NOT NULL,
    unit_code TEXT NOT NULL REFERENCES units(code) ON DELETE RESTRICT,
    tech_level INT NOT NULL,
    quantity INT NOT NULL DEFAULT 0,
    active INT NOT NULL DEFAULT 0,
    cargo INT NOT NULL DEFAULT 0,
    mass INT NOT NULL DEFAULT 0,
    volume INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (game_id, vessel_id, unit_code, tech_level),
    CONSTRAINT vessel_inventory_tech_level_check CHECK (tech_level BETWEEN 0 AND 10),
    CONSTRAINT vessel_inventory_quantity_check CHECK (quantity >= 0),
    CONSTRAINT vessel_inventory_active_check CHECK (active >= 0 AND active <= quantity),
    CONSTRAINT vessel_inventory_cargo_check CHECK (cargo >= 0),
    CONSTRAINT vessel_inventory_mass_check CHECK (mass >= 0),
    CONSTRAINT vessel_inventory_volume_check CHECK (volume >= 0),
    CONSTRAINT vessel_inventory_vessel_fk FOREIGN KEY (game_id, vessel_id) REFERENCES vessels(game_id, id) ON DELETE CASCADE
);

-- Empire instance entities keyed on vessel

CREATE TABLE population_groups (
    game_id BIGINT NOT NULL,
    vessel_id BIGINT NOT NULL,
    group_type TEXT NOT NULL,
    count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (game_id, vessel_id, group_type),
    CONSTRAINT population_groups_group_type_check CHECK (group_type IN ('untrained', 'worker', 'manager', 'soldier', 'pilot')),
    CONSTRAINT population_groups_count_check CHECK (count >= 0),
    CONSTRAINT population_groups_vessel_fk FOREIGN KEY (game_id, vessel_id) REFERENCES vessels(game_id, id) ON DELETE CASCADE
);

CREATE TABLE training_queue (
    id BIGSERIAL PRIMARY KEY,
    game_id BIGINT NOT NULL,
    vessel_id BIGINT NOT NULL,
    from_group_type TEXT NOT NULL,
    to_group_type TEXT NOT NULL,
    count INT NOT NULL,
    start_turn INT NOT NULL,
    completion_turn INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT training_queue_game_id_key UNIQUE (game_id, id),
    CONSTRAINT training_queue_from_group_type_check CHECK (from_group_type IN ('untrained', 'worker', 'manager', 'soldier', 'pilot')),
    CONSTRAINT training_queue_to_group_type_check CHECK (to_group_type IN ('untrained', 'worker', 'manager', 'soldier', 'pilot')),
    CONSTRAINT training_queue_count_check CHECK (count > 0),
    CONSTRAINT training_queue_turn_order_check CHECK (completion_turn >= start_turn),
    CONSTRAINT training_queue_vessel_fk FOREIGN KEY (game_id, vessel_id) REFERENCES vessels(game_id, id) ON DELETE CASCADE
);

CREATE TABLE mining_groups (
    game_id BIGINT NOT NULL,
    vessel_id BIGINT NOT NULL,
    resource_id BIGINT NOT NULL,
    group_no INT NOT NULL,
    unit_code TEXT NOT NULL REFERENCES units(code) ON DELETE RESTRICT,
    tech_level INT NOT NULL,
    quantity INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (game_id, vessel_id, resource_id, group_no, unit_code, tech_level),
    CONSTRAINT mining_groups_tech_level_check CHECK (tech_level BETWEEN 0 AND 10),
    CONSTRAINT mining_groups_quantity_check CHECK (quantity > 0),
    CONSTRAINT mining_groups_vessel_fk FOREIGN KEY (game_id, vessel_id) REFERENCES vessels(game_id, id) ON DELETE CASCADE,
    CONSTRAINT mining_groups_resource_fk FOREIGN KEY (game_id, resource_id) REFERENCES natural_resources(game_id, id) ON DELETE CASCADE,
    CONSTRAINT mining_groups_inventory_fk FOREIGN KEY (game_id, vessel_id, unit_code, tech_level) REFERENCES vessel_inventory(game_id, vessel_id, unit_code, tech_level) ON DELETE CASCADE
);

CREATE TABLE farming_groups (
    game_id BIGINT NOT NULL,
    vessel_id BIGINT NOT NULL,
    resource_id BIGINT NOT NULL,
    group_no INT NOT NULL,
    unit_code TEXT NOT NULL REFERENCES units(code) ON DELETE RESTRICT,
    tech_level INT NOT NULL,
    quantity INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (game_id, vessel_id, resource_id, group_no, unit_code, tech_level),
    CONSTRAINT farming_groups_tech_level_check CHECK (tech_level BETWEEN 0 AND 10),
    CONSTRAINT farming_groups_quantity_check CHECK (quantity > 0),
    CONSTRAINT farming_groups_vessel_fk FOREIGN KEY (game_id, vessel_id) REFERENCES vessels(game_id, id) ON DELETE CASCADE,
    CONSTRAINT farming_groups_resource_fk FOREIGN KEY (game_id, resource_id) REFERENCES natural_resources(game_id, id) ON DELETE CASCADE,
    CONSTRAINT farming_groups_inventory_fk FOREIGN KEY (game_id, vessel_id, unit_code, tech_level) REFERENCES vessel_inventory(game_id, vessel_id, unit_code, tech_level) ON DELETE CASCADE
);

CREATE TABLE factory_groups (
    game_id BIGINT NOT NULL,
    vessel_id BIGINT NOT NULL,
    group_no INT NOT NULL,
    unit_code TEXT NOT NULL REFERENCES units(code) ON DELETE RESTRICT,
    tech_level INT NOT NULL,
    quantity INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (game_id, vessel_id, group_no, unit_code, tech_level),
    CONSTRAINT factory_groups_tech_level_check CHECK (tech_level BETWEEN 0 AND 10),
    CONSTRAINT factory_groups_quantity_check CHECK (quantity > 0),
    CONSTRAINT factory_groups_vessel_fk FOREIGN KEY (game_id, vessel_id) REFERENCES vessels(game_id, id) ON DELETE CASCADE,
    CONSTRAINT factory_groups_inventory_fk FOREIGN KEY (game_id, vessel_id, unit_code, tech_level) REFERENCES vessel_inventory(game_id, vessel_id, unit_code, tech_level) ON DELETE CASCADE
);

INSERT INTO roles (name, description)
VALUES
    ('user', 'Authenticated user'),
    ('admin', 'Administrator'),
    ('tester', 'Seeded test account');
