package hooks

import (
	"regexp"
	"strings"
)

// Matcher はツール名のマッチングを行う
type Matcher struct {
	pattern string
	regex   *regexp.Regexp
	isExact bool
}

// NewMatcher は新しいMatcherを作成する
// patternが空の場合は全てにマッチする
// patternに正規表現メタ文字が含まれる場合は正規表現として扱う
func NewMatcher(pattern string) *Matcher {
	if pattern == "" {
		return &Matcher{pattern: "", isExact: false}
	}

	// 正規表現メタ文字が含まれているかチェック
	hasRegexChars := strings.ContainsAny(pattern, ".*+?^${}[]|()\\")

	m := &Matcher{
		pattern: pattern,
		isExact: !hasRegexChars,
	}

	if hasRegexChars {
		// 正規表現としてコンパイル
		re, err := regexp.Compile(pattern)
		if err != nil {
			// コンパイル失敗時は完全一致として扱う
			m.isExact = true
		} else {
			m.regex = re
		}
	}

	return m
}

// Match はツール名がパターンにマッチするかを判定する
func (m *Matcher) Match(toolName string) bool {
	// パターンが空の場合は全てにマッチ
	if m.pattern == "" {
		return true
	}

	// 完全一致
	if m.isExact {
		return m.pattern == toolName
	}

	// 正規表現マッチング
	if m.regex != nil {
		return m.regex.MatchString(toolName)
	}

	return false
}

// Pattern はパターン文字列を返す
func (m *Matcher) Pattern() string {
	return m.pattern
}

// IsExact は完全一致かどうかを返す
func (m *Matcher) IsExact() bool {
	return m.isExact
}
