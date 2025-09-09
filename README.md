# Torn OC History

A Go application that retrieves faction OC history and writes a structured report directly to Google Sheets. Each execution overwrites the rows at the configured sheet range.

## Setup

1. Clone the repository.
2. Install **Go 1.24** or later.
3. Inside `torn_oc_history` run `go mod tidy` to install dependencies.
4. Obtain a Google Cloud service-account JSON file with **Google Sheets API** access and save it as `credentials.json` in the same directory as the compiled binary (or run directory when using `go run .`).
5. Configure the application using one of the following methods:

   **Option 1: Environment Variables** (supports `.env` file)
   ```env
   # Torn API key (required)
   TORN_API_KEY=your_torn_api_key

   # Destination Google Sheet ID (required for sheets output)
   SPREADSHEET_ID=1abcdEFG_hijklMNOPQRstuVwxyz1234567890

   # Optional configuration
   TORN_CREDENTIALS_FILE=credentials.json
   TORN_LOG_LEVEL=info
   TORN_ENVIRONMENT=development
   ```

   **Option 2: Command Line Flags**
   ```bash
   ./torn-oc-history --torn-api-key=your_key --spreadsheet-id=your_id --log-level=debug
   ```

   **Option 3: Config File** (config.yaml)
   ```yaml
   torn_api_key: your_torn_api_key
   spreadsheet_id: 1abcdEFG_hijklMNOPQRstuVwxyz1234567890
   credentials_file: credentials.json
   log_level: info
   environment: development
   ```

6. Build with `go build` or run in place with `go run .`.

## Usage

```bash
# build
make build   # or: go build -o torn-oc-history .

# run with command line flags
./torn-oc-history --output stdout            # report printed to terminal (default)
./torn-oc-history --output sheets            # overwrite Google Sheet with not-in-OC report
./torn-oc-history --all --output sheets      # overwrite Google Sheet with all-members report
./torn-oc-history --both --output sheets --range-noc "History!A1" --range-all "HistoryAll!A1"

# run with environment variables (recommended)
TORN_OUTPUT=stdout ./torn-oc-history         # report printed to terminal
TORN_OUTPUT=sheets ./torn-oc-history         # overwrite Google Sheet with not-in-OC report  
TORN_OUTPUT=sheets TORN_ALL=true ./torn-oc-history       # all-members report
TORN_OUTPUT=sheets TORN_BOTH=true ./torn-oc-history      # both reports to default ranges

# run continuously every 5 minutes (production usage)
TORN_OUTPUT=sheets TORN_INTERVAL=5m TORN_BOTH=true ./torn-oc-history
```

The application prints the report to stdout and simultaneously overwrites the Google Sheet range (`SPREADSHEET_RANGE`) with tabular data:

Rows are identical in format to the console output, written one line per row into column A of the target ranges.

Flags

* `--all` – generate report for all faction members.
* `--both` – generate both reports (all members AND those not in OC). Mutually exclusive with `--all`.
* `--output` – `stdout` (default) or `sheets`.
* `--range-noc` – target range for the *not-in-OC* report when writing to Sheets (default `History!A1`).
* `--range-all` – target range for the *all members* report (default `HistoryAll!A1`).
* `--interval` – duration such as `5m`. If >0, program repeats forever at that interval.
* `--credentials-file` – path to Google Cloud service account credentials file (default `credentials.json`).
* `--log-level` – logging level: debug, info, warn, error, fatal, panic, disabled (default `info`).
* `--environment` – environment mode: development, production (default `development`).

## Configuration System

This application uses **Viper** for configuration management, supporting multiple configuration sources with the following priority order:

1. **Command line flags** (highest priority)
2. **Environment variables**
3. **Configuration files** (config.yaml, config.yml, config.json)
4. **Default values** (lowest priority)

### Configuration Sources

**Environment Variables:**

- All environment variables can be prefixed with `TORN_` (e.g., `TORN_LOG_LEVEL`)
- Legacy variables are supported: `TORN_API_KEY`, `SPREADSHEET_ID`, `ENV`, `LOGLEVEL`
- Supports `.env` file loading (automatically loaded if present)

**Flag to Environment Variable Mapping:**
```bash
--output          → TORN_OUTPUT
--all             → TORN_ALL  
--both            → TORN_BOTH
--range-noc       → TORN_RANGE_NOC
--range-all       → TORN_RANGE_ALL
--interval        → TORN_INTERVAL
--credentials-file → TORN_CREDENTIALS_FILE
--log-level       → TORN_LOG_LEVEL
--environment     → TORN_ENVIRONMENT
```

**Conversion Rules:** Add `TORN_` prefix, convert hyphens to underscores, convert to uppercase

**Example .env file:**
```env
# Required
TORN_API_KEY=your_api_key_here
SPREADSHEET_ID=your_sheet_id_here

# Application behavior
TORN_OUTPUT=sheets
TORN_ALL=true
TORN_INTERVAL=5m
TORN_RANGE_NOC=History!A1
TORN_RANGE_ALL=AllMembers!A1

# Logging
TORN_LOG_LEVEL=debug
TORN_ENVIRONMENT=production
```

**Configuration Files:**
- Searched in: current directory, `$HOME/.torn-oc-history/`, `/etc/torn-oc-history/`
- Supported formats: YAML, JSON, TOML
- File name: `config.yaml`, `config.yml`, `config.json`, etc.

**Default Behavior:**
- Production environment (`--environment=production`) defaults to `warn` log level
- Development environment defaults to `info` log level and console-friendly logging
- All flags have sensible defaults for immediate usage
