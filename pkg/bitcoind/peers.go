package bitcoind

import (
	"encoding/json"
	"strconv"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// PeersDescriptors contains cached descriptor values for collected peer metrics
var PeersDescriptors = []*prometheus.Desc{
	prometheus.NewDesc("bitcoind_peer_last_send", "UNIX epoch time of the last message sent to the peer", []string{"chain", "peer_id", "peer_addr", "peer_transport", "peer_version"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_peer_last_recv", "UNIX epoch time of the last message received from the peer", []string{"chain", "peer_id", "peer_addr", "peer_transport", "peer_version"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_peer_last_transaction", "UNIX epoch time of the last valid transaction received from the peer", []string{"chain", "peer_id", "peer_addr", "peer_transport", "peer_version"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_peer_last_block", "UNIX epoch time of the last block received from the peer", []string{"chain", "peer_id", "peer_addr", "peer_transport", "peer_version"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_peer_bytes_sent", "Total bytes sent to the peer", []string{"chain", "peer_id", "peer_addr", "peer_transport", "peer_version"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_peer_bytes_recv", "Total bytes received from the peer", []string{"chain", "peer_id", "peer_addr", "peer_transport", "peer_version"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_peer_time_offset", "Time offset in seconds from the peer", []string{"chain", "peer_id", "peer_addr", "peer_transport", "peer_version"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_peer_ping_time", "Ping time to the peer", []string{"chain", "peer_id", "peer_addr", "peer_transport", "peer_version"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_peer_ping_min", "Minimum observed ping time to the peer", []string{"chain", "peer_id", "peer_addr", "peer_transport", "peer_version"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_peer_starting_height", "Starting height (block) of the peer", []string{"chain", "peer_id", "peer_addr", "peer_transport", "peer_version"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_peer_presynced_headers", "Current height of header pre-synchronization with this peer, or -1 if no low-work sync is in progress", []string{"chain", "peer_id", "peer_addr", "peer_transport", "peer_version"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_peer_synced_headers", "Last header we have in common with the peer", []string{"chain", "peer_id", "peer_addr", "peer_transport", "peer_version"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_peer_synced_blocks", "Last block we have in common with the peer", []string{"chain", "peer_id", "peer_addr", "peer_transport", "peer_version"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_peer_addr_processed", "Total number of addresses processed, excluding those dropped due to rate limiting", []string{"chain", "peer_id", "peer_addr", "peer_transport", "peer_version"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_peer_addr_rate_limited", "Total number number of addresses dropped due to rate limiting", []string{"chain", "peer_id", "peer_addr", "peer_transport", "peer_version"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_peer_bytes_sent_per_msg", "Total bytes sent to the peer aggregated by message type", []string{"chain", "peer_id", "peer_addr", "peer_transport", "peer_version", "msg_type"}, prometheus.Labels{}),
	prometheus.NewDesc("bitcoind_peer_bytes_recv_per_msg", "Total bytes received from the peer aggregated by message type", []string{"chain", "peer_id", "peer_addr", "peer_transport", "peer_version", "msg_type"}, prometheus.Labels{}),
}

// NewPeersCollector creates a new prometheus.Collector for getpeerinfo properties
func NewPeersCollector(client *rpcclient.Client, logger *zap.Logger) prometheus.Collector {
	return &PeersCollector{client, logger}
}

// PeersCollector builds metrics from getpeerinfo RPC responses
type PeersCollector struct {
	*rpcclient.Client
	*zap.Logger
}

// Describe returns the collector's metric descriptor set
func (col *PeersCollector) Describe(out chan<- *prometheus.Desc) {
	for _, desc := range PeersDescriptors {
		out <- desc
	}
}

// GetPeerInfoResult extends btcjson.GetPeerInfoResult with more fields for RPC v24.0.0
type GetPeerInfoResult struct {
	btcjson.GetPeerInfoResult

	Network string `json:"network"`

	LastTransaction int64 `json:"last_transaction"`
	LastBlock       int64 `json:"last_block"`

	PingMin float64 `json:"minping"`

	PreSyncedHeaders int64 `json:"presynced_headers"`
	SyncedHeaders    int64 `json:"synced_headers"`
	SyncedBlocks     int64 `json:"synced_blocks"`

	AddrProcessed   int64 `json:"addr_processed"`
	AddrRateLimited int64 `json:"addr_rate_limited"`

	MinFeeFilter float64 `json:"minfeefilter"`

	BytesRecvPerMessage map[string]int64 `json:"bytesrecv_per_msg"`
	BytesSentPerMessage map[string]int64 `json:"bytessent_per_msg"`
}

// Collect calls the getpeerinfo RPC and builds metrics from its response properties
func (col *PeersCollector) Collect(out chan<- prometheus.Metric) {
	chain, err := col.GetBlockChainInfo()
	if err != nil {
		col.Error("RPC call getblockchaininfo failed", zap.Error(err))
		return
	}

	data, err := rpcclient.ReceiveFuture(col.SendCmd(&btcjson.GetPeerInfoCmd{}))
	if err != nil {
		col.Error("RPC call getpeerinfo failed", zap.Error(err))
		return
	}

	var info []GetPeerInfoResult
	err = json.Unmarshal(data, &info)

	if err != nil {
		col.Error("Failed to decode getpeerinfo response", zap.Error(err))
		return
	}

	for _, peer := range info {
		peerID := strconv.FormatInt(int64(peer.ID), 16)

		metric, _ := prometheus.NewConstMetric(PeersDescriptors[0], prometheus.GaugeValue, float64(peer.LastSend), chain.Chain, peerID, peer.Addr, peer.Network, peer.SubVer)
		out <- metric

		metric, _ = prometheus.NewConstMetric(PeersDescriptors[1], prometheus.GaugeValue, float64(peer.LastRecv), chain.Chain, peerID, peer.Addr, peer.Network, peer.SubVer)
		out <- metric

		metric, _ = prometheus.NewConstMetric(PeersDescriptors[2], prometheus.GaugeValue, float64(peer.LastTransaction), chain.Chain, peerID, peer.Addr, peer.Network, peer.SubVer)
		out <- metric

		metric, _ = prometheus.NewConstMetric(PeersDescriptors[3], prometheus.GaugeValue, float64(peer.LastBlock), chain.Chain, peerID, peer.Addr, peer.Network, peer.SubVer)
		out <- metric

		metric, _ = prometheus.NewConstMetric(PeersDescriptors[4], prometheus.GaugeValue, float64(peer.BytesSent), chain.Chain, peerID, peer.Addr, peer.Network, peer.SubVer)
		out <- metric

		metric, _ = prometheus.NewConstMetric(PeersDescriptors[5], prometheus.GaugeValue, float64(peer.BytesRecv), chain.Chain, peerID, peer.Addr, peer.Network, peer.SubVer)
		out <- metric

		metric, _ = prometheus.NewConstMetric(PeersDescriptors[6], prometheus.GaugeValue, float64(peer.TimeOffset), chain.Chain, peerID, peer.Addr, peer.Network, peer.SubVer)
		out <- metric

		metric, _ = prometheus.NewConstMetric(PeersDescriptors[7], prometheus.GaugeValue, float64(peer.PingTime), chain.Chain, peerID, peer.Addr, peer.Network, peer.SubVer)
		out <- metric

		metric, _ = prometheus.NewConstMetric(PeersDescriptors[8], prometheus.GaugeValue, float64(peer.PingMin), chain.Chain, peerID, peer.Addr, peer.Network, peer.SubVer)
		out <- metric

		metric, _ = prometheus.NewConstMetric(PeersDescriptors[9], prometheus.GaugeValue, float64(peer.StartingHeight), chain.Chain, peerID, peer.Addr, peer.Network, peer.SubVer)
		out <- metric

		metric, _ = prometheus.NewConstMetric(PeersDescriptors[10], prometheus.CounterValue, float64(peer.PreSyncedHeaders), chain.Chain, peerID, peer.Addr, peer.Network, peer.SubVer)
		out <- metric

		metric, _ = prometheus.NewConstMetric(PeersDescriptors[11], prometheus.CounterValue, float64(peer.SyncedHeaders), chain.Chain, peerID, peer.Addr, peer.Network, peer.SubVer)
		out <- metric

		metric, _ = prometheus.NewConstMetric(PeersDescriptors[12], prometheus.CounterValue, float64(peer.SyncedBlocks), chain.Chain, peerID, peer.Addr, peer.Network, peer.SubVer)
		out <- metric

		metric, _ = prometheus.NewConstMetric(PeersDescriptors[13], prometheus.CounterValue, float64(peer.AddrProcessed), chain.Chain, peerID, peer.Addr, peer.Network, peer.SubVer)
		out <- metric

		metric, _ = prometheus.NewConstMetric(PeersDescriptors[14], prometheus.CounterValue, float64(peer.AddrRateLimited), chain.Chain, peerID, peer.Addr, peer.Network, peer.SubVer)
		out <- metric

		for msg, count := range peer.BytesSentPerMessage {
			metric, _ = prometheus.NewConstMetric(PeersDescriptors[15], prometheus.CounterValue, float64(count), chain.Chain, peerID, peer.Addr, peer.Network, peer.SubVer, msg)
			out <- metric
		}

		for msg, count := range peer.BytesRecvPerMessage {
			metric, _ = prometheus.NewConstMetric(PeersDescriptors[16], prometheus.CounterValue, float64(count), chain.Chain, peerID, peer.Addr, peer.Network, peer.SubVer, msg)
			out <- metric
		}
	}
}
