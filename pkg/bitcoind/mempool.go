package bitcoind

import (
	"encoding/json"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// MempoolDescriptors contains cached descriptor values for collected mempool metrics
var MempoolDescriptors = []*prometheus.Desc{
	prometheus.NewDesc("bitcoind_mempool_size", "Current mempool transaction count", []string{"chain"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_mempool_bytes", "Sum of all virtual transaction sizes as defined in BIP 141. Differs from actual serialized size because witness data is discounted", []string{"chain"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_mempool_usage", "Total memory usage for the mempool", []string{"chain"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_mempool_total_fee", "Total fees for the mempool in BTC, ignoring modified fees through prioritisetransaction", []string{"chain"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_mempool_max_bytes", "Maximum memory usage for the mempool", []string{"chain"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_mempool_min_fee", "Minimum fee rate in BTC/kvB for transactions to be accepted. Is the maximum of minrelaytxfee and minimum mempool fee", []string{"chain"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_mempool_min_relay_tx_fee", "Current minimum relay fee for transactions", []string{"chain"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_mempool_incremental_relay_fee", "Minimum fee rate increment for mempool limiting or replacement in BTC/kvB", []string{"chain"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_mempool_unbroadcast_count", "Current number of transactions that haven't passed initial broadcast yet", []string{"chain"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_mempool_fullrbf", "True if the mempool accepts RBF without replaceability signaling inspection", []string{"chain"}, prometheus.Labels{}),
}

// NewMempoolCollector creates a new prometheus.Collector for getmempoolinfo properties
func NewMempoolCollector(client *rpcclient.Client, logger *zap.Logger) prometheus.Collector {
	return &MempoolCollector{client, logger}
}

// MempoolCollector builds metrics from getmempoolinfo RPC responses
type MempoolCollector struct {
	*rpcclient.Client
	*zap.Logger
}

// Describe returns the collector's metric descriptor set
func (col *MempoolCollector) Describe(out chan<- *prometheus.Desc) {
	for _, desc := range MempoolDescriptors {
		out <- desc
	}
}

// GetMempoolInfoResult unmarshals the full RPC v24.0.0 getmempoolinfo response message
type GetMempoolInfoResult struct {
	Loaded  bool `json:"loaded"`
	FullRBF bool `json:"fullrbf"`

	Size                int64   `json:"size"`
	Bytes               int64   `json:"bytes"`
	Usage               int64   `json:"usage"`
	TotalFee            float64 `json:"total_fee"`
	MaxBytes            int64   `json:"maxmempool"`
	MinFee              float64 `json:"mempoolminfee"`
	MinRelayTXFee       float64 `json:"minrelaytxfee"`
	IncrementalRelayFee float64 `json:"incrementalrelayfee"`
	UnbroadcastCount    int64   `json:"unbroadcastcount"`
}

// Collect calls the getmempoolinfo RPC and builds metrics from its response properties
func (col *MempoolCollector) Collect(out chan<- prometheus.Metric) {
	chain, err := col.GetBlockChainInfo()
	if err != nil {
		col.Error("RPC call getblockchaininfo failed", zap.Error(err))
		return
	}

	data, err := rpcclient.ReceiveFuture(col.SendCmd(&btcjson.GetMempoolInfoCmd{}))
	if err != nil {
		col.Error("RPC call getmempoolinfo failed", zap.Error(err))
		return
	}

	var info GetMempoolInfoResult
	err = json.Unmarshal(data, &info)

	if err != nil {
		col.Error("Failed to decode getmempoolinfo response", zap.Error(err))
		return
	}

	metric, _ := prometheus.NewConstMetric(MempoolDescriptors[0], prometheus.GaugeValue, float64(info.Size), chain.Chain)
	out <- metric

	metric, _ = prometheus.NewConstMetric(MempoolDescriptors[1], prometheus.GaugeValue, float64(info.Bytes), chain.Chain)
	out <- metric

	metric, _ = prometheus.NewConstMetric(MempoolDescriptors[2], prometheus.GaugeValue, float64(info.Usage), chain.Chain)
	out <- metric

	metric, _ = prometheus.NewConstMetric(MempoolDescriptors[3], prometheus.GaugeValue, info.TotalFee, chain.Chain)
	out <- metric

	metric, _ = prometheus.NewConstMetric(MempoolDescriptors[4], prometheus.GaugeValue, float64(info.MaxBytes), chain.Chain)
	out <- metric

	metric, _ = prometheus.NewConstMetric(MempoolDescriptors[5], prometheus.GaugeValue, info.MinFee, chain.Chain)
	out <- metric

	metric, _ = prometheus.NewConstMetric(MempoolDescriptors[6], prometheus.GaugeValue, info.MinRelayTXFee, chain.Chain)
	out <- metric

	metric, _ = prometheus.NewConstMetric(MempoolDescriptors[7], prometheus.GaugeValue, info.IncrementalRelayFee, chain.Chain)
	out <- metric

	metric, _ = prometheus.NewConstMetric(MempoolDescriptors[8], prometheus.GaugeValue, float64(info.UnbroadcastCount), chain.Chain)
	out <- metric

	if info.FullRBF {
		metric, _ = prometheus.NewConstMetric(MempoolDescriptors[9], prometheus.UntypedValue, 1, chain.Chain)
	} else {
		metric, _ = prometheus.NewConstMetric(MempoolDescriptors[9], prometheus.UntypedValue, 0, chain.Chain)
	}
	out <- metric
}
