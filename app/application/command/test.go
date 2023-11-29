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
	return "application:test"
}

func (test Test) GetDescription() string {
	return "application command"
}

func (test Test) Handle(cmd *cobra.Command, args []string) {
	color.Infoln("application test")
}
