// Copyright 2014 Unknwon
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

// Package ini provides INI file read and write functionality in Go.
package ini

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

const (
	// DefaultSection is the name of empty section
	DefaultSection = "DEFAULT"
	// Maximum allowed depth when recursively substituing variable names.
	_DepthValues = 99
)

var (
	// LineBreak depends on OS. LF by default (Linux)
	LineBreak = "\n"

	// Variable regexp pattern: %(variable)s
	varPattern = regexp.MustCompile(`%\(([^\)]+)\)s`)

	// PrettyFormat put spaces around "=" to look better.
	PrettyFormat = true
	// UnquoteValues strips "quotation marks" from values
	UnquoteValues = true
)

func init() {
	if runtime.GOOS == "windows" {
		LineBreak = "\r\n"
	}
}

func inSlice(str string, s []string) bool {
	for _, v := range s {
		if str == v {
			return true
		}
	}
	return false
}

// dataSource is a interface that returns file content.
type dataSource interface {
	Reader() (io.Reader, error)
}

type sourceFile struct {
	name string
}

func (s sourceFile) Reader() (io.Reader, error) {
	return os.Open(s.name)
}

type sourceData struct {
	data []byte
}

func (s *sourceData) Reader() (io.Reader, error) {
	return bytes.NewReader(s.data), nil
}

//  ____  __.
// |    |/ _|____ ___.__.
// |      <_/ __ <   |  |
// |    |  \  ___/\___  |
// |____|__ \___  > ____|
//         \/   \/\/

// Key represents a key under a section.
type Key struct {
	s          *Section
	Comment    string
	name       string
	value      string
	isAutoIncr bool
}

// Name returns name of key.
func (k *Key) Name() string {
	return k.name
}

// Value returns raw value of key for performance purpose.
func (k *Key) Value() string {
	return k.value
}

// String returns string representation of a value (without quotes)
func (k *Key) String() string {
	vlen := len(k.value) - 1
	if vlen <= 1 {
		return k.Expand()
	}
	if k.value[0] == '"' && k.value[vlen] == '"' {
		return k.Expand()[1:vlen]
	}
	return k.Expand()
}

// Expand returns string representation of value.
func (k *Key) Expand() string {
	val := k.value
	if strings.Index(val, "%") == -1 {
		return val
	}

	for i := 0; i < _DepthValues; i++ {
		vr := varPattern.FindString(val)
		if len(vr) == 0 {
			break
		}

		// Take off leading '%(' and trailing ')s'.
		noption := strings.TrimLeft(vr, "%(")
		noption = strings.TrimRight(noption, ")s")

		// Search in the same section.
		nk, err := k.s.GetKey(noption)
		if err != nil {
			// Search again in default section.
			nk, _ = k.s.f.Section("").GetKey(noption)
		}

		// Substitute by new value and take off leading '%(' and trailing ')s'.
		val = strings.Replace(val, vr, nk.value, -1)
	}
	return val
}

// Bool returns bool type value.
func (k *Key) Bool() (bool, error) {
	return strconv.ParseBool(k.String())
}

// Float64 returns float64 type value.
func (k *Key) Float64() (float64, error) {
	return strconv.ParseFloat(k.String(), 64)
}

// Int returns int type value.
func (k *Key) Int() (int, error) {
	return strconv.Atoi(k.String())
}

// Int64 returns int64 type value.
func (k *Key) Int64() (int64, error) {
	return strconv.ParseInt(k.String(), 10, 64)
}

// MustBool always returns value without error,
// it returns false if error occurs.
func (k *Key) MustBool(defaultVal ...bool) bool {
	val, err := k.Bool()
	if len(defaultVal) > 0 && err != nil {
		return defaultVal[0]
	}
	return val
}

// MustFloat64 always returns value without error,
// it returns 0.0 if error occurs.
func (k *Key) MustFloat64(defaultVal ...float64) float64 {
	value, err := k.Float64()
	if len(defaultVal) > 0 && err != nil {
		return defaultVal[0]
	}
	return value
}

// MustInt always returns value without error,
// it returns 0 if error occurs.
func (k *Key) MustInt(defaultVal ...int) int {
	value, err := k.Int()
	if len(defaultVal) > 0 && err != nil {
		return defaultVal[0]
	}
	return value
}

