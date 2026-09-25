// Package filter implements the prunesh/kubectl filter logic.
//
// Contract:
//   - id:      prunesh/kubectl
//   - command: kubectl
//
// Rewrite: injects --tail=100 into `logs` invocations that do not specify a
// tail limit or a follow flag, capping log output before it reaches the proxy.
//
// FilterOutput:
//   - describe: drops verbose sections (Annotations, Volumes, Tolerations, …);
//     keeps Name/Namespace/Status/Node/Containers/Conditions/Events.
//   - get -o yaml: strips the managedFields block and noisy metadata fields.
//   - get -o json: strips managedFields and noisy metadata fields.
//   - everything else: passes through unchanged.
package filter

import (
	"fmt"
	"strings"
)

const (
	// ID is the full filter identity following the author/<name> rule.
	ID = "prunesh/kubectl"

	// Command is the argv0 intercepted by this module.
	Command = "kubectl"

	defaultLogTail = "100"

	// maxDescribeLines is a safety cap; well-filtered describe rarely hits it.
	maxDescribeLines = 200
)

// skipDescribeSections are top-level describe sections dropped entirely.
var skipDescribeSections = map[string]bool{
	"Annotations":     true,
	"Volumes":         true,
	"Tolerations":     true,
	"QoS Class":       true,
	"Node-Selectors":  true,
	"Priority":        true,
	"Service Account": true,
}

// skipContainerFields are per-container sub-fields dropped inside Containers:.
var skipContainerFields = map[string]bool{
	"Container ID": true,
	"Image ID":     true,
	"Host Port":    true,
	"Mounts":       true,
}

// noisyYAMLFields are YAML keys whose entire block is dropped in -o yaml output.
var noisyYAMLFields = map[string]bool{
	"managedFields":     true,
	"selfLink":          true,
	"resourceVersion":   true,
	"uid":               true,
	"generation":        true,
	"creationTimestamp": true,
}

// Rewrite adds --tail=100 to `logs` invocations when no limit or follow flag
// is already present.
func Rewrite(args []string) ([]string, bool) {
	if subcmd(args) != "logs" {
		return nil, false
	}
	for _, a := range args {
		if a == "-f" || a == "--follow" || strings.HasPrefix(a, "--tail") {
			return nil, false
		}
	}
	out := make([]string, len(args)+1)
	copy(out, args)
	out[len(args)] = "--tail=" + defaultLogTail
	return out, true
}

// FilterOutput dispatches to the appropriate filter by kubectl subcommand.
func FilterOutput(args []string, output string, exitCode int) string {
	if output == "" {
		return output
	}
	switch subcmd(args) {
	case "describe":
		return filterDescribe(output)
	case "get":
		switch outputFormat(args) {
		case "yaml":
			return filterYAML(output)
		case "json":
			return filterJSON(output)
		}
	}
	return output
}

// --- describe ---

func filterDescribe(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	var result []string
	inSkipped := false     // inside a skipped top-level section
	inContainers := false  // inside Containers: section
	skipContainerField := false // skipping a per-container sub-field block
	containerFieldIndent := -1

	for _, line := range lines {
		// Document separator (multiple resources)
		if line == "" {
			if !inSkipped {
				result = append(result, line)
			}
			continue
		}

		indent := leadingSpaces(line)
		trimmed := strings.TrimSpace(line)

		// Top-level line (indent == 0): reset section state
		if indent == 0 {
			inSkipped = false
			inContainers = false
			skipContainerField = false
			containerFieldIndent = -1

			key := topLevelKey(trimmed)
			if skipDescribeSections[key] {
				inSkipped = true
				continue
			}
			if key == "Containers" {
				inContainers = true
			}
			result = append(result, line)
			continue
		}

		if inSkipped {
			continue
		}

		// Inside Containers section: filter noisy per-container fields
		if inContainers {
			// A container sub-field starts at indent 4
			if indent == 4 {
				skipContainerField = false
				containerFieldIndent = -1
				key := fieldKey(trimmed)
				if skipContainerFields[key] {
					skipContainerField = true
					containerFieldIndent = indent
					continue
				}
			}
			// Skip lines that are children of a skipped container field
			if skipContainerField && indent > containerFieldIndent {
				continue
			}
		}

		result = append(result, line)
	}

	if len(result) > maxDescribeLines {
		result = result[:maxDescribeLines]
		result = append(result, fmt.Sprintf("... (%d lines truncated)", len(lines)-maxDescribeLines))
	}

	if len(result) == 0 {
		return output
	}
	return strings.Join(result, "\n") + "\n"
}

// --- get -o yaml ---

