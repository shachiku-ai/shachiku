package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"shachiku/internal/config"
)

// Skill represents a capability that the agent can execute
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsBuiltin   bool   `json:"is_builtin"`
}

// ListSkills returns a list of configured base skills and dynamically installed skills
func ListSkills() []Skill {
	tmpDir := filepath.Join(config.GetDataDir(), "tmp")
	baseSkills := []Skill{
		{Name: "read_file", Description: "Read a local file from the system. Argument should be the path to the file.", IsBuiltin: true},
		{Name: "write_file", Description: "Write content to a file. Argument must be a JSON object: {\"path\": \"<file_path>\", \"content\": \"<file_content>\"}. Please use " + tmpDir + " for storing temporary scripts or files.", IsBuiltin: true},
		{Name: "bash", Description: "Execute a shell command on the host computer (bash on Linux/macOS, cmd on Windows). Argument should be the raw command (e.g., 'ls -la' or 'npm install').", IsBuiltin: true},
		{Name: "install_skill", Description: "Install a skill from a local folder. IF the user provides a URL or zip/tar file, you MUST FIRST use the 'bash' skill to download and extract it to the " + tmpDir + " directory. THEN, use this skill. Argument must be JSON: {\"path\": \"<local_folder_path>\", \"force\": false}. If risks are detected, you must ask the user for approval and then use force=true.", IsBuiltin: true},
		{Name: "playwright", Description: "Control a web browser using Playwright. Arguments must be a JSON object: {\"action\": \"<goto/click/type/screenshot/evaluate/close>\", \"url\": \"<url>\", \"selector\": \"<css_selector>\", \"text\": \"<text_to_type/js_code>\", \"file_path\": \"<screenshot_path>\"}", IsBuiltin: true},
	}

	dynamicSkills := GetDynamicSkills()
	return append(baseSkills, dynamicSkills...)
}

// GetDynamicSkills reads the installed skills from the local skills directory.
func GetDynamicSkills() []Skill {
	var dyn []Skill
	skillsDir := filepath.Join(config.GetDataDir(), "skills")

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return dyn
	}

	for _, entry := range entries {
		if entry.IsDir() {
			skillPath := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
			content, err := os.ReadFile(skillPath)
			if err == nil {
				name, desc := parseSkillMD(string(content))
				if name == "" {
					name = entry.Name()
				}
				if desc == "" {
					desc = "Please read " + skillsDir + "/" + entry.Name() + "/SKILL.md for details."
				}
				dyn = append(dyn, Skill{Name: name, Description: desc, IsBuiltin: false})
			}
		}
	}

	return dyn
}

func parseSkillMD(content string) (string, string) {
	var name, desc string
	lines := strings.Split(content, "\n")
	inFrontmatter := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "---" {
			if inFrontmatter {
				break
			} else {
				inFrontmatter = true
				continue
			}
		}
		if inFrontmatter {
			if strings.HasPrefix(line, "name:") {
				name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
				name = strings.Trim(name, `"'`)
			} else if strings.HasPrefix(line, "description:") {
				desc = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
				desc = strings.Trim(desc, `"'`)
			}
		}
	}
	return name, desc
}

// CreateSkill creates a new skill directory conforming to the Anthropic skill format
func CreateSkill(name, description, instructions string) error {
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("invalid skill name")
	}

	skillDir := filepath.Join(config.GetDataDir(), "skills", name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("failed to create skill directory: %v", err)
	}

	if instructions == "" {
		instructions = "Instructions go here."
	}

	skillMD := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n# %s\n\n%s", name, description, name, instructions)

	err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644)
	if err != nil {
		return err
	}

	// Create common subdirectories to be compliant with standard Anthropic skill spec
	os.MkdirAll(filepath.Join(skillDir, "scripts"), 0755)
	os.MkdirAll(filepath.Join(skillDir, "assets"), 0755)

	return nil
}

