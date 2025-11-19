package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli/v2"
)

func readinputs(out io.Writer, infile string) error {
	b, err := os.ReadFile(infile)
	if err != nil {
		return err
	}

	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	transformed := make(map[string]json.RawMessage, len(m))
	for k, path := range m {
		in, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		transformed[k] = json.RawMessage(in)
	}

	src, err := json.MarshalIndent(transformed, "", "\t")
	if err != nil {
		return err
	}

	if _, err := out.Write(src); err != nil {
		return err
	}
	return nil
}

func main() {
	app := cli.NewApp()
	app.Usage = "readinputs"
	app.Action = func(c *cli.Context) error {
		if c.NArg() < 1 {
			return errors.New("expected at least one argument")
		} else if c.NArg() > 2 {
			return errors.New("too many arguments")
		}

		args := c.Args()
		infile := os.ExpandEnv(args.Get(0))

		out := os.Stdout
		if c.NArg() == 2 {
			outfile := os.ExpandEnv(args.Get(1))
			f, err := os.Create(outfile)
			if err != nil {
				return err
			}
			defer f.Close()

			out = f
		}
		return readinputs(out, infile)
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "fatal error: %+v\n", err)
		os.Exit(1)
	}
}
