package signer_test

import (
	"testing"
	"time"

	"github.com/alpe/akms/pkg/signer"
	"github.com/alpe/akms/pkg/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

func TestSignVote(t *testing.T) {
	pk := ed25519.GenPrivKey()
	privVal := signer.New(pk, &store.MemoryStore{}, log.TestingLogger())

	block1 := types.BlockID{[]byte{1, 2, 3}, types.PartSetHeader{}}
	block2 := types.BlockID{[]byte{3, 2, 1}, types.PartSetHeader{}}
	height, round := int64(10), 1
	voteType := byte(types.PrevoteType)

	// sign a vote for first time
	vote := newVote(privVal.GetAddress(), 0, height, round, voteType, block1)
	err := privVal.SignVote("mychainid", vote)
	require.NoError(t, err, "expected no error signing vote")

	// try to sign the same vote again; should be fine
	err = privVal.SignVote("mychainid", vote)
	require.NoError(t, err, "expected no error on signing same vote")

	// now try some bad votes
	cases := []*types.Vote{
		newVote(privVal.GetAddress(), 0, height, round-1, voteType, block1),   // round regression
		newVote(privVal.GetAddress(), 0, height-1, round, voteType, block1),   // height regression
		newVote(privVal.GetAddress(), 0, height-2, round+4, voteType, block1), // height regression and different round
		newVote(privVal.GetAddress(), 0, height, round, voteType, block2),     // different block
	}

	for _, c := range cases {
		err = privVal.SignVote("mychainid", c)
		assert.Error(t, err, "expected error on signing conflicting vote")
	}

	// try signing a vote with a different time stamp
	sig := vote.Signature
	vote.Timestamp = vote.Timestamp.Add(time.Duration(1000))
	err = privVal.SignVote("mychainid", vote)
	require.NoError(t, err)
	assert.Equal(t, sig, vote.Signature)
}

func TestSignProposal(t *testing.T) {
	pk := ed25519.GenPrivKey()
	privVal := signer.New(pk, &store.MemoryStore{}, log.TestingLogger())

	block1 := types.BlockID{[]byte{1, 2, 3}, types.PartSetHeader{5, []byte{1, 2, 3}}}
	block2 := types.BlockID{[]byte{3, 2, 1}, types.PartSetHeader{10, []byte{3, 2, 1}}}
	height, round := int64(10), 1

	// sign a proposal for first time
	proposal := newProposal(height, round, block1)
	err := privVal.SignProposal("mychainid", proposal)
	require.NoError(t, err, "expected no error signing proposal")

	// try to sign the same proposal again; should be fine
	err = privVal.SignProposal("mychainid", proposal)
	require.NoError(t, err, "expected no error on signing same proposal")

	// now try some bad Proposals
	cases := []*types.Proposal{
		newProposal(height, round-1, block1),   // round regression
		newProposal(height-1, round, block1),   // height regression
		newProposal(height-2, round+4, block1), // height regression and different round
		newProposal(height, round, block2),     // different block
	}

	for _, c := range cases {
		err = privVal.SignProposal("mychainid", c)
		assert.Error(t, err, "expected error on signing conflicting proposal")
	}

	// try signing a proposal with a different time stamp
	sig := proposal.Signature
	proposal.Timestamp = proposal.Timestamp.Add(time.Duration(1000))
	err = privVal.SignProposal("mychainid", proposal)
	require.NoError(t, err)
	assert.Equal(t, sig, proposal.Signature)
}

func TestProposalDifferByTimestamp(t *testing.T) {
	pk := ed25519.GenPrivKey()
	privVal := signer.New(pk, &store.MemoryStore{}, log.TestingLogger())

	block1 := types.BlockID{[]byte{1, 2, 3}, types.PartSetHeader{5, []byte{1, 2, 3}}}
	height, round := int64(10), 1
	chainID := "mychainid"

	firstProposal := newProposal(height, round, block1)
	err := privVal.SignProposal(chainID, firstProposal)
	require.NoError(t, err, "expected no error signing firstProposal")
	originalSignBytes := firstProposal.SignBytes(chainID)
	originalSig := firstProposal.Signature
	originalTimestamp := firstProposal.Timestamp

	// manipulate the timestamp. should get changed back
	secondProposal := newProposal(height, round, block1)
	secondProposal.Timestamp = firstProposal.Timestamp.Add(time.Millisecond)
	var emptySig []byte
	secondProposal.Signature = emptySig
	// when
	err = privVal.SignProposal("mychainid", secondProposal)

	// then
	require.NoError(t, err, "expected no error on signing same firstProposal")
	assert.WithinDuration(t, originalTimestamp, secondProposal.Timestamp, time.Nanosecond)
	assert.Equal(t, originalSignBytes, secondProposal.SignBytes(chainID))
	assert.Equal(t, originalSig, secondProposal.Signature)
}

func TestVoteDifferByTimestamp(t *testing.T) {
	pk := ed25519.GenPrivKey()
	privVal := signer.New(pk, &store.MemoryStore{}, log.TestingLogger())

	height, round := int64(10), 1
	chainID := "mychainid"
	voteType := byte(types.PrevoteType)
	blockID := types.BlockID{[]byte{1, 2, 3}, types.PartSetHeader{}}
	vote := newVote(privVal.GetAddress(), 0, height, round, voteType, blockID)
	err := privVal.SignVote("mychainid", vote)
	require.NoError(t, err, "expected no error signing vote")

	signBytes := vote.SignBytes(chainID)
	sig := vote.Signature
	timeStamp := vote.Timestamp

	// manipulate the timestamp. should get changed back
	vote.Timestamp = vote.Timestamp.Add(time.Millisecond)
	var emptySig []byte
	vote.Signature = emptySig
	err = privVal.SignVote("mychainid", vote)
	require.NoError(t, err, "expected no error on signing same vote")

	assert.Equal(t, timeStamp, vote.Timestamp)
	assert.Equal(t, signBytes, vote.SignBytes(chainID))
	assert.Equal(t, sig, vote.Signature)
}

func newVote(addr types.Address, idx int, height int64, round int, typ byte, blockID types.BlockID) *types.Vote {
	return &types.Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   idx,
		Height:           height,
		Round:            round,
		Type:             types.SignedMsgType(typ),
		Timestamp:        tmtime.Now(),
		BlockID:          blockID,
	}
}

func newProposal(height int64, round int, blockID types.BlockID) *types.Proposal {
	return &types.Proposal{
		Height:    height,
		Round:     round,
		BlockID:   blockID,
		Timestamp: tmtime.Now(),
	}
}
