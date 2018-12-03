package signer_test

import (
	"github.com/alpe/akms/pkg/signer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestRetrySignVote(t *testing.T) {
	pk := ed25519.GenPrivKey()
	s, callsCounter := startBlockingSever()
	defer s.Close()
	r := signer.NewRestClient(log.TestingLogger(), &http.Client{Timeout: 20 * time.Millisecond}, s.URL)

	// when
	vote := newVote(pk.PubKey().Address(), 0, 1, 2, 3, types.BlockID{[]byte{1, 2, 3}, types.PartSetHeader{}})
	err := r.SignVote("myChainID", vote)

	//then
	require.Error(t, err)
	assert.Contains(t, err.Error(), "net/http: request canceled")
	assert.Equal(t, 6, callsCounter())
}

func TestRetryGetPubKey(t *testing.T) {
	s, callsCounter := startBlockingSever()
	defer s.Close()
	r := signer.NewRestClient(log.TestingLogger(), &http.Client{Timeout: 20 * time.Millisecond}, s.URL)

	// when
	pubKey := r.GetPubKey()

	//then
	assert.Nil(t, pubKey)
	assert.Equal(t, 6, callsCounter())
}

func TestRetrySignProposal(t *testing.T) {
	s, callsCounter := startBlockingSever()
	defer s.Close()
	r := signer.NewRestClient(log.TestingLogger(), &http.Client{Timeout: 20 * time.Millisecond}, s.URL)

	aProposal := types.Proposal{
		Type:      types.PrevoteType,
		Height:    1,
		Round:     2,
		POLRound:  3,
		BlockID:   types.BlockID{},
		Timestamp: time.Now(),
	}

	// when
	err := r.SignProposal("myChainId", &aProposal)

	//then
	require.Error(t, err)
	assert.Contains(t, err.Error(), "net/http: request canceled")
	assert.Equal(t, 6, callsCounter())
}

func startBlockingSever() (*httptest.Server, func() int) {
	var mu sync.Mutex
	var callsCounter int
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callsCounter++
		mu.Unlock()
		time.Sleep(2 * time.Second)
	}))
	return s, func() int {
		mu.Lock()
		defer mu.Unlock()
		return callsCounter
	}
}
