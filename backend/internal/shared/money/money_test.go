package money

import "testing"

func TestParse(t *testing.T) {
	cases := map[string]Money{"12.50": 1250, "12.5": 1250, "0.01": 1, "7": 700, "-3.20": -320}
	for in, want := range cases {
		got, err := Parse(in)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "1.234", "abc", "1.", ".5"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) debería fallar", bad)
		}
	}
}

func TestString(t *testing.T) {
	if s := Money(1250).String(); s != "12.50" {
		t.Errorf("got %s", s)
	}
	if s := Money(-5).String(); s != "-0.05" {
		t.Errorf("got %s", s)
	}
}

func TestCombineAndPayout(t *testing.T) {
	// 2.14 × VOID (1.00) × 1.97 = 4.21 truncado; stake 10.00 → 42.10
	total := Combine(214, OddsOne, 197)
	if total != 421 {
		t.Fatalf("Combine = %d; want 421", total)
	}
	if p := Money(1000).MulOdds(total); p != 4210 {
		t.Fatalf("payout = %d; want 4210", p)
	}
}

func TestParseOdds(t *testing.T) {
	o, err := ParseOdds("2.14")
	if err != nil || o != 214 || o.String() != "2.14" {
		t.Fatalf("ParseOdds = %d, %v", o, err)
	}
}
