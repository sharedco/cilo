package cmd

import (
	"fmt"

	"github.com/cilo/cilo/pkg/config"
	"github.com/cilo/cilo/pkg/git"
	"github.com/cilo/cilo/pkg/state"
	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff <env>",
	Short: "Show logical diff between host and environment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, envName, err := getProjectAndEnv(cmd, args)
		if err != nil {
			return err
		}

		env, err := state.GetEnvironment(project, envName)
		if err != nil {
			return err
		}

		hostRoot := env.Source
		envRoot := config.GetEnvPath(project, envName)

		repos, err := git.FindRepos(envRoot)
		if err != nil {
			return err
		}

		if len(repos) == 0 {
			fmt.Println("No git repositories found in environment.")
			return nil
		}

		for _, repo := range repos {
			fmt.Printf("--- Repo: %s ---\n", repo.Name)
			diff, err := git.Diff(hostRoot, envRoot, repo)
			if err != nil {
				fmt.Printf("Error diffing %s: %v\n", repo.Name, err)
				continue
			}
			if diff == "" {
				fmt.Println("No changes.")
			} else {
				fmt.Println(diff)
			}
		}

		return nil
	},
}

var mergeCmd = &cobra.Command{
	Use:   "merge <env>",
	Short: "Merge changes from environment back to host",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, envName, err := getProjectAndEnv(cmd, args)
		if err != nil {
			return err
		}

		env, err := state.GetEnvironment(project, envName)
		if err != nil {
			return err
		}

		hostRoot := env.Source
		envRoot := config.GetEnvPath(project, envName)

		repos, err := git.FindRepos(envRoot)
		if err != nil {
			return err
		}

		if len(repos) == 0 {
			fmt.Println("No git repositories found in environment.")
			return nil
		}

		for _, repo := range repos {
			fmt.Printf("Merging %s...\n", repo.Name)
			if err := git.Merge(hostRoot, envRoot, repo); err != nil {
				fmt.Printf("Error merging %s: %v\n", repo.Name, err)
				continue
			}
			fmt.Printf("✓ %s merged successfully\n", repo.Name)
		}

		return nil
	},
}

func init() {
	diffCmd.Flags().String("project", "", "Project name (defaults to configured project)")
	mergeCmd.Flags().String("project", "", "Project name (defaults to configured project)")
}
