// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

package util

import (
	"github.com/klauspost/cpuid"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDecodeGeneratorSignature(t *testing.T) {
	genSigStr := "017776b64979b6a3fc235f2bd680f576c76f09c7781274f9f0ee6b913581ad5e"
	genSig, err := DecodeGeneratorSignature(genSigStr)
	assert.Nil(t, err)
	assert.Equal(t, []uint8{
		0x1, 0x77, 0x76, 0xb6, 0x49, 0x79, 0xb6, 0xa3,
		0xfc, 0x23, 0x5f, 0x2b, 0xd6, 0x80, 0xf5, 0x76,
		0xc7, 0x6f, 0x9, 0xc7, 0x78, 0x12, 0x74, 0xf9,
		0xf0, 0xee, 0x6b, 0x91, 0x35, 0x81, 0xad, 0x5e}, genSig, "Unpacking genSig failed")

	_, err = DecodeGeneratorSignature("X17776b64979b6a3fc235f2bd680f576c76f09c7781274f9f0ee6b913581ad5e")
	assert.NotNil(t, err)
}

func TestCalculateScoop(t *testing.T) {
	genSig, _ := DecodeGeneratorSignature("2a0757c8af2aa43b29515c872385ede31d0742b1ea29b93a1a8c38a11b8a37a0")
	assert.Equal(t, uint32(0x1e), CalculateScoop(41189, genSig), "Calculated scoop is incorrect (1)")

	genSig, _ = DecodeGeneratorSignature("56747285d0a52dbf7f45bcf7b45b86bd48a11315500d5d5424ee3a1e7c63f712")
	assert.Equal(t, uint32(0x07), CalculateScoop(41190, genSig), "Calculated scoop is incorrect (2)")
}

func TestCalculateDeadlinesSSE4(t *testing.T) {
	if !cpuid.CPU.SSE4() {
		t.Log("SSE4 not supported, skipping related tests")
		return
	}

	genSig, _ := DecodeGeneratorSignature("2a0757c8af2aa43b29515c872385ede31d0742b1ea29b93a1a8c38a11b8a37a0")

	d1, d2, d3, d4 := CalculateDeadlinesSSE4(30, 18325193796, genSig, false,
		10282355196851764065, 10282355196851764065, 10282355196851764065, 10282355196851764065,
		6729, 6729, 6729, 6729)

	assert.Equal(t, uint64(0x1e847df3), d1, "Calculated deadline is incorrect (1)")
	assert.Equal(t, uint64(0x1e847df3), d2, "Calculated deadline is incorrect (2)")
	assert.Equal(t, uint64(0x1e847df3), d3, "Calculated deadline is incorrect (3)")
	assert.Equal(t, uint64(0x1e847df3), d4, "Calculated deadline is incorrect (4)")

	genSig, _ = DecodeGeneratorSignature("5ba5d36be04b16c36edc7e0c3d342e0853ca7ae41b3364dd8b63154978a315d5")

	d1, _, _, _ = CalculateDeadlinesSSE4(2949, 15604408436, genSig, false,
		6418289488649374107, 0, 0, 0,
		2498, 0, 0, 0)

	assert.Equal(t, uint64(231141), d1, "Calculated deadline is incorrect (5)")

	genSig, _ = DecodeGeneratorSignature("56747285d0a52dbf7f45bcf7b45b86bd48a11315500d5d5424ee3a1e7c63f712")

	d1, d2, d3, d4 = CalculateDeadlinesSSE4(7, 17959135569, genSig, false,
		6418289488649374107, 6418289488649374107, 6418289488649374107, 6418289488649374107,
		133789, 133789, 133789, 133789)

	assert.Equal(t, uint64(0x30d71434), d1, "Calculated deadline is incorrect (5)")
	assert.Equal(t, uint64(0x30d71434), d2, "Calculated deadline is incorrect (6)")
	assert.Equal(t, uint64(0x30d71434), d3, "Calculated deadline is incorrect (7)")
	assert.Equal(t, uint64(0x30d71434), d4, "Calculated deadline is incorrect (8)")
}

func TestCalculateDeadlinesSSE4PoC2(t *testing.T) {
	if !cpuid.CPU.SSE4() {
		t.Log("SSE4 not supported, skipping relatedd tests")
		return
	}

	genSig, _ := DecodeGeneratorSignature("305a98571a8b96f699449dd71eff051fc10a3475bce18c7dac81b3d9316a9780")

	d1, d2, d3, d4 := CalculateDeadlinesSSE4(1269, 18325193796, genSig, true,
		10558800117808928257, 10558800117808928257, 10558800117808928257, 10558800117808928257,
		123, 123, 123, 123)

	assert.Equal(t, uint64(720021932), d1, "Calculated deadline is incorrect (1) PoC2")
	assert.Equal(t, uint64(720021932), d2, "Calculated deadline is incorrect (2) PoC2")
	assert.Equal(t, uint64(720021932), d3, "Calculated deadline is incorrect (3) PoC2")
	assert.Equal(t, uint64(720021932), d4, "Calculated deadline is incorrect (4) PoC2")
}

func TestCalculateDeadlinesAVX2PoC2(t *testing.T) {
	if !cpuid.CPU.AVX2() {
		t.Log("AVX2 not supported, skipping related tests")
		return
	}

	genSig, _ := DecodeGeneratorSignature("305a98571a8b96f699449dd71eff051fc10a3475bce18c7dac81b3d9316a9780")

	d1, d2, d3, d4, d5, d6, d7, d8 := CalculateDeadlinesAVX2(1269, 18325193796, genSig, true,
		10558800117808928257, 10558800117808928257, 10558800117808928257, 10558800117808928257,
		10558800117808928257, 10558800117808928257, 10558800117808928257, 10558800117808928257,
		123, 123, 123, 123,
		123, 123, 123, 123)

	assert.Equal(t, uint64(720021932), d1, "Calculated deadline is incorrect (1) PoC2")
	assert.Equal(t, uint64(720021932), d2, "Calculated deadline is incorrect (2) PoC2")
	assert.Equal(t, uint64(720021932), d3, "Calculated deadline is incorrect (3) PoC2")
	assert.Equal(t, uint64(720021932), d4, "Calculated deadline is incorrect (4) PoC2")
	assert.Equal(t, uint64(720021932), d5, "Calculated deadline is incorrect (5) PoC2")
	assert.Equal(t, uint64(720021932), d6, "Calculated deadline is incorrect (6) PoC2")
	assert.Equal(t, uint64(720021932), d7, "Calculated deadline is incorrect (7) PoC2")
	assert.Equal(t, uint64(720021932), d8, "Calculated deadline is incorrect (8) PoC2")
}

func TestCalculateDeadlinesAVX2(t *testing.T) {
	if !cpuid.CPU.AVX2() {
		t.Log("AVX2 not supported, skipping related tests")
		return
	}

	genSig, _ := DecodeGeneratorSignature("2a0757c8af2aa43b29515c872385ede31d0742b1ea29b93a1a8c38a11b8a37a0")

	d1, d2, d3, d4, d5, d6, d7, d8 := CalculateDeadlinesAVX2(30, 18325193796, genSig, false,
		10282355196851764065, 10282355196851764065, 10282355196851764065, 10282355196851764065,
		10282355196851764065, 10282355196851764065, 10282355196851764065, 10282355196851764065,
		6729, 6729, 6729, 6729,
		6729, 6729, 6729, 6729)

	assert.Equal(t, uint64(0x1e847df3), d1, "Calculated deadline is incorrect (1)")
	assert.Equal(t, uint64(0x1e847df3), d2, "Calculated deadline is incorrect (2)")
	assert.Equal(t, uint64(0x1e847df3), d3, "Calculated deadline is incorrect (3)")
	assert.Equal(t, uint64(0x1e847df3), d4, "Calculated deadline is incorrect (4)")
	assert.Equal(t, uint64(0x1e847df3), d5, "Calculated deadline is incorrect (5)")
	assert.Equal(t, uint64(0x1e847df3), d6, "Calculated deadline is incorrect (6)")
	assert.Equal(t, uint64(0x1e847df3), d7, "Calculated deadline is incorrect (7)")
	assert.Equal(t, uint64(0x1e847df3), d8, "Calculated deadline is incorrect (8)")

	genSig, _ = DecodeGeneratorSignature("5ba5d36be04b16c36edc7e0c3d342e0853ca7ae41b3364dd8b63154978a315d5")

	d1, _, _, _ = CalculateDeadlinesSSE4(2949, 15604408436, genSig, false,
		6418289488649374107, 0, 0, 0,
		2498, 0, 0, 0)

	assert.Equal(t, uint64(231141), d1, "Calculated deadline is incorrect (5)")

	genSig, _ = DecodeGeneratorSignature("56747285d0a52dbf7f45bcf7b45b86bd48a11315500d5d5424ee3a1e7c63f712")

	d1, d2, d3, d4, d5, d6, d7, d8 = CalculateDeadlinesAVX2(7, 17959135569, genSig, false,
		6418289488649374107, 6418289488649374107, 6418289488649374107, 6418289488649374107,
		6418289488649374107, 6418289488649374107, 6418289488649374107, 6418289488649374107,
		133789, 133789, 133789, 133789,
		133789, 133789, 133789, 133789)

	assert.Equal(t, uint64(0x30d71434), d1, "Calculated deadline is incorrect (9)")
	assert.Equal(t, uint64(0x30d71434), d2, "Calculated deadline is incorrect (10)")
	assert.Equal(t, uint64(0x30d71434), d3, "Calculated deadline is incorrect (11)")
	assert.Equal(t, uint64(0x30d71434), d4, "Calculated deadline is incorrect (12)")
	assert.Equal(t, uint64(0x30d71434), d5, "Calculated deadline is incorrect (13)")
	assert.Equal(t, uint64(0x30d71434), d6, "Calculated deadline is incorrect (14)")
	assert.Equal(t, uint64(0x30d71434), d7, "Calculated deadline is incorrect (15)")
	assert.Equal(t, uint64(0x30d71434), d8, "Calculated deadline is incorrect (16)")
}

func TestDecimalToPlanck(t *testing.T) {
	assert.Equal(t, int64(0x746a528800), DecimalToPlanck(5000.0), "Decimal to planck conversion incorrect (1)")
	assert.Equal(t, int64(0x1f21241900), DecimalToPlanck(1337.0), "Decimal to planck conversion incorrect (1)")
}

func TestPlanckToDecimal(t *testing.T) {
	assert.Equal(t, 5e-05, PlanckToDecimal(5000), "Planck to decimal conversion incorrect (1)")
	assert.Equal(t, 1.337e-05, PlanckToDecimal(1337), "Planck to decimal conversion incorrect (2)")
}

func TestEEPS(t *testing.T) {
	CacheAlphas(10, 3)
	assert.Equal(t, []float64{
		0.0,
		0.0,
		0.1677584641429577,
		0.23376156435101392,
		0.3068528194400547,
		0.38913951208389663,
		0.48401165528888457,
		0.5976405218914749,
		0.7441572118895505,
		1.0}, alphas, "cached alphas are wrong")

	assert.Equal(t, alpha(10), alphas[9], "cached alpha is wrong")
	assert.Equal(t, alpha(1), alphas[0], "cached alpha is wrong")

	assert.Equal(t, 0.0, EEPS(1, 1234), "EEPS should be zero for no submitted deadlines")
	assert.Equal(t, 0.0, EEPS(2, 1234), "EEPS should be zero for no submitted deadlines")
	assert.Equal(t, 3.207651426204214e+10, EEPS(10, 1234), "EEPS should be zero for no submitted deadlines")
}
