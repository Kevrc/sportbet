import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { Observable, Subject, of, throwError } from 'rxjs';

import { Bet, Market, PlaceBetRequest } from '../../../core/api/models';
import { AuthStore } from '../../../core/auth/auth.store';
import { ApiError } from '../../../core/http/error.interceptor';
import { WalletStore } from '../../wallet/public-api';
import { BetSlipStore, SlipPick } from '../bet-slip.store';
import { BettingService } from '../betting.service';
import { BetSlipComponent } from './bet-slip.component';

const pick = (n: number, extra: Partial<SlipPick> = {}): SlipPick => ({
  selectionId: `s${n}`,
  eventId: `e${n}`,
  eventName: `Local ${n} - Visita ${n}`,
  marketId: `m${n}`,
  marketCode: 'MATCH_RESULT',
  marketLabel: 'Resultado final (1X2)',
  label: 'Local',
  oddX100: 214,
  ...extra,
});

const bet = (): Bet => ({
  id: 'b1',
  stakeCents: 1000,
  totalOddX100: 214,
  potentialPayoutCents: 2140,
  status: 'PENDING',
  payoutCents: 0,
  placedAt: '2026-10-12T18:00:00Z',
  selections: [],
});

function setup(
  opts: {
    place?: (b: PlaceBetRequest, key: string) => Observable<Bet>;
    markets?: () => Observable<Market[]>;
    loggedIn?: boolean;
    balance?: number | null;
  } = {},
) {
  const placed: PlaceBetRequest[] = [];
  const balance = signal<number | null>(opts.balance === undefined ? 100_000 : opts.balance);
  TestBed.configureTestingModule({
    imports: [BetSlipComponent],
    providers: [
      provideRouter([]),
      {
        provide: BettingService,
        useValue: {
          placeBet: (b: PlaceBetRequest, k: string) => (
            placed.push(b),
            (opts.place ?? (() => of(bet())))(b, k)
          ),
          eventMarkets: () => (opts.markets ?? (() => of([])))(),
        },
      },
      { provide: WalletStore, useValue: { balanceCents: balance, refresh: () => undefined } },
      { provide: AuthStore, useValue: { isAuthenticated: signal(opts.loggedIn ?? true) } },
    ],
  });
  const router = TestBed.inject(Router);
  const navigated: unknown[][] = [];
  router.navigate = ((...args: unknown[]) => (
    navigated.push(args),
    Promise.resolve(true)
  )) as never;
  const fixture = TestBed.createComponent(BetSlipComponent);
  return {
    fixture,
    placed,
    navigated,
    slip: TestBed.inject(BetSlipStore),
    el: fixture.nativeElement as HTMLElement,
  };
}

async function settle(fixture: { whenStable(): Promise<unknown>; detectChanges(): void }) {
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
}

function type(el: HTMLElement, value: string) {
  const input = el.querySelector<HTMLInputElement>('#stake')!;
  input.value = value;
  input.dispatchEvent(new Event('input'));
}

const placeButton = (el: HTMLElement) => el.querySelector<HTMLButtonElement>('button.primary')!;
const text = (el: HTMLElement) => el.textContent!.replace(/\s+/g, ' ');

