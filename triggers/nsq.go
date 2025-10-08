// NSQ consumer trigger
package triggers

import (
	"github.com/madcok-co/unicorn"
	"github.com/nsqio/go-nsq"
)

type NSQTrigger struct {
	consumers map[string]*nsq.Consumer
	config    *nsq.Config
}

func NewNSQTrigger() *NSQTrigger
func (t *NSQTrigger) Start() error
func (t *NSQTrigger) Stop() error
func (t *NSQTrigger) RegisterService(def *unicorn.Definition) error
func (t *NSQTrigger) SubscribeTopic(topic, channel, service string) error
