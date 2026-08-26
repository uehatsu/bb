package pr

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/cmdutil"
)

// NewCmdEdit returns `pr edit`.
func NewCmdEdit(f *cmdutil.Factory) *cobra.Command {
	var title, body, bodyFile, base string
	var addReviewers, removeReviewers []string
	cmd := &cobra.Command{
		Use:   "edit [<number> | <branch> | <url>]",
		Short: "Edit a pull request",
		Example: `  $ bb pr edit 42 --title "New title" --body "New body"
  $ bb pr edit --add-reviewer alice --remove-reviewer bob
  $ bb pr edit --base develop`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if bodyFile != "" {
				b, err := readBodyFile(bodyFile, f.IOStreams.In)
				if err != nil {
					return err
				}
				body = b
				cmd.Flags().Set("body", body) //nolint:errcheck
			}
			if !cmd.Flags().Changed("title") && !cmd.Flags().Changed("body") && base == "" && len(addReviewers) == 0 && len(removeReviewers) == 0 {
				return cmdutil.FlagErrorf("specify at least one field to edit")
			}
			sel := cmdutil.OptionalArg(args)
			ctx := cmd.Context()
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			pr, repo, err := resolvePR(ctx, f, client, sel)
			if err != nil {
				return err
			}
			// Bitbucket's PUT requires the title even for partial updates.
			payload := map[string]any{"title": pr.Title}
			if cmd.Flags().Changed("title") {
				payload["title"] = title
			}
			if cmd.Flags().Changed("body") {
				payload["description"] = body
			}
			if base != "" {
				payload["destination"] = map[string]any{"branch": map[string]string{"name": base}}
			}
			if len(addReviewers) > 0 || len(removeReviewers) > 0 {
				current := map[string]bool{}
				for _, r := range pr.Reviewers {
					current[r.UUID] = true
				}
				if len(addReviewers) > 0 {
					uuids, err := resolveReviewers(ctx, client, repo, addReviewers, false)
					if err != nil {
						return err
					}
					for _, u := range uuids {
						current[u] = true
					}
				}
				if len(removeReviewers) > 0 {
					// Match against the PR's own reviewer list first so people who
					// have since left the workspace can still be removed.
					var unresolved []string
					for _, name := range removeReviewers {
						n := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(name), "@"))
						matched := false
						for _, r := range pr.Reviewers {
							if n == strings.ToLower(r.Nickname) || n == strings.ToLower(r.DisplayName) || n == strings.ToLower(r.UUID) {
								delete(current, r.UUID)
								matched = true
							}
						}
						if !matched {
							unresolved = append(unresolved, name)
						}
					}
					if len(unresolved) > 0 {
						uuids, err := resolveReviewers(ctx, client, repo, unresolved, false)
						if err != nil {
							return err
						}
						for _, u := range uuids {
							delete(current, u)
						}
					}
				}
				var rs []map[string]string
				for u := range current {
					rs = append(rs, map[string]string{"uuid": u})
				}
				if rs == nil {
					rs = []map[string]string{}
				}
				payload["reviewers"] = rs
			}
			var updated bitbucket.PullRequest
			if _, err := client.Do(ctx, api.Request{Method: "PUT", Path: prPath(repo, pr.ID, ""), Body: payload}, &updated); err != nil {
				return err
			}
			fmt.Fprintln(f.IOStreams.Out, updated.Links.HTML())
			return nil
		},
	}
	cmd.Flags().StringVarP(&title, "title", "t", "", "Set the new title")
	cmd.Flags().StringVarP(&body, "body", "b", "", "Set the new body")
	cmd.Flags().StringVarP(&bodyFile, "body-file", "F", "", "Read body text from file")
	cmd.Flags().StringVarP(&base, "base", "B", "", "Change the destination branch")
	cmd.Flags().StringSliceVar(&addReviewers, "add-reviewer", nil, "Add reviewers by nickname")
	cmd.Flags().StringSliceVar(&removeReviewers, "remove-reviewer", nil, "Remove reviewers by nickname")
	return cmd
}
