package rest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/globalsign/mgo"
	"github.com/kuuland/kuu"
)

const (
	// ALL 全量模式
	ALL = "ALL"
	// PAGE 分页模式
	PAGE = "PAGE"
)

var (
	defaultMessages = map[string]string{
		"request_error": "Request failed.",
	}
	k     *kuu.Kuu
	name  string
	model func(string) *mgo.Collection
)

// params 请求参数
type params struct {
	page    int
	size    int
	_range  string
	sort    []string
	project map[string]int
	cond    map[string]interface{}
}

// parseParams 解析请求参数
func parseParams(c *gin.Context) *params {
	p := params{}
	// 处理page参数
	if c.Query("page") != "" {
		n, err := strconv.Atoi(c.Query("page"))
		if err == nil {
			p.page = n
		}
	}
	// 处理size参数
	if c.Query("size") != "" {
		n, err := strconv.Atoi(c.Query("size"))
		if err == nil {
			p.size = n
		}
	}
	// 处理range参数
	p._range = strings.ToUpper(c.Query("range"))
	if p._range != ALL && p._range != PAGE {
		p._range = PAGE
	}
	if p._range == PAGE {
		// PAGE模式下赋分页默认值
		if p.page == 0 {
			p.page = 1
		}
		if p.size == 0 {
			p.size = 20
		}
	}
	// 处理sort参数
	if c.Query("sort") != "" {
		p.sort = strings.Split(c.Query("sort"), ",")
	}
	// 处理project参数
	if c.Query("project") != "" {
		p.project = make(map[string]int)
		project := strings.Split(c.Query("project"), ",")
		for _, item := range project {
			var (
				key   string
				value int
			)
			exclude := false
			if strings.HasPrefix(item, "-") {
				exclude = true
			}
			if exclude == true {
				key = item[1:len(item)]
				value = 0
			} else {
				key = item
				value = 1
			}
			p.project[key] = value
		}
	}
	// 处理cond参数
	if c.Query("cond") != "" {
		cond := make(map[string]interface{})
		json.Unmarshal([]byte(c.Query("cond")), &cond)
		p.cond = cond
		// 避免服务端注入攻击
		if p.cond["$where"] != nil {
			delete(p.cond, "$where")
		}
	}
	return &p
}

func handleError(err error, c *gin.Context) {
	kuu.Error(err)
	c.JSON(http.StatusOK, kuu.StdError(kuu.SafeL(defaultMessages, c, "request_error")))
}

// Mount 挂载模型RESTful接口
func Mount(app *kuu.Kuu, n string, c func(string) *mgo.Collection) {
	k = app
	name = n
	model = c
	path := kuu.Join("/", strings.ToLower(name))
	k.POST(path, create)
	k.DELETE(path, remove)
	k.PUT(path, update)
	k.GET(path, list)
	k.GET(kuu.Join(path, "/:id"), id)
}