describe('BetSlipComponent', () => {
  it('vacío: invita a elegir una cuota', async () => {
    const { fixture, el } = setup();
    await settle(fixture);
    expect(text(el)).toContain('Elige una cuota para empezar');
    expect(el.querySelector('button.primary')).toBeNull();
  });

  it('simple: muestra la selección, la cuota y calcula el pago potencial', async () => {
    const { fixture, el, slip } = setup();
    slip.toggle(pick(1));
    await settle(fixture);
    type(el, '10');
    await settle(fixture);

    expect(text(el)).toContain('Simple');
    expect(text(el)).toContain('Local 1 - Visita 1');
    expect(text(el)).toContain('Resultado final (1X2)');
    expect(text(el)).toContain('Cuota total 2.14');
    expect(text(el)).toMatch(/Pago potencial\s*21\.40/);
    expect(placeButton(el).disabled).toBe(false);
  });

  it('combinada: multiplica las cuotas (2.14 × 1.97 = 4.21)', async () => {
    const { fixture, el, slip } = setup();
    slip.toggle(pick(1, { oddX100: 214 }));
    slip.toggle(pick(2, { oddX100: 197 }));
    await settle(fixture);
    type(el, '10');
    await settle(fixture);
    expect(text(el)).toContain('Combinada (2)');
    expect(text(el)).toContain('Cuota total 4.21');
    expect(text(el)).toMatch(/Pago potencial\s*42\.10/);
  });

  it('muestra el saldo y valida el monto, deshabilitando el botón', async () => {
    const { fixture, el, slip } = setup({ balance: 5_000 });
    slip.toggle(pick(1));
    await settle(fixture);
    expect(text(el)).toContain('Saldo disponible: 50.00');
    expect(placeButton(el).disabled).toBe(true); // sin monto

    type(el, '0.5');
    await settle(fixture);
    expect(el.querySelector('#stake-error')!.textContent).toContain('mínimo');
    expect(el.querySelector('#stake')!.getAttribute('aria-invalid')).toBe('true');
    expect(placeButton(el).disabled).toBe(true);

    type(el, '60');
    await settle(fixture);
    expect(el.querySelector('#stake-error')!.textContent).toContain('saldo');

    type(el, '20');
    await settle(fixture);
    expect(el.querySelector('#stake-error')).toBeNull();
    expect(placeButton(el).disabled).toBe(false);
  });

  it('quitar una selección y vaciar el boleto', async () => {
    const { fixture, el, slip } = setup();
    slip.toggle(pick(1));
    slip.toggle(pick(2));
    await settle(fixture);
    el.querySelector<HTMLButtonElement>('button.remove')!.click();
    await settle(fixture);
    expect(slip.count()).toBe(1);

    [...el.querySelectorAll<HTMLButtonElement>('button.link')]
      .find((b) => b.textContent!.includes('Vaciar'))!
      .click();
    await settle(fixture);
    expect(slip.count()).toBe(0);
    expect(text(el)).toContain('Elige una cuota');
  });

  it('apostar: envía una sola apuesta aunque se haga doble clic y muestra la confirmación', async () => {
    const pending = new Subject<Bet>();
    const { fixture, el, slip, placed } = setup({ place: () => pending });
    slip.toggle(pick(1));
    await settle(fixture);
    type(el, '10');
    await settle(fixture);

    placeButton(el).click();
    placeButton(el).click();
    await settle(fixture);
    expect(placed).toHaveLength(1);
    expect(placeButton(el).textContent).toContain('Apostando');
    expect(placeButton(el).disabled).toBe(true);

    pending.next(bet());
    await settle(fixture);
    expect(text(el)).toContain('¡Apuesta realizada!');
    expect(text(el)).toMatch(/Pago potencial\s*21\.40/);
    expect(el.querySelector<HTMLAnchorElement>('.success a')!.getAttribute('href')).toBe('/bets');
    expect(slip.count()).toBe(0);

    [...el.querySelectorAll<HTMLButtonElement>('button.link')]
      .find((b) => b.textContent!.includes('Cerrar'))!
      .click();
    await settle(fixture);
    expect(el.querySelector('.success')).toBeNull();
  });

  it('saldo insuficiente del servidor: muestra el aviso y conserva el boleto', async () => {
    const { fixture, el, slip } = setup({
      place: () => throwError(() => new ApiError(422, 'INSUFFICIENT_FUNDS', 'x')),
    });
    slip.toggle(pick(1));
    await settle(fixture);
    type(el, '10');
    await settle(fixture);
    placeButton(el).click();
    await settle(fixture);
    expect(el.querySelector('[role="alert"].error')!.textContent).toContain('Saldo insuficiente');
    expect(slip.count()).toBe(1);
    expect(el.querySelector<HTMLInputElement>('#stake')!.value).toBe('10');
  });

  it('error de validación: muestra los errores por campo', async () => {
    const { fixture, el, slip } = setup({
      place: () =>
        throwError(() => new ApiError(400, 'VALIDATION_ERROR', 'x', { stakeCents: 'muy bajo' })),
    });
    slip.toggle(pick(1));
    await settle(fixture);
    type(el, '10');
    await settle(fixture);
    placeButton(el).click();
    await settle(fixture);
    expect(text(el)).toContain('stakeCents: muy bajo');
  });

  it('cuota cambiada: tacha la anterior, bloquea apostar hasta aceptar y luego apuesta', async () => {
    const changed: Market = {
      id: 'm1',
      eventId: 'e1',
      code: 'MATCH_RESULT',
      status: 'OPEN',
      selections: [{ id: 's1', code: 'HOME', label: 'Local', oddX100: 190, result: 'PENDING' }],
    };
    let first = true;
    const { fixture, el, slip, placed } = setup({
      place: () => {
        if (first) {
          first = false;
          return throwError(() => new ApiError(409, 'ODDS_CHANGED', 'x'));
        }
        return of(bet());
      },
      markets: () => of([changed]),
    });
    slip.toggle(pick(1, { oddX100: 214 }));
    await settle(fixture);
    type(el, '10');
    await settle(fixture);
    placeButton(el).click();
    await settle(fixture);

    expect(text(el)).toContain('Las cuotas cambiaron');
    expect(el.querySelector('.old')!.textContent).toContain('2.14');
    expect(el.querySelector('.odd')!.textContent).toContain('1.90');
    expect(text(el)).toContain('La cuota cambió');
    expect(placeButton(el).disabled).toBe(true);
    expect(placed).toHaveLength(1);

    [...el.querySelectorAll<HTMLButtonElement>('button.secondary')]
      .find((b) => b.textContent!.includes('Aceptar cuotas nuevas'))!
      .click();
    await settle(fixture);
    expect(placeButton(el).disabled).toBe(false);
    expect(text(el)).toContain('Cuota total 1.90');

    placeButton(el).click();
    await settle(fixture);
    expect(placed).toHaveLength(2);
    expect(placed[1].selections[0].oddX100).toBe(190);
    expect(text(el)).toContain('¡Apuesta realizada!');
  });

  it('mercado cerrado: marca la selección y no deja apostar hasta quitarla', async () => {
    const closed: Market = {
      id: 'm1',
      eventId: 'e1',
      code: 'MATCH_RESULT',
      status: 'CLOSED',
      selections: [{ id: 's1', code: 'HOME', label: 'Local', oddX100: 214, result: 'PENDING' }],
    };
    const { fixture, el, slip } = setup({
      place: () => throwError(() => new ApiError(422, 'MARKET_CLOSED', 'x')),
      markets: () => of([closed]),
    });
    slip.toggle(pick(1));
    await settle(fixture);
    type(el, '10');
    await settle(fixture);
    placeButton(el).click();
    await settle(fixture);
    expect(text(el)).toContain('Mercado cerrado: quítala para apostar');
    expect(placeButton(el).disabled).toBe(true);
  });

  it('avisa cuando se intenta combinar dos mercados del mismo partido', async () => {
    const { fixture, el, slip } = setup();
    slip.toggle(pick(1));
    slip.toggle(pick(1, { selectionId: 'x', marketId: 'mx' }));
    await settle(fixture);
    expect(el.querySelector('.notice')!.textContent).toContain('mismo partido');
    [...el.querySelectorAll<HTMLButtonElement>('.notice button')][0].click();
    await settle(fixture);
    expect(el.querySelector('.notice')).toBeNull();
  });

  it('sin sesión: el botón lleva a iniciar sesión conservando el boleto', async () => {
    const { fixture, el, slip, navigated } = setup({ loggedIn: false });
    slip.toggle(pick(1));
    await settle(fixture);
    expect(text(el)).not.toContain('Saldo disponible');
    expect(placeButton(el).textContent).toContain('Inicia sesión para apostar');
    expect(placeButton(el).disabled).toBe(false);

    placeButton(el).click();
    expect(navigated[0][0]).toEqual(['/auth/login']);
    expect(slip.count()).toBe(1);
  });

  it('si el servidor devuelve la apuesta incompleta, la confirmación no muestra NaN', async () => {
    const partial = { id: 'b9', totalOddX100: 214, potentialPayoutCents: 2140 } as Bet;
    const { fixture, el, slip } = setup({ place: () => of(partial) });
    slip.toggle(pick(1));
    await settle(fixture);
    type(el, '10');
    await settle(fixture);
    placeButton(el).click();
    await settle(fixture);
    expect(text(el)).toContain('¡Apuesta realizada!');
    expect(text(el)).not.toContain('NaN');
    expect(text(el)).toMatch(/Monto\s*10\.00/);
    expect(text(el)).toMatch(/Cuota total\s*2\.14/);
    expect(text(el)).toMatch(/Pago potencial\s*21\.40/);
  });
});
