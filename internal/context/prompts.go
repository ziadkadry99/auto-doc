package context

import (
	"fmt"

	"github.com/manifoldco/promptui"
)

// CollectInteractive runs an interactive prompt session to gather business
// context from the user. All questions are optional; pressing Enter skips them.
func CollectInteractive() (*BusinessContext, error) {
	fmt.Println("Provide optional business context to improve documentation quality.")
	fmt.Println("Press Enter to skip any question.")
	fmt.Println()

	ctx := &BusinessContext{}

	description, err := askOptional("What does this project do?")
	if err != nil {
		return nil, fmt.Errorf("description prompt: %w", err)
	}
	ctx.Description = description

	targetUsers, err := askOptional("Who are the target users?")
	if err != nil {
		return nil, fmt.Errorf("target users prompt: %w", err)
	}
	ctx.TargetUsers = targetUsers

	keyConcepts, err := askOptional("What are the key business domains or concepts?")
	if err != nil {
		return nil, fmt.Errorf("key concepts prompt: %w", err)
	}
	ctx.KeyConcepts = keyConcepts

	archDecisions, err := askOptional("Any important architectural decisions to document?")
	if err != nil {
		return nil, fmt.Errorf("arch decisions prompt: %w", err)
	}
	ctx.ArchDecisions = archDecisions

	additionalInfo, err := askOptional("Any additional context?")
	if err != nil {
		return nil, fmt.Errorf("additional info prompt: %w", err)
	}
	ctx.AdditionalInfo = additionalInfo

	return ctx, nil
}

// askOptional displays a prompt and returns the user's input. An empty string
// is returned if the user presses Enter without typing anything.
func askOptional(label string) (string, error) {
	p := promptui.Prompt{
		Label:     label,
		Default:   "",
		AllowEdit: true,
	}
	result, err := p.Run()
	if err != nil {
		return "", err
	}
	return result, nil
}
