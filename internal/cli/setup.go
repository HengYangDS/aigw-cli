package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSetupCommand(app *App) *cobra.Command {
	var profile, label, openAIURL, anthropicURL string
	var tokenStdin bool
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "以参数方式完成首次配置",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			if len(cfg.Profiles) > 0 {
				return fmt.Errorf("AIGW is already configured; use `aigw add`, `aigw use`, or `aigw status`")
			}
			if profile == "" {
				return fmt.Errorf("setup needs --profile; example: `aigw setup --profile team --openai-url https://gateway.example/v1`")
			}
			args := []string{"add", profile, "--label", label, "--openai-url", openAIURL, "--anthropic-url", anthropicURL}
			if tokenStdin {
				args = append(args, "--token-stdin")
			}
			child := NewRoot(app)
			child.SetArgs(args)
			return child.Execute()
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "profile identifier")
	cmd.Flags().StringVar(&label, "label", "", "human-readable provider label")
	cmd.Flags().StringVar(&openAIURL, "openai-url", "", "OpenAI Responses base URL")
	cmd.Flags().StringVar(&anthropicURL, "anthropic-url", "", "Anthropic base URL")
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "read one token line from stdin")
	return cmd
}
