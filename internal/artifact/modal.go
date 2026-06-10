package artifact

import (
	"github.com/justinstimatze/crystal/internal/compare"
	"github.com/justinstimatze/crystal/internal/record"
)

// Modal is a crystallized hook that serves the single most common output
// observed in its training records, regardless of input. It is the
// concrete artifact a deterministic-hook crystallization would produce for
// a constant- or modal-output pattern (e.g. a heartbeat, or a build whose
// result is "success" in the common case).
//
// For a det=1.0 pattern this serves the only output. For a det<1.0
// near-miss it serves the majority output and is expected to be caught by
// drift detection on the minority cases — which is exactly what the drift
// experiment measures.
type Modal struct {
	name string
	out  record.Output
}

// NewModal builds a Modal from training records and returns it alongside
// the training-set determinism (modal class size / N) — the value the
// promote gate would check before deploying it.
func NewModal(name string, train []record.Record) (Modal, float64) {
	counts := map[string]int{}
	rep := map[string]record.Output{}
	for _, r := range train {
		fp := compare.Fingerprint(r.Tool, r.Result)
		counts[fp]++
		if _, ok := rep[fp]; !ok {
			rep[fp] = r.Result
		}
	}
	best := ""
	bestN := 0
	for fp, n := range counts {
		if n > bestN || (n == bestN && fp < best) {
			bestN, best = n, fp
		}
	}
	det := 0.0
	if len(train) > 0 {
		det = float64(bestN) / float64(len(train))
	}
	return Modal{name: name, out: rep[best]}, det
}

func (m Modal) Name() string { return m.name }

func (m Modal) Produce(in record.Record) (record.Output, error) {
	return m.out, nil
}
