package bitcoind

// getindexinfo

import (
	"encoding/json"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// IndexDescriptors contains cached descriptor values for collected index metrics
var IndexDescriptors = []*prometheus.Desc{
	prometheus.NewDesc("bitcoind_index_synced", "Whether the index is synced or not", []string{"chain", "index"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_index_best_block_height", "Block height to which the index is synced", []string{"chain", "index"}, prometheus.Labels{}),
}

// NewIndexCollector creates a new prometheus.Collector for getindexinfo properties
func NewIndexCollector(client *rpcclient.Client, logger *zap.Logger) prometheus.Collector {
	return &IndexCollector{client, logger}
}

// IndexCollector builds metrics from getindexinfo RPC responses
type IndexCollector struct {
	*rpcclient.Client
	*zap.Logger
}

// Describe returns the collector's metric descriptor set
func (col *IndexCollector) Describe(out chan<- *prometheus.Desc) {
	for _, desc := range IndexDescriptors {
		out <- desc
	}
}

// GetIndexInfoCmd calls the getindexinfo RPC
type GetIndexInfoCmd struct {
	IndexName string `json:"index_name"`
}

func init() {
	btcjson.MustRegisterCmd("getindexinfo", (*GetIndexInfoCmd)(nil), btcjson.UsageFlag(0))
}

// GetIndexInfoResponse decodes the getindexinfo (v24.0.0) RPC response
type GetIndexInfoResponse map[string]struct {
	Synced          bool  `json:"synced"`
	BestBlockHeight int64 `json:"best_block_height"`
}

// Collect calls the getindexinfo RPC and builds metrics from its response properties
func (col *IndexCollector) Collect(out chan<- prometheus.Metric) {
	chain, err := col.GetBlockChainInfo()
	if err != nil {
		col.Error("RPC call getblockchaininfo failed", zap.Error(err))
		return
	}

	data, err := rpcclient.ReceiveFuture(col.SendCmd(&GetIndexInfoCmd{}))
	if err != nil {
		col.Error("RPC call getindexinfo failed", zap.Error(err))
		return
	}

	var info GetIndexInfoResponse
	err = json.Unmarshal(data, &info)

	if err != nil {
		col.Error("Failed to decode getindexinfo response", zap.Error(err))
		return
	}

	var metric prometheus.Metric

	for name, props := range info {
		metric, _ = prometheus.NewConstMetric(IndexDescriptors[0], prometheus.CounterValue, float64(props.BestBlockHeight), chain.Chain, name)
		out <- metric

		if props.Synced {
			metric, _ = prometheus.NewConstMetric(IndexDescriptors[1], prometheus.UntypedValue, 1, chain.Chain, name)
		} else {
			metric, _ = prometheus.NewConstMetric(IndexDescriptors[1], prometheus.UntypedValue, 1, chain.Chain, name)
		}

		out <- metric
	}
}
