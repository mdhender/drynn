-- Create "agents" table
CREATE TABLE "agents" (
  "id" bigserial NOT NULL,
  "name" text NOT NULL,
  "version" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "agents_name_lower_key" to table: "agents"
CREATE UNIQUE INDEX "agents_name_lower_key" ON "agents" ((lower(name)));
-- Create trigger "agents_set_updated_at"
CREATE TRIGGER "agents_set_updated_at" BEFORE UPDATE ON "agents" FOR EACH ROW EXECUTE FUNCTION "set_updated_at"();
-- Create "games" table
CREATE TABLE "games" (
  "id" bigserial NOT NULL,
  "name" text NOT NULL,
  "status" text NOT NULL DEFAULT 'setup',
  "current_turn" integer NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "games_current_turn_check" CHECK (current_turn >= 0),
  CONSTRAINT "games_status_check" CHECK (status = ANY (ARRAY['setup'::text, 'active'::text, 'completed'::text]))
);
-- Create "empires" table
CREATE TABLE "empires" (
  "id" bigserial NOT NULL,
  "game_id" bigint NOT NULL,
  "name" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "empires_game_id_key" UNIQUE ("game_id", "id"),
  CONSTRAINT "empires_game_id_fkey" FOREIGN KEY ("game_id") REFERENCES "games" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "empires_game_name_lower_key" to table: "empires"
CREATE UNIQUE INDEX "empires_game_name_lower_key" ON "empires" ("game_id", (lower(name)));
-- Create "players" table
CREATE TABLE "players" (
  "id" bigserial NOT NULL,
  "game_id" bigint NOT NULL,
  "account_id" uuid NOT NULL,
  "is_gm" boolean NOT NULL DEFAULT false,
  "status" text NOT NULL DEFAULT 'active',
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "players_game_account_key" UNIQUE ("game_id", "account_id"),
  CONSTRAINT "players_game_id_key" UNIQUE ("game_id", "id"),
  CONSTRAINT "players_account_id_fkey" FOREIGN KEY ("account_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "players_game_id_fkey" FOREIGN KEY ("game_id") REFERENCES "games" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "players_status_check" CHECK (status = ANY (ARRAY['active'::text, 'resigned'::text, 'eliminated'::text]))
);
-- Create "empire_control" table
CREATE TABLE "empire_control" (
  "empire_id" bigint NOT NULL,
  "game_id" bigint NOT NULL,
  "player_id" bigint NULL,
  "agent_id" bigint NULL,
  "gm_set" boolean NOT NULL DEFAULT false,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("empire_id"),
  CONSTRAINT "empire_control_agent_fk" FOREIGN KEY ("agent_id") REFERENCES "agents" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  CONSTRAINT "empire_control_empire_fk" FOREIGN KEY ("game_id", "empire_id") REFERENCES "empires" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "empire_control_player_fk" FOREIGN KEY ("game_id", "player_id") REFERENCES "players" ("game_id", "id") ON UPDATE NO ACTION ON DELETE SET NULL,
  CONSTRAINT "empire_control_gm_set_check" CHECK ((agent_id IS NOT NULL) OR (gm_set = false))
);
-- Create index "empire_control_player_id_key" to table: "empire_control"
CREATE UNIQUE INDEX "empire_control_player_id_key" ON "empire_control" ("player_id") WHERE (player_id IS NOT NULL);
-- Create trigger "empire_control_set_updated_at"
CREATE TRIGGER "empire_control_set_updated_at" BEFORE UPDATE ON "empire_control" FOR EACH ROW EXECUTE FUNCTION "set_updated_at"();
-- Create "star_systems" table
CREATE TABLE "star_systems" (
  "id" bigserial NOT NULL,
  "game_id" bigint NOT NULL,
  "x" integer NOT NULL,
  "y" integer NOT NULL,
  "is_home_system" boolean NOT NULL DEFAULT false,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "star_systems_game_coord_key" UNIQUE ("game_id", "x", "y"),
  CONSTRAINT "star_systems_game_id_key" UNIQUE ("game_id", "id"),
  CONSTRAINT "star_systems_game_id_fkey" FOREIGN KEY ("game_id") REFERENCES "games" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "planets" table
CREATE TABLE "planets" (
  "id" bigserial NOT NULL,
  "game_id" bigint NOT NULL,
  "system_id" bigint NOT NULL,
  "orbit" integer NOT NULL,
  "planet_type" text NOT NULL,
  "lsn" integer NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "planets_game_id_key" UNIQUE ("game_id", "id"),
  CONSTRAINT "planets_game_system_orbit_key" UNIQUE ("game_id", "system_id", "orbit"),
  CONSTRAINT "planets_system_fk" FOREIGN KEY ("game_id", "system_id") REFERENCES "star_systems" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "planets_lsn_check" CHECK ((lsn >= 0) AND (lsn <= 100)),
  CONSTRAINT "planets_orbit_check" CHECK (orbit > 0),
  CONSTRAINT "planets_planet_type_check" CHECK (planet_type = ANY (ARRAY['rocky'::text, 'gas giant'::text, 'asteroid belt'::text]))
);
-- Create "vessel_types" table
CREATE TABLE "vessel_types" (
  "code" text NOT NULL,
  "display_name" text NOT NULL,
  "category" text NOT NULL,
  "movement_points" integer NOT NULL DEFAULT 0,
  "cargo_capacity" integer NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("code"),
  CONSTRAINT "vessel_types_cargo_capacity_check" CHECK (cargo_capacity >= 0),
  CONSTRAINT "vessel_types_category_check" CHECK (category = ANY (ARRAY['ship'::text, 'colony'::text])),
  CONSTRAINT "vessel_types_movement_points_check" CHECK (movement_points >= 0)
);
-- Create "vessels" table
CREATE TABLE "vessels" (
  "id" bigserial NOT NULL,
  "game_id" bigint NOT NULL,
  "empire_id" bigint NOT NULL,
  "vessel_type_code" text NOT NULL,
  "name" text NOT NULL,
  "status" text NOT NULL DEFAULT 'active',
  "tech_level" integer NOT NULL,
  "planet_id" bigint NULL,
  "system_id" bigint NULL,
  "docked_at_vessel_id" bigint NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "vessels_game_id_key" UNIQUE ("game_id", "id"),
  CONSTRAINT "vessels_docked_fk" FOREIGN KEY ("game_id", "docked_at_vessel_id") REFERENCES "vessels" ("game_id", "id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "vessels_empire_fk" FOREIGN KEY ("game_id", "empire_id") REFERENCES "empires" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "vessels_planet_fk" FOREIGN KEY ("game_id", "planet_id") REFERENCES "planets" ("game_id", "id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "vessels_system_fk" FOREIGN KEY ("game_id", "system_id") REFERENCES "star_systems" ("game_id", "id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "vessels_vessel_type_code_fkey" FOREIGN KEY ("vessel_type_code") REFERENCES "vessel_types" ("code") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "vessels_location_xor_check" CHECK (((
CASE
    WHEN (planet_id IS NOT NULL) THEN 1
    ELSE 0
END +
CASE
    WHEN (system_id IS NOT NULL) THEN 1
    ELSE 0
END) +
CASE
    WHEN (docked_at_vessel_id IS NOT NULL) THEN 1
    ELSE 0
END) = 1),
  CONSTRAINT "vessels_status_check" CHECK (status = ANY (ARRAY['active'::text, 'abandoned'::text, 'destroyed'::text])),
  CONSTRAINT "vessels_tech_level_check" CHECK ((tech_level >= 0) AND (tech_level <= 10))
);
-- Create index "vessels_game_empire_name_lower_key" to table: "vessels"
CREATE UNIQUE INDEX "vessels_game_empire_name_lower_key" ON "vessels" ("game_id", "empire_id", (lower(name)));
-- Create trigger "vessels_set_updated_at"
CREATE TRIGGER "vessels_set_updated_at" BEFORE UPDATE ON "vessels" FOR EACH ROW EXECUTE FUNCTION "set_updated_at"();
-- Create trigger "players_set_updated_at"
CREATE TRIGGER "players_set_updated_at" BEFORE UPDATE ON "players" FOR EACH ROW EXECUTE FUNCTION "set_updated_at"();
-- Create trigger "empires_set_updated_at"
CREATE TRIGGER "empires_set_updated_at" BEFORE UPDATE ON "empires" FOR EACH ROW EXECUTE FUNCTION "set_updated_at"();
-- Create trigger "games_set_updated_at"
CREATE TRIGGER "games_set_updated_at" BEFORE UPDATE ON "games" FOR EACH ROW EXECUTE FUNCTION "set_updated_at"();
-- Create "jump_routes" table
CREATE TABLE "jump_routes" (
  "id" bigserial NOT NULL,
  "game_id" bigint NOT NULL,
  "system_a_id" bigint NOT NULL,
  "system_b_id" bigint NOT NULL,
  "cost" integer NOT NULL,
  "last_turn_used" integer NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "jump_routes_game_endpoints_key" UNIQUE ("game_id", "system_a_id", "system_b_id"),
  CONSTRAINT "jump_routes_game_id_key" UNIQUE ("game_id", "id"),
  CONSTRAINT "jump_routes_system_a_fk" FOREIGN KEY ("game_id", "system_a_id") REFERENCES "star_systems" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "jump_routes_system_b_fk" FOREIGN KEY ("game_id", "system_b_id") REFERENCES "star_systems" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "jump_routes_cost_check" CHECK (cost > 0),
  CONSTRAINT "jump_routes_ordering_check" CHECK (system_a_id < system_b_id)
);
-- Create "empire_jump_point_knowledge" table
CREATE TABLE "empire_jump_point_knowledge" (
  "game_id" bigint NOT NULL,
  "empire_id" bigint NOT NULL,
  "route_id" bigint NOT NULL,
  "system_id" bigint NOT NULL,
  "detected" boolean NOT NULL DEFAULT false,
  "range_band" text NULL,
  "destination_known" boolean NOT NULL DEFAULT false,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("empire_id", "route_id", "system_id"),
  CONSTRAINT "empire_jump_point_knowledge_empire_fk" FOREIGN KEY ("game_id", "empire_id") REFERENCES "empires" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "empire_jump_point_knowledge_route_fk" FOREIGN KEY ("game_id", "route_id") REFERENCES "jump_routes" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "empire_jump_point_knowledge_system_fk" FOREIGN KEY ("game_id", "system_id") REFERENCES "star_systems" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "empire_planet_names" table
CREATE TABLE "empire_planet_names" (
  "game_id" bigint NOT NULL,
  "empire_id" bigint NOT NULL,
  "planet_id" bigint NOT NULL,
  "name" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("empire_id", "planet_id"),
  CONSTRAINT "empire_planet_names_empire_fk" FOREIGN KEY ("game_id", "empire_id") REFERENCES "empires" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "empire_planet_names_planet_fk" FOREIGN KEY ("game_id", "planet_id") REFERENCES "planets" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "empire_planet_names_empire_name_lower_key" to table: "empire_planet_names"
CREATE UNIQUE INDEX "empire_planet_names_empire_name_lower_key" ON "empire_planet_names" ("empire_id", (lower(name)));
-- Create "empire_system_names" table
CREATE TABLE "empire_system_names" (
  "game_id" bigint NOT NULL,
  "empire_id" bigint NOT NULL,
  "system_id" bigint NOT NULL,
  "name" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("empire_id", "system_id"),
  CONSTRAINT "empire_system_names_empire_fk" FOREIGN KEY ("game_id", "empire_id") REFERENCES "empires" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "empire_system_names_system_fk" FOREIGN KEY ("game_id", "system_id") REFERENCES "star_systems" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "empire_system_names_empire_name_lower_key" to table: "empire_system_names"
CREATE UNIQUE INDEX "empire_system_names_empire_name_lower_key" ON "empire_system_names" ("empire_id", (lower(name)));
-- Create "units" table
CREATE TABLE "units" (
  "code" text NOT NULL,
  "display_name" text NOT NULL,
  "category" text NULL,
  "source" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("code"),
  CONSTRAINT "units_source_check" CHECK (source = ANY (ARRAY['mined'::text, 'farmed'::text, 'factory'::text]))
);
-- Create "vessel_inventory" table
CREATE TABLE "vessel_inventory" (
  "game_id" bigint NOT NULL,
  "vessel_id" bigint NOT NULL,
  "unit_code" text NOT NULL,
  "tech_level" integer NOT NULL,
  "quantity" integer NOT NULL DEFAULT 0,
  "active" integer NOT NULL DEFAULT 0,
  "cargo" integer NOT NULL DEFAULT 0,
  "mass" integer NOT NULL DEFAULT 0,
  "volume" integer NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("game_id", "vessel_id", "unit_code", "tech_level"),
  CONSTRAINT "vessel_inventory_unit_code_fkey" FOREIGN KEY ("unit_code") REFERENCES "units" ("code") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "vessel_inventory_vessel_fk" FOREIGN KEY ("game_id", "vessel_id") REFERENCES "vessels" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "vessel_inventory_active_check" CHECK ((active >= 0) AND (active <= quantity)),
  CONSTRAINT "vessel_inventory_cargo_check" CHECK (cargo >= 0),
  CONSTRAINT "vessel_inventory_mass_check" CHECK (mass >= 0),
  CONSTRAINT "vessel_inventory_quantity_check" CHECK (quantity >= 0),
  CONSTRAINT "vessel_inventory_tech_level_check" CHECK ((tech_level >= 0) AND (tech_level <= 10)),
  CONSTRAINT "vessel_inventory_volume_check" CHECK (volume >= 0)
);
-- Create "factory_groups" table
CREATE TABLE "factory_groups" (
  "game_id" bigint NOT NULL,
  "vessel_id" bigint NOT NULL,
  "group_no" integer NOT NULL,
  "unit_code" text NOT NULL,
  "tech_level" integer NOT NULL,
  "quantity" integer NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("game_id", "vessel_id", "group_no", "unit_code", "tech_level"),
  CONSTRAINT "factory_groups_inventory_fk" FOREIGN KEY ("game_id", "vessel_id", "unit_code", "tech_level") REFERENCES "vessel_inventory" ("game_id", "vessel_id", "unit_code", "tech_level") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "factory_groups_unit_code_fkey" FOREIGN KEY ("unit_code") REFERENCES "units" ("code") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "factory_groups_vessel_fk" FOREIGN KEY ("game_id", "vessel_id") REFERENCES "vessels" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "factory_groups_quantity_check" CHECK (quantity > 0),
  CONSTRAINT "factory_groups_tech_level_check" CHECK ((tech_level >= 0) AND (tech_level <= 10))
);
-- Create "natural_resources" table
CREATE TABLE "natural_resources" (
  "id" bigserial NOT NULL,
  "game_id" bigint NOT NULL,
  "planet_id" bigint NOT NULL,
  "resource_type" text NOT NULL,
  "capacity" integer NOT NULL,
  "base_extraction" integer NOT NULL,
  "yield_percent" integer NOT NULL,
  "reserves" bigint NOT NULL DEFAULT 0,
  "is_infinite" boolean NOT NULL DEFAULT false,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "natural_resources_game_id_key" UNIQUE ("game_id", "id"),
  CONSTRAINT "natural_resources_planet_fk" FOREIGN KEY ("game_id", "planet_id") REFERENCES "planets" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "natural_resources_base_extraction_check" CHECK (base_extraction >= 0),
  CONSTRAINT "natural_resources_capacity_check" CHECK (capacity > 0),
  CONSTRAINT "natural_resources_reserves_check" CHECK (reserves >= 0),
  CONSTRAINT "natural_resources_resource_type_check" CHECK (resource_type = ANY (ARRAY['ore'::text, 'energy'::text, 'gold'::text, 'materials'::text, 'farmland'::text])),
  CONSTRAINT "natural_resources_yield_percent_check" CHECK ((yield_percent >= 1) AND (yield_percent <= 100))
);
-- Create "farming_groups" table
CREATE TABLE "farming_groups" (
  "game_id" bigint NOT NULL,
  "vessel_id" bigint NOT NULL,
  "resource_id" bigint NOT NULL,
  "group_no" integer NOT NULL,
  "unit_code" text NOT NULL,
  "tech_level" integer NOT NULL,
  "quantity" integer NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("game_id", "vessel_id", "resource_id", "group_no", "unit_code", "tech_level"),
  CONSTRAINT "farming_groups_inventory_fk" FOREIGN KEY ("game_id", "vessel_id", "unit_code", "tech_level") REFERENCES "vessel_inventory" ("game_id", "vessel_id", "unit_code", "tech_level") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "farming_groups_resource_fk" FOREIGN KEY ("game_id", "resource_id") REFERENCES "natural_resources" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "farming_groups_unit_code_fkey" FOREIGN KEY ("unit_code") REFERENCES "units" ("code") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "farming_groups_vessel_fk" FOREIGN KEY ("game_id", "vessel_id") REFERENCES "vessels" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "farming_groups_quantity_check" CHECK (quantity > 0),
  CONSTRAINT "farming_groups_tech_level_check" CHECK ((tech_level >= 0) AND (tech_level <= 10))
);
-- Create "home_worlds" table
CREATE TABLE "home_worlds" (
  "game_id" bigint NOT NULL,
  "planet_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("game_id", "planet_id"),
  CONSTRAINT "home_worlds_planet_fk" FOREIGN KEY ("game_id", "planet_id") REFERENCES "planets" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "mining_groups" table
CREATE TABLE "mining_groups" (
  "game_id" bigint NOT NULL,
  "vessel_id" bigint NOT NULL,
  "resource_id" bigint NOT NULL,
  "group_no" integer NOT NULL,
  "unit_code" text NOT NULL,
  "tech_level" integer NOT NULL,
  "quantity" integer NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("game_id", "vessel_id", "resource_id", "group_no", "unit_code", "tech_level"),
  CONSTRAINT "mining_groups_inventory_fk" FOREIGN KEY ("game_id", "vessel_id", "unit_code", "tech_level") REFERENCES "vessel_inventory" ("game_id", "vessel_id", "unit_code", "tech_level") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "mining_groups_resource_fk" FOREIGN KEY ("game_id", "resource_id") REFERENCES "natural_resources" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "mining_groups_unit_code_fkey" FOREIGN KEY ("unit_code") REFERENCES "units" ("code") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "mining_groups_vessel_fk" FOREIGN KEY ("game_id", "vessel_id") REFERENCES "vessels" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "mining_groups_quantity_check" CHECK (quantity > 0),
  CONSTRAINT "mining_groups_tech_level_check" CHECK ((tech_level >= 0) AND (tech_level <= 10))
);
-- Create "population_groups" table
CREATE TABLE "population_groups" (
  "game_id" bigint NOT NULL,
  "vessel_id" bigint NOT NULL,
  "group_type" text NOT NULL,
  "count" integer NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("game_id", "vessel_id", "group_type"),
  CONSTRAINT "population_groups_vessel_fk" FOREIGN KEY ("game_id", "vessel_id") REFERENCES "vessels" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "population_groups_count_check" CHECK (count >= 0),
  CONSTRAINT "population_groups_group_type_check" CHECK (group_type = ANY (ARRAY['untrained'::text, 'worker'::text, 'manager'::text, 'soldier'::text, 'pilot'::text]))
);
-- Create "training_queue" table
CREATE TABLE "training_queue" (
  "id" bigserial NOT NULL,
  "game_id" bigint NOT NULL,
  "vessel_id" bigint NOT NULL,
  "from_group_type" text NOT NULL,
  "to_group_type" text NOT NULL,
  "count" integer NOT NULL,
  "start_turn" integer NOT NULL,
  "completion_turn" integer NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "training_queue_game_id_key" UNIQUE ("game_id", "id"),
  CONSTRAINT "training_queue_vessel_fk" FOREIGN KEY ("game_id", "vessel_id") REFERENCES "vessels" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "training_queue_count_check" CHECK (count > 0),
  CONSTRAINT "training_queue_from_group_type_check" CHECK (from_group_type = ANY (ARRAY['untrained'::text, 'worker'::text, 'manager'::text, 'soldier'::text, 'pilot'::text])),
  CONSTRAINT "training_queue_to_group_type_check" CHECK (to_group_type = ANY (ARRAY['untrained'::text, 'worker'::text, 'manager'::text, 'soldier'::text, 'pilot'::text])),
  CONSTRAINT "training_queue_turn_order_check" CHECK (completion_turn >= start_turn)
);
-- Create "unit_recipes" table
CREATE TABLE "unit_recipes" (
  "unit_code" text NOT NULL,
  "input_code" text NOT NULL,
  "quantity" integer NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("unit_code", "input_code"),
  CONSTRAINT "unit_recipes_input_code_fkey" FOREIGN KEY ("input_code") REFERENCES "units" ("code") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "unit_recipes_unit_code_fkey" FOREIGN KEY ("unit_code") REFERENCES "units" ("code") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "unit_recipes_quantity_check" CHECK (quantity > 0)
);
