//go:build !windows

package plugins

import (
	"errors"
	"fmt"
	"path/filepath"
	"plugin"
)

type unixPlugin struct {
	p *plugin.Plugin
}

func (u *unixPlugin) LookupSymbol(name string) (interface{}, error) {
	return SafeLookupSymbol(u.p, name)
}

func (u *unixPlugin) Close() error {
	return nil
}

func loadPluginOS(path string) error {
	ext := filepath.Ext(path)
	if ext != ".so" && ext != ".dylib" {
		return fmt.Errorf("invalid plugin extension for unix: %s", ext)
	}
	p, err := plugin.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open Go plugin: %w", err)
	}

	u := &unixPlugin{p: p}

	initSym, err := p.Lookup("InitPlugin")
	if err != nil {
		// Fallback: assume self-registering
		pluginsMu.Lock()
		loadedLibraries[path] = u
		pluginsMu.Unlock()
		return nil
	}

	initFunc, ok := initSym.(func())
	if !ok {
		return errors.New("InitPlugin symbol is not a func()")
	}
	initFunc()

	pluginsMu.Lock()
	loadedLibraries[path] = u
	pluginsMu.Unlock()
	return nil
}
