package contract

import (
	"strings"
	"testing"
)

const statusPlan = `{"contract_version":1,"project":{"name":null,"start":"2026-03-02T08:00:00","finish":"2026-06-12T17:00:00"%s},` +
	`"calendar":{"name":null,"non_working_weekdays":[],"exceptions":[]},"resources":[],"tasks":[],"relations":[]}`

func TestAStatusDateFromTheParserIsAccepted(t *testing.T) {
	for name, field := range map[string]string{
		"absent, as in contracts saved before it existed": ``,
		"null, as the parser sends it without one":        `,"status_date":null`,
		"a P6 data date": `,"status_date":"2026-04-10T00:00:00"`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode([]byte(strings.Replace(statusPlan, "%s", field, 1))); err != nil {
				t.Errorf("refused: %v", err)
			}
		})
	}
}

func TestAMalformedStatusDateIsRefused(t *testing.T) {
	if _, err := Decode([]byte(strings.Replace(statusPlan, "%s", `,"status_date":"next friday"`, 1))); err == nil {
		t.Error("a status date that is not a date was accepted")
	}
}
