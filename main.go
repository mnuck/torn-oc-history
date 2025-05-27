package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	sheetspkg "torn-oc-history/internal/sheets"

	"github.com/rs/zerolog/log"
)

type Member struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	IsInOC     bool   `json:"is_in_oc"`
	LastAction struct {
		Status    string `json:"status"`
		Timestamp int64  `json:"timestamp"`
		Relative  string `json:"relative"`
	} `json:"last_action"`
}

type MembersResponse struct {
	Members []Member `json:"members"`
}

type SlotUser struct {
	ID      int    `json:"id"`
	Outcome string `json:"outcome"`
}

type Slot struct {
	Position           string   `json:"position"`
	User               SlotUser `json:"user"`
	CheckpointPassRate int      `json:"checkpoint_pass_rate"`
}

type Crime struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Difficulty int    `json:"difficulty"`
	ExecutedAt int64  `json:"executed_at"`
	Slots      []Slot `json:"slots"`
}

type CrimesResponse struct {
	Crimes []Crime `json:"crimes"`
}

// Store most recent checkpoint pass rate for a member at a given difficulty/position.
type RateInfo struct {
	Rate       int
	ExecutedAt int64
}

// key hierarchy: memberID -> difficulty -> position -> RateInfo
type MemberStats map[int]map[int]map[string]RateInfo

// generateReportLines assembles the human-readable report lines that are printed to stdout.
// The same lines are written into Google Sheets when --output=sheets.
func generateReportLines(selected map[int]Member, stats MemberStats) []string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Report generated at: %s", time.Now().Format(time.RFC3339)))

	type memberEntry struct {
		ID   int
		Name string
	}
	entries := make([]memberEntry, 0, len(selected))
	for id, m := range selected {
		entries = append(entries, memberEntry{ID: id, Name: m.Name})
	}
	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	for _, entry := range entries {
		m := selected[entry.ID]
		// blank line before each member block
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Member: %s (%d) - Last seen: %s (%s)", m.Name, m.ID, m.LastAction.Status, m.LastAction.Relative))

		memberStats, ok := stats[entry.ID]
		if !ok {
			lines = append(lines, "  No historical OC participation recorded.")
			continue
		}

		// sort difficulties
		diffs := make([]int, 0, len(memberStats))
		for d := range memberStats {
			diffs = append(diffs, d)
		}
		sort.Ints(diffs)
		for _, d := range diffs {
			lines = append(lines, fmt.Sprintf("  Difficulty %d:", d))
			positions := memberStats[d]
			// sort positions alphabetically
			var posNames []string
			for p := range positions {
				posNames = append(posNames, p)
			}
			sort.Strings(posNames)
			for _, p := range posNames {
				st := positions[p]
				if st.Rate == 0 {
					lines = append(lines, fmt.Sprintf("    %-15s %s", p, "-"))
				} else {
					t := time.Unix(st.ExecutedAt, 0)
					lines = append(lines, fmt.Sprintf("    %-15s %3d%% (executed_at %s)", p, st.Rate, t.Format(time.RFC3339)))
				}
			}
		}
	}
	return lines
}

// NEW FUNCTION TO BUILD SHEET ROWS
func buildSheetRows(selected map[int]Member, stats MemberStats) [][]interface{} {
	lines := generateReportLines(selected, stats)
	rows := make([][]interface{}, len(lines))
	for i, line := range lines {
		rows[i] = []interface{}{line}
	}
	return rows
}

