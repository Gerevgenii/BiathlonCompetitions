package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Laps        int    `json:"laps"`
	LapLen      int    `json:"lapLen"`
	PenaltyLen  int    `json:"penaltyLen"`
	FiringLines int    `json:"firingLines"`
	Start       string `json:"start"`
	StartDelta  string `json:"startDelta"`
}

type Event struct {
	Time         time.Time
	RawTime      string
	EventID      int
	CompetitorID int
	Extra        string
}

type Competitor struct {
	ID             int
	Started        bool
	LapsCompleted  int
	Hits           int
	isDisqualified bool
	isNotFinished  bool
	StartTime      time.Time
	FinishTime     time.Time
	StartPenalty   time.Time
	lapTimes       []time.Duration
	PenaltyTimes   []time.Duration
}

var (
	eventRegex = regexp.MustCompile(`\[(\d{2}:\d{2}:\d{2}\.\d{3})\] (\d+) (\d+)(?: (.*))?`)
	timeLayout = "15:04:05.000"
)

const (
	undefined = iota
	register
	startTime
	startLine
	isStarted
	onTheFiringRange
	hit
	leftTheFiringRange
	enteredThePenaltyLaps
	leftThePenaltyLaps
	endedTheMainLap
	comment
)

func loadConfig(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {

		}
	}(f)
	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func parseEvent(line string) (Event, error) {
	matches := eventRegex.FindStringSubmatch(line)
	if len(matches) < 4 {
		return Event{}, fmt.Errorf("invalid event format")
	}
	t, err := time.Parse(timeLayout, matches[1])
	if err != nil {
		return Event{}, err
	}
	eid, _ := strconv.Atoi(matches[2])
	cid, _ := strconv.Atoi(matches[3])
	extra := matches[4]
	return Event{Time: t, RawTime: matches[1], EventID: eid, CompetitorID: cid, Extra: extra}, nil
}

func loadEvents(path string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {

		}
	}(f)
	var events []Event
	s := bufio.NewScanner(f)
	for s.Scan() {
		e, err := parseEvent(s.Text())
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, s.Err()
}

func parseDelta(s string) (time.Duration, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid delta format: %s", s)
	}
	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	sSec, _ := strconv.ParseFloat(parts[2], 64)
	sec := int(sSec)
	msec := int((sSec - float64(sec)) * 1000)
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second + time.Duration(msec)*time.Millisecond, nil
}

func printResults(competitors map[int]*Competitor, cfg Config) {
	fmt.Println("\nFinal results:")
	for _, comp := range competitors {
		var status string
		if (comp.FinishTime.Equal(time.Time{}) || comp.isDisqualified || comp.LapsCompleted != cfg.Laps) {
			status = "[NotFinished]"
		} else if comp.isNotFinished {
			status = "[NotStarted]"
		} else if comp.Started {
			status = comp.FinishTime.Sub(comp.StartTime).String()
		} else {
			status = "[Unknown]"
		}
		var averageSpeed []float64
		for _, e := range comp.lapTimes {
			averageSpeed = append(averageSpeed, float64(cfg.LapLen)/e.Seconds())
		}
		fmt.Printf("%s Competitor %d: laps count %d, laps [",
			status, comp.ID, comp.LapsCompleted)
		for i, lap := range comp.lapTimes {
			fmt.Printf("{%s, %.3f}", time.Time{}.Add(lap).Format(timeLayout), averageSpeed[i])
			if i != len(comp.lapTimes)-1 {
				fmt.Printf(", ")
			}
		}
		fmt.Printf("], Penalty [")
		for i, lap := range comp.PenaltyTimes {
			fmt.Printf("{%s, %.3f}",
				time.Time{}.Add(lap).Format(timeLayout),
				float64(cfg.PenaltyLen)/lap.Seconds(),
			)
			if i != len(comp.PenaltyTimes)-1 {
				fmt.Printf(", ")
			}
		}
		fmt.Printf("], Hits %d/%d\n",
			comp.Hits,
			cfg.Laps*5,
		)
	}
}

