// Package fakes tiene una implementación en memoria de cada contrato. Deben
// ser tan estrictas como las reales: si el fake permite algo que el módulo
// real rechaza, la integración falla el domingo. Las suites de
// contracts/contracttest corren contra el fake y contra la implementación real.
package fakes

import (
	"context"
	"net/http"
	"slices"
	"sync"
	"time"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/db"
	"sportbet/internal/shared/errs"
	"sportbet/internal/shared/money"
)

// ---------- Authenticator ----------

// Authenticator inyecta User en cada request. Con User nil responde 401.
type Authenticator struct {
	User *contracts.CurrentUser
}

func (a *Authenticator) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.User == nil {
			http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"token requerido"}}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(contracts.WithCurrentUser(r.Context(), *a.User)))
	})
}

func (a *Authenticator) RequireRole(roles ...contracts.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := contracts.CurrentUserFrom(r.Context())
			if !ok || !slices.Contains(roles, u.Role) {
				http.Error(w, `{"error":{"code":"FORBIDDEN","message":"permiso denegado"}}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ---------- MarketReader ----------

type MarketReader struct {
	mu         sync.RWMutex
	Selections map[string]contracts.SelectionInfo
}

func NewMarketReader() *MarketReader {
	return &MarketReader{Selections: map[string]contracts.SelectionInfo{}}
}

func (m *MarketReader) Add(s contracts.SelectionInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Selections[s.SelectionID] = s
}

func (m *MarketReader) GetSelections(_ context.Context, ids []string) (map[string]contracts.SelectionInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]contracts.SelectionInfo, len(ids))
	for _, id := range ids {
		if s, ok := m.Selections[id]; ok {
			out[id] = s
		}
	}
	return out, nil
}

// ---------- ResultReader ----------

type ResultReader struct {
	mu       sync.RWMutex
	Results  map[string]contracts.EventResult
	Outcomes map[string]map[string]contracts.SelectionOutcome // eventID → selectionID → outcome
}

func NewResultReader() *ResultReader {
	return &ResultReader{
		Results:  map[string]contracts.EventResult{},
		Outcomes: map[string]map[string]contracts.SelectionOutcome{},
	}
}

func (f *ResultReader) Set(r contracts.EventResult, outcomes map[string]contracts.SelectionOutcome) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Results[r.EventID] = r
	f.Outcomes[r.EventID] = outcomes
}

func (f *ResultReader) GetResult(_ context.Context, eventID string) (contracts.EventResult, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	r, ok := f.Results[eventID]
	if !ok {
		return r, errs.New(errs.ErrNotFound, "EVENT_NOT_FOUND", "evento no encontrado")
	}
	if !r.Status.IsTerminal() {
		return r, errs.New(errs.ErrConflict, "EVENT_NOT_FINISHED", "el evento no ha terminado")
	}
	return r, nil
}

func (f *ResultReader) ResolveSelections(ctx context.Context, eventID string) (map[string]contracts.SelectionOutcome, error) {
	r, err := f.GetResult(ctx, eventID)
	if err != nil {
		return nil, err
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make(map[string]contracts.SelectionOutcome, len(f.Outcomes[eventID]))
	for id, o := range f.Outcomes[eventID] {
		if r.Status.VoidsSelections() {
			o = contracts.OutcomeVoid
		}
		out[id] = o
	}
	return out, nil
}

func (f *ResultReader) ListFinishedSince(_ context.Context, since time.Time) ([]contracts.EventResult, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var out []contracts.EventResult
	for _, r := range f.Results {
		if r.Status.IsTerminal() && !r.FinishedAt.Before(since) {
			out = append(out, r)
		}
	}
	return out, nil
}

// ---------- WalletService ----------

// Wallet ignora el db.Querier: no participa en transacciones, pero respeta
// saldo insuficiente y referencias duplicadas como la implementación real.
type Wallet struct {
	mu       sync.Mutex
	Balances map[string]money.Money
	refs     map[contracts.TxRef]bool
}

func NewWallet() *Wallet {
	return &Wallet{Balances: map[string]money.Money{}, refs: map[contracts.TxRef]bool{}}
}

func (w *Wallet) CreateWallet(_ context.Context, _ db.Querier, userID string, initial money.Money) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.Balances[userID]; ok {
		return nil
	}
	w.Balances[userID] = initial
	w.refs[contracts.TxRef{Type: contracts.TxDepositInitial, Reference: "user:" + userID}] = true
	return nil
}

func (w *Wallet) Debit(_ context.Context, _ db.Querier, userID string, amt money.Money, ref contracts.TxRef) (money.Money, error) {
	if !amt.IsPositive() {
		return 0, errs.New(errs.ErrValidation, "INVALID_AMOUNT", "el monto debe ser positivo")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	bal, ok := w.Balances[userID]
	if !ok {
		return 0, errs.New(errs.ErrNotFound, "WALLET_NOT_FOUND", "billetera no encontrada")
	}
	if w.refs[ref] {
		return bal, errs.New(errs.ErrAlreadyProcessed, "DUPLICATE_TRANSACTION", "movimiento ya aplicado")
	}
	if bal < amt {
		return bal, errs.New(errs.ErrInsufficientFunds, "INSUFFICIENT_FUNDS", "saldo insuficiente")
	}
	w.refs[ref] = true
	w.Balances[userID] = bal - amt
	return bal - amt, nil
}

func (w *Wallet) Credit(_ context.Context, _ db.Querier, userID string, amt money.Money, ref contracts.TxRef) (money.Money, error) {
	if !amt.IsPositive() {
		return 0, errs.New(errs.ErrValidation, "INVALID_AMOUNT", "el monto debe ser positivo")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	bal, ok := w.Balances[userID]
	if !ok {
		return 0, errs.New(errs.ErrNotFound, "WALLET_NOT_FOUND", "billetera no encontrada")
	}
	if w.refs[ref] {
		return bal, errs.New(errs.ErrAlreadyProcessed, "DUPLICATE_TRANSACTION", "movimiento ya aplicado")
	}
	w.refs[ref] = true
	w.Balances[userID] = bal + amt
	return bal + amt, nil
}

func (w *Wallet) Balance(_ context.Context, userID string) (money.Money, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	bal, ok := w.Balances[userID]
	if !ok {
		return 0, errs.New(errs.ErrNotFound, "WALLET_NOT_FOUND", "billetera no encontrada")
	}
	return bal, nil
}

// ---------- Providers ----------

type FixtureProvider struct {
	Fixtures []contracts.ProviderFixture
}

func (f *FixtureProvider) Name() string { return "fake" }

func (f *FixtureProvider) FixturesByDate(_ context.Context, date time.Time, _ []string) ([]contracts.ProviderFixture, error) {
	y, m, d := date.UTC().Date()
	var out []contracts.ProviderFixture
	for _, fx := range f.Fixtures {
		fy, fm, fd := fx.StartAt.UTC().Date()
		if fy == y && fm == m && fd == d {
			out = append(out, fx)
		}
	}
	return out, nil
}

func (f *FixtureProvider) FixtureResult(_ context.Context, externalID string) (contracts.ProviderFixture, error) {
	for _, fx := range f.Fixtures {
		if fx.ExternalID == externalID {
			return fx, nil
		}
	}
	return contracts.ProviderFixture{}, errs.New(errs.ErrNotFound, "FIXTURE_NOT_FOUND", "fixture no encontrado")
}

// LiveScoreProvider devuelve un frame por llamada y repite el último al terminar.
type LiveScoreProvider struct {
	mu     sync.Mutex
	Frames [][]contracts.LiveScore
	next   int
}

func (f *LiveScoreProvider) Name() string { return "fake" }

func (f *LiveScoreProvider) LiveScores(context.Context) ([]contracts.LiveScore, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.Frames) == 0 {
		return nil, nil
	}
	i := min(f.next, len(f.Frames)-1)
	f.next++
	return f.Frames[i], nil
}

// Comprobación en compilación de que los fakes cumplen los contratos.
var (
	_ contracts.Authenticator     = (*Authenticator)(nil)
	_ contracts.MarketReader      = (*MarketReader)(nil)
	_ contracts.ResultReader      = (*ResultReader)(nil)
	_ contracts.WalletService     = (*Wallet)(nil)
	_ contracts.FixtureProvider   = (*FixtureProvider)(nil)
	_ contracts.LiveScoreProvider = (*LiveScoreProvider)(nil)
)
