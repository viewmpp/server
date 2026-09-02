package examples

import (
	"embed"
	"encoding/json"
	"fmt"
	"server/internal/contract"
	"sync"
	"time"
)

//go:embed *.json
var files embed.FS

type Example struct {
	Name     string
	Label    string
	Note     string
	FileName string
	anchor   time.Time
}

func anchoredOn(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

var catalogue = []Example{
	{
		Name:     "office-fit-out",
		Label:    "Office fit-out",
		Note:     "62 tasks, a critical path under pressure and four jobs already late",
		FileName: "Fenchurch St - level 4 fit-out.mpp",
		anchor:   anchoredOn(2026, time.June, 18),
	},
	{
		Name:     "viaduct",
		Label:    "Виадук (Cyrillic)",
		Note:     "non-Latin names throughout, Russian holidays shaded, a milestone missed",
		FileName: "Виадук через Каменку - этап 2.mpp",
		anchor:   anchoredOn(2026, time.July, 15),
	},
	{
		Name:     "wms-rollout",
		Label:    "System rollout",
		Note:     "57 tasks across build, data, testing and cutover",
		FileName: "WMS rollout - Tilbury DC.mpp",
		anchor:   anchoredOn(2026, time.May, 13),
	},
}

func All() []Example {
	return catalogue
}

func ByName(name string) (Example, bool) {
	for _, e := range catalogue {
		if e.Name == name {
			return e, true
		}
	}
	return Example{}, false
}

func weekStart(t time.Time) time.Time {
	t = t.UTC()
	back := (int(t.Weekday()) + 6) % 7
	return time.Date(t.Year(), t.Month(), t.Day()-back, 0, 0, 0, 0, time.UTC)
}

func NextWeek(now time.Time) time.Time {
	return weekStart(now).AddDate(0, 0, 7)
}

var (
	mu     sync.Mutex
	cached = map[string][]byte{}
)

func (e Example) Contract(now time.Time) ([]byte, error) {
	days := int(weekStart(now).Sub(weekStart(e.anchor)).Hours() / 24)
	key := fmt.Sprintf("%s@%d", e.Name, days)

	mu.Lock()
	defer mu.Unlock()

	if ready, ok := cached[key]; ok {
		return ready, nil
	}

	raw, err := files.ReadFile(e.Name + ".json")
	if err != nil {
		return nil, err
	}

	c, err := contract.Decode(raw)
	if err != nil {
		return nil, err
	}

	shift(c, days)

	moved, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}

	cached[key] = moved

	return moved, nil
}

func shift(c *contract.Contract, days int) {
	move := func(v *string) {
		if v == nil {
			return
		}
		t, err := time.Parse(contract.DateLayout, *v)
		if err != nil {
			return
		}
		*v = t.AddDate(0, 0, days).Format(contract.DateLayout)
	}

	move(c.Project.Start)
	move(c.Project.Finish)

	for i := range c.Calendar.Exceptions {
		move(c.Calendar.Exceptions[i].From)
		move(c.Calendar.Exceptions[i].To)
	}

	for i := range c.Tasks {
		move(c.Tasks[i].Start)
		move(c.Tasks[i].Finish)

		if b := c.Tasks[i].Baseline; b != nil {
			move(b.Start)
			move(b.Finish)
		}
	}
}
