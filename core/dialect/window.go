package dialect

import (
	"fmt"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"
)

func validateStandardWindowFunction(dialectName string, f qcode.Field, supportsNulls bool) error {
	if f.Window == nil {
		return nil
	}
	if qcode.WindowSpecHasNulls(f.Window) && !supportsNulls {
		return fmt.Errorf("%s does not support this analytics ordering", dialectName)
	}
	return nil
}

func (d *PostgresDialect) ValidateWindowFunction(f qcode.Field) error {
	return validateStandardWindowFunction(d.Name(), f, true)
}

func (d *SQLiteDialect) ValidateWindowFunction(f qcode.Field) error {
	if sqliteVersionLess(d.DBVersion, 32500, 3025000) {
		return fmt.Errorf("analytics directive on field %q is not supported by SQLite %d; SQLite 3.25+ is required",
			f.FieldName, d.DBVersion)
	}
	if qcode.WindowSpecHasNulls(f.Window) && sqliteVersionLess(d.DBVersion, 33000, 3030000) {
		return fmt.Errorf("SQLite %d does not support this analytics ordering; SQLite 3.30+ is required",
			d.DBVersion)
	}
	return nil
}

func sqliteVersionLess(v, compactMin, libMin int) bool {
	if v == 0 {
		return false
	}
	if v >= 1000000 {
		return v < libMin
	}
	return v < compactMin
}
