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
  styles: `
    :host {
      display: inline-flex;
      flex: none;
      width: 22px;
      height: 22px;
      align-items: center;
      justify-content: center;
    }
    :host(.lg) {
      width: 56px;
      height: 56px;
    }
    img {
      width: 100%;
      height: 100%;
      object-fit: contain;
    }
    .initials {
      display: grid;
      place-items: center;
      width: 100%;
      height: 100%;
      border-radius: 50%;
      background: #ededf1;
      color: #5b5b66;
      font-size: 0.6rem;
      font-weight: 700;
    }
    :host(.lg) .initials {
      font-size: 1.1rem;
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
