package utils

import (
	"math/big"
	"strconv"
	"strings"
)

const PRECISION = 100

// Fraction type
type Fraction struct {
	n *big.Int
	d *big.Int
}

var (
	fraczero = big.NewInt(0)
	one      = big.NewInt(1)
	baseF0   = NewFraction(fraczero, one)
	baseF1   = NewFraction(one, one)
)

//const Precision = 16

// NewFraction from a numerator and denominator
func NewFraction(n *big.Int, d *big.Int) *Fraction {
	return &Fraction{n, d}
}

// NewFractionFromMaterializedPath function
func NewFractionFromMaterializedPath(path ...uint64) *Fraction {

	f := baseF0

	var current big.Int

	for i := len(path) - 1; i >= 0; i-- {
		current.SetUint64(path[i])

		f = add(baseF1, invert(f))

		f = add(NewFraction(&current, big.NewInt(1)), invert(f))
	}

	return f
}

// ToPath a Fraction
func ToPath(f *Fraction) string {

	/*
		var r big.Rat
		var c []int64
		for f.n.Cmp(zero) == 1 {

			r.SetFrac(f.n, f.d)

			f64, _ := r.Float64()
			i := int64(f64)

			c = append(c, i)

			f = invert(subtract(f, NewFraction(big.NewInt(i), one)))
			f = invert(subtract(f, baseF1))
		}
	*/
	c := ToPathUint(f)

	if len(c) == 0 {
		return ""
	}

	b := make([]string, len(c))
	for i, v := range c {
		b[i] = strconv.Itoa(int(v))
	}
	return strings.Join(b, ".")
}

func ToPathUint(f *Fraction) []uint64 {
	var r big.Rat
	var c []uint64
	for f.n.Cmp(fraczero) == 1 {

		r.SetFrac(f.n, f.d)

		f64, _ := r.Float64()
		i := int64(f64)
		u := uint64(f64)

		c = append(c, u)

		f = invert(subtract(f, NewFraction(big.NewInt(i), one)))
		f = invert(subtract(f, baseF1))
	}
	return c
}

// Decimal representation of the fraction
func (f Fraction) Decimal() *big.Rat {
	var d big.Rat

	d.SetFrac(f.n, f.d)

	return &d
}

// Num value of the fraction
func (f Fraction) Num() *big.Int {
	return f.n
}

// Den value of the fraction
func (f Fraction) Den() *big.Int {
	return f.d
}

func add(f1 *Fraction, f2 *Fraction) *Fraction {
	var add1, mul1, mul2, mul3 big.Int

	return NewFraction(add1.Add(mul1.Mul(f1.n, f2.d), mul2.Mul(f2.n, f1.d)), mul3.Mul(f2.d, f1.d))
}

func subtract(f1 *Fraction, f2 *Fraction) *Fraction {
	var sub1, mul1, mul2, mul3 big.Int
	return NewFraction(sub1.Sub(mul1.Mul(f1.n, f2.d), mul2.Mul(f2.n, f1.d)), mul3.Mul(f2.d, f1.d))
}

func invert(f *Fraction) *Fraction {
	return NewFraction(f.d, f.n)
}
