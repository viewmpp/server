package xlsx

import "testing"

func TestDisposition(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plan.mpp", `attachment; filename="plan.xlsx"; filename*=UTF-8''plan.xlsx`},
		{"schedule.xml", `attachment; filename="schedule.xlsx"; filename*=UTF-8''schedule.xlsx`},
		{"виадук.mpp", `attachment; filename="______.xlsx"; filename*=UTF-8''%D0%B2%D0%B8%D0%B0%D0%B4%D1%83%D0%BA.xlsx`},
		{"", `attachment; filename="project.xlsx"; filename*=UTF-8''project.xlsx`},
		{`we"ird\.mpp`, `attachment; filename="we_ird_.xlsx"; filename*=UTF-8''we%22ird%5C.xlsx`},
	}

	for _, c := range cases {
		if got := Disposition(c.in); got != c.want {
			t.Errorf("Disposition(%q)\n got  %s\n want %s", c.in, got, c.want)
		}
	}
}
