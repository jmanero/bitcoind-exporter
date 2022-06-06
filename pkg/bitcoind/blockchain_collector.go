package bitcoind

import (
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// BlockchainCollector implements a prometheus.Collector for some operations on a bitcoin RPC connection
type BlockchainCollector struct {
	Client *rpcclient.Client
	*zap.Logger
}

// Describe uses DescribeByCollect, which requires that the collector return a static set of metrics
func (bc *BlockchainCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(bc, ch)
}

var descriptors = map[string]*prometheus.Desc{
	"bitcoind:blockchain:height":                prometheus.NewDesc("bitcoind:blockchain:height", "Height of the most-work fully-validated chain", []string{"chain"}, prometheus.Labels{}),
	"bitcoind:blockchain:headers":               prometheus.NewDesc("bitcoind:blockchain:headers", "Current number of headers validated", []string{"chain"}, prometheus.Labels{}),
	"bitcoind:blockchain:difficulty":            prometheus.NewDesc("bitcoind:blockchain:difficulty", "Current difficulty metric", []string{"chain"}, prometheus.Labels{}),
	"bitcoind:blockchain:median_time":           prometheus.NewDesc("bitcoind:blockchain:median_time", "Median time for the current best block", []string{"chain"}, prometheus.Labels{}),
	"bitcoind:blockchain:verification_progress": prometheus.NewDesc("bitcoind:blockchain:verification_progress", "Estimate of verification progress on range [0..1]", []string{"chain"}, prometheus.Labels{}),
	"bitcoind:blockchain:initializing":          prometheus.NewDesc("bitcoind:blockchain:initializing", "Node is in Initial Block Download mode", []string{"chain"}, prometheus.Labels{}),
	"bitcoind:blockchain:size_on_disk":          prometheus.NewDesc("bitcoind:blockchain:size_on_disk", "Estimated size of the block and undo files on disk", []string{"chain"}, prometheus.Labels{}),
	"bitcoind:blockchain:pruned":                prometheus.NewDesc("bitcoind:blockchain:pruned", "Pruning is enabled", []string{"chain"}, prometheus.Labels{}),
	"bitcoind:blockchain:prune_height":          prometheus.NewDesc("bitcoind:blockchain:prune_height", "Lowest-height complete block stored if pruning is enabled", []string{"chain"}, prometheus.Labels{}),
}

// Collect metrics from the getblockchaininfo RPC call
func (bc *BlockchainCollector) Collect(ch chan<- prometheus.Metric) {
	bc.Info("Collecting blockchain information")
	info, err := bc.Client.GetBlockChainInfo()
	if err != nil {
		bc.Error("Unable to retreive blockchain info", zap.Error(err))
		return
	}

	// Derive floats from booleans
	initializing := float64(0)
	if info.InitialBlockDownload {
		initializing = 1
	}

	pruned := float64(0)
	if info.Pruned {
		pruned = 1
	}

	var errs prometheus.MultiError

	metric, err := prometheus.NewConstMetric(descriptors["bitcoind:blockchain:height"], prometheus.CounterValue, float64(info.Blocks), info.Chain)
	if err != nil {
		errs = append(errs, err)
	} else {
		ch <- metric
	}

	metric, err = prometheus.NewConstMetric(descriptors["bitcoind:blockchain:headers"], prometheus.GaugeValue, float64(info.Headers), info.Chain)
	if err != nil {
		errs = append(errs, err)
	} else {
		ch <- metric
	}

	metric, err = prometheus.NewConstMetric(descriptors["bitcoind:blockchain:difficulty"], prometheus.GaugeValue, info.Difficulty, info.Chain)
	if err != nil {
		errs = append(errs, err)
	} else {
		ch <- metric
	}

	metric, err = prometheus.NewConstMetric(descriptors["bitcoind:blockchain:median_time"], prometheus.GaugeValue, float64(info.MedianTime), info.Chain)
	if err != nil {
		errs = append(errs, err)
	} else {
		ch <- metric
	}

	metric, err = prometheus.NewConstMetric(descriptors["bitcoind:blockchain:verification_progress"], prometheus.GaugeValue, info.VerificationProgress, info.Chain)
	if err != nil {
		errs = append(errs, err)
	} else {
		ch <- metric
	}

	metric, err = prometheus.NewConstMetric(descriptors["bitcoind:blockchain:initializing"], prometheus.UntypedValue, initializing, info.Chain)
	if err != nil {
		errs = append(errs, err)
	} else {
		ch <- metric
	}

	metric, err = prometheus.NewConstMetric(descriptors["bitcoind:blockchain:size_on_disk"], prometheus.GaugeValue, float64(info.SizeOnDisk), info.Chain)
	if err != nil {
		errs = append(errs, err)
	} else {
		ch <- metric
	}

	metric, err = prometheus.NewConstMetric(descriptors["bitcoind:blockchain:pruned"], prometheus.UntypedValue, pruned, info.Chain)
	if err != nil {
		errs = append(errs, err)
	} else {
		ch <- metric
	}

	metric, err = prometheus.NewConstMetric(descriptors["bitcoind:blockchain:prune_height"], prometheus.GaugeValue, float64(info.PruneHeight), info.Chain)
	if err != nil {
		errs = append(errs, err)
	} else {
		ch <- metric
	}

	if errs != nil {
		bc.Error("Unable to build one or more blockchain metrics", zap.Error(errs))
	}
}
