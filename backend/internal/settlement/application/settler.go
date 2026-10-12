package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/settlement/infrastructure"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/db"
	"sportbet/internal/shared/errs"
	"sportbet/internal/shared/eventbus"
	"sportbet/internal/shared/money"
)

type Settler struct {
	pool    *pgxpool.Pool
	wallet  contracts.WalletService
	results contracts.ResultReader
	repo    *infrastructure.Repository
	clock   func() time.Time
	bus     eventbus.Bus
}

// NewSettler recibe el bus para anunciar BetSettled; con nil no publica.
func NewSettler(pool *pgxpool.Pool, wallet contracts.WalletService, results contracts.ResultReader, repo *infrastructure.Repository, clock func() time.Time, bus eventbus.Bus) *Settler {
	return &Settler{pool: pool, wallet: wallet, results: results, repo: repo, clock: clock, bus: bus}
}

// HandleMatchFinished es invocado por el bus de eventos cuando un partido termina.
func (s *Settler) HandleMatchFinished(ctx context.Context, e eventbus.MatchFinished) {
	_ = s.SettleEvent(ctx, e.EventID)
}

// Sweep ejecuta un barrido periódico como respaldo por si se pierden eventos del bus.
func (s *Settler) Sweep(ctx context.Context) error {
	// Buscamos partidos terminados en las últimas 48 horas
	since := s.clock().Add(-48 * time.Hour)
	resultsList, err := s.results.ListFinishedSince(ctx, since)
	if err != nil {
		return fmt.Errorf("settlement sweep: %w", err)
	}

	for _, r := range resultsList {
		// Ignoramos errores individuales para que el sweep continúe
		_ = s.SettleEvent(ctx, r.EventID)
	}
	return nil
}

// SettleEvent orquesta la liquidación de todas las apuestas vinculadas a un evento.
func (s *Settler) SettleEvent(ctx context.Context, eventID string) error {
	outcomes, err := s.results.ResolveSelections(ctx, eventID)
	if err != nil {
		return err
	}
	if len(outcomes) == 0 {
		return nil
	}

	// 1. Marcar el resultado en bet_selections de manera masiva (fuera de transacción)
	if err := s.repo.UpdateEventSelections(ctx, eventID, outcomes); err != nil {
		return fmt.Errorf("update event selections: %w", err)
	}

	// 2. Obtener IDs de apuestas que dependen de este evento y siguen PENDING
	betIDs, err := s.repo.GetPendingBetsForEvent(ctx, eventID)
	if err != nil {
		return fmt.Errorf("get pending bets: %w", err)
	}

	// 3. Liquidar cada apuesta de manera individual y atómica
	for _, betID := range betIDs {
		_ = s.SettleBet(ctx, betID)
	}

	return nil
}

// SettleBet evalúa el estado final de una apuesta y ejecuta el pago o reembolso atómicamente.
func (s *Settler) SettleBet(ctx context.Context, betID string) error {
	var settled *eventbus.BetSettled
	err := db.WithTx(ctx, s.pool, func(q db.Querier) error {
		bet, selections, err := s.repo.GetBetWithSelections(ctx, q, betID)
		if err != nil {
			return err
		}
		if bet.Status != "PENDING" {
			return nil
		}

		newStatus := "WON"
		hasPending := false
		allVoid := true
		var nonVoidOdds []money.Odds

		for _, sel := range selections {
			if sel.Result == contracts.OutcomeLost {
				newStatus = "LOST"
				break
			}
			if sel.Result == contracts.OutcomePending {
				hasPending = true
			}
			if sel.Result != contracts.OutcomeVoid {
				allVoid = false
				nonVoidOdds = append(nonVoidOdds, sel.Odd)
			}
		}

		if newStatus != "LOST" {
			if hasPending {
				return nil // Sigue PENDING
			}
			if allVoid {
				newStatus = "VOID"
			}
		}

		var payout money.Money
		var txType contracts.TxType

		if newStatus == "WON" {
			totalOdd := money.Combine(nonVoidOdds...)
			payout = bet.Stake.MulOdds(totalOdd)
			if payout.Cents() > 1000000 {
				payout = money.FromCents(1000000)
			}
			txType = contracts.TxBetPayout
		} else if newStatus == "VOID" {
			payout = bet.Stake
			txType = contracts.TxBetRefund
		}

		updated, err := s.repo.UpdateBetSettled(ctx, q, betID, newStatus, payout)
		if err != nil {
			return err
		}
		if !updated {
			return nil // Ya fue procesada por concurrencia
		}

		if payout.Cents() > 0 {
			ref := contracts.TxRef{
				Type:      txType,
				Reference: fmt.Sprintf("bet:%s:v%d", bet.ID, bet.SettlementVersion),
			}
			_, err = s.wallet.Credit(ctx, q, bet.UserID, payout, ref)
			if err != nil && !errors.Is(err, errs.ErrAlreadyProcessed) {
				return fmt.Errorf("credit payout: %w", err)
			}
		}

		settled = &eventbus.BetSettled{
			BetID: betID, UserID: bet.UserID, Status: contracts.BetStatus(newStatus),
			Payout: payout, SettledAt: s.clock(),
		}
		return nil
	})
	// Se anuncia solo si esta llamada liquidó la apuesta, y después del COMMIT.
	if err == nil && settled != nil && s.bus != nil {
		s.bus.Publish(ctx, *settled)
	}
	return err
}
