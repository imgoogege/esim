package db2entity

import (
	"fmt"
	"github.com/spf13/viper"
	"go/format"
	"strconv"
	"strings"
	"unicode"
)

// Constants for return types of golang
const (
	golangByteArray  = "[]byte"
	gureguNullInt    = "null.Int"
	sqlNullInt       = "sql.NullInt64"
	golangInt        = "int"
	golangInt64      = "int64"
	gureguNullFloat  = "null.Float"
	sqlNullFloat     = "sql.NullFloat64"
	golangFloat      = "float"
	golangFloat32    = "float32"
	golangFloat64    = "float64"
	gureguNullString = "null.String"
	sqlNullString    = "sql.NullString"
	gureguNullTime   = "null.Time"
	golangTime       = "time.Time"
)

// commonInitialisms is a set of common initialisms.
// Only add entries that are highly unlikely to be non-initialisms.
// For instance, "ID" is fine (Freudian code is rare), but "AND" is not.
var commonInitialisms = map[string]bool{
	"API":   true,
	"ASCII": true,
	"CPU":   true,
	"CSS":   true,
	"DNS":   true,
	"EOF":   true,
	"GUID":  true,
	"HTML":  true,
	"HTTP":  true,
	"HTTPS": true,
	"ID":    true,
	"IP":    true,
	"JSON":  true,
	"LHS":   true,
	"QPS":   true,
	"RAM":   true,
	"RHS":   true,
	"RPC":   true,
	"SLA":   true,
	"SMTP":  true,
	"SSH":   true,
	"TLS":   true,
	"TTL":   true,
	"UI":    true,
	"UID":   true,
	"UUID":  true,
	"URI":   true,
	"URL":   true,
	"UTF8":  true,
	"VM":    true,
	"XML":   true,
}

var intToWordMap = []string{
	"zero",
	"one",
	"two",
	"three",
	"four",
	"five",
	"six",
	"seven",
	"eight",
	"nine",
}

//Debug level logging
var Debug = false

type generateMysqlInfo struct {
	dbTypes    string
	imports    []string
	comment    string
	autoTime   AutoTime
	del_key    string
	priKeyType string
}

// Generate Given a Column map with datatypes and a name structName,
// attempts to generate a struct definition
func Generate(columnTypes []columns, tableName string,
	structName string, pkgName string, jsonAnnotation bool,
	gormAnnotation bool, gureguTypes bool, v *viper.Viper) ([]byte, generateMysqlInfo, error) {

	var importstr string
	var conform string

	var varstr string
	//var vars []string

	genMysqlInfo := generateMysqlTypes(columnTypes, 0, jsonAnnotation,
		gormAnnotation, gureguTypes, v)

	if genMysqlInfo.priKeyType == ""{
		log.Fatalf("没有身份标识")
	}

	genMysqlInfo.imports = append(genMysqlInfo.imports, "github.com/jinzhu/gorm")

	//vars = append(vars, structName+"Obj = "+structName+"{}")

	if v.GetBool("mod") == true {
		genMysqlInfo.imports = append(genMysqlInfo.imports, "context")
	}

	if v.GetBool("mar") == true {
		//vars = append(vars, tableName+`Enc = jingo.NewStructEncoder(`+structName+`{})`)
		//genMysqlInfo.imports = append(genMysqlInfo.imports, "github.com/bet365/jingo")
	}

	//if len(genMysqlInfo.autoTime.CurTimeStamp) > 0 || len(genMysqlInfo.autoTime.OnUpdateTimeStamp) > 0 {
		genMysqlInfo.imports = append(genMysqlInfo.imports, "github.com/jukylin/esim/log")
	//}

	if len(genMysqlInfo.imports) > 0 {
		importstr = "import ("
		for _, v := range genMysqlInfo.imports {
			importstr += "\"" + v + "\" \n"
		}
		importstr += ")"
	}

	//if len(vars) > 0{
	//	varstr = "var ("
	//	for _, vs := range vars{
	//		varstr += vs + "\n"
	//	}
	//	varstr += ")"
	//}

	var mod_tips string
	if v.GetBool("mod") == true {
		mod_tips += "// 使用 mod 时 如果字段有非空默认值，保存的数据想把这个字段变空，需要去掉对应的 trim \n"
		mod_tips += "// OrderID  string `gorm:\"column:order_id;default:'abc'\" json:\"order_id,omitempty\" mod:\"trim\"` \n"
		mod_tips += "// OrderID  string `gorm:\"column:order_id;default:'abc'\" json:\"order_id,omitempty\"` \n"
	}

	src := fmt.Sprintf("package %s\n %s \n %s \n %s \n %s \n %s \n type %s %s}",
		"entity",
		importstr,
		varstr,
		conform,
		genMysqlInfo.comment,
		mod_tips,
		structName,
		genMysqlInfo.dbTypes)

	if genMysqlInfo.del_key != "" {
		delKeyFunc := "// delete field\n" +
			"func (" + strings.ToLower(string(structName[0])) + " *" + structName + ") DelKey() string {\n" +
			"	return \"" + genMysqlInfo.del_key + "\"" +
			"} \n \n"
		src = fmt.Sprintf("%s\n%s", src, delKeyFunc)
	} else {
		log.Warnf("匹配不到del字段")
		delKeyFunc := "// delete field\n" +
			"func (" + strings.ToLower(string(structName[0])) + " *" + structName + ") DelKey() string {\n" +
			"	return \"\" " +
			"} \n \n"
		src = fmt.Sprintf("%s\n%s", src, delKeyFunc)
	}

	//if v.GetBool("hasdata") == true {
	//	src += "//针对查询后是否有结果 \n"
	//	src += "func (" + strings.ToLower(string(structName[0])) + " *" + structName + ") HasData() bool {\n"
	//	src += "return " + strings.ToLower(string(structName[0])) + ".hasData \n"
	//	src += "}\n"
	//}

	beforeCreateBody := getBeforeCreateBody(genMysqlInfo, v, structName)
	beforeUpdateBody := getBeforeUpdateBody(genMysqlInfo, v, structName)
	afterFindBody := getAfterFindBody(genMysqlInfo)

	src += getBeforeCreate(structName, beforeCreateBody)
	src += getBeforeSave(structName, beforeUpdateBody)
	src += getAfterFind(structName, afterFindBody, v.GetBool("hasdata"))

	if v.GetBool("mar") == true {
		//src += getMarshaler(structName, tableName)
	}

	formatted, err := format.Source([]byte(src))
	if err != nil {
		err = fmt.Errorf("error formatting: %s, was formatting\n%s", err, src)
	}
	return formatted, genMysqlInfo, err
}

