package geom

import "testing"

func TestRectContains(t *testing.T) {
	rect := Rect{X: 2, Y: 3, Width: 4, Height: 2}

	cases := []struct {
		x, y int
		want bool
	}{
		{1, 3, false},
		{2, 3, true},
		{5, 4, true},
		{6, 4, false},
		{5, 5, false},
	}

	for _, c := range cases {
		if got := rect.Contains(c.x, c.y); got != c.want {
			t.Errorf("Contains(%d, %d) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
}

func TestRectIntersect(t *testing.T) {
	a := Rect{X: 0, Y: 0, Width: 10, Height: 10}
	b := Rect{X: 5, Y: 5, Width: 10, Height: 10}

	got := a.Intersect(b)
	want := Rect{X: 5, Y: 5, Width: 5, Height: 5}

	if got != want {
		t.Errorf("Intersect = %+v, want %+v", got, want)
	}

	if far := (Rect{X: 100, Y: 100, Width: 1, Height: 1}); a.Intersect(far).Area() != 0 {
		t.Error("disjoint rectangles should intersect into an empty rect")
	}
}

func TestRectClampTo(t *testing.T) {
	bounds := Rect{X: 0, Y: 0, Width: 80, Height: 24}

	cases := []struct {
		name string
		in   Rect
		want Rect
	}{
		{
			name: "inside",
			in:   Rect{X: 10, Y: 5, Width: 20, Height: 10},
			want: Rect{X: 10, Y: 5, Width: 20, Height: 10},
		},
		{
			name: "overflows right",
			in:   Rect{X: 70, Y: 5, Width: 30, Height: 10},
			want: Rect{X: 50, Y: 5, Width: 30, Height: 10},
		},
		{
			name: "negative origin",
			in:   Rect{X: -5, Y: -5, Width: 20, Height: 10},
			want: Rect{X: 0, Y: 0, Width: 20, Height: 10},
		},
		{
			name: "oversized",
			in:   Rect{X: 0, Y: 0, Width: 200, Height: 100},
			want: Rect{X: 0, Y: 0, Width: 80, Height: 24},
		},
		{
			name: "grows to minimum",
			in:   Rect{X: 0, Y: 0, Width: 1, Height: 1},
			want: Rect{X: 0, Y: 0, Width: 10, Height: 5},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.in.ClampTo(bounds, 10, 5)
			if got != c.want {
				t.Errorf("ClampTo = %+v, want %+v", got, c.want)
			}
			if !bounds.ContainsRect(got) {
				t.Errorf("ClampTo result %+v escapes bounds %+v", got, bounds)
			}
		})
	}
}

func TestRectInset(t *testing.T) {
	rect := Rect{X: 4, Y: 4, Width: 20, Height: 10}

	got := rect.Inset(1, 1)
	want := Rect{X: 5, Y: 5, Width: 18, Height: 8}

	if got != want {
		t.Errorf("Inset = %+v, want %+v", got, want)
	}

	if rect != (Rect{X: 4, Y: 4, Width: 20, Height: 10}) {
		t.Error("Inset must not mutate the receiver")
	}
}
