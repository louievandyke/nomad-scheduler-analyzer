# Nomad Scheduler Analyzer

A Go tool for analyzing Nomad debug bundles to detect potential scheduler issues such as goroutine leaks, excessive blocking, lock contention, and scheduler starvation.

## Features

- 🔍 **Goroutine Leak Detection**: Identifies growing goroutine counts across snapshots
- ⚠️ **Excessive Blocking Analysis**: Detects high percentages of blocked goroutines
- 🔒 **Lock Contention Detection**: Finds hotspots where many goroutines are blocked
- ⏱️ **Scheduler Starvation Detection**: Identifies persistent blocking patterns
- 📊 **Detailed Reports**: Generates text or HTML reports with actionable recommendations
- 📈 **Trend Analysis**: Compares multiple profile snapshots over time

## Installation

### From Source

```bash
git clone https://github.com/louie/nomad-scheduler-analyzer.git
cd nomad-scheduler-analyzer
go build -o nomad-scheduler-analyzer ./cmd/main.go
```

### Using Go Install

```bash
go install github.com/louie/nomad-scheduler-analyzer/cmd@latest
```

## Usage

### Basic Usage

Analyze a Nomad debug bundle:

```bash
./nomad-scheduler-analyzer /path/to/nomad-debug-bundle
```

### With Verbose Output

```bash
./nomad-scheduler-analyzer -v /path/to/nomad-debug-bundle
```

### Generate HTML Report

```bash
./nomad-scheduler-analyzer -f html -o report.html /path/to/nomad-debug-bundle
```

### Save Text Report to File

```bash
./nomad-scheduler-analyzer -o report.txt /path/to/nomad-debug-bundle
```

## Command Line Options

```
Usage:
  nomad-scheduler-analyzer [bundle-path]

Flags:
  -f, --format string   Output format: text, html (default "text")
  -h, --help           help for nomad-scheduler-analyzer
  -o, --output string   Output file (default: stdout)
  -v, --verbose        Verbose output
```

## Understanding the Output

### Summary Section

- **Total Snapshots**: Number of profile snapshots analyzed
- **Goroutine Count**: Initial → Final count with percentage change
- **Avg Blocked**: Average percentage of blocked goroutines across all snapshots
- **Critical/Warning Issues**: Count of detected issues by severity
- **Top Blocking Location**: Most common location where goroutines are blocked

### Issue Types

#### 🔴 Goroutine Leak
Indicates goroutine count is growing over time, suggesting goroutines are not being properly cleaned up.

**Severity Thresholds:**
- Warning: >20% growth or >50 goroutines/snapshot
- Critical: >50% growth or >100 goroutines/snapshot

#### ⚠️ Excessive Blocking
High percentage of goroutines are blocked, indicating potential scheduler contention.

**Severity Thresholds:**
- Warning: >50% blocked
- Critical: >70% blocked

#### 🔒 Lock Contention
Many goroutines blocked at the same location, indicating a bottleneck.

**Severity Thresholds:**
- Warning: >10% of goroutines blocked at same location
- Critical: >30% of goroutines blocked at same location

#### ⏱️ Scheduler Starvation
Goroutines consistently blocked across all snapshots, may indicate deadlock or resource exhaustion.

### Goroutine Breakdown

Shows distribution of goroutines by state:
- **running**: Actively executing
- **chan receive/send**: Waiting on channel operations
- **semacquire**: Waiting on mutex/semaphore
- **IO wait**: Waiting on I/O operations
- **sleep**: In time.Sleep()
- **waitgroup**: Waiting on sync.WaitGroup

### Recommendations

Actionable suggestions based on detected issues:
- Review code for goroutine leaks
- Optimize blocking operations
- Reduce lock contention
- Investigate persistent blockers

## Example Output

```
═══════════════════════════════════════════════════════════════
        NOMAD SCHEDULER ANALYSIS REPORT
═══════════════════════════════════════════════════════════════

Bundle Path: /path/to/nomad-debug-2026-01-22-213719Z
Node ID: 00ab044f-e6ca-2f79-5a03-c92944371b3b
Node Type: client
Generated: 2026-01-30T10:40:00-08:00

───────────────────────────────────────────────────────────────
SUMMARY
───────────────────────────────────────────────────────────────

Total Snapshots: 4
Goroutine Count: 3403 → 3567 (+4.8% change)
Avg Blocked: 63.8%
Critical Issues: 1
Warning Issues: 2

Top Blocking Location:
  consul-template/manager.(*quiescence).tick.func1 (2170 goroutines)

───────────────────────────────────────────────────────────────
ISSUES DETECTED: 3
───────────────────────────────────────────────────────────────

1. 🔴 CRITICAL: Excessive Blocking in Snapshot 0
   Type: excessive_blocking
   2170 goroutines (63.8%) are blocked. This may indicate scheduler 
   contention or resource starvation.
   
   Evidence:
   Total goroutines: 3403
   Blocked goroutines: 2170
   
   Top blocking locations:
     - consul-template/manager.(*quiescence).tick.func1: 2170 goroutines
     - consul-template/dependency.(*VaultWriteQuery).Fetch: 290 goroutines
```

## Nomad Debug Bundle Structure

The tool expects a Nomad debug bundle with the following structure:

```
nomad-debug-bundle/
├── client/
│   └── <node-id>/
│       ├── goroutine-debug1_0000.txt
│       ├── goroutine-debug1_0001.txt
│       ├── goroutine-debug1_0002.txt
│       └── ...
└── server/
    └── <node-name>/
        ├── goroutine-debug1_0000.txt
        ├── goroutine-debug1_0001.txt
        └── ...
```

The tool automatically discovers and analyzes goroutine profiles from either client or server nodes.

## How It Works

1. **Discovery**: Scans the bundle for goroutine-debug1 profile files
2. **Parsing**: Parses each profile to extract goroutine information
3. **Analysis**: Runs multiple detection algorithms:
   - Compares goroutine counts across snapshots
   - Calculates blocking percentages
   - Identifies common blocking locations
   - Detects persistent patterns
4. **Reporting**: Generates comprehensive report with findings and recommendations

## Common Issues Detected

### Consul-Template Related
- High goroutine counts in `consul-template/manager` operations
- Blocked watchers in `consul-template/watch`
- Vault query delays

### gRPC Related
- Blocked streams in `grpc/internal/transport`
- Connection issues
- Stream receive delays

### Plugin System
- `go-plugin` goroutine leaks
- Plugin communication delays
- Broker stream issues

## Troubleshooting

### No profiles found
Ensure the bundle path contains either a `client/` or `server/` directory with goroutine-debug1 files.

### Parse errors
The tool expects goroutine-debug1 format. Ensure profiles were captured correctly in the debug bundle.

### Missing dependencies
Run `go mod tidy` to ensure all dependencies are installed.

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

MIT License

## Author

Created for analyzing Nomad scheduler performance and detecting potential issues in production environments.