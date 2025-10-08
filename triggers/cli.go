// CLI command trigger
package triggers

import (
	"github.com/madcok-co/unicorn"
	"github.com/spf13/cobra"
)

type CLITrigger struct {
	rootCmd *cobra.Command
}

func NewCLITrigger() *CLITrigger
func (t *CLITrigger) Start() error
func (t *CLITrigger) RegisterService(def *unicorn.Definition) error
func (t *CLITrigger) Execute(serviceName string, request interface{}) (interface{}, error)
