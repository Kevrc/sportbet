import { Injectable, computed, inject, signal } from '@angular/core';
import { Observable, catchError, finalize, map, of, shareReplay, tap, throwError } from 'rxjs';

import { ApiClient } from '../api/api-client';
import { AuthResponse, LoginRequest, RegisterRequest, User } from '../api/models';

/**
 * Estado de sesión. El token de acceso vive solo en memoria; el refresh token
 * va en una cookie httpOnly que el navegador envía a /auth/refresh.
 */
@Injectable({ providedIn: 'root' })
export class AuthStore {
  private readonly api = inject(ApiClient);

  private readonly _accessToken = signal<string | null>(null);
  private readonly _user = signal<User | null>(null);
  private refreshing$: Observable<string> | null = null;

  readonly accessToken = this._accessToken.asReadonly();
  readonly user = this._user.asReadonly();
  readonly isAuthenticated = computed(() => this._user() !== null);
  readonly isAdmin = computed(() => this._user()?.role === 'admin');

  login(body: LoginRequest): Observable<User> {
    return this.api.post<AuthResponse>('/auth/login', body).pipe(map((r) => this.setSession(r)));
  }

  register(body: RegisterRequest): Observable<User> {
    return this.api.post<AuthResponse>('/auth/register', body).pipe(map((r) => this.setSession(r)));
  }

  /** Una sola llamada a /auth/refresh aunque varios requests reciban 401 a la vez. */
  refresh(): Observable<string> {
    this.refreshing$ ??= this.api.post<AuthResponse>('/auth/refresh').pipe(
      map((r) => {
        this.setSession(r);
        return r.accessToken;
      }),
      catchError((e) => {
        this.clear();
        return throwError(() => e);
      }),
      finalize(() => (this.refreshing$ = null)),
      shareReplay(1),
    );
    return this.refreshing$;
  }

  /** Recupera la sesión al recargar la página. Nunca falla. */
  tryRefresh(): Observable<unknown> {
    return this.refresh().pipe(catchError(() => of(null)));
  }

  logout(): Observable<unknown> {
    return this.api.post('/auth/logout').pipe(
      catchError(() => of(null)),
      tap(() => this.clear()),
    );
  }

  clear(): void {
    this._accessToken.set(null);
    this._user.set(null);
  }

  private setSession(r: AuthResponse): User {
    this._accessToken.set(r.accessToken);
    this._user.set(r.user);
    return r.user;
  }
}
