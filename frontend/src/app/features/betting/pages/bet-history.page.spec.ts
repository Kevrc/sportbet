import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { Observable, of, throwError } from 'rxjs';

import { Bet, BetSelection, BetStatus, Paged } from '../../../core/api/models';
import { BettingService } from '../betting.service';
import { BetHistoryPage } from './bet-history.page';

const sel = (n: number, extra: Partial<BetSelection> = {}): BetSelection => ({
  selectionId: `s${n}`,
  eventId: `e${n}`,
  eventName: `Local ${n} - Visita ${n}`,
  marketCode: 'MATCH_RESULT',
  selectionCode: 'HOME',
  label: 'Gana local',
  oddX100: 214,
  result: 'PENDING',
  ...extra,
});

const bet = (id: string, extra: Partial<Bet> = {}): Bet => ({
  id,
  stakeCents: 1000,
  totalOddX100: 214,
  potentialPayoutCents: 2140,
  status: 'PENDING',
  payoutCents: 0,
  placedAt: '2026-10-12T18:00:00Z',
  selections: [sel(1)],
  ...extra,
});

type Params = { status?: BetStatus; page: number; pageSize: number };

function setup(list?: (p: Params) => Observable<Paged<Bet>>) {
  const calls: Params[] = [];
  const service = {
    list: (p: Params) => {
      calls.push(p);
      return (list ?? ((q) => of({ items: [], page: q.page, pageSize: q.pageSize, total: 0 })))(p);
    },
  };
  TestBed.configureTestingModule({
    imports: [BetHistoryPage],
    providers: [provideRouter([]), { provide: BettingService, useValue: service }],
  });
  const fixture = TestBed.createComponent(BetHistoryPage);
  return { fixture, calls, el: fixture.nativeElement as HTMLElement };
}

async function settle(fixture: { whenStable(): Promise<unknown>; detectChanges(): void }) {
  await fixture.whenStable();
  fixture.detectChanges();
}

const cards = (el: HTMLElement) => [...el.querySelectorAll<HTMLElement>('li.bet')];
const text = (e: Element) => e.textContent!.replace(/\s+/g, ' ');
const button = (el: HTMLElement, label: string) =>
  [...el.querySelectorAll('button')].find((b) => b.textContent!.includes(label))!;

