// Package money holds amounts as integer cents; floats never touch money.
package money

import "fmt"

// Cents is an amount in EUR cents.
type Cents int64

// String formats cents as a plain decimal ("12.34", "-0.50").
func (c Cents) String() string {
	sign := ""
	v := int64(c)
	if v < 0 {
		sign = "-"
		v = -v
	}
	return fmt.Sprintf("%s%d.%02d", sign, v/100, v%100)
}

// FloorPercent returns amount * basisPoints / 10000 with fractions of a cent
// dropped, as tax law does for wage tax, Soli and church tax.
func FloorPercent(amount Cents, basisPoints int64) Cents {
	return Cents(int64(amount) * basisPoints / 10000)
}