// fmtFieldName formats a string as a struct key
//
// Example:
// 	fmtFieldName("foo_id")
// Output: FooID
func fmtFieldName(s string) string {
	name := lintFieldName(s)
	runes := []rune(name)
	for i, c := range runes {
		ok := unicode.IsLetter(c) || unicode.IsDigit(c)
		if i == 0 {
			ok = unicode.IsLetter(c)
		}
		if !ok {
			runes[i] = '_'
		}
	}
	return string(runes)
}

func lintFieldName(name string) string {
	// Fast path for simple cases: "_" and all lowercase.
	if name == "_" {
		return name
	}

	for len(name) > 0 && name[0] == '_' {
		name = name[1:]
	}

	allLower := true
	for _, r := range name {
		if !unicode.IsLower(r) {
			allLower = false
			break
		}
	}
	if allLower {
		runes := []rune(name)
		if u := strings.ToUpper(name); commonInitialisms[u] {
			copy(runes[0:], []rune(u))
		} else {
			runes[0] = unicode.ToUpper(runes[0])
		}
		return string(runes)
	}

	// Split camelCase at any lower->upper transition, and split on underscores.
	// Check each word for common initialisms.
	runes := []rune(name)
	w, i := 0, 0 // index of start of word, scan
	for i+1 <= len(runes) {
		eow := false // whether we hit the end of a word

		if i+1 == len(runes) {
			eow = true
		} else if runes[i+1] == '_' {
			// underscore; shift the remainder forward over any run of underscores
			eow = true
			n := 1
			for i+n+1 < len(runes) && runes[i+n+1] == '_' {
				n++
			}

			// Leave at most one underscore if the underscore is between two digits
			if i+n+1 < len(runes) && unicode.IsDigit(runes[i]) && unicode.IsDigit(runes[i+n+1]) {
				n--
			}

			copy(runes[i+1:], runes[i+n+1:])
			runes = runes[:len(runes)-n]
		} else if unicode.IsLower(runes[i]) && !unicode.IsLower(runes[i+1]) {
			// lower->non-lower
			eow = true
		}
		i++
		if !eow {
			continue
		}

		// [w,i) is a word.
		word := string(runes[w:i])
		if u := strings.ToUpper(word); commonInitialisms[u] {
			// All the common initialisms are ASCII,
			// so we can replace the bytes exactly.
			copy(runes[w:], []rune(u))

		} else if strings.ToLower(word) == word {
			// already all lowercase, and not the first word, so uppercase the first character.
			runes[w] = unicode.ToUpper(runes[w])
		}
		w = i
	}
	return string(runes)
}

