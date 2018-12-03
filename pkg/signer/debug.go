package signer

import (
	"time"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

var _ types.PrivValidator = (*Debugger)(nil)
var _ leaderable = (*Debugger)(nil)

type Debugger struct {
	Logger log.Logger
	Next   types.PrivValidator
}

func (s *Debugger) GetPubKey() crypto.PubKey {
	s.Logger.Info("GetPubKey called")
	start := time.Now()
	defer func() {
		s.Logger.Info("GetPubKey completed", "duration", time.Since(start))
	}()
	return s.Next.GetPubKey()
}

func (s *Debugger) SignVote(chainID string, vote *types.Vote) error {
	s.Logger.Info("SignVote called", "chainID", chainID, "vote", vote)
	start := time.Now()
	defer func() {
		s.Logger.Info("SignVote completed", "duration", time.Since(start))
	}()
	return s.Next.SignVote(chainID, vote)
}

func (s *Debugger) SignProposal(chainID string, proposal *types.Proposal) error {
	s.Logger.Info("SignProposal called", "chainID", chainID, "proposal", proposal)
	start := time.Now()
	defer func() {
		s.Logger.Info("SignProposal completed", "duration", time.Since(start))
	}()
	return s.Next.SignProposal(chainID, proposal)
}

type leaderable interface {
	IsLeader() bool
}

func (s *Debugger) IsLeader() bool {
	if l, ok := s.Next.(leaderable); ok {
		return l.IsLeader()
	}
	return false
}
