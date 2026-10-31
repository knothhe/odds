package updateartifact

import (
	"fmt"
	"strings"
)

type semanticVersion struct {
	core       [3]string
	prerelease []string
}

// parseVersion accepts the complete SemVer 2.0.0 grammar, without a v prefix.
// Numeric identifiers stay as strings so valid large versions cannot overflow.
func parseVersion(value string) (semanticVersion, error) {
	var result semanticVersion
	main, build, hasBuild := strings.Cut(value, "+")
	if hasBuild && !validIdentifiers(build, false) {
		return result, fmt.Errorf("invalid semantic version: %q", value)
	}
	core, prerelease, hasPrerelease := strings.Cut(main, "-")
	parts := strings.Split(core, ".")
	if len(parts) != len(result.core) {
		return result, fmt.Errorf("invalid semantic version: %q", value)
	}
	for index, part := range parts {
		if !numeric(part) || len(part) > 1 && part[0] == '0' {
			return result, fmt.Errorf("invalid semantic version: %q", value)
		}
		result.core[index] = part
	}
	if hasPrerelease {
		if !validIdentifiers(prerelease, true) {
			return result, fmt.Errorf("invalid semantic version: %q", value)
		}
		result.prerelease = strings.Split(prerelease, ".")
	}
	return result, nil
}

func numeric(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func validIdentifiers(value string, prerelease bool) bool {
	for _, identifier := range strings.Split(value, ".") {
		if identifier == "" {
			return false
		}
		for _, char := range identifier {
			if !(char >= '0' && char <= '9' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || char == '-') {
				return false
			}
		}
		if prerelease && numeric(identifier) && len(identifier) > 1 && identifier[0] == '0' {
			return false
		}
	}
	return true
}

func compareNumber(left, right string) int {
	if len(left) < len(right) {
		return -1
	}
	if len(left) > len(right) {
		return 1
	}
	return strings.Compare(left, right)
}

func (left semanticVersion) compare(right semanticVersion) int {
	for index := range left.core {
		if order := compareNumber(left.core[index], right.core[index]); order != 0 {
			return order
		}
	}
	if len(left.prerelease) == 0 {
		if len(right.prerelease) == 0 {
			return 0
		}
		return 1
	}
	if len(right.prerelease) == 0 {
		return -1
	}
	for index, part := range left.prerelease {
		if index == len(right.prerelease) {
			return 1
		}
		other := right.prerelease[index]
		partNumeric, otherNumeric := numeric(part), numeric(other)
		var order int
		switch {
		case partNumeric && otherNumeric:
			order = compareNumber(part, other)
		case partNumeric:
			order = -1
		case otherNumeric:
			order = 1
		default:
			order = strings.Compare(part, other)
		}
		if order != 0 {
			return order
		}
	}
	if len(left.prerelease) < len(right.prerelease) {
		return -1
	}
	return 0
}
