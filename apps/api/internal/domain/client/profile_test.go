package client

import (
	"testing"
	"time"
)

func TestProfileValidation(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		change func(*Profile)
		valid  bool
	}{
		{"empty profile", func(p *Profile) {}, true},
		{"valid email preference", func(p *Profile) {
			p.Email = "patient@example.invalid"
			p.ReminderChannel = "email"
			p.ReminderConsent = true
		}, true},
		{"requires consent", func(p *Profile) { p.Email = "patient@example.invalid"; p.ReminderChannel = "email" }, false},
		{"requires destination", func(p *Profile) { p.ReminderChannel = "whatsapp"; p.ReminderConsent = true }, false},
		{"valid international phone", func(p *Profile) { p.Phone = "+5491112345678"; p.ReminderChannel = "whatsapp"; p.ReminderConsent = true }, true},
		{"invalid phone", func(p *Profile) { p.Phone = "12345" }, false},
		{"invalid email", func(p *Profile) { p.Email = "Display <a@example.invalid>" }, false},
		{"future birth", func(p *Profile) { p.BirthDate = "2027-01-01" }, false},
		{"valid birth", func(p *Profile) { p.BirthDate = "1992-01-01" }, true},
		{"unsupported lead time", func(p *Profile) { p.ReminderLeadHours = 2 }, false},
		{"valid project", func(p *Profile) {
			p.ChatGPTProjects = []ChatGPTProjectReference{{ProjectID: "g-p-11111111111111111111111111111111", ProjectName: "Synthetic case"}}
		}, true},
		{"invalid project identifier", func(p *Profile) {
			p.ChatGPTProjects = []ChatGPTProjectReference{{ProjectID: "https://example.invalid", ProjectName: "Synthetic case"}}
		}, false},
		{"duplicate project", func(p *Profile) {
			ref := ChatGPTProjectReference{ProjectID: "g-p-11111111111111111111111111111111", ProjectName: "Synthetic case"}
			p.ChatGPTProjects = []ChatGPTProjectReference{ref, ref}
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := DefaultProfile()
			tc.change(&p)
			err := p.Validate(now)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%t, err=%v", tc.valid, err)
			}
		})
	}
}
