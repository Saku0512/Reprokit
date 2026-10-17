package lesson

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	idPattern      = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]{0,62}[a-z0-9])?/[a-z0-9](?:[a-z0-9._-]{0,62}[a-z0-9])?$`)
	versionPattern = regexp.MustCompile(`^(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)$`)
	refPattern     = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	cvePattern     = regexp.MustCompile(`^CVE-[0-9]{4}-[0-9]{4,}$`)
)

// Lessonは版固定の学習パッケージを表す。
type Lesson struct {
	APIVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Metadata   Metadata `json:"metadata"`
	Spec       Spec     `json:"spec"`
}

type Metadata struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Title   string `json:"title"`
}

type Spec struct {
	Audience        string       `json:"audience"`
	Prerequisites   []string     `json:"prerequisites"`
	DurationMinutes int          `json:"durationMinutes"`
	Objectives      []string     `json:"objectives"`
	Case            Case         `json:"case"`
	Evidence        Evidence     `json:"evidence"`
	License         License      `json:"license"`
	Labs            []Lab        `json:"labs"`
	Steps           []Step       `json:"steps"`
	Analysis        Analysis     `json:"analysis"`
	Comparison      Comparison   `json:"comparison"`
	Prevention      []string     `json:"prevention"`
	Assessment      []Assessment `json:"assessment"`
}

type Case struct {
	Product          string `json:"product"`
	AffectedVersions string `json:"affectedVersions"`
	Background       string `json:"background"`
}

type Evidence struct {
	CVEs       []string    `json:"cves,omitempty"`
	References []Reference `json:"references"`
}

type Reference struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

type License struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Attribution string `json:"attribution"`
}

type Lab struct {
	Ref     string `json:"ref"`
	LabID   string `json:"labId"`
	Purpose string `json:"purpose"`
}

type Step struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	LabRef         string   `json:"labRef,omitempty"`
	Instruction    string   `json:"instruction"`
	ExpectedResult string   `json:"expectedResult"`
	Observation    string   `json:"observation"`
	Hints          []string `json:"hints"`
}

type Analysis struct {
	Cause       string `json:"cause"`
	Impact      string `json:"impact"`
	Remediation string `json:"remediation"`
}

type Comparison struct {
	Vulnerable  string `json:"vulnerable"`
	Fixed       string `json:"fixed"`
	Difference  string `json:"difference"`
	FixedLabRef string `json:"fixedLabRef,omitempty"`
}

type Assessment struct {
	ID            string   `json:"id"`
	Question      string   `json:"question"`
	AnswerExample string   `json:"answerExample"`
	Criteria      []string `json:"criteria"`
}

func schemaSpace(value rune) bool {
	switch {
	case value >= '\t' && value <= '\r', value >= '\u2000' && value <= '\u200a':
		return true
	}
	switch value {
	case ' ', '\u00a0', '\u1680', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000', '\ufeff':
		return true
	}
	return false
}

func nonBlank(value string) bool {
	return strings.IndexFunc(value, func(value rune) bool { return !schemaSpace(value) }) >= 0
}

func textList(values []string, required bool) bool {
	if values == nil || (required && len(values) == 0) {
		return false
	}
	for _, value := range values {
		if !nonBlank(value) {
			return false
		}
	}
	return true
}

func uniqueTexts(values []string) bool {
	seen := make(map[string]bool)
	for _, value := range values {
		if seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}

func httpsURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Hostname() != "" && parsed.User == nil && strings.IndexFunc(value, schemaSpace) < 0
}

// ValidateはSchemaの必須値と参照整合性を公開前に確認する。
func Validate(value Lesson) error {
	if value.APIVersion != "reprokit.dev/v1alpha1" || value.Kind != "Lesson" {
		return errors.New("unsupported Lesson version or kind")
	}
	if !idPattern.MatchString(value.Metadata.ID) || !versionPattern.MatchString(value.Metadata.Version) || !nonBlank(value.Metadata.Title) || cvePattern.MatchString(value.Metadata.Title) {
		return errors.New("invalid Lesson metadata")
	}
	spec := value.Spec
	if !nonBlank(spec.Audience) || !textList(spec.Prerequisites, false) || !uniqueTexts(spec.Prerequisites) || spec.DurationMinutes < 1 || spec.DurationMinutes > 1440 || !textList(spec.Objectives, true) {
		return errors.New("invalid Lesson audience, prerequisites, duration, or objectives")
	}
	if !nonBlank(spec.Case.Product) || !nonBlank(spec.Case.AffectedVersions) || !nonBlank(spec.Case.Background) {
		return errors.New("incomplete case")
	}
	if len(spec.Evidence.References) == 0 {
		return errors.New("missing evidence references")
	}
	cves := make(map[string]bool)
	for _, cve := range spec.Evidence.CVEs {
		if !cvePattern.MatchString(cve) || cves[cve] {
			return fmt.Errorf("invalid or duplicate CVE: %q", cve)
		}
		cves[cve] = true
	}
	for _, reference := range spec.Evidence.References {
		switch reference.Type {
		case "vendor-advisory", "cve-record", "patch", "product-docs", "other":
		default:
			return errors.New("invalid reference type")
		}
		if !nonBlank(reference.Title) || !httpsURL(reference.URL) {
			return errors.New("invalid reference")
		}
	}
	if !nonBlank(spec.License.Name) || !httpsURL(spec.License.URL) || !nonBlank(spec.License.Attribution) {
		return errors.New("incomplete license")
	}
	if len(spec.Labs) == 0 {
		return errors.New("missing Lab references")
	}
	labs := make(map[string]Lab)
	for _, lab := range spec.Labs {
		if !refPattern.MatchString(lab.Ref) || !idPattern.MatchString(lab.LabID) || (lab.Purpose != "reproduce" && lab.Purpose != "compare-fix") {
			return errors.New("invalid Lab reference")
		}
		if _, exists := labs[lab.Ref]; exists {
			return fmt.Errorf("duplicate Lab ref %q", lab.Ref)
		}
		labs[lab.Ref] = lab
	}
	if len(spec.Steps) == 0 {
		return errors.New("missing steps")
	}
	steps := make(map[string]bool)
	for _, step := range spec.Steps {
		if !refPattern.MatchString(step.ID) || steps[step.ID] || !nonBlank(step.Title) || !nonBlank(step.Instruction) || !nonBlank(step.ExpectedResult) || !nonBlank(step.Observation) || !textList(step.Hints, false) {
			return errors.New("invalid or duplicate step")
		}
		steps[step.ID] = true
		if step.LabRef != "" {
			if _, exists := labs[step.LabRef]; !exists {
				return fmt.Errorf("unknown step Lab ref %q", step.LabRef)
			}
		}
	}
	if !nonBlank(spec.Analysis.Cause) || !nonBlank(spec.Analysis.Impact) || !nonBlank(spec.Analysis.Remediation) || !nonBlank(spec.Comparison.Vulnerable) || !nonBlank(spec.Comparison.Fixed) || !nonBlank(spec.Comparison.Difference) || !textList(spec.Prevention, true) {
		return errors.New("incomplete analysis, comparison, or prevention")
	}
	if spec.Comparison.FixedLabRef != "" && labs[spec.Comparison.FixedLabRef].Purpose != "compare-fix" {
		return errors.New("fixedLabRef must refer to a compare-fix Lab")
	}
	if len(spec.Assessment) == 0 {
		return errors.New("missing assessment")
	}
	questions := make(map[string]bool)
	for _, question := range spec.Assessment {
		if !refPattern.MatchString(question.ID) || questions[question.ID] || !nonBlank(question.Question) || !nonBlank(question.AnswerExample) || !textList(question.Criteria, true) {
			return errors.New("invalid or duplicate assessment")
		}
		questions[question.ID] = true
	}
	return nil
}
