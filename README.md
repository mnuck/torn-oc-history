# Torn OC History

A Go application to generate reports on faction members' OC participation history.

## Setup

1. Clone the repository.
2. Install Go 1.22 or later.
3. Run `go mod tidy` to install dependencies.
4. Create a `.env` file with the following variable:

   ```env
   TORN_API_KEY=your_api_key_here
   ```

5. Build the application with `go build`.

## Usage

Run the application with:

```bash
./torncli
```

By default, the report includes only faction members not currently in an OC. To include all members, use the `--all` flag:

```bash
./torncli --all
```

The report is sorted alphabetically by member name (case-insensitive) and includes the datetime the report was generated in ISO8601 format.
