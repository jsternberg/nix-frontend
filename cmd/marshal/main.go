package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli/v2"
)

func marshal(out io.Writer, infile string) error {
	f, err := os.Open(infile)
	if err != nil {
		return err
	}
	defer f.Close()
	io.Copy(out, f)
	return nil
}

func main() {
	app := cli.NewApp()
	app.Usage = "marshal"
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
		return marshal(out, infile)
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "fatal error: %+v\n", err)
		os.Exit(1)
	}
}
