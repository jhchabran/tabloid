package tabloid

import (
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"time"
)

var NowFunc func() time.Time = time.Now

var helpers template.FuncMap = template.FuncMap{
	"daysAgo": func(t time.Time) string {
		now := NowFunc()
		days := int(now.Sub(t).Hours() / 24)

		if days < 1 {
			return "today"
		}
		return strconv.Itoa(days) + " days ago"
	},
	"title": strings.Title,
	"dict": func(values ...interface{}) (map[string]interface{}, error) {
		if len(values)%2 != 0 {
			return nil, fmt.Errorf("invalid dict call, odd number of arguments")
		}

		dict := make(map[string]interface{}, len(values)/2)
		for i := 0; i < len(values); i += 2 {
			k, ok := values[i].(string)
			if !ok {
				return nil, fmt.Errorf("dict keys must be strings")
			}
			v := values[i+1]
			dict[k] = v
		}

		return dict, nil
	},
}
