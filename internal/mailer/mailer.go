package mailer

import (
	"fmt"

	"claim-watcher/internal/config"

	mail "github.com/wneessen/go-mail"
)

type Mailer struct {
	client *mail.Client
	from   string
	to     string
}

func New(cfg config.Config) (*Mailer, error) {
	opts := []mail.Option{
		mail.WithPort(cfg.SMTPPort),
	}

	switch cfg.SMTPTLSMode {
	case "none":
		opts = append(opts, mail.WithTLSPolicy(mail.NoTLS))
	case "opportunistic":
		opts = append(opts, mail.WithTLSPolicy(mail.TLSOpportunistic))
	case "starttls":
		opts = append(opts, mail.WithTLSPolicy(mail.TLSMandatory))
	default:
		return nil, fmt.Errorf("unsupported SMTP TLS mode %q", cfg.SMTPTLSMode)
	}

	if cfg.SMTPUsername != "" {
		opts = append(opts,
			mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
			mail.WithUsername(cfg.SMTPUsername),
			mail.WithPassword(cfg.SMTPPassword),
		)
	}

	client, err := mail.NewClient(cfg.SMTPHost, opts...)
	if err != nil {
		return nil, fmt.Errorf("create SMTP client: %w", err)
	}

	return &Mailer{
		client: client,
		from:   cfg.SMTPFrom,
		to:     cfg.SMTPTo,
	}, nil
}

func (m *Mailer) Send(subject, plainBody, htmlBody string) error {
	msg := mail.NewMsg()

	if err := msg.From(m.from); err != nil {
		return fmt.Errorf("set From address: %w", err)
	}
	if err := msg.To(m.to); err != nil {
		return fmt.Errorf("set To address: %w", err)
	}

	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextPlain, plainBody)
	msg.AddAlternativeString(mail.TypeTextHTML, htmlBody)

	if err := m.client.DialAndSend(msg); err != nil {
		return fmt.Errorf("send SMTP message: %w", err)
	}

	return nil
}
