package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	defaultNodeName = "ppu-worker-mock"
	defaultPort     = 8080
	defaultGPUCount = 16
)

type Config struct {
	NodeName      string
	NodePoolId    string
	PodSource     string
	Port          int
	GPUCount      int
	DriverVersion string
}

type PPUExporter struct {
	config *Config

	// DCGM Custom metrics
	allocateModeGauge         prometheus.Gauge
	devFBAllocatedGauge       *prometheus.GaugeVec
	devFBTotalGauge           *prometheus.GaugeVec
	illegalProcessDecodeUtil  *prometheus.GaugeVec
	illegalProcessEncodeUtil  *prometheus.GaugeVec
	illegalProcessMemCopyUtil *prometheus.GaugeVec
	illegalProcessMemUsed     *prometheus.GaugeVec
	illegalProcessSMUtil      *prometheus.GaugeVec

	// DCGM FI metrics
	devAppMemClockGauge     *prometheus.GaugeVec
	devAppSMClockGauge      *prometheus.GaugeVec
	devBAR1TotalGauge       *prometheus.GaugeVec
	devBAR1UsedGauge        *prometheus.GaugeVec
	devClockThrottleReasons *prometheus.GaugeVec
	devCountGauge           prometheus.Gauge
	devDecUtilGauge         *prometheus.GaugeVec
	devEncUtilGauge         *prometheus.GaugeVec
	devFBFreeGauge          *prometheus.GaugeVec
	devFBUsedGauge          *prometheus.GaugeVec
	devGPUTempGauge         *prometheus.GaugeVec
	devGPUUtilGauge         *prometheus.GaugeVec
	devMemoryTempGauge      *prometheus.GaugeVec
	devMemClockGauge        *prometheus.GaugeVec
	devMemCopyUtilGauge     *prometheus.GaugeVec
	devNVLinkBandwidthTotal *prometheus.CounterVec
	devPowerUsageGauge      *prometheus.GaugeVec
	devRetiredDBE           *prometheus.CounterVec
	devRetiredPending       *prometheus.CounterVec
	devRetiredSBE           *prometheus.CounterVec
	devSMClockGauge         *prometheus.GaugeVec
	devVideoClockGauge      *prometheus.GaugeVec
	devXIDErrorsGauge       *prometheus.GaugeVec

	// DCGM FI PROF metrics
	profDRAMActiveGauge *prometheus.GaugeVec
	profNVLinkRXBytes   *prometheus.CounterVec
	profNVLinkTXBytes   *prometheus.CounterVec
	profPCIeRXBytes     *prometheus.GaugeVec
	profPCIeTXBytes     *prometheus.GaugeVec
}

