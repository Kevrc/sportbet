import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { Observable, of, throwError } from 'rxjs';

import { EventDetail } from '../../../core/api/models';
import { ApiError } from '../../../core/http/error.interceptor';
import { AuthStore } from '../../../core/auth/auth.store';
import { LiveEventsService } from '../../../core/live/live-events.service';
import { BetSlipStore } from '../../betting/public-api';
import { BettingService } from '../../betting/betting.service';
import { WalletStore } from '../../wallet/public-api';
import { detail, fakeLive, market } from '../events-testing';
import { EventsService } from '../events.service';
import { EventDetailPage } from './event-detail.page';

function setup(get: (id: string) => Observable<EventDetail>, id = 'e1') {
  const calls: string[] = [];
  const live = fakeLive();
  const service = {
    get: (i: string) => (calls.push(i), get(i)),
    live: () => of([] as EventDetail[]),
  };
  TestBed.configureTestingModule({
    imports: [EventDetailPage],
    providers: [
      provideRouter([]),
      { provide: EventsService, useValue: service },
      { provide: LiveEventsService, useValue: live },
      { provide: BettingService, useValue: { placeBet: () => of(), eventMarkets: () => of([]) } },
      { provide: WalletStore, useValue: { balanceCents: signal(null), refresh: () => undefined } },
      { provide: AuthStore, useValue: { isAuthenticated: signal(true) } },
    ],
  });
  const fixture = TestBed.createComponent(EventDetailPage);
  fixture.componentRef.setInput('id', id);
  return {
    fixture,
    calls,
    live,
    slip: TestBed.inject(BetSlipStore),
    el: fixture.nativeElement as HTMLElement,
  };
}

async function settle(fixture: { whenStable(): Promise<unknown>; detectChanges(): void }) {
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
}

const farFuture = '2099-01-01T18:00:00Z';
const selections = (el: HTMLElement) => [...el.querySelectorAll<HTMLButtonElement>('.selection')];

