// Rutas del feature wallet. Dueño: Persona 4.
import { Routes } from '@angular/router';

import { WalletPage } from './pages/wallet.page';

export default [
  { path: '', pathMatch: 'full', component: WalletPage, title: 'Billetera · SportBet' },
] satisfies Routes;
