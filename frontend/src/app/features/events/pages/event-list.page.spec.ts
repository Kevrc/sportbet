import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { Observable, of, throwError } from 'rxjs';

import { League, SportEvent } from '../../../core/api/models';
import { event, league } from '../events-testing';
import { EventFilters, EventsService } from '../events.service';
import { EventListPage } from './event-list.page';

function setup(opts: {
  list?: (f: EventFilters) => Observable<SportEvent[]>;
  leagues?: Observable<League[]>;
}) {
  const calls: EventFilters[] = [];
  const service = {
    list: (f: EventFilters) => {
      calls.push(f);
      return (opts.list ?? (() => of([])))(f);
    },
    leagues: () => opts.leagues ?? of([league('l1', { name: 'LaLiga' })]),
  };
  TestBed.configureTestingModule({
    imports: [EventListPage],
    providers: [provideRouter([]), { provide: EventsService, useValue: service }],
  });
  const fixture = TestBed.createComponent(EventListPage);
  return { fixture, calls, el: fixture.nativeElement as HTMLElement };
}

async function settle(fixture: { whenStable(): Promise<unknown>; detectChanges(): void }) {
  await fixture.whenStable();
  fixture.detectChanges();
}

const button = (el: HTMLElement, text: string) =>
  [...el.querySelectorAll('button')].find((b) => b.textContent!.includes(text))!;

function change(el: HTMLElement, selector: string, value: string) {
  const input = el.querySelector<HTMLInputElement | HTMLSelectElement>(selector)!;
  input.value = value;
  input.dispatchEvent(new Event('change'));
}

describe('EventListPage', () => {
  it('agrupa los eventos por liga y enlaza cada uno a su detalle', async () => {
    const laliga = league('l1', { name: 'LaLiga' });
    const premier = league('l2', { name: 'Premier' });
    const { fixture, el } = setup({
      list: () =>
        of([
          event('e1', { league: laliga }),
          event('e2', { league: premier }),
          event('e3', { league: laliga, status: 'LIVE', minute: 37, homeScore: 1, awayScore: 0 }),
        ]),
    });
    await settle(fixture);

    const titles = [...el.querySelectorAll('.league h2')].map((h) => h.textContent!.trim());
    expect(titles[0]).toContain('LaLiga');
    expect(titles[1]).toContain('Premier');
    const links = [...el.querySelectorAll<HTMLAnchorElement>('a.event')];
    expect(links).toHaveLength(3);
    expect(links.map((a) => a.getAttribute('href'))).toContain('/events/e2');
    const live = links.find((a) => a.textContent!.includes('Local e3'))!;
    expect(live.textContent).toContain('En vivo');
    expect(live.textContent).toContain("37'");
    expect(live.textContent).toContain('1 - 0');
  });

  it('muestra un mensaje cuando no hay eventos', async () => {
    const { fixture, el } = setup({});
    await settle(fixture);
    expect(el.textContent).toContain('No hay eventos disponibles');
  });

  it('muestra el error y permite reintentar', async () => {
    let fail = true;
    const { fixture, el, calls } = setup({
      list: () => (fail ? throwError(() => new Error('boom')) : of([event('e1')])),
    });
    await settle(fixture);
    expect(el.querySelector('[role="alert"]')!.textContent).toContain('No se pudieron cargar');
    expect(el.textContent).not.toContain('No hay eventos');

    fail = false;
    button(el, 'Reintentar').click();
    await settle(fixture);
    expect(el.querySelector('[role="alert"]')).toBeNull();
    expect(el.querySelectorAll('a.event')).toHaveLength(1);
    expect(calls).toHaveLength(2);
  });

  it('cada filtro vuelve a pedir la lista con sus parámetros', async () => {
    const { fixture, el, calls } = setup({});
    await settle(fixture);
    expect(calls[0]).toEqual({});

    change(el, 'select', 'l1');
    await settle(fixture);
    expect(calls[1]).toEqual({ leagueId: 'l1' });

    change(el, 'input[type="date"]', '2026-10-12');
    await settle(fixture);
    expect(calls[2]).toEqual({ leagueId: 'l1', date: '2026-10-12' });

    const featured = el.querySelector<HTMLInputElement>('input[type="checkbox"]')!;
    featured.checked = true;
    featured.dispatchEvent(new Event('change'));
    await settle(fixture);
    expect(calls[3]).toEqual({ leagueId: 'l1', date: '2026-10-12', featured: true });
  });

  it('"Limpiar filtros" quita todos los filtros y avisa cuando no hay resultados', async () => {
    const { fixture, el, calls } = setup({});
    await settle(fixture);
    expect(el.textContent).not.toContain('Limpiar filtros');

    change(el, 'select', 'l1');
    await settle(fixture);
    expect(el.textContent).toContain('No hay eventos con esos filtros');

    button(el, 'Limpiar filtros').click();
    await settle(fixture);
    expect(calls.at(-1)).toEqual({});
    expect(el.textContent).not.toContain('Limpiar filtros');
  });

  it('si fallan las ligas, la lista de eventos igual se muestra', async () => {
    const { fixture, el } = setup({
      leagues: throwError(() => new Error('boom')),
      list: () => of([event('e1')]),
    });
    await settle(fixture);
    expect(el.querySelectorAll('a.event')).toHaveLength(1);
  });
});
