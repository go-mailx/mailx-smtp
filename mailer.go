package smtp

import (
	"context"

	"github.com/go-mailx/mailx"
	gomail "github.com/wneessen/go-mail"
)

type mailer struct {
	client *gomail.Client
}

func (a *mailer) NewMail(context.Context) (mailx.MailInstance, error) {
	return &mailInstance{client: a.client, msg: gomail.NewMsg()}, nil
}

func New(config Config) (*mailer, error) {
	options := []gomail.Option{
		gomail.WithTLSPortPolicy(gomail.TLSPolicy(config.TLSPolicy)),
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
		return &mailer{client: client}, nil
	}
}

type mailInstance struct {
	client   *gomail.Client
	msg      *gomail.Msg
	htmlBody *string
	textBody *string
}

// setBody writes the text body before the HTML body, because the last part
// of a multipart/alternative is the one clients prefer (RFC 2046).
func (m *mailInstance) setBody() {
	switch {
	case m.textBody != nil && m.htmlBody != nil:
		m.msg.SetBodyString(gomail.TypeTextPlain, *m.textBody)
		m.msg.AddAlternativeString(gomail.TypeTextHTML, *m.htmlBody)
	case m.htmlBody != nil:
		m.msg.SetBodyString(gomail.TypeTextHTML, *m.htmlBody)
	case m.textBody != nil:
		m.msg.SetBodyString(gomail.TypeTextPlain, *m.textBody)
	}
}

func (m *mailInstance) Bcc(bccs []string) error {
	for _, bcc := range bccs {
		if err := m.msg.AddBcc(bcc); err != nil {
			return err
		}
	}
	return nil
}

func (m *mailInstance) From(from string) error {
	return m.msg.From(from)
}

func (m *mailInstance) HtmlBody(body string) error {
	m.htmlBody = &body
	m.setBody()
	return nil
}

func (m *mailInstance) ReplyTo(replyTo string) error {
	return m.msg.ReplyTo(replyTo)
}

func (m *mailInstance) Send(ctx context.Context) error {
	return m.client.DialAndSendWithContext(ctx, m.msg)
}

func (m *mailInstance) Subject(sub string) error {
	m.msg.Subject(sub)
	return nil
}

func (m *mailInstance) TextBody(body string) error {
	m.textBody = &body
	m.setBody()
	return nil
}

func (m *mailInstance) To(to []string) error {
	return m.msg.To(to...)
}
