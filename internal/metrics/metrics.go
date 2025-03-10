package metrics

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/forks-lab/go-stai-libs/pkg/rpc"
	"github.com/forks-lab/go-stai-libs/pkg/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	wrappedPrometheus "github.com/forks-lab/stai-exporter/internal/prometheus"
)

type staiService string

const (
	staiServiceFullNode  staiService = "full_node"
	staiServiceWallet    staiService = "wallet"
	staiServiceCrawler   staiService = "crawler"
	staiServiceTimelord  staiService = "timelord"
	staiServiceHarvester staiService = "harvester"
	staiServiceFarmer    staiService = "farmer"
)

// serviceMetrics defines methods that must be on all metrics services
type serviceMetrics interface {
	// InitMetrics registers any metrics (gauges, counters, etc) on creation of the metrics object
	InitMetrics()

	// InitialData is called after the websocket connection is opened to allow each service
	// to load any initial data that should be reported
	InitialData()

	// ReceiveResponse is called when a response is received for the particular metrics service
	ReceiveResponse(*types.WebsocketResponse)

	// Disconnected is called when the websocket is disconnected, to clear metrics, etc
	Disconnected()

	// Reconnected is called when the websocket is reconnected after a disconnection
	Reconnected()
}

// Metrics is the main entrypoint
type Metrics struct {
	metricsPort uint16
	client      *rpc.Client

	// httpClient is another instance of the rpc.Client in HTTP mode
	// This is used rarely, to request data in response to a websocket event that is too large to fit on a single
	// websocket connection or needs to be paginated
	httpClient *rpc.Client

	// This holds a custom prometheus registry so that only our metrics are exported, and not the default go metrics
	registry *prometheus.Registry

	// All the serviceMetrics interfaces that are registered
	serviceMetrics map[staiService]serviceMetrics
}

// NewMetrics returns a new instance of metrics
// All metrics are registered here
func NewMetrics(port uint16, logLevel log.Level) (*Metrics, error) {
	var err error

	metrics := &Metrics{
		metricsPort:    port,
		registry:       prometheus.NewRegistry(),
		serviceMetrics: map[staiService]serviceMetrics{},
	}

	log.SetLevel(logLevel)

	metrics.client, err = rpc.NewClient(rpc.ConnectionModeWebsocket)
	if err != nil {
		return nil, err
	}

	metrics.httpClient, err = rpc.NewClient(rpc.ConnectionModeHTTP)
	if err != nil {
		// For now, http client is optional
		// Sometimes this fails with outdated config.yaml files that don't have the crawler/seeder section present
		log.Errorf("Error creating http client: %s\n", err.Error())
	}

	// Register each service's metrics

	metrics.serviceMetrics[staiServiceFullNode] = &FullNodeServiceMetrics{metrics: metrics}
	metrics.serviceMetrics[staiServiceWallet] = &WalletServiceMetrics{metrics: metrics}
	metrics.serviceMetrics[staiServiceCrawler] = &CrawlerServiceMetrics{metrics: metrics}
	metrics.serviceMetrics[staiServiceTimelord] = &TimelordServiceMetrics{metrics: metrics}
	metrics.serviceMetrics[staiServiceHarvester] = &HarvesterServiceMetrics{metrics: metrics}
	metrics.serviceMetrics[staiServiceFarmer] = &FarmerServiceMetrics{metrics: metrics}

	// Init each service's metrics
	for _, service := range metrics.serviceMetrics {
		service.InitMetrics()
	}

	return metrics, nil
}

// newGauge returns a lazy gauge that follows naming conventions
func (m *Metrics) newGauge(service staiService, name string, help string) *wrappedPrometheus.LazyGauge {
	opts := prometheus.GaugeOpts{
		Namespace: "stai",
		Subsystem: string(service),
		Name:      name,
		Help:      help,
	}

	gm := prometheus.NewGauge(opts)

	lg := &wrappedPrometheus.LazyGauge{
		Gauge:    gm,
		Registry: m.registry,
	}

	return lg
}

