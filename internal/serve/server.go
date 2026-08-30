// Package serve is the minimal observation app (D-40).
//
// One read-only page over SSE, built to A-2's shell so the navigation model and
// component vocabulary get validated early at a fraction of the cost of the
// eleven screens. The full app stays v0.2.
//
// It reads state and renders it. It never dispatches an agent and never runs a
// command — the app suggests, and the suggestions are text you copy.
package serve

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/events"
	"github.com/edwardmontoya/circle/internal/task"
	"github.com/edwardmontoya/circle/internal/timeline"
)

//go:embed assets/*
var assets embed.FS

// Snapshot is everything the page renders. One shape, so the initial load and
// every SSE frame are identical and the client has no second code path.
type Snapshot struct {
	Project   string         `json:"project"`
	Branch    string         `json:"branch"`
	Status    int            `json:"status"`
	Determ    int            `json:"deterministic"`
	Forced    bool           `json:"forced"`
	Comps     []CompView     `json:"components"`
	Contracts []CheckView    `json:"contracts"`
	Tasks     []domain.Task  `json:"tasks"`
	Timeline  []domain.Entry `json:"timeline"`
	Counts    map[string]int `json:"counts"`
	Suggest   []string       `json:"suggest"`
	At        time.Time      `json:"at"`
}

type CompView struct {
	Label    string `json:"label"`
	Achieved int    `json:"achieved"`
	Weight   int    `json:"weight"`
	Detail   string `json:"detail"`
}

type CheckView struct {
	Severity string `json:"severity"`
	Section  string `json:"section"`
	Label    string `json:"label"`
	Detail   string `json:"detail"`
}

func build(repo *contract.Repo) Snapshot {
	evs, _ := events.All(repo)
	gates := repo.Contract.Quality.Gates()
	pass, total := events.GateSummary(evs, gates)

	ts, _ := task.Load(repo)
	closed, verified := 0, 0
	for _, t := range ts {
		if t.State == domain.StateClosed {
			closed++
			if t.ClosedBy != "" {
				verified++
			}
		}
	}
	tl, _ := timeline.Load(repo, "default")
	res := contract.Validate(repo)
	s := domain.ComputeStatus(pass, total, verified, len(ts), closed, len(ts), 0, res.Forced)

	snap := Snapshot{
		Project: repo.Contract.Project.Name,
		Branch:  tl.Branch,
		Status:  s.Total(), Determ: s.DeterministicPct(), Forced: res.Forced,
		Tasks: ts, Timeline: tl.Entries, At: time.Now().UTC(),
		Counts: map[string]int{},
	}
	labels := s.Labels()
	for i, c := range s.Components() {
		snap.Comps = append(snap.Comps, CompView{labels[i], c.Achieved, c.Weight, c.Detail})
	}
	for _, c := range res.Checks {
		snap.Contracts = append(snap.Contracts,
			CheckView{string(c.Severity), c.Section, c.Label, c.Detail})
	}
	live, rew, ph, unc := tl.Counts()
	snap.Counts["live"], snap.Counts["rewritten"] = live, rew
	snap.Counts["phantom"], snap.Counts["uncommitted"] = ph, unc

	// Suggestions are copyable text, never buttons that act. A second execution
	// path is a second thing that goes stale (D-6).
	for _, t := range task.Ready(ts) {
		snap.Suggest = append(snap.Suggest, "circle task claim "+t.ID)
	}
	if pass < total {
		snap.Suggest = append(snap.Suggest, "circle quality run")
	}
	if res.Blocked() {
		snap.Suggest = append(snap.Suggest, "circle preflight --explain")
	}
	return snap
}

// Serve runs the app until the process ends.
func Serve(repo *contract.Repo, addr string) error {
	mux := http.NewServeMux()

	page, err := assets.ReadFile("assets/index.html")
	if err != nil {
		return err
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	})

	mux.HandleFunc("/api/snapshot", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(build(repo))
	})

	mux.HandleFunc("/api/stream", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		tick := time.NewTicker(2 * time.Second)
		defer tick.Stop()
		var last string
		for {
			select {
			case <-r.Context().Done():
				return
			case <-tick.C:
				b, err := json.Marshal(build(repo))
				if err != nil {
					continue
				}
				// Only push when something actually moved. A dashboard that
				// repaints on a timer teaches you to stop looking at it.
				if string(b) == last {
					continue
				}
				last = string(b)
				fmt.Fprintf(w, "data: %s\n\n", b)
				flusher.Flush()
			}
		}
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return srv.ListenAndServe()
}
