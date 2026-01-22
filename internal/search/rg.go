package search

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Result struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Preview string `json:"preview"`
}

type Response struct {
	Query     string   `json:"query"`
	Results   []Result `json:"results"`
	Truncated bool     `json:"truncated"`
}

var ErrRipgrepNotFound = errors.New("ripgrep (rg) not found")

func Ripgrep(rootAbs, query string, limit int) (Response, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return Response{Query: query, Results: nil, Truncated: false}, nil
	}
	if limit <= 0 {
		limit = 200
	}

	if _, err := exec.LookPath("rg"); err != nil {
		return Response{}, ErrRipgrepNotFound
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	args := []string{
		"--json",
		"--no-heading",
		"--line-number",
		"--color=never",
		"--smart-case",
		"--glob=*.md",
		"--glob=*.markdown",
		"--fixed-strings",
		query,
	}
	cmd := exec.CommandContext(ctx, "rg", args...)
	cmd.Dir = rootAbs

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Response{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return Response{}, err
	}

	if err := cmd.Start(); err != nil {
		return Response{}, err
	}

	// Read stderr in case rg fails; keep it small.
	var stderrBuf strings.Builder
	go func() {
		s := bufio.NewScanner(stderr)
		for s.Scan() {
			if stderrBuf.Len() < 4096 {
				stderrBuf.WriteString(s.Text())
				stderrBuf.WriteByte('\n')
			}
		}
	}()

	resp := Response{Query: query, Results: make([]Result, 0, 32)}
	s := bufio.NewScanner(stdout)
	for s.Scan() {
		line := s.Bytes()
		var ev struct {
			Type string `json:"type"`
			Data struct {
				Path struct {
					Text string `json:"text"`
				} `json:"path"`
				Lines struct {
					Text string `json:"text"`
				} `json:"lines"`
				LineNumber int `json:"line_number"`
			} `json:"data"`
		}
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		if ev.Type != "match" {
			continue
		}
		preview := strings.TrimRight(ev.Data.Lines.Text, "\r\n")
		resp.Results = append(resp.Results, Result{Path: ev.Data.Path.Text, Line: ev.Data.LineNumber, Preview: preview})
		if len(resp.Results) >= limit {
			resp.Truncated = true
			break
		}
	}

	_ = stdout.Close()
	// Wait for rg.
	if err := cmd.Wait(); err != nil {
		// rg exits with code 1 if no matches.
		if ee, ok := err.(*exec.ExitError); ok {
			if ee.ExitCode() == 1 {
				return Response{Query: query, Results: nil, Truncated: false}, nil
			}
		}
		sErr := strings.TrimSpace(stderrBuf.String())
		if sErr != "" {
			return Response{}, fmt.Errorf("rg failed: %s", sErr)
		}
		return Response{}, err
	}

	return resp, nil
}
