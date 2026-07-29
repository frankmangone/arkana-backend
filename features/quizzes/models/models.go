package models

import "encoding/json"

type QuestionTranslationPayload struct {
	Prompt  string          `json:"prompt"`
	Content json.RawMessage `json:"content"`
}

type QuestionPayload struct {
	Slug         string                                `json:"slug"`
	Type         string                                `json:"type"`
	Difficulty   int                                   `json:"difficulty"`
	PostPaths    []string                              `json:"post_paths"`
	Tags         []string                              `json:"tags"`
	AnswerKey    json.RawMessage                       `json:"answer_key"`
	Translations map[string]QuestionTranslationPayload `json:"translations"`
}

type QuestionPublishRequest struct {
	Questions []QuestionPayload `json:"questions"`
}

type QuestionPublishResponse struct {
	Published int `json:"published"`
}

type StartAttemptResponse struct {
	AttemptID      string `json:"attemptId"`
	TotalQuestions int    `json:"totalQuestions"`
}

type QuestionDTO struct {
	UUID       string          `json:"uuid"`
	Type       string          `json:"type"`
	Difficulty int             `json:"difficulty"`
	Prompt     string          `json:"prompt"`
	Content    json.RawMessage `json:"content"`
}

type NextQuestionResponse struct {
	Question       *QuestionDTO `json:"question"`
	Position       int          `json:"position"`
	TotalQuestions int          `json:"totalQuestions"`
	Done           bool         `json:"done"`
}

type AnswerRequest struct {
	QuestionID string          `json:"questionId"`
	Response   json.RawMessage `json:"response,omitempty"`
	Skipped    bool            `json:"skipped,omitempty"`
}

type ReinforcementDTO struct {
	PostPaths []string `json:"postPaths"`
}

type AnswerResponse struct {
	Correct       bool              `json:"correct"`
	Skipped       bool              `json:"skipped"`
	CorrectReveal json.RawMessage   `json:"correctReveal,omitempty"`
	Explanation   *string           `json:"explanation,omitempty"`
	Reinforcement *ReinforcementDTO `json:"reinforcement,omitempty"`
	AttemptDone   bool              `json:"attemptDone"`
}
