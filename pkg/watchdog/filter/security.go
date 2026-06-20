package filter

import (
	"regexp"
)

const Redacted = "[REDACTED]"

var (
	apiKeyPatterns = []*regexp.Regexp{
		regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`),
		regexp.MustCompile(`pk-[A-Za-z0-9]{20,}`),
		regexp.MustCompile(`ghp_[A-Za-z0-9]{36,}`),
		regexp.MustCompile(`gho_[A-Za-z0-9]{36,}`),
		regexp.MustCompile(`ghu_[A-Za-z0-9]{36,}`),
		regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`),
		regexp.MustCompile(`sk_live_[A-Za-z0-9]{24,}`),
		regexp.MustCompile(`sk_test_[A-Za-z0-9]{24,}`),
	}

	passwordPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:password|passwd|pwd|secret|token|apikey|api_key)\s*[:=]\s*\S+`),
		regexp.MustCompile(`(?:PASS|SECRET|TOKEN)\s*[:=]\s*\S+`),
	}

	privateKeyPatterns = []*regexp.Regexp{
		regexp.MustCompile(`-----BEGIN\s.*PRIVATE\sKEY-----[\s\S]*?-----END\s.*PRIVATE\sKEY-----`),
		regexp.MustCompile(`-----BEGIN\sRSA\sPRIVATE\sKEY-----[\s\S]*?-----END\sRSA\sPRIVATE\sKEY-----`),
		regexp.MustCompile(`-----BEGIN\sEC\sPRIVATE\sKEY-----[\s\S]*?-----END\sEC\sPRIVATE\sKEY-----`),
		regexp.MustCompile(`-----BEGIN\sOPENSSH\sPRIVATE\sKEY-----[\s\S]*?-----END\sOPENSSH\sPRIVATE\sKEY-----`),
	}

	piiPatterns = []*regexp.Regexp{
		regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`),
		regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
		regexp.MustCompile(`\b(?:[0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}\b`),
		regexp.MustCompile(`\b(?:[0-9a-fA-F]{1,4}:){1,7}:?\b`),
	}

	internalPathPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\\\\[A-Za-z0-9_.$-]+\\[A-Za-z0-9_.$-]+`),
		regexp.MustCompile(`//[A-Za-z0-9_.$-]+/[A-Za-z0-9_.$-]+`),
	}

	envSecretPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:SECRET|TOKEN|PASSWORD|API_KEY|APIKEY)_?\w*\s*=\s*\S+`),
	}

	allPatterns = append(append(append(append(append(apiKeyPatterns,
		passwordPatterns...),
		privateKeyPatterns...),
		piiPatterns...),
		internalPathPatterns...),
		envSecretPatterns...)
)

func scanValue(value string) string {
	for _, p := range allPatterns {
		if p.MatchString(value) {
			return Redacted
		}
	}
	return value
}

func filterString(value string) string {
	return scanValue(value)
}

func filterList(v []any) []any {
	result := make([]any, len(v))
	for i, item := range v {
		if m, ok := item.(map[string]any); ok {
			result[i] = FilterEntityData(m)
		} else {
			result[i] = item
		}
	}
	return result
}

func FilterEntityData(data map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range data {
		switch val := v.(type) {
		case string:
			result[k] = filterString(val)
		case map[string]any:
			result[k] = FilterEntityData(val)
		case []any:
			result[k] = filterList(val)
		default:
			result[k] = v
		}
	}
	return result
}

func FilterRelationData(data map[string]any) map[string]any {
	return FilterEntityData(data)
}

func FilterDescription(desc string) string {
	return filterString(desc)
}