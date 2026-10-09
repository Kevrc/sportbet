package infrastructure_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/db"
	"sportbet/internal/shared/errs"
	"sportbet/internal/users/domain"
	"sportbet/internal/users/infrastructure"
	"sportbet/migrations"
)

// testPool crea un schema propio para la prueba, aplica las migraciones y lo
// borra al terminar. Se salta si TEST_DATABASE_URL no está definida.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL no definida")
	}
	ctx := context.Background()
	admin, err := db.Connect(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("test_users_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
	})
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	qs := u.Query()
	qs.Set("search_path", schema)
	u.RawQuery = qs.Encode()
	pool, err := db.Connect(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool, migrations.FS); err != nil {
		t.Fatal(err)
	}
	return pool
}

func insertUser(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (email, password_hash, display_name) VALUES ($1, 'hash', 'Nombre') RETURNING id`, email).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestUserRepository(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := infrastructure.NewUserRepository(pool)
	ids := []string{insertUser(t, pool, "a@x.com"), insertUser(t, pool, "b@x.com"), insertUser(t, pool, "c@x.com")}

	u, err := repo.FindByID(ctx, ids[0])
	if err != nil || u.Email != "a@x.com" || u.Role != contracts.RoleUser || u.Status != domain.StatusActive {
		t.Fatalf("FindByID = %+v, %v", u, err)
	}
	if _, err := repo.FindByID(ctx, "00000000-0000-0000-0000-000000000000"); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("err = %v; want ErrNotFound", err)
	}

	page, total, err := repo.List(ctx, 2, 0)
	if err != nil || total != 3 || len(page) != 2 {
		t.Fatalf("List = %d usuarios, total %d, %v", len(page), total, err)
	}
	rest, _, _ := repo.List(ctx, 2, 2)
	if len(rest) != 1 {
		t.Fatalf("segunda página = %d; want 1", len(rest))
	}

	admin, blocked := contracts.RoleAdmin, domain.StatusBlocked
	u, err = repo.Update(ctx, ids[1], domain.Changes{Role: &admin})
	if err != nil || u.Role != contracts.RoleAdmin || u.Status != domain.StatusActive {
		t.Fatalf("Update rol = %+v, %v", u, err)
	}
	u, err = repo.Update(ctx, ids[1], domain.Changes{Status: &blocked})
	if err != nil || u.Role != contracts.RoleAdmin || u.Status != domain.StatusBlocked {
		t.Fatalf("Update estado debe conservar el rol: %+v, %v", u, err)
	}
	if _, err := repo.Update(ctx, "00000000-0000-0000-0000-000000000000", domain.Changes{Status: &blocked}); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("err = %v; want ErrNotFound", err)
	}
}
