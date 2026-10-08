// Rutas del feature admin. Dueño: Persona 1.
import { Routes } from '@angular/router';

import { AdminDashboardPage } from './pages/admin-dashboard.page';

export default [
  { path: '', pathMatch: 'full', component: AdminDashboardPage, title: 'Panel de administración · SportBet' },
] satisfies Routes;
