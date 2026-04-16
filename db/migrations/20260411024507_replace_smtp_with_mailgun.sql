-- Create "mailgun_settings" table
CREATE TABLE "mailgun_settings" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "api_key" text NOT NULL,
  "sending_domain" text NOT NULL,
  "from_address" text NOT NULL,
  "from_name" text NOT NULL DEFAULT '',
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "mailgun_settings_from_address_length" CHECK ((char_length(from_address) >= 3) AND (char_length(from_address) <= 320))
);
-- Create trigger "mailgun_settings_set_updated_at"
CREATE TRIGGER "mailgun_settings_set_updated_at" BEFORE UPDATE ON "mailgun_settings" FOR EACH ROW EXECUTE FUNCTION "set_updated_at"();
-- Drop "smtp_settings" table
DROP TABLE "smtp_settings";
