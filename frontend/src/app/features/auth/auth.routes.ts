// Rutas del feature auth. Dueño: Persona 1.
import { Routes } from '@angular/router';

import { LoginPage } from './pages/login.page';
import { RegisterPage } from './pages/register.page';

export default [
  { path: 'login', component: LoginPage, title: 'Iniciar sesión · SportBet' },
  { path: 'register', component: RegisterPage, title: 'Crear cuenta · SportBet' },
] satisfies Routes;
