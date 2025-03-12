package shellparse

import (
	"strings"
)

// CommandType represents the type of command or construct
type CommandType int

const (
	CommandTypeSimple CommandType = iota
	CommandTypeParenGroup
	CommandTypeSubshell
	CommandTypePipeline
	CommandTypeCommandSubstitution
	CommandTypeBacktickSubstitution
)

// ShellPart represents a single part of a shell command
type ShellPart struct {
	Command   string      `json:"command"`   // The command name (e.g., "apt-get")
	Args      string      `json:"args"`      // The raw command arguments as a string
	RawText   string      `json:"raw_text"`  // The original raw text for this part
	Delimiter string      `json:"delimiter"` // The delimiter that follows this part (e.g., "&&", "||", ";", etc.)
	Type      CommandType `json:"type"`      // Type of command
}

// ShellCommand represents a complete shell command with potentially nested structure
type ShellCommand struct {
	Parts    []*ShellPart    `json:"parts"`    // Parts of this command
	Children []*ShellCommand `json:"children"` // Nested commands (e.g., inside parentheses)
	Original string          `json:"original"` // The complete original command text
}

// String returns the original shell command text
func (sc *ShellCommand) String() string {
	if sc == nil {
		return ""
	}
	return sc.Original
}

// FindAllCommands finds all instances of a specific command by name at any nesting level
func (sc *ShellCommand) FindAllCommands(commandName string) []*ShellPart {
	var results []*ShellPart

	// Search in parts at this level
	for _, part := range sc.Parts {
		if extractCommandName(part.Command) == commandName {
			results = append(results, part)
		}
	}

	// Recursively search in children
	for _, child := range sc.Children {
		results = append(results, child.FindAllCommands(commandName)...)
	}

	return results
}

// extractCommandName extracts the base command name without path or options
func extractCommandName(cmd string) string {
	// Remove leading whitespace
	cmd = strings.TrimLeft(cmd, " \t")

	// Handle potential paths like /usr/bin/apt-get
	if idx := strings.LastIndex(cmd, "/"); idx >= 0 {
		cmd = cmd[idx+1:]
	}

	// Handle environment variables and options
	if strings.Contains(cmd, "=") || strings.HasPrefix(cmd, "-") {
		return ""
	}

	// Return the command name
	return cmd
}

// ParseShellLine parses a shell command line and returns a structured representation
func ParseShellLine(content string) *ShellCommand {
	if content == "" {
		return &ShellCommand{Original: ""}
	}

	// Create the root shell command
	command := &ShellCommand{
		Original: content,
	}

	// Tokenize and parse the command
	tokenize(command, content)

	return command
}

