INSERT INTO "roles" ("name", "description")
VALUES ('tester', 'Seeded test account')
ON CONFLICT ("name") DO NOTHING;
