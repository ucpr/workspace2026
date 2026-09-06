package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	ghlib "github.com/ucpr/atama/internal/github"
	"github.com/ucpr/atama/internal/service"
	"github.com/ucpr/atama/internal/store"
)

func newGitHubCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "github",
		Short: "Import/export/sync tasks with GitHub Issues",
	}
	cmd.AddCommand(
		newGitHubSetRepoCmd(),
		newGitHubImportCmd(),
		newGitHubExportCmd(),
		newGitHubSyncCmd(),
	)
	return cmd
}

func newGitHubSetRepoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-repo <owner/repo>",
		Short: "Set the GitHub repo this store syncs with",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, _, err := ghlib.SplitRepo(args[0]); err != nil {
				return err
			}
			st, err := loadStore()
			if err != nil {
				return err
			}
			cfg, err := st.LoadConfig()
			if err != nil {
				return err
			}
			cfg.GitHub = &store.GitHubConfig{Repo: args[0]}
			if err := st.SaveConfig(cfg); err != nil {
				return err
			}
			if isJSON() {
				return printJSON(map[string]string{"github_repo": args[0]})
			}
			printText("Linked to GitHub repo %s", args[0])
			return nil
		},
	}
}

// githubAPI resolves the configured repo and an authenticated API client
// for the store found from the current directory.
func githubAPI(ctx context.Context) (ghlib.API, *store.Store, string, error) {
	st, err := loadStore()
	if err != nil {
		return nil, nil, "", err
	}
	cfg, err := st.LoadConfig()
	if err != nil {
		return nil, nil, "", err
	}
	if cfg.GitHub == nil || cfg.GitHub.Repo == "" {
		return nil, nil, "", fmt.Errorf("no GitHub repo configured; run `atama init --github-repo owner/repo` or `atama github set-repo owner/repo`")
	}
	token, err := ghlib.ResolveToken(ctx)
	if err != nil {
		return nil, nil, "", err
	}
	client, err := ghlib.NewClient(token, cfg.GitHub.Repo)
	if err != nil {
		return nil, nil, "", err
	}
	return client, st, cfg.GitHub.Repo, nil
}

func newGitHubImportCmd() *cobra.Command {
	var overwrite bool
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import GitHub Issues as tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			api, st, repo, err := githubAPI(ctx)
			if err != nil {
				return err
			}
			result, err := ghlib.Import(ctx, st, api, repo, ghlib.ImportOptions{Overwrite: overwrite})
			if err != nil {
				return err
			}
			if isJSON() {
				return printJSON(result)
			}
			printText("Imported %d, updated %d, skipped %d", len(result.Created), len(result.Updated), len(result.Skipped))
			for _, t := range result.Created {
				printText("  + %s  %s", t.ID, t.Title)
			}
			for _, t := range result.Updated {
				printText("  ~ %s  %s", t.ID, t.Title)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "refresh tasks already linked to an issue from GitHub")
	return cmd
}

func newGitHubExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export <id>",
		Short: "Export a task as a new GitHub Issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			api, st, repo, err := githubAPI(ctx)
			if err != nil {
				return err
			}
			taskID, err := service.New(st).ResolveTaskID(args[0])
			if err != nil {
				return err
			}
			task, err := ghlib.Export(ctx, st, api, repo, taskID)
			if err != nil {
				return err
			}
			if isJSON() {
				return printJSON(task)
			}
			printText("Exported %s as %s", task.ID, task.GitHub.URL)
			return nil
		},
	}
}

func newGitHubSyncCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Bidirectionally sync linked tasks (and their comments) with GitHub",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			api, st, repo, err := githubAPI(ctx)
			if err != nil {
				return err
			}
			result, err := ghlib.Sync(ctx, st, api, repo, ghlib.SyncOptions{DryRun: dryRun})
			if err != nil {
				return err
			}
			if isJSON() {
				return printJSON(result)
			}
			printSyncResult(result)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would change without applying it")
	return cmd
}

func printSyncResult(result *ghlib.SyncResult) {
	if result.DryRun {
		printText("Dry run — no changes applied.")
	}
	if len(result.Tasks) == 0 {
		printText("No linked tasks to sync.")
		return
	}
	for _, tp := range result.Tasks {
		printText("%s  issue #%d  %s", tp.TaskID, tp.IssueNumber, tp.Action)
		for _, cp := range tp.Comments {
			if cp.Action == "none" {
				continue
			}
			printText("    comment %s  %s", cp.CommentID, cp.Action)
		}
	}
}