// tokenize performs the main parsing of the shell command
func tokenize(cmd *ShellCommand, content string) {
	var parts []*ShellPart
	var currentPart strings.Builder
	var inSingleQuote, inDoubleQuote bool
	var parenLevel int
	var lastPartEnd int

	// Keep track of the current command type and its start
	var currentType CommandType = CommandTypeSimple
	var groupStart int = -1

	// Scan the content for command substitutions first
	// This allows us to correctly handle command substitutions in quotes
	cmdSubsts := findCommandSubstitutions(content)

	for i := 0; i < len(content); i++ {
		char := content[i]

		// Handle escaping
		if char == '\\' && i+1 < len(content) {
			currentPart.WriteByte(char)
			if i+1 < len(content) {
				i++
				currentPart.WriteByte(content[i])
			}
			continue
		}

		// Handle quotes
		if char == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			currentPart.WriteByte(char)
			continue
		}
		if char == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			currentPart.WriteByte(char)
			continue
		}

		// Handle backticks (outside of single quotes)
		if char == '`' && !inSingleQuote {
			currentPart.WriteByte(char)
			continue
		}

		// Handle $( (outside of single quotes)
		if char == '$' && i+1 < len(content) && content[i+1] == '(' && !inSingleQuote {
			currentPart.WriteByte(char)
			currentPart.WriteByte(content[i+1])
			i++
			continue
		}

		// Track parentheses at the top level
		if parenLevel == 0 && !inSingleQuote && !inDoubleQuote {
			if char == '(' {
				if groupStart == -1 {
					if currentPart.Len() > 0 {
						part := &ShellPart{
							RawText: currentPart.String(),
						}
						parseCommand(part)
						parts = append(parts, part)
						currentPart.Reset()
					}

					groupStart = i
					currentType = CommandTypeParenGroup
				}

				parenLevel++
				currentPart.WriteByte(char)
				continue
			}
		} else if !inSingleQuote && !inDoubleQuote {
			// Track nesting level within a group
			if char == '(' {
				parenLevel++
				currentPart.WriteByte(char)
				continue
			}
		}

		// Handle closing parentheses for parenthesized groups
		if char == ')' && parenLevel > 0 && !inSingleQuote && !inDoubleQuote {
			parenLevel--
			currentPart.WriteByte(char)

			if parenLevel == 0 && groupStart != -1 && currentType == CommandTypeParenGroup {
				groupText := currentPart.String()
				part := &ShellPart{
					RawText: groupText,
					Type:    currentType,
				}

				innerContent := extractInnerContent(groupText, currentType)
				childCmd := &ShellCommand{
					Original: innerContent,
				}
				tokenize(childCmd, innerContent)

				cmd.Children = append(cmd.Children, childCmd)
				parts = append(parts, part)

				currentPart.Reset()
				groupStart = -1
				currentType = CommandTypeSimple
				lastPartEnd = i + 1
			}
			continue
		}

		// Only process operators at the top level outside of any groups or quotes
		if parenLevel == 0 && !inSingleQuote && !inDoubleQuote {
			var operator string

			if char == ';' {
				operator = ";"
			} else if char == '&' && i+1 < len(content) && content[i+1] == '&' {
				operator = "&&"
			} else if char == '&' {
				operator = "&"
			} else if char == '|' && i+1 < len(content) && content[i+1] == '|' {
				operator = "||"
			} else if char == '|' {
				operator = "|"
			} else if char == '>' && i+1 < len(content) && content[i+1] == '>' {
				operator = ">>"
			} else if char == '>' && i+1 < len(content) && content[i+1] == '&' {
				operator = ">&"
			} else if char == '>' {
				operator = ">"
			} else if char == '<' && i+1 < len(content) && content[i+1] == '<' {
				operator = "<<"
			} else if char == '<' && i+1 < len(content) && content[i+1] == '&' {
				operator = "<&"
			} else if char == '<' {
				operator = "<"
			}

			if operator != "" {
				if currentPart.Len() > 0 || i > lastPartEnd {
					rawText := currentPart.String()
					if currentPart.Len() == 0 && i > lastPartEnd {
						rawText = content[lastPartEnd:i]
					}

					part := &ShellPart{
						RawText:   rawText,
						Delimiter: operator,
					}
					parseCommand(part)
					parts = append(parts, part)
					currentPart.Reset()
				}

				if len(operator) > 1 {
					i += len(operator) - 1
				}

				lastPartEnd = i + 1
				continue
			}
		}

		currentPart.WriteByte(char)
	}

	// Add any remaining content as the final part
	if currentPart.Len() > 0 || lastPartEnd < len(content) {
		rawText := currentPart.String()
		if currentPart.Len() == 0 && lastPartEnd < len(content) {
			rawText = content[lastPartEnd:]
		}

		part := &ShellPart{
			RawText: rawText,
		}
		parseCommand(part)
		parts = append(parts, part)
	}

	cmd.Parts = parts

	// Process all found command substitutions
	for _, subst := range cmdSubsts {
		childCmd := &ShellCommand{
			Original: subst.content,
		}
		tokenize(childCmd, subst.content)
		cmd.Children = append(cmd.Children, childCmd)
	}
}

// CommandSubstitution represents a command substitution in the shell command
type CommandSubstitution struct {
	start   int
	end     int
	content string
	typ     CommandType
}

// findCommandSubstitutions finds all command substitutions in a shell command
func findCommandSubstitutions(content string) []CommandSubstitution {
	var result []CommandSubstitution

	// First find all backtick substitutions
	backtickPositions := findBacktickSubstitutions(content)
	for _, pos := range backtickPositions {
		result = append(result, CommandSubstitution{
			start:   pos.start,
			end:     pos.end,
			content: content[pos.start+1 : pos.end],
			typ:     CommandTypeBacktickSubstitution,
		})
	}

	// Find all $() substitutions
	dollarParenPositions := findDollarParenSubstitutions(content)
	for _, pos := range dollarParenPositions {
		result = append(result, CommandSubstitution{
			start:   pos.start,
			end:     pos.end,
			content: content[pos.start+2 : pos.end], // Skip $( and include everything up to the closing )
			typ:     CommandTypeCommandSubstitution,
		})
	}

	return result
}

// Position represents a start and end position in a string
type Position struct {
	start int
	end   int
}

