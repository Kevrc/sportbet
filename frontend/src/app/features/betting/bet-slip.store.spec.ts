import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Observable, Subject, of, throwError } from 'rxjs';

import { Bet, Market, PlaceBetRequest } from '../../core/api/models';
import { ApiError } from '../../core/http/error.interceptor';
import { WalletStore } from '../wallet/public-api';
import { BetSlipStore, SlipPick } from './bet-slip.store';
import { BettingService } from './betting.service';

const pick = (n: number, extra: Partial<SlipPick> = {}): SlipPick => ({
  selectionId: `s${n}`,
  eventId: `e${n}`,
  eventName: `Local ${n} - Visita ${n}`,
  marketId: `m${n}`,
  marketCode: 'MATCH_RESULT',
  marketLabel: 'Resultado final (1X2)',
  label: 'Local',
  oddX100: 200,
  ...extra,
});

const bet = (extra: Partial<Bet> = {}): Bet => ({
  id: 'b1',
  stakeCents: 1000,
  totalOddX100: 200,
  potentialPayoutCents: 2000,
  status: 'PENDING',
  payoutCents: 0,
  placedAt: '2026-10-12T18:00:00Z',
  selections: [],
  ...extra,
});

const market = (n: number, oddX100: number, status: Market['status'] = 'OPEN'): Market => ({
  id: `m${n}`,
  eventId: `e${n}`,
  code: 'MATCH_RESULT',
  status,
  selections: [{ id: `s${n}`, code: 'HOME', label: 'Local', oddX100, result: 'PENDING' }],
});

function setup(
  opts: {
    place?: (body: PlaceBetRequest, key: string) => Observable<Bet>;
    markets?: (eventId: string) => Observable<Market[]>;
    balance?: number | null;
  } = {},
) {
  const placed: { body: PlaceBetRequest; key: string }[] = [];
  const marketCalls: string[] = [];
  const wallet = { balanceCents: signal<number | null>(opts.balance ?? null), refreshed: 0 };
  const service = {
    placeBet: (body: PlaceBetRequest, key: string) => {
      placed.push({ body, key });
      return (opts.place ?? (() => of(bet())))(body, key);
    },
    eventMarkets: (id: string) => {
      marketCalls.push(id);
      return (opts.markets ?? (() => of([])))(id);
    },
  };
  TestBed.configureTestingModule({
    providers: [
      { provide: BettingService, useValue: service },
      {
        provide: WalletStore,
        useValue: { balanceCents: wallet.balanceCents, refresh: () => wallet.refreshed++ },
      },
    ],
  });
  return { store: TestBed.inject(BetSlipStore), placed, marketCalls, wallet };
}

const apiError = (status: number, code: string, fields?: Record<string, string>) =>
  new ApiError(status, code, `mensaje ${code}`, fields);

describe('BetSlipStore · selecciones', () => {
  it('añade y quita al tocar la misma cuota', () => {
    const { store } = setup();
    expect(store.toggle(pick(1))).toBe('added');
    expect(store.count()).toBe(1);
    expect(store.selectedIds().has('s1')).toBe(true);
    expect(store.toggle(pick(1))).toBe('removed');
    expect(store.count()).toBe(0);
  });

  it('otra opción del mismo mercado reemplaza a la anterior', () => {
    const { store } = setup();
    store.toggle(pick(1, { selectionId: 'home', oddX100: 214 }));
    expect(store.toggle(pick(1, { selectionId: 'draw', oddX100: 330 }))).toBe('replaced');
    expect(store.selections().map((s) => s.selectionId)).toEqual(['draw']);
  });

  it('rechaza otro mercado del mismo partido y avisa', () => {
    const { store } = setup();
    store.toggle(pick(1));
    const r = store.toggle(
      pick(1, { selectionId: 'over', marketId: 'm1b', marketCode: 'TOTAL_GOALS_2_5' }),
    );
    expect(r).toBe('same-event');
    expect(store.count()).toBe(1);
    expect(store.notice()).toContain('mismo partido');
  });

  it('admite máximo 5 selecciones', () => {
    const { store } = setup();
    for (let i = 1; i <= 5; i++) expect(store.toggle(pick(i))).toBe('added');
    expect(store.toggle(pick(6))).toBe('full');
    expect(store.count()).toBe(5);
    expect(store.notice()).toContain('máximo 5');
  });

  it('el aviso desaparece al cambiar el boleto', () => {
    const { store } = setup();
    store.toggle(pick(1));
    store.toggle(pick(1, { selectionId: 'x', marketId: 'mx' }));
    expect(store.notice()).not.toBeNull();
    store.toggle(pick(2));
    expect(store.notice()).toBeNull();
  });

  it('clear vacía selecciones y monto', () => {
    const { store } = setup();
    store.toggle(pick(1));
    store.setStake('10');
    store.clear();
    expect([store.count(), store.stakeText()]).toEqual([0, '']);
  });
});

