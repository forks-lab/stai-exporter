package metrics

import (
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/forks-lab/go-stai-libs/pkg/rpc"
	"github.com/forks-lab/go-stai-libs/pkg/types"
	"github.com/prometheus/client_golang/prometheus"

	wrappedPrometheus "github.com/forks-lab/stai-exporter/internal/prometheus"
	"github.com/forks-lab/stai-exporter/internal/utils"
)

// Metrics that are based on Wallet RPC calls are in this file

// WalletServiceMetrics contains all metrics related to the wallet
type WalletServiceMetrics struct {
	// Holds a reference to the main metrics container this is a part of
	metrics *Metrics

	// WalletBalanceMetrics
	walletSynced            *wrappedPrometheus.LazyGauge
	confirmedBalance        *prometheus.GaugeVec
	spendableBalance        *prometheus.GaugeVec
	maxSendAmount           *prometheus.GaugeVec
	pendingCoinRemovalCount *prometheus.GaugeVec
	unspentCoinCount        *prometheus.GaugeVec
}

// InitMetrics sets all the metrics properties
func (s *WalletServiceMetrics) InitMetrics() {
	// Wallet Metrics
	s.walletSynced = s.metrics.newGauge(staiServiceWallet, "synced", "")
	walletLabels := []string{"fingerprint", "wallet_id", "wallet_type", "asset_id"}
	s.confirmedBalance = s.metrics.newGaugeVec(staiServiceWallet, "confirmed_balance", "", walletLabels)
	s.spendableBalance = s.metrics.newGaugeVec(staiServiceWallet, "spendable_balance", "", walletLabels)
	s.maxSendAmount = s.metrics.newGaugeVec(staiServiceWallet, "max_send_amount", "", walletLabels)
	s.pendingCoinRemovalCount = s.metrics.newGaugeVec(staiServiceWallet, "pending_coin_removal_count", "", walletLabels)
	s.unspentCoinCount = s.metrics.newGaugeVec(staiServiceWallet, "unspent_coin_count", "", walletLabels)
}

// InitialData is called on startup of the metrics server, to allow seeding metrics with
// current/initial data
func (s *WalletServiceMetrics) InitialData() {
	utils.LogErr(s.metrics.client.WalletService.GetWallets())
	utils.LogErr(s.metrics.client.WalletService.GetSyncStatus())
}

// Disconnected clears/unregisters metrics when the connection drops
func (s *WalletServiceMetrics) Disconnected() {
	s.walletSynced.Unregister()
	s.confirmedBalance.Reset()
	s.spendableBalance.Reset()
	s.maxSendAmount.Reset()
	s.pendingCoinRemovalCount.Reset()
	s.unspentCoinCount.Reset()
}

// Reconnected is called when the service is reconnected after the websocket was disconnected
func (s *WalletServiceMetrics) Reconnected() {
	s.InitialData()
}

// ReceiveResponse handles wallet responses that are returned over the websocket
func (s *WalletServiceMetrics) ReceiveResponse(resp *types.WebsocketResponse) {
	switch resp.Command {
	case "coin_added":
		s.CoinAdded(resp)
	case "sync_changed":
		s.SyncChanged(resp)
	case "get_sync_status":
		s.GetSyncStatus(resp)
	case "get_wallet_balance":
		s.GetWalletBalance(resp)
	case "get_wallets":
		s.GetWallets(resp)
	}
}

// CoinAdded handles coin_added events by asking for wallet balance details
func (s *WalletServiceMetrics) CoinAdded(resp *types.WebsocketResponse) {
	coinAdded := &types.CoinAddedEvent{}
	err := json.Unmarshal(resp.Data, coinAdded)
	if err != nil {
		log.Errorf("Error unmarshalling: %s\n", err.Error())
		return
	}

	utils.LogErr(s.metrics.client.WalletService.GetWalletBalance(&rpc.GetWalletBalanceOptions{WalletID: coinAdded.WalletID}))
	utils.LogErr(s.metrics.client.WalletService.GetSyncStatus())
}

// SyncChanged handles the sync_changed event from the websocket
func (s *WalletServiceMetrics) SyncChanged(resp *types.WebsocketResponse) {
	// @TODO probably should throttle this call in case we're in a longer sync
	utils.LogErr(s.metrics.client.WalletService.GetSyncStatus())
}

// GetSyncStatus sync status for the wallet
func (s *WalletServiceMetrics) GetSyncStatus(resp *types.WebsocketResponse) {
	syncStatusResponse := &rpc.GetWalletSyncStatusResponse{}
	err := json.Unmarshal(resp.Data, syncStatusResponse)
	if err != nil {
		log.Errorf("Error unmarshalling: %s\n", err.Error())
		return
	}

	if syncStatusResponse.Synced {
		s.walletSynced.Set(1)
	} else {
		s.walletSynced.Set(0)
	}
}

// GetWalletBalance updates wallet balance metrics in response to balance changes
func (s *WalletServiceMetrics) GetWalletBalance(resp *types.WebsocketResponse) {
	walletBalance := &rpc.GetWalletBalanceResponse{}
	err := json.Unmarshal(resp.Data, walletBalance)
	if err != nil {
		log.Errorf("Error unmarshalling: %s\n", err.Error())
		return
	}

	if walletBalance.Balance != nil {
		fingerprint := fmt.Sprintf("%d", walletBalance.Balance.Fingerprint)
		walletID := fmt.Sprintf("%d", walletBalance.Balance.WalletID)
		walletType := ""
		if walletBalance.Balance.WalletType != nil {
			walletType = fmt.Sprintf("%d", *walletBalance.Balance.WalletType)
		}
		assetID := walletBalance.Balance.AssetID

		if walletBalance.Balance.ConfirmedWalletBalance.FitsInUint64() {
			s.confirmedBalance.WithLabelValues(fingerprint, walletID, walletType, assetID).Set(float64(walletBalance.Balance.ConfirmedWalletBalance.Uint64()))
		}

		if walletBalance.Balance.SpendableBalance.FitsInUint64() {
			s.spendableBalance.WithLabelValues(fingerprint, walletID, walletType, assetID).Set(float64(walletBalance.Balance.SpendableBalance.Uint64()))
		}

		s.maxSendAmount.WithLabelValues(fingerprint, walletID, walletType, assetID).Set(float64(walletBalance.Balance.MaxSendAmount))
		s.pendingCoinRemovalCount.WithLabelValues(fingerprint, walletID, walletType, assetID).Set(float64(walletBalance.Balance.PendingCoinRemovalCount))
		s.unspentCoinCount.WithLabelValues(fingerprint, walletID, walletType, assetID).Set(float64(walletBalance.Balance.UnspentCoinCount))
	}
}

// GetWallets handles a response for get_wallets and asks for the balance of each wallet
func (s *WalletServiceMetrics) GetWallets(resp *types.WebsocketResponse) {
	wallets := &rpc.GetWalletsResponse{}
	err := json.Unmarshal(resp.Data, wallets)
	if err != nil {
		log.Errorf("Error unmarshalling: %s\n", err.Error())
		return
	}

	for _, wallet := range wallets.Wallets {
		utils.LogErr(s.metrics.client.WalletService.GetWalletBalance(&rpc.GetWalletBalanceOptions{WalletID: wallet.ID}))
	}
}
