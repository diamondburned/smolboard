package posts

import (
	"encoding/json"
	"sort"
)

// StringSet represents an unordered set of strings.
type StringSet map[string]struct{}

func (s StringSet) MarshalJSON() ([]byte, error) {
	var strings = make([]string, 0, len(s))
	for str := range s {
		strings = append(strings, str)
	}
	return json.Marshal(strings)
}

func (s *StringSet) UnmarshalJSON(b []byte) error {
	var strings []string
	if err := json.Unmarshal(b, &strings); err != nil {
		return err
	}

	*s = make(map[string]struct{}, len(strings))

	for _, str := range strings {
		(*s)[str] = struct{}{}
	}

	return nil
}

func (s StringSet) Sorted() []string {
	var strings = make([]string, 0, len(s))
	for str := range s {
		strings = append(strings, str)
	}

	sort.Strings(strings)
	return strings
}

func (s StringSet) Set(str string) {
	s[str] = struct{}{}
}

// Int64Set represents an unordered set of int64s.
type Int64Set map[int64]struct{}

func (s Int64Set) MarshalJSON() ([]byte, error) {
	var int64s = make([]int64, 0, len(s))
	for str := range s {
		int64s = append(int64s, str)
	}
	return json.Marshal(int64s)
}

func (s *Int64Set) UnmarshalJSON(b []byte) error {
	var int64s []int64
	if err := json.Unmarshal(b, &int64s); err != nil {
		return err
	}

	*s = make(map[int64]struct{}, len(int64s))

	for _, str := range int64s {
		(*s)[str] = struct{}{}
	}

	return nil
}

func (s Int64Set) Set(i64 int64) {
	s[i64] = struct{}{}
}
