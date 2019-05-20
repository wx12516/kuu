package kuu

import (
	"github.com/gin-gonic/gin"
	"regexp"
)

// OrgMiddleware
func OrgMiddleware(c *gin.Context) {
	// 解析登录信息
	var sign *SignContext
	if v, exists := c.Get(SignContextKey); exists {
		sign = v.(*SignContext)
	} else {
		if v, err := DecodedContext(c); err != nil {
			ERROR(err)
		} else {
			sign = v
		}
	}

	c.Next()

	reg := regexp.MustCompile("/login")
	if reg.MatchString(c.Request.RequestURI) {
		if v, exists := c.Get(SignContextKey); exists {
			sign = v.(*SignContext)
			if err := orgAutoLogin(c, sign); err != nil {
				ERROR(err)
				std := STDErr(nil, err.Error())
				std.Action = "ABORT"
				std.Render(c)
			}
		}
	}
}

func orgAutoLogin(c *gin.Context, sign *SignContext) error {
	if list, err := GetOrgList(c, sign.UID); err != nil {
		return err
	} else if len(*list) == 1 {
		orgs := *list
		first := (orgs)[0]
		_, err := ExecOrgLogin(c, sign, first.ID)
		return err

	}
	return nil
}
