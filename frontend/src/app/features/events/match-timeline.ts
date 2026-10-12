import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import { MatchEvent } from '../../core/api/models';

interface Row {
  key: string;
  side: 'HOME' | 'AWAY';
  minute: string;
  icon: string;
  title: string;
  detail: string | null;
}

const VAR_DETAILS: Record<string, string> = {
  'Goal cancelled': 'Gol anulado',
  'Goal Disallowed': 'Gol anulado',
  'Goal confirmed': 'Gol confirmado',
  'Penalty confirmed': 'Penal confirmado',
  'Penalty cancelled': 'Penal anulado',
  'Card upgrade': 'Tarjeta cambiada',
};

/** "45+2'" o "18'". */
export function minuteText(e: Pick<MatchEvent, 'minute' | 'extra'>): string {
  return e.extra ? `${e.minute}+${e.extra}'` : `${e.minute}'`;
}

/** Icono, título y detalle legibles de un suceso del proveedor. */
export function describeEvent(e: MatchEvent): Pick<Row, 'icon' | 'title' | 'detail'> {
  const player = e.player ?? 'Jugador sin nombre';
  const detail = e.detail.toLowerCase();
  switch (e.type) {
    case 'GOAL':
      if (detail.includes('missed')) return { icon: '❌', title: player, detail: 'Penal fallado' };
      return {
        icon: '⚽',
        title: player,
        // API-Football asigna el autogol al equipo que se beneficia, así que
        // aparece del lado de quien suma el gol y lo marca un rival.
        detail: detail.includes('own')
          ? 'Autogol del rival'
          : detail.includes('penalty')
            ? 'De penal'
            : e.assist
              ? `Asistencia: ${e.assist}`
              : null,
      };
    case 'CARD':
      return detail.includes('red') || detail.includes('second')
        ? { icon: '🟥', title: player, detail: 'Tarjeta roja' }
        : { icon: '🟨', title: player, detail: 'Tarjeta amarilla' };
    case 'SUBST':
      return { icon: '🔄', title: e.assist ? `${player} ⇄ ${e.assist}` : player, detail: 'Cambio' };
    case 'VAR':
      return {
        icon: '📺',
        title: VAR_DETAILS[e.detail] ?? `VAR: ${e.detail}`,
        detail: e.player ?? null,
      };
  }
}

/** Línea de tiempo del partido: los sucesos del local a la izquierda y los de la visita a la derecha. */
@Component({
  selector: 'app-match-timeline',
  changeDetection: ChangeDetectionStrategy.OnPush,
  styleUrl: './match-timeline.scss',
  template: `
    <section class="timeline" aria-labelledby="timeline-title">
      <h2 id="timeline-title">Lo que pasó en el partido</h2>
      @if (rows().length) {
        <ol>
          @for (r of rows(); track r.key) {
            <li [class.away]="r.side === 'AWAY'">
              <span class="minute">{{ r.minute }}</span>
              <span class="icon" aria-hidden="true">{{ r.icon }}</span>
              <span class="text">
                <strong>{{ r.title }}</strong>
                @if (r.detail) {
                  <small>{{ r.detail }}</small>
                }
                <span class="visually-hidden">({{ r.side === 'HOME' ? home() : away() }})</span>
              </span>
            </li>
          }
        </ol>
      } @else {
        <p class="empty">Todavía no hay goles, tarjetas ni cambios.</p>
      }
    </section>
  `,
})
export class MatchTimeline {
  readonly events = input.required<MatchEvent[]>();
  readonly home = input.required<string>();
  readonly away = input.required<string>();

  protected readonly rows = computed<Row[]>(() =>
    this.events().map((e, i) => ({
      key: `${i}-${e.minute}-${e.type}-${e.player ?? ''}`,
      side: e.side,
      minute: minuteText(e),
      ...describeEvent(e),
    })),
  );
}
