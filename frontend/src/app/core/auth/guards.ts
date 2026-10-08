import { inject } from '@angular/core';
import { CanMatchFn, Router } from '@angular/router';

import { AuthStore } from './auth.store';

export const authGuard: CanMatchFn = (_route, segments) => {
  const store = inject(AuthStore);
  if (store.isAuthenticated()) return true;
  const returnUrl = '/' + segments.map((s) => s.path).join('/');
  return inject(Router).createUrlTree(['/auth/login'], { queryParams: { returnUrl } });
};

export const adminGuard: CanMatchFn = () => {
  return inject(AuthStore).isAdmin() || inject(Router).createUrlTree(['/events']);
};
