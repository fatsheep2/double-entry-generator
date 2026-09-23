/*
Copyright © 2019 Ce Gao

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ir

import (
	"fmt"
	"math/big"
	"strings"
)

// MaxDecimalScale matches Mirato exact_amount (28 fractional digits).
const MaxDecimalScale = 28

// Decimal is fixed-point money aligned with Mirato engine/src/exact_amount.rs:
// signed integer units with an explicit scale. Runtime paths must treat this as
// authoritative and must not round-trip through float64.
type Decimal struct {
	units big.Int
	scale uint32
}

// ZeroDecimal is the additive identity (0 with scale 0).
func ZeroDecimal() Decimal {
	return Decimal{}
}

// ParseDecimal parses a cleaned decimal literal (optional leading sign, optional
// fraction, optional scientific exponent). Grouping/currency stripping belongs
// to the caller (see importer.CleanAmount).
func ParseDecimal(text string) (Decimal, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Decimal{}, fmt.Errorf("empty amount")
	}
	lower := strings.ToLower(text)
	if lower == "nan" || lower == "inf" || lower == "+inf" || lower == "-inf" ||
		lower == "infinity" || lower == "+infinity" || lower == "-infinity" {
		return Decimal{}, fmt.Errorf("invalid amount %q", text)
	}

	mantissa := text
	exponent := int32(0)
	if i := strings.IndexAny(text, "eE"); i >= 0 {
		mantissa = text[:i]
		expPart := text[i+1:]
		if expPart == "" {
			return Decimal{}, fmt.Errorf("invalid amount exponent in %q", text)
		}
		var exp big.Int
		if _, ok := exp.SetString(expPart, 10); !ok {
			return Decimal{}, fmt.Errorf("invalid amount exponent in %q", text)
		}
		if !exp.IsInt64() {
			return Decimal{}, fmt.Errorf("amount exponent out of range")
		}
		e := exp.Int64()
		if e > 28 || e < -28 {
			return Decimal{}, fmt.Errorf("amount exponent out of range")
		}
		exponent = int32(e)
	}

	negative := false
	switch {
	case strings.HasPrefix(mantissa, "-"):
		negative = true
		mantissa = mantissa[1:]
	case strings.HasPrefix(mantissa, "+"):
		mantissa = mantissa[1:]
	}
	if mantissa == "" {
		return Decimal{}, fmt.Errorf("invalid amount %q", text)
	}

	parts := strings.Split(mantissa, ".")
	if len(parts) > 2 {
		return Decimal{}, fmt.Errorf("invalid amount %q", text)
	}
	intPart := parts[0]
	fracPart := ""
	if len(parts) == 2 {
		fracPart = parts[1]
	}
	if intPart == "" && fracPart == "" {
		return Decimal{}, fmt.Errorf("invalid amount %q", text)
	}
	if intPart == "" {
		intPart = "0"
	}
	for _, p := range []string{intPart, fracPart} {
		for _, c := range p {
			if c < '0' || c > '9' {
				return Decimal{}, fmt.Errorf("invalid amount %q", text)
			}
		}
	}
	if !strings.ContainsAny(intPart+fracPart, "0123456789") {
		return Decimal{}, fmt.Errorf("invalid amount %q", text)
	}

	digits := intPart + fracPart
	var units big.Int
	if _, ok := units.SetString(digits, 10); !ok {
		return Decimal{}, fmt.Errorf("amount out of precise range")
	}
	if negative {
		units.Neg(&units)
	}

	scale := int32(len(fracPart)) - exponent
	if scale < 0 {
		pow := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-scale)), nil)
		units.Mul(&units, pow)
		scale = 0
	}
	if scale > MaxDecimalScale {
		return Decimal{}, fmt.Errorf("amount exceeds %d fractional digits", MaxDecimalScale)
	}

	d := Decimal{scale: uint32(scale)}
	d.units.Set(&units)
	return d.Normalize(), nil
}

// Normalize strips trailing fractional zeros (Mirato parity).
// clone detaches the big.Int backing words before any mutating operation.
func (d Decimal) clone() Decimal {
	out := Decimal{scale: d.scale}
	out.units.Set(&d.units)
	return out
}

func (d Decimal) Normalize() Decimal {
	d = d.clone()
	for d.scale > 0 {
		var rem big.Int
		rem.Mod(&d.units, big.NewInt(10))
		if rem.Sign() != 0 {
			break
		}
		d.units.Div(&d.units, big.NewInt(10))
		d.scale--
	}
	// Canonicalize negative zero to +0.
	if d.units.Sign() == 0 {
		d.units.SetInt64(0)
	}
	return d
}

// IsZero reports whether the value is zero (including negative zero).
func (d Decimal) IsZero() bool {
	return d.units.Sign() == 0
}

// Sign returns -1, 0, or 1.
func (d Decimal) Sign() int {
	return d.units.Sign()
}

// Neg returns -d.
func (d Decimal) Neg() Decimal {
	out := d.clone()
	out.units.Neg(&out.units)
	return out.Normalize()
}

// Abs returns |d|.
func (d Decimal) Abs() Decimal {
	out := d.clone()
	out.units.Abs(&out.units)
	return out.Normalize()
}

// Cmp compares d and other.
func (d Decimal) Cmp(other Decimal) int {
	a, b, err := alignScales(d, other)
	if err != nil {
		// Align failure is extremely rare (scale overflow); fall back to text.
		return strings.Compare(d.Text(0), other.Text(0))
	}
	return a.units.Cmp(&b.units)
}

// Add returns d+other.
func (d Decimal) Add(other Decimal) (Decimal, error) {
	a, b, err := alignScales(d, other)
	if err != nil {
		return Decimal{}, err
	}
	a.units.Add(&a.units, &b.units)
	return a.Normalize(), nil
}

// Sub returns d-other.
func (d Decimal) Sub(other Decimal) (Decimal, error) {
	return d.Add(other.Neg())
}

// Mul returns d*other. Scale must stay within MaxDecimalScale (Mirato parity).
func (d Decimal) Mul(other Decimal) (Decimal, error) {
	var units big.Int
	units.Mul(&d.units, &other.units)
	scale := d.scale + other.scale
	if scale > MaxDecimalScale {
		return Decimal{}, fmt.Errorf("product exceeds %d fractional digits", MaxDecimalScale)
	}
	out := Decimal{scale: scale}
	out.units.Set(&units)
	return out.Normalize(), nil
}

// DivExact returns d/other when the quotient terminates within MaxDecimalScale
// fractional digits. Non-terminating or oversized results are rejected — never
// silently rounded (no silent money loss). Division by zero is rejected.
func (d Decimal) DivExact(other Decimal) (Decimal, error) {
	if other.IsZero() {
		return Decimal{}, fmt.Errorf("division by zero")
	}
	// Convert both to integer dividend/divisor at a common scale, then extend
	// dividend by up to MaxDecimalScale tens until it divides evenly.
	a, b, err := alignScales(d, other)
	if err != nil {
		return Decimal{}, err
	}
	dividend := new(big.Int).Set(&a.units)
	divisor := new(big.Int).Set(&b.units)

	for scale := uint32(0); scale <= MaxDecimalScale; scale++ {
		var rem big.Int
		var quo big.Int
		quo.QuoRem(dividend, divisor, &rem)
		if rem.Sign() == 0 {
			out := Decimal{scale: scale}
			out.units.Set(&quo)
			return out.Normalize(), nil
		}
		if scale == MaxDecimalScale {
			break
		}
		dividend.Mul(dividend, big.NewInt(10))
	}
	return Decimal{}, fmt.Errorf("inexact division rejected (would require silent rounding)")
}

// Text formats the decimal. minScale pads trailing zeros (Mirato Decimal::text).
func (d Decimal) Text(minScale uint32) string {
	neg := d.units.Sign() < 0
	abs := new(big.Int).Abs(&d.units)
	digits := abs.String()
	if d.scale > 0 {
		for len(digits) <= int(d.scale) {
			digits = "0" + digits
		}
		split := len(digits) - int(d.scale)
		digits = digits[:split] + "." + digits[split:]
	}
	if minScale > d.scale {
		if d.scale == 0 {
			digits += "."
		}
		digits += strings.Repeat("0", int(minScale-d.scale))
	}
	if neg {
		digits = "-" + digits
	}
	return digits
}

// Float64Approx returns a float64 view for legacy Order.Money only. It is never
// authoritative and must not be parsed back into ExactMoney.
func (d Decimal) Float64Approx() float64 {
	rat := new(big.Rat).SetFrac(&d.units, pow10(d.scale))
	f, _ := rat.Float64()
	return f
}

func alignScales(a, b Decimal) (Decimal, Decimal, error) {
	scale := a.scale
	if b.scale > scale {
		scale = b.scale
	}
	left, err := a.withScale(scale)
	if err != nil {
		return Decimal{}, Decimal{}, err
	}
	right, err := b.withScale(scale)
	if err != nil {
		return Decimal{}, Decimal{}, err
	}
	return left, right, nil
}

func (d Decimal) withScale(scale uint32) (Decimal, error) {
	if scale < d.scale {
		return Decimal{}, fmt.Errorf("cannot reduce scale without rounding")
	}
	if scale == d.scale {
		return d.clone(), nil
	}
	delta := scale - d.scale
	out := Decimal{scale: scale}
	out.units.Mul(&d.units, pow10(delta))
	return out, nil
}

func pow10(n uint32) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
}
