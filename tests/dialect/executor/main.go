// Executor for the GraphJin GraphQL dialect examples in this folder.
//
// Each .gql file is one example: a single GraphQL operation, optionally
// preceded by header comments:
//
//	# vars: {"slug": "grid-scale-storage-summit"}   variables (JSON)
//	# use_cursor: <file-name>                        bind $cursor to the
//	                                                 cursor captured from
//	                                                 another example's result
//	# analytics: on                                  run against the
//	                                                 analytics_mode engine
//	# expect-error: <substring>                      example is expected to
//	                                                 fail; documents a limit
//	# note: <text>                                   printed with the result
//
// Every example runs inside a database transaction that is ALWAYS rolled
// back, so mutations never pollute the seed data.
//
// Usage (from the workspace root):
//
//	GJ_TEST_PG='postgres://postgres:postgres@localhost:5433/gjtest?sslmode=disable' \
//	  go run ./tests/dialect/executor [-filter filters/] [-v]
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aegion-dynamic/graphjin-slim/core/v3"
	_ "github.com/aegion-dynamic/graphjin-slim/graphql/v3" // register the query language
	postgresmod "github.com/aegion-dynamic/graphjin-slim/postgres/v3"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type example struct {
	path       string
	name       string
	query      string
	vars       map[string]any
	useCursor  string
	analytics  bool
	expectErr  string
	note       string
}

func main() {
	dsn := flag.String("dsn", os.Getenv("GJ_TEST_PG"), "postgres DSN")
	filter := flag.String("filter", "", "only run examples whose name contains this")
	dir := flag.String("dir", ".", "examples directory")
	verbose := flag.Bool("v", false, "print the SQL of passing examples too")
	flag.Parse()

	if *dsn == "" {
		fmt.Println("no DSN: set GJ_TEST_PG or pass -dsn")
		os.Exit(2)
	}

	db, err := sql.Open(postgresmod.DriverPostgres, *dsn)
	must(err)
	defer db.Close()
	must(db.Ping())

	tables := []core.Table{{
		Name:     "events",
		Schema:   "application",
		Database: core.DefaultDBName,
		Columns: []core.Column{
			{Name: "description", FullText: true},
			{Name: "summary", FullText: true},
		},
	}}

	mk := func(analytics bool) *core.GraphJin {
		conf := core.Config{
			DisableAllowList: true,
			AnalyticsMode:    analytics,
			Tables:           tables,
		}
		gj, err := core.NewGraphJin(&conf, db)
		must(err)
		return gj
	}
	plain, an := mk(false), mk(true)
	defer plain.Close()
	defer an.Close()

	examples := load(*dir)
	var ran, failed int

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	cursors := map[string]string{}

	for _, ex := range examples {
		if *filter != "" && !strings.Contains(ex.name, *filter) {
			continue
		}
		ran++
		gj := plain
		if ex.analytics {
			gj = an
		}

		varsRaw := map[string]json.RawMessage{}
		for k, v := range ex.vars {
			if s, ok := v.(string); ok && strings.HasPrefix(s, "@") {
				c, ok := cursors[s[1:]]
				if !ok {
					fmt.Printf("FAIL %s: cursor %q not captured yet (ordering problem)\n", ex.name, s)
					failed++
					break
				}
				varsRaw[k] = json.RawMessage(quote(s[1:], c))
				continue
			}
			b, _ := json.Marshal(v)
			varsRaw[k] = b
		}
		if ex.useCursor != "" {
			c, ok := cursors[ex.useCursor]
			if !ok {
				fmt.Printf("FAIL %s: cursor from %q not captured yet\n", ex.name, ex.useCursor)
				failed++
				continue
			}
			varsRaw["cursor"] = json.RawMessage(quote("cursor", c))
		}
		var varsJSON json.RawMessage
		if len(varsRaw) != 0 {
			varsJSON, _ = json.Marshal(varsRaw)
		}

		// Every example runs in a rolled-back transaction.
		tx, txErr := db.BeginTx(ctx, nil)
		if txErr != nil {
			fmt.Printf("FAIL %s: begin tx: %v\n", ex.name, txErr)
			failed++
			continue
		}
		res, err := gj.GraphQL(ctx, ex.query, varsJSON, &core.RequestConfig{Tx: tx})
		tx.Rollback()

		fmt.Printf("=== %s\n", ex.name)
		switch {
		case err != nil:
			msg := err.Error()
			if ex.expectErr != "" && strings.Contains(msg, ex.expectErr) {
				fmt.Printf("PASS (expected error): %s\n", msg)
			} else {
				failed++
				if res != nil {
					fmt.Printf("FAIL: %v\nsql: %.400s\n", err, res.SQL())
				} else {
					fmt.Printf("FAIL: %v\n", err)
				}
				if ex.note != "" {
					fmt.Printf("note: %s\n", ex.note)
				}
			}
		case len(res.Errors) != 0:
			var msgs []string
			for _, ge := range res.Errors {
				msgs = append(msgs, ge.Message)
			}
			msg := strings.Join(msgs, "; ")
			if ex.expectErr != "" && strings.Contains(msg, ex.expectErr) {
				fmt.Printf("PASS (expected error): %s\n", msg)
			} else {
				failed++
				fmt.Printf("FAIL: %s\nsql: %.400s\n", msg, res.SQL())
				if ex.note != "" {
					fmt.Printf("note: %s\n", ex.note)
				}
			}
		default:
			if ex.expectErr != "" {
				failed++
				fmt.Printf("FAIL: expected error %q but got data: %.300s\n", ex.expectErr, res.Data)
				continue
			}
			out := string(res.Data)
			if len(out) > 500 {
				out = out[:500] + " …TRUNC"
			}
			fmt.Printf("OK: %s\n", out)
			if *verbose {
				fmt.Printf("sql: %s\n", res.SQL())
			}
			captureCursors(ex.name, res.Data, cursors)
		}
	}

	fmt.Printf("\n%d run, %d failed\n", ran, failed)
	if failed != 0 {
		os.Exit(1)
	}
}