// convert first character ints to strings
func stringifyFirstChar(str string) string {
	first := str[:1]

	i, err := strconv.ParseInt(first, 10, 8)

	if err != nil {
		return str
	}

	return intToWordMap[i] + "_" + str[1:]
}

func getBeforeCreate(structName string, body string) string {
	beforeCreateStr := "\n //自动增加时间和 trim \n"
	beforeCreateStr += `func (this *` + structName + `) BeforeCreate(scope *gorm.Scope) (err error) {
			` + body + `
	return
	}

`

	return beforeCreateStr
}

func getBeforeSave(structName string, body string) string {
	beforeSaveStr := "//自动添加更新时间  没有trim \n"
	beforeSaveStr += `func (this *` + structName + `) BeforeSave(scope *gorm.Scope) (err error) {
		` + body + `
	return
	}

`

	return beforeSaveStr
}

func getMarshaler(structName, tableName string) string {
	marshalerStr := `func (this *` + structName + `) Marshaler() []byte {
		buf := jingo.NewBufferFromPool()
		` + tableName + `Enc.Marshal(this, buf)
		by := buf.Bytes
		buf.ReturnToPool()

		return by
	}

`

	return marshalerStr
}

func getBeforeCreateBody(getbeforeBody generateMysqlInfo, v *viper.Viper, structName string) string {
	var getbeforeBodyStr string

	if v.GetBool("mod") == true {
		getbeforeBodyStr += `
		err = conform.Struct(context.Background(), val)
		if err != nil {
			scope.Err(err)
			return err
		}

		`
	}

	if len(getbeforeBody.autoTime.CurTimeStamp) > 0 {
		getbeforeBodyStr += `
		switch scope.Value.(type){
		case *` + structName + `:
			val := scope.Value.(*` + structName + `)
		`
		for _, v := range getbeforeBody.autoTime.CurTimeStamp {
			getbeforeBodyStr += "if val." + v + ".Unix() < 0 {\n"
			getbeforeBodyStr += "val." + v + " = time.Now()\n"
			getbeforeBodyStr += "}\n\n"
		}
		getbeforeBodyStr += `
		default:
			log.Log.Warnf("unknown type")
		}`
	}

	return getbeforeBodyStr
}

func getBeforeUpdateBody(getbeforeBody generateMysqlInfo, v *viper.Viper, structName string) string {
	var getbeforeBodyStr string


	if len(getbeforeBody.autoTime.OnUpdateTimeStamp) > 0 {
		getbeforeBodyStr += `val, ok := scope.InstanceGet("gorm:update_attrs")
	if ok {
		switch val.(type) {
		case map[string]interface{}:`

		getbeforeBodyStr += "\n mapVal := val.(map[string]interface{})\n"

		for _, v := range getbeforeBody.autoTime.OnUpdateTimeStamp {
			getbeforeBodyStr += "if _, ok := mapVal[\"" + v + "\"]; !ok{\n"
			getbeforeBodyStr += "mapVal[\"" + v + "\"] = time.Now()\n"
			getbeforeBodyStr += "}\n\n"
		}

		getbeforeBodyStr += `default:
			log.Log.Warnf("unknown type")
		}
	}`

	}



	return getbeforeBodyStr
}

func getAfterFindBody(getbeforeBody generateMysqlInfo) string {
	return ""
}

func getAfterFind(structName, body string, hasData bool) string {

	afterFindStr := `func (this *` + structName + `) AfterFind(scope *gorm.Scope) (err error) {
		` + body

	if hasData == true {
		afterFindStr += `switch scope.Value.(type){
	case *` + structName + `:
		if scope.DB().RowsAffected > 0{
			val := scope.Value.(*` + structName + `)
			val.HasData = true
		}
	case *[]` + structName + `:
		if scope.DB().RowsAffected > 0 {
			vals := scope.Value.(*[]` + structName + `)
			for k, _ := range *vals {
				(*vals)[k].HasData = true
			}
		}
	default:
		log.Log.Warnf("unknown type")
	}
`
	}
	afterFindStr += `return
	}
`
	return afterFindStr
}
