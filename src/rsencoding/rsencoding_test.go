// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

package rsencoding

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

var encodingPairs = []struct {
	uint64
	string
}{
	{9225891750247351890, "8KLL-PBYV-6DBC-AM942"},
	{2293534822941106833, "BANK-SNAQ-SZAD-374PZ"},
	{1124836811110093452, "26NE-5UYA-KLQU-2T39Z"},
	{10046156727204219923, "K52M-C67G-QK6D-AYSUQ"}}

func TestEncode(t *testing.T) {
	for _, pair := range encodingPairs {
		decoded, encoded := pair.uint64, pair.string
		assert.Equal(t, encoded, Encode(decoded), "Encoded correctly")
	}
}

func TestDecode(t *testing.T) {
	for _, pair := range encodingPairs {
		decoded, encoded := pair.uint64, pair.string
		assert.Equal(t, decoded, Decode(encoded), "Decoded correctly")
	}
}
