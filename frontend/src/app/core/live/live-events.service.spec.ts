import { TestBed } from '@angular/core/testing';

import { LiveEventsService } from './live-events.service';

describe('LiveEventsService.isStale', () => {
  it('marca desactualizado un dato de más de 5 minutos', () => {
    const svc = TestBed.inject(LiveEventsService);
    const now = Date.parse('2026-10-10T12:00:00Z');
    expect(svc.isStale('2026-10-10T11:54:00Z', now)).toBe(true);
    expect(svc.isStale('2026-10-10T11:58:00Z', now)).toBe(false);
    expect(svc.isStale(null, now)).toBe(true);
  });
});
