package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/milenkom81/m/internal/config"
)

func newValidateCmd() *cobra.Command {
	var quiet bool
	cmd := &cobra.Command{
		Use:   "validate <path>...",
		Short: "Lint MD agent/skill/tool/mcp_server files",
		Long: "Validate parses each path (file or directory) and reports schema and " +
			"cross-reference issues. Exits non-zero if any issues are found.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			docs, parseErrs := loadAll(args)
			issues := config.ValidateDocs(docs)

			sort.Slice(issues, func(i, j int) bool {
				if issues[i].Path != issues[j].Path {
					return issues[i].Path < issues[j].Path
				}
				return issues[i].Field < issues[j].Field
			})

			for _, iss := range issues {
				fmt.Fprintln(cmd.ErrOrStderr(), iss.Error())
			}

			if !quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "checked %d document(s); %d issue(s)\n", len(docs), len(issues))
			}

			if parseErrs != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), parseErrs)
				return errors.New("parse failed")
			}
			if len(issues) > 0 {
				return fmt.Errorf("%d validation issue(s)", len(issues))
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "suppress summary line")
	return cmd
}

// loadAll expands each arg (file or directory) and parses every .md file.
func loadAll(args []string) ([]*config.Document, error) {
	var docs []*config.Document
	var errs []error

	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if info.IsDir() {
			d, err := config.LoadDir(arg)
			docs = append(docs, d...)
			if err != nil {
				errs = append(errs, err)
			}
			continue
		}
		if filepath.Ext(arg) != ".md" {
			errs = append(errs, fmt.Errorf("%s: not a .md file", arg))
			continue
		}
		doc, err := config.ParseFile(arg)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		docs = append(docs, doc)
	}
	return docs, errors.Join(errs...)
}
