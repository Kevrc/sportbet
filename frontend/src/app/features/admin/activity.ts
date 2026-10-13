import { AuditEntry } from '../../core/api/models';
import { formatOdd } from '../betting/public-api';
import { formatCents } from '../wallet/public-api';

interface BetLeg {
  eventName?: string;
  pick?: string;
  oddX100?: number;
}

/** Datos que guardan BET_PLACED y BET_SETTLED (el JSON del evento del bus). */
interface BetData {
  stakeCents?: number;
  totalOddX100?: number;
  selections?: number;
  status?: 'WON' | 'LOST' | 'VOID';
  payoutCents?: number;
  legs?: BetLeg[];
}

export interface ActivityLine {
  icon: string;
  text: string;
  /** Código corto de la apuesta, igual en su colocación y su liquidación. */
  code?: string;
  /** "Barcelona vs Real Madrid · Empate @ 3.81", una por selección. */
  legs: string[];
}

function describeLegs(d: BetData | undefined): string[] {
  return (d?.legs ?? []).map(
    (l) =>
      `${l.eventName ?? 'Partido'} · ${l.pick ?? ''}${l.oddX100 ? ` @ ${formatOdd(l.oddX100)}` : ''}`,
  );
}

/** Frase legible de una entrada de auditoría: "Ana apostó 10.00 a cuota 2.14". */
export function describeActivity(e: AuditEntry): ActivityLine {
  const who = e.actorName ?? 'Un usuario';
  const line = describeAction(e, who, (e.data ?? {}) as BetData);
  const placed = (e.action === 'BET_SETTLED' ? e.placed : e.data) as BetData | undefined;
  const code = e.entityId ? `#${e.entityId.slice(0, 8)}` : undefined;
  return { ...line, code, legs: describeLegs(placed) };
}

function describeAction(e: AuditEntry, who: string, d: BetData): Omit<ActivityLine, 'legs'> {
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
          return {
            icon: '❌',
            text: `${who} perdió una apuesta${e.placed?.['stakeCents'] ? ` de ${formatCents(e.placed['stakeCents'] as number)}` : ''}`,
          };
      }
    default:
      return { icon: '•', text: `${who}: ${e.action}` };
  }
}
