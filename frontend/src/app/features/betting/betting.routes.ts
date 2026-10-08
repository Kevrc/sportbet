// Rutas del feature betting. Dueño: Persona 4.
import { Routes } from '@angular/router';

import { BetHistoryPage } from './pages/bet-history.page';

export default [
  { path: '', pathMatch: 'full', component: BetHistoryPage, title: 'Mis apuestas · SportBet' },
] satisfies Routes;
