import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';

import { ApiClient } from '../../core/api/api-client';
import { EventsService } from './events.service';

describe('EventsService', () => {
  function setup() {
    const calls: [string, Record<string, unknown> | undefined][] = [];
    const api = {
      get: (path: string, params?: Record<string, unknown>) => (calls.push([path, params]), of([])),
    };
    TestBed.configureTestingModule({ providers: [{ provide: ApiClient, useValue: api }] });
    return { service: TestBed.inject(EventsService), calls };
  }

  it('envía los filtros como parámetros de GET /events', () => {
    const { service, calls } = setup();
    service
      .list({ date: '2026-10-12', leagueId: 'l1', status: 'LIVE', featured: true })
      .subscribe();
    expect(calls[0]).toEqual([
      '/events',
      { date: '2026-10-12', leagueId: 'l1', status: 'LIVE', featured: true },
    ]);
  });

  it('featured=false se omite en vez de filtrar por no destacados', () => {
    const { service, calls } = setup();
    service.list({ featured: false }).subscribe();
    expect(calls[0][1]!['featured']).toBeUndefined();
  });

  it('GET /events/{id} codifica el id', () => {
    const { service, calls } = setup();
    service.get('a/b').subscribe();
    expect(calls[0][0]).toBe('/events/a%2Fb');
  });

  it('GET /leagues', () => {
    const { service, calls } = setup();
    service.leagues().subscribe();
    expect(calls[0][0]).toBe('/leagues');
  });
});
