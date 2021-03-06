package kuu

import (
	"bytes"
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/json-iterator/go"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
)

var goSrcRegexp = regexp.MustCompile(`kuuland/kuu(@.*)?/.*.go`)
var goTestRegexp = regexp.MustCompile(`kuuland/kuu(@.*)?/.*test.go`)

// IsBlank
func IsBlank(value interface{}) bool {
	if value == nil {
		return true
	}
	indirectValue := indirectValue(value)
	switch indirectValue.Kind() {
	case reflect.String:
		return indirectValue.Len() == 0
	case reflect.Bool:
		return !indirectValue.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return indirectValue.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return indirectValue.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return indirectValue.Float() == 0
	case reflect.Map:
		return indirectValue.Len() == 0
	case reflect.Interface, reflect.Ptr:
		return indirectValue.IsNil()
	}
	vi := reflect.ValueOf(value)
	if vi.Kind() == reflect.Ptr {
		return vi.IsNil()
	}
	return reflect.DeepEqual(indirectValue.Interface(), reflect.Zero(indirectValue.Type()).Interface())
}

// IsNil
func IsNil(i interface{}) bool {
	defer func() {
		recover()
	}()
	if i == nil {
		return true
	}
	vi := reflect.ValueOf(i)
	return vi.IsNil()
}

func fileWithLineNum() string {
	for i := 2; i < 15; i++ {
		_, file, line, ok := runtime.Caller(i)
		if ok && (!goSrcRegexp.MatchString(file) || goTestRegexp.MatchString(file)) {
			return fmt.Sprintf("%v:%v", file, line)
		}
	}
	return ""
}

func indirectValue(value interface{}) reflect.Value {
	reflectValue := reflect.ValueOf(value)
	for reflectValue.Kind() == reflect.Ptr {
		reflectValue = reflectValue.Elem()
	}
	return reflectValue
}

func addrValue(value reflect.Value) reflect.Value {
	if value.CanAddr() && value.Kind() != reflect.Ptr {
		value = value.Addr()
	}
	return value
}

// CORSMiddleware
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "*")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// JSONStringify
func JSONStringify(v interface{}, format ...bool) string {
	var (
		data []byte
		err  error
	)
	if len(format) > 0 && format[0] {
		data, err = json.MarshalIndent(v, "", "  ")
	} else {
		data, err = json.Marshal(v)
	}
	if err == nil {
		return string(data)
	}
	return ""
}

// JSONParse
func JSONParse(v string, r interface{}) error {
	return json.Unmarshal([]byte(v), r)
}

// EnsureDir
func EnsureDir(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Println(err)
		}
	}
}

// ParseID
func ParseID(id string) uint {
	if v, err := strconv.ParseUint(id, 10, 0); err != nil {
		log.Println(err)
	} else {
		return uint(v)
	}
	return 0
}

// Copy
func Copy(src interface{}, dest interface{}) (err error) {
	var data []byte
	if data, err = json.Marshal(src); err == nil {
		err = json.Unmarshal(data, dest)
	}
	if err != nil {
		log.Println(err)
	}
	return
}

// RandCode
func RandCode(size ...int) string {
	length := 8
	if len(size) > 0 {
		length = size[0]
	}
	str := strings.ReplaceAll(uuid.NewV4().String(), "-", "")[:length]
	for i := 0; i < 4; i++ {
		idx := rand.Intn(len(str))
		str = strings.Replace(str, str[idx:idx+1], strings.ToUpper(str[idx:idx+1]), 1)
	}
	return str
}

// If
func If(condition bool, trueVal, falseVal interface{}) interface{} {
	if condition {
		return trueVal
	}
	return falseVal
}

// assert1 defined assert func
func assert1(guard bool, err interface{}) {
	if !guard {
		panic(err)
	}
}

// isFunc defined func check
func isFunc(target interface{}) bool {
	retType := reflect.TypeOf(target)
	return retType.Kind() == reflect.Func
}

// ProjectFields
func ProjectFields(data interface{}, project string) interface{} {
	if IsNil(data) {
		return data
	}

	if len(project) == 0 {
		return data
	}

	fields := strings.Split(project, ",")
	execProject := func(indirectValue reflect.Value) map[string]interface{} {
		var (
			raw interface{}
			val = make(map[string]interface{})
		)
		if indirectValue.CanAddr() {
			raw = indirectValue.Addr().Interface()
		} else {
			raw = indirectValue.Interface()
		}
		scope := DB().NewScope(raw)
		for _, name := range fields {
			if strings.HasPrefix(name, "-") {
				name = name[1:]
			}
			if field, ok := scope.FieldByName(name); ok {
				val[field.Name] = field.Field.Interface()
			}
		}
		return val
	}
	if indirectValue := indirectValue(data); indirectValue.Kind() == reflect.Slice {
		list := make([]map[string]interface{}, 0)
		for i := 0; i < indirectValue.Len(); i++ {
			item := execProject(indirectValue.Index(i))
			list = append(list, item)
		}
		return list
	} else {
		return execProject(indirectValue)
	}
}

// BatchInsertItem
type BatchInsertItem struct {
	SQL  string
	Vars []interface{}
}

// BatchInsert
func BatchInsert(tx *gorm.DB, insertBase string, items []BatchInsertItem, batchSize int) error {
	var (
		insertBuffer bytes.Buffer
		insertVars   []interface{}
	)
	for index, item := range items {
		if insertBuffer.Len() == 0 {
			insertBuffer.WriteString(insertBase)
		}

		insertBuffer.WriteString(item.SQL)
		insertVars = append(insertVars, item.Vars...)

		if (index+1)%batchSize == 0 || index == len(items)-1 {
			if sql := insertBuffer.String(); sql != "" {
				if err := tx.Exec(sql, insertVars...).Error; err != nil {
					return err
				}
				insertBuffer.Reset()
				insertVars = insertVars[0:0]
			}
		} else {
			insertBuffer.WriteString(", ")
		}
	}
	return nil
}

// EncodeURIComponent
func EncodeURIComponent(str string) string {
	r := url.QueryEscape(str)
	r = strings.Replace(r, "+", "%20", -1)
	return r
}

// DecodeURIComponent
func DecodeURIComponent(str string) string {
	r, err := url.QueryUnescape(str)
	if err != nil {
		ERROR(err)
	}
	return r
}

// SetUrlQuery
func SetUrlQuery(rawUrl string, values map[string]interface{}, replace ...bool) string {
	u, _ := url.Parse(rawUrl)
	if len(replace) > 0 && replace[0] {
		u.RawQuery = ""
	}
	query := u.Query()
	for k, v := range values {
		query.Set(k, fmt.Sprintf("%v", v))
	}
	u.RawQuery = query.Encode()
	return u.String()
}

// OmitFields
func OmitFields(src interface{}, fieldNames []string) (omitted map[string]interface{}) {
	_ = Copy(src, &omitted)
	for _, fieldName := range fieldNames {
		delete(omitted, fieldName)
	}
	return
}

// JSON
func JSON() jsoniter.API {
	return json
}

// ParseJSONPath
func ParseJSONPath(path string) []string {
	if v, has := parsePathCache.Load(path); has {
		return v.([]string)
	}

	var reg *regexp.Regexp

	reg = regexp.MustCompile(`(\[\d+\])`)
	path = reg.ReplaceAllString(path, ".$1.")

	reg = regexp.MustCompile(`\.+`)
	path = reg.ReplaceAllString(path, ".")

	path = strings.Trim(path, ".")

	keys := strings.Split(path, ".")
	parsePathCache.Store(path, keys)

	return keys
}
