import { HttpInterceptorFn, HttpRequest } from '@angular/common/http';
import { inject } from '@angular/core';
import { catchError, switchMap, throwError } from 'rxjs';

import { environment } from '../../../environments/environment';
import { AuthStore } from './auth.store';

/** Agrega el Bearer token y, ante un 401, renueva el token una vez y reintenta. */
export const authInterceptor: HttpInterceptorFn = (req, next) => {
  const store = inject(AuthStore);
  if (!req.url.startsWith(environment.apiUrl) || req.url.includes('/auth/')) {
    return next(req);
  }
  const withToken = (r: HttpRequest<unknown>, t: string | null) =>
    t ? r.clone({ setHeaders: { Authorization: `Bearer ${t}` } }) : r;

  return next(withToken(req, store.accessToken())).pipe(
    // err es ApiError (errorInterceptor corre antes en la respuesta); ambos tienen status.
    catchError((err: { status?: number }) => {
      if (err.status !== 401 || !store.accessToken()) return throwError(() => err);
      return store.refresh().pipe(switchMap((t) => next(withToken(req, t))));
    }),
  );
};
