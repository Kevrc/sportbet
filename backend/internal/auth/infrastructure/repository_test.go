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

	"sportbet/internal/auth/application"
	"sportbet/internal/auth/domain"
	"sportbet/internal/auth/infrastructure"
	"sportbet/internal/shared/db"
	"sportbet/internal/shared/errs"
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
	schema := fmt.Sprintf("test_auth_%d", time.Now().UnixNano())
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

func TestUserRepository(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := infrastructure.NewUserRepository(pool)

	u, _ := domain.NewUser("ana@mail.com", "Ana", "secreto123")
	u.PasswordHash = "hash"
	created, err := repo.Create(ctx, pool, u)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.CreatedAt.IsZero() {
		t.Fatalf("faltan ID o CreatedAt: %+v", created)
	}

	u.Email = "ANA@mail.com"
	if _, err := repo.Create(ctx, pool, u); !errors.Is(err, errs.ErrConflict) {
		t.Fatalf("email duplicado: err = %v; want ErrConflict", err)
	}

	found, err := repo.FindByEmail(ctx, "Ana@Mail.com")
	if err != nil || found.ID != created.ID || found.Role != "user" || !found.IsActive() {
		t.Fatalf("FindByEmail = %+v, %v", found, err)
	}
	if _, err := repo.FindByEmail(ctx, "nadie@mail.com"); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("err = %v; want ErrNotFound", err)
	}
}

func TestRefreshTokenRepository(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	u, _ := domain.NewUser("ana@mail.com", "Ana", "secreto123")
	u.PasswordHash = "hash"
	created, err := infrastructure.NewUserRepository(pool).Create(ctx, pool, u)
	if err != nil {
		t.Fatal(err)
	}

	_, hash := application.NewRefreshToken()
	repo := infrastructure.NewRefreshTokenRepository()
	if err := repo.Create(ctx, pool, created.ID, hash, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, pool, created.ID, hash, time.Now().Add(time.Hour)); !db.IsUniqueViolation(err, "") {
		t.Fatalf("hash repetido: err = %v; want violación única", err)
	}
}
