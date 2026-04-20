package engine

import "fmt"

// ValidatePhaseSequence checks that every Phase.DependsOn refers to an
// earlier phase in the sequence. Called at process startup; a failure here
// is a programming error, not a runtime condition — fail fast and loudly.
func ValidatePhaseSequence(phases []Phase) error {
	seen := make(map[string]int, len(phases))
	for i, p := range phases {
		if p.Name == "" {
			return fmt.Errorf("engine: phase %d has empty name", i)
		}
		if _, dupe := seen[p.Name]; dupe {
			return fmt.Errorf("engine: duplicate phase name %q at index %d", p.Name, i)
		}
		for _, dep := range p.DependsOn {
			idx, ok := seen[dep]
			if !ok {
				return fmt.Errorf("engine: phase %q depends on %q which is not an earlier phase", p.Name, dep)
			}
			if idx >= i {
				return fmt.Errorf("engine: phase %q depends on %q which does not precede it", p.Name, dep)
			}
		}
		seen[p.Name] = i
	}
	return nil
}
