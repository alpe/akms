package store

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/raft"
	"github.com/pkg/errors"
	tmcommon "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
)

const (
	retainSnapshotCount = 2
	raftTimeout         = 2 * time.Second
	transportTimeout    = 2 * time.Second
)

// raftStore is very much inspired by https://github.com/otoolep/hraftd and contains code copied from this project.
type raftStore struct {
	snapshotDir  string
	RaftBindAddr string
	m            *MemoryStore
	raft         *raft.Raft
	logger       log.Logger
	mu           sync.Mutex
	inFlight     *state
}

func NewRaftStore(snapshotDir, raftAddr string, logger log.Logger) *raftStore {
	return &raftStore{
		snapshotDir:  snapshotDir,
		RaftBindAddr: raftAddr,
		m:            &MemoryStore{},
		logger:       logger,
	}
}

func (s *raftStore) Open(nodeID string, raftServers ...raft.Server) error {
	s.logger.Info("Starting raft server", "addr", s.RaftBindAddr, "nodeID", nodeID, "cluster", raftServers)
	// Setup Raft configuration.
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(nodeID)

	// Setup Raft communication.
	addr, err := net.ResolveTCPAddr("tcp", s.RaftBindAddr)
	if err != nil {
		return err
	}
	transport, err := raft.NewTCPTransport(s.RaftBindAddr, addr, 3, transportTimeout, os.Stderr)
	if err != nil {
		return err
	}

	// Create the snapshot store. This allows the Raft to truncate the log.
	snapshots, err := raft.NewFileSnapshotStore(s.snapshotDir, retainSnapshotCount, os.Stderr)
	if err != nil {
		return errors.Errorf("file snapshot store: %s", err)
	}
	// Instantiate the Raft systems.
	ra, err := raft.NewRaft(config, s, raft.NewInmemStore(), raft.NewInmemStore(), snapshots, transport)
	if err != nil {
		return errors.Errorf("new raft: %s", err)
	}
	s.raft = ra
	if len(raftServers) == 0 {
		raftServers = []raft.Server{
			{
				ID:      config.LocalID,
				Address: transport.LocalAddr(),
			},
		}
	}

	err = ra.BootstrapCluster(raft.Configuration{raftServers}).Error()
	if err == raft.ErrCantBootstrap {
		s.logger.Info("Cluster already running")
		return s.joinCluster(raftServers)
	}
	return err
}

func (s *raftStore) joinCluster(servers []raft.Server) error {
	configFuture := s.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		return errors.Wrap(err, "failed to get raft configuration")
	}
	for _, v := range servers {
		nodeID := v.ID
		addr := v.Address
		for _, srv := range configFuture.Configuration().Servers {
			// If a node already exists with either the joining node's ID or address,
			// that node may need to be removed from the config first.
			if srv.ID == raft.ServerID(nodeID) || srv.Address == raft.ServerAddress(addr) {
				// However if *both* the ID and the address are the same, then nothing -- not even
				// a join operation -- is needed.
				if srv.Address == raft.ServerAddress(addr) && srv.ID == raft.ServerID(nodeID) {
					s.logger.Info("Node is already member of cluster, ignoring join request", "nodeID", nodeID, "addr", addr)
					return nil
				}
				future := s.raft.RemoveServer(srv.ID, 0, 0)
				if err := future.Error(); err != nil {
					return errors.Wrapf(err, "error removing existing node %s at %s", nodeID, addr)
				}
			}
		}
		f := s.raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(addr), 0, 0)
		if f.Error() != nil {
			return f.Error()
		}
		s.logger.Info("Node joined successfully", "nodeID", nodeID, "addr", addr)
	}
	return nil
}

// Leader is used to return the current leader of the cluster.
// It may return empty string if there is no current leader
// or the leader is unknown.
func (s *raftStore) LeaderAddress() string {
	return string(s.raft.Leader())
}

func (s *raftStore) IsCurrent(height int64, round int, step int8) (bool, error) {
	s.raft.Leader()
	return s.m.IsCurrent(height, round, step)
}

// Shutdown gracefully stops the raft store. blocks
func (s *raftStore) Shutdown(ctx context.Context) error {
	r := make(chan error, 0)
	go func() {
		r <- s.raft.Shutdown().Error()
		close(r)
	}()
	select {
	case <-r:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *raftStore) GetLastSignature() []byte {
	return s.m.GetLastSignature()
}
func (s *raftStore) GetLastSignBytes() tmcommon.HexBytes {
	return s.m.GetLastSignBytes()
}

func (s *raftStore) IsLeader() bool {
	return s.raft.State() == raft.Leader
}

func (s *raftStore) StoreSigned(height int64, round int, step int8, signBytes []byte, sig []byte) error {
	if !s.IsLeader() {
		return errors.New("not leader")
	}
	newState := newState(height, round, step, signBytes, sig)
	b, err := json.Marshal(newState)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.inFlight != nil {
		s.mu.Unlock()
		return errors.New("safety check: inflight data")
	}
	s.inFlight = &newState
	s.mu.Unlock()
	return s.raft.Apply(b, raftTimeout).Error()
}

// Apply applies a Raft log entry to the key-value store.
func (f *raftStore) Apply(l *raft.Log) interface{} {
	var newState state
	if err := json.Unmarshal(l.Data, &newState); err != nil {
		return errors.Wrap(err, "failed to unmarshal state")
	}
	f.logger.Debug("Applying new status", "old", f.m.GetCurrentState(), "new", newState)
	f.mu.Lock()
	if f.inFlight != nil && newState.CompareHRSTo(*f.inFlight) >= 0 {
		f.inFlight = nil
	}
	f.mu.Unlock()
	f.m.setNewState(newState)
	return nil
}

func (f *raftStore) Snapshot() (raft.FSMSnapshot, error) {
	return &fsmSnapshot{state: f.m.GetCurrentState()}, nil
}

// Restore stores the key-value store to a previous state.
func (f *raftStore) Restore(rc io.ReadCloser) error {
	var o state
	if err := json.NewDecoder(rc).Decode(&o); err != nil {
		return err
	}
	f.mu.Lock()
	f.inFlight = nil
	f.mu.Unlock()

	f.m.setNewState(o)
	return nil
}

type fsmSnapshot struct {
	state state
}

func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		// Encode data.
		b, err := json.Marshal(f.state)
		if err != nil {
			return err
		}

		// Write data to sink.
		if _, err := sink.Write(b); err != nil {
			return err
		}

		// Close the sink.
		return sink.Close()
	}()

	if err != nil {
		sink.Cancel()
	}
	return err
}

func (f *fsmSnapshot) Release() {}
