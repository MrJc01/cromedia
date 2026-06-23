package plugins

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"cromedia/core"
	"cromedia/core/demux"
	"cromedia/core/mux"
)

// CurrentABIVersion represents the ABI version of the host.
const CurrentABIVersion = "cromedia-abi-v1"

// PluginMetadata defines the versioning and compatibility metadata for plugins.
type PluginMetadata struct {
	Name       string
	Version    string // SemVer format, e.g. "1.0.0"
	ABIVersion string // ABI version to verify compatibility
}

// IsValidSemVer reports whether a string matches a basic SemVer schema.
func IsValidSemVer(v string) bool {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

// DemuxerPlugin is the interface that a demuxer plugin must implement.
type DemuxerPlugin interface {
	Name() string
	Extensions() []string
	NewDemuxer(file string) (demux.Demuxer, error)
}

// MuxerPlugin is the interface that a muxer plugin must implement.
type MuxerPlugin interface {
	Name() string
	Extensions() []string
	NewMuxer(file string) (mux.Muxer, error)
}

// DecoderPlugin represents a plugin decoder.
type DecoderPlugin interface {
	Name() string
	Type() core.TrackType
	NewDecoder() (interface{}, error)
}

// EncoderPlugin represents a plugin encoder.
type EncoderPlugin interface {
	Name() string
	Type() core.TrackType
	NewEncoder() (interface{}, error)
}

// pluginHandle abstracts the OS-specific dynamic library handle.
type pluginHandle interface {
	LookupSymbol(name string) (interface{}, error)
	Close() error
}

type registeredItem struct {
	kind string // "demux", "mux", "decoder", "encoder"
	name string
	exts []string // for demux/mux extensions
}

var (
	pluginsMu       sync.RWMutex
	demuxerPlugins  = make(map[string]DemuxerPlugin)
	muxerPlugins    = make(map[string]MuxerPlugin)
	decoderPlugins  = make(map[string]DecoderPlugin)
	encoderPlugins  = make(map[string]EncoderPlugin)
	loadedLibraries = make(map[string]pluginHandle) // Tracks loaded handles

	symbolCache   = make(map[string]interface{})
	symbolCacheMu sync.RWMutex

	currentLoadingPath string
	registeredByPath   = make(map[string][]registeredItem)
	loadingMu          sync.Mutex

	// ErrPluginNotCompatible is returned when the plugin has an incompatible ABI or version.
	ErrPluginNotCompatible = errors.New("plugin not compatible: ABI or version mismatch")
)

// RegisterDemuxer registers a demuxer plugin.
func RegisterDemuxer(p DemuxerPlugin) {
	pluginsMu.Lock()
	defer pluginsMu.Unlock()
	demuxerPlugins[p.Name()] = p
	for _, ext := range p.Extensions() {
		demuxerPlugins["ext:"+ext] = p
	}
	if currentLoadingPath != "" {
		registeredByPath[currentLoadingPath] = append(registeredByPath[currentLoadingPath], registeredItem{
			kind: "demux",
			name: p.Name(),
			exts: p.Extensions(),
		})
	}
}

// RegisterMuxer registers a muxer plugin.
func RegisterMuxer(p MuxerPlugin) {
	pluginsMu.Lock()
	defer pluginsMu.Unlock()
	muxerPlugins[p.Name()] = p
	for _, ext := range p.Extensions() {
		muxerPlugins["ext:"+ext] = p
	}
	if currentLoadingPath != "" {
		registeredByPath[currentLoadingPath] = append(registeredByPath[currentLoadingPath], registeredItem{
			kind: "mux",
			name: p.Name(),
			exts: p.Extensions(),
		})
	}
}

// RegisterDecoder registers a decoder plugin.
func RegisterDecoder(p DecoderPlugin) {
	pluginsMu.Lock()
	defer pluginsMu.Unlock()
	decoderPlugins[p.Name()] = p
	if currentLoadingPath != "" {
		registeredByPath[currentLoadingPath] = append(registeredByPath[currentLoadingPath], registeredItem{
			kind: "decoder",
			name: p.Name(),
		})
	}
}

// RegisterEncoder registers an encoder plugin.
func RegisterEncoder(p EncoderPlugin) {
	pluginsMu.Lock()
	defer pluginsMu.Unlock()
	encoderPlugins[p.Name()] = p
	if currentLoadingPath != "" {
		registeredByPath[currentLoadingPath] = append(registeredByPath[currentLoadingPath], registeredItem{
			kind: "encoder",
			name: p.Name(),
		})
	}
}

// GetDemuxer retrieves a registered demuxer plugin by name or extension (prefixed with "ext:").
func GetDemuxer(key string) (DemuxerPlugin, bool) {
	pluginsMu.RLock()
	defer pluginsMu.RUnlock()
	p, ok := demuxerPlugins[key]
	return p, ok
}

// GetMuxer retrieves a registered muxer plugin by name or extension (prefixed with "ext:").
func GetMuxer(key string) (MuxerPlugin, bool) {
	pluginsMu.RLock()
	defer pluginsMu.RUnlock()
	p, ok := muxerPlugins[key]
	return p, ok
}

// LoadPluginDynamic loads a plugin file (.so on Linux, .dll on Windows) dynamically.
func LoadPluginDynamic(path string) error {
	loadingMu.Lock()
	defer loadingMu.Unlock()

	pluginsMu.RLock()
	_, loaded := loadedLibraries[path]
	pluginsMu.RUnlock()
	if loaded {
		return nil // Already loaded
	}

	currentLoadingPath = path
	defer func() {
		currentLoadingPath = ""
	}()

	err := loadPluginOS(path)
	if err != nil {
		core.Log(core.LogLevelWarn, "[Plugin Loader] Dynamic load failed for %s: %v. Using fallback mode.", path, err)
		return fmt.Errorf("plugin load failed (fallback allowed): %w", err)
	}

	pluginsMu.RLock()
	handle := loadedLibraries[path]
	pluginsMu.RUnlock()

	if err := verifyPluginABI(handle); err != nil {
		_ = handle.Close()
		pluginsMu.Lock()
		delete(loadedLibraries, path)
		pluginsMu.Unlock()
		return err
	}

	pluginStatsMu.Lock()
	pluginStats[path] = &PluginStats{
		Name:     filepath.Base(path),
		Path:     path,
		LoadedAt: time.Now(),
	}
	pluginStatsMu.Unlock()

	return nil
}

// verifyPluginABI checks the ABI compatibility of the plugin.
func verifyPluginABI(handle pluginHandle) error {
	// Look up "PluginMetadata" (as variable pointer or value)
	sym, err := handle.LookupSymbol("PluginMetadata")
	var meta *PluginMetadata
	if err == nil {
		if m, ok := sym.(*PluginMetadata); ok {
			meta = m
		} else if m, ok := sym.(PluginMetadata); ok {
			meta = &m
		}
	}

	// Try "GetPluginMetadata" (as function)
	if meta == nil {
		symFunc, errFunc := handle.LookupSymbol("GetPluginMetadata")
		if errFunc == nil {
			if fn, ok := symFunc.(func() PluginMetadata); ok {
				m := fn()
				meta = &m
			} else if fn, ok := symFunc.(func() *PluginMetadata); ok {
				meta = fn()
			}
		}
	}

	if meta == nil {
		// Allow loading plugins without metadata to ensure backward compatibility and basic test mocks.
		return nil
	}

	if meta.Version != "" && !IsValidSemVer(meta.Version) {
		return fmt.Errorf("%w: invalid SemVer version format '%s'", ErrPluginNotCompatible, meta.Version)
	}

	if meta.ABIVersion != CurrentABIVersion {
		return fmt.Errorf("%w: expected ABI '%s', got '%s'", ErrPluginNotCompatible, CurrentABIVersion, meta.ABIVersion)
	}

	return nil
}

// LoadPluginsFromDir loads all plugins in a directory.
func LoadPluginsFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if (runtime.GOOS == "windows" && ext == ".dll") ||
			(runtime.GOOS == "linux" && ext == ".so") ||
			(runtime.GOOS == "darwin" && ext == ".dylib") {
			_ = LoadPluginDynamic(filepath.Join(dir, entry.Name()))
		}
	}
	return nil
}

