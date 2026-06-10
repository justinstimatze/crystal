package cmd

import (
	"fmt"

	"github.com/justinstimatze/crystal/internal/lattice"
)

// LatticeCmd prints the depth × per-hop-loss convergence grid for the
// self-reauthoring feedback topology — the cheap, no-API test of crystal's
// riskiest assumption.
type LatticeCmd struct {
	DriftErr float64 `help:"Bottom-tier error rate after the injected shift." default:"0.5"`
	Gain     float64 `help:"Re-author correction gain." default:"0.5"`
	Demote   float64 `help:"Observed-error threshold to trigger re-authoring." default:"0.08"`
	Recover  float64 `help:"Bottom error considered recovered." default:"0.10"`
	MaxDepth int     `help:"Deepest stack to evaluate." default:"6"`
}

func (c *LatticeCmd) Run() error {
	base := lattice.Params{
		DriftErr:         c.DriftErr,
		CorrectionGain:   c.Gain,
		DemoteThreshold:  c.Demote,
		RecoverThreshold: c.Recover,
		InjectStep:       5,
		MaxSteps:         200,
	}
	depths := make([]int, c.MaxDepth)
	for i := range depths {
		depths[i] = i + 1
	}
	losses := []float64{0.0, 0.1, 0.2, 0.3, 0.4, 0.5}

	fmt.Printf("self-reauthoring convergence grid  (drift=%.2f gain=%.2f demote>%.2f recover≤%.2f)\n",
		c.DriftErr, c.Gain, c.Demote, c.Recover)
	fmt.Print("legend: ok=recovers  res=residual silent floor  SIL=top never alarms\n\n")
	fmt.Printf("%-7s", "depth\\λ")
	for _, l := range losses {
		fmt.Printf(" %5.1f", l)
	}
	fmt.Println()
	for _, d := range depths {
		fmt.Printf("%-7d", d)
		for _, l := range losses {
			r := lattice.Simulate(withDL(base, d, l))
			fmt.Printf(" %5s", regimeShort(r.Regime))
		}
		fmt.Println()
	}
	fmt.Println()
	for _, l := range losses {
		fmt.Printf("  max safe depth @ λ=%.1f : %d\n", l, lattice.MaxSafeDepth(base, l, c.MaxDepth))
	}
	return nil
}

func regimeShort(regime string) string {
	switch regime {
	case "ok":
		return "ok"
	case "residual":
		return "res"
	default:
		return "SIL"
	}
}

func withDL(p lattice.Params, d int, l float64) lattice.Params {
	p.Depth, p.HopLoss = d, l
	return p
}
