package harness

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// bg is the context for all seeding operations.
func bg() context.Context { return context.Background() }

// Domain models — these double as both the migration source and the
// ground-truth shapes scenarios assert against.

type User struct {
	ID        int64     `bun:"id,pk,autoincrement"`
	FullName  string    `bun:"full_name,notnull"`
	Email     string    `bun:"email,notnull"`
	CreatedAt time.Time `bun:"created_at,nullzero"`
}

func (*User) TableName() string { return "users" }

type Product struct {
	ID      int64   `bun:"id,pk,autoincrement"`
	Name    string  `bun:"name,notnull"`
	Price   float64 `bun:"price,notnull"`
	UsersID int64   `bun:"users_id"`
}

func (*Product) TableName() string { return "products" }

type BlobRow struct {
	ID      int64  `bun:"id,pk,autoincrement"`
	Content string `bun:"content,notnull"`
}

func (*BlobRow) TableName() string { return "blobs" }

// SeedShop creates users + products with deterministic rows:
// Ada / Alan / Grace, four products spread across them.
func SeedShop(db *bun.DB) error {
	if _, err := db.NewCreateTable().Model((*User)(nil)).IfNotExists().Exec(bg()); err != nil {
		return err
	}
	if _, err := db.NewCreateTable().Model((*Product)(nil)).
		ForeignKey(`(users_id) REFERENCES users(id)`).
		IfNotExists().Exec(bg()); err != nil {
		return err
	}

	users := []User{
		{FullName: "Ada Lovelace", Email: "ada@example.com"},
		{FullName: "Alan Turing", Email: "alan@example.com"},
		{FullName: "Grace Hopper", Email: "grace@example.com"},
	}
	if _, err := db.NewInsert().Model(&users).Exec(bg()); err != nil {
		return err
	}

	products := []Product{
		{Name: "Analytical Engine", Price: 999.99, UsersID: users[0].ID},
		{Name: "Enigma Replica", Price: 499.50, UsersID: users[1].ID},
		{Name: "Compiler Design", Price: 149.00, UsersID: users[2].ID},
		{Name: "COBOL Manual", Price: 39.99, UsersID: users[2].ID},
	}
	_, err := db.NewInsert().Model(&products).Exec(bg())
	return err
}

// SeedBlob stores a single ~size-byte text row.
func SeedBlob(db *bun.DB, size int) error {
	if _, err := db.NewCreateTable().Model((*BlobRow)(nil)).IfNotExists().Exec(bg()); err != nil {
		return err
	}
	payload := make([]byte, size)
	for i := range payload {
		payload[i] = byte('a' + i%26)
	}
	_, err := db.NewInsert().Model(&BlobRow{Content: string(payload)}).Exec(bg())
	return err
}

// SeedChain builds n tables ta, tb, tc… (pure-alpha names survive
// GraphJin's name normalization untouched) where each carries an FK to
// its predecessor, holding exactly one chained row.
//
// These tables differ only by name and FK target, which static ORM
// models cannot express — so this one builder drops below the model
// layer deliberately. Everything else stays model-driven.
func SeedChain(db *bun.DB, n int) error {
	for i := 0; i < n; i++ {
		cur := ChainTable(i)
		prevCol := ""
		fk := ""
		if i > 0 {
			prevCol = ", prev_id INTEGER"
			fk = fmt.Sprintf(", FOREIGN KEY (prev_id) REFERENCES %s(id)", ChainTable(i-1))
		}
		ddl := fmt.Sprintf(
			`CREATE TABLE %s (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL%s%s)`,
			cur, prevCol, fk,
		)
		if _, err := db.Exec(ddl); err != nil {
			return fmt.Errorf("create %s: %w", cur, err)
		}

		row := map[string]any{"name": cur + " row"}
		if i > 0 {
			row["prev_id"] = 1
		}
		if _, err := db.NewInsert().Model(&row).
			TableExpr(cur).
			Exec(bg()); err != nil {
			return fmt.Errorf("seed %s: %w", cur, err)
		}
	}
	return nil
}

// chainLetters are level names ta…tz minus "to" (SQL keyword).
var chainLetters = func() []string {
	out := []string{}
	for r := 'a'; r <= 'z'; r++ {
		if r == 'o' {
			continue
		}
		out = append(out, "t"+string(r))
	}
	return out
}()

// ChainTable names level i (0-based). Pure-alpha names pass through
// GraphJin's name normalization untouched.
func ChainTable(i int) string {
	if i < len(chainLetters) {
		return chainLetters[i]
	}
	return fmt.Sprintf("tx%d", i) // beyond 25 levels: numeric fallback
}

var _ = time.Second // reserved for future timing-aware seeds
