// Binary kubectl is the stdin/v1 filter for the `kubectl` command.
// It reads a single JSON request from stdin, applies the filter logic,
// and writes a single JSON response to stdout.
//
// Protocol: stdin/v1 — see the prunesh/date HOWTO.md for the module contract.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/prunesh/kubectl/filter"
)

type request struct {
	Operation string   `json:"operation"`
	Args      []string `json:"args"`
	Output    string   `json:"output"`
	ExitCode  int      `json:"exit_code"`
}

type response struct {
	Args    []string `json:"args,omitempty"`
	Changed bool     `json:"changed"`
	Output  string   `json:"output,omitempty"`
}

func main() {
	var req request
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		fmt.Fprintf(os.Stderr, "kubectl: decode request: %v\n", err)
		os.Exit(1)
	}

	var resp response
	switch req.Operation {
	case "rewrite":
		rewritten, changed := filter.Rewrite(req.Args)
		resp.Changed = changed
		if changed {
			resp.Args = rewritten
		}
	case "filter_output":
		filtered := filter.FilterOutput(req.Args, req.Output, req.ExitCode)
		resp.Output = filtered
		resp.Changed = filtered != req.Output
	default:
		fmt.Fprintf(os.Stderr, "kubectl: unknown operation %q\n", req.Operation)
		os.Exit(1)
	}

	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		fmt.Fprintf(os.Stderr, "kubectl: encode response: %v\n", err)
		os.Exit(1)
	}
}
