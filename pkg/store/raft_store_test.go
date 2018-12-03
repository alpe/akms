package store

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
)

func TestRaftSingleInstanceStoreUpdate(t *testing.T) {
	tmpDir, _ := ioutil.TempDir("", "store_test")
	defer os.RemoveAll(tmpDir)

	s := NewRaftStore(tmpDir, "127.0.0.1:0", log.TestingLogger())
	if err := s.Open("node0"); err != nil {
		t.Fatalf("failed to open store: %s", err)
	}
	for i := 0; i < 20 && !s.IsLeader(); i++ {
		time.Sleep(100 * time.Millisecond)
	}

	// when
	err := s.StoreSigned(1, 2, 3, []byte("signedContent"), []byte("signature"))

	// then
	require.NoError(t, err)
	exp := state{1, 2, 3, []byte("signedContent"), []byte("signature")}
	var got state
	for i := 0; i < 10; i++ {
		got = s.m.GetCurrentState()
		if reflect.DeepEqual(exp, got) {
			return
		}
		time.Sleep(100 * time.Millisecond) // Wait for committed log entry to be applied.
	}
	t.Errorf("exected:\n%#v\n but got:\n%#v", exp, got)
}

func TestRaftClusterStoreUpdate(t *testing.T) {
	const nodeCount = 3
	nodes := make([]*raftStore, nodeCount)
	svr := []raft.Server{
		{ID: raft.ServerID("node0"), Address: "127.0.0.1:9090"},
		{ID: raft.ServerID("node1"), Address: "127.0.0.1:9091"},
		{ID: raft.ServerID("node2"), Address: "127.0.0.1:9092"},
	}
	for i := 0; i < nodeCount; i++ {
		tmpDir, _ := ioutil.TempDir("", "store_test")
		defer os.RemoveAll(tmpDir)

		s := NewRaftStore(tmpDir, string(svr[i].Address), log.TestingLogger())
		if err := s.Open(string(svr[i].ID), svr...); err != nil {
			t.Fatalf("failed to open store: %s", err)
		}
		nodes[i] = s
	}
	var leader *raftStore
OUTER:
	for i := 0; i < 20 && leader == nil; i++ {
		for _, v := range nodes {
			if v.IsLeader() {
				leader = v
				break OUTER
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	// when
	err := leader.StoreSigned(1, 2, 3, []byte("signedContent"), []byte("signature"))

	// then
	require.NoError(t, err)
	exp := state{1, 2, 3, []byte("signedContent"), []byte("signature")}
	for _, v := range nodes {
		var got state
		var found bool
	OUTER2:
		for i := 0; i < 10; i++ {
			got = v.m.GetCurrentState()
			if reflect.DeepEqual(exp, got) {
				found = true
				continue OUTER2
			}
			time.Sleep(100 * time.Millisecond) // Wait for committed log entry to be applied.
		}
		if !found {
			t.Errorf("exected:\n%#v\n but got:\n%#v", exp, got)
		}
	}
}
