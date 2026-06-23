package main

import (
	"fmt"
	"os"

	"cromedia/core/plugins"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run validate_abi.go <path_to_plugin.so>")
		os.Exit(1)
	}

	pluginPath := os.Args[1]
	fmt.Printf("=== Checking plugin ABI and metadata: %s ===\n", pluginPath)

	err := plugins.LoadPluginDynamic(pluginPath)
	if err != nil {
		fmt.Printf("❌ ABI Verification failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ ABI Verification successful! Plugin complies with CurrentABIVersion.")
}
