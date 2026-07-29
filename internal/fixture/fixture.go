package fixture

import _ "embed"

var (
	//go:embed mpp14baseline.json
	mpp14Baseline []byte

	//go:embed cyrillic.json
	cyrillic []byte
)

type Fixture struct {
	FileName string
	Contract []byte
}

var byDemo = map[string]Fixture{
	"cyrillic": {FileName: "виадук.mpp", Contract: cyrillic},
}

var fallback = Fixture{FileName: "mpp14baseline.mpp", Contract: mpp14Baseline}

func ByDemo(demo string) Fixture {
	if f, ok := byDemo[demo]; ok {
		return f
	}
	return fallback
}