// ExecuteSkill provides the implementation for executing skills
func ExecuteSkill(name string, args string) string {
	switch name {
	case "read_file":
		return performReadFile(args)
	case "write_file":
		return performWriteFile(args)
	case "bash":
		return performBashCommand(args)
	case "install_skill":
		return performInstallSkill(args)
	case "playwright":
		return performPlaywrightCommand(args)
	default:
		// Execute local skill script if present
		return performLocalSkill(name, args)
	}
}

func performLocalSkill(name string, args string) string {
	if name == "" || name == "." || name == ".." || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Sprintf("Error: Invalid skill name '%s'.", name)
	}

	skillDir := findSkillDir(name)
	if skillDir == "" {
		return fmt.Sprintf("Error: skill '%s' not found or has no matching folder.", name)
	}

	// Try to execute a script
	entryPoints := []string{
		filepath.Join(skillDir, "scripts", "run.sh"),
		filepath.Join(skillDir, "run.sh"),
	}

	var executablePath string
	for _, ep := range entryPoints {
		if stat, err := os.Stat(ep); err == nil && !stat.IsDir() {
			executablePath = ep
			break
		}
	}

	if executablePath == "" {
		return fmt.Sprintf("Skill '%s' exists but has no executable entry point (e.g. %s/scripts/run.sh). Please use the 'read_file' skill to read %s/SKILL.md for instructions, or use 'bash' to run its scripts manually.", name, skillDir, skillDir)
	}

	cmd := exec.Command("bash", executablePath, args)
	out, err := cmd.CombinedOutput()

	output := string(out)
	if len(output) > 2000 {
		output = output[:2000] + "\n... [Output truncated due to length]"
	}

	if err != nil {
		return fmt.Sprintf("Skill '%s' execution failed: %v\nOutput:\n%s", name, err, output)
	}

	if output == "" {
		return fmt.Sprintf("Skill '%s' executed successfully with no output.", name)
	}
	return output
}

func findSkillDir(skillName string) string {
	skillsDir := filepath.Join(config.GetDataDir(), "skills")
	// First check if it matches a folder name directly
	dir := filepath.Join(skillsDir, skillName)
	if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
		return dir
	}

	// Otherwise, parse SKILL.md in all folders to match the logical name
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			skillPath := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
			content, err := os.ReadFile(skillPath)
			if err == nil {
				parsedName, _ := parseSkillMD(string(content))
				if parsedName == skillName {
					return filepath.Join(skillsDir, entry.Name())
				}
			}
		}
	}
	return ""
}

func performReadFile(filePath string) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Sprintf("Error reading file '%s': %v", filePath, err)
	}

	strData := string(data)
	if len(strData) > 2000 {
		strData = strData[:2000] + "\n... [Content truncated due to length]"
	}

	return fmt.Sprintf("Contents of '%s':\n%s", filePath, strData)
}

func performWriteFile(args string) string {
	var payload struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(args), &payload); err != nil {
		return fmt.Sprintf("Error parsing write_file arguments. Ensure it is a valid JSON: %v", err)
	}

	err := os.WriteFile(payload.Path, []byte(payload.Content), 0644)
	if err != nil {
		return fmt.Sprintf("Error writing file '%s': %v", payload.Path, err)
	}

	return fmt.Sprintf("Successfully wrote to file '%s'.", payload.Path)
}

func performBashCommand(command string) string {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", command)
	} else {
		cmd = exec.Command("bash", "-c", command)
	}
	out, err := cmd.CombinedOutput()

	output := string(out)
	if len(output) > 2000 {
		output = output[:2000] + "\n... [Output truncated due to length]"
	}

	if err != nil {
		return fmt.Sprintf("Command exited with error: %v\nOutput:\n%s", err, output)
	}

	if output == "" {
		return "Command executed successfully with no output."
	}
	return fmt.Sprintf("Command Output:\n%s", output)
}

// DeleteSkill deletes a non-builtin skill by its name
func DeleteSkill(name string) error {
	if name == "read_file" || name == "write_file" || name == "bash" || name == "install_skill" {
		return fmt.Errorf("cannot delete built-in skill")
	}

	skillDir := findSkillDir(name)
	if skillDir == "" {
		return fmt.Errorf("skill '%s' not found or has no matching folder", name)
	}

	return os.RemoveAll(skillDir)
}

