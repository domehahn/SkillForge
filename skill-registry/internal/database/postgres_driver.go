package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"strconv"
	"strings"

	"github.com/lib/pq"
)

func init() {
	sql.Register("skillforge-postgres", rebindDriver{inner: &pq.Driver{}})
}

type rebindDriver struct {
	inner driver.Driver
}

func (d rebindDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.inner.Open(name)
	if err != nil {
		return nil, err
	}
	return rebindConn{Conn: conn}, nil
}

type rebindConn struct {
	driver.Conn
}

func (c rebindConn) Prepare(query string) (driver.Stmt, error) {
	return c.Conn.Prepare(rewritePostgres(query))
}

func (c rebindConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if p, ok := c.Conn.(driver.ConnPrepareContext); ok {
		return p.PrepareContext(ctx, rewritePostgres(query))
	}
	return c.Prepare(query)
}

func (c rebindConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if e, ok := c.Conn.(driver.ExecerContext); ok {
		return e.ExecContext(ctx, rewritePostgres(query), args)
	}
	return nil, driver.ErrSkip
}

func (c rebindConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if q, ok := c.Conn.(driver.QueryerContext); ok {
		return q.QueryContext(ctx, rewritePostgres(query), args)
	}
	return nil, driver.ErrSkip
}

func (c rebindConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if b, ok := c.Conn.(driver.ConnBeginTx); ok {
		return b.BeginTx(ctx, opts)
	}
	//lint:ignore SA1019 database/sql still requires Begin for legacy drivers without ConnBeginTx.
	return c.Conn.Begin()
}

func (c rebindConn) Ping(ctx context.Context) error {
	if p, ok := c.Conn.(driver.Pinger); ok {
		return p.Ping(ctx)
	}
	return nil
}

func rewritePostgres(query string) string {
	query = rebindPlaceholders(query)
	query = rewritePostgresDialect(query)
	return query
}

func rebindPlaceholders(query string) string {
	var b strings.Builder
	b.Grow(len(query) + 8)
	inSingle := false
	inDouble := false
	arg := 1
	for i := 0; i < len(query); i++ {
		ch := query[i]
		if ch == '\'' && !inDouble {
			b.WriteByte(ch)
			if inSingle && i+1 < len(query) && query[i+1] == '\'' {
				i++
				b.WriteByte(query[i])
				continue
			}
			inSingle = !inSingle
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			b.WriteByte(ch)
			continue
		}
		if ch == '?' && !inSingle && !inDouble {
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(arg))
			arg++
			continue
		}
		b.WriteByte(ch)
	}
	return b.String()
}

var sqliteDateNowParam = regexp.MustCompile(`date\('now',\s*(\$\d+)\)`)

func rewritePostgresDialect(query string) string {
	originalHadInsertIgnore := strings.Contains(query, "INSERT OR IGNORE INTO")
	replacements := []struct {
		old string
		new string
	}{
		{"INTEGER PRIMARY KEY AUTOINCREMENT", "BIGSERIAL PRIMARY KEY"},
		{"DATETIME", "TIMESTAMPTZ"},
		{"datetime('now')", "NOW()"},
		{"INSERT OR IGNORE INTO", "INSERT INTO"},
		{"json_object(", "json_build_object("},
		{"json_array()", "json_build_array()"},
		{"FROM json_each(skills.tags) WHERE json_each.value =", "FROM jsonb_array_elements_text(skills.tags::jsonb) AS tag(value) WHERE tag.value ="},
		{"json_extract(av.manifest, '$.spec.dependencies') IS NOT NULL", "(av.manifest::jsonb #> '{spec,dependencies}') IS NOT NULL"},
		{"json_each(json_extract(av.manifest, '$.spec.dependencies')) dep", "jsonb_array_elements(COALESCE(av.manifest::jsonb #> '{spec,dependencies}', '[]'::jsonb)) AS dep(value)"},
		{"json_extract(dep.value, '$.namespace')", "dep.value->>'namespace'"},
		{"json_extract(dep.value, '$.name')", "dep.value->>'name'"},
	}
	for _, r := range replacements {
		query = strings.ReplaceAll(query, r.old, r.new)
	}
	query = sqliteDateNowParam.ReplaceAllString(query, "(CURRENT_DATE + ($1)::interval)")
	if originalHadInsertIgnore {
		query = addConflictDoNothing(query)
	}
	return query
}

func addConflictDoNothing(query string) string {
	parts := strings.Split(query, ";")
	for i, part := range parts {
		if strings.Contains(part, "INSERT INTO") && !strings.Contains(part, "ON CONFLICT") {
			parts[i] = strings.TrimRight(part, " \n\t") + " ON CONFLICT DO NOTHING"
		}
	}
	return strings.Join(parts, ";")
}