describe('BetSlipStore · resultado', () => {
  it('dismissResult quita el mensaje de resultado', () => {
    const { store } = setup();
    store.toggle(pick(1));
    store.setStake('10');
    store.place();
    expect(store.result()).not.toBeNull();
    store.dismissResult();
    expect(store.result()).toBeNull();
  });
});

describe('BetSlipStore · cálculo y validación', () => {
  it('cuota total y pago: ejemplo del ADR (10.00 × 2.14 × 1.97 = 42.10)', () => {
    const { store } = setup();
    store.toggle(pick(1, { oddX100: 214 }));
    store.toggle(pick(2, { oddX100: 197 }));
    store.setStake('10');
    expect(store.totalOddX100()).toBe(421);
    expect(store.payoutCents()).toBe(4210);
  });

  it('sin monto no hay pago ni error', () => {
    const { store } = setup();
    store.toggle(pick(1));
    expect(store.payoutCents()).toBeNull();
    expect(store.stakeError()).toBeNull();
    expect(store.canPlace()).toBe(false);
  });

  it('valida el monto: formato, mínimo, máximo y saldo', () => {
    const { store } = setup({ balance: 20_000 });
    store.toggle(pick(1));
    store.setStake('abc');
    expect(store.stakeError()).toContain('monto válido');
    store.setStake('0.50');
    expect(store.stakeError()).toContain('mínimo');
    store.setStake('500.01');
    expect(store.stakeError()).toContain('máximo');
    store.setStake('200.01');
    expect(store.stakeError()).toContain('saldo');
    store.setStake('200');
    expect(store.stakeError()).toBeNull();
    expect(store.canPlace()).toBe(true);
  });

  it('si el saldo aún no se conoce, no se valida contra él', () => {
    const { store } = setup({ balance: null });
    store.toggle(pick(1));
    store.setStake('500');
    expect(store.stakeError()).toBeNull();
  });

  it('avisa si el pago potencial supera el máximo y no deja apostar', () => {
    const { store } = setup();
    store.toggle(pick(1, { oddX100: 5000 }));
    store.toggle(pick(2, { oddX100: 5000 }));
    store.setStake('500'); // cuota total topada en 500.00 → 2 500 000 > 1 000 000
    expect(store.oddCapped()).toBe(true);
    expect(store.payoutError()).toContain('10 000.00');
    expect(store.canPlace()).toBe(false);
  });
});

describe('BetSlipStore · apostar', () => {
  it('envía stake y cuotas vistas, vacía el boleto y refresca el saldo', () => {
    const { store, placed, wallet } = setup();
    store.toggle(pick(1, { oddX100: 214 }));
    store.toggle(pick(2, { oddX100: 197 }));
    store.setStake('10.50');
    store.place();

    expect(placed).toHaveLength(1);
    expect(placed[0].body).toEqual({
      stakeCents: 1050,
      selections: [
        { selectionId: 's1', oddX100: 214 },
        { selectionId: 's2', oddX100: 197 },
      ],
    });
    expect(store.result()).toEqual({ kind: 'success', bet: bet() });
    expect([store.count(), store.stakeText(), store.placing()]).toEqual([0, '', false]);
    expect(wallet.refreshed).toBe(1);
  });

  it('no envía si el boleto no es válido', () => {
    const { store, placed } = setup();
    store.place();
    store.toggle(pick(1));
    store.place(); // sin monto
    expect(placed).toHaveLength(0);
  });

  it('un doble clic envía una sola apuesta', () => {
    const pending = new Subject<Bet>();
    const { store, placed } = setup({ place: () => pending });
    store.toggle(pick(1));
    store.setStake('10');
    store.place();
    expect(store.placing()).toBe(true);
    expect(store.canPlace()).toBe(false);
    store.place();
    store.place();
    expect(placed).toHaveLength(1);

    pending.next(bet());
    expect(store.placing()).toBe(false);
  });

  it('un error de red se reintenta con la misma clave de idempotencia', () => {
    let fail = true;
    const { store, placed } = setup({
      place: () => (fail ? throwError(() => apiError(0, 'NETWORK_ERROR')) : of(bet())),
    });
    store.toggle(pick(1));
    store.setStake('10');
    store.place();
    expect(store.result()).toMatchObject({ kind: 'error', failure: { kind: 'network' } });
    expect(store.count()).toBe(1); // el boleto se conserva

    fail = false;
    store.place();
    expect(placed).toHaveLength(2);
    expect(placed[1].key).toBe(placed[0].key);
  });

  it('si el usuario cambia el monto o las selecciones, la clave es otra', () => {
    const { store, placed } = setup({
      place: () => throwError(() => apiError(0, 'NETWORK_ERROR')),
    });
    store.toggle(pick(1));
    store.setStake('10');
    store.place();
    store.setStake('20');
    store.place();
    store.toggle(pick(2));
    store.place();
    expect(new Set(placed.map((p) => p.key)).size).toBe(3);
  });

  it('tras una apuesta exitosa, la siguiente usa otra clave', () => {
    const { store, placed } = setup();
    for (const n of [1, 2]) {
      store.toggle(pick(n));
      store.setStake('10');
      store.place();
    }
    expect(placed).toHaveLength(2);
    expect(placed[1].key).not.toBe(placed[0].key);
  });

  it('saldo insuficiente: avisa, conserva el boleto y refresca el saldo', () => {
    const { store, wallet } = setup({
      place: () => throwError(() => apiError(422, 'INSUFFICIENT_FUNDS')),
    });
    store.toggle(pick(1));
    store.setStake('10');
    store.place();
    expect(store.result()).toMatchObject({
      kind: 'error',
      failure: { kind: 'insufficient-funds' },
    });
    expect(store.count()).toBe(1);
    expect(store.stakeText()).toBe('10');
    expect(wallet.refreshed).toBe(1);
  });

  it('el error de validación conserva los errores por campo', () => {
    const fields = { stakeCents: 'muy bajo' };
    const { store } = setup({
      place: () => throwError(() => apiError(400, 'VALIDATION_ERROR', fields)),
    });
    store.toggle(pick(1));
    store.setStake('10');
    store.place();
    expect(store.result()).toMatchObject({
      kind: 'error',
      failure: { kind: 'validation', fields },
    });
  });

  it('CORRELATED_SELECTIONS del servidor no borra nada', () => {
    const { store } = setup({
      place: () => throwError(() => apiError(422, 'CORRELATED_SELECTIONS')),
    });
    store.toggle(pick(1));
    store.toggle(pick(2));
    store.setStake('10');
    store.place();
    expect(store.result()).toMatchObject({ kind: 'error', failure: { kind: 'correlated' } });
    expect(store.count()).toBe(2);
  });
});

