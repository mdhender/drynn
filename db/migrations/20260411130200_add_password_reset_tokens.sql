-- Create "password_reset_tokens" table
CREATE TABLE "password_reset_tokens" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "user_id" uuid NOT NULL,
  "code" text NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "used_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "password_reset_tokens_code_key" UNIQUE ("code"),
  CONSTRAINT "password_reset_tokens_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "password_reset_tokens_code_idx" to table: "password_reset_tokens"
CREATE INDEX "password_reset_tokens_code_idx" ON "password_reset_tokens" ("code");
-- Create index "password_reset_tokens_user_id_idx" to table: "password_reset_tokens"
CREATE INDEX "password_reset_tokens_user_id_idx" ON "password_reset_tokens" ("user_id");