// findBacktickSubstitutions finds all backtick command substitutions
func findBacktickSubstitutions(content string) []Position {
	var result []Position
	var inSingleQuote, inDoubleQuote bool
	var backtickStart = -1

	for i := 0; i < len(content); i++ {
		char := content[i]

		// Handle escaping
		if char == '\\' && i+1 < len(content) {
			i++ // Skip the next character
			continue
		}

		// Handle quotes
		if char == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}
		if char == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}

		// Only track backticks outside of single quotes
		if char == '`' && !inSingleQuote {
			if backtickStart == -1 {
				// Start of backtick substitution
				backtickStart = i
			} else {
				// End of backtick substitution
				result = append(result, Position{
					start: backtickStart,
					end:   i,
				})
				backtickStart = -1
			}
		}
	}

	return result
}

// findDollarParenSubstitutions finds all $() command substitutions
func findDollarParenSubstitutions(content string) []Position {
	var result []Position
	var inSingleQuote bool

	for i := 0; i < len(content); i++ {
		// Handle escaping
		if content[i] == '\\' && i+1 < len(content) {
			i++ // Skip the next character
			continue
		}

		// Handle single quotes - no substitutions in single quotes
		if content[i] == '\'' {
			inSingleQuote = !inSingleQuote
			continue
		}

		// Only look for $( outside of single quotes
		if !inSingleQuote && content[i] == '$' && i+1 < len(content) && content[i+1] == '(' {
			start := i
			i += 2 // Skip $( characters

			// Find the closing parenthesis
			nesting := 1
			for j := i; j < len(content); j++ {
				if content[j] == '\\' && j+1 < len(content) {
					j++ // Skip escaped chars
					continue
				}

				if content[j] == '(' && !isInSingleQuotes(content, start, j) {
					nesting++
				} else if content[j] == ')' && !isInSingleQuotes(content, start, j) {
					nesting--
					if nesting == 0 {
						result = append(result, Position{
							start: start,
							end:   j,
						})
						i = j // Jump to the closing parenthesis
						break
					}
				}
			}
		}
	}

	return result
}

// isInSingleQuotes checks if a position in a string is inside single quotes
func isInSingleQuotes(s string, start, pos int) bool {
	inSingleQuote := false
	for i := start; i < pos; i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++ // Skip escaped chars
			continue
		}
		if s[i] == '\'' {
			inSingleQuote = !inSingleQuote
		}
	}
	return inSingleQuote
}

// extractInnerContent extracts the content inside parentheses or command substitution
func extractInnerContent(text string, cmdType CommandType) string {
	if cmdType == CommandTypeParenGroup {
		if len(text) >= 2 && text[0] == '(' && text[len(text)-1] == ')' {
			return text[1 : len(text)-1]
		}
	} else if cmdType == CommandTypeCommandSubstitution {
		if len(text) >= 3 && text[0] == '$' && text[1] == '(' && text[len(text)-1] == ')' {
			return text[2 : len(text)-1]
		}
	} else if cmdType == CommandTypeBacktickSubstitution {
		if len(text) >= 2 && text[0] == '`' && text[len(text)-1] == '`' {
			return text[1 : len(text)-1]
		}
	}
	return text
}

// parseCommand extracts the command and arguments from the raw text
func parseCommand(part *ShellPart) {
	if part == nil || part.RawText == "" {
		return
	}

	trimmed := strings.TrimSpace(part.RawText)
	if trimmed == "" {
		return
	}

	if part.Type == CommandTypeParenGroup || part.Type == CommandTypeCommandSubstitution {
		return
	}

	tokens := tokenizeLine(trimmed)
	if len(tokens) == 0 {
		return
	}

	cmdIndex := 0
	for i, token := range tokens {
		if !strings.Contains(token, "=") {
			cmdIndex = i
			break
		}
	}

	if cmdIndex >= len(tokens) {
		return
	}

	part.Command = tokens[cmdIndex]

	if cmdIndex+1 < len(tokens) {
		argsStart := strings.Index(trimmed, tokens[cmdIndex]) + len(tokens[cmdIndex])
		if argsStart < len(trimmed) {
			// Get the arguments and trim whitespace from both sides
			args := strings.TrimLeft(trimmed[argsStart:], " \t")

			// Remove trailing backslashes followed by whitespace or newlines
			// This handles line continuations
			args = strings.TrimRightFunc(args, func(r rune) bool {
				return r == '\\' || r == ' ' || r == '\t' || r == '\n'
			})

			part.Args = args
		}
	}
}

