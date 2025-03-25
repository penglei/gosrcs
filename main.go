package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/urfave/cli/v3"
)

func RootCommand() *cli.Command {
	cmd := &cli.Command{
		Arguments: []cli.Argument{},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "base-dir",
				Value: "./",
				Usage: "base directory to which every file relative",
			},
			&cli.StringSliceFlag{
				Name:    "tags",
				Aliases: []string{},
				Usage:   "tags to be added to the file",
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			return ctx, nil
		},

		Action: func(ctx context.Context, cmd *cli.Command) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			dir := cmd.Args().Get(0)
			if dir == "" {
				dir = cwd
			} else {
				if !path.IsAbs(dir) {
					dir = path.Join(cwd, dir)
				}
			}

			slog.Info("list go package sources.", "dir", dir)

			var buildFlags []string
			buildTags := cmd.StringSlice("tags")

			if len(buildTags) > 0 {
				buildFlags = append(buildFlags, "--tags", strings.Join(buildTags, ","))
			}

			sources, err := listSources(dir, buildFlags)
			if err != nil {
				return err
			}
			var files []string

			for _, s := range sources {
				files = append(files, s.Path)
			}
			sort.Strings(files)
			for _, file := range files {
				//output file relative main module path
				fmt.Println(file)
			}
			return nil
		},
	}
	return cmd
}

func main() {
	cmd := RootCommand()
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error("exited", "error", err)
	}
}
