// Rutas del feature events. Dueño: Persona 4.
import { Routes } from '@angular/router';

import { EventListPage } from './pages/event-list.page';
import { EventDetailPage } from './pages/event-detail.page';

export default [
  { path: '', pathMatch: 'full', component: EventListPage, title: 'Eventos · SportBet' },
  { path: ':id', component: EventDetailPage, title: 'Detalle del evento · SportBet' },
] satisfies Routes;
