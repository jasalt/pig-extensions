// Copyright (c) 2025 Alessandro Pungitore
// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package schedule

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	msMinute = int64(60_000)
	msHour   = int64(3_600_000)
	msDay    = int64(86_400_000)
	maxOnce  = 90 * msDay
)

// parseError is a user-facing schedule parsing error.
type parseError struct{ msg string }

func (e parseError) Error() string { return e.msg }

var (
	spaceRun     = regexp.MustCompile(`\s+`)
	dailyPattern = regexp.MustCompile(`^(?:daily\s+(?:at\s+)?)?(\d{1,2}):(\d{2})$`)
	atPattern    = regexp.MustCompile(`^at\s+(\d{1,2}):(\d{2})$`)
	intervalForm = regexp.MustCompile(`^(?:every\s+)?(\d+)\s*([mhd])$`)
	oncePattern  = regexp.MustCompile(`^(?:in|once)\s+(\d+)\s*([smhd])$`)
	unitMs       = map[string]int64{"s": 1_000, "m": msMinute, "h": msHour, "d": msDay}
)

// parseSchedule accepts "every 30m", "30m", "daily at 09:00", "at 09:00",
// "09:00", "in 10m" and "once 30s".
func parseSchedule(input string) (Spec, error) {
	raw := spaceRun.ReplaceAllString(strings.ToLower(strings.TrimSpace(input)), " ")
	if raw == "" {
		return Spec{}, parseError{"Empty schedule string"}
	}
	if spec, ok, err := parseDaily(raw); ok || err != nil {
		return spec, err
	}
	if spec, ok, err := parseOnce(raw); ok || err != nil {
		return spec, err
	}
	if spec, ok, err := parseInterval(raw); ok || err != nil {
		return spec, err
	}
	return Spec{}, parseError{fmt.Sprintf(`Unrecognized schedule %q. Use e.g. "every 30m", "every 2h", "every 1d", "daily at 09:00", or "in 10m".`, input)}
}

// scheduleFromParts builds a spec from exactly one of every/dailyAt/once.
func scheduleFromParts(every, dailyAt, once string) (Spec, error) {
	every, dailyAt, once = strings.TrimSpace(every), strings.TrimSpace(dailyAt), strings.TrimSpace(once)
	count := 0
	for _, part := range []string{every, dailyAt, once} {
		if part != "" {
			count++
		}
	}
	if count != 1 {
		return Spec{}, parseError{`Provide exactly one of "every" (e.g. "30m"), "dailyAt" (e.g. "09:00"), or "once" (e.g. "10m").`}
	}
	switch {
	case every != "":
		return parseSchedule("every " + every)
	case dailyAt != "":
		return parseSchedule("daily at " + dailyAt)
	}
	return parseSchedule("in " + once)
}

func parseDaily(raw string) (Spec, bool, error) {
	m := dailyPattern.FindStringSubmatch(raw)
	if m == nil {
		m = atPattern.FindStringSubmatch(raw)
	}
	if m == nil {
		return Spec{}, false, nil
	}
	hour, _ := strconv.Atoi(m[1])
	minute, _ := strconv.Atoi(m[2])
	if hour > 23 {
		return Spec{}, true, parseError{fmt.Sprintf("Invalid hour in %q (expected 0-23)", raw)}
	}
	if minute > 59 {
		return Spec{}, true, parseError{fmt.Sprintf("Invalid minute in %q (expected 0-59)", raw)}
	}
	return Spec{Type: "daily", Hour: hour, Minute: minute, At: fmt.Sprintf("%02d:%02d", hour, minute)}, true, nil
}

// positiveCount mirrors Number(digits): huge digit strings become large
// numbers rather than errors.
func positiveCount(digits string) float64 {
	n, err := strconv.ParseFloat(digits, 64)
	if err != nil {
		return math.Inf(1)
	}
	return n
}

func parseInterval(raw string) (Spec, bool, error) {
	m := intervalForm.FindStringSubmatch(raw)
	if m == nil {
		return Spec{}, false, nil
	}
	n := positiveCount(m[1])
	if n < 1 {
		return Spec{}, true, parseError{fmt.Sprintf("Interval must be a positive integer: %q", raw)}
	}
	everyMs := n * float64(unitMs[m[2]])
	if everyMs < 60_000 {
		return Spec{}, true, parseError{"Minimum interval is 1m"}
	}
	if everyMs > float64(90*msDay) {
		return Spec{}, true, parseError{"Maximum interval is 90d"}
	}
	return Spec{Type: "interval", EveryMs: int64(everyMs), Every: strconv.FormatInt(int64(n), 10) + m[2]}, true, nil
}