// load walks dir for .gql files, sorted by path (file prefixes control order).
func load(dir string) []example {
	var paths []string
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(p, ".gql") {
			paths = append(paths, p)
		}
		return nil
	})
	must(err)
	sort.Strings(paths)

	var out []example
	for _, p := range paths {
		b, err := os.ReadFile(p)
		must(err)
		ex := example{path: p, query: strings.TrimSpace(string(b))}
		ex.name = strings.TrimSuffix(p, ".gql")
		ex.name = strings.TrimPrefix(ex.name, dir)
		ex.name = strings.TrimPrefix(ex.name, "/")

		var body []string
		for _, line := range strings.Split(ex.query, "\n") {
			t := strings.TrimSpace(line)
			if !strings.HasPrefix(t, "# ") {
				body = append(body, line)
				continue
			}
			t = t[2:]
			switch {
			case strings.HasPrefix(t, "name: "):
				ex.name = t[6:]
			case strings.HasPrefix(t, "vars: "):
				must(json.Unmarshal([]byte(t[6:]), &ex.vars))
			case strings.HasPrefix(t, "use_cursor: "):
				ex.useCursor = t[len("use_cursor: "):]
			case strings.HasPrefix(t, "analytics: "):
				ex.analytics = strings.TrimSpace(t[len("analytics: "):]) == "on"
			case strings.HasPrefix(t, "expect-error: "):
				ex.expectErr = strings.TrimSpace(t[len("expect-error: "):])
			case strings.HasPrefix(t, "note: "):
				ex.note = t[6:]
			}
		}
		ex.query = strings.TrimSpace(strings.Join(body, "\n"))
		out = append(out, ex)
	}
	return out
}

// captureCursors records any "<root>_cursor" key found in the result JSON,
// keyed by the example name, so later files can reference it with
// # use_cursor: <name> or vars {"cursor": "@<name>"}.
func captureCursors(name string, data []byte, into map[string]string) {
	var v any
	if json.Unmarshal(data, &v) != nil {
		return
	}
	scan(name, v, into)
}

func scan(name string, v any, into map[string]string) {
	switch t := v.(type) {
	case map[string]any:
		for k, vv := range t {
			if k == "cursor" || strings.HasSuffix(k, "_cursor") {
				if s, ok := vv.(string); ok && s != "" {
					into[name] = s
				}
			}
			scan(name, vv, into)
		}
	case []any:
		for _, vv := range t {
			scan(name, vv, into)
		}
	}
}

func quote(name, v string) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
