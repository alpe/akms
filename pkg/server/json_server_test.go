package server

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/alpe/akms/pkg/signer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/types"
)

func TestGetPubKey(t *testing.T) {
	f, _ := ioutil.TempFile("", "test-state.json")
	filePV := privval.GenFilePV(f.Name(), filepath.Dir(f.Name()))
	chainID := "testChain"
	s := httptest.NewServer(Routes(filePV, chainID))
	c := signer.RestClient{
		BaseAddress: s.URL,
		Logger:      log.TestingLogger(),
		HttpClient:  &http.Client{Timeout: 100 * time.Millisecond},
	}
	// when && then
	require.Equal(t, filePV.GetPubKey(), c.GetPubKey())
}

func TestSignVote(t *testing.T) {
	f, _ := ioutil.TempFile("", "test-state.json")
	filePV := privval.GenFilePV(f.Name(), filepath.Dir(f.Name()))
	chainID := "testChain"
	s := httptest.NewServer(Routes(filePV, chainID))
	c := signer.RestClient{
		BaseAddress: s.URL,
		Logger:      log.TestingLogger(),
		HttpClient:  &http.Client{Timeout: 100 * time.Millisecond},
	}
	// and vote signed
	aVote := types.Vote{
		Type:             types.PrevoteType,
		Height:           1,
		Round:            2,
		Timestamp:        time.Now(),
		BlockID:          types.BlockID{},
		ValidatorAddress: filePV.GetAddress(), // any address
		ValidatorIndex:   3,
	}
	aVoteCopy := *aVote.Copy()
	// whne
	require.NoError(t, filePV.SignVote(chainID, &aVote))
	require.NoError(t, c.SignVote(chainID, &aVoteCopy))

	// then
	assert.Equal(t, aVote.Type, aVoteCopy.Type)
	assert.Equal(t, aVote.Height, aVoteCopy.Height)
	assert.Equal(t, aVote.Round, aVoteCopy.Round)
	assert.True(t, aVote.Timestamp.Equal(aVoteCopy.Timestamp))
	assert.True(t, aVote.BlockID.Equals(aVoteCopy.BlockID))
	assert.Equal(t, aVote.ValidatorAddress.Bytes(), aVoteCopy.ValidatorAddress.Bytes())
	assert.Equal(t, aVote.ValidatorIndex, aVoteCopy.ValidatorIndex)
	assert.Equal(t, aVote.Signature, aVoteCopy.Signature)
	defer s.Close()
}

func TestSignProposal(t *testing.T) {
	f, _ := ioutil.TempFile("", "test-state.json")
	filePV := privval.GenFilePV(f.Name(), filepath.Dir(f.Name()))
	chainID := "testChain"

	s := httptest.NewServer(Routes(filePV, chainID))
	c := signer.RestClient{
		BaseAddress: s.URL,
		Logger:      log.TestingLogger(),
		HttpClient:  &http.Client{Timeout: 100 * time.Millisecond},
	}

	aProposal := types.Proposal{
		Type:      types.PrevoteType,
		Height:    1,
		Round:     2,
		POLRound:  3,
		BlockID:   types.BlockID{},
		Timestamp: time.Now(),
	}

	copyProposal := func(p types.Proposal) types.Proposal {
		return p
	}
	proposalCopy := copyProposal(aProposal)
	// whne
	require.NoError(t, filePV.SignProposal(chainID, &aProposal))
	require.NoError(t, c.SignProposal(chainID, &proposalCopy))

	// then
	assert.Equal(t, aProposal.Type, proposalCopy.Type)
	assert.Equal(t, aProposal.Height, proposalCopy.Height)
	assert.Equal(t, aProposal.Round, proposalCopy.Round)
	assert.Equal(t, aProposal.POLRound, proposalCopy.POLRound)
	assert.True(t, aProposal.BlockID.Equals(proposalCopy.BlockID))
	assert.True(t, aProposal.Timestamp.Equal(proposalCopy.Timestamp))
	assert.Equal(t, aProposal.Signature, proposalCopy.Signature)
	defer s.Close()
}
