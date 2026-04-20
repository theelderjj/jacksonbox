package engine

import "testing"

func TestValidatePhaseSequence_ok(t *testing.T) {
	t.Log("Scenario: three phases in topological order (a → b → c), each depending only on earlier phases.")
	t.Log("Expected: ValidatePhaseSequence returns nil — forward DAG is well-formed.")
	phases := []Phase{
		{Name: "a"},
		{Name: "b", DependsOn: []string{"a"}},
		{Name: "c", DependsOn: []string{"a", "b"}},
	}
	if err := ValidatePhaseSequence(phases); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestValidatePhaseSequence_forwardDep(t *testing.T) {
	t.Log("Scenario: phase `a` declares a dependency on `b`, but `b` is declared later in the slice.")
	t.Log("Expected: validation rejects the sequence — dependencies must point backward, not forward.")
	phases := []Phase{
		{Name: "a", DependsOn: []string{"b"}},
		{Name: "b"},
	}
	if err := ValidatePhaseSequence(phases); err == nil {
		t.Fatal("expected forward-dep error")
	}
}

func TestValidatePhaseSequence_missingDep(t *testing.T) {
	t.Log("Scenario: phase `a` depends on a name (`nope`) that is not present anywhere in the sequence.")
	t.Log("Expected: validation rejects — every DependsOn entry must resolve to a declared phase.")
	phases := []Phase{
		{Name: "a", DependsOn: []string{"nope"}},
	}
	if err := ValidatePhaseSequence(phases); err == nil {
		t.Fatal("expected missing-dep error")
	}
}

func TestValidatePhaseSequence_duplicateName(t *testing.T) {
	t.Log("Scenario: two phases declared with the same name.")
	t.Log("Expected: validation rejects — phase names form the result-store key space and must be unique.")
	phases := []Phase{{Name: "a"}, {Name: "a"}}
	if err := ValidatePhaseSequence(phases); err == nil {
		t.Fatal("expected duplicate-name error")
	}
}
