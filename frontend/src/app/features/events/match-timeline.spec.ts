import { TestBed } from '@angular/core/testing';

import { MatchEvent } from '../../core/api/models';
import { MatchTimeline, describeEvent, minuteText } from './match-timeline';

const ev = (e: Partial<MatchEvent>): MatchEvent => ({
  minute: 10,
  side: 'HOME',
  type: 'GOAL',
  detail: 'Normal Goal',
  player: 'Salah',
  ...e,
});

describe('describeEvent', () => {
  it('describe goles, penales, autogoles y penales fallados', () => {
    expect(describeEvent(ev({ assist: 'Szoboszlai' }))).toEqual({
      icon: '⚽',
      title: 'Salah',
      detail: 'Asistencia: Szoboszlai',
    });
    expect(describeEvent(ev({ detail: 'Penalty' })).detail).toBe('De penal');
    expect(describeEvent(ev({ detail: 'Own Goal' })).detail).toBe('Autogol del rival');
    expect(describeEvent(ev({ detail: 'Missed Penalty' })).icon).toBe('❌');
  });

  it('distingue amarilla y roja, cambios y VAR', () => {
    expect(describeEvent(ev({ type: 'CARD', detail: 'Yellow Card' })).icon).toBe('🟨');
    expect(describeEvent(ev({ type: 'CARD', detail: 'Red Card' })).icon).toBe('🟥');
    expect(
      describeEvent(ev({ type: 'SUBST', detail: 'Substitution 1', assist: 'Núñez' })).title,
    ).toBe('Salah ⇄ Núñez');
    expect(describeEvent(ev({ type: 'VAR', detail: 'Goal cancelled' })).title).toBe('Gol anulado');
  });

  it('muestra el descuento en el minuto', () => {
    expect(minuteText({ minute: 45, extra: 2 })).toBe("45+2'");
    expect(minuteText({ minute: 18, extra: null })).toBe("18'");
  });
});

describe('MatchTimeline', () => {
  it('pone los sucesos de la visita del otro lado y anuncia el equipo', () => {
    const fixture = TestBed.createComponent(MatchTimeline);
    fixture.componentRef.setInput('events', [
      ev({}),
      ev({ side: 'AWAY', type: 'CARD', detail: 'Yellow Card', player: 'Rodri' }),
    ]);
    fixture.componentRef.setInput('home', 'Liverpool');
    fixture.componentRef.setInput('away', 'Manchester City');
    fixture.detectChanges();
    const items = [...(fixture.nativeElement as HTMLElement).querySelectorAll('li')];
    expect(items.length).toBe(2);
    expect(items[0].classList.contains('away')).toBe(false);
    expect(items[1].classList.contains('away')).toBe(true);
    expect(items[1].textContent).toContain('(Manchester City)');
  });

  it('muestra un aviso si todavía no hay sucesos', () => {
    const fixture = TestBed.createComponent(MatchTimeline);
    fixture.componentRef.setInput('events', []);
    fixture.componentRef.setInput('home', 'A');
    fixture.componentRef.setInput('away', 'B');
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).textContent).toContain('Todavía no hay');
  });
});
