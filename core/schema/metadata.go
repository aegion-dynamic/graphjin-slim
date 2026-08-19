package schema

type MetadataSnapshot struct {
	Databases     []MetadataDatabase
	Tables        []MetadataTable
	Columns       []MetadataColumn
	Relationships []MetadataRelationship
	Functions     []MetadataFunction
	Indexes       []MetadataIndex
}
type MetadataDatabase struct {
	ID, Name, Type      string
	IsDefault, ReadOnly bool
}
type MetadataTable struct {
	ID, DatabaseName, SchemaName, TableName, Type, Comment, PrimaryKey string
	ColumnCount                                                        int
	TableKey                                                           string
}
type MetadataColumn struct {
	ID, TableID, DatabaseName, SchemaName, TableName, ColumnName, Type string
	Array, NotNull, PrimaryKey, UniqueKey, Indexed                     bool
	IndexName, DefaultValue, Comment                                   string
	Ordinal                                                            int
	TableKey, ColumnKey                                                string
}
type MetadataRelationship struct {
	ID, FromDatabaseName, FromSchemaName, FromTableName, FromColumnName, FromColumnID string
	ToDatabaseName, ToSchemaName, ToTableName, ToColumnName, ToColumnID               string
	RelType                                                                           string
	IsCrossDatabase                                                                   bool
	Source                                                                            string
}
type MetadataFunction struct {
	ID, DatabaseName, SchemaName, Name, ReturnType string
	Aggregate                                      bool
	Comment                                        string
}
type MetadataIndex struct {
	ID, DatabaseName, SchemaName, TableName, ColumnName, Name string
	Unique                                                    bool
}
