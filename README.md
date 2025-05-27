# Torn OC History

A Go application that retrieves faction OC history and writes a structured report directly to Google Sheets. Each execution overwrites the rows at the configured sheet range.

## Setup

1. Clone the repository.
2. Install **Go 1.24** or later.
3. Inside `torn_oc_history` run `go mod tidy` to install dependencies.
4. Obtain a Google Cloud service-account JSON file with **Google Sheets API** access and save it as `credentials.json` in the same directory as the compiled binary (or run directory when using `go run .`).
5. Create a `.env` file with the required variables:

   ```env
   # Torn API key
   TORN_API_KEY=your_torn_api_key

   # Destination Google Sheet ID (the long string after /d/ in the sheet URL)
   SPREADSHEET_ID=1abcdEFG_hijklMNOPQRstuVwxyz1234567890

   ```

6. Build with `go build` or run in place with `go run .`.

## Usage

```bash
# build
make build   # or: go build -o torn-oc-history .

# run (overwrites sheet range each time)
./torn-oc-history --output stdout            # report printed to terminal (default)
./torn-oc-history --output sheets                         # overwrite Google Sheet with not-in-OC report
./torn-oc-history --all --output sheets                   # overwrite Google Sheet with all-members report
./torn-oc-history --both --output sheets --range-noc "History!A1" --range-all "HistoryAll!A1"  # write both reports to different ranges
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
