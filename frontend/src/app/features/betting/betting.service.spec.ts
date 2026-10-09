import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';

import { ApiClient } from '../../core/api/api-client';
import { BettingService } from './betting.service';

describe('BettingService', () => {
  function setup() {
    const calls: { method: string; path: string; arg?: unknown; opts?: unknown }[] = [];
    const api = {
      get: (path: string, arg?: unknown) => (calls.push({ method: 'get', path, arg }), of(null)),
      post: (path: string, arg?: unknown, opts?: unknown) => (
        calls.push({ method: 'post', path, arg, opts }),
        of(null)
      ),
    };
    TestBed.configureTestingModule({ providers: [{ provide: ApiClient, useValue: api }] });
    return { service: TestBed.inject(BettingService), calls };
  }

  it('POST /bets envía el cuerpo y la Idempotency-Key', () => {
    const { service, calls } = setup();
    const body = { stakeCents: 1000, selections: [{ selectionId: 's1', oddX100: 214 }] };
    service.placeBet(body, 'clave-1').subscribe();
    expect(calls[0]).toEqual({
      method: 'post',
      path: '/bets',
      arg: body,
      opts: { idempotencyKey: 'clave-1' },
    });
  });

  it('GET /bets con estado y paginación', () => {
    const { service, calls } = setup();
    service.list({ status: 'WON', page: 2, pageSize: 20 }).subscribe();
    expect(calls[0]).toMatchObject({
      path: '/bets',
      arg: { status: 'WON', page: 2, pageSize: 20 },
    });
  });

  it('GET /bets/{id} y /events/{id}/markets codifican el id', () => {
    const { service, calls } = setup();
    service.get('a/b').subscribe();
    service.eventMarkets('x y').subscribe();
    expect(calls.map((c) => c.path)).toEqual(['/bets/a%2Fb', '/events/x%20y/markets']);
  });
});
