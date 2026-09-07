env "local" {
  src = [
    "file://backend/schema-application.hcl",
    "file://backend/schema-nimbus.hcl",
  ]
  dev = "postgres://postgres:postgres@localhost:5433/atlas_dev?sslmode=disable"
  migration {
    dir = "file://backend/migrations"
  }
}

env "remote" {
  migration {
    dir            = "file://backend/migrations"
    # Store Atlas's revision ledger in `public` (not `application`): migration 1 creates the
    # `application` schema, so a revisions_schema of `application` makes Atlas auto-create that
    # schema first and then `CREATE SCHEMA "application"` collides on fresh DBs. `public` always
    # exists and is never created by a migration. (See docs/atlas_migrations.md.)
    revisions_schema = "public"
  }
  url = getenv("DATABASE_URL")
}