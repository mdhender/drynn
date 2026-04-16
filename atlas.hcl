env "local" {
  src = "file://db/schema.sql"
  dev = "postgres://drynn_atlas_user:strong-password-here@localhost:5432/drynn_atlas?sslmode=disable&search_path=drynn_dev"

  migration {
    dir = "file://db/migrations"
  }
}
