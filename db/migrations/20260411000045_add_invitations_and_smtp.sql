-- Create "smtp_settings" table
CREATE TABLE "smtp_settings" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "host" text NOT NULL,
  "port" integer NOT NULL DEFAULT 587,
  "username" text NOT NULL DEFAULT '',
  "password" text NOT NULL DEFAULT '',
  "from_address" text NOT NULL,
  "from_name" text NOT NULL DEFAULT '',
  "tls_enabled" boolean NOT NULL DEFAULT true,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "smtp_settings_from_address_length" CHECK ((char_length(from_address) >= 3) AND (char_length(from_address) <= 320)),
  CONSTRAINT "smtp_settings_port_range" CHECK ((port >= 1) AND (port <= 65535))
);
-- Create trigger "smtp_settings_set_updated_at"
CREATE TRIGGER "smtp_settings_set_updated_at" BEFORE UPDATE ON "smtp_settings" FOR EACH ROW EXECUTE FUNCTION "set_updated_at"();
-- Create "invitations" table
CREATE TABLE "invitations" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "email" text NOT NULL,
  "code" text NOT NULL,
  "invited_by" uuid NOT NULL,
  "used_by" uuid NULL,
  "used_at" timestamptz NULL,
  "expires_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "invitations_code_key" UNIQUE ("code"),
  CONSTRAINT "invitations_invited_by_fkey" FOREIGN KEY ("invited_by") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "invitations_used_by_fkey" FOREIGN KEY ("used_by") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  CONSTRAINT "invitations_email_length" CHECK ((char_length(email) >= 3) AND (char_length(email) <= 320)),
  CONSTRAINT "invitations_email_lowercase" CHECK (email = lower(email))
);
-- Create index "invitations_code_idx" to table: "invitations"
CREATE INDEX "invitations_code_idx" ON "invitations" ("code");
-- Create index "invitations_email_idx" to table: "invitations"
CREATE INDEX "invitations_email_idx" ON "invitations" ("email");
