package smtp

import (
	"context"

	"github.com/go-mailx/mailx"
	gomail "github.com/wneessen/go-mail"
)

type mailerAdapter struct {
	client *gomail.Client
}

func (a *mailerAdapter) NewMail(context.Context) (mailx.MailInstance, error) {
	return &messageAdapter{client: a.client, msg: gomail.NewMsg()}, nil
}

type Config struct {
	Host, Username, Password string
	Port                     int
	TLSPolicy                gomail.TLSPolicy
	ImplicitTLS              bool
}

func New(config Config) (*mailerAdapter, error) {
	options := []gomail.Option{
		gomail.WithTLSPortPolicy(config.TLSPolicy),
	}
	if config.Port != 0 {
		options = append(options, gomail.WithPort(config.Port))
	}
	if config.Username != "" {
		options = append(options,
			gomail.WithSMTPAuth(gomail.SMTPAuthLogin),
			gomail.WithUsername(config.Username),
			gomail.WithPassword(config.Password),
			gomail.WithoutNoop(),
		)
	}
	if config.ImplicitTLS {
		options = append(options, gomail.WithSSL())
	}
	client, err := gomail.NewClient(config.Host, options...)

	if err != nil {
		return nil, err
	} else {
		return &mailerAdapter{client: client}, nil
	}
}

type messageAdapter struct {
	client *gomail.Client
	msg    *gomail.Msg
}

func (m *messageAdapter) Bcc(bccs []string) error {
	for _, bcc := range bccs {
		if err := m.msg.AddBcc(bcc); err != nil {
			return err
		}
	}
	return nil
}

func (m *messageAdapter) From(from string) error {
	return m.msg.From(from)
}

func (m *messageAdapter) HtmlBody(body string) error {
	m.msg.SetBodyString(gomail.TypeTextHTML, body)
	return nil
}

func (m *messageAdapter) ReplyTo(replyTo string) error {
	return m.ReplyTo(replyTo)
}

func (m *messageAdapter) Send(ctx context.Context) error {
	return m.client.DialAndSendWithContext(ctx, m.msg)
}

func (m *messageAdapter) Subject(sub string) error {
	m.msg.Subject(sub)
	return nil
}

func (m *messageAdapter) TextBody(body string) error {
	m.msg.SetBodyString(gomail.TypeTextPlain, body)
	return nil
}

func (m *messageAdapter) To(to []string) error {
	return m.msg.To(to...)
}
