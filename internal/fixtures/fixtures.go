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

type Example struct {
	Name     string
	Label    string
	Note     string
	FileName string
	Contract []byte
}

var examples = []Example{
	{
		Name:     "office-fit-out",
		Label:    "Office fit-out",
		Note:     "nested phases, dependencies, critical path",
		FileName: "office-fit-out.mpp",
		Contract: mpp14Baseline,
	},
	{
		Name:     "viaduct",
		Label:    "Viaduct (Cyrillic)",
		Note:     "non-Latin task names render as they should",
		FileName: "виадук.mpp",
		Contract: cyrillic,
	},
	{
		Name:     "roadworks-xml",
		Label:    "Roadworks (.xml)",
		Note:     "Project XML, not only .mpp",
		FileName: "roadworks.xml",
		Contract: mspdi,
	},
}

func Examples() []Example {
	return examples
}

func ByName(name string) (Example, bool) {
	for _, e := range examples {
		if e.Name == name {
			return e, true
		}
	}
	return Example{}, false
}