func NewPPUExporter(config *Config) *PPUExporter {
	deviceLabels := []string{"Hostname", "NodeName", "NodePoolId", "PodSource", "UUID", "device", "gpu", "modelName"}
	customDeviceLabels := []string{"DriverVersion", "NodeName", "NodePoolId", "PodSource", "SupportDCGM", "UUID", "device", "gpu", "modelName"}
	illegalProcessLabels := []string{"AllocateMode", "ContainerName", "NamespaceName", "NodeName", "NodePoolId", "PodName", "PodSource", "ProcessId", "ProcessName", "ProcessType", "UUID", "device", "gpu", "modelName"}

	return &PPUExporter{
		config: config,

		// DCGM Custom metrics
		allocateModeGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "DCGM_CUSTOM_ALLOCATE_MODE",
			Help: "GPU allocate mode of node,value in [None:0,Exclusive:1,Share:2]",
		}),
		devFBAllocatedGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_CUSTOM_DEV_FB_ALLOCATED",
			Help: "Allocated framebuffer memory ratio(0~1) of device,it is a custom metric created by ack",
		}, customDeviceLabels),
		devFBTotalGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_CUSTOM_DEV_FB_TOTAL",
			Help: "Total framebuffer memory of device(in MiB),it is a custom metric created by ack",
		}, customDeviceLabels),
		illegalProcessDecodeUtil: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_CUSTOM_ILLEGAL_PROCESS_DECODE_UTIL",
			Help: "Decode utilization of illegal gpu process(container request gpus with NVIDIA_VISIBLE_DEVICES=all),it is a custom metric defined by ACK",
		}, illegalProcessLabels),
		illegalProcessEncodeUtil: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_CUSTOM_ILLEGAL_PROCESS_ENCODE_UTIL",
			Help: "Encode utilization of illegal gpu process(container request gpus with NVIDIA_VISIBLE_DEVICES=all),it is a custom metric defined by ACK",
		}, illegalProcessLabels),
		illegalProcessMemCopyUtil: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_CUSTOM_ILLEGAL_PROCESS_MEM_COPY_UTIL",
			Help: "Memory copy utilization of illegal gpu process(container request gpus with NVIDIA_VISIBLE_DEVICES=all),it is a custom metric defined by ACK",
		}, illegalProcessLabels),
		illegalProcessMemUsed: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_CUSTOM_ILLEGAL_PROCESS_MEM_USED",
			Help: "Used memory(in MiB) of illegal gpu process(container request gpus with NVIDIA_VISIBLE_DEVICES=all),it is a custom metric defined by ACK",
		}, illegalProcessLabels),
		illegalProcessSMUtil: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_CUSTOM_ILLEGAL_PROCESS_SM_UTIL",
			Help: "SM utilization of illegal gpu process(container request gpus with NVIDIA_VISIBLE_DEVICES=all),it is a custom metric defined by ACK",
		}, illegalProcessLabels),

		// DCGM FI metrics
		devAppMemClockGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_FI_DEV_APP_MEM_CLOCK",
			Help: "Memory Application clocks(in MHz).",
		}, deviceLabels),
		devAppSMClockGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_FI_DEV_APP_SM_CLOCK",
			Help: "SM Application clocks (in MHz).",
		}, deviceLabels),
		devBAR1TotalGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_FI_DEV_BAR1_TOTAL",
			Help: "Total BAR1 of the GPU in MB",
		}, deviceLabels),
		devBAR1UsedGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_FI_DEV_BAR1_USED",
			Help: "Used BAR1 of the GPU in MB",
		}, deviceLabels),
		devClockThrottleReasons: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_FI_DEV_CLOCK_THROTTLE_REASONS",
			Help: "A bitmap of why the clock is throttled.",
		}, deviceLabels),
		devCountGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "DCGM_FI_DEV_COUNT",
			Help: "total devices on the node.",
		}),
		devDecUtilGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_FI_DEV_DEC_UTIL",
			Help: "Decoder utilization (in %).",
		}, deviceLabels),
		devEncUtilGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_FI_DEV_ENC_UTIL",
			Help: "Encoder utilization (in %).",
		}, deviceLabels),
		devFBFreeGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_FI_DEV_FB_FREE",
			Help: "Framebuffer memory free (in MiB).",
		}, deviceLabels),
		devFBUsedGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_FI_DEV_FB_USED",
			Help: "Framebuffer memory used (in MiB).",
		}, deviceLabels),
		devGPUTempGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_FI_DEV_GPU_TEMP",
			Help: "GPU temperature (in C).",
		}, deviceLabels),
		devGPUUtilGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_FI_DEV_GPU_UTIL",
			Help: "GPU utilization (in %).",
		}, deviceLabels),
		devMemoryTempGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_FI_DEV_MEMORY_TEMP",
			Help: "Memory temperature (in C).",
		}, deviceLabels),
		devMemClockGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_FI_DEV_MEM_CLOCK",
			Help: "Memory clock frequency (in MHz).",
		}, deviceLabels),
		devMemCopyUtilGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_FI_DEV_MEM_COPY_UTIL",
			Help: "Memory utilization (in %).",
		}, deviceLabels),
		devNVLinkBandwidthTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "DCGM_FI_DEV_NVLINK_BANDWIDTH_TOTAL",
			Help: "Total number of NVLink bandwidth counters for all lanes.",
		}, deviceLabels),
		devPowerUsageGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_FI_DEV_POWER_USAGE",
			Help: "Power draw (in W).",
		}, deviceLabels),
		devRetiredDBE: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "DCGM_FI_DEV_RETIRED_DBE",
			Help: "Total number of retired pages due to double-bit errors.",
		}, deviceLabels),
		devRetiredPending: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "DCGM_FI_DEV_RETIRED_PENDING",
			Help: "Total number of pages pending retirement.",
		}, deviceLabels),
		devRetiredSBE: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "DCGM_FI_DEV_RETIRED_SBE",
			Help: "Total number of retired pages due to single-bit errors.",
		}, deviceLabels),
		devSMClockGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_FI_DEV_SM_CLOCK",
			Help: "SM clock frequency (in MHz).",
		}, deviceLabels),
		devVideoClockGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_FI_DEV_VIDEO_CLOCK",
			Help: "Video encoder/decoder clock for the device.",
		}, deviceLabels),
		devXIDErrorsGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_FI_DEV_XID_ERRORS",
			Help: "Value of the last XID error encountered.",
		}, deviceLabels),

		// DCGM FI PROF metrics
		profDRAMActiveGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_FI_PROF_DRAM_ACTIVE",
			Help: "Ratio of cycles the device memory interface is active sending or receiving data (in %).",
		}, deviceLabels),
		profNVLinkRXBytes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "DCGM_FI_PROF_NVLINK_RX_BYTES",
			Help: "The number of bytes of active NvLink rx (receive) data including both header and payload.",
		}, deviceLabels),
		profNVLinkTXBytes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "DCGM_FI_PROF_NVLINK_TX_BYTES",
			Help: "The number of bytes of active NvLink tx (transmit) data including both header and payload.",
		}, deviceLabels),
		profPCIeRXBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_FI_PROF_PCIE_RX_BYTES",
			Help: "The rate of data received over the PCIe bus - including both protocol headers and data payloads - in bytes per second.",
		}, deviceLabels),
		profPCIeTXBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "DCGM_FI_PROF_PCIE_TX_BYTES",
			Help: "The rate of data transmitted over the PCIe bus - including both protocol headers and data payloads - in bytes per second.",
		}, deviceLabels),
	}
}

