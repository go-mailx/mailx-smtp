package smtp

import gomail "github.com/wneessen/go-mail"

type TLSPolicy gomail.TLSPolicy

const (
	TLSMandatory     TLSPolicy = TLSPolicy(gomail.TLSMandatory)
	TLSOpportunistic TLSPolicy = TLSPolicy(gomail.TLSOpportunistic)
	NoTLS            TLSPolicy = TLSPolicy(gomail.NoTLS)
)

type Config struct {
	Host, Username, Password string
	Port                     int
	TLSPolicy                TLSPolicy
	ImplicitTLS              bool
}
