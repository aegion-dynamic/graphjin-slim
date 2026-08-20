package introspection

import _ "embed"

//go:embed sql/postgres_functions.sql
var postgresFunctionsStmt string

//go:embed sql/postgres_info.sql
var postgresInfo string

//go:embed sql/postgres_columns_basic.sql
var postgresColumnsBasicStmt string

//go:embed sql/postgres_constraints_count.sql
var postgresConstraintsCountStmt string

//go:embed sql/postgres_constraint_columns.sql
var postgresConstraintColumnsStmt string

//go:embed sql/postgres_view_pks.sql
var postgresViewPKsStmt string

//go:embed sql/sqlite_functions.sql
var sqliteFunctionsStmt string

//go:embed sql/sqlite_info.sql
var sqliteInfo string

//go:embed sql/sqlite_columns.sql
var sqliteColumnsStmt string

var (
	PostgresColumnsBasicStmt       = postgresColumnsBasicStmt
	PostgresConstraintsCountStmt   = postgresConstraintsCountStmt
	PostgresConstraintColumnsStmt = postgresConstraintColumnsStmt
	PostgresFunctionsStmt          = postgresFunctionsStmt
	PostgresInfo                   = postgresInfo
	PostgresViewPKsStmt            = postgresViewPKsStmt
	SQLiteFunctionsStmt            = sqliteFunctionsStmt
	SQLiteInfo                     = sqliteInfo
	SQLiteColumnsStmt              = sqliteColumnsStmt
)
