package modelx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddOrUpdate(t *testing.T) {
	bs := newBlocks(10)
	ns := []uint64{9, 2, 3, 16, 17, 18, 19, 20, 1, 10,
		12, 11, 13, 14, 15, 4, 5, 6, 7, 8}

	for _, n := range ns {
		bs.add(n)
	}

	assert.Equal(t, 10, len(bs.index))
	assert.Equal(t, 10, bs.heights.Len())

	for i, j := 0, uint64(11); i < 10; i, j = i+1, j+1 {
		e, exists := bs.index[j]
		assert.True(t, exists)
		assert.Equal(t, j, e.Value.(uint64))
	}

	for i, e := uint64(11), bs.heights.Front(); i <= 20; i, e = i+1, e.Next() {
		h := e.Value.(uint64)
		assert.Equal(t, i, h)
	}
}
