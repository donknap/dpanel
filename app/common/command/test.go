package command

import (
	"github.com/gookit/color"
	"github.com/spf13/cobra"
	"github.com/we7coreteam/w7-rangine-go/src/console"
)

type Test struct {
	console.Abstract
}

func (test Test) GetName() string {
	return "home:test"
}

func (test Test) GetDescription() string {
	return "test command"
}

func (self Test) Configure(command *cobra.Command) {
	command.Flags().String("name", "test", "test name params")
}

func (test Test) Handle(cmd *cobra.Command, args []string) {
	color.Infoln("home test")
}
