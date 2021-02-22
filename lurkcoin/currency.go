//
// lurkcoin
// Copyright © 2020 by luk3yx
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
//

package lurkcoin

import (
	"errors"
	"math/big"
	"strings"
)

// Create a custom Currency type that stores read-only currency values.
type Currency struct {
	raw *big.Int
}

var i0 = big.NewInt(0)
var i10 = big.NewInt(10)
var i100 = big.NewInt(100)

// A method to convert currency to a string.
func (self Currency) RawString() string {
	// This should probably be improved.
	whole := new(big.Int)
	frac := new(big.Int)

	var res string
	if self.raw.Cmp(i0) >= 0 {
		whole.DivMod(self.raw, i100, frac)
		res = whole.String()
	} else {
		whole.DivMod(new(big.Int).Abs(self.raw), i100, frac)
		res = "-" + whole.String()
	}

	res += "."
	if frac.Cmp(i10) < 0 {
		res += "0"
	} else if frac.Cmp(i100) >= 0 {
		panic("Unreachable code (big.Int DivMod did something it shouldn't).")
	}
	return res + frac.String()
}

// Returns the currency as a human-readable string.
func (self Currency) String() string {
	raw := self.RawString()
	var builder strings.Builder

	// Add the leading ¤.
	s := 0
	if raw[0] == '-' {
		s = 1
		builder.WriteByte('-')
	}
	builder.WriteString(SYMBOL)

	// Insert a comma when required
	// 123456.78 → 123,456.78
	l := len(raw) - 3
	for i := s; i < len(raw); i++ {
		if l > i && i > s && (l-i)%3 == 0 {
			builder.WriteByte(',')
		}
		builder.WriteByte(raw[i])
	}

	// Return the result
	return builder.String()
}

// Returns the currency as a human-readable string. Positive numbers are
// prefixed with +.
func (self Currency) DeltaString() string {
	s := self.String()
	if self.GtZero() {
		return "+" + s
	}
	return s
}

// Addition/division
func (self Currency) Add(num Currency) Currency {
	raw := new(big.Int)
	raw.Add(self.raw, num.raw)
	return Currency{raw}
}

func (self Currency) Sub(num Currency) Currency {
	raw := new(big.Int)
	raw.Sub(self.raw, num.raw)
	return Currency{raw}
}

func (self Currency) Div(num Currency) *big.Float {
	return new(big.Float).Quo(self.Float(), num.Float())
}

func (self Currency) Neg() Currency {
	return Currency{new(big.Int).Sub(i0, self.raw)}
}

// Comparisons
func (self Currency) Cmp(num Currency) int {
	return self.raw.Cmp(num.raw)
}

func (self Currency) Eq(num Currency) bool {
	return self.Cmp(num) == 0
}

func (self Currency) Gt(num Currency) bool {
	return self.Cmp(num) == 1
}

func (self Currency) Lt(num Currency) bool {
	return self.Cmp(num) == -1
}

func (self Currency) LtZero() bool {
	return self.raw.Cmp(i0) == -1
}

func (self Currency) IsZero() bool {
	return self.raw.Cmp(i0) == 0
}

func (self Currency) GtZero() bool {
	return self.raw.Cmp(i0) == 1
}

func (self Currency) IsNil() bool {
	return self.raw == nil
}

// Conversions
var f100 *big.Float = big.NewFloat(100)

func (self Currency) Float() *big.Float {
	raw := new(big.Float).SetInt(self.raw)
	return new(big.Float).Quo(raw, f100)
}

func (self Currency) Int() *big.Int {
	return new(big.Int).Set(self.raw)
}

// JSON
func (self Currency) MarshalJSON() ([]byte, error) {
	res := []byte(self.RawString())
	// Remove a single trailing zero (if any). If all trailing zeroes were
	// removed, Python would interpret the value as an integer instead.
	if res[len(res)-1] == '0' {
		res = res[:len(res)-1]
	}
	return res, nil
}

func (self *Currency) setString(data string) bool {
	if self.raw != nil {
		return false
	}

	if strings.HasPrefix(data, SYMBOL) {
		data = data[2:]
	}

	f, success := new(big.Float).SetString(data)
	if success {
		res := new(big.Int)
		new(big.Float).Mul(f, f100).Int(res)
		self.raw = res
	}
	return success
}

func (self *Currency) UnmarshalJSON(data []byte) error {
	// Accept quoted values
	var s string
	if data[0] == '"' && data[len(data)-1] == '"' {
		s = string(data[1 : len(data)-1])
	} else {
		s = string(data)
	}
	if self.setString(s) {
		return nil
	} else {
		return errors.New("Invalid currency value.")
	}
}

func (self *Currency) GobEncode() ([]byte, error) {
	return self.raw.GobEncode()
}

func (self *Currency) GobDecode(data []byte) error {
	if self.raw != nil {
		return errors.New("GobDecode() on already initialised Currency.")
	}
	self.raw = new(big.Int)
	return self.raw.GobDecode(data)
}

// Create new currency values
func CurrencyFromFloat(num *big.Float) Currency {
	f := new(big.Float)
	f.Mul(num, f100)
	raw := new(big.Int)
	f.Int(raw)
	return Currency{raw}
}

func CurrencyFromInt(num *big.Int) Currency {
	return Currency{new(big.Int).Set(num)}
}

func CurrencyFromInt64(num int64) Currency {
	return Currency{new(big.Int).SetInt64(num * 100)}
}

func CurrencyFromFloat64(num float64) Currency {
	return CurrencyFromFloat(new(big.Float).SetFloat64(num))
}

func CurrencyFromString(num string) Currency {
	var res Currency
	if res.setString(strings.ReplaceAll(num, "_", "")) {
		return res
	} else {
		return Currency{i0}
	}
}

func ParseCurrency(num string) (Currency, error) {
	var res Currency
	if res.setString(strings.ReplaceAll(num, "_", "")) {
		return res, nil
	} else {
		return Currency{i0}, errors.New("ERR_INVALIDAMOUNT")
	}
}
