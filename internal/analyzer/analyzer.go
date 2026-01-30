package analyzer

import (
	"fmt"
	"sort"
	"time"

	"github.com/louie/nomad-scheduler-analyzer/internal/parser"
)

// SchedulerIssue represents a detected scheduler issue
type SchedulerIssue struct {
	Severity    string // "critical", "warning", "info"
	Type        string // "goroutine_leak", "excessive_blocking", "lock_contention", etc.
	Title       string
	Description string
	Evidence    []string
	Metrics     map[string]interface{}
}

// Analysis represents the complete analysis result
type Analysis struct {
	BundlePath  string
	NodeID      string
	NodeType    string // "client" or "server"
	Snapshots   []*parser.GoroutineProfile
	Issues      []SchedulerIssue
	Summary     AnalysisSummary
	GeneratedAt time.Time
}

// AnalysisSummary provides high-level metrics
type AnalysisSummary struct {
	TotalSnapshots      int
	GoroutineGrowth     float64 // Percentage growth
	AvgBlockedPercent   float64
	CriticalIssues      int
	WarningIssues       int
	TopBlockingLocation string
	TopBlockingCount    int
}

// Analyzer performs scheduler analysis on profiles
type Analyzer struct {
	profiles []*parser.GoroutineProfile
}

// NewAnalyzer creates a new analyzer
func NewAnalyzer(profiles []*parser.GoroutineProfile) *Analyzer {
	return &Analyzer{
		profiles: profiles,
	}
}

// Analyze performs comprehensive analysis
func (a *Analyzer) Analyze() (*Analysis, error) {
	if len(a.profiles) == 0 {
		return nil, fmt.Errorf("no profiles to analyze")
	}

	analysis := &Analysis{
		Snapshots:   a.profiles,
		Issues:      make([]SchedulerIssue, 0),
		GeneratedAt: time.Now(),
	}

	// Run all detection algorithms
	a.detectGoroutineLeaks(analysis)
	a.detectExcessiveBlocking(analysis)
	a.detectLockContention(analysis)
	a.detectSchedulerStarvation(analysis)

	// Generate summary
	a.generateSummary(analysis)

	return analysis, nil
}

// detectGoroutineLeaks detects goroutine count growth over time
func (a *Analyzer) detectGoroutineLeaks(analysis *Analysis) {
	if len(a.profiles) < 2 {
		return
	}

	first := a.profiles[0]
	last := a.profiles[len(a.profiles)-1]

	growthPercent := float64(last.TotalGoroutines-first.TotalGoroutines) / float64(first.TotalGoroutines) * 100
	growthRate := float64(last.TotalGoroutines-first.TotalGoroutines) / float64(len(a.profiles)-1)

	// Flag if growth > 20% or growth rate > 50 goroutines per snapshot
	if growthPercent > 20 || growthRate > 50 {
		severity := "warning"
		if growthPercent > 50 || growthRate > 100 {
			severity = "critical"
		}

		issue := SchedulerIssue{
			Severity: severity,
			Type:     "goroutine_leak",
			Title:    "Goroutine Leak Detected",
			Description: fmt.Sprintf(
				"Goroutine count increased from %d to %d (%.1f%% growth) across %d snapshots. Growth rate: %.1f goroutines/snapshot.",
				first.TotalGoroutines, last.TotalGoroutines, growthPercent, len(a.profiles), growthRate,
			),
			Evidence: []string{
				fmt.Sprintf("Snapshot 0: %d goroutines", first.TotalGoroutines),
				fmt.Sprintf("Snapshot %d: %d goroutines", len(a.profiles)-1, last.TotalGoroutines),
			},
			Metrics: map[string]interface{}{
				"growth_percent": growthPercent,
				"growth_rate":    growthRate,
				"initial_count":  first.TotalGoroutines,
				"final_count":    last.TotalGoroutines,
			},
		}

		// Identify primary sources of growth
		sources := a.identifyGrowthSources(first, last)
		for _, source := range sources {
			issue.Evidence = append(issue.Evidence, source)
		}

		analysis.Issues = append(analysis.Issues, issue)
	}
}

// detectExcessiveBlocking detects high percentage of blocked goroutines
func (a *Analyzer) detectExcessiveBlocking(analysis *Analysis) {
	for i, profile := range a.profiles {
		blockedPercent := profile.GetBlockedPercentage()

		// Flag if > 50% blocked
		if blockedPercent > 50 {
			severity := "warning"
			if blockedPercent > 70 {
				severity = "critical"
			}

			blockedCount := 0
			for _, g := range profile.Goroutines {
				if g.State != "running" && g.State != "" {
					blockedCount += g.Count
				}
			}

			issue := SchedulerIssue{
				Severity: severity,
				Type:     "excessive_blocking",
				Title:    fmt.Sprintf("Excessive Blocking in Snapshot %d", i),
				Description: fmt.Sprintf(
					"%d goroutines (%.1f%%) are blocked. This may indicate scheduler contention or resource starvation.",
					blockedCount, blockedPercent,
				),
				Evidence: []string{
					fmt.Sprintf("Total goroutines: %d", profile.TotalGoroutines),
					fmt.Sprintf("Blocked goroutines: %d", blockedCount),
				},
				Metrics: map[string]interface{}{
					"blocked_percent": blockedPercent,
					"blocked_count":   blockedCount,
					"total_count":     profile.TotalGoroutines,
					"snapshot":        i,
				},
			}

			// Add top blocking locations
			locations := profile.GetTopBlockingLocations(5)
			issue.Evidence = append(issue.Evidence, "\nTop blocking locations:")
			for loc, count := range locations {
				issue.Evidence = append(issue.Evidence, fmt.Sprintf("  - %s: %d goroutines", loc, count))
			}

			analysis.Issues = append(analysis.Issues, issue)
		}
	}
}

