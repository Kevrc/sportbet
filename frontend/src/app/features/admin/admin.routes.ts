// Rutas del feature admin. Dueño: Persona 1.
import { Routes } from '@angular/router';

import { AdminUsersPage } from './pages/admin-users.page';

export default [
  {
    path: '',
    pathMatch: 'full',
    component: AdminUsersPage,
    title: 'Administración · SportBet',
  },
] satisfies Routes;
