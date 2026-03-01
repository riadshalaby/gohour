package classify

import "gohour/onepoint"

// ClassifySubmitWorklogs splits local candidates by submit outcome against
// existing remote payload values.
func ClassifySubmitWorklogs(local, existing []onepoint.PersistWorklog) ([]onepoint.PersistWorklog, []onepoint.OverlapInfo, int) {
	toAdd := make([]onepoint.PersistWorklog, 0, len(local))
	overlaps := make([]onepoint.OverlapInfo, 0)
	duplicates := 0

	for _, candidate := range local {
		isDuplicate := false
		for _, existingEntry := range existing {
			if onepoint.PersistWorklogsEquivalent(existingEntry, candidate) {
				isDuplicate = true
				break
			}
		}
		if isDuplicate {
			duplicates++
			continue
		}

		hasOverlap := false
		for _, existingEntry := range existing {
			if onepoint.WorklogTimeOverlaps(candidate, existingEntry) {
				overlaps = append(overlaps, onepoint.OverlapInfo{
					Local:    candidate,
					Existing: existingEntry,
				})
				hasOverlap = true
				break
			}
		}
		if hasOverlap {
			continue
		}

		toAdd = append(toAdd, candidate)
	}

	return toAdd, overlaps, duplicates
}
