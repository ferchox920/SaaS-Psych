package client

import (
	"net/mail"
	"regexp"
	"sort"
	"strings"
	"time"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

// Profile contains contact details and reminder preferences, not delivery activation.
type Profile struct {
	Email             string                    `json:"email"`
	Phone             string                    `json:"phone"`
	BirthDate         string                    `json:"birth_date"`
	ReminderChannel   string                    `json:"reminder_channel"`
	ReminderLeadHours int                       `json:"reminder_lead_hours"`
	ReminderConsent   bool                      `json:"reminder_consent"`
	Revision          int                       `json:"revision"`
	ChatGPTProjects   []ChatGPTProjectReference `json:"chatgpt_projects"`
}

// References record an explicit patient association, without importing clinical content.
type ChatGPTProjectReference struct {
	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name"`
}

func DefaultProfile() Profile { return Profile{ReminderChannel: "none", ReminderLeadHours: 24} }

var internationalPhone = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
var chatGPTProjectID = regexp.MustCompile(`^g-p-[a-f0-9]{32}$`)

func (p *Profile) Validate(now time.Time) error {
	if len(p.ChatGPTProjects) > 5 {
		return domainerrors.NewValidation("at most five ChatGPT projects per patient")
	}
	seenProjects := make(map[string]bool)
	for i := range p.ChatGPTProjects {
		ref := &p.ChatGPTProjects[i]
		ref.ProjectID = strings.TrimSpace(ref.ProjectID)
		ref.ProjectName = strings.TrimSpace(ref.ProjectName)
		if !chatGPTProjectID.MatchString(ref.ProjectID) || ref.ProjectName == "" || len([]rune(ref.ProjectName)) > 200 {
			return domainerrors.NewValidation("ChatGPT project needs a valid project_id and name")
		}
		if seenProjects[ref.ProjectID] {
			return domainerrors.NewValidation("duplicate ChatGPT project reference")
		}
		seenProjects[ref.ProjectID] = true
	}
	sort.Slice(p.ChatGPTProjects, func(i, j int) bool { return p.ChatGPTProjects[i].ProjectID < p.ChatGPTProjects[j].ProjectID })
	p.Email = strings.TrimSpace(p.Email)
	p.Phone = strings.TrimSpace(p.Phone)
	if p.Revision < 0 {
		return domainerrors.NewValidation("revision must be non-negative")
	}
	if p.Email != "" {
		address, err := mail.ParseAddress(p.Email)
		if err != nil || address.Address != p.Email || len(p.Email) > 254 {
			return domainerrors.NewValidation("email must be a valid address")
		}
	}
	if p.Phone != "" && !internationalPhone.MatchString(p.Phone) {
		return domainerrors.NewValidation("phone must include country code, e.g. +5491112345678")
	}
	if p.BirthDate != "" {
		date, err := time.Parse("2006-01-02", p.BirthDate)
		if err != nil || p.BirthDate > now.Format("2006-01-02") || date.Year() < 1900 {
			return domainerrors.NewValidation("birth_date must be a past date from 1900 onward")
		}
	}
	if p.ReminderLeadHours != 24 {
		return domainerrors.NewValidation("reminders currently support 24 hours only")
	}
	switch p.ReminderChannel {
	case "none":
	case "email":
		if p.Email == "" || !p.ReminderConsent {
			return domainerrors.NewValidation("email reminders require an address and explicit consent")
		}
	case "whatsapp":
		if p.Phone == "" || !p.ReminderConsent {
			return domainerrors.NewValidation("WhatsApp reminders require a phone and explicit consent")
		}
	default:
		return domainerrors.NewValidation("invalid reminder channel")
	}
	return nil
}
