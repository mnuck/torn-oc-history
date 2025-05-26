package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/joho/godotenv"
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

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	apiKey := flag.String("key", os.Getenv("TORN_API_KEY"), "Torn API key (or set TORN_API_KEY env var)")
	baseURL := flag.String("base", "https://api.torn.com/v2", "Base URL for Torn API")
	flag.Parse()

	if *apiKey == "" {
		log.Fatal("API key must be provided via -key flag or TORN_API_KEY env var")
	}

	// Step 1: fetch faction members and find those not in OC
	members, err := fetchMembers(*baseURL, *apiKey)
	if err != nil {
		log.Fatalf("fetch members: %v", err)
	}

	inactiveIDs := make(map[int]Member)
	for _, m := range members {
		if !m.IsInOC {
			inactiveIDs[m.ID] = m
		}
	}

	if len(inactiveIDs) == 0 {
		fmt.Println("All faction members are currently in an OC.")
		return
	}

	// Step 2: fetch all completed crimes
	crimes, err := fetchAllCrimes(*baseURL, *apiKey)
	if err != nil {
		log.Fatalf("fetch crimes: %v", err)
	}

	// Step 3: accumulate stats
	stats := make(MemberStats)

	for _, crime := range crimes {
		for _, slot := range crime.Slots {
			uid := slot.User.ID
			if _, ok := inactiveIDs[uid]; !ok {
				continue // consider only inactive members
			}
			if _, ok := stats[uid]; !ok {
				stats[uid] = make(map[int]map[string]RateInfo)
			}
			if _, ok := stats[uid][crime.Difficulty]; !ok {
				stats[uid][crime.Difficulty] = make(map[string]RateInfo)
			}
			if _, ok := stats[uid][crime.Difficulty][slot.Position]; !ok {
				stats[uid][crime.Difficulty][slot.Position] = RateInfo{}
			}
			st := stats[uid][crime.Difficulty][slot.Position]
			// update if this crime is more recent than stored one
			if crime.ExecutedAt > st.ExecutedAt {
				st.Rate = slot.CheckpointPassRate
				st.ExecutedAt = crime.ExecutedAt
				stats[uid][crime.Difficulty][slot.Position] = st
			}
		}
	}

	// Step 4: output report
	printReport(inactiveIDs, stats)
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

func printReport(inactive map[int]Member, stats MemberStats) {
	ids := make([]int, 0, len(inactive))
	for id := range inactive {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	for _, id := range ids {
		m := inactive[id]
		fmt.Printf("\nMember: %s (%d) - Last seen: %s (%s)\n", m.Name, m.ID, m.LastAction.Status, m.LastAction.Relative)
		memberStats, ok := stats[id]
		if !ok {
			fmt.Println("  No historical OC participation recorded.")
			continue
		}
		// sort difficulties
		diffs := make([]int, 0, len(memberStats))
		for d := range memberStats {
			diffs = append(diffs, d)
		}
		sort.Ints(diffs)
		for _, d := range diffs {
			fmt.Printf("  Difficulty %d:\n", d)
			positions := memberStats[d]
			// sort positions alphabetically for consistency
			posNames := make([]string, 0, len(positions))
			for p := range positions {
				posNames = append(posNames, p)
			}
			sort.Strings(posNames)
			for _, p := range posNames {
				st := positions[p]
				if st.Rate == 0 {
					fmt.Printf("    %-15s %s\n", p, "-")
				} else {
					t := time.Unix(st.ExecutedAt, 0)
					fmt.Printf("    %-15s %3d%% (executed_at %s)\n", p, st.Rate, t.Format(time.RFC3339))
				}
			}
		}
	}
}
