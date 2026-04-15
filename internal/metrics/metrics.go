package metrics

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	PluginActionTotal    *prometheus.CounterVec
	PluginActionDuration *prometheus.HistogramVec

	HostAPITotal    *prometheus.CounterVec
	HostAPIDuration *prometheus.HistogramVec

	PluginHostCallTotal    *prometheus.CounterVec
	PluginHostCallDuration *prometheus.HistogramVec

	PluginReloadTotal    *prometheus.CounterVec
	PluginReloadDuration *prometheus.HistogramVec

	HTTPTriggerTotal    *prometheus.CounterVec
	HTTPTriggerDuration *prometheus.HistogramVec

	ChannelUpdateDuration  *prometheus.HistogramVec
	CommandExecutionsTotal *prometheus.CounterVec

	MessageSendDuration     *prometheus.HistogramVec
	MessageSendRetriesTotal *prometheus.CounterVec

	DedupChecksTotal *prometheus.CounterVec

	PluginEventHandleDuration *prometheus.HistogramVec

	AuthzCheckDuration *prometheus.HistogramVec
	AuthzCacheTotal    *prometheus.CounterVec

	DialogTransitionsTotal *prometheus.CounterVec
	DialogStorageDuration  *prometheus.HistogramVec

	CronTriggerTotal *prometheus.CounterVec

	AuthzOutboxPending        prometheus.Gauge
	AuthzOutboxProcessedTotal *prometheus.CounterVec

	PubSubListenerReconnectTotal *prometheus.CounterVec

	HTTPServerRequestDuration *prometheus.HistogramVec

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

		PluginHostCallTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "plugin_host_call_total",
			Help: "Total number of host function calls per plugin, function, and status",
		}, []string{"plugin_id", "function", "status"}),

		PluginHostCallDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "plugin_host_call_duration_seconds",
			Help:    "Duration of host function calls per plugin and function",
			Buckets: []float64{.0005, .001, .0025, .005, .01, .025, .05, .1, .25, .5, 1},
		}, []string{"plugin_id", "function"}),

		PluginReloadTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "plugin_reload_total",
			Help: "Total number of plugin reload attempts",
		}, []string{"plugin_id", "status"}),

		PluginReloadDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "plugin_reload_duration_seconds",
			Help:    "Duration of plugin reload operations",
			Buckets: []float64{.01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		}, []string{"plugin_id"}),

		HTTPTriggerTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_trigger_requests_total",
			Help: "Total number of HTTP trigger requests",
		}, []string{"plugin_id", "method", "status_code"}),

		HTTPTriggerDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_trigger_request_duration_seconds",
			Help:    "Duration of HTTP trigger request handling",
			Buckets: prometheus.DefBuckets,
		}, []string{"plugin_id", "method"}),

		ChannelUpdateDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "channel_update_duration_seconds",
			Help:    "Duration of channel update processing",
			Buckets: prometheus.DefBuckets,
		}, []string{"channel", "input_type", "result"}),

		CommandExecutionsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "command_executions_total",
			Help: "Total number of command executions and command resolution outcomes",
		}, []string{"channel", "plugin_id", "command", "result"}),

		MessageSendDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "message_send_duration_seconds",
			Help:    "Duration of outbound message delivery attempts",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		}, []string{"channel", "target", "result"}),

		MessageSendRetriesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "message_send_retries_total",
			Help: "Total number of outbound message delivery retries",
		}, []string{"channel", "target"}),

		DedupChecksTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dedup_checks_total",
			Help: "Total number of update deduplication checks",
		}, []string{"channel", "result"}),

		PluginEventHandleDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "plugin_event_handle_duration_seconds",
			Help:    "Duration of plugin event handling by trigger type",
			Buckets: prometheus.DefBuckets,
		}, []string{"plugin_id", "trigger_type", "result"}),

		AuthzCheckDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "authz_check_duration_seconds",
			Help:    "Duration of authorization checks",
			Buckets: []float64{.0005, .001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2.5},
		}, []string{"plugin_id", "command", "result"}),

		AuthzCacheTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "authz_cache_total",
			Help: "Total number of authorization cache hits and misses",
		}, []string{"cache", "result"}),

		DialogTransitionsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dialog_transitions_total",
			Help: "Total number of dialog state transitions",
		}, []string{"plugin_id", "command", "event"}),

		DialogStorageDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "dialog_storage_duration_seconds",
			Help:    "Duration of dialog storage operations",
			Buckets: []float64{.0005, .001, .0025, .005, .01, .025, .05, .1, .25, .5, 1},
		}, []string{"op", "result"}),

		CronTriggerTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "cron_trigger_total",
			Help: "Total number of cron trigger executions and skips",
		}, []string{"plugin_id", "trigger", "result"}),

		AuthzOutboxPending: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "authz_outbox_pending",
			Help: "Number of pending authorization outbox entries",
		}),

		AuthzOutboxProcessedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "authz_outbox_processed_total",
			Help: "Total number of authorization outbox operations processed",
		}, []string{"operation", "result"}),

		PubSubListenerReconnectTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pubsub_listener_reconnect_total",
			Help: "Total number of pubsub listener reconnect attempts",
		}, []string{"reason"}),

		HTTPServerRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_server_request_duration_seconds",
			Help:    "Duration of HTTP server requests",
			Buckets: prometheus.DefBuckets,
		}, []string{"route", "method", "status_class"}),

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
		m.PluginHostCallTotal,
		m.PluginHostCallDuration,
		m.PluginReloadTotal,
		m.PluginReloadDuration,
		m.HTTPTriggerTotal,
		m.HTTPTriggerDuration,
		m.ChannelUpdateDuration,
		m.CommandExecutionsTotal,
		m.MessageSendDuration,
		m.MessageSendRetriesTotal,
		m.DedupChecksTotal,
		m.PluginEventHandleDuration,
		m.AuthzCheckDuration,
		m.AuthzCacheTotal,
		m.DialogTransitionsTotal,
		m.DialogStorageDuration,
		m.CronTriggerTotal,
		m.AuthzOutboxPending,
		m.AuthzOutboxProcessedTotal,
		m.PubSubListenerReconnectTotal,
		m.HTTPServerRequestDuration,
		m.LoadedPluginsGauge,
	)

	return m
}
