// Package contracttest tiene suites reutilizables que verifican un contrato.
// Cada suite corre contra el fake (en shared) y el dueño la corre contra su
// implementación real, para que fake y real se comporten igual.
package contracttest

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/db"
	"sportbet/internal/shared/errs"
	"sportbet/internal/shared/money"
)

// WalletFactory devuelve un servicio vacío, un Querier para pasarle y el ID
// de un usuario existente sin billetera.
type WalletFactory func(t *testing.T) (svc contracts.WalletService, q db.Querier, userID string)

func RunWalletServiceSuite(t *testing.T, newSvc WalletFactory) {
	ctx := context.Background()

	t.Run("CreateWallet es idempotente", func(t *testing.T) {
		svc, q, uid := newSvc(t)
		must(t, svc.CreateWallet(ctx, q, uid, 1000))
		must(t, svc.CreateWallet(ctx, q, uid, 1000))
		if bal, _ := svc.Balance(ctx, uid); bal != 1000 {
			t.Fatalf("saldo = %d; want 1000", bal)
		}
	})

	t.Run("Debit rechaza saldo insuficiente", func(t *testing.T) {
		svc, q, uid := newSvc(t)
		must(t, svc.CreateWallet(ctx, q, uid, 500))
		_, err := svc.Debit(ctx, q, uid, 501, contracts.TxRef{Type: contracts.TxBetStake, Reference: "bet:x"})
		if !errors.Is(err, errs.ErrInsufficientFunds) {
			t.Fatalf("err = %v; want ErrInsufficientFunds", err)
		}
	})

	t.Run("Credit con referencia repetida no paga dos veces", func(t *testing.T) {
		svc, q, uid := newSvc(t)
		must(t, svc.CreateWallet(ctx, q, uid, 0))
		ref := contracts.TxRef{Type: contracts.TxBetPayout, Reference: "bet:1:v0"}
		if _, err := svc.Credit(ctx, q, uid, 300, ref); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Credit(ctx, q, uid, 300, ref); !errors.Is(err, errs.ErrAlreadyProcessed) {
			t.Fatalf("err = %v; want ErrAlreadyProcessed", err)
		}
		if bal, _ := svc.Balance(ctx, uid); bal != 300 {
			t.Fatalf("saldo = %d; want 300", bal)
		}
	})

	t.Run("20 débitos simultáneos con saldo para uno", func(t *testing.T) {
		svc, q, uid := newSvc(t)
		must(t, svc.CreateWallet(ctx, q, uid, 1000))
		var ok atomic.Int32
		var wg sync.WaitGroup
		for i := range 20 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ref := contracts.TxRef{Type: contracts.TxBetStake, Reference: "bet:c" + string(rune('a'+i))}
				if _, err := svc.Debit(ctx, q, uid, 1000, ref); err == nil {
					ok.Add(1)
				}
			}()
		}
		wg.Wait()
		if ok.Load() != 1 {
			t.Fatalf("débitos exitosos = %d; want 1", ok.Load())
		}
		if bal, _ := svc.Balance(ctx, uid); bal != money.Money(0) {
			t.Fatalf("saldo = %d; want 0", bal)
		}
	})
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