func (e *PPUExporter) Register(registry *prometheus.Registry) {
	registry.MustRegister(
		e.allocateModeGauge,
		e.devFBAllocatedGauge,
		e.devFBTotalGauge,
		e.illegalProcessDecodeUtil,
		e.illegalProcessEncodeUtil,
		e.illegalProcessMemCopyUtil,
		e.illegalProcessMemUsed,
		e.illegalProcessSMUtil,
		e.devAppMemClockGauge,
		e.devAppSMClockGauge,
		e.devBAR1TotalGauge,
		e.devBAR1UsedGauge,
		e.devClockThrottleReasons,
		e.devCountGauge,
		e.devDecUtilGauge,
		e.devEncUtilGauge,
		e.devFBFreeGauge,
		e.devFBUsedGauge,
		e.devGPUTempGauge,
		e.devGPUUtilGauge,
		e.devMemoryTempGauge,
		e.devMemClockGauge,
		e.devMemCopyUtilGauge,
		e.devNVLinkBandwidthTotal,
		e.devPowerUsageGauge,
		e.devRetiredDBE,
		e.devRetiredPending,
		e.devRetiredSBE,
		e.devSMClockGauge,
		e.devVideoClockGauge,
		e.devXIDErrorsGauge,
		e.profDRAMActiveGauge,
		e.profNVLinkRXBytes,
		e.profNVLinkTXBytes,
		e.profPCIeRXBytes,
		e.profPCIeTXBytes,
	)
}

func (e *PPUExporter) generateUUID(gpuID int) string {
	// Generate realistic UUID based on GPU ID
	prefixes := []string{
		"GPU-019e0219-0331-020a-0000-0000608e8e2e",
		"GPU-019e0225-c611-0110-0000-0000c0663c0e",
		"GPU-019e120d-8850-032c-0000-0000406a3958",
		"GPU-019e120d-8930-0516-0000-000040b6030b",
		"GPU-019e1211-40c0-0624-0000-000060f3f056",
		"GPU-019e1211-4120-0524-0000-0000c09f426b",
		"GPU-019e1215-0231-0014-0000-000060512b5e",
		"GPU-019e1215-0241-0820-0000-0000a0087936",
		"GPU-019e1215-0281-0210-0000-0000a0d60a51",
		"GPU-019e1215-c280-0416-0000-0000407aa063",
		"GPU-019e1215-c2a0-0226-0000-0000c0c6fa0a",
		"GPU-019e4201-0591-0330-0000-000060416e2b",
		"GPU-019e4201-8920-0430-0000-0000605abd70",
		"GPU-019e4201-8920-0614-0000-0000603e9c39",
		"GPU-019e4201-8930-0014-0000-000020029626",
		"GPU-019ec20c-49c2-0224-0000-0000e02b8d24",
	}
	if gpuID < len(prefixes) {
		return prefixes[gpuID]
	}
	return fmt.Sprintf("GPU-019e%04d-%04d-%04d-0000-0000%08x", gpuID, rand.Intn(10000), rand.Intn(10000), rand.Intn(0xFFFFFFFF))
}

