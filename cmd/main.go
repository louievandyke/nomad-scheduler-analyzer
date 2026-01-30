package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/louie/nomad-scheduler-analyzer/internal/analyzer"
	"github.com/louie/nomad-scheduler-analyzer/internal/parser"
	"github.com/louie/nomad-scheduler-analyzer/internal/report"
	"github.com/spf13/cobra"
)

var (
	outputFile   string
	outputFormat string
	verbose      bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "nomad-scheduler-analyzer [bundle-path]",
		Short: "Analyze Nomad debug bundles for scheduler issues",
		Long: `Nomad Scheduler Analyzer analyzes pprof data from Nomad debug bundles
to detect potential scheduler issues such as goroutine leaks, excessive blocking,
lock contention, and scheduler starvation.`,
		Args: cobra.ExactArgs(1),
		RunE: runAnalysis,
	}

	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file (default: stdout)")
	rootCmd.Flags().StringVarP(&outputFormat, "format", "f", "text", "Output format: text, html")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runAnalysis(cmd *cobra.Command, args []string) error {
	bundlePath := args[0]

	// Verify bundle path exists
	if _, err := os.Stat(bundlePath); os.IsNotExist(err) {
		return fmt.Errorf("bundle path does not exist: %s", bundlePath)
	}

	if verbose {
		fmt.Printf("Analyzing bundle: %s\n", bundlePath)
	}

	// Discover profile files
	profiles, nodeInfo, err := discoverProfiles(bundlePath)
	if err != nil {
		return fmt.Errorf("failed to discover profiles: %w", err)
	}

	if len(profiles) == 0 {
		return fmt.Errorf("no goroutine profiles found in bundle")
	}

	if verbose {
		fmt.Printf("Found %d profile snapshots\n", len(profiles))
	}

	// Parse all profiles
	parsedProfiles := make([]*parser.GoroutineProfile, 0, len(profiles))
	for i, profilePath := range profiles {
		if verbose {
			fmt.Printf("Parsing profile %d/%d: %s\n", i+1, len(profiles), filepath.Base(profilePath))
		}

		profile, err := parser.ParseGoroutineDebug1(profilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", profilePath, err)
			continue
		}

		parsedProfiles = append(parsedProfiles, profile)
	}

	if len(parsedProfiles) == 0 {
		return fmt.Errorf("failed to parse any profiles")
	}

	if verbose {
		fmt.Printf("Successfully parsed %d profiles\n", len(parsedProfiles))
		fmt.Println("Running analysis...")
	}

	// Run analysis
	analyzer := analyzer.NewAnalyzer(parsedProfiles)
	analysis, err := analyzer.Analyze()
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Set bundle info
	analysis.BundlePath = bundlePath
	analysis.NodeID = nodeInfo.nodeID
	analysis.NodeType = nodeInfo.nodeType

	if verbose {
		fmt.Printf("Analysis complete. Found %d issues.\n", len(analysis.Issues))
	}

	// Generate report
	generator := report.NewGenerator(analysis)

	var output string
	switch outputFormat {
	case "html":
		output, err = generator.GenerateHTML()
		if err != nil {
			return fmt.Errorf("failed to generate HTML report: %w", err)
		}
	case "text", "txt":
		output = generator.GenerateText()
	default:
		return fmt.Errorf("unsupported output format: %s", outputFormat)
	}

	// Write output
	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Printf("Report written to: %s\n", outputFile)
	} else {
		fmt.Println(output)
	}

	return nil
}

type nodeInfo struct {
	nodeID   string
	nodeType string
}

// discoverProfiles finds all goroutine profile files in the bundle
func discoverProfiles(bundlePath string) ([]string, nodeInfo, error) {
	profiles := make([]string, 0)
	info := nodeInfo{}

	// Look for client and server directories
	clientDir := filepath.Join(bundlePath, "client")
	serverDir := filepath.Join(bundlePath, "server")

	var targetDir string
	var nodeType string

	// Check client directory first
	if entries, err := os.ReadDir(clientDir); err == nil && len(entries) > 0 {
		// Find first directory entry (skip files like .DS_Store)
		for _, entry := range entries {
			if entry.IsDir() {
				targetDir = filepath.Join(clientDir, entry.Name())
				info.nodeID = entry.Name()
				info.nodeType = "client"
				nodeType = "client"
				break
			}
		}
	}

	// If no client directory found, try server
	if targetDir == "" {
		if entries, err := os.ReadDir(serverDir); err == nil && len(entries) > 0 {
			// Find first directory entry (skip files like .DS_Store)
			for _, entry := range entries {
				if entry.IsDir() {
					targetDir = filepath.Join(serverDir, entry.Name())
					info.nodeID = entry.Name()
					info.nodeType = "server"
					nodeType = "server"
					break
				}
			}
		}
	}

	if targetDir == "" {
		return nil, info, fmt.Errorf("no client or server directories found in bundle")
	}

	// Find all goroutine-debug1 files
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return nil, info, fmt.Errorf("failed to read %s directory: %w", nodeType, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Look for goroutine-debug1 files
		if strings.HasPrefix(name, "goroutine-debug1_") && strings.HasSuffix(name, ".txt") {
			profiles = append(profiles, filepath.Join(targetDir, name))
		}
	}

	// Sort profiles by name to ensure chronological order
	sort.Strings(profiles)

	return profiles, info, nil
}

// Made with Bob
