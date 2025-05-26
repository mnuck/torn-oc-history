package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

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

func main() {
	setupEnvironment()
	apiKey := getRequiredEnv("TORN_API_KEY")
	baseURL := getEnvWithDefault("TORN_API_BASE_URL", "https://api.torn.com/v2")

	allFlag := flag.Bool("all", false, "Include all faction members, not just those not in an OC")
	flag.Parse()

	members, err := fetchMembers(baseURL, apiKey)
	if err != nil {
		log.Fatal().Err(err).Msg("fetch members")
	}

	selected := make(map[int]Member)
	if *allFlag {
		for _, m := range members {
			selected[m.ID] = m
		}
	} else {
		for _, m := range members {
			if !m.IsInOC {
				selected[m.ID] = m
			}
		}
	}

	if len(selected) == 0 {
		fmt.Println("No matching faction members found.")
		return
	}

	crimes, err := fetchAllCrimes(baseURL, apiKey)
	if err != nil {
		log.Fatal().Err(err).Msg("fetch crimes")
	}

	stats := make(MemberStats)

	for _, crime := range crimes {
		for _, slot := range crime.Slots {
			uid := slot.User.ID
			if _, ok := selected[uid]; !ok {
				continue
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
			if crime.ExecutedAt > st.ExecutedAt {
				st.Rate = slot.CheckpointPassRate
				st.ExecutedAt = crime.ExecutedAt
				stats[uid][crime.Difficulty][slot.Position] = st
			}
		}
	}

	printReport(selected, stats)
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
	type memberEntry struct {
		ID   int
		Name string
	}
	entries := make([]memberEntry, 0, len(selected))
	for id, m := range selected {
		entries = append(entries, memberEntry{ID: id, Name: m.Name})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	for _, entry := range entries {
		m := selected[entry.ID]
		fmt.Printf("\nMember: %s (%d) - Last seen: %s (%s)\n", m.Name, m.ID, m.LastAction.Status, m.LastAction.Relative)
		memberStats, ok := stats[entry.ID]
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
