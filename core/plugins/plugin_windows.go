//go:build windows

package plugins

import (
	"fmt"
	"path/filepath"
	"syscall"
)

type windowsPlugin struct {
	dll *syscall.DLL
}

func (w *windowsPlugin) LookupSymbol(name string) (interface{}, error) {
	proc, err := w.dll.FindProc(name)
	if err != nil {
		return nil, err
	}
	return proc, nil
}

func (w *windowsPlugin) Close() error {
	return w.dll.Release()
}

func loadPluginOS(path string) error {
	ext := filepath.Ext(path)
	if ext != ".dll" {
		return fmt.Errorf("invalid plugin extension for Windows: %s", ext)
	}
	dll, err := syscall.LoadDLL(path)
	if err != nil {
		return fmt.Errorf("failed to load DLL: %w", err)
	}

	w := &windowsPlugin{dll: dll}

	proc, err := dll.FindProc("InitPlugin")
	if err == nil {
		_, _, _ = proc.Call()
	}

	pluginsMu.Lock()
	loadedLibraries[path] = w
	pluginsMu.Unlock()
	return nil
}
