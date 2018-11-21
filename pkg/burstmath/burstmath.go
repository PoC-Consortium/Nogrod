package burstmath

// #cgo LDFLAGS: -Llibs -lburstmath
/*
#include "libs/burstmath.h"
#include "stdlib.h"

CalcDeadlineRequest** alloc_reqs_avx2() {
  return (CalcDeadlineRequest**) malloc(8 * sizeof(CalcDeadlineRequest*));
}
*/
import "C"

import (
	"encoding/hex"
	"errors"
	"time"
	"unsafe"

	"github.com/klauspost/cpuid"
)

const (
	avx2Parallel    = 8
	sse4Parallel    = 4
	blockChainStart = 1407722400

	// GenesisBaseTarget is the base target of the first block
	GenesisBaseTarget = 18325193796

	// BlockChainStart is the timestamp of the first Block
	BlockChainStart = 1407722400
)

// CalcDeadlineRequest stores paramters native that are
// needed for deadline calculation
type CalcDeadlineRequest struct {
	native   *C.CalcDeadlineRequest
	deadline chan uint64
}

// NewCalcDeadlineRequest allocated paramters neeeded for deadline
// calculation native so that C can deal with it
func NewCalcDeadlineRequest(accountID, nonce, baseTarget uint64, scoop uint32, genSig []byte) *CalcDeadlineRequest {
	var deadline C.uint64_t
	return &CalcDeadlineRequest{
		native: &C.CalcDeadlineRequest{
			account_id:  C.uint64_t(accountID),
			nonce:       C.uint64_t(nonce),
			base_target: C.uint64_t(baseTarget),
			scoop_nr:    C.uint32_t(scoop),
			gen_sig:     (*C.uint8_t)(unsafe.Pointer(&genSig[0])),
			deadline:    deadline},
		deadline: make(chan uint64)}
}

func newCalcDeadlineRequests() []*C.CalcDeadlineRequest {
	var arr **C.CalcDeadlineRequest = C.alloc_reqs_avx2()
	return (*[avx2Parallel]*C.CalcDeadlineRequest)(unsafe.Pointer(arr))[:avx2Parallel:avx2Parallel]
}

// CalcScoop calculated the scoop for a given height and generation signature
func CalcScoop(height uint64, genSig []byte) uint32 {
	return uint32(C.calculate_scoop(C.uint64_t(height), (*C.uint8_t)(&genSig[0])))
}

// CalculateDeadline calculates a single deadline
// TODO: might fail on some go versions(cgo argument has Go pointer to Go pointer): GODEBUG=cgocheck=0
// func CalculateDeadline(req *C.CalcDeadlineRequest) {
// 	C.calculate_deadline(req)
// }

// CalculateDeadlinesSSE4 can calculate 4 deadlines in parallel using sse4 intrinsics
func CalculateDeadlinesSSE4(reqs []*C.CalcDeadlineRequest) {
	C.calculate_deadlines_sse4((**C.CalcDeadlineRequest)(unsafe.Pointer(&reqs[0])))
}

// CalculateDeadlinesAVX2 can calculate 8 deadlines in parallel using avx2 vector extensions
func CalculateDeadlinesAVX2(reqs []*C.CalcDeadlineRequest) {
	C.calculate_deadlines_avx2((**C.CalcDeadlineRequest)(unsafe.Pointer(&reqs[0])))
}

// DecodeGeneratorSignature transforms the generation signature given as hex string into a byte string
func DecodeGeneratorSignature(genSigStr string) ([]byte, error) {
	if len(genSigStr) != 64 {
		return nil, errors.New("Generation signature's length differs from 64")
	}

	genSig, err := hex.DecodeString(genSigStr)
	if err != nil {
		return nil, err
	}

	return genSig, nil
}

// DeadlineRequestHandler is used for dispatching deadline calculation requests among workers
type DeadlineRequestHandler struct {
	reqs      chan *CalcDeadlineRequest
	batchReqs chan calcDeadlineRequestBatch
	stop      chan struct{}
	workers   []*worker
	timeout   time.Duration
}

type worker struct {
	reqBatches chan calcDeadlineRequestBatch
	stop       chan struct{}
	creqs      []*C.CalcDeadlineRequest
	avx2       bool
}

type calcDeadlineRequestBatch struct {
	pending int
	reqs    [avx2Parallel]*CalcDeadlineRequest
}

