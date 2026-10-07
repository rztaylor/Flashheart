package protocol

import "strings"

// AwaitCommand is the shell command ask_human gives an agent to wait for
// its answer in the background (agent-protocol §7.5). An empty binary means
// flashheart on PATH; an empty root means the default root.
func AwaitCommand(binary, root, project, question string) string {
	if binary == "" {
		binary = "flashheart"
	}
	command := ShellQuote(binary) + " await " + ShellQuote(question) + " --project " + ShellQuote(project)
	if root != "" {
		command += " --root " + ShellQuote(root)
	}
	return command
}

// ShellQuote quotes s for a POSIX shell, leaving plain words unquoted.
func ShellQuote(s string) string {
	if s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_/.-+=:@") == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
