package application

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/betting/domain"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/db"
	"sportbet/internal/shared/errs"
	"sportbet/internal/shared/money"
)

const (
	MinStakeCents  int64 = 100
	MaxStakeCents  int64 = 50000
	MaxPayoutCents int64 = 1000000
	MaxTotalOdd    int64 = 50000
)

type PlaceBetCommand struct {
	UserID         string
	IdempotencyKey string
	Stake          money.Money
	Selections     []SelectionInput
}

type SelectionInput struct {
	SelectionID string
	Odd         money.Odds
}

type PlaceBetResult struct {
	BetID           string
	TotalOdd        money.Odds
	PotentialPayout money.Money
	ChangedOdds     []domain.OddsChangedError
}

type PlaceBetUseCase struct {
	pool    *pgxpool.Pool
	markets contracts.MarketReader
	wallet  contracts.WalletService
	repo    Repository
	clock   func() time.Time
}

type Repository interface {
	InsertBet(ctx context.Context, q db.Querier, bet *domain.Bet, selections []domain.BetSelection) (string, error)
}

func NewPlaceBetUseCase(pool *pgxpool.Pool, markets contracts.MarketReader, wallet contracts.WalletService, repo Repository, clock func() time.Time) *PlaceBetUseCase {
	return &PlaceBetUseCase{pool: pool, markets: markets, wallet: wallet, repo: repo, clock: clock}
}

func (uc *PlaceBetUseCase) Execute(ctx context.Context, cmd PlaceBetCommand) (PlaceBetResult, error) {
	if cmd.Stake.Cents() < MinStakeCents {
		return PlaceBetResult{}, domain.ErrStakeTooLow
	}
	if cmd.Stake.Cents() > MaxStakeCents {
		return PlaceBetResult{}, domain.ErrStakeTooHigh
	}
	if len(cmd.Selections) < 1 || len(cmd.Selections) > 5 {
		return PlaceBetResult{}, domain.ErrInvalidSelectionCount
	}

	selIDs := make([]string, len(cmd.Selections))
	for i, s := range cmd.Selections {
		selIDs[i] = s.SelectionID
	}

	marketsInfo, err := uc.markets.GetSelections(ctx, selIDs)
	if err != nil {
		return PlaceBetResult{}, fmt.Errorf("markets: %w", err)
	}

	now := uc.clock()
	eventSet := make(map[string]bool)
	var changed []domain.OddsChangedError
	var betSelections []domain.BetSelection
	var oddsToCombine []money.Odds

	for _, sInput := range cmd.Selections {
		info, ok := marketsInfo[sInput.SelectionID]
		if !ok {
			return PlaceBetResult{}, domain.ErrSelectionNotFound
		}
		if !info.IsOpen(now) {
			return PlaceBetResult{}, domain.ErrMarketClosed
		}
		if eventSet[info.EventID] {
			return PlaceBetResult{}, domain.ErrCorrelatedSelections
		}
		eventSet[info.EventID] = true

		if info.Odd != sInput.Odd {
			changed = append(changed, domain.OddsChangedError{
				SelectionID: sInput.SelectionID,
				NewOdd:      info.Odd,
			})
		}

		oddsToCombine = append(oddsToCombine, info.Odd)
		betSelections = append(betSelections, domain.BetSelection{
			SelectionID: sInput.SelectionID,
			EventID:     info.EventID,
			Odd:         info.Odd,
			Result:      "PENDING",
		})
	}

	if len(changed) > 0 {
		return PlaceBetResult{ChangedOdds: changed}, errs.New(errs.ErrOddsChanged, "ODDS_CHANGED", "La cuota de una o más selecciones ha cambiado")
	}

	totalOdd := money.Combine(oddsToCombine...)
	if int64(totalOdd) > MaxTotalOdd {
		return PlaceBetResult{}, domain.ErrTotalOddTooHigh
	}

	potentialPayout := cmd.Stake.MulOdds(totalOdd)
	if potentialPayout.Cents() > MaxPayoutCents {
		return PlaceBetResult{}, domain.ErrPayoutTooHigh
	}

	bet := &domain.Bet{
		UserID:            cmd.UserID,
		IdempotencyKey:    cmd.IdempotencyKey,
		Stake:             cmd.Stake,
		TotalOdd:          totalOdd,
		PotentialPayout:   potentialPayout,
		Status:            domain.StatusPending,
		Payout:            money.FromCents(0),
		SettlementVersion: 0,
	}

	var betID string
	err = db.WithTx(ctx, uc.pool, func(q db.Querier) error {
		id, errInsert := uc.repo.InsertBet(ctx, q, bet, betSelections)
		if errInsert != nil {
			return errInsert
		}
		betID = id

		ref := contracts.TxRef{Type: contracts.TxBetStake, Reference: "bet:" + id}
		_, errDebit := uc.wallet.Debit(ctx, q, cmd.UserID, cmd.Stake, ref)
		if errDebit != nil {
			return errDebit
		}
		return nil
	})

	if err != nil {
		return PlaceBetResult{}, fmt.Errorf("place bet: %w", err)
	}

	return PlaceBetResult{
		BetID:           betID,
		TotalOdd:        totalOdd,
		PotentialPayout: potentialPayout,
	}, nil
}
