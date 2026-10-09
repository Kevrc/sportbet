package infrastructure_test

import (
	"context"
	"testing"
	"os"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/contracts/contracttest"
	"sportbet/internal/shared/db"
	"sportbet/internal/wallet/infrastructure"
)

func TestWalletService(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://sportbet:sportbet@localhost:5432/sportbet?sslmode=disable"
	}

	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

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
