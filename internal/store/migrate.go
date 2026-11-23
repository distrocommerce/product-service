package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var coreMigrations embed.FS

//go:embed migrations/local/*.sql
var localMigrations embed.FS

// migrationLock is the pg_advisory_lock key; any constant unique to this service.
const migrationLock = 7_301_001

type migration struct {
	version int
	name    string
	sql     string
}

// Migrate applies pending migrations in order, each in its own transaction.
// Local migrations (1000+) run only when includeLocal is true.
func Migrate(ctx context.Context, pool *pgxpool.Pool, includeLocal bool) error {
	ms, err := load(coreMigrations, "migrations", 1, 999)
	if err != nil {
		return err
	}
	if includeLocal {
		local, err := load(localMigrations, "migrations/local", 1000, 1<<30)
		if err != nil {
			return err
		}
		ms = append(ms, local...)
	}

	conn, err := pool.Acquire(ctx) // the advisory lock belongs to one session
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLock); err != nil {
		return fmt.Errorf("lock: %w", err)
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLock) //nolint:errcheck

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	for _, m := range ms {
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		var applied bool
		if err := tx.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", m.version).Scan(&applied); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if applied {
			_ = tx.Rollback(ctx)
			continue
		}
		if _, err := tx.Exec(ctx, m.sql); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", m.name, err)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", m.version); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit %s: %w", m.name, err)
		}
	}

	return nil
}

// load reads the *.sql files of dir in fsys, parses each file's version from its
// name (<version>_<description>.sql) and returns them sorted by version. A version
// outside [minVersion, maxVersion], an unparsable name or a duplicate version is an error.
func load(fsys fs.FS, dir string, minVersion, maxVersion int) ([]migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}

	seen := make(map[int]string)
	var ms []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue // migrations/local is a subdirectory of migrations
		}
		name := e.Name()

		prefix, _, ok := strings.Cut(name, "_")
		if !ok {
			return nil, fmt.Errorf("migration %s: name must be <version>_<description>.sql", name)
		}
		version, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("migration %s: version %q is not a number", name, prefix)
		}
		if version < minVersion || version > maxVersion {
			return nil, fmt.Errorf("migration %s: version %d outside %d-%d", name, version, minVersion, maxVersion)
		}
		if prev, dup := seen[version]; dup {
			return nil, fmt.Errorf("migration %s: version %d already used by %s", name, version, prev)
		}
		seen[version] = name

		body, err := fs.ReadFile(fsys, path.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		ms = append(ms, migration{version: version, name: name, sql: string(body)})
	}

	sort.Slice(ms, func(i, j int) bool { return ms[i].version < ms[j].version })

	return ms, nil
}
