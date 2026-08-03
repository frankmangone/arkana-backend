package services

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestGrade(t *testing.T) {
	cases := []struct {
		qType     string
		answerKey string
		response  string
		wantOK    bool
	}{
		{"single_choice", `{"correctOptionIds":["a"]}`, `{"selectedOptionIds":["a"]}`, true},
		{"multi_choice", `{"correctOptionIds":["a","b"]}`, `{"selectedOptionIds":["b","a"]}`, true},
		{"matching", `{"correctAssignments":{"1":"a"}}`, `{"assignments":{"1":"a"}}`, true},
		{"bucket_sort", `{"correctAssignments":{"1":"a"}}`, `{"assignments":{"1":"a"}}`, true},
		{"range", `{"correctValues":{"1":{"value":10,"tolerance":1}}}`, `{"values":{"1":10}}`, true},
		{"sequencing", `{"correctOrder":["a","b"]}`, `{"order":["a","b"]}`, true},
		{"fill_blank", `{"correctWords":{"1":"cat"}}`, `{"filled":{"1":"cat"}}`, true},
	}

	for _, c := range cases {
		t.Run(c.qType, func(t *testing.T) {
			correct, reveal, err := grade(c.qType, json.RawMessage(c.answerKey), json.RawMessage(c.response))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if correct != c.wantOK {
				t.Errorf("correct = %v, want %v", correct, c.wantOK)
			}
			if reveal == nil {
				t.Error("expected a non-nil reveal payload")
			}
		})
	}

	t.Run("unknown question type", func(t *testing.T) {
		_, _, err := grade("essay", json.RawMessage(`{}`), json.RawMessage(`{}`))
		if !errors.Is(err, ErrUnknownQuestionType) {
			t.Fatalf("err = %v, want ErrUnknownQuestionType", err)
		}
	})
}

func TestSameSet(t *testing.T) {
	cases := []struct {
		name string
		a, b []string
		want bool
	}{
		{"identical order", []string{"a", "b"}, []string{"a", "b"}, true},
		{"different order", []string{"a", "b"}, []string{"b", "a"}, true},
		{"both empty", nil, nil, true},
		{"different length", []string{"a"}, []string{"a", "b"}, false},
		{"different multiplicity", []string{"a", "a", "b"}, []string{"a", "b", "b"}, false},
		{"different elements", []string{"a", "b"}, []string{"a", "c"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sameSet(c.a, c.b); got != c.want {
				t.Errorf("sameSet(%v, %v) = %v, want %v", c.a, c.b, got, c.want)
			}
		})
	}
}

