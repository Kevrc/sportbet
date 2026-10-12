import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';

import { UserSummary } from '../../../core/api/models';
import { AuthStore } from '../../../core/auth/auth.store';
import { ApiError } from '../../../core/http/error.interceptor';
import { AdminOverview } from '../admin-overview';
import { AdminUsersService, UserChanges } from '../admin-users.service';

const PAGE_SIZE = 20;

const ERRORS: Record<string, string> = {
  CANNOT_MODIFY_SELF: 'No puedes quitarte el rol de admin ni bloquear tu propia cuenta.',
  USER_NOT_FOUND: 'El usuario ya no existe. Recarga la lista.',
  FORBIDDEN: 'Tu sesión ya no tiene permisos de administrador.',
};

@Component({
  selector: 'app-admin-users-page',
  imports: [DatePipe, AdminOverview],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './admin-users.page.html',
  styleUrl: './admin-users.page.scss',
})
export class AdminUsersPage {
  private readonly service = inject(AdminUsersService);
  private readonly auth = inject(AuthStore);
  protected readonly currentUserId = computed(() => this.auth.user()?.id);

  protected readonly users = signal<UserSummary[]>([]);
  protected readonly page = signal(1);
  protected readonly total = signal(0);
  protected readonly loading = signal(true);
  protected readonly error = signal<string | null>(null);
  /** ID del usuario con un cambio en curso; deshabilita sus botones. */
  protected readonly pendingId = signal<string | null>(null);

  protected readonly totalPages = computed(() => Math.max(1, Math.ceil(this.total() / PAGE_SIZE)));

  constructor() {
    this.load(1);
  }

  protected load(page: number): void {
    this.loading.set(true);
    this.error.set(null);
    this.service.list(page, PAGE_SIZE).subscribe({
      next: (res) => {
        this.users.set(res.items);
        this.total.set(res.total);
        this.page.set(res.page);
        this.loading.set(false);
      },
      error: (err) => {
        this.error.set(message(err, 'No se pudo cargar la lista de usuarios.'));
        this.loading.set(false);
      },
    });
  }

  protected isSelf(u: UserSummary): boolean {
    return u.id === this.currentUserId();
  }

  protected toggleStatus(u: UserSummary): void {
    this.apply(u, { status: u.status === 'active' ? 'blocked' : 'active' });
  }

  protected toggleRole(u: UserSummary): void {
    this.apply(u, { role: u.role === 'admin' ? 'user' : 'admin' });
  }

  private apply(u: UserSummary, changes: UserChanges): void {
    this.pendingId.set(u.id);
    this.error.set(null);
    this.service.update(u.id, changes).subscribe({
      next: (updated) => {
        this.users.update((list) => list.map((x) => (x.id === updated.id ? updated : x)));
        this.pendingId.set(null);
      },
      error: (err) => {
        this.error.set(message(err, 'No se pudo guardar el cambio.'));
        this.pendingId.set(null);
      },
    });
  }
}

function message(err: unknown, fallback: string): string {
  return (err instanceof ApiError && ERRORS[err.code]) || fallback;
}
