-- Create "races" table
CREATE TABLE "races" (
  "id" bigserial NOT NULL,
  "game_id" bigint NOT NULL,
  "home_planet_id" bigint NOT NULL,
  "name" text NOT NULL,
  "temperature_class" integer NOT NULL,
  "pressure_class" integer NOT NULL,
  "gas" integer[] NOT NULL,
  "gas_percent" integer[] NOT NULL,
  "required_gas" integer NOT NULL,
  "required_gas_min" integer NOT NULL,
  "required_gas_max" integer NOT NULL,
  "neutral_gas" integer[] NOT NULL,
  "poison_gas" integer[] NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "races_game_home_planet_key" UNIQUE ("game_id", "home_planet_id"),
  CONSTRAINT "races_game_id_key" UNIQUE ("game_id", "id"),
  CONSTRAINT "races_game_id_fkey" FOREIGN KEY ("game_id") REFERENCES "games" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "races_home_planet_fk" FOREIGN KEY ("game_id", "home_planet_id") REFERENCES "planets" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "races_gas_len_check" CHECK (array_length(gas, 1) = 4),
  CONSTRAINT "races_gas_percent_len_check" CHECK (array_length(gas_percent, 1) = 4),
  CONSTRAINT "races_neutral_gas_len_check" CHECK (array_length(neutral_gas, 1) = 6),
  CONSTRAINT "races_poison_gas_len_check" CHECK (array_length(poison_gas, 1) = 6),
  CONSTRAINT "races_pressure_class_check" CHECK ((pressure_class >= 0) AND (pressure_class <= 29)),
  CONSTRAINT "races_temperature_class_check" CHECK ((temperature_class >= 1) AND (temperature_class <= 30))
);
-- Create index "races_game_name_lower_key" to table: "races"
CREATE UNIQUE INDEX "races_game_name_lower_key" ON "races" ("game_id", (lower(name)));
-- Create trigger "races_set_updated_at"
CREATE TRIGGER "races_set_updated_at" BEFORE UPDATE ON "races" FOR EACH ROW EXECUTE FUNCTION "set_updated_at"();
-- Modify "empires" table
ALTER TABLE "empires" ADD COLUMN "race_id" bigint NOT NULL, ADD CONSTRAINT "empires_race_fk" FOREIGN KEY ("game_id", "race_id") REFERENCES "races" ("game_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE;
