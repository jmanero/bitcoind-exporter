package bitcoind

import (
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// BlockchainDescriptors contains cached descriptor values for collected blockchain metrics
var BlockchainDescriptors = []*prometheus.Desc{
	prometheus.NewDesc("bitcoind_blockchain_blocks", "Height of the most-work fully-validated chain", []string{"chain"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_blockchain_headers", "Current number of headers validated", []string{"chain"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_blockchain_difficulty", "Current difficulty metric", []string{"chain"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_blockchain_median_time", "Median time for the current best block", []string{"chain"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_blockchain_verification_progress", "Estimate of verification progress on range [0..1]", []string{"chain"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_initial_block_download", "Estimate of whether this node is in Initial Block Download mode", []string{"chain"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_blockchain_size_on_disk", "Estimated size of the block and undo files on disk", []string{"chain"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_blockchain_prune_height", "Height of the last block pruned, plus one", []string{"chain"}, prometheus.Labels{}),
}

// NewBlockchainCollector creates a new prometheus.Collector for getblockchaininfo properties
func NewBlockchainCollector(client *rpcclient.Client, logger *zap.Logger) prometheus.Collector {
	return &BlockchainCollector{client, logger}
}

// BlockchainCollector builds metrics from getblockchaininfo RPC responses
type BlockchainCollector struct {
	*rpcclient.Client
	*zap.Logger
}

// Describe returns the collector's metric descriptor set
func (col *BlockchainCollector) Describe(out chan<- *prometheus.Desc) {
	for _, desc := range BlockchainDescriptors {
		out <- desc
	}
}

// Collect calls the getblockchaininfo RPC and builds metrics from its response properties
func (col *BlockchainCollector) Collect(out chan<- prometheus.Metric) {
	info, err := col.GetBlockChainInfo()
	if err != nil {
		col.Error("RPC call getblockchaininfo failed", zap.Error(err))
		return
	}

	metric, _ := prometheus.NewConstMetric(BlockchainDescriptors[0], prometheus.CounterValue, float64(info.Blocks), info.Chain)
	out <- metric

	metric, _ = prometheus.NewConstMetric(BlockchainDescriptors[1], prometheus.CounterValue, float64(info.Headers), info.Chain)
	out <- metric

	metric, _ = prometheus.NewConstMetric(BlockchainDescriptors[2], prometheus.GaugeValue, float64(info.Difficulty), info.Chain)
	out <- metric

	metric, _ = prometheus.NewConstMetric(BlockchainDescriptors[3], prometheus.GaugeValue, float64(info.MedianTime), info.Chain)
	out <- metric

	metric, _ = prometheus.NewConstMetric(BlockchainDescriptors[4], prometheus.GaugeValue, info.VerificationProgress, info.Chain)
	out <- metric

	if info.InitialBlockDownload {
		metric, _ = prometheus.NewConstMetric(BlockchainDescriptors[5], prometheus.UntypedValue, 1, info.Chain)
	} else {
		metric, _ = prometheus.NewConstMetric(BlockchainDescriptors[5], prometheus.UntypedValue, 0, info.Chain)
	}
	out <- metric

	metric, _ = prometheus.NewConstMetric(BlockchainDescriptors[6], prometheus.GaugeValue, float64(info.SizeOnDisk), info.Chain)
	out <- metric

	metric, _ = prometheus.NewConstMetric(BlockchainDescriptors[7], prometheus.GaugeValue, float64(info.PruneHeight), info.Chain)
	out <- metric
}