func (e *PPUExporter) UpdateMetrics() {
	// Set allocate mode (typically 0 for none)
	e.allocateModeGauge.Set(0)

	// Set device count
	e.devCountGauge.Set(float64(e.config.GPUCount))

	for i := 0; i < e.config.GPUCount; i++ {
		gpuID := strconv.Itoa(i)
		deviceName := fmt.Sprintf("nvidia%d", i)
		uuid := e.generateUUID(i)

		deviceLabels := prometheus.Labels{
			"Hostname":   e.config.NodeName,
			"NodeName":   e.config.NodeName,
			"NodePoolId": e.config.NodePoolId,
			"PodSource":  e.config.PodSource,
			"UUID":       uuid,
			"device":     deviceName,
			"gpu":        gpuID,
			"modelName":  "",
		}

		customDeviceLabels := prometheus.Labels{
			"DriverVersion": e.config.DriverVersion,
			"NodeName":      e.config.NodeName,
			"NodePoolId":    e.config.NodePoolId,
			"PodSource":     e.config.PodSource,
			"SupportDCGM":   "Yes",
			"UUID":          uuid,
			"device":        deviceName,
			"gpu":           gpuID,
			"modelName":     "PPU-ZW810E",
		}

		// Memory metrics (98304 MiB total for PPU-ZW810E)
		memoryTotal := 98304.0
		memoryUsed := 18.0 + rand.Float64()*100 // Base usage plus random
		if i == 0 || i == 14 {                  // Some GPUs have higher usage
			memoryUsed = 500 + rand.Float64()*4000
		}
		memoryFree := memoryTotal - memoryUsed

		// Custom metrics
		e.devFBAllocatedGauge.With(customDeviceLabels).Set(0) // Typically 0 when not allocated
		e.devFBTotalGauge.With(customDeviceLabels).Set(memoryTotal)

		// Standard device metrics
		e.devAppMemClockGauge.With(deviceLabels).Set(1800) // MHz
		e.devAppSMClockGauge.With(deviceLabels).Set(1700)  // MHz
		e.devBAR1TotalGauge.With(deviceLabels).Set(memoryTotal)
		e.devBAR1UsedGauge.With(deviceLabels).Set(memoryUsed)

		// Clock throttle reasons (1=idle, 5=power limit)
		throttleReason := 1.0
		if rand.Float64() < 0.2 { // 20% chance of power limit
			throttleReason = 5.0
		}
		e.devClockThrottleReasons.With(deviceLabels).Set(throttleReason)

		// Utilization metrics (typically 0% when idle)
		e.devDecUtilGauge.With(deviceLabels).Set(0)
		e.devEncUtilGauge.With(deviceLabels).Set(0)
		e.devGPUUtilGauge.With(deviceLabels).Set(rand.Float64() * 10) // Low utilization

		// Memory metrics
		e.devFBFreeGauge.With(deviceLabels).Set(memoryFree)
		e.devFBUsedGauge.With(deviceLabels).Set(memoryUsed)
		e.devMemCopyUtilGauge.With(deviceLabels).Set(rand.Float64() * 5) // Low memory utilization

		// Temperature metrics (realistic for idle GPUs)
		gpuTemp := 30 + rand.Float64()*10         // 30-40°C
		memTemp := gpuTemp + 2 + rand.Float64()*3 // Slightly higher
		e.devGPUTempGauge.With(deviceLabels).Set(gpuTemp)
		e.devMemoryTempGauge.With(deviceLabels).Set(memTemp)

		// Clock frequencies
		e.devMemClockGauge.With(deviceLabels).Set(1800)   // MHz
		e.devSMClockGauge.With(deviceLabels).Set(200)     // MHz (idle frequency)
		e.devVideoClockGauge.With(deviceLabels).Set(1000) // MHz

		// Power usage (realistic for PPU cards)
		powerUsage := 80 + rand.Float64()*15 // 80-95W
		e.devPowerUsageGauge.With(deviceLabels).Set(powerUsage)

		// Error counters (typically 0)
		e.devRetiredDBE.With(deviceLabels).Add(0)
		e.devRetiredPending.With(deviceLabels).Add(0)
		e.devRetiredSBE.With(deviceLabels).Add(0)
		e.devXIDErrorsGauge.With(deviceLabels).Set(0)

		// Bandwidth counters (typically 0 when idle)
		e.devNVLinkBandwidthTotal.With(deviceLabels).Add(0)

		// Profiling metrics
		e.profDRAMActiveGauge.With(deviceLabels).Set(rand.Float64() * 5) // Low activity
		e.profNVLinkRXBytes.With(deviceLabels).Add(0)
		e.profNVLinkTXBytes.With(deviceLabels).Add(0)
		e.profPCIeRXBytes.With(deviceLabels).Set(0)
		e.profPCIeTXBytes.With(deviceLabels).Set(0)
	}

	// Add some illegal process metrics for demo (only on GPU 0 and 14)
	for _, gpuIdx := range []int{0, 14} {
		gpuID := strconv.Itoa(gpuIdx)
		deviceName := fmt.Sprintf("nvidia%d", gpuIdx)
		uuid := e.generateUUID(gpuIdx)

		illegalLabels := prometheus.Labels{
			"AllocateMode":  "none",
			"ContainerName": "",
			"NamespaceName": "",
			"NodeName":      e.config.NodeName,
			"NodePoolId":    e.config.NodePoolId,
			"PodName":       "",
			"PodSource":     e.config.PodSource,
			"ProcessId":     "3003",
			"ProcessName":   "python",
			"ProcessType":   "C",
			"UUID":          uuid,
			"device":        deviceName,
			"gpu":           gpuID,
			"modelName":     "PPU-ZW810E",
		}

		// Illegal process metrics
		e.illegalProcessDecodeUtil.With(illegalLabels).Set(0)
		e.illegalProcessEncodeUtil.With(illegalLabels).Set(0)

		if gpuIdx == 14 {
			e.illegalProcessMemCopyUtil.With(illegalLabels).Set(4)
			e.illegalProcessMemUsed.With(illegalLabels).Set(4454)
		} else {
			e.illegalProcessMemCopyUtil.With(illegalLabels).Set(0)
			e.illegalProcessMemUsed.With(illegalLabels).Set(544)
		}

		e.illegalProcessSMUtil.With(illegalLabels).Set(0)
	}
}

