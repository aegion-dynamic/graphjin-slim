//nolint:errcheck
package sqlgen

import (
	"fmt"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/dialect"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"
)

func (c *compilerContext) renderInsert() {
	i := 0
	for _, m := range c.qc.Mutates {
		switch {
		case m.Type == qcode.MTInsert:
			i = c.renderComma(i)
			c.renderInsertStmt(m, false)
		case m.Type == qcode.MTUpsert:
			i = c.renderComma(i)
			c.renderInsertStmt(m, true)
		case m.Type == qcode.MTConnect:
			// A connect under an insert is a lookup: the CTE holds the
			// parent's key column for the row the filter matches, and the
			// insert binds its foreign key to it. The insert renders it as
			// one of its dependencies, so it is not rendered here.
			continue
		}
	}
}

func (c *compilerContext) renderInsertStmt(m qcode.Mutate, embedded bool) {
	n := c.renderOneToManyModifiers(m)
	if n != 0 {
		c.w.WriteString(`, `)
	}

	if m.ConflictAction == qcode.ConflictGet {
		c.renderInsertConflictGet(m)
		return
	}

	c.dialect.RenderMutationCTE(c, &m, func() {
		c.renderInsertBody(m)
		if !embedded {
			c.renderReturning(m)
		}
	})
}

func (c *compilerContext) renderInsertBody(m qcode.Mutate) {
	c.dialect.RenderInsert(c, &m, func() {
		n := c.renderInsertUpdateColumns(m)
		c.renderNestedRelColumns(m, false, false, n)
	})
	c.renderValues(m, false)
}

func (c *compilerContext) renderInsertConflictGet(m qcode.Mutate) {
	renderer, ok := c.dialect.(dialect.InsertConflictGetRenderer)
	if !ok {
		c.SetError(fmt.Errorf("on_conflict: get is not supported by the %s dialect", c.dialect.Name()))
		return
	}

	renderInsert := func() {
		c.dialect.RenderInsert(c, &m, func() {
			n := c.renderInsertUpdateColumns(m)
			c.renderNestedRelColumns(m, false, false, n)
		})
		c.renderValues(m, false)
		renderer.RenderInsertConflictGetClause(c, &m)
		c.renderReturning(m)
	}

	if renderer.InsertConflictGetMode() == dialect.InsertConflictGetNative {
		c.dialect.RenderMutationCTE(c, &m, renderInsert)
		return
	}

	if renderer.InsertConflictGetMode() != dialect.InsertConflictGetWritableCTE {
		c.SetError(fmt.Errorf("on_conflict: get cannot use the %s non-linear mutation path", c.dialect.Name()))
		return
	}

	c.qc.InsertConflictFallback = true
	inserted := fmt.Sprintf("_gj_inserted_%d", m.ID)
	c.quoted(inserted)
	c.w.WriteString(` AS (`)
	renderInsert()
	c.w.WriteString(`), `)
	c.renderCteName(m)
	c.w.WriteString(` AS (SELECT * FROM `)
	c.quoted(inserted)
	c.w.WriteString(` UNION ALL SELECT *`)
	if c.captureMutationChanges && c.dialect.Name() == "postgres" {
		c.w.WriteString(`, NULL::jsonb AS `)
		c.quoted(mutationAuditOld)
		c.w.WriteString(`, NULL::jsonb AS `)
		c.quoted(mutationAuditNew)
	}
	c.w.WriteString(` FROM `)
	c.colWithTable(m.Ti.Schema, m.Ti.Name)
	c.w.WriteString(` AS `)
	c.quoted("_gj_existing")
	c.w.WriteString(` WHERE `)
	c.renderInsertConflictWhere(m, "_gj_existing")
	c.w.WriteString(` LIMIT 1)`)
}

func (c *compilerContext) renderInsertConflictWhere(m qcode.Mutate, table string) {
	for i, col := range m.ConflictCols {
		if i != 0 {
			c.w.WriteString(` AND `)
		}
		c.colWithTable(table, col.Col.Name)
		c.w.WriteString(` IS NOT DISTINCT FROM `)
		if !m.IsJSON {
			c.renderColumnValue(m, col)
			continue
		}

		c.w.WriteString(`(SELECT `)
		c.renderColumnValue(m, col)
		c.w.WriteString(` FROM `)
		c.quoted("_sg_input")
		c.w.WriteString(` i, `)
		c.renderMutateToRecordSet(m, 0)
		c.w.WriteString(` LIMIT 1)`)
	}
}
