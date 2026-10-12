import { AuditEntry } from '../../core/api/models';
import { formatOdd } from '../betting/public-api';
import { formatCents } from '../wallet/public-api';

/** Datos que guardan BET_PLACED y BET_SETTLED (el JSON del evento del bus). */
interface BetData {
  stakeCents?: number;
  totalOddX100?: number;
  selections?: number;
  status?: 'WON' | 'LOST' | 'VOID';
  payoutCents?: number;
}

export interface ActivityLine {
  icon: string;
  text: string;
}

/** Frase legible de una entrada de auditoría: "Ana apostó 10.00 a cuota 2.14". */
export function describeActivity(e: AuditEntry): ActivityLine {
  const who = e.actorName ?? 'Un usuario';
  const d = (e.data ?? {}) as BetData;
  switch (e.action) {
    case 'BET_PLACED': {
      const kind = (d.selections ?? 1) > 1 ? `combinada de ${d.selections}` : 'simple';
      const odd = d.totalOddX100 ? ` a cuota ${formatOdd(d.totalOddX100)}` : '';
      return {
        icon: '🎟️',
        text: `${who} apostó ${formatCents(d.stakeCents ?? 0)}${odd} (${kind})`,
      };
    }
    case 'BET_SETTLED':
      switch (d.status) {
        case 'WON':
          return { icon: '🏆', text: `${who} ganó ${formatCents(d.payoutCents ?? 0)}` };
        case 'VOID':
          return {
            icon: '↩️',
            text: `Apuesta de ${who} anulada: se devolvieron ${formatCents(d.payoutCents ?? 0)}`,
          };
        default:
          return { icon: '❌', text: `${who} perdió una apuesta` };
      }
    default:
      return { icon: '•', text: `${who}: ${e.action}` };
  }
}
