// AWS SQS consumer trigger
package triggers

import (
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/madcok-co/unicorn"
)

type SQSTrigger struct {
	client    *sqs.SQS
	queueURLs map[string]string
	stopChan  map[string]chan bool
}

func NewSQSTrigger(region string) *SQSTrigger
func (t *SQSTrigger) Start() error
func (t *SQSTrigger) Stop() error
func (t *SQSTrigger) RegisterService(def *unicorn.Definition) error
func (t *SQSTrigger) SubscribeQueue(queueURL, service string) error
