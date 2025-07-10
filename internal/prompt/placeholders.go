package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"scripto/internal/args"
)

// PrompterInterface allows for testing by mocking user input
type PrompterInterface interface {
	PromptForValue(name, description string) (string, error)
	PromptYesNo(message string) (bool, error)
}

// ConsolePrompter implements PrompterInterface using console input
type ConsolePrompter struct {
	reader *bufio.Reader
}

// NewConsolePrompter creates a new console-based prompter
func NewConsolePrompter() *ConsolePrompter {
	return &ConsolePrompter{
		reader: bufio.NewReader(os.Stdin),
	}
}

// PromptForValue prompts the user to enter a value for a placeholder
func (p *ConsolePrompter) PromptForValue(name, description string) (string, error) {
	// Create a user-friendly prompt
	prompt := fmt.Sprintf("Enter value for %s", name)
	if description != "" {
		prompt += fmt.Sprintf(" (%s)", description)
	}
	prompt += ": "

	fmt.Print(prompt)

	// Read user input
	input, err := p.reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}

	// Trim whitespace and return
	return strings.TrimSpace(input), nil
}

// PromptYesNo prompts the user for a yes/no response
func (p *ConsolePrompter) PromptYesNo(message string) (bool, error) {
	fmt.Printf("%s (y/n): ", message)

	input, err := p.reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read input: %w", err)
	}

	response := strings.ToLower(strings.TrimSpace(input))
	switch response {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		fmt.Println("Please enter 'y' or 'n'")
		return p.PromptYesNo(message)
	}
}

// PlaceholderPrompter handles prompting for missing placeholders
type PlaceholderPrompter struct {
	prompter PrompterInterface
}

// NewPlaceholderPrompter creates a new placeholder prompter
func NewPlaceholderPrompter(prompter PrompterInterface) *PlaceholderPrompter {
	return &PlaceholderPrompter{
		prompter: prompter,
	}
}

// PromptForMissingPlaceholders prompts the user for values of missing placeholders
func (p *PlaceholderPrompter) PromptForMissingPlaceholders(missingArgs []args.PlaceholderValue) (map[string]string, error) {
	values := make(map[string]string)

	if len(missingArgs) == 0 {
		return values, nil
	}

	fmt.Printf("Missing %d argument(s):\n", len(missingArgs))

	for _, placeholder := range missingArgs {
		value, err := p.prompter.PromptForValue(placeholder.Name, placeholder.Description)
		if err != nil {
			return nil, fmt.Errorf("failed to prompt for %s: %w", placeholder.Name, err)
		}

		values[placeholder.Name] = value
	}

	return values, nil
}

// PromptToSaveCommand prompts the user to save a new command
func (p *PlaceholderPrompter) PromptToSaveCommand(command string) (bool, string, string, error) {
	// Ask if they want to save the command
	save, err := p.prompter.PromptYesNo(fmt.Sprintf("Command '%s' not found. Save as script?", command))
	if err != nil {
		return false, "", "", err
	}

	if !save {
		return false, "", "", nil
	}

	// Prompt for optional name
	fmt.Print("Enter script name (optional, press Enter to skip): ")
	reader := bufio.NewReader(os.Stdin)
	name, err := reader.ReadString('\n')
	if err != nil {
		return false, "", "", fmt.Errorf("failed to read name: %w", err)
	}
	name = strings.TrimSpace(name)

	// Prompt for optional description
	fmt.Print("Enter description (optional, press Enter to skip): ")
	description, err := reader.ReadString('\n')
	if err != nil {
		return false, "", "", fmt.Errorf("failed to read description: %w", err)
	}
	description = strings.TrimSpace(description)

	return true, name, description, nil
}

// ConfirmExecution asks the user to confirm command execution
func (p *PlaceholderPrompter) ConfirmExecution(command string) (bool, error) {
	fmt.Printf("Execute command: %s\n", command)
	return p.prompter.PromptYesNo("Continue?")
}

// MockPrompter is a test implementation of PrompterInterface
type MockPrompter struct {
	responses      map[string]string
	yesNoResponses map[string]bool
}

// NewMockPrompter creates a new mock prompter for testing
func NewMockPrompter() *MockPrompter {
	return &MockPrompter{
		responses:      make(map[string]string),
		yesNoResponses: make(map[string]bool),
	}
}

// SetResponse sets a mock response for a given prompt
func (m *MockPrompter) SetResponse(prompt, response string) {
	m.responses[prompt] = response
}

// SetYesNoResponse sets a mock yes/no response for a given message
func (m *MockPrompter) SetYesNoResponse(message string, response bool) {
	m.yesNoResponses[message] = response
}

// PromptForValue returns the mock response for testing
func (m *MockPrompter) PromptForValue(name, description string) (string, error) {
	key := name
	if description != "" {
		key = fmt.Sprintf("%s (%s)", name, description)
	}

	if response, exists := m.responses[key]; exists {
		return response, nil
	}

	return "", fmt.Errorf("no mock response set for prompt: %s", key)
}

// PromptYesNo returns the mock yes/no response for testing
func (m *MockPrompter) PromptYesNo(message string) (bool, error) {
	if response, exists := m.yesNoResponses[message]; exists {
		return response, nil
	}

	return false, fmt.Errorf("no mock yes/no response set for message: %s", message)
}
