package services

import (
	"encoding/json"
	"errors"
	"fmt"
)

var ErrUnknownQuestionType = errors.New("unknown question type")

// ErrMalformedResponse wraps a json.Unmarshal failure on the learner's
// submitted response - a bad-input condition (the client sent a shape
// that doesn't match this question's type), not a server fault, so
// handlers map it to 400 rather than the default 500.
var ErrMalformedResponse = errors.New("response does not match the question's expected shape")

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

// sameSet reports whether a and b contain the same elements with the same
// multiplicities, ignoring order.
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

// gradeChoice grades single_choice and multi_choice questions by comparing
// the submitted selectedOptionIds against the key's correctOptionIds as an
// unordered set (sameSet) - selecting the right options in the wrong order
// still counts as correct. Returns whether they match and a reveal payload
// naming the correct option ids.
func gradeChoice(answerKey, response json.RawMessage) (bool, json.RawMessage, error) {
	var key choiceKey
	if err := json.Unmarshal(answerKey, &key); err != nil {
		return false, nil, err
	}
	var resp choiceResponse
	if err := json.Unmarshal(response, &resp); err != nil {
		return false, nil, fmt.Errorf("%w: %w", ErrMalformedResponse, err)
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
		return false, nil, fmt.Errorf("%w: %w", ErrMalformedResponse, err)
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

// gradeRange grades range questions: each submitted numeric value must fall
// within its answer key's tolerance of the expected value (missing an id
// from the response fails that value), and every id in the key must be
// satisfied for the question to count as correct. Returns whether it's
// fully correct and a reveal payload with the full correctValues map
// (expected value plus tolerance) for every id.
func gradeRange(answerKey, response json.RawMessage) (bool, json.RawMessage, error) {
	var key rangeKey
	if err := json.Unmarshal(answerKey, &key); err != nil {
		return false, nil, err
	}
	var resp rangeResponse
	if err := json.Unmarshal(response, &resp); err != nil {
		return false, nil, fmt.Errorf("%w: %w", ErrMalformedResponse, err)
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

// gradeSequencing grades sequencing questions by comparing the submitted
// order against correctOrder position by position - unlike gradeChoice,
// order matters here, so the same ids in a different sequence are wrong.
// Returns whether the order matches exactly and a reveal payload with the
// correct order.
func gradeSequencing(answerKey, response json.RawMessage) (bool, json.RawMessage, error) {
	var key sequencingKey
	if err := json.Unmarshal(answerKey, &key); err != nil {
		return false, nil, err
	}
	var resp sequencingResponse
	if err := json.Unmarshal(response, &resp); err != nil {
		return false, nil, fmt.Errorf("%w: %w", ErrMalformedResponse, err)
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

// gradeFillBlank grades fill_blank questions by comparing each blank's
// submitted word against the expected word exactly (string equality, no
// case-folding or whitespace trimming), requiring every blank in the key to
// be present and matching. Returns whether every blank is correct and a
// reveal payload with the full correctWords map.
func gradeFillBlank(answerKey, response json.RawMessage) (bool, json.RawMessage, error) {
	var key fillBlankKey
	if err := json.Unmarshal(answerKey, &key); err != nil {
		return false, nil, err
	}
	var resp fillBlankResponse
	if err := json.Unmarshal(response, &resp); err != nil {
		return false, nil, fmt.Errorf("%w: %w", ErrMalformedResponse, err)
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
