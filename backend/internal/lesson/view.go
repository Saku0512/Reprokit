package lesson

// LearnerLesson は段階開示前のヒントと講師用の回答・評価基準を含まない。
type LearnerLesson struct {
	APIVersion string      `json:"apiVersion"`
	Kind       string      `json:"kind"`
	Metadata   Metadata    `json:"metadata"`
	Spec       LearnerSpec `json:"spec"`
}

type LearnerSpec struct {
	Audience        string        `json:"audience"`
	Prerequisites   []string      `json:"prerequisites"`
	DurationMinutes int           `json:"durationMinutes"`
	Objectives      []string      `json:"objectives"`
	Case            Case          `json:"case"`
	Evidence        Evidence      `json:"evidence"`
	License         License       `json:"license"`
	Labs            []Lab         `json:"labs"`
	Steps           []LearnerStep `json:"steps"`
	Analysis        Analysis      `json:"analysis"`
	Comparison      Comparison    `json:"comparison"`
	Prevention      []string      `json:"prevention"`
	Assessment      []Question    `json:"assessment"`
}

type LearnerStep struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	LabRef         string `json:"labRef,omitempty"`
	Instruction    string `json:"instruction"`
	ExpectedResult string `json:"expectedResult"`
	Observation    string `json:"observation"`
	HintCount      int    `json:"hintCount"`
}

type Question struct {
	ID       string `json:"id"`
	Question string `json:"question"`
}

func learnerView(value Lesson) LearnerLesson {
	spec := value.Spec
	result := LearnerLesson{APIVersion: value.APIVersion, Kind: value.Kind, Metadata: value.Metadata, Spec: LearnerSpec{
		Audience: spec.Audience, Prerequisites: spec.Prerequisites, DurationMinutes: spec.DurationMinutes,
		Objectives: spec.Objectives, Case: spec.Case, Evidence: spec.Evidence, License: spec.License,
		Labs: spec.Labs, Analysis: spec.Analysis, Comparison: spec.Comparison, Prevention: spec.Prevention,
		Steps: make([]LearnerStep, 0, len(spec.Steps)), Assessment: make([]Question, 0, len(spec.Assessment)),
	}}
	for _, step := range spec.Steps {
		result.Spec.Steps = append(result.Spec.Steps, LearnerStep{ID: step.ID, Title: step.Title, LabRef: step.LabRef,
			Instruction: step.Instruction, ExpectedResult: step.ExpectedResult, Observation: step.Observation, HintCount: len(step.Hints)})
	}
	for _, question := range spec.Assessment {
		result.Spec.Assessment = append(result.Spec.Assessment, Question{ID: question.ID, Question: question.Question})
	}
	return result
}

type SessionView struct {
	StudySession
	Hints map[string][]string `json:"hints"`
}

func (store *SessionStore) view(session StudySession) SessionView {
	value, _ := store.catalog.Get(session.LessonID, session.LessonVersion)
	hints := make(map[string][]string)
	for _, step := range value.Spec.Steps {
		if count := session.RevealedHints[step.ID]; count > 0 {
			hints[step.ID] = append([]string{}, step.Hints[:count]...)
		}
	}
	return SessionView{StudySession: session, Hints: hints}
}
