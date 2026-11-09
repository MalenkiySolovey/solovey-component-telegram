//go:build !minimal

package settings

import (
	"github.com/MalenkiySolovey/solovey-component-telegram/internal/schedule"
	"github.com/robfig/cron/v3"
)

// ParseSchedule exposes the existing feature cron contract to owner adapters.
func ParseSchedule(spec string) (cron.Schedule, error) { return schedule.Parse(spec) }
