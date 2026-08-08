package money

import "testing"

func TestRound2(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		want  float64
	}{
		{"already exact", 159.92, 159.92},
		{"binary artifact", 107207.94000000002, 107207.94},
		{"accumulated sum", 0.1 + 0.2, 0.3},
		{"rounds half away from zero", 450.555, 450.56},
		{"negative", -450.555, -450.56},
		{"whole", 5000, 5000},
		{"zero", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Round2(tt.value); got != tt.want {
				t.Fatalf("Round2(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
