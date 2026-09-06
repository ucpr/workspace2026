// Package depgraph implements dependency-graph operations over tasks:
// cycle detection, topological ordering, and Ready-task resolution
// (requirements §5.2).
package depgraph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ucpr/atama/internal/model"
)

// CycleError reports a dependency cycle detected among tasks.
type CycleError struct {
	// Path lists the task IDs forming the cycle, in dependency order,
	// repeating the first ID at the end (e.g. [a, b, c, a]).
	Path []string
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("dependency cycle detected: %s", strings.Join(e.Path, " -> "))
}

type visitState int

const (
	unvisited visitState = iota
	visiting
	visited
)

// FindCycle walks the depends_on edges of tasks and returns the first cycle
// found, or nil if the graph is acyclic. Edges pointing to IDs absent from
// tasks are ignored (an unknown dependency is a separate validation
// concern, not a cycle).
func FindCycle(tasks map[string]*model.Task) *CycleError {
	state := make(map[string]visitState, len(tasks))

	// Iterate in a stable order so error messages are deterministic for a
	// given input, regardless of map iteration order.
	ids := make([]string, 0, len(tasks))
	for id := range tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var path []string
	var cycle *CycleError

	var visit func(id string) bool // returns true to stop the walk
	visit = func(id string) bool {
		switch state[id] {
		case visited:
			return false
		case visiting:
			// Found a cycle: report the portion of path from id's first
			// occurrence onward.
			start := indexOf(path, id)
			cyclePath := append(append([]string{}, path[start:]...), id)
			cycle = &CycleError{Path: cyclePath}
			return true
		}

		state[id] = visiting
		path = append(path, id)

		task := tasks[id]
		if task != nil {
			deps := append([]string{}, task.DependsOn...)
			sort.Strings(deps)
			for _, dep := range deps {
				if _, ok := tasks[dep]; !ok {
					continue
				}
				if visit(dep) {
					return true
				}
			}
		}

		path = path[:len(path)-1]
		state[id] = visited
		return false
	}

	for _, id := range ids {
		if state[id] == unvisited {
			if visit(id) {
				return cycle
			}
		}
	}
	return nil
}

// WouldCreateCycle reports whether adding the edge "from depends_on to"
// would introduce a cycle, without mutating tasks. from and to need not
// already exist in tasks.
func WouldCreateCycle(tasks map[string]*model.Task, from, to string) bool {
	if from == to {
		return true
	}

	trial := make(map[string]*model.Task, len(tasks)+1)
	for id, t := range tasks {
		trial[id] = t
	}
	fromTask := &model.Task{}
	if existing, ok := tasks[from]; ok {
		*fromTask = *existing
	} else {
		fromTask.ID = from
	}
	fromTask.DependsOn = append(append([]string{}, fromTask.DependsOn...), to)
	trial[from] = fromTask
	if _, ok := trial[to]; !ok {
		trial[to] = &model.Task{ID: to}
	}

	return FindCycle(trial) != nil
}

// TopoSort returns ids ordered so that every task appears after all of its
// depends_on tasks that are also present in ids. Dependencies outside ids
// are ignored for ordering purposes. Returns a CycleError if the subset
// contains a cycle.
func TopoSort(tasks map[string]*model.Task, ids []string) ([]string, error) {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}

	subset := make(map[string]*model.Task, len(ids))
	for _, id := range ids {
		t := tasks[id]
		if t == nil {
			subset[id] = &model.Task{ID: id}
			continue
		}
		filtered := *t
		var deps []string
		for _, d := range t.DependsOn {
			if set[d] {
				deps = append(deps, d)
			}
		}
		filtered.DependsOn = deps
		subset[id] = &filtered
	}

	if cyc := FindCycle(subset); cyc != nil {
		return nil, cyc
	}

	sorted := make([]string, 0, len(ids))
	state := make(map[string]visitState, len(ids))

	ordered := append([]string{}, ids...)
	sort.Strings(ordered)

	var visit func(id string)
	visit = func(id string) {
		if state[id] == visited {
			return
		}
		state[id] = visited
		deps := append([]string{}, subset[id].DependsOn...)
		sort.Strings(deps)
		for _, dep := range deps {
			visit(dep)
		}
		sorted = append(sorted, id)
	}
	for _, id := range ordered {
		visit(id)
	}
	return sorted, nil
}

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}