// UnloadPlugin unloads and clears a specific loaded plugin by path.
func UnloadPlugin(path string) error {
	pluginsMu.Lock()
	defer pluginsMu.Unlock()

	handle, loaded := loadedLibraries[path]
	if !loaded {
		return fmt.Errorf("plugin not loaded: %s", path)
	}

	err := handle.Close()
	delete(loadedLibraries, path)

	// Clean up symbol cache
	symbolCacheMu.Lock()
	for key := range symbolCache {
		if strings.HasPrefix(key, path+"::") {
			delete(symbolCache, key)
		}
	}
	symbolCacheMu.Unlock()

	// Clean up registered plugins associated with this path
	if items, ok := registeredByPath[path]; ok {
		for _, item := range items {
			switch item.kind {
			case "demux":
				delete(demuxerPlugins, item.name)
				for _, ext := range item.exts {
					delete(demuxerPlugins, "ext:"+ext)
				}
			case "mux":
				delete(muxerPlugins, item.name)
				for _, ext := range item.exts {
					delete(muxerPlugins, "ext:"+ext)
				}
			case "decoder":
				delete(decoderPlugins, item.name)
			case "encoder":
				delete(encoderPlugins, item.name)
			}
		}
		delete(registeredByPath, path)
	}

	pluginStatsMu.Lock()
	delete(pluginStats, path)
	pluginStatsMu.Unlock()

	return err
}

