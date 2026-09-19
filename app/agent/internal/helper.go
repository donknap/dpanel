package internal

import (
	"fmt"
	"strings"
)

func ParseCapabilities(value string) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	if strings.TrimSpace(value) == "" {
		return result, nil
	}
	for _, item := range strings.Split(value, ",") {
		capability := strings.TrimSpace(item)
		switch capability {
		case "fs", "usage", "stat":
			result[capability] = struct{}{}
		case "":
			return nil, fmt.Errorf("invalid DP_AGENT_CAPS: empty capability")
		default:
			return nil, fmt.Errorf("invalid DP_AGENT_CAPS: unknown capability %q", capability)
		}
	}
	return result, nil
}
