// Package services implements the quiz feature's business logic: question
// publishing and validation (QuestionService, question_service.go), next-
// question selection over a module's question pool (QuestionSelector /
// WeightedRandomSelector, selector.go), type-specific answer grading
// (grade and its gradeChoice/gradeAssignments/gradeRange/gradeSequencing/
// gradeFillBlank helpers, grading.go), and the quiz attempt lifecycle -
// start or resume an attempt, serve the next question, grade an answer,
// and complete an attempt (QuizSessionService, this file).
package services

import (
	"arkana/features/quizzes/models"
	"arkana/features/quizzes/queries"
	"arkana/shared/idgen"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// questionsPerAttempt and passThreshold are deliberate implementation
// constants, not config - see docs/superpowers/specs/2026-07-29-quiz-data-model-design.md.
const questionsPerAttempt = 8
const passThreshold = 0.7

var ErrModuleNotFound = errors.New("module not found")
var ErrAttemptNotFound = errors.New("attempt not found")
var ErrAttemptForbidden = errors.New("attempt belongs to another user")
var ErrAttemptCompleted = errors.New("attempt already completed")
var ErrWrongQuestion = errors.New("questionId does not match the attempt's current question")
var ErrAttemptIncomplete = errors.New("not every question has been answered or skipped")

// QuestionDelivery is the correctness-stripped shape served over
// GET .../next - deliberately has no answer_key/correct-* field anywhere,
// same rule the DB schema already enforces on questions.answer_key, now
// also enforced explicitly on this read path.
type QuestionDelivery struct {
	UUID       string
	Type       string
	Difficulty int
	Prompt     string
	Content    json.RawMessage
}

type attemptRow struct {
	UUID        string
	CompletedAt *time.Time
}

type QuizSessionService struct {
	queries queries.QuizSessionQueries
}

// NewQuizSessionService constructs a QuizSessionService. db serves the
// question bank and reading-list joins; redisClient serves attempt/session
// state.
func NewQuizSessionService(db *sql.DB, redisClient *redis.Client) *QuizSessionService {
	return &QuizSessionService{queries: queries.NewRedisQuizSessionQueries(db, redisClient)}
}

// CanAttempt is a monetization gating stub (quiz_spec.md requirement
// 4.1) - always true today. The counting logic and its call site already
// exist so wiring in the real free/paid gate later is a one-function
// change, not a redesign.
func (s *QuizSessionService) CanAttempt(userID, moduleID int) (bool, error) {
	return true, nil
}

// Start is get-or-create: if the user already has an in-progress attempt
// for this module it's resumed (navigating away and back never burns a new
// attempt, and the POST is idempotent); otherwise it begins a new attempt,
// running the selector once and persisting its full pick-order into the
// attempt blob's Questions field - this is what makes every later Next()
// call a trivial, idempotent lookup instead of a re-run of selection logic.
func (s *QuizSessionService) Start(userID int, listSlug, moduleSlug string) (attemptUUID string, totalQuestions int, err error) {
	moduleID, err := s.resolveModuleID(listSlug, moduleSlug)
	if err != nil {
		return "", 0, err
	}

	// Resume before the CanAttempt gate - finishing an attempt already
	// started must never be blocked by a future attempt-count limit.
	existingUUID, err := s.queries.FindActiveAttemptUUID(userID, moduleID)
	if err != nil && !errors.Is(err, queries.ErrNotFound) {
		return "", 0, err
	}
	if err == nil {
		total, err := s.queries.TotalQuestions(existingUUID)
		if err != nil {
			return "", 0, err
		}
		return existingUUID, total, nil
	}

	if ok, err := s.CanAttempt(userID, moduleID); err != nil {
		return "", 0, err
	} else if !ok {
		return "", 0, errors.New("attempt not allowed")
	}

	pool, err := s.queries.QuestionPool(moduleID)
	if err != nil {
		return "", 0, err
	}

	selector := NewWeightedRandomSelector()
	var history []models.AnsweredQuestion
	var chosen []models.Question
	for {
		q, done := selector.Next(pool, history)
		if done {
			break
		}
		chosen = append(chosen, *q)
		history = append(history, models.AnsweredQuestion{QuestionID: q.ID})
	}

	uuid, err := idgen.NewV4()
	if err != nil {
		return "", 0, err
	}

	questionOrder := make([]int, len(chosen))
	for i, q := range chosen {
		questionOrder[i] = q.ID
	}
	if err := s.queries.CreateAttempt(uuid, moduleID, userID, questionOrder); err != nil {
		return "", 0, err
	}
	return uuid, len(chosen), nil
}

// resolveModuleID looks up a reading_list_modules.id from its public
// listSlug/moduleSlug pair, shared by Start and Availability so there's
// exactly one place that translates "not found" into ErrModuleNotFound.
func (s *QuizSessionService) resolveModuleID(listSlug, moduleSlug string) (int, error) {
	moduleID, err := s.queries.ResolveModuleID(listSlug, moduleSlug)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrModuleNotFound
	}
	if err != nil {
		return 0, err
	}
	return moduleID, nil
}

