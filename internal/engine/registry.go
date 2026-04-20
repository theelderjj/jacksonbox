package engine

import (
	"encoding/json"
	"fmt"
	"sync"
)

// AnyDecision is the type-erased Decision[R]. The room actor works with this;
// it only needs to know "should we advance?" and "what events go out?".
type AnyDecision struct {
	AdvancePhase bool
	Broadcast    []Event
	Result       PhaseResult // nil if no terminal result produced
}

// AnyPrimitive is the type-erased Primitive[R]. Erase[R] wraps a typed
// primitive into one.
type AnyPrimitive interface {
	Name() string
	Start(*PhaseContext) (AnyDecision, error)
	Handle(*PhaseContext, Input) (AnyDecision, error)
	Timeout(*PhaseContext) (AnyDecision, error)
}

type erased[R any] struct{ inner Primitive[R] }

// Erase lifts a typed primitive into the actor-facing non-generic form.
// The typed R is preserved inside StoredResult[R] so downstream GetResult[R]
// still recovers it without `any` gymnastics.
func Erase[R any](p Primitive[R]) AnyPrimitive { return &erased[R]{inner: p} }

func (e *erased[R]) Name() string { return e.inner.Name() }

func (e *erased[R]) Start(ctx *PhaseContext) (AnyDecision, error) {
	d, err := e.inner.Start(ctx)
	return e.wrap(ctx, d), err
}

func (e *erased[R]) Handle(ctx *PhaseContext, in Input) (AnyDecision, error) {
	d, err := e.inner.Handle(ctx, in)
	return e.wrap(ctx, d), err
}

func (e *erased[R]) Timeout(ctx *PhaseContext) (AnyDecision, error) {
	d, err := e.inner.Timeout(ctx)
	return e.wrap(ctx, d), err
}

func (e *erased[R]) wrap(ctx *PhaseContext, d Decision[R]) AnyDecision {
	ad := AnyDecision{AdvancePhase: d.AdvancePhase, Broadcast: d.Broadcast}
	// Only box the result on advance; intermediate Handle calls shouldn't
	// overwrite PhaseData. The actor enforces this too, but belt-and-suspenders.
	if d.AdvancePhase {
		ad.Result = StoredResult[R]{
			Phase:     ctx.Phase.Name,
			Primitive: e.inner.Name(),
			Value:     d.Result,
		}
	}
	return ad
}

// Factory builds a primitive from its phase config. The factory decides
// which generic instantiation to return.
type Factory func(config json.RawMessage) (AnyPrimitive, error)

var (
	regMu    sync.RWMutex
	registry = map[string]Factory{}
)

// Register wires a primitive key to its factory. Call from init().
// Panics on duplicate — that's a programming error, not runtime data.
func Register(key string, f Factory) {
	regMu.Lock()
	defer regMu.Unlock()
	if _, exists := registry[key]; exists {
		panic(fmt.Sprintf("engine: primitive %q already registered", key))
	}
	registry[key] = f
}

// Make resolves a phase's primitive via the registry.
func Make(phase Phase) (AnyPrimitive, error) {
	regMu.RLock()
	f, ok := registry[phase.Primitive]
	regMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("engine: unknown primitive %q for phase %q", phase.Primitive, phase.Name)
	}
	return f(phase.Config)
}

// KnownPrimitives is for test visibility.
func KnownPrimitives() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}
