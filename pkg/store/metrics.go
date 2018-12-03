package store

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tendermint/tendermint/libs/log"
	"sync"
)

const namespace = "kms"

var (
	peersCountDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "peers_total"),
		"The number of peers connected",
		nil, nil,
	)
)

type RaftStoreCollector struct {
	logger log.Logger
	mu     sync.Mutex
	store  *raftStore
}

func (r *RaftStoreCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- peersCountDesc
}

func (r *RaftStoreCollector) Collect(ch chan<- prometheus.Metric) {
	r.mu.Lock()
	defer r.mu.Unlock()
	configuration := r.store.raft.GetConfiguration()
	if err := configuration.Error(); err != nil {
		r.logger.Error("failed to get raft config", "cause", err)
		return
	}
	ch <- prometheus.MustNewConstMetric(
		peersCountDesc, prometheus.GaugeValue, float64(len(configuration.Configuration().Servers)-1),
	)
}

func RegisterRaftStoreMetrics(reg prometheus.Registerer, store *raftStore, logger log.Logger) *RaftStoreCollector {
	s := &RaftStoreCollector{
		logger: logger,
		store:  store,
	}
	reg.MustRegister(s)
	return s
}
