package automats

import "strconv"

type block struct {
	states map[int]bool
}

func acceptKey(accept *AcceptInfo) string {
	if accept == nil {
		return "-"
	}
	return accept.Token + ":" + strconv.FormatBool(accept.Skip) + ":" +
		strconv.FormatBool(accept.Error)
}

func Minimize(dfa *DFA) *DFA {
	byAccept := make(map[string]*block)
	var groups []*block
	for state, item := range dfa.States {
		key := acceptKey(item.Accept)
		group := byAccept[key]
		if group == nil {
			group = &block{states: make(map[int]bool)}
			byAccept[key] = group
			groups = append(groups, group)
		}
		group.states[state] = true
	}

	work := append([]*block(nil), groups...)
	for len(work) > 0 {
		chosen := work[0]
		work = work[1:]

		for symbol := 0; symbol < ASCII; symbol++ {
			// Предшественники chosen по одному символу.
			previous := make(map[int]bool)
			for state, item := range dfa.States {
				if chosen.states[item.Transitions[symbol]] {
					previous[state] = true
				}
			}

			var nextGroups []*block
			for _, group := range groups {
				inside := &block{states: make(map[int]bool)}
				outside := &block{states: make(map[int]bool)}
				for state := range group.states {
					if previous[state] {
						inside.states[state] = true
					} else {
						outside.states[state] = true
					}
				}

				if len(inside.states) == 0 || len(outside.states) == 0 {
					nextGroups = append(nextGroups, group)
					continue
				}

				nextGroups = append(nextGroups, inside, outside)
				inWork := false
				for i, pending := range work {
					if pending == group {
						work[i] = inside
						work = append(work, outside)
						inWork = true
						break
					}
				}
				if !inWork {
					if len(inside.states) < len(outside.states) {
						work = append(work, inside)
					} else {
						work = append(work, outside)
					}
				}
			}
			groups = nextGroups
		}
	}

	oldToGroup := make([]int, len(dfa.States))
	for groupID, group := range groups {
		for state := range group.states {
			oldToGroup[state] = groupID
		}
	}

	// Нумеруем новые состояния обходом от старта.
	startGroup := oldToGroup[dfa.Start]
	newIDs := map[int]int{startGroup: 0}
	queue := []int{startGroup}
	result := &DFA{Start: 0, States: []DFAState{{}}}
	for i := 0; i < len(queue); i++ {
		groupID := queue[i]
		newID := newIDs[groupID]
		var representative int
		for state := range groups[groupID].states {
			representative = state
			break
		}
		result.States[newID].Accept = dfa.States[representative].Accept
		for symbol := 0; symbol < ASCII; symbol++ {
			oldTarget := dfa.States[representative].Transitions[symbol]
			targetGroup := oldToGroup[oldTarget]
			targetID, found := newIDs[targetGroup]
			if !found {
				targetID = len(queue)
				newIDs[targetGroup] = targetID
				queue = append(queue, targetGroup)
				result.States = append(result.States, DFAState{})
			}
			result.States[newID].Transitions[symbol] = targetID
		}
	}
	result.Trap = newIDs[oldToGroup[dfa.Trap]]
	return result
}
