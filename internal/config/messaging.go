package config

import (
	"fmt"
	"github.com/spf13/viper"
	"reflect"
	"regexp"
	"strings"
)

// Bind leaves explicitly: AutomaticEnv alone does not discover env-only keys
// during Unmarshal when a minimal/production YAML omits a provider section.
func bindMessagingEnvironment() {
	var bind func(reflect.Type, string)
	bind = func(typ reflect.Type, prefix string) {
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			key := f.Tag.Get("mapstructure")
			if key == "" {
				continue
			}
			if prefix != "" {
				key = prefix + "." + key
			}
			if f.Type.Kind() == reflect.Struct {
				bind(f.Type, key)
			} else {
				_ = viper.BindEnv(key, "FREERANGE_"+strings.ToUpper(strings.ReplaceAll(key, ".", "_")))
			}
		}
	}
	for _, name := range []string{"Messaging", "Providers"} {
		f, _ := reflect.TypeOf(Config{}).FieldByName(name)
		bind(f.Type, f.Tag.Get("mapstructure"))
	}
}

func (c *Config) resolveMessaging() error {
	m := c.Messaging
	p := &c.Providers.MetaWhatsApp
	if m.MetaEnabled != nil {
		p.Enabled = *m.MetaEnabled
		c.Features.WhatsAppMetaEnabled = *m.MetaEnabled
	}
	if m.MetaAppID != "" {
		p.MetaAppID = m.MetaAppID
	}
	if m.MetaGraphVersion != "" {
		p.APIVersion = m.MetaGraphVersion
	}
	if !regexp.MustCompile(`^v[0-9]+\.[0-9]+$`).MatchString(p.APIVersion) {
		return fmt.Errorf("messaging Meta Graph version must use vN.N format")
	}
	if m.MetaAppSecret != "" {
		if p.AppSecret != "" && p.AppSecret != m.MetaAppSecret || p.MetaAppSecret != "" && p.MetaAppSecret != m.MetaAppSecret {
			return fmt.Errorf("canonical Meta secret conflicts with legacy signing/OAuth context; remove conflicting aliases")
		}
		p.AppSecret = m.MetaAppSecret
		p.MetaAppSecret = m.MetaAppSecret
	}
	// Legacy distinct contexts remain distinct; a sole secret represents one app.
	if p.AppSecret == "" {
		p.AppSecret = p.MetaAppSecret
	}
	if p.MetaAppSecret == "" {
		p.MetaAppSecret = p.AppSecret
	}
	shared := func(account, token *string) {
		if *account == "" && *token == "" {
			*account = m.TwilioAccountSID
			*token = m.TwilioAuthToken
		} else if *account == m.TwilioAccountSID && *token == "" {
			*token = m.TwilioAuthToken
		}
	}
	shared(&c.Providers.Twilio.AccountSID, &c.Providers.Twilio.AuthToken)
	shared(&c.Providers.WhatsApp.AccountSID, &c.Providers.WhatsApp.AuthToken)
	return nil
}
