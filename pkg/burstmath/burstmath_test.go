package burstmath

import (
	"sync"
	"testing"
	"time"

	"github.com/klauspost/cpuid"
	"github.com/stretchr/testify/assert"
)

func TestCalcScoop(t *testing.T) {
	genSig, _ := DecodeGeneratorSignature("2a0757c8af2aa43b29515c872385ede31d0742b1ea29b93a1a8c38a11b8a37a0")
	assert.Equal(t, uint32(0x1e), CalcScoop(41189, genSig), "Calculated scoop is incorrect (1)")

	genSig, _ = DecodeGeneratorSignature("56747285d0a52dbf7f45bcf7b45b86bd48a11315500d5d5424ee3a1e7c63f712")
	assert.Equal(t, uint32(0x07), CalcScoop(41190, genSig), "Calculated scoop is incorrect (2)")

	_, err := DecodeGeneratorSignature("2a0757c8af2aa43b29515c872385ede31d0742b1ea29b93a1a8c38a11b8a37a")
	assert.NotNil(t, err, "did not return error if gen sig is too short")

	_, err = DecodeGeneratorSignature("2a0757c8af2aa43b29515c872385ede31d0742b1ea29b93a1a8c38a11b8a37aaa")
	assert.NotNil(t, err, "did not return error if gen sig is too long")

	_, err = DecodeGeneratorSignature("2a0757c8af2aa43b29515c872385ede31d0742b1ea29b93a1a8c38a11b8a37ao")
	assert.NotNil(t, err, "did not return error if hex string is not valid")
}

func TestCalculateDeadlines(t *testing.T) {
	genSig, _ := DecodeGeneratorSignature("2a0757c8af2aa43b29515c872385ede31d0742b1ea29b93a1a8c38a11b8a37a0")

	reqs := newCalcDeadlineRequests()
	for i := 0; i < 8; i++ {
		reqs[i] = NewCalcDeadlineRequest(10282355196851764065, 6729, 18325193796, 30, genSig).native
	}

	// CalculateDeadline(reqs[0])
	// assert.Equal(t, uint64(0x37143a0a), uint64(reqs[0].deadline), "Calculated deadline is incorrect")

	if !cpuid.CPU.SSE4() {
		t.Log("SSE4 not supported, skipping related tests")
		return
	}

	CalculateDeadlinesSSE4(reqs)
	for i := 0; i < 4; i++ {
		assert.Equal(t, uint64(0x37143a0a), uint64(reqs[i].deadline),
			"Calculated deadline is incorrect SSE4")
	}

	if !cpuid.CPU.AVX2() {
		t.Log("AVX2 not supported, skipping related tests")
		return
	}

	CalculateDeadlinesAVX2(reqs)
	for i := 0; i < 8; i++ {
		assert.Equal(t, uint64(0x37143a0a), uint64(reqs[i].deadline),
			"Calculated deadline is incorrect AVX2")
	}
}

func TestAll(t *testing.T) {
	reqHandler := NewDeadlineRequestHandler(4, 3)

	assert.Equal(t, 3*time.Second, reqHandler.timeout, "timout is wrong")

	genSig, _ := DecodeGeneratorSignature("2a0757c8af2aa43b29515c872385ede31d0742b1ea29b93a1a8c38a11b8a37a0")
	req := NewCalcDeadlineRequest(10282355196851764065, 6729, 18325193796, 30, genSig)

	deadline := reqHandler.CalcDeadline(req)

	assert.Equal(t, uint64(0x37143a0a), deadline, "Calculated deadline is incorrect")

	reqHandler.Stop()
}

func TestBurstToPlanck(t *testing.T) {
	assert.Equal(t, int64(0x746a528800), BurstToPlanck(5000.0), "Decimal to planck conversion incorrect (1)")
	assert.Equal(t, int64(0x1f21241900), BurstToPlanck(1337.0), "Decimal to planck conversion incorrect (1)")
}

func TestPlanckToBurst(t *testing.T) {
	assert.Equal(t, 5e-05, PlanckToBurst(5000), "Planck to decimal conversion incorrect (1)")
	assert.Equal(t, 1.337e-05, PlanckToBurst(1337), "Planck to decimal conversion incorrect (2)")
}

func TestDateToTimeStamp(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")
	assert.Equal(t, int64(0), DateToTimeStamp(time.Date(1995, time.August, 2, 2, 2, 0, 0, loc)))
	assert.Equal(t, int64(62380920), DateToTimeStamp(time.Date(2016, time.August, 2, 2, 2, 0, 0, loc)))
}

func BenchmarkCalcDeadline(b *testing.B) {
	reqHandler := NewDeadlineRequestHandler(8)
	genSig, _ := DecodeGeneratorSignature("2a0757c8af2aa43b29515c872385ede31d0742b1ea29b93a1a8c38a11b8a37a0")

	sem := make(chan struct{}, 100)
	var wg sync.WaitGroup
	for n := 0; n < b.N-b.N%8; n++ {
		sem <- struct{}{}
		wg.Add(1)
		go func(accountID uint64) {
			req := NewCalcDeadlineRequest(accountID, 6729, 18325193796, 30, genSig)
			reqHandler.CalcDeadline(req)
			<-sem
			wg.Done()

		}(uint64(n))
	}
	wg.Wait()
}
