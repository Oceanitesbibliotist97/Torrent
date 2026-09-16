package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"testing"
)

var pluralForms = map[string]bool{"zero": true, "one": true, "two": true, "few": true, "many": true, "other": true}

func loadDict(t *testing.T, lang string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("frontend", "dist", "i18n", lang+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var dict map[string]any
	if err := json.Unmarshal(data, &dict); err != nil {
		t.Fatalf("%s.json: %v", lang, err)
	}
	return dict
}

func isPlural(m map[string]any) bool {
	if len(m) == 0 {
		return false
	}
	for k, v := range m {
		if _, ok := v.(string); !ok || !pluralForms[k] {
			return false
		}
	}
	return true
}

// flatten maps dotted keys to either "string" or the sorted plural forms.
func flatten(prefix string, m map[string]any, out map[string][]string) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case string:
			out[key] = nil
		case map[string]any:
			if isPlural(val) {
				forms := make([]string, 0, len(val))
				for f := range val {
					forms = append(forms, f)
				}
				sort.Strings(forms)
				out[key] = forms
			} else {
				flatten(key, val, out)
			}
		}
	}
}

func TestTranslationsMatch(t *testing.T) {
	en := map[string][]string{}
	ru := map[string][]string{}
	flatten("", loadDict(t, "en"), en)
	flatten("", loadDict(t, "ru"), ru)
	for key, forms := range en {
		ruForms, ok := ru[key]
		if !ok {
			t.Errorf("ru.json is missing %q", key)
			continue
		}
		if (forms == nil) != (ruForms == nil) {
			t.Errorf("%q is plural in one language only", key)
			continue
		}
		if forms != nil {
			if !slices.Contains(forms, "one") || !slices.Contains(forms, "other") {
				t.Errorf("en %q needs one/other forms, has %v", key, forms)
			}
			for _, f := range []string{"one", "few", "many", "other"} {
				if !slices.Contains(ruForms, f) {
					t.Errorf("ru %q is missing plural form %q", key, f)
				}
			}
		}
	}
	for key := range ru {
		if _, ok := en[key]; !ok {
			t.Errorf("en.json is missing %q", key)
		}
	}
}

// TestStaticKeysExist catches typos in t('literal.key') calls.
func TestStaticKeysExist(t *testing.T) {
	en := map[string][]string{}
	flatten("", loadDict(t, "en"), en)
	call := regexp.MustCompile(`\bt\(\s*'([A-Za-z0-9_.]+)'`)
	err := filepath.WalkDir(filepath.Join("frontend", "dist", "js"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".js" {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range call.FindAllStringSubmatch(string(src), -1) {
			if _, ok := en[m[1]]; !ok {
				t.Errorf("%s uses unknown key %q", path, m[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
