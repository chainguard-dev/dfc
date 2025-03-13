package shellparse2

import (
	"strings"
)

// Node represents a node in the command tree
type Node struct {
	Type      NodeType
	Command   string
	Args      []string
	Parts     []string
	Raw       string
	Indent    string
	LineBreak string
	Prefix    string
	Suffix    string
	Next      *Node
	Prev      *Node
}

// NodeType represents the type of a node
type NodeType int

const (
	NodeCommand NodeType = iota
	NodeOperator
	NodeText
)

// ShellCommand represents a shell command
type ShellCommand struct {
	RootNode *Node
	Raw      string
}

// NewShellCommand creates a new shell command
func NewShellCommand(cmd string) *ShellCommand {
	sc := &ShellCommand{
		Raw: cmd,
	}
	sc.RootNode = sc.parse(cmd)
	return sc
}

// parse parses a shell command
func (sc *ShellCommand) parse(cmd string) *Node {
	// Process the command
	result := &Node{
		Type: NodeText,
		Raw:  cmd,
	}

	// Split by logical operators (&&, ||, ;)
	parts := splitByOperators(cmd)

	if len(parts) == 0 {
		return result
	}

	// Build the node tree
	var rootNode *Node
	var lastNode *Node

	for _, part := range parts {
		if isOperator(part) {
			// This is an operator
			node := &Node{
				Type: NodeOperator,
				Raw:  part,
			}

			if rootNode == nil {
				rootNode = node
			} else {
				lastNode.Next = node
				node.Prev = lastNode
			}

			lastNode = node
		} else {
			// This is a command or text
			// Process continuations and indentation
			actualCmd, indent, lineBreak, prefix := processCommand(part)
			actualCmd = strings.TrimSpace(actualCmd)

			node := &Node{
				Type:      NodeCommand,
				Raw:       part,
				Indent:    indent,
				LineBreak: lineBreak,
				Prefix:    prefix,
			}

			// Try to parse command and arguments
			if actualCmd != "" {
				cmdParts := splitCommand(actualCmd)

				if len(cmdParts) > 0 {
					node.Command = cmdParts[0]
					if len(cmdParts) > 1 {
						node.Args = cmdParts[1:]
					}
					node.Parts = cmdParts
				}
			}

			if rootNode == nil {
				rootNode = node
			} else {
				lastNode.Next = node
				node.Prev = lastNode
			}

			lastNode = node
		}
	}

	return rootNode
}

// splitByOperators splits a command by logical operators
func splitByOperators(cmd string) []string {
	var result []string
	current := ""
	inQuote := false
	quoteChar := byte(' ')
	escaped := false

	for i := 0; i < len(cmd); i++ {
		c := cmd[i]

		if escaped {
			current += string(c)
			escaped = false
			continue
		}

		if c == '\\' {
			current += string(c)
			escaped = true
			continue
		}

		if c == '"' || c == '\'' {
			if inQuote && quoteChar == c {
				inQuote = false
			} else if !inQuote {
				inQuote = true
				quoteChar = c
			}
			current += string(c)
			continue
		}

		if !inQuote && i < len(cmd)-1 {
			if (c == '&' && cmd[i+1] == '&') || (c == '|' && cmd[i+1] == '|') {
				if current != "" {
					result = append(result, current)
					current = ""
				}
				result = append(result, string(c)+string(cmd[i+1]))
				i++
				continue
			} else if c == ';' {
				if current != "" {
					result = append(result, current)
					current = ""
				}
				result = append(result, string(c))
				continue
			}
		}

		current += string(c)
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}

// isOperator checks if a string is a logical operator
func isOperator(s string) bool {
	return s == "&&" || s == "||" || s == ";"
}

// processCommand processes a command to extract indentation and line breaks
func processCommand(cmd string) (string, string, string, string) {
	var actualCmd strings.Builder
	var indent strings.Builder
	var lineBreak strings.Builder

	// Extract leading whitespace as prefix
	prefix := ""
	i := 0
	for ; i < len(cmd); i++ {
		if !isWhitespace(cmd[i]) {
			break
		}
	}
	if i > 0 {
		prefix = cmd[:i]
	}

	lines := strings.Split(cmd, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if i > 0 {
			// Add line break and indentation for previous line
			lineBreak.WriteString("\n")

			// Extract indentation
			j := 0
			for ; j < len(line); j++ {
				if !isWhitespace(line[j]) {
					break
				}
				indent.WriteByte(line[j])
			}
		}

		// Handle line continuation
		if i < len(lines)-1 && strings.HasSuffix(trimmed, "\\") {
			actualCmd.WriteString(trimmed[:len(trimmed)-1])
			actualCmd.WriteString(" ")
		} else {
			actualCmd.WriteString(trimmed)
		}
	}

	return actualCmd.String(), indent.String(), lineBreak.String(), prefix
}

// isWhitespace checks if a character is whitespace
func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t'
}

