package fakes_test

import (
	"testing"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/contracts/contracttest"
	"sportbet/internal/shared/contracts/fakes"
	"sportbet/internal/shared/db"
)

func TestFakeWalletCumpleContrato(t *testing.T) {
	contracttest.RunWalletServiceSuite(t, func(t *testing.T) (contracts.WalletService, db.Querier, string) {
		return fakes.NewWallet(), nil, "user-1"
	})
}
