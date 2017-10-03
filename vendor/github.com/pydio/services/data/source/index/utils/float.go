package utils

import (
	"math/big"
)

// Gob codec version. Permits backward-compatible changes to the encoding.
const floatGobVersion byte = 1

// Float type
type Float struct {
	*big.Float
}

func NewFloat() *Float {
	f := new(Float)
	f.Float = new(big.Float)

	f.SetPrec(512)

	return f
}

// Mant representation of a float
func (f *Float) Nat() Nat {
	nat := new(Nat)

	b, _ := f.GobEncode()

	*nat = nat.setBytes(b[10:])

	return Nat(*nat)
}