// MustInt64 always returns value without error,
// it returns 0 if error occurs.
func (k *Key) MustInt64(defaultVal ...int64) int64 {
	value, err := k.Int64()
	if len(defaultVal) > 0 && err != nil {
		return defaultVal[0]
	}
	return value
}

// In always returns value without error,
// it returns default value if error occurs or doesn't fit into candidates.
func (k *Key) In(defaultVal string, candidates []string) string {
	val := k.String()
	for _, cand := range candidates {
		if val == cand {
			return val
		}
	}
	return defaultVal
}

// InFloat64 always returns value without error,
// it returns default value if error occurs or doesn't fit into candidates.
func (k *Key) InFloat64(defaultVal float64, candidates []float64) float64 {
	val := k.MustFloat64()
	for _, cand := range candidates {
		if val == cand {
			return val
		}
	}
	return defaultVal
}

// InInt always returns value without error,
// it returns default value if error occurs or doesn't fit into candidates.
func (k *Key) InInt(defaultVal int, candidates []int) int {
	val := k.MustInt()
	for _, cand := range candidates {
		if val == cand {
			return val
		}
	}
	return defaultVal
}

// InInt64 always returns value without error,
// it returns default value if error occurs or doesn't fit into candidates.
func (k *Key) InInt64(defaultVal int64, candidates []int64) int64 {
	val := k.MustInt64()
	for _, cand := range candidates {
		if val == cand {
			return val
		}
	}
	return defaultVal
}

// Strings returns list of string devide by given delimiter.
func (k *Key) Strings(delim string) []string {
	vals := strings.Split(k.String(), delim)
	for i := range vals {
		vals[i] = strings.TrimSpace(vals[i])
	}
	return vals
}

// Float64s returns list of float64 devide by given delimiter.
func (k *Key) Float64s(delim string) []float64 {
	strs := k.Strings(delim)
	vals := make([]float64, len(strs))
	for i := range strs {
		vals[i], _ = strconv.ParseFloat(strs[i], 64)
	}
	return vals
}

// Ints returns list of int devide by given delimiter.
func (k *Key) Ints(delim string) []int {
	strs := k.Strings(delim)
	vals := make([]int, len(strs))
	for i := range strs {
		vals[i], _ = strconv.Atoi(strs[i])
	}
	return vals
}

// Int64s returns list of int64 devide by given delimiter.
func (k *Key) Int64s(delim string) []int64 {
	strs := k.Strings(delim)
	vals := make([]int64, len(strs))
	for i := range strs {
		vals[i], _ = strconv.ParseInt(strs[i], 10, 64)
	}
	return vals
}

// SetValue changes key value.
func (k *Key) SetValue(v string) {
	k.value = v
}

//   _________              __  .__
//  /   _____/ ____   _____/  |_|__| ____   ____
//  \_____  \_/ __ \_/ ___\   __\  |/  _ \ /    \
//  /        \  ___/\  \___|  | |  (  <_> )   |  \
// /_______  /\___  >\___  >__| |__|\____/|___|  /
//         \/     \/     \/                    \/

// Section represents a config section.
type Section struct {
	f        *File
	Comment  string
	name     string
	keys     map[string]*Key
	keyList  []string
	keysHash map[string]string
}

func newSection(f *File, name string) *Section {
	return &Section{f, "", name, make(map[string]*Key), make([]string, 0, 10), make(map[string]string)}
}

// Name returns name of Section.
func (s *Section) Name() string {
	return s.name
}

// NewKey creates a new key to given section.
func (s *Section) NewKey(name, val string) (*Key, error) {
	if len(name) == 0 {
		return nil, errors.New("error creating new key: empty key name")
	}

	if s.f.BlockMode {
		s.f.lock.Lock()
		defer s.f.lock.Unlock()
	}

	if inSlice(name, s.keyList) {
		s.keys[name].value = val
		return s.keys[name], nil
	}

	s.keyList = append(s.keyList, name)
	s.keys[name] = &Key{s, "", name, val, false}
	s.keysHash[name] = val
	return s.keys[name], nil
}