func main() {
	cfg, err := loadConfig("config/config.json")
	if err != nil {
		fmt.Println("Config error:", err)
		return
	}
	
	baseStart, err := time.Parse(timeLayout, cfg.Start)
	if err != nil {
		fmt.Println("Invalid start time in config:", err)
		return
	}

	delta, err := parseDelta(cfg.StartDelta)
	if err != nil {
		fmt.Println("Invalid startDelta in config:", err)
		return
	}

	events, err := loadEvents("events")
	if err != nil {
		fmt.Println("Events error:", err)
		return
	}
	sort.Slice(events, func(i, j int) bool {
		return events[i].Time.Before(events[j].Time)
	})

	competitors := make(map[int]*Competitor)
	var startOrder []Competitor

	for _, e := range events {
		comp := competitors[e.CompetitorID]
		switch e.EventID {
		case register:
			var competitor = &Competitor{ID: e.CompetitorID}
			competitors[e.CompetitorID] = competitor
			fmt.Printf("[%s] The competitor(%d) registered\n", e.RawTime, e.CompetitorID)
		case startTime:
			comp.StartTime, err = time.Parse(timeLayout, e.Extra)
			if err != nil {
				fmt.Println("Invalid incoming startTime in events:", err)
			}
			deltaTime, err := time.Parse("15:04:05", cfg.StartDelta)
			if err != nil {
				fmt.Println("Invalid delta time in config:", err)
			}
			if len(startOrder) == 0 {
				if comp.StartTime.Sub(baseStart) > deltaTime.Sub(time.Date(deltaTime.Year(), deltaTime.Month(), deltaTime.Day(), 0, 0, 0, 0, deltaTime.Location())) {
					comp.isNotFinished = true
				}
			} else if comp.StartTime.Sub(startOrder[len(startOrder)-1].StartTime) > deltaTime.Sub(time.Date(deltaTime.Year(), deltaTime.Month(), deltaTime.Day(), 0, 0, 0, 0, deltaTime.Location())) {
				comp.isNotFinished = true
			}
			startOrder = append(startOrder, *comp)
			fmt.Printf("[%s] The start time for the competitor(%d) was set by a draw to %s\n", e.RawTime, e.CompetitorID, comp.StartTime.Format(timeLayout))
		case startLine:
			fmt.Printf("[%s] The competitor is on the start line\n", e.RawTime)
		case isStarted:
			allowed := comp.StartTime.Add(delta)
			if e.Time.After(allowed) {
				comp.isNotFinished = true
				fmt.Printf("[%s] The competitor(%d) is disqualified for late start\n", e.RawTime, e.CompetitorID)
			}
			comp.Started = true
			fmt.Printf("[%s] The competitor(%d) has started\n", e.RawTime, e.CompetitorID)
		case onTheFiringRange:
			fmt.Printf("[%s] The competitor(%d) is on the firing range (%s)\n", e.RawTime, e.CompetitorID, e.Extra)
		case hit:
			comp.Hits++
			fmt.Printf("[%s] The target has been hit (%s) by competitor(%d)\n", e.RawTime, e.Extra, e.CompetitorID)
		case leftTheFiringRange:
			fmt.Printf("[%s] The competitor(%d) left the firing range (%d)\n", e.RawTime, e.CompetitorID, comp.LapsCompleted)
		case enteredThePenaltyLaps:
			comp.StartPenalty = e.Time
			fmt.Printf("[%s] The competitor(%d) entered the penalty laps\n", e.RawTime, e.CompetitorID)
		case leftThePenaltyLaps:
			comp.PenaltyTimes = append(comp.PenaltyTimes, e.Time.Sub(comp.StartPenalty))
			fmt.Printf("[%s] The competitor(%d) left the penalty laps\n", e.RawTime, e.CompetitorID)
		case endedTheMainLap:
			comp.LapsCompleted++
			if len(comp.lapTimes) == 0 && comp.LapsCompleted == cfg.Laps {
				comp.lapTimes = append(comp.lapTimes, e.Time.Sub(comp.StartTime))
			}
			comp.FinishTime = e.Time
			fmt.Printf("[%s] The competitor(%d) ended the main lap\n", e.RawTime, e.CompetitorID)
		case comment:
			if comp.LapsCompleted != cfg.Laps {
				comp.lapTimes = append(comp.lapTimes, e.Time.Sub(comp.StartTime))
			}
			comp.isDisqualified = true
			fmt.Printf("[%s] The competitor(%d) can`t continue: %s\n", e.RawTime, e.CompetitorID, e.Extra)
		default:
			fmt.Printf("Unknown EventId %d\n. The EventID must be in the range [1, 11]", e.EventID)
		}
	}
	printResults(competitors, cfg)
}
