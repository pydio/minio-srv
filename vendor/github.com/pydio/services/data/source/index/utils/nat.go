// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements unsigned multi-precision integers (natural
// numbers). They are the building blocks for the implementation
// of signed integers, rationals, and floating-point numbers.

package utils

import (
	"math/big"
)

const (
	// Compute the size _S of a Word in bytes.
	_m    = ^big.Word(0)
	_logS = _m>>8&1 + _m>>16&1 + _m>>32&1
	_S    = 1<<_logS - 1

	_W = _S << 3 // word size in bits
	_B = 1 << _W // digit base
	_M = _B - 1  // digit mask

	_W2 = _W / 2   // half word size in bits
	_B2 = 1 << _W2 // half digit base
	_M2 = _B2 - 1  // half digit mask
)

// An unsigned integer x of the form
//
//   x = x[n-1]*_B^(n-1) + x[n-2]*_B^(n-2) + ... + x[1]*_B + x[0]
//
// with 0 <= x[i] < _B and 0 <= i < n is stored in a slice of length n,
// with the digits x[i] as the slice elements.
//
// A number is normalized if the slice contains no leading 0 digits.
// During arithmetic operations, denormalized values may occur but are
// always normalized before returning the final result. The normalized
// representation of 0 is the empty or nil slice (length = 0).
//

type Nat []big.Word

var (
	natOne = Nat{1}
	natTwo = Nat{2}
	natTen = Nat{10}
)

func (z Nat) clear() {
	for i := range z {
		z[i] = 0
	}
}

func (z Nat) norm() Nat {
	i := len(z)
	for i > 0 && z[i-1] == 0 {
		i--
	}
	return z[0:i]
}

func (z Nat) make(n int) Nat {
	if n <= cap(z) {
		return z[:n] // reuse z
	}
	// Choosing a good value for e has significant performance impact
	// because it increases the chance that a value can be reused.
	const e = 4 // extra capacity
	return make(Nat, n, n+e)
}

// setBytes interprets buf as the bytes of a big-endian unsigned
// integer, sets z to that value, and returns z.
func (z Nat) setBytes(buf []byte) Nat {
	z = z.make((len(buf) + _S - 1) / _S)

	k := 0
	s := uint(0)
	var d big.Word
	for i := len(buf); i > 0; i-- {
		d |= big.Word(buf[i-1]) << s
		if s += 8; s == _S*8 {
			z[k] = d
			k++
			s = 0
			d = 0
		}
	}
	if k < len(z) {
		z[k] = d
	}

	return z.norm()
}
