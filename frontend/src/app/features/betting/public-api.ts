// API pública del feature betting. Otros features solo importan desde aquí
// (o desde core/), nunca archivos internos de este feature.
export { BetSlipComponent } from './components/bet-slip.component';
export { BetSlipStore } from './bet-slip.store';
export type { SlipPick } from './bet-slip.store';
export { formatOdd } from './odds';
