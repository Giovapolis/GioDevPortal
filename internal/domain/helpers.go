package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	RoleAdmin  = "admin"
	RoleClient = "client"

	StatusActive    = "active"
	StatusPaused    = "paused"
	StatusCompleted = "completed"
	StatusArchived  = "archived"

	TaskBacklog = "backlog"
	TaskDoing   = "doing"
	TaskTesting = "testing"
	TaskDone    = "done"
)

var TaskStatuses = []string{TaskBacklog, TaskDoing, TaskTesting, TaskDone}

var TaskStatusLabels = map[string]string{
	TaskBacklog: "Backlog",
	TaskDoing:   "En Curso",
	TaskTesting: "Testeo",
	TaskDone:    "Terminado",
}

var ProjectStatusLabels = map[string]string{
	StatusActive:    "Activo",
	StatusPaused:    "En pausa",
	StatusCompleted: "Completado",
	StatusArchived:  "Archivado",
}

func Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func ParseTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func FormatDate(s string) string {
	t, ok := ParseTime(s)
	if !ok {
		return "—"
	}
	return t.Local().Format("02 Jan 2006")
}

func FormatDateTime(s string) string {
	t, ok := ParseTime(s)
	if !ok {
		return "—"
	}
	return t.Local().Format("02 Jan 2006 15:04")
}

func IsValidTaskStatus(s string) bool {
	for _, v := range TaskStatuses {
		if v == s {
			return true
		}
	}
	return false
}

func Money(amount float64, currency string) string {
	if currency == "" {
		currency = "MXN"
	}
	symbol := map[string]string{"USD": "$", "EUR": "€", "ARS": "$", "MXN": "MX$", "COP": "$"}[currency]
	if symbol == "" {
		symbol = currency + " "
	}
	return symbol + strconv.FormatFloat(amount, 'f', 2, 64)
}

func StatusBadgeClass(status string) string {
	switch status {
	case StatusActive, TaskDone:
		return "border-emerald-400/30 bg-emerald-400/10 text-emerald-300 shadow-[0_0_12px_rgba(52,211,153,0.12)]"
	case StatusPaused, TaskDoing:
		return "border-amber-400/30 bg-amber-400/10 text-amber-300 shadow-[0_0_12px_rgba(251,191,36,0.12)]"
	case StatusCompleted, TaskTesting:
		return "border-sky-400/30 bg-sky-400/10 text-sky-300 shadow-[0_0_12px_rgba(56,189,248,0.12)]"
	case StatusArchived, TaskBacklog:
		return "border-purple-400/25 bg-purple-500/10 text-purple-200"
	default:
		return "border-purple-400/25 bg-purple-500/10 text-purple-200"
	}
}

func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-' || r == '_' || r == '.':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "archivo"
	}
	return out
}

func Truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func FormatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