func main() {
	setupEnvironment()
	ctx := context.Background()
	// Command-line flags
	outputDest := flag.String("output", "stdout", "output destination: stdout or sheets")
	allFlag := flag.Bool("all", false, "Generate report for all faction members")
	bothFlag := flag.Bool("both", false, "Generate both reports (all members and those not in OC)")
	nocRange := flag.String("range-noc", "History!A1", "Spreadsheet range for members not in OC")
	allRange := flag.String("range-all", "HistoryAll!A1", "Spreadsheet range for all members report")
	interval := flag.Duration("interval", 0, "Repeat execution at this interval (e.g. 5m). 0 runs once")
	flag.Parse()

	if *bothFlag && *allFlag {
		log.Fatal().Msg("--all and --both cannot be used together")
	}

	var sheetsClient *sheetspkg.Client
	if *outputDest == "sheets" {
		credsFile := "credentials.json" // credentials placed alongside binary
		var err error
		sheetsClient, err = sheetspkg.NewClient(ctx, credsFile)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create sheets client")
		}
	} else if *outputDest != "stdout" {
		log.Fatal().Msg("--output must be either 'stdout' or 'sheets'")
	}

	apiKey := getRequiredEnv("TORN_API_KEY")
	baseURL := "https://api.torn.com/v2"

	runReports := func() {
		members, err := fetchMembers(baseURL, apiKey)
		if err != nil {
			log.Error().Err(err).Msg("fetch members")
			return
		}

		selectedAll := make(map[int]Member)
		for _, m := range members {
			selectedAll[m.ID] = m
		}

		selectedNoOC := make(map[int]Member)
		for _, m := range members {
			if !m.IsInOC {
				selectedNoOC[m.ID] = m
			}
		}

		var selected map[int]Member
		if *bothFlag {
			selected = selectedNoOC // used for empty check only
		} else if *allFlag {
			selected = selectedAll
		} else {
			selected = selectedNoOC
		}

		if len(selected) == 0 && !*bothFlag {
			fmt.Println("No matching faction members found.")
			return
		}

		crimes, err := fetchAllCrimes(baseURL, apiKey)
		if err != nil {
			log.Error().Err(err).Msg("fetch crimes")
			return
		}

		statsAll := make(MemberStats)
		for _, crime := range crimes {
			for _, slot := range crime.Slots {
				uid := slot.User.ID
				if _, ok := statsAll[uid]; !ok {
					statsAll[uid] = make(map[int]map[string]RateInfo)
				}
				if _, ok := statsAll[uid][crime.Difficulty]; !ok {
					statsAll[uid][crime.Difficulty] = make(map[string]RateInfo)
				}
				if _, ok := statsAll[uid][crime.Difficulty][slot.Position]; !ok {
					statsAll[uid][crime.Difficulty][slot.Position] = RateInfo{}
				}
				st := statsAll[uid][crime.Difficulty][slot.Position]
				if crime.ExecutedAt > st.ExecutedAt {
					st.Rate = slot.CheckpointPassRate
					st.ExecutedAt = crime.ExecutedAt
					statsAll[uid][crime.Difficulty][slot.Position] = st
				}
			}
		}

		if *bothFlag {
			if *outputDest == "stdout" {
				fmt.Println("=== Members not in OC ===")
				printReport(selectedNoOC, statsAll)
				fmt.Println("\n=== All Members ===")
				printReport(selectedAll, statsAll)
			} else {
				spreadsheetID := getRequiredEnv("SPREADSHEET_ID")
				rowsNoOC := buildSheetRows(selectedNoOC, statsAll)
				if err := sheetsClient.ClearRange(ctx, spreadsheetID, *nocRange); err != nil {
					log.Error().Err(err).Msg("clear not-in-OC sheet")
				}
				if err := sheetsClient.UpdateRange(ctx, spreadsheetID, *nocRange, rowsNoOC); err != nil {
					log.Error().Err(err).Msg("write not-in-OC sheet")
				} else {
					log.Info().Int("rows", len(rowsNoOC)).Msg("Wrote NOT_IN_OC report to Google Sheet")
				}

				rowsAll := buildSheetRows(selectedAll, statsAll)
				if err := sheetsClient.ClearRange(ctx, spreadsheetID, *allRange); err != nil {
					log.Error().Err(err).Msg("clear ALL sheet")
				}
				if err := sheetsClient.UpdateRange(ctx, spreadsheetID, *allRange, rowsAll); err != nil {
					log.Error().Err(err).Msg("write ALL sheet")
				} else {
					log.Info().Int("rows", len(rowsAll)).Msg("Wrote ALL report to Google Sheet")
				}
			}
		} else {
			if *outputDest == "stdout" {
				printReport(selected, statsAll)
			} else {
				spreadsheetID := getRequiredEnv("SPREADSHEET_ID")
				rows := buildSheetRows(selected, statsAll)
				targetRange := *nocRange
				if *allFlag {
					targetRange = *allRange
				}
				if err := sheetsClient.ClearRange(ctx, spreadsheetID, targetRange); err != nil {
					log.Error().Err(err).Msg("clear sheet")
				}
				if err := sheetsClient.UpdateRange(ctx, spreadsheetID, targetRange, rows); err != nil {
					log.Error().Err(err).Msg("write sheet")
				} else {
					log.Info().Int("rows", len(rows)).Msg("Wrote report to Google Sheet")
				}
			}
		}
	}

	// first run
	runReports()

	if *interval > 0 {
		ticker := time.NewTicker(*interval)
		defer ticker.Stop()
		for range ticker.C {
			runReports()
		}
	}
}

func fetchMembers(baseURL, key string) ([]Member, error) {
	url := fmt.Sprintf("%s/faction/members?key=%s", baseURL, key)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bad status: %s: %s", resp.Status, string(body))
	}

	var mr MembersResponse
	if err := json.NewDecoder(resp.Body).Decode(&mr); err != nil {
		return nil, err
	}
	return mr.Members, nil
}

func fetchAllCrimes(baseURL, key string) ([]Crime, error) {
	const pageSize = 100
	offset := 0
	var all []Crime

	for {
		url := fmt.Sprintf("%s/faction/crimes?key=%s&cat=completed&offset=%d", baseURL, key, offset)
		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("bad status: %s: %s", resp.Status, string(body))
		}
		var cr CrimesResponse
		if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		all = append(all, cr.Crimes...)
		if len(cr.Crimes) < pageSize {
			break
		}
		offset += pageSize
	}
	return all, nil
}

func printReport(selected map[int]Member, stats MemberStats) {
	for _, line := range generateReportLines(selected, stats) {
		fmt.Println(line)
	}
}
