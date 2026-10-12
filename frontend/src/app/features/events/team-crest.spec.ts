import { TestBed } from '@angular/core/testing';

import { Team } from '../../core/api/models';
import { TeamCrest } from './team-crest';

function render(team: Team) {
  const fixture = TestBed.createComponent(TeamCrest);
  fixture.componentRef.setInput('team', team);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement };
}

describe('TeamCrest', () => {
  it('muestra el escudo cuando hay URL', () => {
    const { el } = render({
      id: '1',
      name: 'Liverpool',
      logoUrl: 'https://media.api-sports.io/football/teams/40.png',
    });
    const img = el.querySelector('img')!;
    expect(img.getAttribute('src')).toBe('https://media.api-sports.io/football/teams/40.png');
    expect(img.getAttribute('alt')).toBe('');
  });

  it('muestra las iniciales sin URL, como en el replay', () => {
    const { el } = render({ id: '1', name: 'Atlético Demo', logoUrl: null });
    expect(el.querySelector('img')).toBeNull();
    expect(el.textContent!.trim()).toBe('AD');
  });

  it('cae a las iniciales si la imagen no carga', async () => {
    const { fixture, el } = render({
      id: '1',
      name: 'Alianza Lima',
      logoUrl: 'https://x.invalid/1.png',
    });
    el.querySelector('img')!.dispatchEvent(new Event('error'));
    await fixture.whenStable();
    expect(el.querySelector('img')).toBeNull();
    expect(el.textContent!.trim()).toBe('AL');
  });
});
