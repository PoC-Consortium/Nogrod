package rsencoding

import "testing"

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
		decoded, expected := pair.uint64, pair.string
		encoded := Encode(decoded)

		if encoded != expected {
			t.Errorf("decoding: expected %v, got %v", expected, encoded)
		}
	}
}

func TestDecode(t *testing.T) {
	for _, pair := range encodingPairs {
		expected, encoded := pair.uint64, pair.string
		decoded, err := Decode(encoded)

		if err != nil {
			t.Errorf("decoding: %v", err)
			continue
		}

		if decoded != expected {
			t.Errorf("decoding: expected %v, got %v", expected, decoded)
		}
	}
}

func BenchmarkEncode(b *testing.B) {
	for n := 0; n < b.N; n++ {
		Encode(uint64(n))
	}
}

func BenchmarkDecode(b *testing.B) {
	for n := 0; n < b.N; n++ {
		Decode(encodingPairs[n%len(encodingPairs)].string)
	}
}
