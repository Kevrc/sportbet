import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { ApiClient } from '../../core/api/api-client';
import { Paged, Role, UserSummary } from '../../core/api/models';

export interface UserChanges {
  role?: Role;
  status?: 'active' | 'blocked';
}

/** Gestión de usuarios para el admin: GET /admin/users y PATCH /admin/users/{id}. */
@Injectable({ providedIn: 'root' })
export class AdminUsersService {
  private readonly api = inject(ApiClient);

  list(page: number, pageSize: number): Observable<Paged<UserSummary>> {
    return this.api.get<Paged<UserSummary>>('/admin/users', { page, pageSize });
  }

  update(id: string, changes: UserChanges): Observable<UserSummary> {
    return this.api.patch<UserSummary>(`/admin/users/${encodeURIComponent(id)}`, changes);
  }
}
