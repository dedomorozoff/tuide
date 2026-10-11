package expr

import (
	"math"
	"testing"
)

func TestEval(t *testing.T) {
	cases := []struct {
		input string
		want  float64
	}{
		{"1+2", 3},
		{"1+2*3", 7},
		{"(1+2)*3", 9},
		{"10/4", 2.5},
		{"-3+1", -2},
		{"2*-3", -6},
		{"-(2+3)", -5},
		{"1.5*2", 3},
		{"  7  ", 7},
		{"((2))", 2},
	}

	for _, c := range cases {
		got, err := Eval(c.input)
		if err != nil {
			t.Errorf("Eval(%q) failed: %v", c.input, err)
			continue
		}
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("Eval(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestEvalErrors(t *testing.T) {
	cases := []string{
		"",
		"1+",
		"*2",
		"(1+2",
		"1+2)",
		"1/0",
		"abc",
		"1..2",
	}

	for _, input := range cases {
		if value, err := Eval(input); err == nil {
			t.Errorf("Eval(%q) = %v, want an error", input, value)
		}
	}
}