// newGauge returns a gaugeVec that follows naming conventions and registers it with the prometheus collector
// This doesn't need a lazy wrapper, as they're inherently lazy registered for each label value provided
func (m *Metrics) newGaugeVec(service staiService, name string, help string, labels []string) *prometheus.GaugeVec {
	opts := prometheus.GaugeOpts{
		Namespace: "stai",
		Subsystem: string(service),
		Name:      name,
		Help:      help,
	}

	gm := prometheus.NewGaugeVec(opts, labels)

	m.registry.MustRegister(gm)

	return gm
}

// newGauge returns a counter that follows naming conventions and registers it with the prometheus collector
func (m *Metrics) newCounter(service staiService, name string, help string) *wrappedPrometheus.LazyCounter {
	opts := prometheus.CounterOpts{
		Namespace: "stai",
		Subsystem: string(service),
		Name:      name,
		Help:      help,
	}

	cm := prometheus.NewCounter(opts)

	lc := &wrappedPrometheus.LazyCounter{
		Counter:  cm,
		Registry: m.registry,
	}

	return lc
}

// newCounterVec returns a counter that follows naming conventions and registers it with the prometheus collector
func (m *Metrics) newCounterVec(service staiService, name string, help string, labels []string) *prometheus.CounterVec {
	opts := prometheus.CounterOpts{
		Namespace: "stai",
		Subsystem: string(service),
		Name:      name,
		Help:      help,
	}

	gm := prometheus.NewCounterVec(opts, labels)

	m.registry.MustRegister(gm)

	return gm
}

// OpenWebsocket sets up the RPC client and subscribes to relevant topics
func (m *Metrics) OpenWebsocket() error {
	err := m.client.SubscribeSelf()
	if err != nil {
		return err
	}

	err = m.client.Subscribe("metrics")
	if err != nil {
		return err
	}

	err = m.client.AddHandler(m.websocketReceive)
	if err != nil {
		return err
	}

	m.client.AddDisconnectHandler(m.disconnectHandler)
	m.client.AddReconnectHandler(m.reconnectHandler)

	for _, service := range m.serviceMetrics {
		service.InitialData()
	}

	return nil
}

// CloseWebsocket closes the websocket connection
func (m *Metrics) CloseWebsocket() error {
	// @TODO reenable once fixed in the upstream dep
	//return m.client.DaemonService.CloseConnection()
	return nil
}

// StartServer starts the metrics server
func (m *Metrics) StartServer() error {
	log.Printf("Starting metrics server on port %d", m.metricsPort)

	http.Handle("/metrics", promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{}))
	http.HandleFunc("/healthz", healthcheckEndpoint)
	return http.ListenAndServe(fmt.Sprintf(":%d", m.metricsPort), nil)
}

func (m *Metrics) websocketReceive(resp *types.WebsocketResponse, err error) {
	if err != nil {
		log.Errorf("Websocket received err: %s\n", err.Error())
		return
	}

	log.Printf("recv: %s %s\n", resp.Origin, resp.Command)
	log.Debugf("origin: %s command: %s destination: %s data: %s\n", resp.Origin, resp.Command, resp.Destination, string(resp.Data))

	switch resp.Origin {
	case "stai_full_node":
		m.serviceMetrics[staiServiceFullNode].ReceiveResponse(resp)
	case "stai_wallet":
		m.serviceMetrics[staiServiceWallet].ReceiveResponse(resp)
	case "stai_crawler":
		m.serviceMetrics[staiServiceCrawler].ReceiveResponse(resp)
	case "stai_timelord":
		m.serviceMetrics[staiServiceTimelord].ReceiveResponse(resp)
	case "stai_harvester":
		m.serviceMetrics[staiServiceHarvester].ReceiveResponse(resp)
	case "stai_farmer":
		m.serviceMetrics[staiServiceFarmer].ReceiveResponse(resp)
	}
}

func (m *Metrics) disconnectHandler() {
	log.Debug("Calling disconnect handlers")
	for _, service := range m.serviceMetrics {
		service.Disconnected()
	}
}

func (m *Metrics) reconnectHandler() {
	log.Debug("Calling reconnect handlers")
	for _, service := range m.serviceMetrics {
		service.Reconnected()
	}
}

// Healthcheck endpoint for metrics server
func healthcheckEndpoint(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := fmt.Fprintf(w, "Ok")
	if err != nil {
		log.Errorf("Error writing healthcheck response %s\n", err.Error())
	}
}