// UnloadAllPlugins unloads and clears all loaded plugins.
func UnloadAllPlugins() {
	pluginsMu.Lock()
	defer pluginsMu.Unlock()

	for path, handle := range loadedLibraries {
		_ = handle.Close()
		delete(loadedLibraries, path)
	}

	symbolCacheMu.Lock()
	symbolCache = make(map[string]interface{})
	symbolCacheMu.Unlock()

	demuxerPlugins = make(map[string]DemuxerPlugin)
	muxerPlugins = make(map[string]MuxerPlugin)
	decoderPlugins = make(map[string]DecoderPlugin)
	encoderPlugins = make(map[string]EncoderPlugin)
	registeredByPath = make(map[string][]registeredItem)
}

// ListPlugins returns names of all registered plugins.
func ListPlugins() map[string][]string {
	pluginsMu.RLock()
	defer pluginsMu.RUnlock()

	res := make(map[string][]string)
	for name := range demuxerPlugins {
		if !strings.HasPrefix(name, "ext:") {
			res["demuxers"] = append(res["demuxers"], name)
		}
	}
	for name := range muxerPlugins {
		if !strings.HasPrefix(name, "ext:") {
			res["muxers"] = append(res["muxers"], name)
		}
	}
	for name := range decoderPlugins {
		res["decoders"] = append(res["decoders"], name)
	}
	for name := range encoderPlugins {
		res["encoders"] = append(res["encoders"], name)
	}
	return res
}

// GetCachedSymbol retrieves a looked up symbol from cache, or looks it up and caches it.
func GetCachedSymbol(path string, symName string) (interface{}, error) {
	cacheKey := path + "::" + symName

	symbolCacheMu.RLock()
	sym, exists := symbolCache[cacheKey]
	symbolCacheMu.RUnlock()
	if exists {
		return sym, nil
	}

	pluginsMu.RLock()
	handle, loaded := loadedLibraries[path]
	pluginsMu.RUnlock()
	if !loaded {
		return nil, fmt.Errorf("plugin not loaded: %s", path)
	}

	resolved, err := handle.LookupSymbol(symName)
	if err != nil {
		return nil, err
	}

	symbolCacheMu.Lock()
	symbolCache[cacheKey] = resolved
	symbolCacheMu.Unlock()
	return resolved, nil
}

// LogFromPlugin bridges plugin logs to the core logger.
func LogFromPlugin(lvl int, msg string) {
	var coreLvl core.LogLevel
	switch lvl {
	case 0:
		coreLvl = core.LogLevelDebug
	case 1:
		coreLvl = core.LogLevelInfo
	case 2:
		coreLvl = core.LogLevelWarn
	case 3:
		coreLvl = core.LogLevelError
	default:
		coreLvl = core.LogLevelInfo
	}
	core.Log(coreLvl, "[Plugin Bridge] %s", msg)
}

func init() {
	demux.PluginDemuxerFinder = func(format string, file *os.File) (demux.Demuxer, error) {
		if p, ok := GetDemuxer("ext:" + format); ok {
			return p.NewDemuxer(file.Name())
		}
		if p, ok := GetDemuxer(format); ok {
			return p.NewDemuxer(file.Name())
		}
		return nil, nil
	}
}