describe('BetHistoryPage', () => {
  it('muestra una apuesta simple pendiente con su pago potencial', async () => {
    const { fixture, el } = setup((p) =>
      of({ items: [bet('b1')], page: p.page, pageSize: p.pageSize, total: 1 }),
    );
    await settle(fixture);

    const [card] = cards(el);
    expect(text(card)).toContain('Simple');
    expect(text(card)).toContain('Pendiente');
    expect(text(card)).toContain('Local 1 - Visita 1');
    expect(text(card)).toContain('Resultado final (1X2)');
    expect(text(card)).toContain('Gana local');
    expect(text(card)).toMatch(/Monto\s*10\.00/);
    expect(text(card)).toMatch(/Cuota total\s*2\.14/);
    expect(text(card)).toMatch(/Pago potencial\s*21\.40/);
    expect(card.dataset['status']).toBe('PENDING');
  });

  it('muestra la cuota congelada de cada selección en una combinada', async () => {
    const combo = bet('b2', {
      totalOddX100: 421,
      potentialPayoutCents: 4210,
      selections: [sel(1, { oddX100: 214 }), sel(2, { oddX100: 197 })],
    });
    const { fixture, el } = setup((p) =>
      of({ items: [combo], page: p.page, pageSize: p.pageSize, total: 1 }),
    );
    await settle(fixture);
    const [card] = cards(el);
    expect(text(card)).toContain('Combinada (2)');
    expect([...card.querySelectorAll('.selections .odd')].map((o) => o.textContent)).toEqual([
      '2.14',
      '1.97',
    ]);
    expect(text(card)).toMatch(/Cuota total\s*4\.21/);
  });

  it('ganada: muestra el pago recibido; perdida: sin pago; anulada: reembolso', async () => {
    const items = [
      bet('w', { status: 'WON', payoutCents: 2140, selections: [sel(1, { result: 'WON' })] }),
      bet('l', { status: 'LOST', selections: [sel(2, { result: 'LOST' })] }),
      bet('v', {
        status: 'VOID',
        payoutCents: 1000,
        selections: [sel(3, { result: 'VOID' })],
      }),
    ];
    const { fixture, el } = setup((p) =>
      of({ items, page: p.page, pageSize: p.pageSize, total: 3 }),
    );
    await settle(fixture);
    const [won, lost, voided] = cards(el);

    expect(text(won)).toContain('Ganada');
    expect(text(won)).toMatch(/Pago recibido\s*21\.40/);
    expect(text(lost)).toContain('Perdida');
    expect(text(lost)).toMatch(/Sin pago\s*—/);
    expect(text(voided)).toContain('Anulada');
    expect(text(voided)).toMatch(/Reembolsado\s*10\.00/);
    expect(text(voided)).toContain('cuenta como 1.00');
    expect([won, lost, voided].map((c) => c.dataset['status'])).toEqual(['WON', 'LOST', 'VOID']);
  });

  it('sin apuestas: invita a ver los eventos', async () => {
    const { fixture, el } = setup();
    await settle(fixture);
    expect(text(el)).toContain('Todavía no has apostado');
    expect(el.querySelector<HTMLAnchorElement>('.empty a')!.getAttribute('href')).toBe('/events');
  });

  it('cuando el servidor devuelve items null lo trata como lista vacía', async () => {
    const { fixture, el } = setup((p) =>
      of({ items: null as unknown as Bet[], page: p.page, pageSize: p.pageSize, total: 0 }),
    );
    await settle(fixture);
    expect(cards(el)).toHaveLength(0);
    expect(text(el)).toContain('Todavía no has apostado');
  });

  it('el filtro de estado vuelve a pedir la lista desde la página 1', async () => {
    const { fixture, el, calls } = setup((p) =>
      of({ items: [bet('b1')], page: p.page, pageSize: p.pageSize, total: 35 }),
    );
    await settle(fixture);
    expect(calls[0]).toEqual({ status: undefined, page: 1, pageSize: 10 });

    button(el, 'Siguiente').click();
    await settle(fixture);
    expect(calls[1].page).toBe(2);

    const select = el.querySelector('select')!;
    select.value = 'WON';
    select.dispatchEvent(new Event('change'));
    await settle(fixture);
    expect(calls[2]).toEqual({ status: 'WON', page: 1, pageSize: 10 });
  });

  it('con un filtro sin resultados lo dice', async () => {
    const { fixture, el } = setup();
    await settle(fixture);
    const select = el.querySelector('select')!;
    select.value = 'LOST';
    select.dispatchEvent(new Event('change'));
    await settle(fixture);
    expect(text(el)).toContain('No tienes apuestas con ese estado');
  });

  it('pagina y deshabilita los extremos', async () => {
    const { fixture, el, calls } = setup((p) =>
      of({ items: [bet(`p${p.page}`)], page: p.page, pageSize: p.pageSize, total: 25 }),
    );
    await settle(fixture);
    expect(text(el.querySelector('.pager')!)).toContain('Página 1 de 3');
    expect(button(el, 'Anterior').disabled).toBe(true);

    button(el, 'Siguiente').click();
    await settle(fixture);
    expect(calls.map((c) => c.page)).toEqual([1, 2]);
    expect(text(el.querySelector('.pager')!)).toContain('Página 2 de 3');
  });

  it('error: avisa y permite reintentar sin decir que no hay apuestas', async () => {
    let fail = true;
    const { fixture, el, calls } = setup((p) =>
      fail
        ? throwError(() => new Error('405'))
        : of({ items: [bet('b1')], page: p.page, pageSize: p.pageSize, total: 1 }),
    );
    await settle(fixture);
    expect(el.querySelector('[role="alert"]')!.textContent).toContain('No se pudo cargar');
    expect(text(el)).not.toContain('Todavía no has apostado');

    fail = false;
    button(el, 'Reintentar').click();
    await settle(fixture);
    expect(el.querySelector('[role="alert"]')).toBeNull();
    expect(cards(el)).toHaveLength(1);
    expect(calls).toHaveLength(2);
  });
});
