import { TestBed } from '@angular/core/testing';
import { Observable, of, throwError } from 'rxjs';

import { Paged, Wallet, WalletTransaction } from '../../../core/api/models';
import { WalletService } from '../wallet.service';
import { WalletPage } from './wallet.page';

const tx = (id: string, extra: Partial<WalletTransaction> = {}): WalletTransaction => ({
  id,
  type: 'BET_STAKE',
  amountCents: -1000,
  balanceAfterCents: 99_000,
  reference: `bet:${id}`,
  createdAt: '2026-10-09T12:00:00Z',
  ...extra,
});

function setup(opts: {
  wallet?: Observable<Wallet>;
  txs?: (page: number) => Observable<Paged<WalletTransaction>>;
}) {
  const calls: number[] = [];
  const service = {
    get: () => opts.wallet ?? of({ balanceCents: 99_000, currency: 'VIRT' as const }),
    transactions: (page: number) => {
      calls.push(page);
      return (opts.txs ?? (() => of({ items: [], page, pageSize: 20, total: 0 })))(page);
    },
  };
  TestBed.configureTestingModule({
    imports: [WalletPage],
    providers: [{ provide: WalletService, useValue: service }],
  });
  const fixture = TestBed.createComponent(WalletPage);
  return { fixture, calls, el: fixture.nativeElement as HTMLElement };
}

const rows = (el: HTMLElement) => [...el.querySelectorAll('tbody tr')];
const button = (el: HTMLElement, text: string) =>
  [...el.querySelectorAll('button')].find((b) => b.textContent!.includes(text))!;

describe('WalletPage', () => {
  it('muestra el saldo y los movimientos con signo y saldo posterior', async () => {
    const { fixture, el } = setup({
      txs: (page) =>
        of({
          items: [
            tx('a', { type: 'BET_PAYOUT', amountCents: 4210, balanceAfterCents: 103_210 }),
            tx('b'),
          ],
          page,
          pageSize: 20,
          total: 2,
        }),
    });
    await fixture.whenStable();
    fixture.detectChanges();

    expect(el.querySelector('.balance-value')!.textContent).toContain('990.00');
    const [first, second] = rows(el);
    expect(first.textContent).toContain('Premio');
    expect(first.textContent).toContain('+42.10');
    expect(first.textContent).toContain('1 032.10');
    expect(second.textContent).toContain('Apuesta');
    expect(second.textContent).toContain('-10.00');
  });

  it('muestra un mensaje cuando no hay movimientos', async () => {
    const { fixture, el } = setup({});
    await fixture.whenStable();
    fixture.detectChanges();
    expect(rows(el)[0].textContent).toContain('Todavía no tienes movimientos');
  });

  it('muestra el error y permite reintentar', async () => {
    let fail = true;
    const { fixture, el, calls } = setup({
      txs: (page) =>
        fail
          ? throwError(() => new Error('boom'))
          : of({ items: [tx('a')], page, pageSize: 20, total: 1 }),
    });
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.querySelector('[role="alert"]')!.textContent).toContain('No se pudieron cargar');
    expect(rows(el)).toHaveLength(0);

    fail = false;
    button(el, 'Reintentar').click();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.querySelector('[role="alert"]')).toBeNull();
    expect(rows(el)).toHaveLength(1);
    expect(calls).toEqual([1, 1]);
  });

  it('pagina: pide la página siguiente y deshabilita los extremos', async () => {
    const { fixture, el, calls } = setup({
      txs: (page) => of({ items: [tx(`p${page}`)], page, pageSize: 20, total: 45 }),
    });
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.querySelector('.pager')!.textContent).toContain('Página 1 de 3');
    expect(button(el, 'Anterior').disabled).toBe(true);

    button(el, 'Siguiente').click();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(calls).toEqual([1, 2]);
    expect(el.querySelector('.pager')!.textContent).toContain('Página 2 de 3');
  });

  it('un saldo de cero se muestra como 0.00, no como vacío', async () => {
    const { fixture, el } = setup({ wallet: of({ balanceCents: 0, currency: 'VIRT' }) });
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.querySelector('.balance-value')!.textContent).toContain('0.00');
  });
});
