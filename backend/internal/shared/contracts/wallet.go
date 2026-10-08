package contracts

import (
	"context"

	"sportbet/internal/shared/db"
	"sportbet/internal/shared/money"
)

// Implementa: Persona 3 (wallet). Lo usan: Persona 1 (registro) y Persona 3 (betting, settlement).

type TxType string

const (
	TxDepositInitial    TxType = "DEPOSIT_INITIAL"     // +  user:<id>
	TxBetStake          TxType = "BET_STAKE"           // −  bet:<id>
	TxBetPayout         TxType = "BET_PAYOUT"          // +  bet:<id>:v<n>
	TxBetRefund         TxType = "BET_REFUND"          // +  bet:<id>:v<n>
	TxBetPayoutReversal TxType = "BET_PAYOUT_REVERSAL" // −  bet:<id>:v<n>
	TxBetRefundReversal TxType = "BET_REFUND_REVERSAL" // −  bet:<id>:v<n>
	TxAdminAdjustment   TxType = "ADMIN_ADJUSTMENT"    // ±  admin:<uuid>
)

// TxRef identifica un movimiento. UNIQUE(reference, type) en
// wallet_transactions impide aplicar dos veces el mismo movimiento.
type TxRef struct {
	Type      TxType
	Reference string
}

// WalletService recibe un db.Querier para participar en la transacción del
// llamador: PlaceBet debita e inserta la apuesta en una sola transacción.
type WalletService interface {
	// CreateWallet crea la billetera con su DEPOSIT_INITIAL. Es idempotente.
	CreateWallet(ctx context.Context, q db.Querier, userID string, initial money.Money) error
	// Debit hace UPDATE ... WHERE balance_cents >= amt. Devuelve el saldo
	// nuevo o errs.ErrInsufficientFunds.
	Debit(ctx context.Context, q db.Querier, userID string, amt money.Money, ref TxRef) (money.Money, error)
	// Credit devuelve errs.ErrAlreadyProcessed si la referencia ya existe.
	Credit(ctx context.Context, q db.Querier, userID string, amt money.Money, ref TxRef) (money.Money, error)
	Balance(ctx context.Context, userID string) (money.Money, error)
}
