// Rutas del feature auth. Dueño: Persona 1.
import { Routes } from '@angular/router';

import { guestGuard } from './guest.guard';
import { LoginPage } from './pages/login.page';
import { RegisterPage } from './pages/register.page';

export default [
  { path: '', pathMatch: 'full', redirectTo: 'login' },
  {
    path: 'login',
    canMatch: [guestGuard],
    component: LoginPage,
    title: 'Iniciar sesión · SportBet',
  },
  {
    path: 'register',
    canMatch: [guestGuard],
    component: RegisterPage,
    title: 'Crear cuenta · SportBet',
  },
] satisfies Routes;
