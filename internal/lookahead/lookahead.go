package lookahead

import (
	"errors"
	"fmt"
	"path/filepath"
	"server/internal/contract"
	"strconv"
	"strings"
	"time"
)

const (
	dateLayout = "2006-01-02"
	MinWeeks   = 1
	MaxWeeks   = 12
)

var ErrWindow = errors.New("a lookahead needs a from date and 1 to 12 weeks")

type Window struct {
	From  time.Time
	Weeks int
}

func Parse(from, weeks string) (Window, error) {
	start, err := time.Parse(dateLayout, from)
	if err != nil {
		return Window{}, ErrWindow
	}

	n, err := strconv.Atoi(weeks)
	if err != nil || n < MinWeeks || n > MaxWeeks {
		return Window{}, ErrWindow
	}

	return Window{From: start, Weeks: n}, nil
}

func (w Window) Until() time.Time {
	return w.From.AddDate(0, 0, 7*w.Weeks)
}

func (w Window) LastDay() time.Time {
	return w.Until().AddDate(0, 0, -1)
}

func (w Window) Holds(t contract.Task) bool {
	if t.IsSummary || t.PercentComplete >= 100 || t.Start == nil || t.Finish == nil {
		return false
	}

	start, finish := day(*t.Start), day(*t.Finish)
	if start == "" || finish == "" {
		return false
	}

	return start < w.Until().Format(dateLayout) && finish >= w.From.Format(dateLayout)
}

func (w Window) Select(c *contract.Contract) map[int64]bool {
	parents := make(map[int64]*int64, len(c.Tasks))
	for _, t := range c.Tasks {
		parents[t.ID] = t.ParentID
	}

	kept := make(map[int64]bool)
	for _, t := range c.Tasks {
		if !w.Holds(t) {
			continue
		}

		id := &t.ID
		for id != nil && !kept[*id] {
			kept[*id] = true
			id = parents[*id]
		}
	}

	return kept
}

func day(value string) string {
	if len(value) < len(dateLayout) {
		return ""
	}
	return value[:len(dateLayout)]
}

func (w Window) FileName(original string) string {
	ext := filepath.Ext(original)
	base := strings.TrimSuffix(original, ext)
	if base == "" {
		base = "project"
	}
	return fmt.Sprintf("%s - %d-week lookahead%s", base, w.Weeks, ext)
}
