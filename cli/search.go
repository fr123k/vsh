package cli

import (
	"fmt"
	"index/suffixarray"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/andreyvit/diff"
	"github.com/fatih/color"
)

// SearchingCommand interface to describe a command that performs a search operation
type SearchingCommand interface {
	GetSearchParams() SearchParameters
}

// SearchParameters struct are parameters common to a command that performs a search operation
type SearchParameters struct {
	Search      string
	Replacement *string
	KeySelector string
	Mode        KeyValueMode
	IsRegexp    bool
}

// Match structure to keep indices of matched and replaced terms
type Match struct {
	path  string
	key   string
	value string
	// sorted slices of indices of match starts and length
	keyIndex   [][]int
	valueIndex [][]int
	// in-line diffs of key and value replacements
	keyLineDiff   string
	valueLineDiff string
	// final strings after replacement
	replacedKey   string
	replacedValue string
}

// Searcher provides matching and replacement methods while maintaining references to the command
// that provides an interface to search operations. Also maintains reference to a compiled regexp.
type Searcher struct {
	cmd           SearchingCommand
	regexp        *regexp.Regexp
	keySelectorRe *regexp.Regexp
}

// NewSearcher creates a new Searcher container for performing search and optionally replace
func NewSearcher(cmd SearchingCommand) (*Searcher, error) {
	var re, keySelectorRe *regexp.Regexp
	var err error
	params := cmd.GetSearchParams()

	if params.IsRegexp {
		re, err = regexp.Compile(params.Search)
		if err != nil {
			return nil, fmt.Errorf("cannot parse regex pattern")
		}
	}
	if params.KeySelector != "" && params.IsRegexp == true {
		keySelectorRe, err = regexp.Compile(params.KeySelector)
		if err != nil {
			return nil, fmt.Errorf("key-selector: %s", err)
		}
	}

	return &Searcher{cmd: cmd, regexp: re, keySelectorRe: keySelectorRe}, nil
}

// IsMode returns true if the specified mode is enabled
func (s *Searcher) IsMode(mode KeyValueMode) bool {
	return s.cmd.GetSearchParams().Mode&mode == mode
}

// DoSearch searches with either regexp or substring search methods
func (s *Searcher) DoSearch(path string, k string, v string) (m []*Match) {
	// Default to original strings
	replacedKey, keyLineDiff := k, k
	replacedValue, valueLineDiff := v, v
	var keyMatchPairs, valueMatchPairs, keySelectorMatches [][]int

	if s.cmd.GetSearchParams().KeySelector != "" {
		keySelectorMatches = s.keySelectorMatches(k)
		if len(keySelectorMatches) == 0 {
			return m
		}
	}
	if s.IsMode(ModeKeys) {
		keyMatchPairs, replacedKey, keyLineDiff = s.matchData(k)
	}
	if len(keySelectorMatches) > 0 {
		keyLineDiff = highlightMatches(keyLineDiff, s.keySelectorMatches(keyLineDiff))
	}

	if s.IsMode(ModeValues) {
		valueMatchPairs, replacedValue, valueLineDiff = s.matchData(v)
	}

	if len(keyMatchPairs) > 0 || len(valueMatchPairs) > 0 {
		m = []*Match{
			{
				path:          path,
				key:           k,
				value:         v,
				keyIndex:      keyMatchPairs,
				valueIndex:    valueMatchPairs,
				keyLineDiff:   keyLineDiff,
				valueLineDiff: valueLineDiff,
				replacedKey:   replacedKey,
				replacedValue: replacedValue,
			},
		}
	}
	return m
}

func (match *Match) print(out io.Writer, diff bool) {
	if diff == true {
		fmt.Fprintf(out, "%s> %s = %s\n", match.path, match.keyLineDiff, match.valueLineDiff)
	} else {
		fmt.Fprintf(out, "%s> %s = %s\n", match.path, highlightMatches(match.key, match.keyIndex), highlightMatches(match.value, match.valueIndex))
	}
}