// GetKey returns key in section by given name.
func (s *Section) GetKey(name string) (*Key, error) {
	// FIXME: change to section level lock?
	if s.f.BlockMode {
		s.f.lock.RLock()
		defer s.f.lock.RUnlock()
	}

	key := s.keys[name]
	if key == nil {
		// Check if it is a child-section.
		if i := strings.LastIndex(s.name, "."); i > -1 {
			return s.f.Section(s.name[:i]).GetKey(name)
		}
		return nil, fmt.Errorf("error when getting key of section '%s': key '%s' not exists", s.name, name)
	}
	return key, nil
}

// Key assumes named Key exists in section and returns a zero-value when not.
func (s *Section) Key(name string) *Key {
	key, err := s.GetKey(name)
	if err != nil {
		return &Key{}
	}
	return key
}

// Keys returns list of keys of section.
func (s *Section) Keys() []*Key {
	keys := make([]*Key, len(s.keyList))
	for i := range s.keyList {
		keys[i] = s.Key(s.keyList[i])
	}
	return keys
}

// KeyStrings returns list of key names of section.
func (s *Section) KeyStrings() []string {
	list := make([]string, len(s.keyList))
	copy(list, s.keyList)
	return list
}

// KeysHash returns keys hash consisting of names and values.
func (s *Section) KeysHash(unquote bool) map[string]string {
	if s.f.BlockMode {
		s.f.lock.RLock()
		defer s.f.lock.RUnlock()
	}

	hash := map[string]string{}
	for key, value := range s.keysHash {
		if unquote && value[0] == '"' {
			l := len(value) - 1
			hash[key] = value[1:l]
		} else {
			hash[key] = value
		}
	}
	return hash
}

// DeleteKey deletes a key from section.
func (s *Section) DeleteKey(name string) {
	if s.f.BlockMode {
		s.f.lock.Lock()
		defer s.f.lock.Unlock()
	}

	for i, k := range s.keyList {
		if k == name {
			s.keyList = append(s.keyList[:i], s.keyList[i+1:]...)
			delete(s.keys, name)
			return
		}
	}
}

// ___________.__.__
// \_   _____/|__|  |   ____
//  |    __)  |  |  | _/ __ \
//  |     \   |  |  |_\  ___/
//  \___  /   |__|____/\___  >
//      \/                 \/

// File represents a combination of a or more INI file(s) in memory.
type File struct {
	// Should make things safe, but sometimes doesn't matter.
	BlockMode bool
	// Make sure data is safe in multiple goroutines.
	lock sync.RWMutex

	// Allow combination of multiple data sources.
	dataSources []dataSource
	// Actual data is stored here.
	sections map[string]*Section

	// To keep data in order.
	sectionList []string
}

// newFile initializes File object with given data sources.
func newFile(dataSources []dataSource) *File {
	return &File{
		BlockMode:   true,
		dataSources: dataSources,
		sections:    make(map[string]*Section),
		sectionList: make([]string, 0, 10),
	}
}

func parseDataSource(source interface{}) (dataSource, error) {
	switch s := source.(type) {
	case string:
		return sourceFile{s}, nil
	case []byte:
		return &sourceData{s}, nil
	default:
		return nil, fmt.Errorf("error parsing data source: unknown type '%s'", s)
	}
}

// Load loads and parses from INI data sources.
// Arguments can be mixed of file name with string type, or raw data in []byte.
func Load(source interface{}, others ...interface{}) (_ *File, err error) {
	sources := make([]dataSource, len(others)+1)
	sources[0], err = parseDataSource(source)
	if err != nil {
		return nil, err
	}
	for i := range others {
		sources[i+1], err = parseDataSource(others[i])
		if err != nil {
			return nil, err
		}
	}
	f := newFile(sources)
	return f, f.Reload()
}

// NewSection creates a new section.
func (f *File) NewSection(name string) (*Section, error) {
	if len(name) == 0 {
		return nil, errors.New("error creating new section: empty section name")
	}

	if f.BlockMode {
		f.lock.Lock()
		defer f.lock.Unlock()
	}

	if inSlice(name, f.sectionList) {
		return f.sections[name], nil
	}

	f.sectionList = append(f.sectionList, name)
	f.sections[name] = newSection(f, name)
	return f.sections[name], nil
}

// GetSection returns section by given name.
func (f *File) GetSection(name string) (*Section, error) {
	if len(name) == 0 {
		name = DefaultSection
	}

	if f.BlockMode {
		f.lock.RLock()
		defer f.lock.RUnlock()
	}

	sec := f.sections[name]
	if sec == nil {
		return nil, fmt.Errorf("error when getting section: section '%s' not exists", name)
	}
	return sec, nil
}

