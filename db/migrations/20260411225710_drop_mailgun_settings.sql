-- Drop "mailgun_settings" table
-- Phase 2 removed the database-backed Mailgun settings flow entirely;
-- Mailgun credentials now come from server.json / env. The DROP is
-- intentional and the table is expected to be empty at migration time.
-- atlas:nolint DS102
DROP TABLE "mailgun_settings";
