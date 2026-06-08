package cmd

import (
	"fmt"
	"net/http"
	"os"
)

// VizCmd serves the repo root over HTTP so the live flow dashboard
// (docs/viz/live.html) can fetch the data-driven viz source
// (.crystal-viz/hook-loop-flow.json) and re-render in realtime — meant to sit on
// a second monitor while you work. A run that re-emits the flow record updates the
// dashboard on its next poll. Plain file server, no deps.
type VizCmd struct {
	Dir  string `help:"Directory to serve (repo root, so /docs/viz and /.crystal-viz both resolve)." default:"."`
	Port int    `help:"Port to listen on." default:"8777"`
}

func (c *VizCmd) Run() error {
	if _, err := os.Stat(c.Dir); err != nil {
		return usageError{fmt.Errorf("serve dir %q: %w", c.Dir, err)}
	}
	addr := fmt.Sprintf("127.0.0.1:%d", c.Port)
	url := fmt.Sprintf("http://%s/docs/viz/live.html", addr)
	fmt.Printf("crystal viz: serving %s\n", c.Dir)
	fmt.Printf("  live flow dashboard → %s\n", url)
	fmt.Printf("  (re-run `crystal hook-loop …` and the dashboard updates on its next poll; Ctrl-C to stop)\n")
	return http.ListenAndServe(addr, http.FileServer(http.Dir(c.Dir)))
}