// keySelectorMatches provides an array of start and end indexes of key selector matches
func (s *Searcher) keySelectorMatches(k string) (matches [][]int) {
	if s.cmd.GetSearchParams().IsRegexp == true {
		return s.keySelectorRe.FindAllStringIndex(k, -1)
	}
	if k == s.cmd.GetSearchParams().KeySelector {
		return [][]int{{0, len(k)}}
	}
	return [][]int{}
}

// highlightMatches will take an array of start and end indexes and highlight them
func highlightMatches(s string, matches [][]int) (result string) {
	cur := 0
	if len(matches) > 0 {
		for _, pair := range matches {
			next := pair[0]
			end := pair[1]
			result += s[cur:next]
			result += color.New(color.FgYellow).SprintFunc()(s[next:end])
			cur = end
		}
		result += s[cur:]
	} else {
		return s
	}
	return result
}

// highlightLineDiff will consume (~~del~~)(++add++) markup and colorize in its place
func (s *Searcher) highlightLineDiff(d string) string {
	var buf, res []byte
	removeMode, addMode := false, false
	removeColor := color.New(color.FgWhite).Add(color.BgRed)
	addColor := color.New(color.FgWhite).Add(color.BgGreen)

	for _, b := range []byte(d) {
		buf = append(buf, b)
		if len(buf) >= 3 && string(buf[len(buf)-3:]) == "(~~" && !removeMode && !addMode {
			res = append(res, buf[0:len(buf)-3]...)
			buf = make([]byte, 0)
			removeMode = true
		} else if len(buf) > 3 && string(buf[len(buf)-3:]) == "~~)" && removeMode {
			res = append(res, removeColor.SprintFunc()(string(buf[0:len(buf)-3]))...)
			buf = make([]byte, 0)
			removeMode = false
		} else if len(buf) >= 3 && string(buf[len(buf)-3:]) == "(++" && !removeMode && !addMode {
			res = append(res, buf[0:len(buf)-3]...)
			buf = make([]byte, 0)
			addMode = true
		} else if len(buf) > 3 && string(buf[len(buf)-3:]) == "++)" && addMode {
			res = append(res, addColor.SprintFunc()(string(buf[0:len(buf)-3]))...)
			buf = make([]byte, 0)
			addMode = false
		}
	}
	return string(append(res, buf...))
}

func (s *Searcher) substrMatchData(subject string, search string) (matchPairs [][]int) {
	index := suffixarray.New([]byte(subject))
	matches := index.Lookup([]byte(search), -1)
	sort.Ints(matches)
	substrLength := len(search)
	for _, offset := range matches {
		matchPairs = append(matchPairs, []int{offset, offset + substrLength})
	}
	return matchPairs
}

func (s *Searcher) regexpMatchData(subject string, re *regexp.Regexp) (matchPairs [][]int) {
	return re.FindAllStringIndex(subject, -1)
}

func (s *Searcher) matchData(subject string) (matchPairs [][]int, replaced string, inlineDiff string) {
	replaced, inlineDiff = subject, subject
	matchPairs = make([][]int, 0)

	if s.cmd.GetSearchParams().IsRegexp {
		matchPairs = s.regexpMatchData(subject, s.regexp)
	} else {
		matchPairs = s.substrMatchData(subject, s.cmd.GetSearchParams().Search)
	}

	if s.cmd.GetSearchParams().Replacement != nil {
		if s.cmd.GetSearchParams().IsRegexp {
			replaced = s.regexp.ReplaceAllString(subject, *s.cmd.GetSearchParams().Replacement)
		} else {
			replaced = strings.ReplaceAll(subject, s.cmd.GetSearchParams().Search, *s.cmd.GetSearchParams().Replacement)
		}
		inlineDiff = s.highlightLineDiff(diff.CharacterDiff(subject, replaced))
	}

	return matchPairs, replaced, inlineDiff
}