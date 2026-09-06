package cli

import (
	"github.com/spf13/cobra"

	ghlib "github.com/ucpr/atama/internal/github"
	"github.com/ucpr/atama/internal/store"
)

func newInitCmd() *cobra.Command {
	var githubRepo string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a task store in the current directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := workDir()
			if err != nil {
				return err
			}
			s, err := store.Init(wd)
			if err != nil {
				return err
			}
			if githubRepo != "" {
				if _, _, err := ghlib.SplitRepo(githubRepo); err != nil {
					return err
				}
				cfg, err := s.LoadConfig()
				if err != nil {
					return err
				}
				cfg.GitHub = &store.GitHubConfig{Repo: githubRepo}
				if err := s.SaveConfig(cfg); err != nil {
					return err
				}
			}
			if isJSON() {
				return printJSON(map[string]string{"store": s.Root(), "github_repo": githubRepo})
			}
			printText("Initialized empty atama store at %s", s.Root())
			if githubRepo != "" {
				printText("Linked to GitHub repo %s", githubRepo)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&githubRepo, "github-repo", "", "GitHub repo (owner/repo) to link for import/export/sync")
	return cmd
}
