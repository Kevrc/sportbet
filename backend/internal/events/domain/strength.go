package domain

import "math"

// Rango de fuerza que admite teams.strength y el modelo de cuotas (ADR 0001).
const (
	MinStrength = 0.60
	MaxStrength = 1.60
	// strengthSmoothing suaviza el inicio de temporada: con pocos partidos
	// la fuerza se queda cerca de 1.00.
	strengthSmoothing = 3.0
)

// StrengthFromGoals calcula la fuerza de un equipo con sus goles a favor y
// en contra en la tabla: sqrt((GF+3)/(GC+3)), acotada a [0.60, 1.60] y
// redondeada a 2 decimales. Un equipo promedio queda en 1.00.
func StrengthFromGoals(goalsFor, goalsAgainst int) float64 {
	if goalsFor < 0 || goalsAgainst < 0 {
		return 1.0
	}
	s := math.Sqrt((float64(goalsFor) + strengthSmoothing) / (float64(goalsAgainst) + strengthSmoothing))
	s = math.Max(MinStrength, math.Min(MaxStrength, s))
	return math.Round(s*100) / 100
}
