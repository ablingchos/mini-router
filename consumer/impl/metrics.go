package consumer

import (
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	u_net "github.com/shirou/gopsutil/net"
	"go.uber.org/zap"
)

type Metrics struct {
	quest_number   prometheus.Counter
	fail_number    prometheus.Counter
	p_cpu          prometheus.Gauge
	p_mem          prometheus.Gauge
	p_packets_recv prometheus.Gauge
	p_packets_sent prometheus.Gauge
	p_bytes_recv   prometheus.Gauge
	p_bytes_sent   prometheus.Gauge
	p_delay        prometheus.Histogram
	mu             sync.Mutex
}

func NewMetrics(id string) *Metrics {
	return &Metrics{
		quest_number: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "quest_number_" + id,
			Help: "Total quest number",
		}),
		fail_number: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "fail_number_" + id,
			Help: "Total fail number",
		}),
		p_cpu: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "cpu_percent_" + id,
			Help: "Current CPU usage percent",
		}),
		p_mem: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "memory_mb_" + id,
			Help: "Current allocated memory in MB",
		}),
		p_packets_recv: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "packets_recv_" + id,
			Help: "Current net in bytes",
		}),
		p_packets_sent: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "packets_sent_" + id,
			Help: "Current net out bytes",
		}),
		p_bytes_recv: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "bytes_recv_" + id,
			Help: "Current net in bytes",
		}),
		p_bytes_sent: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "bytes_sent_" + id,
			Help: "Current net out bytes",
		}),
		p_delay: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "app_task_latency_seconds",
			Help:    "Task latency in seconds",
			Buckets: prometheus.LinearBuckets(0, 0.1, 100), // 10 buckets, each 100ms wide
		}),
	}
}

func (m *Metrics) incrQuestNumber() {
	m.quest_number.Add(1)
}

func (m *Metrics) incrFailNumber() {
	m.fail_number.Add(1)
}

func (m *Metrics) Start(addr string) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		mlog.Fatalf("failed to parse local address (%q): %v", addr, err)
	}
	port_i, err := strconv.Atoi(port)
	if err != nil {
		mlog.Fatalf("failed to parse port: %v", err)
	}
	port = strconv.Itoa(port_i + 1000)

	prometheus.MustRegister(m.quest_number)
	prometheus.MustRegister(m.p_cpu)
	prometheus.MustRegister(m.p_mem)
	prometheus.MustRegister(m.p_packets_recv)
	prometheus.MustRegister(m.p_packets_sent)
	prometheus.MustRegister(m.p_bytes_recv)
	prometheus.MustRegister(m.p_bytes_sent)

	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":"+port, nil)

	// 每5s更新一次
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		m.Tick()
		mlog.Debug("tick complete")
	}
}

func (m *Metrics) Tick() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	m.p_cpu.Set(getCPUPercent())
	m.p_mem.Set(float64(memStats.Alloc))

	ioCounters, err := u_net.IOCounters(false)
	if err != nil || len(ioCounters) <= 0 {
		mlog.Warn("get io counters failed", zap.Error(err))
	}

	m.p_packets_recv.Set(float64(ioCounters[0].PacketsRecv))
	m.p_packets_sent.Set(float64(ioCounters[0].PacketsSent))
	m.p_bytes_recv.Set(float64(ioCounters[0].BytesRecv))
	m.p_bytes_sent.Set(float64(ioCounters[0].BytesSent))
}

func getCPUPercent() float64 {
	pid := os.Getpid()
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "%cpu")
	output, _ := cmd.Output()
	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return 0
	}
	cpuPercent, _ := strconv.ParseFloat(strings.TrimSpace(lines[1]), 64)
	return cpuPercent
}
