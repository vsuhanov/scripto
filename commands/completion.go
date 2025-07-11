package commands

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

//go:embed scripts/completion.zsh
var customZshCompletion string

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script",
	Long: `To load completions:

Bash:

   fo$ source <(scripto completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ scripto completion bash > /etc/bash_completion.d/scripto
  # macOS:
  $ scripto completion bash > /usr/local/etc/bash_completion.d/scripto

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  # For oh-my-zsh users:
  $ scripto completion zsh > ~/.oh-my-zsh/completions/_scripto
  # For standard installations:
  $ scripto completion zsh > /usr/local/share/zsh/site-functions/_scripto

  # You will need to start a new shell for this setup to take effect.

fish:

  $ scripto completion fish | source

  # To load completions for each session, execute once:
  $ scripto completion fish > ~/.config/fish/completions/scripto.fish

PowerShell:

  PS> scripto completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> scripto completion powershell > scripto.ps1
  # and source this file from your PowerShell profile.

`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			fmt.Print(customZshCompletion)
		// case "zsh":
		// 	cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
