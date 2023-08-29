package service

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/lib/pq"
	"github.com/pascaldekloe/jwt"
	"regexp"
	"strings"
	"sync"
	"text/template" // not html/template
	"time"
)

const (
	minPageSize     = 1
	defaultPageSize = 10
	maxPageSize     = 99
)

var queriesCache sync.Map

var (
	reUUID                = regexp.MustCompile("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")
	reMultiSpace          = regexp.MustCompile(`(\s)+`)
	reMoreThan2Linebreaks = regexp.MustCompile(`(\n){2,}`)
	reMentions            = regexp.MustCompile(`\B@([a-zA-Z][a-zA-Z0-9_-]{0,17})(?:\b[^@]|$)`)
	reTags                = regexp.MustCompile(`\B#((?:\p{L}|\p{N}|_)+)(?:\b[^#]|$)`)
)

func isUniqueViolation(err error) bool {
	pqerr, ok := err.(*pq.Error)
	return ok && pqerr.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	pqerr, ok := err.(*pq.Error)
	return ok && pqerr.Code == "23503"
}

func NewClaims(userID string) jwt.Claims {
	var claims jwt.Claims
	claims.Subject = userID
	claims.Expires = jwt.NewNumericTime(time.Now().Add(authTokenTTL))
	return claims
}

/*
 * This function does three things.
 * 1. It uses golang's text/template package so you can use stuff like {{if .something}}{{end}}.
 * 2. It replaces @varnames with indexed placeholders ($1, $2, ...) which are the ones that the driver uses,
 *    so it doesn't do string concatenation which can lead to sql injection.
 * 3. Caches the templates so it's not that slow. Even tho this won't be a bottle neck in your application.
 */
func buildQuery(text string, data map[string]interface{}) (string, []interface{}, error) {
	var t *template.Template
	v, ok := queriesCache.Load(text)
	if !ok {
		var err error
		t, err = template.New("query").Parse(text)
		if err != nil {
			return "", nil, fmt.Errorf("could not parse sql query template: %w", err)
		}

		queriesCache.Store(text, t)
	} else {
		t = v.(*template.Template)
	}

	var wr bytes.Buffer
	if err := t.Execute(&wr, data); err != nil {
		return "", nil, fmt.Errorf("could not apply sql query data: %w", err)
	}

	query := wr.String()
	args := []interface{}{}
	for key, val := range data {
		if !strings.Contains(query, "@"+key) {
			continue
		}

		args = append(args, val)
		query = strings.ReplaceAll(query, "@"+key, fmt.Sprintf("$%d", len(args)))
	}
	return query, args, nil
}

func normalizePageSize(i uint64) uint64 {
	if i == 0 {
		return defaultPageSize
	}
	if i < minPageSize {
		return minPageSize
	}
	if i > maxPageSize {
		return maxPageSize
	}
	return i
}

func smartTrim(s string) string {
	oldLines := strings.Split(s, "\n")
	newLines := []string{}
	for _, line := range oldLines {
		line = strings.TrimSpace(reMultiSpace.ReplaceAllString(line, "$1"))
		newLines = append(newLines, line)
	}
	s = strings.Join(newLines, "\n")
	s = reMoreThan2Linebreaks.ReplaceAllString(s, "$1$1")
	return strings.TrimSpace(s)
}

func encodeCursor(key string, ts time.Time) string {
	s := fmt.Sprintf("%s,%s", key, ts.Format(time.RFC3339Nano))
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func decodeCursor(s string) (string, time.Time, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("could not base64 decode cursor: %w", err)
	}

	parts := strings.Split(string(b), ",")
	if len(parts) != 2 {
		return "", time.Time{}, errors.New("expected cursor to have two items split by comma")
	}

	ts, err := time.Parse(time.RFC3339Nano, parts[1])
	if err != nil {
		return "", time.Time{}, fmt.Errorf("could not parse cursor timestamp: %w", err)
	}

	key := parts[0]
	return key, ts, nil
}

func collectMentions(s string) []string {
	m := map[string]struct{}{}
	var u []string
	for _, submatch := range reMentions.FindAllStringSubmatch(s, -1) {
		val := submatch[1]
		if _, ok := m[val]; !ok {
			m[val] = struct{}{}
			u = append(u, val)
		}
	}
	return u
}

func ptrString(v string) *string {
	return &v
}

type client interface {
	*timelineItemClient | *commentClient | *notificationClient
}

type Set[C client] map[C]struct{}

func (s Set[C]) Add(key C) {
	s[key] = struct{}{}
}
func (s Set[C]) Remove(key C) {
	delete(s, key)
}
