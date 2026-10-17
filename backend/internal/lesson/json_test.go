package lesson

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLessonAcceptsExactIntegerNotation(t *testing.T) {
	source := string(fixture(t))
	if !strings.Contains(source, `"durationMinutes": 60`) {
		t.Fatal("unexpected fixture duration")
	}
	for _, number := range []string{"30", "30.0", "3e1", "0.3e2", "300e-1", "30.0000000000000000000000000"} {
		t.Run(number, func(t *testing.T) {
			value, err := decodeLesson([]byte(strings.Replace(source, `"durationMinutes": 60`, `"durationMinutes": `+number, 1)))
			if err != nil || value.Spec.DurationMinutes != 30 {
				t.Fatalf("valid integer %s: %v %#v", number, err, value.Metadata)
			}
		})
	}
	for _, number := range []string{"30.00000000000000000001", "29.99999999999999999999", "3e-1", "1e999999999999", "1e-999999999999", "1e-99999"} {
		t.Run(number, func(t *testing.T) {
			if _, err := decodeLesson([]byte(strings.Replace(source, `"durationMinutes": 60`, `"durationMinutes": `+number, 1))); err == nil {
				t.Fatalf("invalid duration accepted: %s", number)
			}
		})
	}
}

func TestIntegerNormalizationHandlesZeroAndSignedValues(t *testing.T) {
	for input, want := range map[string]string{"-3.0": "-3", "-0.0e9000000000": "0", "0.000e-9000000000": "0", "100000e-5": "1", "0.00010e4": "1"} {
		got, err := normalizeInteger(json.Number(input))
		if err != nil || string(got) != want {
			t.Fatalf("%s: %s %v, want %s", input, got, err, want)
		}
	}
}

func TestLessonWhitespaceMatchesSchema(t *testing.T) {
	for _, space := range []rune{'\t', '\n', '\v', '\f', '\r', ' ', '\u00a0', '\u1680', '\u2000', '\u200a', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000', '\ufeff'} {
		t.Run(string(space), func(t *testing.T) {
			data := mutateFixture(t, func(value map[string]any) {
				value["spec"].(map[string]any)["license"].(map[string]any)["url"] = "https://example.test/a" + string(space) + "b"
			})
			if _, err := decodeLesson(data); err == nil {
				t.Fatalf("URL containing U+%04X accepted", space)
			}
			data = mutateFixture(t, func(value map[string]any) { value["metadata"].(map[string]any)["title"] = string(space) })
			if _, err := decodeLesson(data); err == nil {
				t.Fatalf("blank title U+%04X accepted", space)
			}
		})
	}
	// U+0085はGoのTrimSpaceでは空白だが、正本SchemaのECMAScript \sには含まれない。
	if !nonBlank("\u0085") {
		t.Fatal("nonBlank differs from Schema")
	}
}

func TestCatalogRejectsReferencesBeyondRunnerPrepareLimit(t *testing.T) {
	for _, change := range []func(map[string]any){
		func(value map[string]any) {
			lab := value["spec"].(map[string]any)["labs"].([]any)[0].(map[string]any)
			lab["ref"] = strings.Repeat("a", maxRunnerPrepareBytes+1)
			// 参照だけが原因で除外されることを確認するため手順との紐付けも更新する。
			for _, step := range value["spec"].(map[string]any)["steps"].([]any) {
				if step.(map[string]any)["labRef"] == "vulnerable" {
					step.(map[string]any)["labRef"] = lab["ref"]
				}
			}
		},
		func(value map[string]any) {
			value["metadata"].(map[string]any)["version"] = "1." + strings.Repeat("9", maxRunnerPrepareBytes) + ".0"
		},
	} {
		data := mutateFixture(t, change)
		if _, err := decodeLesson(data); err == nil || !strings.Contains(err.Error(), "Runner Prepare") {
			t.Fatalf("oversized reference not rejected before publishing: %v", err)
		}
	}
}
