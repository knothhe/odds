package uicopy

import (
	"fmt"
	"reflect"
	"regexp"
	"testing"

	"github.com/egoist/mygo/i18n"
)

func TestCopyContracts(t *testing.T) {
	placeholders := regexp.MustCompile(`%[+#0 .\d-]*[a-zA-Z]`)
	for key, value := range messages {
		if value.en == "" || value.zh == "" {
			t.Errorf("empty translation: %s", key)
		}
		if !reflect.DeepEqual(placeholders.FindAllString(value.en, -1), placeholders.FindAllString(value.zh, -1)) {
			t.Errorf("placeholder mismatch: %s", key)
		}
	}
}

func TestRegionalLanguagesAndEnglishFallback(t *testing.T) {
	defer i18n.SetLocaleFunc(nil)
	for _, test := range []struct{ locale, want string }{
		{"en", "Settings"}, {"en-GB", "Settings"}, {"zh-Hans", "设置"}, {"zh-CN", "设置"}, {"zh-SG", "设置"},
		{"zh-Hant", "Settings"}, {"zh-TW", "Settings"}, {"fr-FR", "Settings"}, {"invalid", "Settings"},
	} {
		i18n.SetLocaleFunc(func() string { return test.locale })
		if got := Settings(); got != test.want {
			t.Errorf("%s: %s, want %s", test.locale, got, test.want)
		}
	}
}

func TestMessagesReformatAfterLanguageChanges(t *testing.T) {
	locale := "en"
	i18n.SetLocaleFunc(func() string { return locale })
	defer i18n.SetLocaleFunc(nil)
	if got := fmt.Sprintf(ShortcutLabel(), CenterCurrent()); got != "Shortcut for Move to the center of this display" {
		t.Fatal(got)
	}
	locale = "zh-Hans"
	if got := fmt.Sprintf(ShortcutLabel(), CenterCurrent()); got != "移到当前屏幕中央的快捷键" {
		t.Fatal(got)
	}
}
