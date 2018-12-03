package server_test

import (
	"github.com/alpe/akms/pkg/server"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNotLeaderable(t *testing.T) {
	h := server.WithLeaderOnly("/any", &struct{}{}, nil)
	assert.Nil(t, h)
}

type leaderStub struct {
	leader bool
}

func (l *leaderStub) IsLeader() bool {
	return l.leader
}

func TestWithLeader(t *testing.T) {
	h := server.WithLeaderOnly("/protected/", &leaderStub{true}, http.HandlerFunc(server.OKHandler))
	w := httptest.NewRecorder()

	// when
	h.ServeHTTP(w, httptest.NewRequest("GET", "/protected/any", nil))

	// then
	assert.Equal(t, 200, w.Code)
}

func TestWithoutLeader(t *testing.T) {
	h := server.WithLeaderOnly("/protected/", &leaderStub{false}, http.HandlerFunc(server.OKHandler))
	w := httptest.NewRecorder()

	// when
	h.ServeHTTP(w, httptest.NewRequest("GET", "/protected/any", nil))

	// then
	assert.Equal(t, 503, w.Code)
}

func TestPublicPathWithoutLeader(t *testing.T) {
	h := server.WithLeaderOnly("/protected/", &leaderStub{false}, http.HandlerFunc(server.OKHandler))
	w := httptest.NewRecorder()

	// when
	h.ServeHTTP(w, httptest.NewRequest("GET", "/any", nil))

	// then
	assert.Equal(t, 200, w.Code)
}