// Availability reports whether a module currently has any quiz questions
// at all, and if so, which languages have full translation coverage over
// the *current* pool - every question in questionPool(moduleID) must have
// a question_translations row for a language to count as available in it.
// This mirrors questionPool exactly (same query, same source of truth as
// Start/Next) so this can never drift from what a real attempt would try
// to serve, and avoids the untranslated-question failure loadQuestionDelivery
// would otherwise hit mid-session.
func (s *QuizSessionService) Availability(listSlug, moduleSlug string) (available bool, languages []string, err error) {
	moduleID, err := s.resolveModuleID(listSlug, moduleSlug)
	if err != nil {
		return false, nil, err
	}

	pool, err := s.queries.QuestionPool(moduleID)
	if err != nil {
		return false, nil, err
	}
	if len(pool) == 0 {
		return false, []string{}, nil
	}

	ids := make([]int, len(pool))
	for i, q := range pool {
		ids[i] = q.ID
	}

	languages, err = s.queries.LanguagesWithFullCoverage(ids)
	if err != nil {
		return false, nil, err
	}
	return true, languages, nil
}

// getOwnedAttempt loads an attempt's Redis-backed metadata by its public
// uuid and enforces that it belongs to userID - the actual guard against
// cross-user access; the opaque uuid is defense-in-depth, never a
// substitute for this check. Tasks 6-7 (Answer/Complete) call this same
// helper.
func (s *QuizSessionService) getOwnedAttempt(userID int, attemptUUID string) (*attemptRow, error) {
	ownerID, completedAt, err := s.queries.GetAttemptMeta(attemptUUID)
	if errors.Is(err, queries.ErrNotFound) {
		return nil, ErrAttemptNotFound
	}
	if err != nil {
		return nil, err
	}
	if ownerID != userID {
		return nil, ErrAttemptForbidden
	}
	return &attemptRow{UUID: attemptUUID, CompletedAt: completedAt}, nil
}

// Next returns the question at the attempt's current position (however
// many answers exist so far), stripped of every correct-answer field.
// Repeated calls without an intervening Answer() call return the
// identical question every time - nothing here advances state, only a
// graded or skipped answer does.
func (s *QuizSessionService) Next(userID int, attemptUUID, lang string) (question *QuestionDelivery, position, total int, done bool, err error) {
	attempt, err := s.getOwnedAttempt(userID, attemptUUID)
	if err != nil {
		return nil, 0, 0, false, err
	}
	if attempt.CompletedAt != nil {
		return nil, 0, 0, false, ErrAttemptCompleted
	}

	total, err = s.queries.TotalQuestions(attempt.UUID)
	if err != nil {
		return nil, 0, 0, false, err
	}
	answered, err := s.queries.AnsweredCount(attempt.UUID)
	if err != nil {
		return nil, 0, 0, false, err
	}
	if answered >= total {
		return nil, answered, total, true, nil
	}

	questionID, err := s.queries.QuestionIDAtPosition(attempt.UUID, answered)
	if err != nil {
		return nil, 0, 0, false, err
	}

	q, err := s.loadQuestionDelivery(questionID, lang)
	if err != nil {
		return nil, 0, 0, false, err
	}
	return q, answered, total, false, nil
}

// loadQuestionDelivery loads questionID's translated content for lang and
// strips its explanation field (via stripExplanation) before returning it
// as the correctness-stripped QuestionDelivery shape Next() serves.
func (s *QuizSessionService) loadQuestionDelivery(questionID int, lang string) (*QuestionDelivery, error) {
	uuid, qType, prompt, content, difficulty, err := s.queries.LoadQuestionDelivery(questionID, lang)
	if err != nil {
		return nil, err
	}
	stripped, err := stripExplanation([]byte(content))
	if err != nil {
		return nil, err
	}
	return &QuestionDelivery{UUID: uuid, Type: qType, Difficulty: difficulty, Prompt: prompt, Content: stripped}, nil
}

// stripExplanation removes the top-level "explanation" key (if present)
// from a question_translations.content JSON blob before it's served over
// the correctness-stripped Next() read path - that key is reveal-only
// content, meant to appear solely in the answers response once a
// question is wrong or skipped (see reinforcement(), which reads this
// same column independently and is unaffected by this stripping).
func stripExplanation(content []byte) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(content, &fields); err != nil {
		return nil, err
	}
	if _, ok := fields["explanation"]; !ok {
		return json.RawMessage(content), nil
	}
	delete(fields, "explanation")
	stripped, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(stripped), nil
}

type AnswerResult struct {
	Correct       bool
	Skipped       bool
	CorrectReveal json.RawMessage
	Explanation   *string
	PostPaths     []string
	AttemptDone   bool
}