func parseOnce(raw string) (Spec, bool, error) {
	m := oncePattern.FindStringSubmatch(raw)
	if m == nil {
		return Spec{}, false, nil
	}
	n := positiveCount(m[1])
	if n < 1 {
		return Spec{}, true, parseError{fmt.Sprintf("Once delay must be a positive integer: %q", raw)}
	}
	delayMs := n * float64(unitMs[m[2]])
	if delayMs > float64(maxOnce) {
		return Spec{}, true, parseError{"Maximum once delay is 90d"}
	}
	return Spec{Type: "once", DelayMs: int64(delayMs), Delay: strconv.FormatInt(int64(n), 10) + m[2]}, true, nil
}

// computeNextRunAt returns the next run strictly after from, or at/after it
// when inclusive (first scheduling of a daily job). ok is false for an
// unreadable spec.
func computeNextRunAt(spec Spec, from time.Time, inclusive bool) (time.Time, bool) {
	switch spec.Type {
	case "interval":
		if inclusive {
			return from, true
		}
		return from.Add(time.Duration(spec.EveryMs) * time.Millisecond), true
	case "once":
		if inclusive {
			return from, true
		}
		return from.Add(time.Duration(spec.DelayMs) * time.Millisecond), true
	case "daily":
		return nextDaily(spec, from, inclusive), true
	}
	return time.Time{}, false
}

func nextDaily(spec Spec, from time.Time, inclusive bool) time.Time {
	local := from.In(time.Local)
	candidate := localWallClock(local, spec.Hour, spec.Minute)
	if (inclusive && !candidate.Before(from)) || (!inclusive && candidate.After(from)) {
		return candidate
	}
	return localWallClock(local.AddDate(0, 0, 1), spec.Hour, spec.Minute)
}

// localWallClock returns hour:minute on base's local calendar day. A time
// inside a spring-forward gap moves to the next day where it exists, as the
// original's setHours loop does. In a fall-back overlap the earlier instant
// is used.
func localWallClock(base time.Time, hour, minute int) time.Time {
	y, m, d := base.Date()
	for guard := 0; guard <= 48; guard++ {
		t := time.Date(y, m, d+guard, hour, minute, 0, 0, time.Local)
		if t.Hour() == hour && t.Minute() == minute {
			// Prefer the earlier of two instants with the same wall clock.
			if earlier := t.Add(-time.Hour); earlier.Hour() == hour && earlier.Minute() == minute && earlier.Day() == t.Day() {
				return earlier
			}
			return t
		}
	}
	return time.Date(y, m, d, hour, minute, 0, 0, time.Local)
}

func formatSchedule(spec Spec) string {
	switch spec.Type {
	case "interval":
		return "every " + spec.Every
	case "once":
		return "once in " + spec.Delay
	case "daily":
		return "daily at " + spec.At
	}
	return "invalid schedule"
}

// jsRound is Math.round: halves round toward +Infinity.
func jsRound(x float64) int64 { return int64(math.Floor(x + 0.5)) }

// formatRelative renders "in 5m", "3h ago", "just now" like the original.
func formatRelative(iso string, now time.Time) string {
	t, ok := parseISO(iso)
	if !ok {
		return "unknown"
	}
	delta := t.Sub(now).Milliseconds()
	abs := delta
	if abs < 0 {
		abs = -abs
	}
	past := delta < 0
	if abs < 60_000 {
		if past {
			return "just now"
		}
		return "in <1m"
	}
	unit := func(n int64, suffix string) string {
		if past {
			return fmt.Sprintf("%d%s ago", n, suffix)
		}
		return fmt.Sprintf("in %d%s", n, suffix)
	}
	if minutes := jsRound(float64(abs) / 60_000); minutes < 60 {
		return unit(minutes, "m")
	}
	if abs < 48*msHour {
		return unit(jsRound(float64(abs)/float64(msHour)), "h")
	}
	return unit(jsRound(float64(abs)/float64(msDay)), "d")
}
