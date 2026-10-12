package domain

import "testing"

func TestStrengthFromGoals(t *testing.T) {
	cases := []struct {
		gf, ga int
		want   float64
	}{
		{0, 0, 1.00},   // inicio de temporada
		{12, 12, 1.00}, // equipo promedio
		{20, 7, 1.52},  // equipo fuerte
		{6, 18, 0.65},  // equipo débil
		{40, 3, 1.60},  // se acota arriba
		{1, 30, 0.60},  // se acota abajo
		{-1, 5, 1.00},  // dato inválido
	}
	for _, c := range cases {
		if got := StrengthFromGoals(c.gf, c.ga); got != c.want {
			t.Errorf("StrengthFromGoals(%d, %d) = %.2f; want %.2f", c.gf, c.ga, got, c.want)
		}
	}
}
