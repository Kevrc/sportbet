import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Observable, of, throwError } from 'rxjs';

import { Paged, UserSummary } from '../../../core/api/models';
import { AuthStore } from '../../../core/auth/auth.store';
import { ApiError } from '../../../core/http/error.interceptor';
import { AdminUsersService, UserChanges } from '../admin-users.service';
import { AdminUsersPage } from './admin-users.page';

const user = (id: string, extra: Partial<UserSummary> = {}): UserSummary => ({
  id,
  email: `${id}@x.com`,
  displayName: id.toUpperCase(),
  role: 'user',
  status: 'active',
  createdAt: '2026-10-09T12:00:00Z',
  ...extra,
});

function setup(opts: {
  list?: (page: number) => Observable<Paged<UserSummary>>;
  update?: (id: string, c: UserChanges) => Observable<UserSummary>;
}) {
  const calls: { list: number[]; update: [string, UserChanges][] } = { list: [], update: [] };
  const service = {
    list: (page: number) => {
      calls.list.push(page);
      return (opts.list ?? (() => of({ items: [], page, pageSize: 20, total: 0 })))(page);
    },
    update: (id: string, c: UserChanges) => {
      calls.update.push([id, c]);
      return (opts.update ?? (() => of(user(id))))(id, c);
    },
  };
  TestBed.configureTestingModule({
    imports: [AdminUsersPage],
    providers: [
      { provide: AdminUsersService, useValue: service },
      { provide: AuthStore, useValue: { user: signal({ id: 'admin1' }) } },
    ],
  });
  const fixture = TestBed.createComponent(AdminUsersPage);
  return { fixture, calls, el: fixture.nativeElement as HTMLElement };
}

const rows = (el: HTMLElement) => [...el.querySelectorAll('tbody tr')];
const button = (row: Element, text: string) =>
  [...row.querySelectorAll('button')].find((b) => b.textContent!.includes(text))!;

describe('AdminUsersPage', () => {
  const page1 = () =>
    of({
      items: [user('admin1', { role: 'admin' }), user('ana'), user('beto', { status: 'blocked' })],
      page: 1,
      pageSize: 20,
      total: 3,
    });

  it('lista usuarios y no permite acciones sobre la propia cuenta', async () => {
    const { fixture, el, calls } = setup({ list: page1 });
    await fixture.whenStable();
    expect(calls.list).toEqual([1]);
    expect(rows(el).length).toBe(3);
    expect(el.textContent).toContain('Usuarios registrados: 3');
    expect(rows(el)[0].querySelectorAll('button').length).toBe(0);
    expect(rows(el)[0].textContent).toContain('(tú)');
    expect(button(rows(el)[2], 'Desbloquear')).toBeTruthy();
  });

  it('bloquea a un usuario y actualiza su fila', async () => {
    const { fixture, el, calls } = setup({
      list: page1,
      update: (id) => of(user(id, { status: 'blocked' })),
    });
    await fixture.whenStable();
    button(rows(el)[1], 'Bloquear').click();
    await fixture.whenStable();
    expect(calls.update).toEqual([['ana', { status: 'blocked' }]]);
    expect(rows(el)[1].textContent).toContain('Bloqueado');
    expect(button(rows(el)[1], 'Desbloquear')).toBeTruthy();
  });

  it('cambia el rol', async () => {
    const { fixture, el, calls } = setup({
      list: page1,
      update: (id) => of(user(id, { role: 'admin' })),
    });
    await fixture.whenStable();
    button(rows(el)[1], 'Hacer admin').click();
    await fixture.whenStable();
    expect(calls.update).toEqual([['ana', { role: 'admin' }]]);
    expect(button(rows(el)[1], 'Quitar admin')).toBeTruthy();
  });

  it('muestra el error del servidor y no cambia la fila', async () => {
    const { fixture, el } = setup({
      list: page1,
      update: () => throwError(() => new ApiError(404, 'USER_NOT_FOUND', 'x')),
    });
    await fixture.whenStable();
    button(rows(el)[1], 'Bloquear').click();
    await fixture.whenStable();
    expect(el.querySelector('[role=alert]')!.textContent).toContain('ya no existe');
    expect(rows(el)[1].textContent).toContain('Activo');
  });

  it('pagina de 20 en 20', async () => {
    const { fixture, el, calls } = setup({
      list: (page) => of({ items: [user('u' + page)], page, pageSize: 20, total: 45 }),
    });
    await fixture.whenStable();
    expect(el.textContent).toContain('Página 1 de 3');
    button(el.querySelector('.pager')!, 'Siguiente').click();
    await fixture.whenStable();
    expect(calls.list).toEqual([1, 2]);
    expect(el.textContent).toContain('Página 2 de 3');
  });
});
