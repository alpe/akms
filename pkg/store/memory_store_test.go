package store

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemoryStore(t *testing.T) {
	s := &MemoryStore{}
	// when
	err := s.StoreSigned(1, 2, 3, []byte("signedContent"), []byte("signature"))
	// then
	require.NoError(t, err)
	exp := state{1, 2, 3, []byte("signedContent"), []byte("signature")}
	got := s.GetCurrentState()
	if !reflect.DeepEqual(exp, got) {
		t.Errorf("exected:\n%#v\n but got:\n%#v", exp, got)
	}
}

func TestCheckHRS(t *testing.T) {
	t.Parallel()
	specs := map[string]struct {
		iHeight, iRound, iStep int
		nHeight, nRound, nStep int
		expResult, expErr      bool
	}{
		"equal state should be same": {
			1, 1, 1,
			1, 1, 1,
			true, false,
		},
		"new height should be accepted": {
			1, 2, 2,
			2, 1, 1,
			false, false,
		},
		"new round should be accepted": {
			1, 2, 2,
			1, 3, 1,
			false, false,
		},
		"new step should be accepted": {
			1, 2, 2,
			1, 2, 3,
			false, false,
		},
		"lower height should be rejected": {
			2, 2, 2,
			1, 3, 2,
			false, true,
		},
		"lower round should be rejected": {
			1, 2, 2,
			1, 1, 3,
			false, true,
		},
		"lower step should be rejected": {
			1, 2, 2,
			1, 2, 1,
			false, true,
		},
	}
	for desc, spec := range specs {
		t.Run(desc, func(t *testing.T) {
			m := &MemoryStore{}
			m.setNewState(newState(int64(spec.iHeight), spec.iRound, int8(spec.iStep), []byte{}, []byte{}))
			same, err := m.checkHRS(int64(spec.nHeight), spec.nRound, int8(spec.nStep))
			if spec.expErr {
				require.Error(t, err)
			}
			assert.Equal(t, spec.expResult, same)
		})
	}
}
func TestCompareHRSTo(t *testing.T) {
	t.Parallel()
	specs := map[string]struct {
		iHeight, iRound, iStep int
		nHeight, nRound, nStep int
		expResult              int
	}{
		"equal state should be same": {
			1, 1, 1,
			1, 1, 1,
			0,
		},
		"new height should be higher": {
			1, 2, 2,
			2, 1, 1,
			1,
		},
		"new round should be higher": {
			1, 2, 2,
			1, 3, 1,
			1,
		},
		"new step should be higher": {
			1, 2, 2,
			1, 2, 3,
			1,
		},
		"lower height should be smaller": {
			2, 2, 2,
			1, 3, 2,
			-1,
		},
		"lower round should be smaller": {
			1, 2, 2,
			1, 1, 3,
			-1,
		},
		"lower step should be smaler": {
			1, 2, 2,
			1, 2, 1,
			-1,
		},
	}
	for desc, spec := range specs {
		t.Run(desc, func(t *testing.T) {
			s := newState(int64(spec.iHeight), spec.iRound, int8(spec.iStep), []byte{}, []byte{})
			o := newState(int64(spec.nHeight), spec.nRound, int8(spec.nStep), []byte{}, []byte{})
			v := s.CompareHRSTo(o)
			switch {
			case spec.expResult == 0:
				assert.Equal(t, 0, v)
			case spec.expResult < 0:
				assert.True(t, v < 0, v)
			case spec.expResult > 0:
				assert.True(t, v > 0, v)
			}
		})
	}
}
