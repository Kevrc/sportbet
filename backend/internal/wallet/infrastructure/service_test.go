package infrastructure_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/contracts/contracttest"
	"sportbet/internal/shared/db"
	"sportbet/internal/wallet/infrastructure"
	"sportbet/migrations"
)

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
	schema := fmt.Sprintf("test_wallet_%d", time.Now().UnixNano())
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

func TestWalletService(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)

	service := infrastructure.NewWalletService(pool)

	contracttest.RunWalletServiceSuite(t, func(t *testing.T) (contracts.WalletService, db.Querier, string) {
		// Crear un usuario temporal
		var userID string
		err := pool.QueryRow(ctx, "INSERT INTO users (email, password_hash, display_name) VALUES ('test_' || gen_random_uuid() || '@sportbet.local', 'hash', 'Test User') RETURNING id").Scan(&userID)
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
		return service, pool, userID
	})
}
