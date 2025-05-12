package apko

import (
	"fmt"
	"log"
	"strings"

	"github.com/chainguard-dev/dfc/pkg/dfc"
)

// Debug flag controls whether debug logging is enabled
var Debug bool = false

// ApkoConfig represents the apko YAML configuration
type ApkoConfig struct {
	Contents struct {
		Repositories []string `yaml:"repositories"`
		Packages     []string `yaml:"packages"`
	} `yaml:"contents"`
	Entrypoint struct {
		Command string `yaml:"command,omitempty"`
		Type    string `yaml:"type,omitempty"`
	} `yaml:"entrypoint,omitempty"`
	WorkDir     string            `yaml:"work-dir,omitempty"`
	Environment map[string]string `yaml:"environment,omitempty"`
	Accounts    struct {
		Users  []User  `yaml:"users,omitempty"`
		Groups []Group `yaml:"groups,omitempty"`
		RunAs  string  `yaml:"run-as,omitempty"`
	} `yaml:"accounts,omitempty"`
	Paths []Path `yaml:"paths,omitempty"`
}

type User struct {
	Username string `yaml:"username"`
	UID      int    `yaml:"uid"`
	Shell    string `yaml:"shell,omitempty"`
}

type Group struct {
	Groupname string `yaml:"groupname"`
	GID       int    `yaml:"gid"`
}

type Path struct {
	Path        string `yaml:"path"`
	Type        string `yaml:"type"`
	UID         int    `yaml:"uid,omitempty"`
	GID         int    `yaml:"gid,omitempty"`
	Permissions string `yaml:"permissions,omitempty"`
	Source      string `yaml:"source,omitempty"`
}

type BuildStage struct {
	Name      string
	BaseImage string
	Packages  []string
	EnvVars   map[string]string
	WorkDir   string
	Users     []User
	Groups    []Group
	Services  []string
	Paths     []Path
	FromStage string
}

func parseEntrypointOrCmd(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		// JSON array form: ENTRYPOINT ["/bin/sh", "-c", "echo hello"]
		raw = strings.Trim(raw, "[]")
		parts := strings.Split(raw, ",")
		for i := range parts {
			parts[i] = strings.Trim(parts[i], " \"")
		}
		return strings.Join(parts, " ")
	}
	return raw
}

func parseEnvLine(parts []string) map[string]string {
	envs := make(map[string]string)
	for _, kv := range parts[1:] {
		if strings.Contains(kv, "=") {
			pair := strings.SplitN(kv, "=", 2)
			envs[pair[0]] = pair[1]
		}
	}
	return envs
}

func parseCopyChown(parts []string) (src, dest, chown, chmod string) {
	src, dest, chown, chmod = "", "", "", ""
	for i := 1; i < len(parts); i++ {
		if parts[i] == "--chown" && i+1 < len(parts) {
			chown = parts[i+1]
			i++
		} else if parts[i] == "--chmod" && i+1 < len(parts) {
			chmod = parts[i+1]
			i++
		} else if src == "" {
			src = parts[i]
		} else if dest == "" {
			dest = parts[i]
		}
	}
	return
}

