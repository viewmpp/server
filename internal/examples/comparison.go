package examples

import (
	"fmt"
	"server/internal/contract"
)

type Pair struct {
	PreviousName string
	CurrentName  string
	Previous     *contract.Contract
	Current      *contract.Contract
	PreviousRaw  []byte
	CurrentRaw   []byte
}

const (
	pairPrevious = "comparison-before.json"
	pairCurrent  = "comparison-after.json"
)

func ComparisonPair() (Pair, error) {
	p := Pair{
		PreviousName: "Level 6 refurbishment - week 1.mpp",
		CurrentName:  "Level 6 refurbishment - week 3.mpp",
	}

	var err error
	if p.PreviousRaw, p.Previous, err = readPlan(pairPrevious); err != nil {
		return Pair{}, err
	}
	if p.CurrentRaw, p.Current, err = readPlan(pairCurrent); err != nil {
		return Pair{}, err
	}

	return p, nil
}

func readPlan(name string) ([]byte, *contract.Contract, error) {
	raw, err := files.ReadFile(name)
	if err != nil {
		return nil, nil, fmt.Errorf("comparison example %s: %w", name, err)
	}

	c, err := contract.Decode(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("comparison example %s: %w", name, err)
	}

	return raw, c, nil
}
