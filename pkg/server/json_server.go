package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/alpe/akms/pkg/signer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
	"io"
	"net/http"
	"strconv"
)

func NewJsonApiServer(port int, logger log.Logger, privVal types.PrivValidator, chainID string, gatherer prometheus.Gatherer) *http.Server {
	serverAddress := fmt.Sprintf(":%d", port)
	handler := Routes(&signer.Debugger{logger, privVal}, chainID)
	metrics := NewHttpMetrics(prometheus.DefaultRegisterer)
	handler.Handle("/metrics", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))
	handler.HandleFunc("/healthz", OKHandler)
	srv := &http.Server{
		Addr: serverAddress,
		Handler: WithPrometheus("prom", metrics,
			WithLeaderOnly("/sign/", privVal, handler),
		),
	}
	logger.Info("Http server started", "address", serverAddress)
	return srv
}

func Routes(privVal types.PrivValidator, chainID string) *http.ServeMux {
	encoder := base64.StdEncoding
	mux := http.NewServeMux()

	mux.HandleFunc("/pubkey", func(w http.ResponseWriter, r *http.Request) {
		x, ok := privVal.GetPubKey().(ed25519.PubKeyEd25519)
		if !ok {
			respondJson(w, http.StatusInternalServerError, []map[string]interface{}{{"error": "can not cast PubKeyEd25519"}})
			return
		}
		i := []map[string]interface{}{{"data": encoder.EncodeToString([]byte(x[:]))}}
		respondJson(w, 200, i)
	})
	mux.HandleFunc("/sign/ready", OKHandler)
	mux.HandleFunc("/sign/vote", func(w http.ResponseWriter, r *http.Request) {
		var vote types.Vote
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&vote); err != nil {
			respondJson(w, http.StatusBadRequest, []map[string]interface{}{{"error": err.Error()}})
			return
		}
		err := privVal.SignVote(chainID, &vote)
		if err != nil {
			respondJson(w, http.StatusBadRequest, []map[string]interface{}{{"error": err.Error()}})
			return
		}
		respondJson(w, 200, &vote)
	})
	mux.HandleFunc("/sign/proposal", func(w http.ResponseWriter, r *http.Request) {
		var proposal types.Proposal
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&proposal); err != nil {
			respondJson(w, http.StatusBadRequest, []map[string]interface{}{{"error": err.Error()}})
			return
		}
		err := privVal.SignProposal(chainID, &proposal)
		if err != nil {
			respondJson(w, http.StatusBadRequest, []map[string]interface{}{{"error": err.Error()}})
			return
		}
		respondJson(w, 200, &proposal)
	})
	return mux
}

func respondJson(w http.ResponseWriter, code int, content interface{}) error {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(content)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(code)
	_, _ = io.Copy(w, &buf)
	return nil
}