func main() {
	var (
		nodeName      = flag.String("node-name", defaultNodeName, "Node name for metrics")
		nodePoolId    = flag.String("node-pool-id", "default", "Node pool ID")
		podSource     = flag.String("pod-source", "ecs", "Pod source")
		port          = flag.Int("port", defaultPort, "Port to serve metrics")
		gpuCount      = flag.Int("gpu-count", defaultGPUCount, "Number of GPUs to simulate")
		driverVersion = flag.String("driver-version", "1.5.1-1d747a", "Driver version")
	)
	flag.Parse()

	config := &Config{
		NodeName:      *nodeName,
		NodePoolId:    *nodePoolId,
		PodSource:     *podSource,
		Port:          *port,
		GPUCount:      *gpuCount,
		DriverVersion: *driverVersion,
	}

	// Initialize random seed
	rand.Seed(time.Now().UnixNano())

	// Create registry and exporter
	registry := prometheus.NewRegistry()
	exporter := NewPPUExporter(config)
	exporter.Register(registry)

	// Update metrics periodically
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for {
			exporter.UpdateMetrics()
			<-ticker.C
		}
	}()

	// Initial metrics update
	exporter.UpdateMetrics()

	// Setup HTTP server
	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
<head><title>PPU Exporter</title></head>
<body>
<h1>PPU Exporter</h1>
<p><a href="/metrics">Metrics</a></p>
</body>
</html>`))
	})

	log.Printf("Starting PPU exporter on port %d", config.Port)
	log.Printf("Node name: %s", config.NodeName)
	log.Printf("GPU count: %d", config.GPUCount)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", config.Port), nil))
}
