package metrics

import (
	"encoding/json"

	"github.com/chia-network/go-chia-libs/pkg/types"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

// Metrics that are based on Farmer RPC calls are in this file

// FarmerServiceMetrics contains all metrics related to the harvester
type FarmerServiceMetrics struct {
	// Holds a reference to the main metrics container this is a part of
	metrics *Metrics

	// Partial/Pooling Metrics
	submittedPartials   *prometheus.CounterVec
	currentDifficulty   *prometheus.GaugeVec
	pointsAckSinceStart *prometheus.GaugeVec
}

// InitMetrics sets all the metrics properties
func (s *FarmerServiceMetrics) InitMetrics() {
	// Partial/Pooling Metrics, by launcher ID
	poolLabels := []string{"launcher_id"}
	s.submittedPartials = s.metrics.newCounterVec(chiaServiceFarmer, "submitted_partials", "Number of partials submitted since the exporter was started", poolLabels)
	s.currentDifficulty = s.metrics.newGaugeVec(chiaServiceFarmer, "current_difficulty", "Current difficulty for this launcher id", poolLabels)
	s.pointsAckSinceStart = s.metrics.newGaugeVec(chiaServiceFarmer, "points_acknowledged_since_start", "Points acknowledged since start. This is calculated by chia, NOT since start of the exporter.", poolLabels)
}

// InitialData is called on startup of the metrics server, to allow seeding metrics with current/initial data
func (s *FarmerServiceMetrics) InitialData() {}

// Disconnected clears/unregisters metrics when the connection drops
func (s *FarmerServiceMetrics) Disconnected() {}

// ReceiveResponse handles crawler responses that are returned over the websocket
func (s *FarmerServiceMetrics) ReceiveResponse(resp *types.WebsocketResponse) {
	switch resp.Command {
	case "submitted_partial":
		s.SubmittedPartial(resp)
	case "proof":
		log.Printf("%+v", resp)
		// @TODO
	}
}

// SubmittedPartial handles a received submitted_partial event
func (s *FarmerServiceMetrics) SubmittedPartial(resp *types.WebsocketResponse) {
	partial := &types.EventFarmerSubmittedPartial{}
	err := json.Unmarshal(resp.Data, partial)
	if err != nil {
		log.Errorf("Error unmarshalling: %s\n", err.Error())
		return
	}

	s.submittedPartials.WithLabelValues(partial.LauncherID).Inc()
	s.currentDifficulty.WithLabelValues(partial.LauncherID).Set(float64(partial.CurrentDifficulty))
	s.pointsAckSinceStart.WithLabelValues(partial.LauncherID).Set(float64(partial.PointsAcknowledgedSinceStart))
}