// splitCommand splits a command into parts
func splitCommand(cmd string) []string {
	var result []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(' ')
	escaped := false

	for i := 0; i < len(cmd); i++ {
		c := cmd[i]

		if escaped {
			current.WriteByte(c)
			escaped = false
			continue
		}

		if c == '\\' {
			current.WriteByte(c)
			escaped = true
			continue
		}

		if c == '"' || c == '\'' {
			if inQuote && quoteChar == c {
				inQuote = false
			} else if !inQuote {
				inQuote = true
				quoteChar = c
			}
			current.WriteByte(c)
			continue
		}

		if !inQuote && isWhitespace(c) {
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteByte(c)
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}

// FindCommandsByPrefix finds all commands that start with a given prefix
func (sc *ShellCommand) FindCommandsByPrefix(prefix string) []Node {
	var result []Node
	currentNode := sc.RootNode

	for currentNode != nil {
		if currentNode.Type == NodeCommand && strings.HasPrefix(currentNode.Command, prefix) {
			result = append(result, *currentNode)
		}
		currentNode = currentNode.Next
	}

	return result
}

// FindCommandsByPrefixAndSubcommand finds all commands that start with a given prefix and have a specific subcommand
func (sc *ShellCommand) FindCommandsByPrefixAndSubcommand(prefix, subcommand string) []Node {
	var result []Node
	currentNode := sc.RootNode

	for currentNode != nil {
		if currentNode.Type == NodeCommand &&
			strings.HasPrefix(currentNode.Command, prefix) &&
			len(currentNode.Args) > 0 &&
			currentNode.Args[0] == subcommand {
			result = append(result, *currentNode)
		}
		currentNode = currentNode.Next
	}

	return result
}

// ReplaceCommand replaces a command with a new command
func (sc *ShellCommand) ReplaceCommand(node Node, newCmd string) {
	// Find the matching node
	currentNode := sc.RootNode
	for currentNode != nil {
		if currentNode.Type == node.Type &&
			currentNode.Command == node.Command &&
			strings.Join(currentNode.Args, " ") == strings.Join(node.Args, " ") {

			// Parse the new command
			cmdParts := splitCommand(newCmd)
			if len(cmdParts) > 0 {
				currentNode.Command = cmdParts[0]
				if len(cmdParts) > 1 {
					currentNode.Args = cmdParts[1:]
				} else {
					currentNode.Args = []string{}
				}
				currentNode.Parts = cmdParts
			}

			// Preserve the original formatting
			if strings.Contains(currentNode.Raw, "\n") {
				// This is a multi-line command, format it nicely
				lines := strings.Split(currentNode.Raw, "\n")
				if len(lines) > 1 {
					// Get the indentation from the second line
					indent := ""
					for i := 0; i < len(lines[1]); i++ {
						if !isWhitespace(lines[1][i]) {
							break
						}
						indent += string(lines[1][i])
					}

					// Format the new command with the same indentation
					currentNode.Raw = newCmd
				} else {
					currentNode.Raw = currentNode.Prefix + newCmd + currentNode.LineBreak
				}
			} else {
				// Single-line command
				currentNode.Raw = currentNode.Prefix + newCmd + currentNode.LineBreak
			}

			return
		}
		currentNode = currentNode.Next
	}
}

// RemoveCommand removes a command from the command tree
func (sc *ShellCommand) RemoveCommand(node Node) {
	// Find the matching node
	currentNode := sc.RootNode
	for currentNode != nil {
		if currentNode.Type == node.Type &&
			currentNode.Command == node.Command &&
			strings.Join(currentNode.Args, " ") == strings.Join(node.Args, " ") {

			// Handle the removal
			if currentNode.Prev != nil {
				// If this is the first command after an operator, preserve the whitespace
				if currentNode.Prev.Type == NodeOperator {
					// Keep the whitespace for the next node
					if currentNode.Next != nil {
						currentNode.Next.Prefix = currentNode.Prefix
					}
				}

				currentNode.Prev.Next = currentNode.Next
			} else {
				sc.RootNode = currentNode.Next
			}

			if currentNode.Next != nil {
				currentNode.Next.Prev = currentNode.Prev
			}

			return
		}
		currentNode = currentNode.Next
	}
}

// String returns the string representation of the command
func (sc *ShellCommand) String() string {
	var result strings.Builder
	currentNode := sc.RootNode

	for currentNode != nil {
		if currentNode.Type == NodeCommand {
			if len(currentNode.Parts) > 0 {
				result.WriteString(currentNode.Prefix)
				result.WriteString(currentNode.Indent)
				result.WriteString(strings.Join(currentNode.Parts, " "))
				result.WriteString(currentNode.LineBreak)
			} else {
				result.WriteString(currentNode.Raw)
			}
		} else {
			result.WriteString(currentNode.Raw)
		}

		currentNode = currentNode.Next
	}

	return result.String()
}

// ExtractPackagesFromInstallCommand extracts package names from an install command
func ExtractPackagesFromInstallCommand(node Node) []string {
	var packages []string

	// We assume the command is structured like:
	// [package-manager] [subcommand] [options] [package1] [package2] ...
	// Skip the package manager, subcommand, and any options

	skipCount := 2 // Skip the package manager and subcommand
	for i, arg := range node.Args {
		if i < skipCount {
			continue
		}

		// Skip options (starting with -)
		if strings.HasPrefix(arg, "-") {
			continue
		}

		// If a next arg is equals, treat them as a unit
		if strings.HasSuffix(arg, "=") && i+1 < len(node.Args) {
			i++
			continue
		}

		// If this looks like a plain package name, add it
		if !strings.Contains(arg, "=") && !strings.HasPrefix(arg, "$") {
			packages = append(packages, arg)
		}
	}

	return packages
}
