/*
 * Copyright 2026 XMiete Core Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package mailer

import (
	"bytes"
	"fmt"
	"io"
	"text/template"

	"gopkg.in/gomail.v2"

	"github.com/xmiete/server/internal/models"
)

// SMTPMailer sends emails via SMTP with the PDF receipt as an attachment.
type SMTPMailer struct {
	dialer *gomail.Dialer
	from   string
}

func NewSMTPMailer(host string, port int, username, password, from string) *SMTPMailer {
	d := gomail.NewDialer(host, port, username, password)
	return &SMTPMailer{dialer: d, from: from}
}

func (m *SMTPMailer) SendReceipt(d *models.Deposit, pdf []byte) error {
	subject := fmt.Sprintf("Your rental deposit receipt — %s %s", d.Property.Address.Street, d.Property.Address.City)

	body, err := renderBody(d)
	if err != nil {
		return fmt.Errorf("render email body: %w", err)
	}

	msg := gomail.NewMessage()
	msg.SetHeader("From", m.from)
	msg.SetHeader("To", d.Tenant.Email)
	msg.SetHeader("Subject", subject)
	msg.SetBody("text/plain", body)

	filename := fmt.Sprintf("receipt-%s.pdf", d.ID)
	msg.Attach(filename,
		gomail.SetCopyFunc(func(w io.Writer) error {
			_, err := w.Write(pdf)
			return err
		}),
	)

	return m.dialer.DialAndSend(msg)
}

func (m *SMTPMailer) SendReleaseReceipt(d *models.Deposit, pdf []byte) error {
	subject := fmt.Sprintf("Your rental deposit has been released — %s %s", d.Property.Address.Street, d.Property.Address.City)

	body, err := renderReleaseBody(d)
	if err != nil {
		return fmt.Errorf("render release email body: %w", err)
	}

	msg := gomail.NewMessage()
	msg.SetHeader("From", m.from)
	msg.SetHeader("To", d.Tenant.Email)
	msg.SetHeader("Subject", subject)
	msg.SetBody("text/plain", body)

	filename := fmt.Sprintf("release-receipt-%s.pdf", d.ID)
	msg.Attach(filename,
		gomail.SetCopyFunc(func(w io.Writer) error {
			_, err := w.Write(pdf)
			return err
		}),
	)

	return m.dialer.DialAndSend(msg)
}

var bodyTmpl = template.Must(template.New("receipt").Parse(`Dear {{ .Tenant.FirstName }} {{ .Tenant.LastName }},

Please find attached the receipt for your rental deposit at {{ .Property.Address.Street }}, {{ .Property.Address.ZIP }} {{ .Property.Address.City }}.

Deposit amount:  {{ printf "%.2f" .Deposit.Amount }} {{ .Deposit.Currency }}
{{ if .Pledge -}}
Pledge date:     {{ .Pledge.PledgeDate }}
Statutory basis: {{ .Pledge.StatutoryBasis }}
{{ end -}}

This receipt confirms that your deposit has been received and secured by your bank.
{{ if .Tenant.WalletMetadata -}}
Your EUDI Wallet has also received a Qualified Electronic Attestation of Attributes (QEAA) for this deposit.
{{ else -}}
Once EUDI Wallet support is widely available, you will also receive a Qualified Electronic Attestation of Attributes (QEAA) as a tamper-evident digital credential.
{{ end -}}

Best regards,
XMiete
`))

func renderBody(d *models.Deposit) (string, error) {
	var buf bytes.Buffer
	if err := bodyTmpl.Execute(&buf, d); err != nil {
		return "", err
	}
	return buf.String(), nil
}

var releaseBodyTmpl = template.Must(template.New("release").Parse(`Dear {{ .Tenant.FirstName }} {{ .Tenant.LastName }},

Your rental deposit for {{ .Property.Address.Street }}, {{ .Property.Address.ZIP }} {{ .Property.Address.City }} has been released by your landlord.

Deposit amount:  {{ printf "%.2f" .Deposit.Amount }} {{ .Deposit.Currency }}
{{ if .Pledge -}}
Originally pledged: {{ .Pledge.PledgeDate }}
{{ end -}}

Please find the release confirmation attached. The funds will be returned to you according to your bank's processing times.

Best regards,
XMiete
`))

func renderReleaseBody(d *models.Deposit) (string, error) {
	var buf bytes.Buffer
	if err := releaseBodyTmpl.Execute(&buf, d); err != nil {
		return "", err
	}
	return buf.String(), nil
}
