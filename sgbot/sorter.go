package main

import "sort"

// By is the type of a "less" function that defines the ordering of its Time arguments.
// time sorter function
type By func(p1, p2 *GiveAway) bool

func (by By) sortGAs(entries []GiveAway) {
	ps := &timeSorter{
		entries: entries,
		by:      by, // The Sort method's receiver is the function (closure) that defines the sort order.
	}
	sort.Sort(ps)
}

// timeSorter joins a By function and a slice of Time to be sorted.
type timeSorter struct {
	entries []GiveAway
	by      func(p1, p2 *GiveAway) bool // Closure used in the Less method.
}

// Len is part of sort.Interface.
func (s *timeSorter) Len() int {
	return len(s.entries)
}

// Swap is part of sort.Interface.
func (s *timeSorter) Swap(i, j int) {
	s.entries[i], s.entries[j] = s.entries[j], s.entries[i]
}

// Less is part of sort.Interface. It is implemented by calling the "by" closure in the sorter.
func (s *timeSorter) Less(i, j int) bool {
	return s.by(&s.entries[i], &s.entries[j])
}
