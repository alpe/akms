package signer

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
	"io/ioutil"
	"net/http"
	"sync"
)

const defaultRetryCount = 5

var _ types.PrivValidator = (*RestClient)(nil)

type RestClient struct {
	Logger        log.Logger
	HttpClient    *http.Client
	BaseAddress   string
	aMu           sync.Mutex
	cachedAddress types.Address
	pkMu          sync.Mutex
	cachedPubKey  *ed25519.PubKeyEd25519
}

func NewRestClient(logger log.Logger, client *http.Client, baseAddr string) *RestClient {
	return &RestClient{
		Logger:      logger,
		HttpClient:  client,
		BaseAddress: baseAddr,
	}
}

func (s *RestClient) Ready() bool {
	req, err := http.NewRequest("GET", s.BaseAddress+"/sign/ready", nil)
	if err != nil {
		s.Logger.Error("failed to fetch ready state", "cause", err)
		return false
	}
	resp, err := s.HttpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (s *RestClient) GetPubKey() crypto.PubKey {
	s.pkMu.Lock()
	defer s.pkMu.Unlock()
	if s.cachedPubKey != nil {
		return *s.cachedPubKey
	}

	r := func() (*http.Request, error) {
		return http.NewRequest("GET", s.BaseAddress+"/pubkey", nil)
	}
	var v []map[string]interface{}
	if err := s.doWithRetry(r, &v, defaultRetryCount); err != nil {
		s.Logger.Error("failed to request pubKey", "cause", err)
		return nil
	}
	if len(v) == 0 || v[0]["data"] == nil {
		s.Logger.Error("received unexpected response format")
		return nil
	}
	d, ok := v[0]["data"].(string)
	if !ok {
		s.Logger.Error("received unexpected response type")
		return nil
	}
	in := []byte(d)
	out := make([]byte, base64.StdEncoding.DecodedLen(len(in)))
	if _, err := base64.StdEncoding.Decode(out, in); err != nil {
		s.Logger.Error("failed to decode pubKey", "cause", err)
		return nil
	}
	var pubKey ed25519.PubKeyEd25519
	copy(pubKey[:], out)
	s.cachedPubKey = &pubKey
	return *s.cachedPubKey
}

func (s *RestClient) SignVote(chainID string, vote *types.Vote) error {
	r := func() (*http.Request, error) {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(vote); err != nil {
			return nil, err
		}
		return http.NewRequest("POST", s.BaseAddress+"/sign/vote", &buf)
	}
	if err := s.doWithRetry(r, vote, defaultRetryCount); err != nil {
		return err
	}
	return nil
}

func (s *RestClient) SignProposal(chainID string, proposal *types.Proposal) error {
	r := func() (*http.Request, error) {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(proposal); err != nil {
			return nil, err
		}
		return http.NewRequest("POST", s.BaseAddress+"/sign/proposal", &buf)
	}
	if err := s.doWithRetry(r, proposal, defaultRetryCount); err != nil {
		return err
	}
	return nil
}

func (s *RestClient) doWithRetry(reqFactory func() (*http.Request, error), respType interface{}, count int) error {
	req, err := reqFactory()
	if err != nil {
		return errors.Wrap(err, "failed to build request")
	}
	err = do(s.HttpClient, req, respType)
	if err != nil && count > 0 {
		return s.doWithRetry(reqFactory, respType, count-1)
	}
	return err
}

func do(c *http.Client, req *http.Request, v interface{}) error {
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if resp.Body != nil {
			resp.Body.Close()
		}
	}()
	if resp.StatusCode != http.StatusOK {
		body := "nil"
		if resp.Body != nil {
			b, err := ioutil.ReadAll(resp.Body)
			if err == nil {
				body = string(b)
			}
		}
		return errors.Errorf("unexpected response. code: %d , body: %s", resp.StatusCode, body)
	}
	return json.NewDecoder(resp.Body).Decode(&v)
}
