import { ChangeDetectionStrategy, Component, computed, input, signal } from '@angular/core';

import { Team } from '../../core/api/models';

/**
 * Escudo del equipo. Si no hay URL (p. ej. en el replay) o la imagen no carga,
 * muestra las iniciales. Es decorativo: el nombre siempre va al lado.
 */
@Component({
  selector: 'app-team-crest',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { '[class.lg]': "size() === 'lg'", 'aria-hidden': 'true' },
  styleUrl: './team-crest.scss',
  template: `
    @if (team().logoUrl && !failed()) {
      <img
        [src]="team().logoUrl"
        alt=""
        [width]="px()"
        [height]="px()"
        loading="lazy"
        referrerpolicy="no-referrer"
        (error)="failed.set(true)"
      />
    } @else {
      <span class="initials">{{ initials() }}</span>
    }
  `,
})
export class TeamCrest {
  readonly team = input.required<Team>();
  readonly size = input<'sm' | 'lg'>('sm');

  protected readonly failed = signal(false);
  protected readonly px = computed(() => (this.size() === 'lg' ? 56 : 22));
  protected readonly initials = computed(() =>
    this.team()
      .name.split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((w) => w[0]!.toUpperCase())
      .join(''),
  );
}