// NewDeadlineRequestHandler creates a new struct that spawns workerCount workers
// processing deadline requests
func NewDeadlineRequestHandler(workerCount int, timeoutSeconds ...int64) *DeadlineRequestHandler {
	var timeout time.Duration
	if len(timeoutSeconds) > 0 {
		if timeoutSeconds[0] < 0 {
			panic("timeout cannot be negativ")
		}
		timeout = time.Duration(timeoutSeconds[0]) * time.Second
	} else {
		timeout = 2 * time.Second
	}

	reqHandler := &DeadlineRequestHandler{
		workers:   make([]*worker, workerCount),
		reqs:      make(chan *CalcDeadlineRequest),
		batchReqs: make(chan calcDeadlineRequestBatch, workerCount),
		stop:      make(chan struct{}),
		timeout:   timeout}

	var avx2 bool
	switch {
	case cpuid.CPU.AVX2():
		avx2 = true
		go reqHandler.collectDeadlineReqsAVX2()
	case cpuid.CPU.SSE4():
		go reqHandler.collectDeadlineReqsSSE4()
	default:
		panic("avx2 and sse4 not available")
	}

	for i := 0; i < workerCount; i++ {
		reqHandler.workers[i] = newWorker(reqHandler.batchReqs, avx2)
	}

	return reqHandler
}

// CalcDeadline calculates a deadline
func (reqHandler *DeadlineRequestHandler) CalcDeadline(req *CalcDeadlineRequest) uint64 {
	reqHandler.reqs <- req
	return <-req.deadline
}

func (reqHandler *DeadlineRequestHandler) collectDeadlineReqsAVX2() {
	var timeout <-chan time.Time

	var reqs [avx2Parallel]*CalcDeadlineRequest
	for i := 0; i < avx2Parallel; i++ {
		reqs[i] = &CalcDeadlineRequest{}
	}

	var pending int
	for {
		select {
		case reqs[pending] = <-reqHandler.reqs:
			if pending == 0 {
				timeout = time.After(reqHandler.timeout)
			} else if pending == avx2Parallel-1 {
				reqHandler.batchReqs <- calcDeadlineRequestBatch{
					pending: pending + 1,
					reqs:    reqs}
				pending = 0
				continue
			}
			pending++
		case <-timeout:
			reqHandler.batchReqs <- calcDeadlineRequestBatch{
				pending: pending,
				reqs:    reqs}
			pending = 0
		case <-reqHandler.stop:
			return
		}
	}
}

func (reqHandler *DeadlineRequestHandler) collectDeadlineReqsSSE4() {
	var timeout <-chan time.Time

	var reqs [avx2Parallel]*CalcDeadlineRequest
	for i := 0; i < sse4Parallel; i++ {
		reqs[i] = &CalcDeadlineRequest{}
	}

	var pending int
	for {
		select {
		case reqs[pending] = <-reqHandler.reqs:
			if pending == 0 {
				timeout = time.After(2 * time.Second)
			} else if pending == sse4Parallel-1 {
				reqHandler.batchReqs <- calcDeadlineRequestBatch{
					pending: pending + 1,
					reqs:    reqs}
				pending = 0
				continue
			}
			pending++
		case <-timeout:
			reqHandler.batchReqs <- calcDeadlineRequestBatch{
				pending: pending,
				reqs:    reqs}
			pending = 0
		case <-reqHandler.stop:
			return
		}
	}
}

// Stop stops all go routines started inside the request handler
func (reqHandler *DeadlineRequestHandler) Stop() {
	for _, w := range reqHandler.workers {
		w.stop <- struct{}{}
		C.free(unsafe.Pointer(&w.creqs[0]))
	}
	reqHandler.stop <- struct{}{}
}

func newWorker(reqBatches chan calcDeadlineRequestBatch, avx2 bool) *worker {
	w := &worker{
		reqBatches: reqBatches,
		stop:       make(chan struct{}),
		creqs:      newCalcDeadlineRequests(),
		avx2:       avx2}

	go func() {
		for {
			select {
			case reqBatch := <-w.reqBatches:
				w.processReqs(reqBatch.reqs, reqBatch.pending)
			case <-w.stop:
				return
			}
		}
	}()

	return w
}

func (w *worker) processReqs(reqs [avx2Parallel]*CalcDeadlineRequest, total int) {
	for i := 0; i < total; i++ {
		w.creqs[i] = reqs[i].native
	}

	// dummies to avoid seg faults
	for i := total; i < avx2Parallel; i++ {
		w.creqs[i] = reqs[0].native
	}

	if w.avx2 {
		CalculateDeadlinesAVX2(w.creqs)
	} else {
		CalculateDeadlinesSSE4(w.creqs)
	}

	for i := 0; i < total; i++ {
		reqs[i].deadline <- uint64(reqs[i].native.deadline)
	}
}

// BurstToPlanck converts an amount in burst to an amount in burst
func BurstToPlanck(n float64) int64 {
	return int64(n * 100000000)
}

// PlanckToBurst converts an amount in placnk to an amount in burst
func PlanckToBurst(n int64) float64 {
	return float64(n) / 100000000.0
}

// DateToTimeStamp yields a timestamp counted since block chain start
func DateToTimeStamp(date time.Time) int64 {
	ts := date.Unix() - blockChainStart
	if ts < 0 {
		return 0
	}
	return ts
}
