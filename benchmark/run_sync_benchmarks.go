package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"cromedia/benchmark2"
)

type PageData struct {
	Results []benchmark2.SyncResult
}

func main() {
	fmt.Println("🚀 Starting CroMedia vs FFmpeg Benchmark 2 (PTS-DTS Matrix / Sync)...")
	results := benchmark2.RunAllSyncTests()
	
	// Output JSON
	jsonPath := filepath.Join("benchmark", "results_sync.json")
	jsonFile, err := os.Create(jsonPath)
	if err != nil {
		fmt.Printf("Error creating results JSON: %v\n", err)
		os.Exit(1)
	}
	defer jsonFile.Close()
	
	encoder := json.NewEncoder(jsonFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(results); err != nil {
		fmt.Printf("Error encoding JSON: %v\n", err)
		os.Exit(1)
	}
	
	// Generate HTML Report
	htmlPath := filepath.Join("benchmark", "report_sync.html")
	htmlFile, err := os.Create(htmlPath)
	if err != nil {
		fmt.Printf("Error creating HTML report: %v\n", err)
		os.Exit(1)
	}
	defer htmlFile.Close()
	
	tmplStr := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>CroMedia vs FFmpeg Sync Benchmarks Dashboard</title>
    <style>
        :root {
            --bg-color: #0b0f19;
            --card-bg: #111827;
            --text-color: #d1d5db;
            --accent-color: #3b82f6;
            --success-color: #10b981;
            --border-color: #1f2937;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
            background-color: var(--bg-color);
            color: var(--text-color);
            margin: 0;
            padding: 40px 20px;
        }
        header {
            text-align: center;
            margin-bottom: 50px;
        }
        h1 {
            color: #fff;
            font-size: 2.2rem;
            margin-bottom: 10px;
        }
        table {
            width: 100%;
            max-width: 1200px;
            margin: 0 auto;
            border-collapse: collapse;
            background-color: var(--card-bg);
            border: 1px solid var(--border-color);
            border-radius: 8px;
            overflow: hidden;
        }
        th, td {
            padding: 15px 20px;
            text-align: left;
            border-bottom: 1px solid var(--border-color);
        }
        th {
            background-color: #1f2937;
            color: #fff;
        }
        tr:hover {
            background-color: #1f2937;
        }
        .badge {
            background-color: var(--success-color);
            color: #fff;
            padding: 4px 10px;
            border-radius: 9999px;
            font-size: 0.85rem;
            font-weight: bold;
        }
    </style>
</head>
<body>
    <header>
        <h1>⏰ Benchmark 2: PTS-DTS Matrix & Sync Analysis</h1>
        <p>Comparative metrics running FATE offsets, OBS frame drops, CCTV drifts, WebRTC Jitter, and MPEG-TS discontinuities.</p>
    </header>

    <table>
        <thead>
            <tr>
                <th>ID</th>
                <th>Scenario Name</th>
                <th>CroMedia Metrics</th>
                <th>FFmpeg Metrics</th>
                <th>Speedup Ratio</th>
                <th>Mem Reduction</th>
                <th>Status</th>
            </tr>
        </thead>
        <tbody>
            {{range .Results}}
            <tr>
                <td>{{.ID}}</td>
                <td>
                    <strong>{{.Name}}</strong><br>
                    <small style="color: #9ca3af;">{{.Description}}</small>
                </td>
                <td>{{.CroMediaMs}} ms / {{printf "%.2f" .CroMediaMem}} MB</td>
                <td>{{.FFmpegMs}} ms / {{printf "%.2f" .FFmpegMem}} MB</td>
                <td><span class="badge">×{{printf "%.1f" (div .FFmpegMs .CroMediaMs)}}</span></td>
                <td>×{{printf "%.1f" (divf .FFmpegMem .CroMediaMem)}}</td>
                <td>{{.Status}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
</body>
</html>`

	// Helper functions for divisions in templates
	funcs := template.FuncMap{
		"div": func(a, b int) float64 {
			if b == 0 {
				return 0
			}
			return float64(a) / float64(b)
		},
		"divf": func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b
		},
	}

	tmpl, err := template.New("sync").Funcs(funcs).Parse(tmplStr)
	if err != nil {
		fmt.Printf("Error parsing template: %v\n", err)
		os.Exit(1)
	}

	if err := tmpl.Execute(htmlFile, PageData{Results: results}); err != nil {
		fmt.Printf("Error executing template: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully ran all sync benchmarks. Output saved to:\n")
	fmt.Printf(" - JSON: %s\n", jsonPath)
	fmt.Printf(" - HTML: %s\n", htmlPath)
}
