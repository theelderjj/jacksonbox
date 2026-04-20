package drawful

import (
	"strings"
	"testing"
)

func TestDefaultPromptsIncludesGeneratedSet(t *testing.T) {
	want := len(basePrompts) + generatedPromptCount
	if got := len(DefaultPrompts); got != want {
		t.Fatalf("DefaultPrompts length: want %d, got %d", want, got)
	}

	seen := map[string]bool{}
	for _, prompt := range DefaultPrompts {
		if prompt == "" {
			t.Fatal("DefaultPrompts contains an empty prompt")
		}
		if seen[prompt] {
			t.Fatalf("DefaultPrompts contains duplicate prompt %q", prompt)
		}
		seen[prompt] = true
	}
}

func TestGeneratedPromptPoolsHaveExpectedSize(t *testing.T) {
	cases := map[string][]string{
		"subjects": generatedSubjects(),
		"actions":  generatedActions(),
		"scenes":   generatedScenes(),
	}
	for name, pool := range cases {
		if got := len(pool); got != generatedPoolSize {
			t.Fatalf("%s pool length: want %d, got %d", name, generatedPoolSize, got)
		}
		seen := map[string]bool{}
		for _, item := range pool {
			if item == "" {
				t.Fatalf("%s pool contains an empty item", name)
			}
			if seen[item] {
				t.Fatalf("%s pool contains duplicate item %q", name, item)
			}
			seen[item] = true
		}
	}
}

func TestGeneratedPromptsStayShort(t *testing.T) {
	for _, prompt := range generatedPrompts() {
		words := len(strings.Fields(prompt))
		if words > 12 {
			t.Fatalf("generated prompt is too long (%d words): %q", words, prompt)
		}
	}
}

func TestGeneratedSubjectsIncludeRequestedPopCulturePool(t *testing.T) {
	subjects := generatedSubjects()
	joined := "\n" + strings.Join(subjects, "\n") + "\n"
	for _, want := range []string{
		"SpongeBob",
		"Timmy Turner",
		"Dexter",
		"Samurai Jack",
		"Spider-Man",
		"Wolverine",
		"Batman",
		"Wonder Woman",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("generated subjects should include %q", want)
		}
	}
}
