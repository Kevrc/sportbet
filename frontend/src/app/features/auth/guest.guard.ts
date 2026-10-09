import { inject } from '@angular/core';
import { CanMatchFn, Router } from '@angular/router';

import { AuthStore } from '../../core/auth/auth.store';

/** Login y registro solo para quien no tiene sesión; con sesión va a eventos. */
export const guestGuard: CanMatchFn = () => {
  return !inject(AuthStore).isAuthenticated() || inject(Router).createUrlTree(['/events']);
};

/** Acepta solo rutas internas para evitar redirecciones abiertas (?returnUrl=https://…). */
export function safeReturnUrl(url: string | null | undefined): string {
  if (!url || !url.startsWith('/') || url.startsWith('//') || url.startsWith('/auth'))
    return '/events';
  return url;
}
