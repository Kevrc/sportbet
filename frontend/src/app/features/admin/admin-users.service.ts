import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { ApiClient } from '../../core/api/api-client';
import { AdminStats, AuditEntry, Paged, Role, UserSummary } from '../../core/api/models';

export interface UserChanges {
  role?: Role;
  status?: 'active' | 'blocked';
}

/** Panel del admin: usuarios (GET/PATCH /admin/users), estadísticas y actividad. */
@Injectable({ providedIn: 'root' })
export class AdminUsersService {
  private readonly api = inject(ApiClient);

  list(page: number, pageSize: number): Observable<Paged<UserSummary>> {
    return this.api.get<Paged<UserSummary>>('/admin/users', { page, pageSize });
  }

  update(id: string, changes: UserChanges): Observable<UserSummary> {
    return this.api.patch<UserSummary>(`/admin/users/${encodeURIComponent(id)}`, changes);
  }

  stats(): Observable<AdminStats> {
    return this.api.get<AdminStats>('/admin/stats');
  }

  audit(page: number, pageSize: number): Observable<Paged<AuditEntry>> {
    return this.api.get<Paged<AuditEntry>>('/admin/audit', { page, pageSize });
  }
}
