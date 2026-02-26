package jobs

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// MatchesCron checks if the given time matches a standard 5-field cron expression.
// Fields: minute hour day-of-month month day-of-week
// Supports: *, */N, N, N-M, comma-separated values.
func MatchesCron(expr string, t time.Time) bool {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return false
	}
	return matchField(fields[0], t.Minute(), 0, 59) &&
		matchField(fields[1], t.Hour(), 0, 23) &&
		matchField(fields[2], t.Day(), 1, 31) &&
		matchField(fields[3], int(t.Month()), 1, 12) &&
		matchField(fields[4], int(t.Weekday()), 0, 6)
}

func matchField(field string, val, min, max int) bool {
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if matchPart(part, val, min, max) {
			return true
		}
	}
	return false
}

func matchPart(part string, val, min, max int) bool {
	if part == "*" {
		return true
	}

	// */N — every N
	if strings.HasPrefix(part, "*/") {
		step, err := strconv.Atoi(part[2:])
		if err != nil || step <= 0 {
			return false
		}
		return val%step == 0
	}

	// N-M — range
	if strings.Contains(part, "-") {
		bounds := strings.SplitN(part, "-", 2)
		lo, err1 := strconv.Atoi(bounds[0])
		hi, err2 := strconv.Atoi(bounds[1])
		if err1 != nil || err2 != nil {
			return false
		}
		return val >= lo && val <= hi
	}

	// Exact value
	n, err := strconv.Atoi(part)
	if err != nil {
		return false
	}
	return val == n
}

// HumanReadable converts a cron expression to a human-friendly string.
func HumanReadable(expr string) string {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return expr
	}
	min, hr, dom, mon, dow := fields[0], fields[1], fields[2], fields[3], fields[4]

	// Every N minutes
	if strings.HasPrefix(min, "*/") && hr == "*" && dom == "*" && mon == "*" && dow == "*" {
		return fmt.Sprintf("Every %s min", min[2:])
	}
	// Every N hours
	if min == "0" && strings.HasPrefix(hr, "*/") && dom == "*" && mon == "*" && dow == "*" {
		return fmt.Sprintf("Every %sh", hr[2:])
	}
	// Daily at HH:MM
	if !strings.Contains(min, "*") && !strings.Contains(hr, "*") && dom == "*" && mon == "*" && dow == "*" {
		return fmt.Sprintf("Daily at %s:%02s", hr, min)
	}
	// Weekly
	if dom == "*" && mon == "*" && dow != "*" {
		days := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
		d, err := strconv.Atoi(dow)
		if err == nil && d >= 0 && d < 7 {
			return fmt.Sprintf("Weekly %s %s:%02s", days[d], hr, min)
		}
	}

	return expr
}