describe('EventDetailPage', () => {
  it('muestra equipos, liga y las cuotas con dos decimales', async () => {
    const { fixture, el, calls } = setup(() => of(detail({ startAt: farFuture })));
    await settle(fixture);

    expect(calls).toEqual(['e1']);
    expect(el.textContent).toContain('Local e1');
    expect(el.textContent).toContain('Visita e1');
    expect(el.textContent).toContain('Resultado final (1X2)');
    expect(selections(el).map((b) => b.getAttribute('aria-label'))).toEqual([
      'Local, cuota 2.14',
      'Empate, cuota 3.30',
      'Visita, cuota 1.97',
    ]);
    expect(selections(el).map((b) => b.querySelector('.odd')!.textContent)).toEqual([
      '2.14',
      '3.30',
      '1.97',
    ]);
    expect(selections(el).every((b) => !b.disabled)).toBe(true);
  });

  it('deshabilita las selecciones de un mercado cerrado y dice por qué', async () => {
    const { fixture, el } = setup(() =>
      of(detail({ startAt: farFuture, markets: [market({ status: 'CLOSED' })] })),
    );
    await settle(fixture);
    expect(selections(el).every((b) => b.disabled)).toBe(true);
    expect(el.querySelector('.lock')!.textContent).toContain('Mercado cerrado');
  });

  it('deshabilita todo si el partido ya empezó', async () => {
    const { fixture, el } = setup(() =>
      of(detail({ status: 'LIVE', minute: 12, homeScore: 0, awayScore: 1, startAt: farFuture })),
    );
    await settle(fixture);
    expect(selections(el).every((b) => b.disabled)).toBe(true);
    expect(el.textContent).toContain('En vivo');
    expect(el.textContent).toContain("12'");
    expect(el.querySelector('.score')!.textContent).toContain('0 - 1');
  });

  it('muestra "vs" y no un marcador cuando el partido no ha empezado', async () => {
    const { fixture, el } = setup(() => of(detail({ startAt: farFuture })));
    await settle(fixture);
    expect(el.querySelector('.vs')!.textContent).toBe('vs');
    expect(el.querySelector('.score')).toBeNull();
  });

  it('avisa cuando el evento no tiene mercados', async () => {
    const { fixture, el } = setup(() => of(detail({ markets: [] })));
    await settle(fixture);
    expect(el.textContent).toContain('todavía no tiene mercados');
  });

  it('404: dice que el evento no existe, sin botón de reintento', async () => {
    const { fixture, el } = setup(() =>
      throwError(() => new ApiError(404, 'EVENT_NOT_FOUND', 'no existe')),
    );
    await settle(fixture);
    expect(el.textContent).toContain('no existe o ya no está disponible');
    expect(el.querySelector('[role="alert"]')).toBeNull();
  });

  it('otro error: muestra el aviso y reintenta con el mismo id', async () => {
    let fail = true;
    const { fixture, el, calls } = setup(() =>
      fail
        ? throwError(() => new ApiError(0, 'NETWORK_ERROR', 'x'))
        : of(detail({ startAt: farFuture })),
    );
    await settle(fixture);
    expect(el.querySelector('[role="alert"]')!.textContent).toContain('No se pudo cargar');

    fail = false;
    el.querySelector<HTMLButtonElement>('[role="alert"] button')!.click();
    await settle(fixture);
    expect(calls).toEqual(['e1', 'e1']);
    expect(selections(el)).toHaveLength(3);
  });

  it('si cambia el id de la ruta, carga el otro evento', async () => {
    const { fixture, calls } = setup(() => of(detail({ startAt: farFuture })));
    await settle(fixture);
    fixture.componentRef.setInput('id', 'e2');
    await settle(fixture);
    expect(calls).toEqual(['e1', 'e2']);
  });

  describe('en vivo', () => {
    it('al empezar el partido cierra los mercados y muestra el estado en vivo', async () => {
      const { fixture, el, live } = setup(() => of(detail({ startAt: farFuture })));
      await settle(fixture);
      expect(selections(el).every((b) => !b.disabled)).toBe(true);

      live.emit.started({ eventId: 'e1', startedAt: '2026-10-12T18:00:00Z' });
      await settle(fixture);
      expect(selections(el).every((b) => b.disabled)).toBe(true);
      expect(el.textContent).toContain('En vivo');
      expect(el.querySelector('.score')!.textContent).toContain('0 - 0');
    });

    it('actualiza marcador y minuto con match.updated', async () => {
      const { fixture, el, live } = setup(() => of(detail({ startAt: farFuture })));
      await settle(fixture);
      live.emit.updated({
        eventId: 'e1',
        status: 'LIVE',
        homeScore: 2,
        awayScore: 1,
        minute: 63,
        updatedAt: '2026-10-12T19:03:00Z',
      });
      await settle(fixture);
      expect(el.querySelector('.score')!.textContent).toContain('2 - 1');
      expect(el.textContent).toContain("63'");
    });

    it('marca "Desactualizado" si el dato en vivo tiene más de 5 minutos', async () => {
      const old = new Date(Date.now() - 10 * 60_000).toISOString();
      const { fixture, el } = setup(() =>
        of(detail({ status: 'LIVE', minute: 70, homeScore: 0, awayScore: 0, liveUpdatedAt: old })),
      );
      await settle(fixture);
      expect(el.querySelector('.stale')!.textContent).toContain('Desactualizado');
    });

    it('avisa si no hay conexión y, al reconectar, vuelve a pedir el evento', async () => {
      const { fixture, el, live, calls } = setup(() => of(detail({ startAt: farFuture })));
      await settle(fixture);
      live.connected.set(false);
      await settle(fixture);
      expect(el.querySelector('.offline')).not.toBeNull();

      live.reconnect();
      await settle(fixture);
      expect(calls).toEqual(['e1', 'e1']);
    });
  });

  describe('boleto', () => {
    it('tocar una cuota la añade al boleto y la marca; tocarla otra vez la quita', async () => {
      const { fixture, el, slip } = setup(() => of(detail({ startAt: farFuture })));
      await settle(fixture);

      selections(el)[0].click();
      await settle(fixture);
      expect(slip.selections()).toMatchObject([
        {
          selectionId: 's1',
          eventId: 'e1',
          eventName: 'Local e1 - Visita e1',
          marketId: 'm1',
          marketLabel: 'Resultado final (1X2)',
          label: 'Local',
          oddX100: 214,
        },
      ]);
      expect(selections(el)[0].getAttribute('aria-pressed')).toBe('true');
      expect(selections(el)[1].getAttribute('aria-pressed')).toBe('false');
      expect(el.querySelector('app-bet-slip')!.textContent).toContain('Local e1 - Visita e1');
      expect(el.querySelector('app-bet-slip')!.textContent).toContain('2.14');

      selections(el)[0].click();
      await settle(fixture);
      expect(slip.count()).toBe(0);
      expect(selections(el)[0].getAttribute('aria-pressed')).toBe('false');
    });

    it('otra opción del mismo mercado reemplaza a la anterior', async () => {
      const { fixture, el, slip } = setup(() => of(detail({ startAt: farFuture })));
      await settle(fixture);
      selections(el)[0].click();
      selections(el)[1].click();
      await settle(fixture);
      expect(slip.selections().map((s) => s.selectionId)).toEqual(['s2']);
    });

    it('otro mercado del mismo partido no se añade y se avisa', async () => {
      const second = market({
        id: 'm2',
        code: 'BOTH_TEAMS_TO_SCORE',
        selections: [{ id: 's9', code: 'YES', label: 'Sí', oddX100: 180, result: 'PENDING' }],
      });
      const { fixture, el, slip } = setup(() =>
        of(detail({ startAt: farFuture, markets: [market(), second] })),
      );
      await settle(fixture);
      selections(el)[0].click();
      selections(el)[3].click();
      await settle(fixture);
      expect(slip.count()).toBe(1);
      expect(el.querySelector('app-bet-slip .notice')!.textContent).toContain('mismo partido');
    });
  });

  describe('móvil', () => {
    it('muestra la fecha con el día y el mes en español', async () => {
      const { fixture, el } = setup(() => of(detail({ startAt: '2099-01-01T18:00:00Z' })));
      await settle(fixture);
      expect(el.querySelector('.meta')!.textContent).toContain('de enero');
      expect(el.querySelector('.meta')!.textContent).not.toMatch(/January|Thursday|Friday/);
    });

    it('la barra del boleto aparece al elegir una cuota y lleva al campo de monto', async () => {
      const scrolled: string[] = [];
      HTMLElement.prototype.scrollIntoView = function (this: HTMLElement) {
        scrolled.push(this.id);
      };
      const { fixture, el } = setup(() => of(detail({ startAt: farFuture })));
      await settle(fixture);
      expect(el.querySelector('.slip-bar')).toBeNull();

      selections(el)[0].click();
      await settle(fixture);
      const bar = el.querySelector<HTMLButtonElement>('.slip-bar')!;
      expect(bar.textContent!.replace(/\s+/g, ' ')).toContain('Boleto (1)');
      expect(bar.textContent).toContain('Cuota 2.14');

      bar.click();
      await settle(fixture);
      expect(scrolled).toEqual(['boleto']);
      expect(document.activeElement?.id).toBe('stake');

      selections(el)[1].click(); // reemplaza dentro del mismo mercado: sigue 1 selección
      selections(el)[0].click();
      await settle(fixture);
      expect(el.querySelector('.slip-bar')).not.toBeNull();
    });

    it('sin selecciones no hay barra', async () => {
      const { fixture, el } = setup(() => of(detail({ startAt: farFuture })));
      await settle(fixture);
      expect(el.querySelector('.slip-bar')).toBeNull();
    });
  });
});
