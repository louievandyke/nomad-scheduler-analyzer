package parser

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Goroutine represents a single goroutine from the profile
type Goroutine struct {
	ID        int
	Count     int           // Number of goroutines with this stack
	State     string        // "chan receive", "semacquire", "IO wait", etc.
	WaitTime  time.Duration // Only available in debug2 profiles
	Stack     []StackFrame
	BlockedOn string // What it's waiting for
}

// StackFrame represents a single frame in a goroutine stack
type StackFrame struct {
	Function string
	File     string
	Line     int
	Address  string
}

// GoroutineProfile represents a complete goroutine profile snapshot
type GoroutineProfile struct {
	Timestamp       time.Time
	TotalGoroutines int
	Goroutines      []Goroutine
	FilePath        string
}

var (
	// Regex patterns for parsing goroutine profiles
	headerRegex     = regexp.MustCompile(`^goroutine profile: total (\d+)$`)
	goroutineRegex  = regexp.MustCompile(`^(\d+) @ (0x[0-9a-f]+(?:\s+0x[0-9a-f]+)*)$`)
	stackFrameRegex = regexp.MustCompile(`^#\s+(0x[0-9a-f]+)\s+(.+?)\+0x[0-9a-f]+\s+(.+)$`)
)

// ParseGoroutineDebug1 parses a goroutine-debug1 text file
func ParseGoroutineDebug1(filePath string) (*GoroutineProfile, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	profile := &GoroutineProfile{
		FilePath:   filePath,
		Goroutines: make([]Goroutine, 0),
	}

	scanner := bufio.NewScanner(file)
	var currentGoroutine *Goroutine
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		// Parse header line
		if matches := headerRegex.FindStringSubmatch(line); matches != nil {
			total, _ := strconv.Atoi(matches[1])
			profile.TotalGoroutines = total
			continue
		}

		// Parse goroutine count and addresses
		if matches := goroutineRegex.FindStringSubmatch(line); matches != nil {
			if currentGoroutine != nil {
				profile.Goroutines = append(profile.Goroutines, *currentGoroutine)
			}
			count, _ := strconv.Atoi(matches[1])
			currentGoroutine = &Goroutine{
				Count: count,
				Stack: make([]StackFrame, 0),
			}
			continue
		}

		// Parse stack frame
		if currentGoroutine != nil {
			if matches := stackFrameRegex.FindStringSubmatch(line); matches != nil {
				frame := StackFrame{
					Address:  matches[1],
					Function: matches[2],
					File:     matches[3],
				}
				currentGoroutine.Stack = append(currentGoroutine.Stack, frame)

				// Infer state from function names
				if currentGoroutine.State == "" {
					currentGoroutine.State = inferStateFromStack(frame.Function)
				}

				// Identify what it's blocked on
				if currentGoroutine.BlockedOn == "" {
					currentGoroutine.BlockedOn = identifyBlockingLocation(frame.Function)
				}
			}
		}

		// Empty line indicates end of goroutine
		if line == "" && currentGoroutine != nil {
			profile.Goroutines = append(profile.Goroutines, *currentGoroutine)
			currentGoroutine = nil
		}
	}

	// Add last goroutine if exists
	if currentGoroutine != nil {
		profile.Goroutines = append(profile.Goroutines, *currentGoroutine)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return profile, nil
}

// ParseGoroutineDebug2 parses a goroutine-debug2 text file (includes wait times)
func ParseGoroutineDebug2(filePath string) (*GoroutineProfile, error) {
	// Similar to debug1 but also extracts wait times
	// For now, use debug1 parser as base
	profile, err := ParseGoroutineDebug1(filePath)
	if err != nil {
		return nil, err
	}

	// TODO: Parse wait times from debug2 format
	// Format: goroutine 1 [running]: or goroutine 1 [chan receive, 5 minutes]:

	return profile, nil
}

// inferStateFromStack infers goroutine state from stack trace
func inferStateFromStack(function string) string {
	function = strings.ToLower(function)

	if strings.Contains(function, "chan receive") || strings.Contains(function, "chanrecv") {
		return "chan receive"
	}
	if strings.Contains(function, "chan send") || strings.Contains(function, "chansend") {
		return "chan send"
	}
	if strings.Contains(function, "semacquire") || strings.Contains(function, "sync.runtime_semacquire") {
		return "semacquire"
	}
	if strings.Contains(function, "pollwait") || strings.Contains(function, "runtime_pollwait") {
		return "IO wait"
	}
	if strings.Contains(function, "sleep") {
		return "sleep"
	}
	if strings.Contains(function, "waitgroup") {
		return "waitgroup"
	}
	if strings.Contains(function, "select") {
		return "select"
	}

	return "running"
}

// identifyBlockingLocation identifies what the goroutine is blocked on
func identifyBlockingLocation(function string) string {
	// Extract meaningful blocking location from function name
	if strings.Contains(function, "consul-template") {
		return "consul-template"
	}
	if strings.Contains(function, "vault") {
		return "vault"
	}
	if strings.Contains(function, "grpc") {
		return "grpc"
	}
	if strings.Contains(function, "plugin") {
		return "plugin"
	}
	if strings.Contains(function, "http") {
		return "http"
	}

	return function
}

// GetBlockedGoroutines returns goroutines that are blocked
func (p *GoroutineProfile) GetBlockedGoroutines() []Goroutine {
	blocked := make([]Goroutine, 0)
	for _, g := range p.Goroutines {
		if g.State != "running" && g.State != "" {
			blocked = append(blocked, g)
		}
	}
	return blocked
}

// GetBlockedPercentage returns percentage of blocked goroutines
func (p *GoroutineProfile) GetBlockedPercentage() float64 {
	if p.TotalGoroutines == 0 {
		return 0
	}

	blockedCount := 0
	for _, g := range p.Goroutines {
		if g.State != "running" && g.State != "" {
			blockedCount += g.Count
		}
	}

	return float64(blockedCount) / float64(p.TotalGoroutines) * 100
}

// GetTopBlockingLocations returns the most common blocking locations
func (p *GoroutineProfile) GetTopBlockingLocations(limit int) map[string]int {
	locations := make(map[string]int)

	for _, g := range p.Goroutines {
		if g.State != "running" && g.State != "" && len(g.Stack) > 0 {
			// Use the top stack frame as the blocking location
			location := g.Stack[0].Function
			locations[location] += g.Count
		}
	}

	return locations
}

// Made with Bob
