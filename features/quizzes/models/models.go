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
