package core

import (
	"fmt"
	"os"
)

// PipelineBuilder provides a fluent API for building and executing pipelines
type PipelineBuilder struct {
	inputFile  string
	outputFile string
	scaleW     int
	scaleH     int
	volumeGain float32
	err        error
}

// Input sets the input file path
func Input(filePath string) *PipelineBuilder {
	return &PipelineBuilder{inputFile: filePath}
}

// Scale adds a video scaling step to the pipeline builder
func (b *PipelineBuilder) Scale(w, h int) *PipelineBuilder {
	if b.err != nil { return b }
	b.scaleW = w
	b.scaleH = h
	return b
}

// Volume adds an audio volume adjustment step
func (b *PipelineBuilder) Volume(gain float32) *PipelineBuilder {
	if b.err != nil { return b }
	b.volumeGain = gain
	return b
}

// Output sets the output file path
func (b *PipelineBuilder) Output(filePath string) *PipelineBuilder {
	if b.err != nil { return b }
	b.outputFile = filePath
	return b
}

// Run executes the configured pipeline
func (b *PipelineBuilder) Run() error {
	if b.err != nil {
		return b.err
	}
	if b.inputFile == "" || b.outputFile == "" {
		return fmt.Errorf("both input and output file paths must be specified")
	}

	fmt.Printf("[FluentAPI] Executing pipeline: Input=%s, Output=%s\n", b.inputFile, b.outputFile)
	
	// Simulate checking input file exists
	f, err := os.Open(b.inputFile)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	f.Close()

	// Simulate successful execution
	out, err := os.Create(b.outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	out.Close()

	fmt.Println("[FluentAPI] Pipeline run completed successfully.")
	return nil
}
