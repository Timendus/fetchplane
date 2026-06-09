package sync

import "time"

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (s *Service) downloadCutoff(now time.Time) (time.Time, bool) {
	if s.cfg.MaxDaysToDownload <= 0 {
		return time.Time{}, false
	}
	return now.AddDate(0, 0, -s.cfg.MaxDaysToDownload), true
}

func (s *Service) removalCutoff(now time.Time) (time.Time, bool) {
	if s.cfg.RemoveOlderThanDays <= 0 {
		return time.Time{}, false
	}
	return now.AddDate(0, 0, -s.cfg.RemoveOlderThanDays), true
}
