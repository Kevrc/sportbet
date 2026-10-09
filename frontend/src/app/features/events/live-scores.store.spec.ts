import { TestBed } from '@angular/core/testing';
import { Observable, of, throwError } from 'rxjs';

import { SportEvent } from '../../core/api/models';
import { LiveEventsService } from '../../core/live/live-events.service';
import { event, fakeLive } from './events-testing';
import { EventsService } from './events.service';
import { LiveScoresStore } from './live-scores.store';

function setup(live$: () => Observable<SportEvent[]> = () => of([])) {
  const live = fakeLive();
  const calls = { n: 0 };
  TestBed.configureTestingModule({
    providers: [
      LiveScoresStore,
      { provide: LiveEventsService, useValue: live },
      { provide: EventsService, useValue: { live: () => (calls.n++, live$()) } },
    ],
  });
  return { store: TestBed.inject(LiveScoresStore), live, calls };
}

const updated = (at: string, h = 0, a = 0, minute = 10) => ({
  eventId: 'e1',
  status: 'LIVE' as const,
  homeScore: h,
  awayScore: a,
  minute,
  updatedAt: at,
});

describe('LiveScoresStore', () => {
  it('sin datos en vivo devuelve el evento tal cual', () => {
    const { store } = setup();
    const e = event('e1');
    expect(store.apply(e)).toBe(e);
  });

  it('match.updated cambia estado, marcador y minuto', () => {
    const { store, live } = setup();
    live.emit.updated(updated('2026-10-12T18:10:00Z', 1, 0, 10));
    const e = store.apply(event('e1'));
    expect([e.status, e.homeScore, e.awayScore, e.minute]).toEqual(['LIVE', 1, 0, 10]);
  });

  it('match.started pone LIVE con 0 - 0 y conserva el marcador si ya había uno', () => {
    const { store, live } = setup();
    live.emit.started({ eventId: 'e1', startedAt: '2026-10-12T18:00:00Z' });
    let e = store.apply(event('e1'));
    expect([e.status, e.homeScore, e.awayScore]).toEqual(['LIVE', 0, 0]);

    live.emit.updated(updated('2026-10-12T18:30:00Z', 2, 1, 30));
    live.emit.started({ eventId: 'e1', startedAt: '2026-10-12T18:40:00Z' });
    e = store.apply(event('e1'));
    expect([e.homeScore, e.awayScore]).toEqual([2, 1]);
  });

  it('match.finished fija el resultado y quita el minuto', () => {
    const { store, live } = setup();
    live.emit.updated(updated('2026-10-12T19:00:00Z', 1, 1, 80));
    live.emit.finished({
      eventId: 'e1',
      status: 'FINISHED',
      homeScore: 2,
      awayScore: 1,
      finishedAt: '2026-10-12T19:50:00Z',
    });
    const e = store.apply(event('e1'));
    expect([e.status, e.homeScore, e.awayScore, e.minute]).toEqual(['FINISHED', 2, 1, null]);
  });

  it('ignora un mensaje más viejo que el estado que ya tiene', () => {
    const { store, live } = setup();
    live.emit.updated(updated('2026-10-12T18:30:00Z', 2, 0, 30));
    live.emit.updated(updated('2026-10-12T18:20:00Z', 1, 0, 20));
    expect(store.apply(event('e1')).homeScore).toBe(2);
  });

  it('un partido terminado no vuelve a estar en juego', () => {
    const { store, live } = setup();
    live.emit.finished({
      eventId: 'e1',
      status: 'FINISHED',
      homeScore: 1,
      awayScore: 0,
      finishedAt: '2026-10-12T19:50:00Z',
    });
    live.emit.updated(updated('2026-10-12T19:55:00Z', 5, 5, 95));
    live.emit.started({ eventId: 'e1', startedAt: '2026-10-12T19:56:00Z' });
    const e = store.apply(event('e1'));
    expect([e.status, e.homeScore, e.awayScore]).toEqual(['FINISHED', 1, 0]);
  });

  it('un evento cargado más reciente que el SSE manda sobre este', () => {
    const { store, live } = setup();
    live.emit.updated(updated('2026-10-12T18:10:00Z', 1, 0, 10));
    const fresher = event('e1', {
      status: 'LIVE',
      homeScore: 3,
      awayScore: 0,
      liveUpdatedAt: '2026-10-12T18:50:00Z',
    });
    expect(store.apply(fresher)).toBe(fresher);
  });

  it('al abrir carga GET /events/live y aplica esos partidos', () => {
    const { store, calls } = setup(() =>
      of([
        event('e1', {
          status: 'LIVE',
          homeScore: 1,
          awayScore: 1,
          minute: 44,
          liveUpdatedAt: '2026-10-12T18:44:00Z',
        }),
      ]),
    );
    expect(calls.n).toBe(1);
    const e = store.apply(event('e1'));
    expect([e.status, e.homeScore, e.minute]).toEqual(['LIVE', 1, 44]);
  });

  it('si /events/live falla, sigue funcionando con el SSE', () => {
    const { store, live } = setup(() => throwError(() => new Error('boom')));
    live.emit.updated(updated('2026-10-12T18:10:00Z', 1, 0, 10));
    expect(store.apply(event('e1')).homeScore).toBe(1);
  });

  it('al reconectar vuelve a pedir /events/live y avisa con resynced$', () => {
    const { store, live, calls } = setup();
    let resynced = 0;
    store.resynced$.subscribe(() => resynced++);
    live.reconnect();
    expect(calls.n).toBe(2);
    expect(resynced).toBe(1);
  });

  it('isStale solo aplica a partidos en vivo con dato de más de 5 minutos', () => {
    const { store } = setup();
    const old = new Date(Date.now() - 10 * 60_000).toISOString();
    const fresh = new Date().toISOString();
    expect(store.isStale({ status: 'LIVE', liveUpdatedAt: old })).toBe(true);
    expect(store.isStale({ status: 'LIVE', liveUpdatedAt: fresh })).toBe(false);
    expect(store.isStale({ status: 'LIVE', liveUpdatedAt: null })).toBe(true);
    expect(store.isStale({ status: 'FINISHED', liveUpdatedAt: old })).toBe(false);
    expect(store.isStale({ status: 'SCHEDULED', liveUpdatedAt: null })).toBe(false);
  });

  it('deja de escuchar el SSE al destruirse', () => {
    const { store, live } = setup();
    TestBed.resetTestingModule();
    live.emit.updated(updated('2026-10-12T18:10:00Z', 9, 9, 10));
    expect(store.apply(event('e1')).homeScore).not.toBe(9);
  });
});