// Section assumes named section exists and returns a zero-value when not.
func (f *File) Section(name string) *Section {
	sec, err := f.GetSection(name)
	if err != nil {
		return newSection(f, name)
	}
	return sec
}

// Sections returns list of Section.
func (f *File) Sections() []*Section {
	sections := make([]*Section, len(f.sectionList))
	for i := range f.sectionList {
		sections[i] = f.Section(f.sectionList[i])
	}
	return sections
}

// SectionStrings returns list of section names.
func (f *File) SectionStrings() []string {
	list := make([]string, len(f.sectionList))
	copy(list, f.sectionList)
	return list
}

// DeleteSection deletes a section.
func (f *File) DeleteSection(name string) {
	if f.BlockMode {
		f.lock.Lock()
		defer f.lock.Unlock()
	}

	if len(name) == 0 {
		name = DefaultSection
	}

	for i, s := range f.sectionList {
		if s == name {
			f.sectionList = append(f.sectionList[:i], f.sectionList[i+1:]...)
			delete(f.sections, name)
			return
		}
	}
}

// parse parses data through an io.Reader.
func (f *File) parse(reader io.Reader) error {
	buf := bufio.NewReader(reader)

	// Handle BOM-UTF8.
	// http://en.wikipedia.org/wiki/Byte_order_mark#Representations_of_byte_order_marks_by_encoding
	mask, err := buf.Peek(3)
	if err == nil && len(mask) >= 3 && mask[0] == 239 && mask[1] == 187 && mask[2] == 191 {
		buf.Read(mask)
	}

	count := 1
	comments := ""
	isEnd := false

	section, err := f.NewSection(DefaultSection)
	if err != nil {
		return err
	}

	for {
		line, err := buf.ReadString('\n')
		line = strings.TrimSpace(line)
		length := len(line)

		// Check error and ignore io.EOF just for a moment.
		if err != nil {
			if err != io.EOF {
				return fmt.Errorf("error reading next line: %v", err)
			}
			// The last line of file could be an empty line.
			if length == 0 {
				break
			}
			isEnd = true
		}

		// Skip empty lines.
		if length == 0 {
			continue
		}

		switch {
		case line[0] == '#' || line[0] == ';': // Comments.
			if len(comments) == 0 {
				comments = line
			} else {
				comments += LineBreak + line
			}
			continue
		case line[0] == '[' && line[length-1] == ']': // New sction.
			name := strings.TrimSpace(line[1 : length-1])
			section, err = f.NewSection(name)
			if err != nil {
				return err
			}

			if len(comments) > 0 {
				section.Comment = comments
				comments = ""
			}
			// Reset counter.
			count = 1
			continue
		}

		// Other possibilities.
		var (
			i        int
			keyQuote string
			kname    string
			valQuote string
			val      string
		)

		// Key name surrounded by quotes.
		if line[0] == '"' {
			if length > 6 && line[0:3] == `"""` {
				keyQuote = `"""`
			} else {
				keyQuote = `"`
			}
		} else if line[0] == '`' {
			keyQuote = "`"
		}
		if len(keyQuote) > 0 {
			qLen := len(keyQuote)
			pos := strings.Index(line[qLen:], keyQuote)
			if pos == -1 {
				return fmt.Errorf("error parsing line: missing closing key quote: %s", line)
			}
			pos = pos + qLen
			i = strings.IndexAny(line[pos:], "=:")
			if i < 0 {
				return fmt.Errorf("error parsing line: key-value delimiter not found: %s", line)
			} else if i == pos {
				return fmt.Errorf("error parsing line: key is empty: %s", line)
			}
			i = i + pos
			kname = line[qLen:pos] // Just keep spaces inside quotes.
		} else {
			i = strings.IndexAny(line, "=:")
			if i < 0 {
				return fmt.Errorf("error parsing line: key-value delimiter not found: %s", line)
			} else if i == 0 {
				return fmt.Errorf("error parsing line: key is empty: %s", line)
			}
			kname = strings.TrimSpace(line[0:i])
		}

		isAutoIncr := false
		// Auto increment.
		if kname == "-" {
			isAutoIncr = true
			kname = "#" + fmt.Sprint(count)
			count++
		}

		lineRight := strings.TrimSpace(line[i+1:])
		lineRightLength := len(lineRight)
		firstChar := ""
		if lineRightLength >= 2 {
			firstChar = lineRight[0:1]
		}
		if firstChar == "`" {
			valQuote = "`"
		} else if lineRightLength >= 6 && lineRight[0:3] == `"""` {
			valQuote = `"""`
		}
		if len(valQuote) > 0 {
			qLen := len(valQuote)
			pos := strings.LastIndex(lineRight[qLen:], valQuote)
			// For multiple lines value.
			if pos == -1 {
				isEnd := false
				val = lineRight[qLen:] + "\n"
				for {
					next, err := buf.ReadString('\n')
					val += next
					if err != nil {
						if err != io.EOF {
							return err
						}
						isEnd = true
					}
					pos = strings.LastIndex(next, valQuote)
					if pos > -1 {
						val = val[:len(val)-len(valQuote)-1]
						break
					}
					if isEnd {
						return fmt.Errorf("error parsing line: missing closing key quote from '%s' to '%s'", line, next)
					}
				}
			} else {
				val = lineRight[qLen : pos+qLen]
			}
		} else {
			val = strings.TrimSpace(lineRight[0:])
		}

		k, err := section.NewKey(kname, val)
		if err != nil {
			return err
		}
		k.isAutoIncr = isAutoIncr
		if len(comments) > 0 {
			k.Comment = comments
			comments = ""
		}

		if isEnd {
			break
		}
	}
	return nil
}

