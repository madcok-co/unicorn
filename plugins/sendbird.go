// SendGrid email plugin
package plugins

import (
	"github.com/sendgrid/sendgrid-go"
)

type SendGridPlugin struct {
	apiKey    string
	fromEmail string
	fromName  string
	client    *sendgrid.Client
}

type EmailRequest struct {
	To          []string
	Subject     string
	HTMLContent string
	TextContent string
	CC          []string
	ReplyTo     string
}

func NewSendGridPlugin(config map[string]interface{}) (*SendGridPlugin, error)
func (p *SendGridPlugin) SendEmail(req *EmailRequest) error
func (p *SendGridPlugin) SendSimpleEmail(to, subject, body string) error
