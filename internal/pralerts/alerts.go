// Package pralerts tracks PR activity between dashboard sessions.
package pralerts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Event struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Actor string `json:"actor"`
}
type PR struct {
	ID     string  `json:"id"`
	Repo   string  `json:"repo"`
	Number int     `json:"number"`
	Title  string  `json:"title"`
	CI     string  `json:"ci"`
	Events []Event `json:"events"`
}
type stateEntry struct {
	Events []string `json:"events"`
	CI     string   `json:"ci"`
}

// Process records a snapshot and identifies activity since the previous snapshot.
func Process(path string, prs []PR) ([]string, error) {
	state := map[string]stateEntry{}
	b, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(b, &state); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	var messages []string
	for _, pr := range prs {
		previous, exists := state[pr.ID]
		seen := map[string]bool{}
		for _, id := range previous.Events {
			seen[id] = true
		}
		for _, event := range pr.Events {
			if exists && !seen[event.ID] {
				messages = append(messages, fmt.Sprintf("%s: %s — %s #%d · %s", event.Kind, event.Actor, pr.Repo, pr.Number, pr.Title))
			}
			if !seen[event.ID] {
				previous.Events = append(previous.Events, event.ID)
				seen[event.ID] = true
			}
		}
		if exists && (pr.CI == "FAILURE" || pr.CI == "ERROR") && previous.CI != "FAILURE" && previous.CI != "ERROR" {
			messages = append(messages, fmt.Sprintf("CI fallido: %s #%d · %s", pr.Repo, pr.Number, pr.Title))
		}
		previous.CI = pr.CI
		state[pr.ID] = previous
	}
	b, err = json.Marshal(state)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".pralerts-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err != nil {
		f.Close()
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return nil, err
	}
	return messages, nil
}
