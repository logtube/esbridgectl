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

func sortCandidateIndices(indices []string) []string {
	var indicesProd []string
	var indicesNoProd []string

	for _, index := range indices {
		if strings.Contains(strings.ToLower(index), "-prod") {
			indicesProd = append(indicesProd, index)
		} else {
			indicesNoProd = append(indicesNoProd, index)
		}
	}

	sortIndicesByDate(indicesProd)
	sortIndicesByDate(indicesNoProd)

	return append(indicesNoProd, indicesProd...)
}

func sortIndicesByDate(indices []string) {
	sort.Slice(indices, func(i, j int) bool {
		ti, oki := dateFromIndex(indices[i])
		tj, okj := dateFromIndex(indices[j])
		if oki && okj {
			return ti.Before(tj)
		}
		return indices[i] > indices[j]
	})
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
