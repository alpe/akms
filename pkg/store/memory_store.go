package store

import (
	"bytes"
	"sync"

	"github.com/pkg/errors"
	tmcommon "github.com/tendermint/tendermint/libs/common"
)

type state struct {
	LastHeight    int64  `json:"last_height"`
	LastRound     int    `json:"last_round"`
	LastStep      int8   `json:"last_step"`
	LastSignBytes []byte `json:"last_sign_bytes"`
	LastSignature []byte `json:"last_signature"`
}

func (s state) Equal(o state) bool {
	return s.LastHeight == o.LastHeight &&
		s.LastRound == o.LastRound &&
		s.LastStep == o.LastStep &&
		bytes.Equal(s.LastSignBytes, o.LastSignBytes) &&
		bytes.Equal(s.LastSignature, o.LastSignature)
}

// CompareHRSTo return `< 0` when this is later than other
// 0 when equal
// `> 0` when other is later
func (s state) CompareHRSTo(o state) int {
	if hDiff := o.LastHeight - s.LastHeight; hDiff != 0 {
		return int(hDiff)
	}
	if rDiff := o.LastRound - s.LastRound; rDiff != 0 {
		return rDiff
	}
	return int(o.LastStep - s.LastStep)
}

type MemoryStore struct {
	mx    sync.Mutex
	state state
}

// IsCurrent returns error if HRS regression or no LastSignBytes. returns true if HRS is unchanged
func (m *MemoryStore) IsCurrent(height int64, round int, step int8) (bool, error) {
	m.mx.Lock()
	defer m.mx.Unlock()

	same, err := m.checkHRS(height, round, step)
	switch {
	case err != nil:
		return false, err
	case !same:
		return false, nil
	case m.state.LastSignBytes == nil:
		return false, errors.New("last sign bytes not found")
	case m.state.LastSignature == nil:
		return false, errors.New("last signature not found")
	}
	return true, nil
}

func (m *MemoryStore) checkHRS(height int64, round int, step int8) (bool, error) {
	switch {
	case m.state.LastHeight > height:
		return false, errors.New("height regression")
	case m.state.LastHeight < height:
		return false, nil
	case m.state.LastRound > round:
		return false, errors.New("round regression")
	case m.state.LastRound < round:
		return false, nil
	case m.state.LastStep > step:
		return false, errors.New("step regression")
	case m.state.LastStep < step:
		return false, nil
	}
	return true, nil
}

func (m *MemoryStore) GetLastSignature() []byte {
	m.mx.Lock()
	defer m.mx.Unlock()

	return m.state.LastSignature
}

func (m *MemoryStore) GetLastSignBytes() tmcommon.HexBytes {
	m.mx.Lock()
	defer m.mx.Unlock()

	return m.state.LastSignBytes
}

func (m *MemoryStore) StoreSigned(height int64, round int, step int8, signBytes []byte, sig []byte) error {
	m.setNewState(newState(height, round, step, signBytes, sig))
	return nil
}

func newState(height int64, round int, step int8, signBytes []byte, sig []byte) state {
	newSignBytes := make([]byte, len(signBytes))
	copy(newSignBytes, signBytes)

	newSig := make([]byte, len(sig))
	copy(newSig, sig)

	return state{
		LastHeight:    height,
		LastRound:     round,
		LastStep:      step,
		LastSignBytes: newSignBytes,
		LastSignature: newSig,
	}
}

func (m *MemoryStore) setNewState(s state) {
	defer m.mx.Unlock()
	m.mx.Lock()
	m.state = s
}

func (m *MemoryStore) GetCurrentState() state {
	defer m.mx.Unlock()
	m.mx.Lock()
	return m.state
}

func (m *MemoryStore) IsLeader() bool {
	return true
}