// Reload reloads and parses all data sources.
func (f *File) Reload() error {
	for _, s := range f.dataSources {
		r, err := s.Reader()
		if err != nil {
			return err
		}
		if err = f.parse(r); err != nil {
			return err
		}
	}
	return nil
}

// Append appends one or more data sources and reloads automatically.
func (f *File) Append(source interface{}, others ...interface{}) error {
	ds, err := parseDataSource(source)
	if err != nil {
		return err
	}
	f.dataSources = append(f.dataSources, ds)
	for _, s := range others {
		ds, err = parseDataSource(s)
		if err != nil {
			return err
		}
		f.dataSources = append(f.dataSources, ds)
	}
	return f.Reload()
}

// SaveTo writes content to filesystem.
func (f *File) SaveTo(filename string) (err error) {
	equalSign := "="
	if PrettyFormat {
		equalSign = " = "
	}

	// Use buffer to make sure target is safe until finish encoding.
	buf := bytes.NewBuffer(nil)
	for i, sname := range f.sectionList {
		sec := f.Section(sname)
		if len(sec.Comment) > 0 {
			if sec.Comment[0] != '#' && sec.Comment[0] != ';' {
				sec.Comment = "; " + sec.Comment
			}
			if _, err = buf.WriteString(sec.Comment + LineBreak); err != nil {
				return err
			}
		}

		if i > 0 {
			if _, err = buf.WriteString("[" + sname + "]" + LineBreak); err != nil {
				return err
			}
		}

		for _, kname := range sec.keyList {
			key := sec.Key(kname)
			if len(key.Comment) > 0 {
				if key.Comment[0] != '#' && key.Comment[0] != ';' {
					key.Comment = "; " + key.Comment
				}
				if _, err = buf.WriteString(key.Comment + LineBreak); err != nil {
					return err
				}
			}

			switch {
			case key.isAutoIncr:
				kname = "-"
			case strings.Contains(kname, "`") || strings.Contains(kname, `"`):
				kname = `"""` + kname + `"""`
			case strings.Contains(kname, `=`) || strings.Contains(kname, `:`):
				kname = "`" + kname + "`"
			}

			val := key.value
			// In case key value contains "\n", "`" or "\"".
			if strings.Contains(val, "\n") || strings.Contains(val, "`") || strings.Contains(val, `"`) {
				val = `"""` + val + `"""`
			}
			if _, err = buf.WriteString(kname + equalSign + val + LineBreak); err != nil {
				return err
			}
		}

		// Put a line between sections.
		if _, err = buf.WriteString(LineBreak); err != nil {
			return err
		}
	}

	fw, err := os.Create(filename)
	if err != nil {
		return err
	}
	if _, err = buf.WriteTo(fw); err != nil {
		return err
	}
	return fw.Close()
}
