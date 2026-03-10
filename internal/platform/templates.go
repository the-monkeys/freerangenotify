package platform

import (
	"bytes"
	"html/template"
	"strings"
)

// OrgInviteEmailData holds template variables for organization invite emails.
type OrgInviteEmailData struct {
	OrgName     string
	InviterName string
	Role        string
	RecipientEmail string
	CTALink     string
	CTAText     string
	Description string
}

// OrgInviteEmailSubject is the subject line for org invite emails.
const OrgInviteEmailSubject = "You've been invited to {{.OrgName}} — FreeRangeNotify"

// OrgInviteEmailBody is the HTML template for organization invite emails.
const OrgInviteEmailBody = `<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"><title>Organization Invitation</title></head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
    <div style="background-color: #f8f9fa; border-radius: 10px; padding: 30px; margin-bottom: 20px;">
        <h1 style="color: #2563eb; margin-top: 0;">You're Invited!</h1>
        <p>You've been invited to join the organization <strong>{{.OrgName}}</strong> as <strong style="color: #2563eb;">{{.Role}}</strong>.</p>
        <p>{{.Description}}</p>
        <div style="text-align: center; margin: 30px 0;">
            <a href="{{.CTALink}}"
               style="background-color: #2563eb; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; display: inline-block; font-weight: bold;">
                {{.CTAText}}
            </a>
        </div>
        <div style="background-color: #e9ecef; border-radius: 8px; padding: 16px; margin: 20px 0;">
            <p style="margin: 0 0 8px 0; font-weight: bold; color: #495057;">Your role: {{.Role}}</p>
            <p style="margin: 0; color: #6c757d; font-size: 14px;">Organization members can access all apps belonging to {{.OrgName}} and collaborate with the team.</p>
        </div>
        <hr style="border: none; border-top: 1px solid #dee2e6; margin: 30px 0;">
        <p style="color: #6c757d; font-size: 14px;">
            If you weren't expecting this invitation, you can safely ignore this email.
        </p>
    </div>
    <div style="text-align: center; color: #6c757d; font-size: 12px;">
        <p>&copy; 2026 FreeRangeNotify. All rights reserved.</p>
    </div>
</body>
</html>`

// OrgInviteInAppTitle is the title template for in-app org invite notifications.
const OrgInviteInAppTitle = "Invited to {{.OrgName}}"

// OrgInviteInAppBody is the body template for in-app org invite notifications.
const OrgInviteInAppBody = "You've been invited to join {{.OrgName}} as {{.Role}} by {{.InviterName}}. Open Organizations to view."

// RenderOrgInviteEmail renders the org invite email with the given data.
func RenderOrgInviteEmail(data OrgInviteEmailData) (subject, body string, err error) {
	tmplSubject := template.Must(template.New("subject").Parse(OrgInviteEmailSubject))
	var subjBuf bytes.Buffer
	if err := tmplSubject.Execute(&subjBuf, data); err != nil {
		return "", "", err
	}

	tmplBody := template.Must(template.New("body").Parse(OrgInviteEmailBody))
	var bodyBuf bytes.Buffer
	if err := tmplBody.Execute(&bodyBuf, data); err != nil {
		return "", "", err
	}
	return subjBuf.String(), bodyBuf.String(), nil
}

// RenderOrgInviteInApp renders the in-app notification content for org invite.
func RenderOrgInviteInApp(orgName, inviterName, role string) (title, body string) {
	data := map[string]string{
		"OrgName":     orgName,
		"InviterName": inviterName,
		"Role":        role,
	}
	tmplTitle := template.Must(template.New("title").Parse(OrgInviteInAppTitle))
	var tBuf bytes.Buffer
	_ = tmplTitle.Execute(&tBuf, data)

	tmplBody := template.Must(template.New("body").Parse(OrgInviteInAppBody))
	var bBuf bytes.Buffer
	_ = tmplBody.Execute(&bBuf, data)
	return tBuf.String(), bBuf.String()
}

// FormatRoleDisplay capitalizes role for display (e.g., "admin" -> "Admin").
func FormatRoleDisplay(role string) string {
	if len(role) == 0 {
		return role
	}
	return strings.ToUpper(role[:1]) + role[1:]
}

// OrgInviteEmailDataWithURL creates OrgInviteEmailData with frontend URL and CTA.
func OrgInviteEmailDataWithURL(orgName, inviterName, role, recipientEmail, frontendURL string, userExists bool) OrgInviteEmailData {
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}
	data := OrgInviteEmailData{
		OrgName:        orgName,
		InviterName:    inviterName,
		Role:           FormatRoleDisplay(role),
		RecipientEmail: recipientEmail,
	}
	if userExists {
		data.CTALink = frontendURL + "/login"
		data.CTAText = "Log In to Accept"
		data.Description = "You already have a FreeRangeNotify account. Log in to access the organization."
	} else {
		data.CTALink = frontendURL + "/register"
		data.CTAText = "Create Your Account"
		data.Description = "Create a FreeRangeNotify account with this email address to accept the invitation."
	}
	return data
}
