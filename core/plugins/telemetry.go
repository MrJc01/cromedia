package plugins

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// PluginStats holds telemetry data for loaded plugins.
type PluginStats struct {
	Name        string        `json:"name"`
	Path        string        `json:"path"`
	LoadedAt    time.Time     `json:"loaded_at"`
	CallCount   int64         `json:"call_count"`
	TotalTime   time.Duration `json:"total_time"`
	TotalCPU    time.Duration `json:"total_cpu"`
	LastLatency time.Duration `json:"last_latency"`
}

var (
	pluginStatsMu sync.RWMutex
	pluginStats   = make(map[string]*PluginStats)
)

// RecordPluginCall records telemetry for a plugin call.
func RecordPluginCall(name string, duration time.Duration, cpu time.Duration) {
	pluginStatsMu.Lock()
	defer pluginStatsMu.Unlock()
	s, ok := pluginStats[name]
	if !ok {
		s = &PluginStats{
			Name:     name,
			LoadedAt: time.Now(),
		}
		pluginStats[name] = s
	}
	s.CallCount++
	s.TotalTime += duration
	s.TotalCPU += cpu
	s.LastLatency = duration
}

// HandlePluginDebugInfo serves the plugin telemetry data over HTTP.
func HandlePluginDebugInfo(w http.ResponseWriter, r *http.Request) {
	pluginStatsMu.RLock()
	defer pluginStatsMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(pluginStats)
}

// GetPluginStats returns the current statistics of all plugins.
func GetPluginStats() map[string]PluginStats {
	pluginStatsMu.RLock()
	defer pluginStatsMu.RUnlock()

	res := make(map[string]PluginStats)
	for k, v := range pluginStats {
		res[k] = *v
	}
	return res
}