func parsePackageInstall(fullCommand string) []string {
	if Debug {
		log.Printf("DEBUG: parsePackageInstall received full command: %s", fullCommand)
	}
	allPackages := []string{}

	// Split the full command into sub-commands based on shell operators
	// We need to be careful with escaped operators, but for now, a simple split.
	// Replace || and ; with && to simplify splitting, then split by &&.
	// This is a simplification; a proper shell parser would be more robust.
	processedCommand := strings.ReplaceAll(fullCommand, "||", "&&")
	processedCommand = strings.ReplaceAll(processedCommand, ";", "&&")
	subCommands := strings.Split(processedCommand, "&&")

	for _, cmd := range subCommands {
		trimmedCmd := strings.TrimSpace(cmd)
		if trimmedCmd == "" {
			continue
		}
		if Debug {
			log.Printf("DEBUG: Processing sub-command: [%s]", trimmedCmd)
		}

		parts := strings.Fields(trimmedCmd)
		if len(parts) < 2 { // Must have at least <manager> <action>
			if Debug {
				log.Printf("DEBUG: Sub-command too short or not a recognized format: [%s]", trimmedCmd)
			}
			continue
		}

		cmdPrefix := ""
		packageManager := parts[0]
		action := ""
		packageStartIndex := -1

		if len(parts) > 1 {
			action = parts[1]
		}

		// Check for known package manager commands
		if (packageManager == "apk" && action == "add") || (packageManager == "clean-install") {
			cmdPrefix = packageManager
			if packageManager == "clean-install" {
				packageStartIndex = 1 // packages start after "clean-install"
			} else { // apk add
				packageStartIndex = 2 // packages start after "apk add"
			}
		} else if (packageManager == "apt-get" || packageManager == "apt") && action == "install" {
			cmdPrefix = packageManager
			packageStartIndex = 2 // packages start after "apt-get install" or "apt install"
		} else if packageManager == "yum" || packageManager == "dnf" || packageManager == "microdnf" {
			// Handle cases like "yum -y install" or "yum install -y"
			if action == "install" {
				cmdPrefix = packageManager
				packageStartIndex = 2
			} else if len(parts) > 2 && strings.HasPrefix(action, "-") && parts[2] == "install" { // e.g. yum -y install
				cmdPrefix = packageManager
				action = parts[2]     // Correctly set action to "install"
				packageStartIndex = 3 // Packages start after "yum -y install"
			}
		}

		if packageStartIndex != -1 && len(parts) > packageStartIndex {
			// Take the part of the command that should contain packages
			packageRelevantParts := parts[packageStartIndex:]
			var currentCommandPackages []string

			// Iterate through parts to strip flags and stop at further shell operators (though less likely here due to pre-split)
			for _, part := range packageRelevantParts {
				// Stop if we hit a shell operator (should ideally not happen if pre-split was perfect)
				// or a path-like string or redirection, or variable expansion.
				if part == "&&" || part == "||" || part == "|" || part == ";" ||
					strings.Contains(part, "/") || // Basic check for paths
					strings.HasPrefix(part, "$") || // Variable expansion
					strings.Contains(part, "=") || // Variable assignment or flag with value
					part == ">" || part == "<" { // Redirection
					if Debug {
						log.Printf("DEBUG: Stopping package parsing at part: [%s] for sub-command: [%s]", part, trimmedCmd)
					}
					break
				}

				// Skip flags (simple check for prefix)
				if strings.HasPrefix(part, "-") {
					// This won't handle flags that take a separate value (e.g., -o file) perfectly.
					// It assumes flags are self-contained or combined (e.g., -y, --yes, --option=value).
					if Debug {
						log.Printf("DEBUG: Skipping flag: [%s] in sub-command: [%s]", part, trimmedCmd)
					}
					continue
				}
				// Assume what's left is a package
				if part != "" {
					currentCommandPackages = append(currentCommandPackages, part)
				}
			}

			if len(currentCommandPackages) > 0 {
				if Debug {
					log.Printf("DEBUG: Extracted packages: %v from %s install in sub-command: [%s]", currentCommandPackages, cmdPrefix, trimmedCmd)
				}
				allPackages = append(allPackages, currentCommandPackages...)
			} else {
				if Debug {
					log.Printf("DEBUG: No packages extracted after filtering for sub-command: [%s]", trimmedCmd)
				}
			}
		} else {
			if Debug {
				log.Printf("DEBUG: Sub-command not a recognized package install operation: [%s]", trimmedCmd)
			}
		}
	}

	if len(allPackages) > 0 {
		// Deduplicate packages before returning
		seen := make(map[string]bool)
		result := []string{}
		for _, pkg := range allPackages {
			if !seen[pkg] {
				seen[pkg] = true
				result = append(result, pkg)
			}
		}
		if Debug {
			log.Printf("DEBUG: Final packages from full command [%s]: %v", fullCommand, result)
		}
		return result
	}

	if Debug {
		log.Printf("DEBUG: No packages extracted from full command: [%s]", fullCommand)
	}
	return nil
}

func parseUserAdd(cmd string) (User, error) {
	// Basic parsing of adduser command
	// In reality, this would need to handle all adduser options
	parts := strings.Fields(cmd)
	if len(parts) < 3 {
		return User{}, fmt.Errorf("invalid adduser command")
	}
	return User{
		Username: parts[2],
		UID:      1000, // Default UID
	}, nil
}

func parseServiceEnable(cmd string) string {
	if strings.Contains(cmd, "systemctl enable") {
		parts := strings.Fields(cmd)
		for i, part := range parts {
			if part == "enable" && i+1 < len(parts) {
				return parts[i+1]
			}
		}
	}
	return ""
}