// detectLockContention detects many goroutines waiting on same location
func (a *Analyzer) detectLockContention(analysis *Analysis) {
	for i, profile := range a.profiles {
		locations := profile.GetTopBlockingLocations(10)

		// Check if any single location has > 10% of total goroutines
		threshold := float64(profile.TotalGoroutines) * 0.1

		for location, count := range locations {
			if float64(count) > threshold {
				severity := "warning"
				if float64(count) > float64(profile.TotalGoroutines)*0.3 {
					severity = "critical"
				}

				percent := float64(count) / float64(profile.TotalGoroutines) * 100

				issue := SchedulerIssue{
					Severity: severity,
					Type:     "lock_contention",
					Title:    fmt.Sprintf("Lock Contention Detected in Snapshot %d", i),
					Description: fmt.Sprintf(
						"%d goroutines (%.1f%%) are blocked at the same location. This indicates potential lock contention or a bottleneck.",
						count, percent,
					),
					Evidence: []string{
						fmt.Sprintf("Location: %s", location),
						fmt.Sprintf("Blocked goroutines: %d", count),
						fmt.Sprintf("Percentage of total: %.1f%%", percent),
					},
					Metrics: map[string]interface{}{
						"location":        location,
						"blocked_count":   count,
						"blocked_percent": percent,
						"snapshot":        i,
					},
				}

				analysis.Issues = append(analysis.Issues, issue)
			}
		}
	}
}

// detectSchedulerStarvation detects patterns indicating scheduler starvation
func (a *Analyzer) detectSchedulerStarvation(analysis *Analysis) {
	// Check for persistent blockers across all snapshots
	if len(a.profiles) < 2 {
		return
	}

	// Find goroutines blocked in same location across all snapshots
	persistentBlockers := make(map[string]int)

	for _, profile := range a.profiles {
		locations := profile.GetTopBlockingLocations(5)
		for location := range locations {
			persistentBlockers[location]++
		}
	}

	// Flag locations that appear in all snapshots
	for location, count := range persistentBlockers {
		if count == len(a.profiles) {
			issue := SchedulerIssue{
				Severity: "warning",
				Type:     "scheduler_starvation",
				Title:    "Persistent Blocking Detected",
				Description: fmt.Sprintf(
					"Goroutines are consistently blocked at '%s' across all %d snapshots. This may indicate scheduler starvation or a deadlock.",
					location, len(a.profiles),
				),
				Evidence: []string{
					fmt.Sprintf("Location: %s", location),
					fmt.Sprintf("Present in all %d snapshots", len(a.profiles)),
				},
				Metrics: map[string]interface{}{
					"location":       location,
					"snapshot_count": count,
				},
			}

			analysis.Issues = append(analysis.Issues, issue)
		}
	}
}

// identifyGrowthSources identifies which components are causing goroutine growth
func (a *Analyzer) identifyGrowthSources(first, last *parser.GoroutineProfile) []string {
	sources := make([]string, 0)

	// Compare blocking locations between first and last
	firstLocs := first.GetTopBlockingLocations(10)
	lastLocs := last.GetTopBlockingLocations(10)

	type growth struct {
		location string
		increase int
	}
	growths := make([]growth, 0)

	for loc, lastCount := range lastLocs {
		firstCount := firstLocs[loc]
		if lastCount > firstCount {
			growths = append(growths, growth{loc, lastCount - firstCount})
		}
	}

	// Sort by increase
	sort.Slice(growths, func(i, j int) bool {
		return growths[i].increase > growths[j].increase
	})

	// Add top 3 sources
	for i, g := range growths {
		if i >= 3 {
			break
		}
		sources = append(sources, fmt.Sprintf("Primary source: %s (+%d goroutines)", g.location, g.increase))
	}

	return sources
}

// generateSummary generates analysis summary
func (a *Analyzer) generateSummary(analysis *Analysis) {
	if len(a.profiles) == 0 {
		return
	}

	summary := AnalysisSummary{
		TotalSnapshots: len(a.profiles),
	}

	// Calculate goroutine growth
	if len(a.profiles) > 1 {
		first := a.profiles[0]
		last := a.profiles[len(a.profiles)-1]
		summary.GoroutineGrowth = float64(last.TotalGoroutines-first.TotalGoroutines) / float64(first.TotalGoroutines) * 100
	}

	// Calculate average blocked percentage
	totalBlocked := 0.0
	for _, profile := range a.profiles {
		totalBlocked += profile.GetBlockedPercentage()
	}
	summary.AvgBlockedPercent = totalBlocked / float64(len(a.profiles))

	// Find top blocking location across all snapshots
	allLocations := make(map[string]int)
	for _, profile := range a.profiles {
		locations := profile.GetTopBlockingLocations(10)
		for loc, count := range locations {
			allLocations[loc] += count
		}
	}

	maxCount := 0
	for loc, count := range allLocations {
		if count > maxCount {
			maxCount = count
			summary.TopBlockingLocation = loc
			summary.TopBlockingCount = count
		}
	}

	// Count issues by severity
	for _, issue := range analysis.Issues {
		switch issue.Severity {
		case "critical":
			summary.CriticalIssues++
		case "warning":
			summary.WarningIssues++
		}
	}

	analysis.Summary = summary
}

// Made with Bob
