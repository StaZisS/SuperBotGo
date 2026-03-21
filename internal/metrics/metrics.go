package metrics

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	PluginActionTotal    *prometheus.CounterVec
	PluginActionDuration *prometheus.HistogramVec

	HostAPITotal    *prometheus.CounterVec
	HostAPIDuration *prometheus.HistogramVec

	HTTPTriggerTotal    *prometheus.CounterVec
	HTTPTriggerDuration *prometheus.HistogramVec

	LoadedPluginsGauge prometheus.Gauge
}

func New() *Metrics {
	m := &Metrics{
		PluginActionTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "plugin_action_total",
			Help: "Total number of plugin action executions",
		}, []string{"plugin_id", "action", "status"}),

		PluginActionDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "plugin_action_duration_seconds",
			Help:    "Duration of plugin action executions",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		}, []string{"plugin_id", "action"}),

		HostAPITotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "host_api_call_total",
			Help: "Total number of host API calls from plugins",
		}, []string{"plugin_id", "function", "status"}),

		HostAPIDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "host_api_call_duration_seconds",
			Help:    "Duration of host API calls from plugins",
			Buckets: []float64{.0005, .001, .0025, .005, .01, .025, .05, .1, .25, .5, 1},
		}, []string{"plugin_id", "function"}),

		HTTPTriggerTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_trigger_requests_total",
			Help: "Total number of HTTP trigger requests",
		}, []string{"plugin_id", "method", "status_code"}),

		HTTPTriggerDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_trigger_request_duration_seconds",
			Help:    "Duration of HTTP trigger request handling",
			Buckets: prometheus.DefBuckets,
		}, []string{"plugin_id", "method"}),

		LoadedPluginsGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "plugins_loaded",
			Help: "Number of currently loaded WASM plugins",
		}),
	}

	prometheus.MustRegister(
		m.PluginActionTotal,
		m.PluginActionDuration,
		m.HostAPITotal,
		m.HostAPIDuration,
		m.HTTPTriggerTotal,
		m.HTTPTriggerDuration,
		m.LoadedPluginsGauge,
	)

	return m
}
