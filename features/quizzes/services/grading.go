package services

import (
	"encoding/json"
	"errors"
)

var ErrUnknownQuestionType = errors.New("unknown question type")

// grade dispatches to the type-specific comparison function, returning
// whether response matches answerKey and the type-specific "what was
// actually right" reveal payload. Both response and answerKey/reveal use
// stable content ids, never array positions - this matters specifically
// for fill_blank, where the frontend's internal state tracks a wordBank
// array index, but the wire response carries the answered word string
// instead, decoupling grading from array order.
func grade(qType string, answerKey, response json.RawMessage) (correct bool, reveal json.RawMessage, err error) {
	switch qType {
	case "single_choice", "multi_choice":
		return gradeChoice(answerKey, response)
	case "matching", "bucket_sort":
		return gradeAssignments(answerKey, response)
	case "range":
		return gradeRange(answerKey, response)
	case "sequencing":
		return gradeSequencing(answerKey, response)
	case "fill_blank":
		return gradeFillBlank(answerKey, response)
	default:
		return false, nil, ErrUnknownQuestionType
	}
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, v := range a {
		counts[v]++
	}
	for _, v := range b {
		counts[v]--
	}
	for _, n := range counts {
		if n != 0 {
			return false
		}
	}
	return true
}

type choiceKey struct {
	CorrectOptionIDs []string `json:"correctOptionIds"`
}
type choiceResponse struct {
	SelectedOptionIDs []string `json:"selectedOptionIds"`
}

func gradeChoice(answerKey, response json.RawMessage) (bool, json.RawMessage, error) {
	var key choiceKey
	if err := json.Unmarshal(answerKey, &key); err != nil {
		return false, nil, err
	}
	var resp choiceResponse
	if err := json.Unmarshal(response, &resp); err != nil {
		return false, nil, err
	}
	reveal, err := json.Marshal(map[string][]string{"correctOptionIds": key.CorrectOptionIDs})
	return sameSet(key.CorrectOptionIDs, resp.SelectedOptionIDs), reveal, err
}

type assignmentsKey struct {
	CorrectAssignments map[string]string `json:"correctAssignments"`
}
type assignmentsResponse struct {
	Assignments map[string]string `json:"assignments"`
}

// gradeAssignments backs both matching and bucket_sort - identical wire
// shape (a map of id -> id), so one implementation covers both types.
func gradeAssignments(answerKey, response json.RawMessage) (bool, json.RawMessage, error) {
	var key assignmentsKey
	if err := json.Unmarshal(answerKey, &key); err != nil {
		return false, nil, err
	}
	var resp assignmentsResponse
	if err := json.Unmarshal(response, &resp); err != nil {
		return false, nil, err
	}
	reveal, err := json.Marshal(map[string]map[string]string{"correctAssignments": key.CorrectAssignments})
	if err != nil {
		return false, nil, err
	}
	if len(key.CorrectAssignments) != len(resp.Assignments) {
		return false, reveal, nil
	}
	for id, want := range key.CorrectAssignments {
		if resp.Assignments[id] != want {
			return false, reveal, nil
		}
	}
	return true, reveal, nil
}

type rangeValue struct {
	Value     float64 `json:"value"`
	Tolerance float64 `json:"tolerance"`
}
type rangeKey struct {
	CorrectValues map[string]rangeValue `json:"correctValues"`
}
type rangeResponse struct {
	Values map[string]float64 `json:"values"`
}

func gradeRange(answerKey, response json.RawMessage) (bool, json.RawMessage, error) {
	var key rangeKey
	if err := json.Unmarshal(answerKey, &key); err != nil {
		return false, nil, err
	}
	var resp rangeResponse
	if err := json.Unmarshal(response, &resp); err != nil {
		return false, nil, err
	}
	reveal, err := json.Marshal(map[string]map[string]rangeValue{"correctValues": key.CorrectValues})
	if err != nil {
		return false, nil, err
	}
	for id, want := range key.CorrectValues {
		got, ok := resp.Values[id]
		if !ok {
			return false, reveal, nil
		}
		diff := got - want.Value
		if diff < 0 {
			diff = -diff
		}
		if diff > want.Tolerance {
			return false, reveal, nil
		}
	}
	return true, reveal, nil
}

type sequencingKey struct {
	CorrectOrder []string `json:"correctOrder"`
}
type sequencingResponse struct {
	Order []string `json:"order"`
}

func gradeSequencing(answerKey, response json.RawMessage) (bool, json.RawMessage, error) {
	var key sequencingKey
	if err := json.Unmarshal(answerKey, &key); err != nil {
		return false, nil, err
	}
	var resp sequencingResponse
	if err := json.Unmarshal(response, &resp); err != nil {
		return false, nil, err
	}
	reveal, err := json.Marshal(map[string][]string{"correctOrder": key.CorrectOrder})
	if err != nil {
		return false, nil, err
	}
	if len(key.CorrectOrder) != len(resp.Order) {
		return false, reveal, nil
	}
	for i, want := range key.CorrectOrder {
		if resp.Order[i] != want {
			return false, reveal, nil
		}
	}
	return true, reveal, nil
}

type fillBlankKey struct {
	CorrectWords map[string]string `json:"correctWords"`
}
type fillBlankResponse struct {
	Filled map[string]string `json:"filled"`
}

func gradeFillBlank(answerKey, response json.RawMessage) (bool, json.RawMessage, error) {
	var key fillBlankKey
	if err := json.Unmarshal(answerKey, &key); err != nil {
		return false, nil, err
	}
	var resp fillBlankResponse
	if err := json.Unmarshal(response, &resp); err != nil {
		return false, nil, err
	}
	reveal, err := json.Marshal(map[string]map[string]string{"correctWords": key.CorrectWords})
	if err != nil {
		return false, nil, err
	}
	if len(key.CorrectWords) != len(resp.Filled) {
		return false, reveal, nil
	}
	for id, want := range key.CorrectWords {
		if resp.Filled[id] != want {
			return false, reveal, nil
		}
	}
	return true, reveal, nil
}
