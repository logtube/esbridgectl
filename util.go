package main

import (
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	indexDatePattern = regexp.MustCompile(`(\d{4})[._-]?(\d{2})[._-]?(\d{2})$`)
)

func dateFromIndex(index string) (date time.Time, ok bool) {
	matches := indexDatePattern.FindStringSubmatch(filepath.Base(index))
	if len(matches) == 0 {
		return
	}
	year, _ := strconv.Atoi(matches[1])
	month, _ := strconv.Atoi(matches[2])
	day, _ := strconv.Atoi(matches[3])
	date = time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
	ok = true
	return
}

func dateMidnight(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

func removeFromStrSlice(indices []string, index string) []string {
	out := make([]string, 0, len(indices))
	for _, index0 := range indices {
		if index0 == index {
			continue
		}
		out = append(out, index0)
	}
	return out
}

var (
	deferIndices = []string{
		"info-prod-",
		"info-production-",
		"access-prod-",
		"access-production-",
		"-prod",
	}
)

func sortCandidateIndices(indices []string) {
	sort.SliceStable(indices, func(i, j int) bool {
		for _, di := range deferIndices {
			if strings.Contains(strings.ToLower(indices[j]), di) {
				if !strings.Contains(strings.ToLower(indices[i]), di) {
					return true
				}
			} else {
				if strings.Contains(strings.ToLower(indices[i]), di) {
					return false
				}
			}
		}
		if len(indices[i]) > 10 && len(indices[j]) > 10 {
			return indices[j][len(indices[j])-10:] > indices[i][len(indices[i])-10:]
		}
		return indices[j] > indices[i]
	})
}
