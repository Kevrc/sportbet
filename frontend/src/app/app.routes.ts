// CONGELADO después del commit base. Cada feature define sus rutas en su
// propio <feature>.routes.ts y se carga de forma diferida.
import { Routes } from '@angular/router';

import { adminGuard, authGuard } from './core/auth/guards';

export const routes: Routes = [
  { path: '', pathMatch: 'full', redirectTo: 'events' },
  { path: 'auth', loadChildren: () => import('./features/auth/auth.routes') },
  { path: 'events', loadChildren: () => import('./features/events/events.routes') },
  {
    path: 'bets',
    canMatch: [authGuard],
    loadChildren: () => import('./features/betting/betting.routes'),
  },
  {
    path: 'wallet',
    canMatch: [authGuard],
    loadChildren: () => import('./features/wallet/wallet.routes'),
  },
  {
    path: 'admin',
    canMatch: [authGuard, adminGuard],
    loadChildren: () => import('./features/admin/admin.routes'),
  },
  { path: '**', redirectTo: 'events' },
];
