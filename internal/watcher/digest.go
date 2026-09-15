package watcher

import (
	"fmt"
	"html"
	"net/url"
	"strings"
	"time"

	"claim-watcher/internal/model"
)

type Digest struct {
	Subject   string
	PlainBody string
	HTMLBody  string
	Overdue   int
	Upcoming  int
}

func BuildDigest(claims []model.Claim, baseURL string, warningDays int, now time.Time) Digest {
	overdue := make([]model.Claim, 0)
	current := make([]model.Claim, 0)

	for _, claim := range claims {
		if claim.IsOverdue() {
			overdue = append(overdue, claim)
		} else {
			current = append(current, claim)
		}
	}

	subject := fmt.Sprintf(
		"Claims Alert: %d Overdue / %d Due Within %d Days – %s",
		len(overdue),
		len(current),
		warningDays,
		now.Format("02.01.2006"),
	)

	var plain strings.Builder
	fmt.Fprintf(&plain, "CLAIMS DEADLINE ALERT – %s\n\n", now.Format("02.01.2006"))

	if len(overdue) > 0 {
		plain.WriteString("OVERDUE CLAIMS\n")
		plain.WriteString("==============\n")
		for _, c := range overdue {
			fmt.Fprintf(&plain, "%s | %s | %d day(s) overdue | %s | %s | %s\n",
				c.ClaimNumber,
				c.Deadline.Format("02.01.2006"),
				valueInt(c.DaysOverdue),
				c.Priority,
				handlerName(c),
				claimLink(baseURL, c.ID),
			)
		}
		plain.WriteString("\n")
	}

	if len(current) > 0 {
		fmt.Fprintf(&plain, "UPCOMING DEADLINES – NEXT %d DAYS\n", warningDays)
		plain.WriteString("=================================\n")
		for _, c := range current {
			label := fmt.Sprintf("%d day(s) remaining", valueInt(c.DaysRemaining))
			if c.IsDueToday() {
				label = "DUE TODAY"
			}
			fmt.Fprintf(&plain, "%s | %s | %s | %s | %s | %s\n",
				c.ClaimNumber,
				c.Deadline.Format("02.01.2006"),
				label,
				c.Priority,
				handlerName(c),
				claimLink(baseURL, c.ID),
			)
		}
	}

	var body strings.Builder
	body.WriteString(`<!doctype html><html><body style="font-family:Arial,Helvetica,sans-serif;color:#222;">`)
	fmt.Fprintf(&body, `<h2 style="margin-bottom:4px;">Claims Deadline Alert</h2><p style="margin-top:0;color:#666;">%s</p>`,
		html.EscapeString(now.Format("02.01.2006")))

	if len(overdue) > 0 {
		body.WriteString(`<h3 style="color:#b00020;">Overdue Claims</h3>`)
		body.WriteString(buildHTMLTable(overdue, baseURL, true))
	}

	if len(current) > 0 {
		fmt.Fprintf(&body, `<h3>Upcoming Deadlines – Next %d Days</h3>`, warningDays)
		body.WriteString(buildHTMLTable(current, baseURL, false))
	}

	body.WriteString(`<p style="margin-top:24px;color:#666;font-size:12px;">This is an automated message from Claim Watcher. Claims with status ABGESCHLOSSEN or ABGELEHNT are excluded.</p>`)
	body.WriteString(`</body></html>`)

	return Digest{
		Subject:   subject,
		PlainBody: plain.String(),
		HTMLBody:  body.String(),
		Overdue:   len(overdue),
		Upcoming:  len(current),
	}
}

func buildHTMLTable(claims []model.Claim, baseURL string, overdue bool) string {
	var b strings.Builder
	b.WriteString(`<table cellpadding="7" cellspacing="0" border="1" style="border-collapse:collapse;border-color:#ddd;width:100%;max-width:1050px;">`)
	b.WriteString(`<thead><tr style="background:#f4f4f4;"><th align="left">Claim</th><th align="left">Deadline</th><th align="left">Timing</th><th align="left">Priority</th><th align="left">Type</th><th align="left">Category</th><th align="left">Responsible</th></tr></thead><tbody>`)

	for _, c := range claims {
		timing := ""
		if overdue {
			timing = fmt.Sprintf("%d day(s) overdue", valueInt(c.DaysOverdue))
		} else if c.IsDueToday() {
			timing = "DUE TODAY"
		} else {
			timing = fmt.Sprintf("%d day(s) remaining", valueInt(c.DaysRemaining))
		}

		claimDisplay := html.EscapeString(c.ClaimNumber)
		if link := claimLink(baseURL, c.ID); link != "" {
			claimDisplay = fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(link), claimDisplay)
		}

		fmt.Fprintf(&b,
			`<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
			claimDisplay,
			html.EscapeString(c.Deadline.Format("02.01.2006")),
			html.EscapeString(timing),
			html.EscapeString(c.Priority),
			html.EscapeString(c.ClaimType),
			html.EscapeString(c.Category),
			html.EscapeString(handlerName(c)),
		)
	}

	b.WriteString(`</tbody></table>`)
	return b.String()
}

func handlerName(c model.Claim) string {
	if c.ResponsibleHandlerName != nil && strings.TrimSpace(*c.ResponsibleHandlerName) != "" {
		return *c.ResponsibleHandlerName
	}
	return "Unassigned"
}

func valueInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func claimLink(baseURL, claimID string) string {
	if strings.TrimSpace(baseURL) == "" {
		return ""
	}
	return strings.TrimRight(baseURL, "/") + "/claims/" + url.PathEscape(claimID)
}
