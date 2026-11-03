package ir

import "testing"

func TestParseDecimalMiratoParity(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"0.123456789012345678", "0.123456789012345678"},
		{"9007199254740993", "9007199254740993"},
		{"0.000000000000000001", "0.000000000000000001"},
		{"1.65E-4", "0.000165"},
		{"-0", "0"},
		{"+0.0", "0"},
	}
	for _, tc := range cases {
		d, err := ParseDecimal(tc.in)
		if err != nil {
			t.Fatalf("ParseDecimal(%q): %v", tc.in, err)
		}
		if got := d.Text(0); got != tc.want {
			t.Fatalf("ParseDecimal(%q).Text(0)=%q want %q", tc.in, got, tc.want)
		}
	}
	sum, err := MustParse("0.1").Add(MustParse("-0.100"))
	if err != nil || !sum.IsZero() {
		t.Fatalf("0.1 + -0.100 => %v %v", sum.Text(0), err)
	}
	for _, invalid := range []string{"NaN", "inf", "--1", "1e1000", "1.2.3", ""} {
		if _, err := ParseDecimal(invalid); err == nil {
			t.Fatalf("expected error for %q", invalid)
		}
	}
}

func MustParse(s string) Decimal {
	d, err := ParseDecimal(s)
	if err != nil {
		panic(err)
	}
	return d
}

func TestDivExactRejectsInexactAndZero(t *testing.T) {
	if _, err := MustParse("1").DivExact(MustParse("0")); err == nil {
		t.Fatal("div by zero")
	}
	if _, err := MustParse("1").DivExact(MustParse("3")); err == nil {
		t.Fatal("inexact 1/3 must be rejected")
	}
	got, err := MustParse("1").DivExact(MustParse("4"))
	if err != nil || got.Text(0) != "0.25" {
		t.Fatalf("1/4 = %q %v", got.Text(0), err)
	}
}

func TestAboveFloat53IntegerPreserved(t *testing.T) {
	const n = "9007199254740993"
	d := MustParse(n)
	if d.Text(0) != n {
		t.Fatalf("got %q", d.Text(0))
	}
	if d.Float64Approx() == float64(9007199254740993) {
		// float64 cannot represent this exactly; approx must not be treated as authority.
		t.Log("float64 happens to round-trip on this platform; ExactMoney text is still authoritative")
	}
}

// Regression: copying a big.Int struct does not copy its backing words.
func TestReviewDecimalValueSemantics(t *testing.T) {
	a := MustParse("123456789012345678.123456789")
	b := MustParse("1.123456789")
	beforeA, beforeB := a.Text(0), b.Text(0)
	sum, err := a.Add(b)
	if err != nil || sum.Text(0) != "123456789012345679.246913578" {
		t.Fatalf("sum=%s err=%v", sum.Text(0), err)
	}
	if a.Text(0) != beforeA || b.Text(0) != beforeB {
		t.Fatalf("Add mutated inputs: %s / %s", a.Text(0), b.Text(0))
	}
	_ = a.Normalize()
	_ = a.Abs()
	_ = a.Neg()
	if a.Text(0) != beforeA {
		t.Fatalf("unary mutated input: %s", a.Text(0))
	}
}

// Mirato exact_amount.rs uses i128 units (not 96-bit Decimal128).
// Shared compatible range = values whose coefficient fits in i128 with scale<=28.
// DEG currently uses big.Int (arbitrary precision); do not reject Mirato-legal i128 values.
// Values beyond i128 remain a documented cross-engine gap (needs parent/Mirato decision).
func TestReviewDecimalI128CompatibleRange(t *testing.T) {
	// Near i128 boundary but well within scale rules — must parse (Mirato-legal magnitude class).
	const near = "17014118346046923173168730371588410572" // 38 digits < i128 max ~39 digits
	d, err := ParseDecimal(near)
	if err != nil {
		t.Fatalf("rejected Mirato-compatible magnitude: %v", err)
	}
	if d.Text(0) != near {
		t.Fatalf("got %q", d.Text(0))
	}
	// Classic 96-bit Decimal128 max+1 must NOT be rejected solely on 96-bit grounds
	// (would incorrectly shrink below Mirato i128 capacity).
	const above96 = "79228162514264337593543950336"
	if _, err := ParseDecimal(above96); err != nil {
		t.Fatalf("must not impose unverified 96-bit ceiling (Mirato is i128): %v", err)
	}
}
