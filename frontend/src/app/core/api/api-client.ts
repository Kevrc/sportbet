import { HttpClient, HttpHeaders, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { environment } from '../../../environments/environment';

type Params = Record<string, string | number | boolean | undefined | null>;

/** Cliente HTTP base. Los features construyen sus servicios encima de este. */
@Injectable({ providedIn: 'root' })
export class ApiClient {
  private readonly http = inject(HttpClient);
  readonly baseUrl = environment.apiUrl;

  get<T>(path: string, params?: Params): Observable<T> {
    return this.http.get<T>(this.url(path), { params: toParams(params) });
  }

  post<T>(path: string, body?: unknown, opts?: { idempotencyKey?: string }): Observable<T> {
    let headers = new HttpHeaders();
    if (opts?.idempotencyKey) headers = headers.set('Idempotency-Key', opts.idempotencyKey);
    return this.http.post<T>(this.url(path), body ?? null, { headers, withCredentials: true });
  }

  patch<T>(path: string, body: unknown): Observable<T> {
    return this.http.patch<T>(this.url(path), body);
  }

  url(path: string): string {
    return `${this.baseUrl}${path}`;
  }
}

function toParams(params?: Params): HttpParams {
  let p = new HttpParams();
  for (const [k, v] of Object.entries(params ?? {})) {
    if (v !== undefined && v !== null && v !== '') p = p.set(k, String(v));
  }
  return p;
}