// ConvertDockerfileToApko converts a Chainguard Dockerfile to apko configuration
// For multi-stage builds, it returns a map of stage names to their ApkoConfig.
func ConvertDockerfileToApko(dockerfile *dfc.Dockerfile) (map[string]*ApkoConfig, error) {
	stageConfigs := make(map[string]*ApkoConfig)
	var currentConfig *ApkoConfig
	var currentStageName string
	stageIndex := 0

	// Initialize maps for the current stage's context
	seenPackages := make(map[string]bool)
	// seenPaths := make(map[string]bool) // Path de-duplication should be per-stage
	// seenUsers := make(map[string]bool) // User de-duplication should be per-stage
	// seenServices := make(map[string]bool) // Service de-duplication should be per-stage

	for _, line := range dockerfile.Lines {
		if line.From != nil {
			// Start new stage
			if line.From.Alias != "" {
				currentStageName = line.From.Alias
			} else {
				currentStageName = fmt.Sprintf("stage%d", stageIndex)
				stageIndex++
			}

			currentConfig = &ApkoConfig{
				Contents: struct {
					Repositories []string `yaml:"repositories"`
					Packages     []string `yaml:"packages"`
				}{
					Packages: make([]string, 0),
				},
				Accounts: struct {
					Users  []User  `yaml:"users,omitempty"`
					Groups []Group `yaml:"groups,omitempty"`
					RunAs  string  `yaml:"run-as,omitempty"`
				}{
					Users:  make([]User, 0),
					Groups: make([]Group, 0),
				},
				Entrypoint: struct {
					Command string `yaml:"command,omitempty"`
					Type    string `yaml:"type,omitempty"`
				}{},
				Environment: make(map[string]string),
				Paths:       make([]Path, 0),
			}
			stageConfigs[currentStageName] = currentConfig
			// Reset seen items for the new stage
			seenPackages = make(map[string]bool)
			// seenPaths = make(map[string]bool) // Initialize if needed per stage
			// seenUsers = make(map[string]bool) // Initialize if needed per stage
			// seenServices = make(map[string]bool) // Initialize if needed per stage

			// Handle base image packages for the current stage's config
			base := strings.TrimPrefix(line.From.Base, "cgr.dev/") // Assuming cgr.dev/ORG/img format
			// Further base image processing might be needed if we want to inherit from previous stage's packages in apko
			// For now, we just note the base. Apko doesn't directly support FROM like Docker.
			// We can add common packages based on image name if desired.
			if strings.Contains(base, "alpine") && !strings.Contains(base, "distroless") && !strings.Contains(base, "static") {
				if _, ok := seenPackages["alpine-base"]; !ok {
					currentConfig.Contents.Packages = append(currentConfig.Contents.Packages, "alpine-base")
					seenPackages["alpine-base"] = true
				}
			}
			// Example: add nodejs if base image suggests it
			if strings.Contains(base, "nodejs") {
				if _, ok := seenPackages["nodejs"]; !ok {
					currentConfig.Contents.Packages = append(currentConfig.Contents.Packages, "nodejs")
					seenPackages["nodejs"] = true
				}
			}
			if strings.Contains(base, "python") {
				if _, ok := seenPackages["python3"]; !ok {
					currentConfig.Contents.Packages = append(currentConfig.Contents.Packages, "python3") // or specific version
					seenPackages["python3"] = true
				}
			}

		}

		if currentConfig == nil {
			// This case should ideally not happen if Dockerfile is valid and starts with FROM
			// Or, we can decide to have a default "base" stage if no FROM is found first.
			// For now, skip lines until a FROM is processed.
			continue
		}

		// Process instructions based on line.Raw, as specific fields might not exist for all commands
		parts := strings.Fields(line.Raw)
		if len(parts) == 0 {
			continue
		}
		instruction := strings.ToUpper(parts[0])

		switch instruction {
		case "RUN":
			if line.Run != nil && line.Run.Shell != nil && line.Run.Shell.Before != nil {
				// Build the command string - use the converted command if available
				var cmdBuilder strings.Builder
				if line.Converted != "" {
					// Use the converted command string, skipping the "RUN " prefix
					if strings.HasPrefix(line.Converted, "RUN ") {
						cmdBuilder.WriteString(strings.TrimPrefix(line.Converted, "RUN "))
					} else {
						cmdBuilder.WriteString(line.Converted)
					}
				} else {
					// Fall back to original command if no conversion available
					for _, part := range line.Run.Shell.Before.Parts {
						cmdBuilder.WriteString(part.Command)
						if len(part.Args) > 0 {
							cmdBuilder.WriteString(" ")
							cmdBuilder.WriteString(strings.Join(part.Args, " "))
						}
						if part.Delimiter != "" {
							cmdBuilder.WriteString(" ")
							cmdBuilder.WriteString(part.Delimiter)
							cmdBuilder.WriteString(" ")
						}
					}
				}
				cmd := cmdBuilder.String()

				if Debug {
					log.Printf("DEBUG: Using command for package parsing: [%s]", cmd)
				}

				pkgs := parsePackageInstall(cmd)
				for _, pkg := range pkgs {
					if _, ok := seenPackages[pkg]; !ok {
						currentConfig.Contents.Packages = append(currentConfig.Contents.Packages, pkg)
						seenPackages[pkg] = true
					}
				}

				if strings.Contains(cmd, "adduser") {
					// Simplified: assumes 'adduser <username>' or similar
					cmdParts := strings.Fields(cmd) // Re-split the reconstructed command
					var username string
					var shell string

					for i, p := range cmdParts {
						if strings.ToLower(p) == "adduser" && i+1 < len(cmdParts) {
							// Check for options like -s or --shell
							if cmdParts[i+1] == "-s" || cmdParts[i+1] == "--shell" {
								if i+2 < len(cmdParts) { // shell path
									shell = cmdParts[i+2]
									if i+3 < len(cmdParts) { // username after shell
										username = cmdParts[i+3]
									}
								}
							} else if cmdParts[i+1] == "-D" { // Common busybox adduser flag
								if i+2 < len(cmdParts) {
									username = cmdParts[i+2]
								}
							} else {
								username = cmdParts[i+1] // simple 'adduser username'
							}
							break // Found adduser and processed
						}
					}
					if username == "" && len(cmdParts) > 1 && strings.ToLower(cmdParts[0]) == "adduser" { // fallback
						username = cmdParts[len(cmdParts)-1]
					}

					if username != "" {
						userExists := false
						for _, u := range currentConfig.Accounts.Users {
							if u.Username == username {
								userExists = true
								break
							}
						}
						if !userExists {
							newUser := User{Username: username, UID: 1000 + len(currentConfig.Accounts.Users)}
							if shell != "" {
								newUser.Shell = shell
							}
							currentConfig.Accounts.Users = append(currentConfig.Accounts.Users, newUser)
						}
					}
				}
			}

		case "ENV":
			if len(parts) > 1 {
				// ENV can be key=value or key value
				if strings.Contains(parts[1], "=") {
					// Handles ENV key=value key2=value2 ...
					for _, kvPair := range parts[1:] {
						pair := strings.SplitN(kvPair, "=", 2)
						if len(pair) == 2 {
							currentConfig.Environment[strings.TrimSpace(pair[0])] = strings.TrimSpace(pair[1])
						}
					}
				} else if len(parts) == 3 {
					// Handles ENV key value
					currentConfig.Environment[strings.TrimSpace(parts[1])] = strings.TrimSpace(parts[2])
				}
			}

		case "WORKDIR":
			if len(parts) > 1 {
				currentConfig.WorkDir = parts[1]
			}

		case "USER":
			if len(parts) > 1 {
				currentConfig.Accounts.RunAs = parts[1]
			}

		case "ENTRYPOINT":
			if len(parts) > 1 {
				cmd := strings.Join(parts[1:], " ")
				currentConfig.Entrypoint.Command = parseEntrypointOrCmd(cmd)
			}

		case "CMD":
			if len(parts) > 1 {
				cmd := strings.Join(parts[1:], " ")
				if currentConfig.Entrypoint.Command == "" {
					currentConfig.Entrypoint.Command = parseEntrypointOrCmd(cmd)
				} else {
					if !strings.HasPrefix(currentConfig.Entrypoint.Command, "[") { // Simplistic check for shell form
						currentConfig.Entrypoint.Command += " " + parseEntrypointOrCmd(cmd)
					}
				}
			}
		case "COPY", "ADD": // Treat ADD as COPY for path purposes
			if len(parts) >= 3 { // Need at least COPY src dest
				var srcs []string
				var dest, chownVal, chmodVal, fromVal string

				// Parse flags like --chown, --chmod, --from
				idx := 1
				for idx < len(parts) { // iterate through potential flags and sources
					currentPart := parts[idx]
					if strings.HasPrefix(currentPart, "--chown=") {
						chownVal = strings.SplitN(currentPart, "=", 2)[1]
						idx++
						continue
					} else if currentPart == "--chown" && idx+1 < len(parts) {
						chownVal = parts[idx+1]
						idx += 2
						continue
					}

					if strings.HasPrefix(currentPart, "--chmod=") {
						chmodVal = strings.SplitN(currentPart, "=", 2)[1]
						idx++
						continue
					} else if currentPart == "--chmod" && idx+1 < len(parts) {
						chmodVal = parts[idx+1]
						idx += 2
						continue
					}

					if strings.HasPrefix(currentPart, "--from=") {
						fromVal = strings.SplitN(currentPart, "=", 2)[1]
						idx++
						continue
					} else if currentPart == "--from" && idx+1 < len(parts) {
						fromVal = parts[idx+1]
						idx += 2
						continue
					}
					// If it's not a flag, it's a source or destination
					break // Break to collect sources and destination
				}

				// Collect sources and destination from remaining parts
				// parts[idx:] are the source(s) and destination
				remainingArgs := parts[idx:]
				if len(remainingArgs) >= 1 {
					dest = remainingArgs[len(remainingArgs)-1]
					if len(remainingArgs) > 1 {
						srcs = remainingArgs[:len(remainingArgs)-1]
					} else {
						// Handle cases like `COPY .` or `COPY artifact` where src might be implied or same as dest
						// If --from is used, the single remaining arg is likely the source from that stage
						// If not --from, and it's a single arg, it's often `COPY .` (dest is current WORKDIR, src is .)
						// For simplicity, if only one arg remains, treat it as both src and dest if not --from,
						// or just src if --from is set (dest would have to be explicitly WORKDIR or similar)
						// This part is tricky; Docker's COPY is very flexible.
						// Let's assume if only one arg after flags, it's the source, and dest is WORKDIR.
						// However, our 'dest' var needs to be explicit for path entry.
						// A safer bet: if one arg, it's source, and dest must have been WORKDIR.
						// Apko needs explicit dest. If WORKDIR is /app, COPY foo becomes COPY foo /app/foo
						// For now, if len(remainingArgs) == 1, it must be the source.
						// The user will have to ensure destination is clear.
						// The previous logic: if len(parts) - idx == 1, dest = parts[idx], src = parts[idx] (if no fromVal)
						// This is a complex case. Let's stick to: last is dest, rest are srcs. If only one, it's dest, and src is same.
						if len(remainingArgs) == 1 {
							srcs = append(srcs, remainingArgs[0]) // treat single remaining as a source
							// If dest is directory (ends with /), Docker copies *into* it.
							// If dest is a file, it copies *as* it.
							// Our model assumes `dest` is the final path.
						}
					}
				}

				if dest != "" { // Only add if we have a destination
					if len(srcs) == 0 { // If no explicit sources, but dest is present (e.g. WORKDIR /foo; COPY bar) - this is not how Docker COPY works.
						// This implies an issue if srcs is empty but dest is not.
						// However, if it was `COPY .` (and `.` was the single remaining arg), `srcs` would have `.`
						// If srcs is empty, it means parsing logic for srcs/dest needs review for single arg copies.
						// For `COPY artifact_from_stage /app/`, remainingArgs is [`artifact_from_stage`, `/app/`]. srcs=[`artifact_from_stage`], dest=`/app/`
						// For `COPY . .`, remainingArgs is [`.`, `.`]. srcs=[`.`], dest=`.`
						// For `COPY . /app`, remainingArgs is [`.`, `/app`]. srcs=[`.`], dest=`/app`
						// For `COPY singlefile`, if dest is WORKDIR, it becomes `COPY singlefile WORKDIR_PATH/singlefile`.
						// Our parser requires explicit dest.
						// If after flags, only one item `X` remains:
						//   - if X is clearly a dir (ends with /) or WORKDIR is set, it could be `dest=X, src="."` (implied CWD).
						//   - if X is a file, it could be `src=X, dest=WORKDIR/X`.
						// The current loop for flags and then `remainingArgs` should correctly identify
						// the last element of `remainingArgs` as `dest` and the rest as `srcs`.
						// So, `if len(srcs) == 0 && dest != ""` implies remainingArgs had only one element (dest).
						// In this case, that one element is also the source.
						if len(remainingArgs) == 1 { // This means dest was the only non-flag argument
							srcs = append(srcs, dest)
						}
					}

					for _, src := range srcs { // Create a path entry for each source
						pathEntry := Path{
							Path:   dest,       // Destination path
							Type:   "hardlink", // Default type
							Source: src,        // Source path
						}
						if chownVal != "" {
							// TODO: Parse chown (user:group) and set UID/GID on pathEntry
							// This requires mapping user/group names to UIDs/GIDs defined in the stage.
							// For now, we acknowledge it but don't translate.
							// Could add a comment or an annotation if apko supported it.
							// fmt.Printf("INFO: COPY --chown='%s' encountered for %s -> %s. Manual UID/GID mapping may be needed.\\n", chownVal, src, dest)
						}
						if chmodVal != "" {
							pathEntry.Permissions = chmodVal // Apply chmod directly
						}
						if fromVal != "" {
							// If --from is used, the 'src' is relative to that stage.
							pathEntry.Source = fmt.Sprintf("--from=%s %s", fromVal, src)
						}
						currentConfig.Paths = append(currentConfig.Paths, pathEntry)
					}
				}
			}
		}
	}
	if len(stageConfigs) == 0 && dockerfile != nil && len(dockerfile.Lines) > 0 {
		// If no FROM was found but there are lines, create a default stage
		// This might happen for a Dockerfile snippet or an invalid one.
		// For now, we require a FROM to start processing.
		return nil, fmt.Errorf("no FROM instruction found in Dockerfile, cannot determine stages")
	}

	return stageConfigs, nil
}

