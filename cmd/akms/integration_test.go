// +build integration

package main

import (
	"encoding/base64"
	"fmt"
	"github.com/alpe/akms/pkg/signer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
	"net/http"
	"testing"
	"time"
)

func TestGetPubKey(t *testing.T) {
	b, _ := base64.StdEncoding.DecodeString("smAHzSepeo7g5jAFt5GudpW7fHxBdkZTRmZ/K+54xx0=")
	var exp [ed25519.PubKeyEd25519Size]byte
	copy(exp[:], b)
	pubKey := ed25519.PubKeyEd25519(exp)

	// when
	leader := leaderClient()
	got := leader.GetPubKey()
	// then

	assert.Equal(t, pubKey, got)
}

func xTestSign(t *testing.T) {
	b, _ := base64.StdEncoding.DecodeString("smAHzSepeo7g5jAFt5GudpW7fHxBdkZTRmZ/K+54xx0=")
	var exp [ed25519.PubKeyEd25519Size]byte
	copy(exp[:], b)
	pubKey := ed25519.PubKeyEd25519(exp)

	// when
	leader := leaderClient()
	proposal := types.Proposal{
		Type:      types.PrevoteType,
		Height:    time.Now().UnixNano(),
		Round:     1,
		BlockID:   types.BlockID{[]byte{1, 2, 3}, types.PartSetHeader{5, []byte{1, 2, 3}}},
		Timestamp: tmtime.Now(),
	}
	err := leader.SignProposal("test-chain", &proposal)
	// then
	require.NoError(t, err)
	assert.True(t, pubKey.VerifyBytes(proposal.SignBytes("test-chain"), proposal.Signature))
}

func BenchmarkProposal(b *testing.B) {
	leader := leaderClient()
	h := time.Now().UnixNano()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		proposal := types.Proposal{
			Type:      types.PrevoteType,
			Height:    h + int64(n),
			Round:     1,
			BlockID:   types.BlockID{[]byte{1, 2, 3}, types.PartSetHeader{5, []byte{1, 2, 3}}},
			Timestamp: tmtime.Now(),
		}
		err := leader.SignProposal("test-chain", &proposal)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func leaderClient() *signer.RestClient {
	nodeCount := 3
	c := make([]*signer.RestClient, nodeCount)
	for i := 0; i < nodeCount; i++ {
		serverAddr := fmt.Sprintf("http://localhost:808%d", i+1)
		c[i] = &signer.RestClient{
			Logger:      log.TestingLogger(),
			BaseAddress: serverAddr,
			HttpClient:  &http.Client{Timeout: 100 * time.Millisecond},
		}
	}
	for _, v := range c {
		if v.Ready() {
			return v
		}
	}
	panic("no leader found")
}
