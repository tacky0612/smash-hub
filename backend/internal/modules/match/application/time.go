package application

import "time"

func parseTime(s string) (time.Time, error) { return time.Parse(time.RFC3339, s) }