// tokenizeLine splits a command line into tokens, respecting quotes and escapes
func tokenizeLine(line string) []string {
	var tokens []string
	var currentToken strings.Builder
	var inSingleQuote, inDoubleQuote bool

	for i := 0; i < len(line); i++ {
		char := line[i]

		if char == '\\' && i+1 < len(line) {
			currentToken.WriteByte(char)
			if i+1 < len(line) {
				i++
				currentToken.WriteByte(line[i])
			}
			continue
		}

		if char == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			currentToken.WriteByte(char)
			continue
		}
		if char == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			currentToken.WriteByte(char)
			continue
		}

		if inSingleQuote || inDoubleQuote {
			currentToken.WriteByte(char)
			continue
		}

		if char == ' ' || char == '\t' {
			if currentToken.Len() > 0 {
				tokens = append(tokens, currentToken.String())
				currentToken.Reset()
			}
			continue
		}

		currentToken.WriteByte(char)
	}

	if currentToken.Len() > 0 {
		tokens = append(tokens, currentToken.String())
	}

	return tokens
}

// GetFullCommand returns the full command with arguments
func (sp *ShellPart) GetFullCommand() string {
	if sp.Command == "" {
		return sp.RawText
	}

	if sp.Args == "" {
		return sp.Command
	}

	return sp.Command + " " + sp.Args
}

// GetCommandsByExe returns all commands that use the specified executable name
// An "exe" is the actual command being run in bash (e.g., "apt-get", "docker", "git")
func (sc *ShellCommand) GetCommandsByExe(exeName string) []string {
	var results []string

	// Find all instances of the specified command
	commandParts := sc.FindAllCommands(exeName)

	// Extract the full command strings with arguments
	for _, part := range commandParts {
		results = append(results, part.GetFullCommand())
	}

	return results
}

// GetCommandsByMultiExe returns all commands that use any of the specified executable names
// This is useful for commands that might have aliases (e.g., "yum"/"dnf", "python"/"python3")
func (sc *ShellCommand) GetCommandsByMultiExe(exeNames []string) []string {
	var results []string

	// Check each executable name
	for _, exeName := range exeNames {
		// Find commands for this executable
		commands := sc.GetCommandsByExe(exeName)

		// Add commands to results
		results = append(results, commands...)
	}

	return results
}

// GetCommandsByExeAndSubcommand returns commands that use the specified executable and subcommand
// For example: sc.GetCommandsByExeAndSubcommand("apt-get", "install") will find all "apt-get install" commands
func (sc *ShellCommand) GetCommandsByExeAndSubcommand(exeName string, subcommand string) []string {
	var results []string

	// Find all instances of the specified command
	commandParts := sc.FindAllCommands(exeName)

	// Filter for the specified subcommand
	for _, part := range commandParts {
		// Check if args starts with the subcommand
		if strings.HasPrefix(part.Args, subcommand) {
			// Make sure it's a complete subcommand match (followed by space or end of string)
			if len(part.Args) == len(subcommand) ||
				(len(part.Args) > len(subcommand) &&
					(part.Args[len(subcommand)] == ' ' || part.Args[len(subcommand)] == '\t')) {
				results = append(results, part.GetFullCommand())
			}
		}
	}

	return results
}

// GetCommandsByMultiExeAndSubcommand returns commands that use any of the specified executables with the given subcommand
// This is useful for commands with aliases that share the same subcommands (e.g., "yum install"/"dnf install")
func (sc *ShellCommand) GetCommandsByMultiExeAndSubcommand(exeNames []string, subcommand string) []string {
	var results []string

	// Check each executable name
	for _, exeName := range exeNames {
		// Find commands for this executable and subcommand
		commands := sc.GetCommandsByExeAndSubcommand(exeName, subcommand)

		// Add commands to results
		results = append(results, commands...)
	}

	return results
}

// ReplaceCommand replaces a specific command with a new one in the shell command tree
func (sc *ShellCommand) ReplaceCommand(oldCmd string, newCmd string) {
	// Replace in parts at this level
	for i, part := range sc.Parts {
		if part.GetFullCommand() == oldCmd {
			// Create a new part with the updated command
			newPart := &ShellPart{
				RawText:   newCmd,
				Delimiter: part.Delimiter,
				Type:      part.Type,
			}
			parseCommand(newPart)
			sc.Parts[i] = newPart
		}
	}

	// Recursively replace in children
	for _, child := range sc.Children {
		child.ReplaceCommand(oldCmd, newCmd)
	}
}

