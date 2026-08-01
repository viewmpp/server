package fixtures

import _ "embed"

var (
	//go:embed mpp8.json
	mpp8 []byte

	//go:embed mpp9.json
	mpp9 []byte

	//go:embed mpp12.json
	mpp12 []byte

	//go:embed mpp14baseline.json
	mpp14Baseline []byte

	//go:embed cyrillic.json
	cyrillic []byte

	//go:embed mspdi.json
	mspdi []byte
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