describe('BetSlipStore · cuota cambiada', () => {
  function oddsChanged() {
    return setup({
      place: (body) =>
        body.selections[0].oddX100 === 200
          ? throwError(() => apiError(409, 'ODDS_CHANGED'))
          : of(bet()),
      markets: (id) => of(id === 'e1' ? [market(1, 180)] : [market(2, 200)]),
    });
  }

  it('actualiza las cuotas desde el servidor y pide confirmar; nunca reenvía solo', () => {
    const { store, placed, marketCalls } = oddsChanged();
    store.toggle(pick(1, { oddX100: 200 }));
    store.toggle(pick(2, { oddX100: 200 }));
    store.setStake('10');
    store.place();

    expect(marketCalls.sort()).toEqual(['e1', 'e2']);
    const [first, second] = store.selections();
    expect([first.status, first.oddX100, first.previousOddX100]).toEqual(['changed', 180, 200]);
    expect([second.status, second.oddX100]).toEqual(['ok', 200]);
    expect(store.result()).toMatchObject({ kind: 'error', failure: { kind: 'odds-changed' } });
    expect(store.hasChanged()).toBe(true);
    expect(store.canPlace()).toBe(false);
    store.place();
    expect(placed).toHaveLength(1);
  });

  it('al confirmar, apuesta con la cuota nueva y otra clave', () => {
    const { store, placed } = oddsChanged();
    store.toggle(pick(1, { oddX100: 200 }));
    store.setStake('10');
    store.place();
    store.acceptChanges();
    expect(store.canPlace()).toBe(true);
    expect(store.result()).toBeNull();

    store.place();
    expect(placed).toHaveLength(2);
    expect(placed[1].body.selections[0].oddX100).toBe(180);
    expect(placed[1].key).not.toBe(placed[0].key);
    expect(store.result()).toMatchObject({ kind: 'success' });
  });

  it('la cuota total se recalcula con las cuotas nuevas', () => {
    const { store } = oddsChanged();
    store.toggle(pick(1, { oddX100: 200 }));
    store.toggle(pick(2, { oddX100: 200 }));
    store.setStake('10');
    expect(store.totalOddX100()).toBe(400);
    store.place();
    expect(store.totalOddX100()).toBe(360);
    expect(store.payoutCents()).toBe(3600);
  });

  it('si no se pueden leer los mercados, el boleto queda como estaba', () => {
    const { store } = setup({
      place: () => throwError(() => apiError(409, 'ODDS_CHANGED')),
      markets: () => throwError(() => new Error('boom')),
    });
    store.toggle(pick(1));
    store.setStake('10');
    store.place();
    expect(store.selections()[0].status).toBe('ok');
    expect(store.result()).toMatchObject({ kind: 'error', failure: { kind: 'odds-changed' } });
  });
});

describe('BetSlipStore · mercado cerrado', () => {
  it('marca la selección como cerrada y bloquea hasta quitarla', () => {
    const { store } = setup({
      place: () => throwError(() => apiError(422, 'MARKET_CLOSED')),
      markets: (id) => of(id === 'e1' ? [market(1, 200, 'CLOSED')] : [market(2, 200)]),
    });
    store.toggle(pick(1));
    store.toggle(pick(2));
    store.setStake('10');
    store.place();

    expect(store.selections()[0].status).toBe('closed');
    expect(store.hasClosed()).toBe(true);
    expect(store.canPlace()).toBe(false);

    store.remove('s1');
    expect(store.hasClosed()).toBe(false);
    expect(store.canPlace()).toBe(true);
  });
});
