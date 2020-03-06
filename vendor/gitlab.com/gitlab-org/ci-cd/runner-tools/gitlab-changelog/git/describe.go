package git

import (
	"context"
	"fmt"
	"strings"
)

type DescribeOpts struct {
	Tags   bool
	Abbrev int
	Match  string
}

func (o *DescribeOpts) Args() []string {
	args := []string{
		fmt.Sprintf("--abbrev=%d", o.Abbrev),
	}

	if o.Tags {
		args = append(args, "--tags")
	}

	if o.Match != "" {
		args = append(args, []string{"--match", o.Match}...)
	}

	return args
}

func defaultDescribeWithMatchOpts() *DescribeOpts {
	return &DescribeOpts{
		Abbrev: DefaultAbbreviation,
	}
}

func (c *gitCommand) Describe(opts *DescribeOpts) (string, error) {
	if opts == nil {
		opts = defaultDescribeWithMatchOpts()
	}

	ctx, cancelFn := context.WithTimeout(context.Background(), DefaultCommandTimeout)
	defer cancelFn()

	args := append([]string{"describe"}, opts.Args()...)
	cmd := c.newCommander(ctx, Command, args...)

	out, err := cmd.Output()
	if err != nil {
		return "", NewError(cmd.String(), err)
	}

	return strings.TrimSpace(string(out)), nil
}