// GenerateApkoYAML generates the apko YAML configuration
func GenerateApkoYAML(config *ApkoConfig) (string, error) {
	var sb strings.Builder

	sb.WriteString("contents:\n")
	if len(config.Contents.Repositories) > 0 {
		sb.WriteString("  repositories:\n")
		for _, repo := range config.Contents.Repositories {
			sb.WriteString(fmt.Sprintf("    - %s\n", repo))
		}
	}
	sb.WriteString("  packages:\n")
	for _, pkg := range config.Contents.Packages {
		sb.WriteString(fmt.Sprintf("    - %s\n", pkg))
	}

	if config.WorkDir != "" {
		sb.WriteString(fmt.Sprintf("work-dir: %s\n", config.WorkDir))
	}

	if len(config.Environment) > 0 {
		sb.WriteString("environment:\n")
		for k, v := range config.Environment {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	if config.Entrypoint.Command != "" {
		sb.WriteString("entrypoint:\n")
		sb.WriteString(fmt.Sprintf("  command: %s\n", config.Entrypoint.Command))
		if config.Entrypoint.Type != "" {
			sb.WriteString(fmt.Sprintf("  type: %s\n", config.Entrypoint.Type))
		}
	}

	if config.Accounts.RunAs != "" || len(config.Accounts.Users) > 0 || len(config.Accounts.Groups) > 0 {
		sb.WriteString("accounts:\n")
		if config.Accounts.RunAs != "" {
			sb.WriteString(fmt.Sprintf("  run-as: %s\n", config.Accounts.RunAs))
		}
		if len(config.Accounts.Users) > 0 {
			sb.WriteString("  users:\n")
			for _, u := range config.Accounts.Users {
				sb.WriteString(fmt.Sprintf("    - username: %s\n", u.Username))
				if u.UID != 0 {
					sb.WriteString(fmt.Sprintf("      uid: %d\n", u.UID))
				}
				if u.Shell != "" {
					sb.WriteString(fmt.Sprintf("      shell: %s\n", u.Shell))
				}
			}
		}
		if len(config.Accounts.Groups) > 0 {
			sb.WriteString("  groups:\n")
			for _, g := range config.Accounts.Groups {
				sb.WriteString(fmt.Sprintf("    - groupname: %s\n", g.Groupname))
				if g.GID != 0 {
					sb.WriteString(fmt.Sprintf("      gid: %d\n", g.GID))
				}
			}
		}
	}

	if len(config.Paths) > 0 {
		sb.WriteString("paths:\n")
		for _, p := range config.Paths {
			sb.WriteString(fmt.Sprintf("  - path: %s\n", p.Path))
			sb.WriteString(fmt.Sprintf("    type: %s\n", p.Type))
			if p.UID != 0 {
				sb.WriteString(fmt.Sprintf("    uid: %d\n", p.UID))
			}
			if p.GID != 0 {
				sb.WriteString(fmt.Sprintf("    gid: %d\n", p.GID))
			}
			if p.Permissions != "" {
				sb.WriteString(fmt.Sprintf("    permissions: %s\n", p.Permissions))
			}
			if p.Source != "" {
				sb.WriteString(fmt.Sprintf("    source: %s\n", p.Source))
			}
		}
	}

	return sb.String(), nil
}