// Answer grades questionUUID's response (or records a skip) against the
// question at the attempt's current position, advancing it. Rejects if
// questionUUID isn't the question actually at that position - this, plus
// the Answers map being keyed by question id (a question id can only
// occupy one key, so RecordAnswer can never store a second answer against
// the same question), is what stops answering out of order or twice.
func (s *QuizSessionService) Answer(userID int, attemptUUID, questionUUID string, response json.RawMessage, skipped bool, lang string) (*AnswerResult, error) {
	attempt, err := s.getOwnedAttempt(userID, attemptUUID)
	if err != nil {
		return nil, err
	}
	if attempt.CompletedAt != nil {
		return nil, ErrAttemptCompleted
	}

	answered, err := s.queries.AnsweredCount(attempt.UUID)
	if err != nil {
		return nil, err
	}
	total, err := s.queries.TotalQuestions(attempt.UUID)
	if err != nil {
		return nil, err
	}
	if answered >= total {
		return nil, ErrAttemptCompleted
	}

	questionID, questionUUIDAtPosition, qType, answerKey, err := s.queries.QuestionAtPosition(attempt.UUID, answered)
	if err != nil {
		return nil, err
	}
	if questionUUIDAtPosition != questionUUID {
		return nil, ErrWrongQuestion
	}

	var correct bool
	var reveal json.RawMessage
	responseToStore := response
	if skipped {
		responseToStore = json.RawMessage("null")
		// reveal depends only on answerKey, so grading against an empty
		// response still produces the right "what was correct" payload;
		// correctness is forced to false regardless, since a skip never
		// counts as correct.
		_, reveal, err = grade(qType, json.RawMessage(answerKey), json.RawMessage(`{}`))
		if err != nil {
			return nil, err
		}
		correct = false
	} else {
		correct, reveal, err = grade(qType, json.RawMessage(answerKey), response)
		if err != nil {
			return nil, err
		}
	}

	if err := s.queries.RecordAnswer(attempt.UUID, questionID, string(responseToStore), correct, skipped); err != nil {
		return nil, err
	}

	result := &AnswerResult{Correct: correct, Skipped: skipped, AttemptDone: answered+1 >= total}
	if !correct {
		result.CorrectReveal = reveal
		explanation, postPaths, err := s.reinforcement(questionID, lang)
		if err != nil {
			return nil, err
		}
		result.Explanation = explanation
		result.PostPaths = postPaths
	}
	return result, nil
}

// reinforcement loads the optional explanation string embedded in this
// question's translated content, plus every post linked via
// question_posts (no "primary" post - reinforcement surfaces all of
// them).
func (s *QuizSessionService) reinforcement(questionID int, lang string) (explanation *string, postPaths []string, err error) {
	content, err := s.queries.ReinforcementContent(questionID, lang)
	if err != nil {
		return nil, nil, err
	}
	var parsed struct {
		Explanation *string `json:"explanation"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, nil, err
	}

	postPaths, err = s.queries.ReinforcementPostPaths(questionID)
	if err != nil {
		return nil, nil, err
	}
	return parsed.Explanation, postPaths, nil
}

type CompleteResult struct {
	Score  int
	Passed bool
	// Reinforcement posts aggregated over every missed (wrong or skipped)
	// answer, in the order they were missed - the server-side version of
	// what per-answer reinforcement already reports, so review pointers
	// survive resumed attempts.
	ReviewPostPaths []string
}

// Complete finalizes an attempt, requiring every question already have an
// entry in the attempt blob's Answers map (answered or skipped, either
// counts as "resolved"). The client always learns exactly when it's
// reached this point from the last Answer() call's AttemptDone, so a
// premature Complete call is a client bug, not a state this method papers
// over - no auto-skip-the-rest behavior.
func (s *QuizSessionService) Complete(userID int, attemptUUID string) (*CompleteResult, error) {
	attempt, err := s.getOwnedAttempt(userID, attemptUUID)
	if err != nil {
		return nil, err
	}
	if attempt.CompletedAt != nil {
		return nil, ErrAttemptCompleted
	}

	total, err := s.queries.TotalQuestions(attempt.UUID)
	if err != nil {
		return nil, err
	}
	answered, err := s.queries.AnsweredCount(attempt.UUID)
	if err != nil {
		return nil, err
	}
	if answered < total {
		return nil, ErrAttemptIncomplete
	}

	correctCount, err := s.queries.CountCorrectAnswers(attempt.UUID)
	if err != nil {
		return nil, err
	}

	score := 0
	passed := false
	if total > 0 {
		score = correctCount * 100 / total
		passed = float64(correctCount)/float64(total) >= passThreshold
	}

	if err := s.queries.MarkAttemptCompleted(attempt.UUID, score, passed); err != nil {
		return nil, err
	}

	reviewPaths, err := s.queries.ReviewPostPaths(attempt.UUID)
	if err != nil {
		return nil, err
	}

	return &CompleteResult{Score: score, Passed: passed, ReviewPostPaths: reviewPaths}, nil
}