func performInstallSkill(args string) string {
	var payload struct {
		Path  string `json:"path"`
		Force bool   `json:"force"`
	}

	// Support both raw path string (legacy) and JSON object
	args = strings.TrimSpace(args)
	if strings.HasPrefix(args, "{") {
		if err := json.Unmarshal([]byte(args), &payload); err != nil {
			return fmt.Sprintf("Error parsing install_skill arguments. Ensure it is a valid JSON: %v", err)
		}
	} else {
		payload.Path = args
		payload.Force = false
	}

	sourcePath := strings.TrimSpace(payload.Path)
	if sourcePath == "" {
		return "Error: path is required."
	}

	stat, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Sprintf("Error: source path '%s' does not exist or cannot be accessed: %v", sourcePath, err)
	}
	if !stat.IsDir() {
		return fmt.Sprintf("Error: source path '%s' is not a directory. Did you forget to extract it first using the bash skill?", sourcePath)
	}

	skillMdPath := filepath.Join(sourcePath, "SKILL.md")
	content, err := os.ReadFile(skillMdPath)
	if err != nil {
		// Try to find if SKILL.md is in a subdirectory (common when github repos are downloaded)
		foundSubdir := ""
		filepath.WalkDir(sourcePath, func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && filepath.Base(path) == "SKILL.md" {
				foundSubdir = filepath.Dir(path)
				return filepath.SkipDir
			}
			return nil
		})

		if foundSubdir != "" {
			sourcePath = foundSubdir
			content, _ = os.ReadFile(filepath.Join(sourcePath, "SKILL.md"))
		} else {
			return fmt.Sprintf("Error: source '%s' (nor its subdirectories) contains a SKILL.md file. It does not meet the installation criteria.", sourcePath)
		}
	}

	name, _ := parseSkillMD(string(content))
	if name == "" {
		name = filepath.Base(sourcePath)
	}

	// Scan for risky operations
	if !payload.Force {
		risks := scanForRiskyOperations(sourcePath)
		if len(risks) > 0 {
			riskMsg := strings.Join(risks, "\n- ")
			return fmt.Sprintf("INSTALLATION PAUSED. Risky operations detected in the skill:\n- %s\n\nPlease disclose these risks to the user and ask for their decision. If they approve, execute install_skill again with {\"path\": \"%s\", \"force\": true}.", riskMsg, sourcePath)
		}
	}

	destPath := filepath.Join(config.GetDataDir(), "skills", name)

	// Remove if already exists
	os.RemoveAll(destPath)

	err = copyDir(sourcePath, destPath)
	if err != nil {
		return fmt.Sprintf("Error installing skill '%s': %v", name, err)
	}

	return fmt.Sprintf("Successfully installed skill '%s' to '%s'.", name, destPath)
}

func scanForRiskyOperations(sourcePath string) []string {
	riskyKeywords := []string{
		"sudo ", "rm -rf", "curl ", "wget ", "apt ", "apt-get ",
		"yum ", "apk ", "pip install", "npm install", "docker ",
		"chmod ", "chown ", "mkfs", "dd ", "> /dev/",
	}

	var risks []string
	riskMap := make(map[string]bool)

	filepath.WalkDir(sourcePath, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".sh" || ext == ".py" || ext == ".js" || ext == ".go" {
				data, err := os.ReadFile(path)
				if err == nil {
					contentStr := string(data)
					for _, kw := range riskyKeywords {
						if strings.Contains(contentStr, kw) {
							risk := fmt.Sprintf("Found risky keyword '%s' in %s", strings.TrimSpace(kw), filepath.Base(path))
							if !riskMap[risk] {
								riskMap[risk] = true
								risks = append(risks, risk)
							}
						}
					}
				}
			}
		}
		return nil
	})
	return risks
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		var mode os.FileMode = 0644
		if err == nil {
			mode = info.Mode()
		}
		return os.WriteFile(target, data, mode)
	})
}
