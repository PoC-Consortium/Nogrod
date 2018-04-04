// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

package util

// #cgo LDFLAGS: -L../libs -lutils
// #include "../libs//utils.h"
import "C"

import (
	"encoding/hex"
	"errors"
	"math"
	"time"
)

var alphas []float64

const (
	GenesisBaseTarget = 18325193796
	BlockChainStart   = 1407722400
)

func CacheAlphas(nAvg int, nMin int) {
	alphas = make([]float64, nAvg)
	for i := 0; i < nAvg; i++ {
		if i < nMin-1 {
			alphas[i] = 0.0
		} else {
			nConf := float64(i + 1)
			alphas[i] = 1.0 - (float64(nAvg)-nConf)/nConf*math.Log(float64(nAvg)/(float64(nAvg)-nConf))
		}
	}
	alphas[nAvg-1] = 1.0
}

func alpha(nConf int) float64 {
	if nConf == 0 {
		return 0.0
	}
	if len(alphas) < nConf {
		return 1.0
	}
	return alphas[nConf-1]
}

func EEPS(nConf int, weightedDeadlineSum float64) float64 {
	if weightedDeadlineSum == 0 {
		return 0.0
	}
	return alpha(nConf) * 240.0 * float64(nConf-1) / (weightedDeadlineSum / float64(GenesisBaseTarget))
}

func WeightDeadline(deadline, baseTarget uint64) float64 {
	return float64(deadline * baseTarget)
}

func DecodeGeneratorSignature(genSigStr string) ([]byte, error) {
	if len(genSigStr) != 64 {
		errMsg := "Generation signature's length differs from 64"
		return []byte{}, errors.New(errMsg)
	}

	genSig, err := hex.DecodeString(genSigStr)

	if err != nil {
		return []byte{}, err
	}

	return genSig, nil
}

func CalculateScoop(height uint64, genSig []byte) uint32 {
	return uint32(C.calculate_scoop(C.uint64_t(height), (*C.uint8_t)(&genSig[0])))
}

func CalculateDeadlinesAVX2(
	scoop uint32, baseTarget uint64, genSig []byte, poc2 bool,
	accountID1 uint64, accountID2 uint64, accountID3 uint64, accountID4 uint64,
	accountID5 uint64, accountID6 uint64, accountID7 uint64, accountID8 uint64,
	nonce1 uint64, nonce2 uint64, nonce3 uint64, nonce4 uint64,
	nonce5 uint64, nonce6 uint64, nonce7 uint64, nonce8 uint64) (uint64, uint64, uint64, uint64,
	uint64, uint64, uint64, uint64) {

	var deadline1, deadline2, deadline3, deadline4,
		deadline5, deadline6, deadline7, deadline8 C.uint64_t

	C.calculate_deadlines_avx2(
		C.uint32_t(scoop), C.uint64_t(baseTarget), (*C.uint8_t)(&genSig[0]), C.bool(poc2),
		C.uint64_t(accountID1), C.uint64_t(accountID2), C.uint64_t(accountID3), C.uint64_t(accountID4),
		C.uint64_t(accountID5), C.uint64_t(accountID6), C.uint64_t(accountID7), C.uint64_t(accountID8),
		C.uint64_t(nonce1), C.uint64_t(nonce2), C.uint64_t(nonce3), C.uint64_t(nonce4),
		C.uint64_t(nonce5), C.uint64_t(nonce6), C.uint64_t(nonce7), C.uint64_t(nonce8),
		&deadline1, &deadline2, &deadline3, &deadline4,
		&deadline5, &deadline6, &deadline7, &deadline8)

	return uint64(deadline1), uint64(deadline2), uint64(deadline3), uint64(deadline4),
		uint64(deadline5), uint64(deadline6), uint64(deadline7), uint64(deadline8)
}

func CalculateDeadlinesSSE4(
	scoop uint32, baseTarget uint64, genSig []byte, poc2 bool,
	accountID1 uint64, accountID2 uint64, accountID3 uint64, accountID4 uint64,
	nonce1 uint64, nonce2 uint64, nonce3 uint64, nonce4 uint64) (uint64, uint64, uint64, uint64) {

	var deadline1, deadline2, deadline3, deadline4 C.uint64_t

	C.calculate_deadlines_sse4(
		C.uint32_t(scoop), C.uint64_t(baseTarget), (*C.uint8_t)(&genSig[0]), C.bool(poc2),
		C.uint64_t(accountID1), C.uint64_t(accountID2), C.uint64_t(accountID3), C.uint64_t(accountID4),
		C.uint64_t(nonce1), C.uint64_t(nonce2), C.uint64_t(nonce3), C.uint64_t(nonce4),
		&deadline1, &deadline2, &deadline3, &deadline4)

	return uint64(deadline1), uint64(deadline2), uint64(deadline3), uint64(deadline4)
}

func DecimalToPlanck(n float64) int64 {
	return int64(n * 100000000)
}

func PlanckToDecimal(n int64) float64 {
	return float64(n) / 100000000.0
}

func Round(f float64) int64 {
	if math.Abs(f) < 0.5 {
		return 0
	}
	return int64(f + math.Copysign(0.5, f))
}

func DateToTimeStamp(date time.Time) int64 {
	ts := date.Unix() - BlockChainStart
	if ts < 0 {
		return 0
	}
	return ts
}
