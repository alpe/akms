package signer

import (
	"bytes"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/encoding/amino"
	tmcommon "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

const (
	stepNone      int8 = 0 // Used to distinguish the initial state
	stepPropose   int8 = 1
	stepPrevote   int8 = 2
	stepPrecommit int8 = 3
)

var voteToStep = map[types.SignedMsgType]int8{
	types.PrevoteType:   stepPrevote,
	types.PrecommitType: stepPrecommit,
}

var cdc = amino.NewCodec()

func init() {
	cryptoAmino.RegisterAmino(cdc)
}

var _ types.PrivValidator = (*Signer)(nil)

type store interface {
	GetLastSignature() []byte
	GetLastSignBytes() tmcommon.HexBytes
	StoreSigned(height int64, round int, step int8, signBytes []byte, sig []byte) error
	IsCurrent(height int64, round int, step int8) (bool, error)
	IsLeader() bool
}

// Signer was very much inspired and contains code copied from https://github.com/tendermint/tendermint/tree/master/privval
type Signer struct {
	m       sync.Mutex
	Logger  log.Logger
	PrivKey crypto.PrivKey
	Store   store
}

func New(privKey crypto.PrivKey, store store, logger log.Logger) *Signer {
	return &Signer{
		PrivKey: privKey,
		Store:   store,
		Logger:  logger,
	}
}

func (s *Signer) GetAddress() types.Address {
	return s.GetPubKey().Address()
}

func (s *Signer) IsLeader() bool {
	return s.Store.IsLeader()
}

func (s *Signer) GetPubKey() crypto.PubKey {
	return s.PrivKey.PubKey()
}

func (pv *Signer) SignVote(chainID string, vote *types.Vote) error {
	pv.m.Lock()
	defer pv.m.Unlock()

	if err := pv.signVote(chainID, vote); err != nil {
		return errors.Wrap(err, "Error signing vote")
	}
	return nil

}

func (pv *Signer) SignProposal(chainID string, proposal *types.Proposal) error {
	pv.m.Lock()
	defer pv.m.Unlock()
	return pv.signProposal(chainID, proposal)
}

// signVote checks if the vote is good to sign and sets the vote signature.
// It may need to set the timestamp as well if the vote is otherwise the same as
// a previously signed vote (ie. we crashed after signing but before the vote hit the WAL).
func (pv *Signer) signVote(chainID string, vote *types.Vote) error {
	step, ok := voteToStep[vote.Type]
	if !ok {
		return errors.Errorf("Unsupported type %X", vote.Type)
	}
	height, round := vote.Height, vote.Round
	signBytes := vote.SignBytes(chainID)
	sameHRS, err := pv.Store.IsCurrent(height, round, step)
	if err != nil {
		return err
	}

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS.
	// If signbytes are the same, use the last signature.
	// If they only differ by timestamp, use last timestamp and signature
	// Otherwise, return error
	if sameHRS {
		if bytes.Equal(signBytes, pv.Store.GetLastSignBytes()) {
			vote.Signature = pv.Store.GetLastSignature()
			return nil
		}
		switch timestamp, ok, err := checkVotesOnlyDifferByTimestamp(pv.Store.GetLastSignBytes(), signBytes); {
		case err != nil:
			return err
		case ok:
			vote.Timestamp = timestamp
			vote.Signature = pv.Store.GetLastSignature()
			return nil
		default:
			return errors.New("conflicting data")
		}
	}

	// It passed the checks. Sign the vote
	sig, err := pv.PrivKey.Sign(signBytes)
	if err != nil {
		return err
	}
	vote.Signature = sig
	return pv.Store.StoreSigned(height, round, step, signBytes, sig)
}

// signProposal checks if the proposal is good to sign and sets the proposal signature.
// It may need to set the timestamp as well if the proposal is otherwise the same as
// a previously signed proposal ie. we crashed after signing but before the proposal hit the WAL).
func (pv *Signer) signProposal(chainID string, proposal *types.Proposal) error {
	height, round, step := proposal.Height, proposal.Round, stepPropose
	signBytes := proposal.SignBytes(chainID)
	sameHRS, err := pv.Store.IsCurrent(height, round, step)
	if err != nil {
		return err
	}

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS.
	// If signbytes are the same, use the last signature.
	// If they only differ by timestamp, use last timestamp and signature
	// Otherwise, return error
	if sameHRS {
		if bytes.Equal(signBytes, pv.Store.GetLastSignBytes()) {
			proposal.Signature = pv.Store.GetLastSignature()
			return nil
		}
		switch timestamp, ok, err := checkProposalsOnlyDifferByTimestamp(pv.Store.GetLastSignBytes(), signBytes); {
		case err != nil:
			return err
		case ok:
			proposal.Timestamp = timestamp
			proposal.Signature = pv.Store.GetLastSignature()
			return nil
		default:
			return errors.New("conflicting data")
		}
	}

	// It passed the checks. Sign the proposal
	sig, err := pv.PrivKey.Sign(signBytes)
	if err != nil {
		return err
	}
	proposal.Signature = sig
	return pv.Store.StoreSigned(height, round, step, signBytes, sig)
}

// returns the timestamp from the lastSignBytes.
// returns true if the only difference in the votes is their timestamp.
func checkVotesOnlyDifferByTimestamp(lastSignBytes, newSignBytes []byte) (time.Time, bool, error) {
	var lastVote, newVote types.CanonicalVote
	if err := cdc.UnmarshalBinaryLengthPrefixed(lastSignBytes, &lastVote); err != nil {
		return time.Time{}, false, errors.Wrap(err, "lastSignBytes cannot be unmarshalled into vote")
	}
	if err := cdc.UnmarshalBinaryLengthPrefixed(newSignBytes, &newVote); err != nil {
		return time.Time{}, false, errors.Wrap(err, "signBytes cannot be unmarshalled into vote")
	}

	lastTime := lastVote.Timestamp

	// set the times to the same value and check equality
	now := tmtime.Now()
	lastVote.Timestamp = now
	newVote.Timestamp = now
	lastVoteBytes, _ := cdc.MarshalJSON(lastVote)
	newVoteBytes, _ := cdc.MarshalJSON(newVote)

	return lastTime, bytes.Equal(newVoteBytes, lastVoteBytes), nil
}

// returns the timestamp from the lastSignBytes.
// returns true if the only difference in the proposals is their timestamp
func checkProposalsOnlyDifferByTimestamp(lastSignBytes, newSignBytes []byte) (time.Time, bool, error) {
	var lastProposal, newProposal types.CanonicalProposal
	if err := cdc.UnmarshalBinaryLengthPrefixed(lastSignBytes, &lastProposal); err != nil {
		return time.Time{}, false, errors.Wrap(err, "lastSignBytes cannot be unmarshalled into vote")
	}
	if err := cdc.UnmarshalBinaryLengthPrefixed(newSignBytes, &newProposal); err != nil {
		return time.Time{}, false, errors.Wrap(err, "signBytes cannot be unmarshalled into vote")
	}

	lastTime := lastProposal.Timestamp
	// set the times to the same value and check equality
	now := tmtime.Now()
	lastProposal.Timestamp = now
	newProposal.Timestamp = now
	lastProposalBytes, _ := cdc.MarshalBinaryLengthPrefixed(lastProposal)
	newProposalBytes, _ := cdc.MarshalBinaryLengthPrefixed(newProposal)

	return lastTime, bytes.Equal(newProposalBytes, lastProposalBytes), nil
}