func TestGradeChoice(t *testing.T) {
	t.Run("selecting the right options in a different order still counts as correct", func(t *testing.T) {
		correct, reveal, err := gradeChoice(
			json.RawMessage(`{"correctOptionIds":["a","b"]}`),
			json.RawMessage(`{"selectedOptionIds":["b","a"]}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		if !correct {
			t.Error("expected correct = true")
		}
		var revealed map[string][]string
		if err := json.Unmarshal(reveal, &revealed); err != nil {
			t.Fatal(err)
		}
		if got := revealed["correctOptionIds"]; !sameSet(got, []string{"a", "b"}) {
			t.Errorf("revealed correctOptionIds = %v, want [a b]", got)
		}
	})

	t.Run("wrong selection is incorrect but still reveals the answer", func(t *testing.T) {
		correct, reveal, err := gradeChoice(
			json.RawMessage(`{"correctOptionIds":["a"]}`),
			json.RawMessage(`{"selectedOptionIds":["b"]}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		if correct {
			t.Error("expected correct = false")
		}
		if reveal == nil {
			t.Error("expected a reveal payload even when incorrect")
		}
	})

	t.Run("malformed response is a client error wrapping ErrMalformedResponse", func(t *testing.T) {
		_, _, err := gradeChoice(json.RawMessage(`{"correctOptionIds":["a"]}`), json.RawMessage(`not json`))
		if !errors.Is(err, ErrMalformedResponse) {
			t.Fatalf("err = %v, want it to wrap ErrMalformedResponse", err)
		}
	})

	t.Run("malformed answer key is not mistaken for a client error", func(t *testing.T) {
		_, _, err := gradeChoice(json.RawMessage(`not json`), json.RawMessage(`{"selectedOptionIds":["a"]}`))
		if err == nil {
			t.Fatal("expected an error")
		}
		if errors.Is(err, ErrMalformedResponse) {
			t.Fatal("a malformed answer key is a server-side data problem, not ErrMalformedResponse")
		}
	})
}

func TestGradeAssignments(t *testing.T) {
	t.Run("exact match is correct", func(t *testing.T) {
		correct, _, err := gradeAssignments(
			json.RawMessage(`{"correctAssignments":{"1":"a","2":"b"}}`),
			json.RawMessage(`{"assignments":{"1":"a","2":"b"}}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		if !correct {
			t.Error("expected correct = true")
		}
	})

	t.Run("one wrong assignment is incorrect", func(t *testing.T) {
		correct, _, err := gradeAssignments(
			json.RawMessage(`{"correctAssignments":{"1":"a","2":"b"}}`),
			json.RawMessage(`{"assignments":{"1":"a","2":"z"}}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		if correct {
			t.Error("expected correct = false")
		}
	})

	t.Run("extra assignments in the response count as incorrect", func(t *testing.T) {
		correct, _, err := gradeAssignments(
			json.RawMessage(`{"correctAssignments":{"1":"a"}}`),
			json.RawMessage(`{"assignments":{"1":"a","2":"b"}}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		if correct {
			t.Error("expected correct = false when the response has more entries than the key")
		}
	})

	t.Run("malformed response wraps ErrMalformedResponse", func(t *testing.T) {
		_, _, err := gradeAssignments(json.RawMessage(`{"correctAssignments":{}}`), json.RawMessage(`not json`))
		if !errors.Is(err, ErrMalformedResponse) {
			t.Fatalf("err = %v, want it to wrap ErrMalformedResponse", err)
		}
	})
}

func TestGradeRange(t *testing.T) {
	t.Run("value within tolerance is correct", func(t *testing.T) {
		correct, _, err := gradeRange(
			json.RawMessage(`{"correctValues":{"1":{"value":10,"tolerance":0.5}}}`),
			json.RawMessage(`{"values":{"1":10.4}}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		if !correct {
			t.Error("expected correct = true (within tolerance)")
		}
	})

	t.Run("value outside tolerance is incorrect", func(t *testing.T) {
		correct, _, err := gradeRange(
			json.RawMessage(`{"correctValues":{"1":{"value":10,"tolerance":0.5}}}`),
			json.RawMessage(`{"values":{"1":11}}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		if correct {
			t.Error("expected correct = false (outside tolerance)")
		}
	})

	t.Run("missing id in response is incorrect", func(t *testing.T) {
		correct, _, err := gradeRange(
			json.RawMessage(`{"correctValues":{"1":{"value":10,"tolerance":0.5}}}`),
			json.RawMessage(`{"values":{}}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		if correct {
			t.Error("expected correct = false when a keyed id is missing from the response")
		}
	})

	t.Run("malformed response wraps ErrMalformedResponse", func(t *testing.T) {
		_, _, err := gradeRange(json.RawMessage(`{"correctValues":{}}`), json.RawMessage(`not json`))
		if !errors.Is(err, ErrMalformedResponse) {
			t.Fatalf("err = %v, want it to wrap ErrMalformedResponse", err)
		}
	})
}

func TestGradeSequencing(t *testing.T) {
	t.Run("matching order is correct", func(t *testing.T) {
		correct, _, err := gradeSequencing(
			json.RawMessage(`{"correctOrder":["a","b","c"]}`),
			json.RawMessage(`{"order":["a","b","c"]}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		if !correct {
			t.Error("expected correct = true")
		}
	})

	t.Run("same elements in the wrong order is incorrect", func(t *testing.T) {
		correct, _, err := gradeSequencing(
			json.RawMessage(`{"correctOrder":["a","b","c"]}`),
			json.RawMessage(`{"order":["a","c","b"]}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		if correct {
			t.Error("expected correct = false: order matters for sequencing")
		}
	})

	t.Run("length mismatch is incorrect", func(t *testing.T) {
		correct, _, err := gradeSequencing(
			json.RawMessage(`{"correctOrder":["a","b"]}`),
			json.RawMessage(`{"order":["a"]}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		if correct {
			t.Error("expected correct = false on length mismatch")
		}
	})

	t.Run("malformed response wraps ErrMalformedResponse", func(t *testing.T) {
		_, _, err := gradeSequencing(json.RawMessage(`{"correctOrder":[]}`), json.RawMessage(`not json`))
		if !errors.Is(err, ErrMalformedResponse) {
			t.Fatalf("err = %v, want it to wrap ErrMalformedResponse", err)
		}
	})
}

func TestGradeFillBlank(t *testing.T) {
	t.Run("exact match is correct", func(t *testing.T) {
		correct, _, err := gradeFillBlank(
			json.RawMessage(`{"correctWords":{"1":"cat"}}`),
			json.RawMessage(`{"filled":{"1":"cat"}}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		if !correct {
			t.Error("expected correct = true")
		}
	})

	t.Run("different case is incorrect: no case-folding", func(t *testing.T) {
		correct, _, err := gradeFillBlank(
			json.RawMessage(`{"correctWords":{"1":"cat"}}`),
			json.RawMessage(`{"filled":{"1":"Cat"}}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		if correct {
			t.Error("expected correct = false: fill_blank compares words exactly, no case-folding")
		}
	})

	t.Run("length mismatch is incorrect", func(t *testing.T) {
		correct, _, err := gradeFillBlank(
			json.RawMessage(`{"correctWords":{"1":"cat","2":"dog"}}`),
			json.RawMessage(`{"filled":{"1":"cat"}}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		if correct {
			t.Error("expected correct = false on length mismatch")
		}
	})

	t.Run("malformed response wraps ErrMalformedResponse", func(t *testing.T) {
		_, _, err := gradeFillBlank(json.RawMessage(`{"correctWords":{}}`), json.RawMessage(`not json`))
		if !errors.Is(err, ErrMalformedResponse) {
			t.Fatalf("err = %v, want it to wrap ErrMalformedResponse", err)
		}
	})
}
