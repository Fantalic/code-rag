package ziganalyser

// Local scope tracker. While walking a function body we need to know
// which names refer to a local variable (and therefore can NOT be
// resolved through the GlobalIndex) versus which names refer to a
// package-level symbol or a field access. The scope is a simple
// linked-list-of-maps — push on block entry, pop on block exit —
// because Zig has block-scoped locals with no implicit shadowing rules
// beyond lexical nesting.
//
// Resolution order for a callee name at a call site:
//  1. Walk up scope chain looking for a local whose name matches.
//  2. Look up the name in the GlobalIndex (free function or method).
//  3. Give up — emit nothing (no phantom edges).

// Scope is one frame of the local-variable stack.
type Scope struct {
	parent  *Scope
	locals  map[string]string // name -> optional type name (best-effort)
	aliases map[string]string // name -> qualifier hint from "const X = expr"
}

func NewScope(parent *Scope) *Scope {
	return &Scope{
		parent:  parent,
		locals:  make(map[string]string),
		aliases: make(map[string]string),
	}
}

// Declare registers a local variable / parameter.
func (s *Scope) Declare(name, typeName string) {
	if s == nil || name == "" {
		return
	}
	s.locals[name] = typeName
}

// Alias registers a name -> qualifier mapping inferred from an
// "const x = obj.field;" rebinding (heuristic only).
func (s *Scope) Alias(name, qualifier string) {
	if s == nil || name == "" {
		return
	}
	s.aliases[name] = qualifier
}

// LookupLocal walks the chain looking for a local declaration. Returns
// the type name (may be "") and a found flag.
func (s *Scope) LookupLocal(name string) (string, bool) {
	for cur := s; cur != nil; cur = cur.parent {
		if t, ok := cur.locals[name]; ok {
			return t, true
		}
		if q, ok := cur.aliases[name]; ok {
			return q, true
		}
	}
	return "", false
}

// Push enters a nested block. Caller must defer the matching Pop.
func (s *Scope) Push() *Scope {
	return NewScope(s)
}

// Pop is the inverse of Push. Kept as a method for symmetry; we never
// actually need the popped frame after returning to the caller.
func (s *Scope) Pop() *Scope {
	return s.parent
}
