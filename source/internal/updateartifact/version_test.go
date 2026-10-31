package updateartifact

import "testing"

func TestSemVerOrdering(t *testing.T) {
	ordered := []string{
		"0.0.0", "1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta",
		"1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "1.0.1", "1.1.0", "2.0.0",
		"18446744073709551616.0.0", "18446744073709551617.0.0",
	}
	for leftIndex, leftText := range ordered {
		left, err := parseVersion(leftText)
		if err != nil {
			t.Fatal(err)
		}
		for rightIndex, rightText := range ordered {
			right, err := parseVersion(rightText)
			if err != nil {
				t.Fatal(err)
			}
			order := left.compare(right)
			if leftIndex < rightIndex && order >= 0 || leftIndex > rightIndex && order <= 0 || leftIndex == rightIndex && order != 0 {
				t.Errorf("unexpected ordering %s vs %s: %d", leftText, rightText, order)
			}
		}
	}
	for _, pair := range [][2]string{{"1.0.0+build.01", "1.0.0+another.2"}, {"1.0.0-alpha+build", "1.0.0-alpha"}} {
		left, _ := parseVersion(pair[0])
		right, _ := parseVersion(pair[1])
		if left.compare(right) != 0 {
			t.Errorf("build metadata affected precedence: %v", pair)
		}
	}
}

func TestSemVerRejectsInvalidVersions(t *testing.T) {
	for _, value := range []string{
		"", "v1.0.0", "1", "1.0", "1.0.0.0", "01.0.0", "1.00.0", "1.0.00", "-1.0.0", "+1.0.0",
		"1.0.0-", "1.0.0-01", "1.0.0-alpha.01", "1.0.0-alpha..1", "1.0.0+", "1.0.0+build..1",
		"1.0.0+build+two", "1.0.0-α", "1.0.0-alpha_1", " 1.0.0", "1.0.0\n", "1.0.0/escape",
	} {
		t.Run(value, func(t *testing.T) {
			if _, err := parseVersion(value); err == nil {
				t.Errorf("invalid semantic version accepted: %q", value)
			}
		})
	}
}