// RemoveCommand removes a specific command from the shell command tree
func (sc *ShellCommand) RemoveCommand(cmd string) {
	// Remove parts that match the command
	var newParts []*ShellPart

	for i, part := range sc.Parts {
		if part.GetFullCommand() == cmd {
			// Skip this part, but if it's not the last part and there are previous parts,
			// preserve its delimiter for the previous part
			if i > 0 && i < len(sc.Parts)-1 && len(newParts) > 0 {
				newParts[len(newParts)-1].Delimiter = part.Delimiter
			}
		} else {
			newParts = append(newParts, part)
		}
	}

	sc.Parts = newParts

	// Recursively remove from children
	for _, child := range sc.Children {
		child.RemoveCommand(cmd)
	}
}

// FilterCommands removes commands that match a specific pattern
func (sc *ShellCommand) FilterCommands(shouldKeep func(cmd *ShellPart) bool) {
	// Filter parts at this level
	var newParts []*ShellPart
	for i, part := range sc.Parts {
		if shouldKeep(part) {
			newParts = append(newParts, part)
		} else if i > 0 && i < len(sc.Parts)-1 && part.Delimiter != "" && len(newParts) > 0 {
			// If removing a middle part, preserve its delimiter for the previous part
			newParts[len(newParts)-1].Delimiter = part.Delimiter
		}
	}
	sc.Parts = newParts

	// Recursively filter in children
	for _, child := range sc.Children {
		child.FilterCommands(shouldKeep)
	}
}

// Reconstruct returns the reconstructed shell command as a string
func (sc *ShellCommand) Reconstruct() string {
	if len(sc.Parts) == 0 {
		return ""
	}

	var result strings.Builder

	// Check if the original command had line continuations
	hasLineContinuations := strings.Contains(sc.Original, "\\\n")

	for i, part := range sc.Parts {
		// Preserve original formatting of the command part as much as possible
		result.WriteString(part.RawText)

		// Add delimiter if present
		if part.Delimiter != "" && i < len(sc.Parts)-1 {
			if hasLineContinuations {
				// Try to preserve original whitespace around delimiters
				original := sc.Original
				rawCmd := part.RawText
				delimIdx := strings.Index(original, rawCmd) + len(rawCmd)

				// Look for the delimiter in the original string
				delimStr := " " + part.Delimiter + " "
				if delimIdx < len(original) {
					// Find the delimiter with surrounding whitespace
					for j := delimIdx; j < len(original); j++ {
						if strings.HasPrefix(original[j:], part.Delimiter) {
							// Extract the whitespace before and after
							beforeSpace := ""
							for k := j - 1; k >= 0 && (original[k] == ' ' || original[k] == '\t'); k-- {
								beforeSpace = string(original[k]) + beforeSpace
							}

							afterSpace := ""
							delimEnd := j + len(part.Delimiter)
							for k := delimEnd; k < len(original) && (original[k] == ' ' || original[k] == '\t' || original[k] == '\\' || original[k] == '\n'); k++ {
								afterSpace += string(original[k])
							}

							delimStr = beforeSpace + part.Delimiter + afterSpace
							break
						}
					}
				}

				result.WriteString(delimStr)
			} else {
				result.WriteString(" " + part.Delimiter + " ")
			}
		}
	}

	// If the result doesn't match any of the expected line continuations,
	// but the original had them, try to preserve them
	reconstructed := result.String()
	if hasLineContinuations && !strings.Contains(reconstructed, "\\\n") {
		// Simple approach: replace spaces before newlines with line continuations
		reconstructed = strings.ReplaceAll(reconstructed, " \n", " \\\n")
	}

	return reconstructed
}

// Clone creates a deep copy of the ShellCommand
func (sc *ShellCommand) Clone() *ShellCommand {
	if sc == nil {
		return nil
	}

	clone := &ShellCommand{
		Original: sc.Original,
	}

	// Clone all parts
	for _, part := range sc.Parts {
		clonedPart := &ShellPart{
			Command:   part.Command,
			Args:      part.Args,
			RawText:   part.RawText,
			Delimiter: part.Delimiter,
			Type:      part.Type,
		}
		clone.Parts = append(clone.Parts, clonedPart)
	}

	// Clone all children
	for _, child := range sc.Children {
		clone.Children = append(clone.Children, child.Clone())
	}

	return clone
}