func filterYAML(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	var result []string
	skipIndent := -1 // when >= 0, skip lines indented more than this

	for _, line := range lines {
		if line == "---" {
			skipIndent = -1
			result = append(result, line)
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if skipIndent < 0 {
				result = append(result, line)
			}
			continue
		}

		indent := leadingSpaces(line)

		// End skip block when indent returns to same or lower level
		if skipIndent >= 0 && indent <= skipIndent {
			skipIndent = -1
		}

		if skipIndent >= 0 {
			continue
		}

		key := yamlKey(trimmed)
		if noisyYAMLFields[key] {
			// Block field (ends with ':'): skip entire indented block
			if strings.HasSuffix(trimmed, ":") {
				skipIndent = indent
			}
			// Single-value field: just skip this line
			continue
		}

		result = append(result, line)
	}

	if len(result) == 0 {
		return output
	}
	return strings.Join(result, "\n") + "\n"
}

// --- get -o json ---

func filterJSON(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	var result []string

	// Remove lines that are the key or value of noisy fields.
	// JSON output from kubectl is always pretty-printed with 4-space indent.
	skipIndent := -1

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if skipIndent < 0 {
				result = append(result, line)
			}
			continue
		}

		indent := leadingSpaces(line)

		// End skip when indent returns to opening key level
		if skipIndent >= 0 && indent <= skipIndent {
			skipIndent = -1
		}
		if skipIndent >= 0 {
			continue
		}

		key := jsonKey(trimmed)
		if noisyYAMLFields[key] { // same set applies to JSON keys
			skipIndent = indent
			continue
		}

		result = append(result, line)
	}

	// Remove dangling trailing commas left by dropped last-field in objects
	result = fixTrailingCommas(result)

	if len(result) == 0 {
		return output
	}
	return strings.Join(result, "\n") + "\n"
}

// fixTrailingCommas removes commas from lines just before a closing brace/bracket
// that were left by removing the last field in a JSON object.
func fixTrailingCommas(lines []string) []string {
	for i := 0; i < len(lines)-1; i++ {
		next := strings.TrimSpace(lines[i+1])
		if next == "}" || next == "}," || next == "]" || next == "]," {
			cur := lines[i]
			if strings.HasSuffix(strings.TrimRight(cur, " \t"), ",") {
				lines[i] = strings.TrimRight(cur, " \t,")
			}
		}
	}
	return lines
}

// --- helpers ---

// subcmd finds the kubectl subcommand in args, regardless of global flag position.
func subcmd(args []string) string {
	known := map[string]bool{
		"get": true, "describe": true, "logs": true, "log": true,
		"apply": true, "delete": true, "create": true, "patch": true,
		"exec": true, "rollout": true, "scale": true, "port-forward": true,
		"run": true, "expose": true, "edit": true, "label": true,
		"annotate": true, "config": true, "top": true, "diff": true,
		"wait": true, "version": true, "explain": true, "auth": true,
		"cp": true, "attach": true, "debug": true, "drain": true,
	}
	for _, a := range args {
		if known[a] {
			return a
		}
	}
	return ""
}

// outputFormat returns the value of -o / --output, or "" if not set.
func outputFormat(args []string) string {
	for i, a := range args {
		switch {
		case (a == "-o" || a == "--output") && i+1 < len(args):
			return args[i+1]
		case strings.HasPrefix(a, "-o="):
			return strings.TrimPrefix(a, "-o=")
		case strings.HasPrefix(a, "--output="):
			return strings.TrimPrefix(a, "--output=")
		}
	}
	return ""
}

// leadingSpaces counts the leading space characters in s.
func leadingSpaces(s string) int {
	n := 0
	for _, c := range s {
		if c != ' ' {
			break
		}
		n++
	}
	return n
}

// topLevelKey returns the key part of a top-level describe line ("Key: value" → "Key").
func topLevelKey(trimmed string) string {
	if i := strings.Index(trimmed, ":"); i > 0 {
		return trimmed[:i]
	}
	return trimmed
}

// fieldKey returns the field name from an indented describe line ("  Key:  value" → "Key").
func fieldKey(trimmed string) string {
	return topLevelKey(trimmed)
}

// yamlKey returns the YAML key from a trimmed line ("key: value" or "key:" → "key").
func yamlKey(trimmed string) string {
	if i := strings.Index(trimmed, ":"); i > 0 {
		return trimmed[:i]
	}
	return ""
}

// jsonKey returns the JSON key from a pretty-printed line ("\"key\": ..." → "key").
func jsonKey(trimmed string) string {
	if !strings.HasPrefix(trimmed, "\"") {
		return ""
	}
	end := strings.Index(trimmed[1:], "\"")
	if end < 0 {
		return ""
	}
	return trimmed[1 : end+1]
}
