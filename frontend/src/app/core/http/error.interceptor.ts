import { HttpErrorResponse, HttpInterceptorFn } from '@angular/common/http';
import { catchError, throwError } from 'rxjs';

import { ApiErrorBody } from '../api/models';

/** Error tipado que reciben los features: code viene del backend (p. ej. ODDS_CHANGED). */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly fields?: Record<string, string>,
  ) {
    super(message);
  }
}

export const errorInterceptor: HttpInterceptorFn = (req, next) =>
  next(req).pipe(
    catchError((err: HttpErrorResponse) => {
      const body = err.error as Partial<ApiErrorBody> | null;
      if (body?.error?.code) {
        return throwError(
          () => new ApiError(err.status, body.error!.code, body.error!.message, body.error!.fields),
        );
      }
      const code = err.status === 0 ? 'NETWORK_ERROR' : 'HTTP_' + err.status;
      return throwError(() => new ApiError(err.status, code, err.message));
    }),
  );
