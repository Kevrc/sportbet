import { TestBed } from '@angular/core/testing';
import { of, throwError } from 'rxjs';

import { AuditEntry } from '../../core/api/models';
import { describeActivity } from './activity';
import { AdminOverview } from './admin-overview';
import { AdminUsersService } from './admin-users.service';

const entry = (e: Partial<AuditEntry>): AuditEntry => ({
  id: 1,
  actorId: 'u1',
  actorName: 'Ana',
  action: 'BET_PLACED',
  entity: 'bet',
  entityId: 'b1',
  data: {},
  createdAt: '2026-10-12T15:00:00Z',
  ...e,
});

describe('describeActivity', () => {
  it('describe apuestas hechas y liquidadas', () => {
    expect(
      describeActivity(entry({ data: { stakeCents: 1000, totalOddX100: 214, selections: 1 } }))
        .text,
    ).toBe('Ana apostó 10.00 a cuota 2.14 (simple)');
    expect(
      describeActivity(entry({ data: { stakeCents: 500, totalOddX100: 640, selections: 3 } })).text,
    ).toContain('combinada de 3');
    expect(
      describeActivity(entry({ action: 'BET_SETTLED', data: { status: 'WON', payoutCents: 2140 } }))
        .text,
    ).toBe('Ana ganó 21.40');
    expect(
      describeActivity(entry({ action: 'BET_SETTLED', data: { status: 'LOST', payoutCents: 0 } }))
        .text,
    ).toBe('Ana perdió una apuesta');
    expect(
      describeActivity(
        entry({
          action: 'BET_SETTLED',
          actorName: null,
          data: { status: 'VOID', payoutCents: 500 },
        }),
      ).text,
    ).toBe('Apuesta de Un usuario anulada: se devolvieron 5.00');
  });
});

describe('AdminOverview', () => {
  function render(service: Partial<AdminUsersService>) {
    TestBed.configureTestingModule({
      imports: [AdminOverview],
      providers: [{ provide: AdminUsersService, useValue: service }],
    });
    const fixture = TestBed.createComponent(AdminOverview);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('muestra las cifras, el resultado de la casa y la actividad', () => {
    const el = render({
      stats: () =>
        of({ users: 4, betsPlaced: 10, betsSettled: 6, stakeCents: 12000, payoutCents: 8000 }),
      audit: () =>
        of({
          items: [entry({ data: { stakeCents: 1000, totalOddX100: 214, selections: 1 } })],
          page: 1,
          pageSize: 10,
          total: 1,
        }),
    });
    const values = [...el.querySelectorAll('dd')].map((d) => d.textContent!.trim());
    expect(values).toEqual(['4', '10', '6', '120.00', '80.00', '40.00']);
    expect(el.querySelector('.activity li')!.textContent).toContain('Ana apostó 10.00');
  });

  it('avisa si no hay actividad o si falla la carga', () => {
    let el = render({
      stats: () => of({ users: 1, betsPlaced: 0, betsSettled: 0, stakeCents: 0, payoutCents: 0 }),
      audit: () => of({ items: [], page: 1, pageSize: 10, total: 0 }),
    });
    expect(el.textContent).toContain('Todavía no hay apuestas registradas');

    TestBed.resetTestingModule();
    el = render({
      stats: () => throwError(() => new Error('x')),
      audit: () => of({ items: [], page: 1, pageSize: 10, total: 0 }),
    });
    expect(el.querySelector('[role=alert]')!.textContent).toContain('No se pudieron cargar');
  });
});
