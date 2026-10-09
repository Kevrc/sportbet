import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { ApiClient } from '../../core/api/api-client';
import { Paged, Wallet, WalletTransaction } from '../../core/api/models';

/** GET /wallet y GET /wallet/transactions. */
@Injectable({ providedIn: 'root' })
export class WalletService {
  private readonly api = inject(ApiClient);

  get(): Observable<Wallet> {
    return this.api.get<Wallet>('/wallet');
  }

  transactions(page: number, pageSize: number): Observable<Paged<WalletTransaction>> {
    return this.api.get<Paged<WalletTransaction>>('/wallet/transactions', { page, pageSize });
  }
}
