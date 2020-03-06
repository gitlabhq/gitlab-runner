package git

import (
	"context"
)

type LogOpts struct {
	FirstParent bool
}

func (o *LogOpts) Args() []string {
	args := make([]string, 0)

	if o.FirstParent {
		args = append(args, "--first-parent")
	}

	return args
}

func (c *gitCommand) Log(query string, opts *LogOpts) ([]byte, error) {
	if opts == nil {
		opts = new(LogOpts)
	}

	ctx, cancelFn := context.WithTimeout(context.Background(), DefaultCommandTimeout)
	defer cancelFn()

	args := []string{"log"}
	if query != "" {
		args = append(args, query)
	}
	args = append(args, opts.Args()...)

	cmd := c.newCommander(ctx, Command, args...)

	out, err := cmd.Output()
	if err != nil {
		return nil, NewError(cmd.String(), err)
	}

	return out, nil
}
