package infrastructure

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/db"
	"sportbet/internal/shared/errs"
	"sportbet/internal/shared/money"
)

type walletService struct {
	pool *pgxpool.Pool
}

func NewWalletService(pool *pgxpool.Pool) contracts.WalletService {
	return &walletService{pool: pool}
}

func (s *walletService) CreateWallet(ctx context.Context, q db.Querier, userID string, initial money.Money) error {
	_, err := q.Exec(ctx, `
		INSERT INTO wallets (user_id, balance_cents)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO NOTHING
	`, userID, initial.Cents())
	if err != nil {
		return fmt.Errorf("wallet: create: %w", err)
	}

	if initial.Cents() > 0 {
		ref := contracts.TxRef{
			Type:      contracts.TxDepositInitial,
			Reference: "user:" + userID,
		}
		_, err = q.Exec(ctx, `
			INSERT INTO wallet_transactions (user_id, type, amount_cents, balance_after_cents, reference)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT ON CONSTRAINT wallet_transactions_reference_type_key DO NOTHING
		`, userID, ref.Type, initial.Cents(), initial.Cents(), ref.Reference)
		if err != nil {
			return fmt.Errorf("wallet: insert initial tx: %w", err)
		}
	}

	return nil
}

func (s *walletService) Debit(ctx context.Context, q db.Querier, userID string, amt money.Money, ref contracts.TxRef) (money.Money, error) {
	if amt.Cents() <= 0 {
		return 0, errors.New("wallet: debit amount must be positive")
	}

	var newBalance int64
	err := q.QueryRow(ctx, `
		WITH updated AS (
			UPDATE wallets
			SET balance_cents = balance_cents - $1, updated_at = now()
			WHERE user_id = $2 AND balance_cents >= $1
			RETURNING balance_cents
		)
		INSERT INTO wallet_transactions (user_id, type, amount_cents, balance_after_cents, reference)
		SELECT $2, $3, -$1, balance_cents, $4
		FROM updated
		RETURNING balance_after_cents
	`, amt.Cents(), userID, ref.Type, ref.Reference).Scan(&newBalance)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, errs.ErrInsufficientFunds
		}
		if db.IsUniqueViolation(err, "wallet_transactions_reference_type_key") {
			return 0, errs.ErrAlreadyProcessed
		}
		return 0, fmt.Errorf("wallet: debit: %w", err)
	}

	return money.FromCents(newBalance), nil
}

func (s *walletService) Credit(ctx context.Context, q db.Querier, userID string, amt money.Money, ref contracts.TxRef) (money.Money, error) {
	if amt.Cents() <= 0 {
		return 0, errors.New("wallet: credit amount must be positive")
	}

	var newBalance int64
	err := q.QueryRow(ctx, `
		WITH updated AS (
			UPDATE wallets
			SET balance_cents = balance_cents + $1, updated_at = now()
			WHERE user_id = $2
			RETURNING balance_cents
		)
		INSERT INTO wallet_transactions (user_id, type, amount_cents, balance_after_cents, reference)
		SELECT $2, $3, $1, balance_cents, $4
		FROM updated
		RETURNING balance_after_cents
	`, amt.Cents(), userID, ref.Type, ref.Reference).Scan(&newBalance)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, errors.New("wallet: user not found")
		}
		if db.IsUniqueViolation(err, "wallet_transactions_reference_type_key") {
			return 0, errs.ErrAlreadyProcessed
		}
		return 0, fmt.Errorf("wallet: credit: %w", err)
	}

	return money.FromCents(newBalance), nil
}

func (s *walletService) Balance(ctx context.Context, userID string) (money.Money, error) {
	var balance int64
	err := s.pool.QueryRow(ctx, "SELECT balance_cents FROM wallets WHERE user_id = $1", userID).Scan(&balance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, errors.New("wallet: user not found")
		}
		return 0, fmt.Errorf("wallet: balance: %w", err)
	}
	return money.FromCents(balance), nil
}
